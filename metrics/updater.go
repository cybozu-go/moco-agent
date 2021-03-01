package metrics

func IncrementBinlogBackupCountMetrics(clusterName string) {
	binlogBackupCountMetrics.WithLabelValues(clusterName).Inc()
}
func IncrementBinlogBackupFailureCountMetrics(clusterName string, action string) {
	binlogBackupFailureCountMetrics.WithLabelValues(clusterName, action).Inc()
}
func UpdateBinlogBackupDurationSecondsMetrics(clusterName string, durationSeconds float64) {
	binlogBackupDurationSecondsMetrics.WithLabelValues(clusterName).Observe(durationSeconds)
}
func SetBinlogBackupInProgressMetrics(clusterName string, inProgress bool) {
	if inProgress {
		binlogBackupInProgressMetrics.WithLabelValues(clusterName).Set(1.0)
		return
	}
	binlogBackupInProgressMetrics.WithLabelValues(clusterName).Set(0.0)
}

func IncrementCloneCountMetrics(clusterName string) {
	cloneCountMetrics.WithLabelValues(clusterName).Inc()
}
func IncrementCloneFailureCountMetrics(clusterName string) {
	cloneFailureCountMetrics.WithLabelValues(clusterName).Inc()
}
func UpdateCloneDurationSecondsMetrics(clusterName string, durationSeconds float64) {
	cloneDurationSecondsMetrics.WithLabelValues(clusterName).Observe(durationSeconds)
}
func SetCloneInProgressMetrics(clusterName string, inProgress bool) {
	if inProgress {
		cloneInProgressMetrics.WithLabelValues(clusterName).Set(1.0)
		return
	}
	cloneInProgressMetrics.WithLabelValues(clusterName).Set(0.0)
}

func IncrementLogRotationCountMetrics(clusterName string) {
	logRotationCountMetrics.WithLabelValues(clusterName).Inc()
}
func IncrementLogRotationFailureCountMetrics(clusterName string) {
	logRotationFailureCountMetrics.WithLabelValues(clusterName).Inc()
}
func UpdateLogRotationDurationSecondsMetrics(clusterName string, durationSeconds float64) {
	logRotationDurationSecondsMetrics.WithLabelValues(clusterName).Observe(durationSeconds)
}

func IncrementDumpBackupCountMetrics(clusterName string) {
	dumpBackupCountMetrics.WithLabelValues(clusterName).Inc()
}
func IncrementDumpBackupFailureCountMetrics(clusterName string, action string) {
	dumpBackupFailureCountMetrics.WithLabelValues(clusterName, action).Inc()
}
func UpdateDumpBackupDurationSecondsMetrics(clusterName string, durationSeconds float64) {
	dumpBackupDurationSecondsMetrics.WithLabelValues(clusterName).Observe(durationSeconds)
}
