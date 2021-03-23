package server

import (
	"fmt"
	"net/http"

	"github.com/cybozu-go/log"
	mocoagent "github.com/cybozu-go/moco-agent"
)

// Health returns the health check result of own MySQL
func (a *Agent) MySQLDHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rows, err := a.db.QueryxContext(r.Context(), `SHOW MASTER STATUS`)
	if err != nil {
		log.Info("failed to execute a query", nil)
		http.Error(w, "failed to execute a query", http.StatusServiceUnavailable)
		return
	}
	rows.Close()
}

func (a *Agent) MySQLDReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Check the instance is under cloning or not
	mergedStatus, err := GetMySQLGlobalVariableAndCloneStateStatus(r.Context(), a.db)
	if err != nil {
		log.Error("failed to get global variables and/or clone status", map[string]interface{}{
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get global variables and/or clone status: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if mergedStatus.State.Valid && mergedStatus.State.String != mocoagent.CloneStatusCompleted {
		log.Info("the instance is under cloning", nil)
		http.Error(w, "the instance is under cloning", http.StatusServiceUnavailable)
		return
	}

	// Check the instance works primary or not
	if !mergedStatus.ReadOnly {
		return
	}

	// Check the instance has IO/SQLThread error or not
	replicaStatus, err := GetMySQLReplicaStatus(r.Context(), a.db)
	if err != nil {
		log.Error("failed to get replica status", map[string]interface{}{
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get replica status: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var hasIOThreadError, hasSQLThreadError bool
	if replicaStatus.LastIoErrno != 0 {
		hasIOThreadError = true
	}
	if replicaStatus.LastSQLErrno != 0 {
		hasSQLThreadError = true
	}
	if hasIOThreadError || hasSQLThreadError {
		log.Info("the instance has error(s)", map[string]interface{}{
			"hasIOThreadError":  hasIOThreadError,
			"hasSQLThreadError": hasSQLThreadError,
		})
		err = fmt.Errorf("the instance has error(s): hasIOThreadError=%t, hasSQLThreadError=%t", hasIOThreadError, hasSQLThreadError)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Check the delay isn't over the threshold
	if a.maxDelayThreshold == 0 {
		// Skip delay check
		return
	}

	timestamps, err := GetMySQLLastAppliedTransactionTimestamps(r.Context(), a.db)
	if err != nil {
		log.Error("failed to get transaction timestamps", map[string]interface{}{
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get transaction timestamps: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !timestamps.OriginalCommitTimestamp.Valid || !timestamps.EndApplyTimestamp.Valid {
		log.Error("failed to parse transaction timestamps", nil)
		http.Error(w, "failed to parse transaction timestamps", http.StatusInternalServerError)
		return
	}

	// This expression calculates the delay between "the end timestamp of last transaction at the own instance"
	// and "the original commit timestamp at the primary".
	// If this value becomes larger, it means the own instance cannot processing the original commits in time.
	delayed := timestamps.EndApplyTimestamp.Time.Sub(timestamps.OriginalCommitTimestamp.Time)
	if delayed >= a.maxDelayThreshold {
		log.Info("the instance delays from the primary", map[string]interface{}{
			"maxDelayThreshold": a.maxDelayThreshold,
			"delayed":           delayed,
		})
		err = fmt.Errorf("the instance delays from the primary: maxDelaySecondsThreshold=%s, delayed=%s", a.maxDelayThreshold, delayed)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
}
