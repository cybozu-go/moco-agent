package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
)

// RotateLog rotates error log files
func (a *Agent) RotateErrorLog() {
	ctx := context.Background()

	metrics.LogRotationCount.Inc()
	startTime := time.Now()

	errFile := filepath.Join(a.logDir, mocoagent.MySQLErrorLogName)
	err := os.Rename(errFile, errFile+".0")
	if err != nil && !os.IsNotExist(err) {
		a.logger.Error(err, "failed to rotate err log file")
		metrics.LogRotationFailureCount.Inc()
		return
	}

	if _, err := a.db.ExecContext(ctx, "FLUSH LOCAL ERROR LOGS"); err != nil {
		a.logger.Error(err, "failed to exec FLUSH LOCAL ERROR LOGS")
		metrics.LogRotationFailureCount.Inc()
		return
	}

	durationSeconds := time.Since(startTime).Seconds()
	metrics.LogRotationDurationSeconds.Observe(durationSeconds)
}

// RotateLog rotates slow log files
func (a *Agent) RotateSlowLog() {
	ctx := context.Background()

	metrics.LogRotationCount.Inc()
	startTime := time.Now()

	slowFile := filepath.Join(a.logDir, mocoagent.MySQLSlowLogName)
	err := os.Rename(slowFile, slowFile+".0")
	if err != nil && !os.IsNotExist(err) {
		a.logger.Error(err, "failed to rotate slow query log file")
		metrics.LogRotationFailureCount.Inc()
		return
	}

	if _, err := a.db.ExecContext(ctx, "FLUSH LOCAL SLOW LOGS"); err != nil {
		a.logger.Error(err, "failed to exec FLUSH LOCAL SLOW LOGS")
		metrics.LogRotationFailureCount.Inc()
		return
	}

	durationSeconds := time.Since(startTime).Seconds()
	metrics.LogRotationDurationSeconds.Observe(durationSeconds)
}

// RotateLogIfSizeExceeded rotates log files if it exceeds rotationSize
func (a *Agent) RotateLogIfSizeExceeded(rotationSize int64) {
	errFile := filepath.Join(a.logDir, mocoagent.MySQLErrorLogName)
	errFileStat, err := os.Stat(errFile)
	if err != nil {
		a.logger.Error(err, "failed to get stat of error log file")
		return
	}
	if errFileStat.Size() > rotationSize {
		a.RotateErrorLog()
	}

	slowFile := filepath.Join(a.logDir, mocoagent.MySQLSlowLogName)
	slowFileStat, err := os.Stat(slowFile)
	if err != nil {
		a.logger.Error(err, "failed to get stat of slow query log file")
		return
	}
	if slowFileStat.Size() > rotationSize {
		a.RotateSlowLog()
	}
}
