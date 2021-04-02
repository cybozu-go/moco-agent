package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
)

// RotateLog rotates log files
func (a *Agent) RotateLog() {
	ctx := context.Background()

	a.logRotationCount.Inc()
	startTime := time.Now()

	errFile := filepath.Join(a.logDir, mocoagent.MySQLErrorLogName)
	err := os.Rename(errFile, errFile+".0")
	if err != nil && !os.IsNotExist(err) {
		a.logger.Error(err, "failed to rotate err log file")
		a.logRotationFailureCount.Inc()
		return
	}

	slowFile := filepath.Join(a.logDir, mocoagent.MySQLSlowLogName)
	err = os.Rename(slowFile, slowFile+".0")
	if err != nil && !os.IsNotExist(err) {
		a.logger.Error(err, "failed to rotate slow query log file")
		a.logRotationFailureCount.Inc()
		return
	}

	if _, err := a.db.ExecContext(ctx, "FLUSH LOCAL ERROR LOGS, SLOW LOGS"); err != nil {
		a.logger.Error(err, "failed to exec FLUSH LOCAL LOGS")
		a.logRotationFailureCount.Inc()
		return
	}

	durationSeconds := time.Since(startTime).Seconds()
	a.logRotationDurationSeconds.Observe(durationSeconds)
}
