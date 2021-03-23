package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cybozu-go/log"
	mocoagent "github.com/cybozu-go/moco-agent"
)

// Health returns the health check result of own MySQL
func (a *Agent) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	db, err := a.getMySQLConn()
	if err != nil {
		log.Error("failed to connect to database before health check", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "failed to connect to database before health check: %+v", err)
		return
	}
	defer db.Close()

	_, err = db.QueryxContext(r.Context(), `SHOW MASTER STATUS`)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Print("failed to execute a query")
		return
	}
}

func (a *Agent) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	db, err := a.getMySQLConn()
	if err != nil {
		log.Error("failed to connect to database before readiness check", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "failed to connect to database before readiness check: %+v", err)
		return
	}
	defer db.Close()

	// Check the instance is under cloning or not
	cloneStatus, err := GetMySQLCloneStateStatus(r.Context(), db)
	if err != nil {
		log.Error("failed to get clone status", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get clone status: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var isUnderCloning bool
	if cloneStatus.State.Valid && cloneStatus.State.String != mocoagent.CloneStatusCompleted {
		isUnderCloning = true
	}
	if isUnderCloning {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "the instance is under cloning")
		return
	}

	// Check the instance works primary or not
	globalVariables, err := GetMySQLGlobalVariablesStatus(r.Context(), db)
	if err != nil {
		log.Error("failed to get global variables", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get global variables: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !globalVariables.ReadOnly {
		return
	}

	// Check the instance has IO/SQLThread error or not
	replicaStatus, err := GetMySQLReplicaStatus(r.Context(), db)
	if err != nil {
		log.Error("failed to get replica status", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get replica status: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if replicaStatus == nil {
		log.Info("the instance is under reconciling: read_only=true, but not works as a replica", nil)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "the instance is under reconciling: read_only=true, but not works as a replica")
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
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Print("the instance is under reconciling: read_only=true, but not works as a replica")
		return
	}

	// Check the instance has IO/SQLThread error or not
	timestamps, err := GetMySQLLastAppliedTransactionTimestamps(r.Context(), db)
	if err != nil {
		log.Error("failed to get transaction timestamps", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		err := fmt.Errorf("failed to get transaction timestamps: %+v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check the delay isn't over the threshold
	if !timestamps.OriginalCommitTimestamp.Valid || !timestamps.EndApplyTimestamp.Valid {
		log.Error("failed to parse transaction timestamps", nil)
		err := errors.New("failed to parse transaction timestamps")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	delayied := timestamps.EndApplyTimestamp.Time.Sub(timestamps.OriginalCommitTimestamp.Time)
	if delayied >= a.maxDelayThreshold {
		log.Info("the instance delays from the primary", map[string]interface{}{
			"maxDelayThreshold": a.maxDelayThreshold,
			"delayied":          delayied,
		})
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "the instance delays from the primary: maxDelaySecondsThreshold=%s, delayied=%s", a.maxDelayThreshold, delayied)
		return
	}
}
