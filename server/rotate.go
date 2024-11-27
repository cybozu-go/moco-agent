package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
)

// RotateLog rotates log file
func (a *Agent) RotateLog() {
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

// RotateLogIfSizeExceeded rotates log file if it exceeds rotationSize
func (a *Agent) RotateLogIfSizeExceeded(rotationSize int64) {
	file := filepath.Join(a.logDir, mocoagent.MySQLSlowLogName)
	fileStat, err := os.Stat(file)
	if err != nil {
		a.logger.Error(err, "failed to get stat of log file")
		return
	}
	if fileStat.Size() > rotationSize {
		a.RotateLog()
	}
}
