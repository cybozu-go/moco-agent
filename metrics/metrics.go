package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "moco"
	metricsSubsystem = "agent"
)

var (
	binlogBackupCountMetrics           prometheus.Counter
	binlogBackupFailureCountMetrics    *prometheus.CounterVec
	binlogBackupDurationSecondsMetrics prometheus.Summary
	cloneCountMetrics                  prometheus.Counter
	cloneFailureCountMetrics           prometheus.Counter
	cloneDurationSecondsMetrics        prometheus.Summary
	logRotationCountMetrics            prometheus.Counter
	logRotationFailureCountMetrics     prometheus.Counter
	logRotationDurationSecondsMetrics  prometheus.Summary
	dumpBackupCountMetrics             prometheus.Counter
	dumpBackupFailureCountMetrics      *prometheus.CounterVec
	dumpBackupDurationSecondsMetrics   prometheus.Summary
)

func RegisterMetrics(registry *prometheus.Registry) {
	binlogBackupCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "binlog_backup_count",
		Help:      "The binlog backup operation count",
	})
	binlogBackupFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "binlog_backup_failure_count",
		Help:      "The failed binlog backup operation count",
	}, []string{"action"})
	binlogBackupDurationSecondsMetrics = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "binlog_backup_duration_seconds",
		Help:       "The time took to binlog backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	cloneCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_count",
		Help:      "The clone operation count",
	})
	cloneFailureCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_failure_count",
		Help:      "The clone operation count",
	})
	cloneDurationSecondsMetrics = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "clone_duration_seconds",
		Help:       "The time took to clone operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	logRotationCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_count",
		Help:      "The log rotation operation count",
	})
	logRotationFailureCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_failure_count",
		Help:      "The logRotation operation count",
	})
	logRotationDurationSecondsMetrics = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "log_rotation_duration_seconds",
		Help:       "The time took to log rotation operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	binlogBackupCountMetrics = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dump_backup_count",
		Help:      "The dump backup operation count",
	})
	binlogBackupFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dump_backup_failure_count",
		Help:      "The failed dump backup operation count",
	}, []string{"action"})
	binlogBackupDurationSecondsMetrics = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "dump_backup_duration_seconds",
		Help:       "The time took to dump backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	registry.MustRegister(
		binlogBackupCountMetrics,
		binlogBackupFailureCountMetrics,
		binlogBackupDurationSecondsMetrics,
		cloneCountMetrics,
		cloneFailureCountMetrics,
		cloneDurationSecondsMetrics,
		logRotationCountMetrics,
		logRotationFailureCountMetrics,
		logRotationDurationSecondsMetrics,
		dumpBackupCountMetrics,
		dumpBackupFailureCountMetrics,
		dumpBackupDurationSecondsMetrics,
	)
}
