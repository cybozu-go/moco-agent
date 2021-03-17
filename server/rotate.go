package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/log"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
)

// RotateLog rotates log files
func (a *Agent) RotateLog() {
	ctx := context.Background()

	metrics.IncrementLogRotationCountMetrics(a.clusterName)
	startTime := time.Now()

	errFile := filepath.Join(a.logDir, mocoagent.MySQLErrorLogName)
	err := os.Rename(errFile, errFile+".0")
	if err != nil && !os.IsNotExist(err) {
		log.Error("failed to rotate err log file", map[string]interface{}{
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics(a.clusterName)
		return
	}

	slowFile := filepath.Join(a.logDir, mocoagent.MySQLSlowLogName)
	err = os.Rename(slowFile, slowFile+".0")
	if err != nil && !os.IsNotExist(err) {
		log.Error("failed to rotate slow query log file", map[string]interface{}{
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics(a.clusterName)
		return
	}

	db, err := a.getMySQLConn()
	if err != nil {
		log.Error("failed to connect to database before log flush", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics(a.clusterName)
		return
	}

	if _, err := db.ExecContext(ctx, "FLUSH LOCAL ERROR LOGS, SLOW LOGS"); err != nil {
		log.Error("failed to exec mysql FLUSH", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics(a.clusterName)
		return
	}

	durationSeconds := time.Since(startTime).Seconds()
	metrics.UpdateLogRotationDurationSecondsMetrics(a.clusterName, durationSeconds)
}
