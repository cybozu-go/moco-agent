package metrics

func IncrementBinlogBackupCountMetrics() {
	binlogBackupCountMetrics.Inc()
}

func IncrementBinlogBackupFailureCountMetrics(action string) {
	binlogBackupFailureCountMetrics.WithLabelValues(action).Inc()
}

func UpdateBinlogBackupDurationSecondsMetrics(durationSeconds float64) {
	binlogBackupDurationSecondsMetrics.Observe(durationSeconds)
}

func IncrementCloneCountMetrics() {
	cloneCountMetrics.Inc()
}

func IncrementCloneFailureCountMetrics() {
	cloneFailureCountMetrics.Inc()
}

func UpdateCloneDurationSecondsMetrics(durationSeconds float64) {
	cloneDurationSecondsMetrics.Observe(durationSeconds)
}

func IncrementLogRotationCountMetrics() {
	logRotationCountMetrics.Inc()
}

func IncrementLogRotationFailureCountMetrics() {
	logRotationFailureCountMetrics.Inc()
}

func UpdateLogRotationDurationSecondsMetrics(durationSeconds float64) {
	logRotationDurationSecondsMetrics.Observe(durationSeconds)
}

func IncrementDumpBackupCountMetrics() {
	dumpBackupCountMetrics.Inc()
}

func IncrementDumpBackupFailureCountMetrics(action string) {
	dumpBackupFailureCountMetrics.WithLabelValues(action).Inc()
}

func UpdateDumpBackupDurationSecondsMetrics(durationSeconds float64) {
	dumpBackupDurationSecondsMetrics.Observe(durationSeconds)
}
