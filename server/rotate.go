package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/metrics"
)

// RotateLog rotates log files
func (a *Agent) RotateLog() {
	ctx := context.Background()

	metrics.IncrementLogRotationCountMetrics()
	startTime := time.Now()

	errFile := filepath.Join(a.logDir, moco.MySQLErrorLogName)
	err := os.Rename(errFile, errFile+".0")
	if err != nil && !os.IsNotExist(err) {
		log.Error("failed to rotate err log file", map[string]interface{}{
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics()
		return
	}

	slowFile := filepath.Join(a.logDir, moco.MySQLSlowLogName)
	err = os.Rename(slowFile, slowFile+".0")
	if err != nil && !os.IsNotExist(err) {
		log.Error("failed to rotate slow query log file", map[string]interface{}{
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics()
		return
	}

	db, err := a.acc.Get(fmt.Sprintf("%s:%d", a.mysqlAdminHostname, a.mysqlAdminPort), moco.MiscUser, a.miscUserPassword)
	if err != nil {
		log.Error("failed to connect to database before log flush", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics()
		return
	}

	if _, err := db.ExecContext(ctx, "FLUSH LOCAL ERROR LOGS, SLOW LOGS"); err != nil {
		log.Error("failed to exec mysql FLUSH", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		metrics.IncrementLogRotationFailureCountMetrics()
		return
	}

	durationSeconds := time.Since(startTime).Seconds()
	metrics.UpdateLogRotationDurationSecondsMetrics(durationSeconds)
}
