package server

import (
	"fmt"
	"net/http"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// Health returns the health check result of own MySQL
func (a *Agent) MySQLDHealth(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryxContext(r.Context(), `SELECT VERSION()`)
	if err != nil {
		a.logger.Info("health check failed")
		http.Error(w, "failed to execute a query", http.StatusServiceUnavailable)
		return
	}
	rows.Close()
}

func (a *Agent) MySQLDReady(w http.ResponseWriter, r *http.Request) {
	// Check the instance is under cloning or not
	cloneStatus, err := a.GetMySQLCloneStateStatus(r.Context())
	if err != nil {
		a.logger.Error(err, "failed to get clone status")
		msg := fmt.Sprintf("failed to get clone status: %+v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if cloneStatus.State.Valid && cloneStatus.State.String != "Completed" {
		a.logger.Info("the instance is under cloning")
		http.Error(w, "the instance is under cloning", http.StatusServiceUnavailable)
		return
	}

	// Check the instance works primary or not
	globalVariables, err := a.GetMySQLGlobalVariable(r.Context())
	if err != nil {
		a.logger.Error(err, "failed to get global variables")
		msg := fmt.Sprintf("failed to get global variables: %+v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	if !globalVariables.ReadOnly {
		a.configureReplicationMetrics(false)
		metrics.UnregisterReplicationMetrics(prometheus.DefaultRegisterer)
		return
	}

	// Check the instance has IO/SQLThread error or not
	replicaStatus, err := a.GetMySQLReplicaStatus(r.Context())
	if err != nil {
		a.configureReplicationMetrics(false)
		a.logger.Error(err, "failed to get replica status")
		msg := fmt.Sprintf("failed to get replica status: %+v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	if replicaStatus.SlaveIORunning != "Yes" || replicaStatus.SlaveSQLRunning != "Yes" {
		a.logger.Info("replication threads are stopped")
		http.Error(w, "replication thread are stopped", http.StatusServiceUnavailable)
		return
	}

	if replicaStatus.LastIOErrno != 0 || replicaStatus.LastSQLErrno != 0 {
		a.logger.Info("the instance has replication error(s)",
			"Last_IO_Errno", replicaStatus.LastIOErrno,
			"Last_IO_Error", replicaStatus.LastIOError,
			"Last_SQL_Errno", replicaStatus.LastSQLErrno,
			"Last_SQL_Error", replicaStatus.LastSQLError,
		)
		http.Error(w, "the instance has replication errors", http.StatusServiceUnavailable)
		return
	}

	// Check the delay isn't over the threshold
	if a.maxDelayThreshold == 0 {
		// Skip delay check
		return
	}

	timestamps, err := a.GetMySQLLastAppliedTransactionTimestamps(r.Context())
	if err != nil {
		a.logger.Error(err, "failed to get transaction timestamps")
		msg := fmt.Sprintf("failed to get transaction timestamps: %+v", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	// This expression calculates the delay between "the end timestamp of last transaction at the own instance"
	// and "the original commit timestamp at the primary".
	// If this value becomes larger, it means the own instance cannot processing the original commits in time.
	delayed := timestamps.EndApplyTimestamp.Sub(timestamps.OriginalCommitTimestamp)
	a.configureReplicationMetrics(true)
	metrics.ReplicationDelay.Set(delayed.Seconds())
	if delayed >= a.maxDelayThreshold {
		a.logger.Info("the instance delays from the primary",
			"maxDelayThreshold", a.maxDelayThreshold,
			"delayed", delayed,
		)
		msg := fmt.Sprintf("the instance delays from the primary: maxDelaySecondsThreshold=%v, delayed=%v", a.maxDelayThreshold, delayed)
		http.Error(w, msg, http.StatusServiceUnavailable)
		return
	}
}
