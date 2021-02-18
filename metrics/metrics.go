package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "moco"
	metricsSubsystem = "agent"
)

var (
	binlogBackupCountMetrics           *prometheus.CounterVec
	binlogBackupFailureCountMetrics    *prometheus.CounterVec
	binlogBackupDurationSecondsMetrics *prometheus.SummaryVec

	cloneCountMetrics           *prometheus.CounterVec
	cloneFailureCountMetrics    *prometheus.CounterVec
	cloneDurationSecondsMetrics *prometheus.SummaryVec

	logRotationCountMetrics           *prometheus.CounterVec
	logRotationFailureCountMetrics    *prometheus.CounterVec
	logRotationDurationSecondsMetrics *prometheus.SummaryVec

	dumpBackupCountMetrics           *prometheus.CounterVec
	dumpBackupFailureCountMetrics    *prometheus.CounterVec
	dumpBackupDurationSecondsMetrics *prometheus.SummaryVec
)

// RegisterMetrics registers MOCO's metrics to the registry
func RegisterMetrics(registry *prometheus.Registry) {
	registerBinlogBackupMetrics(registry)
	registerCloneMetrics(registry)
	registerLogrotationMetrics(registry)
	registerDumpBackupMetrics(registry)
}

func registerBinlogBackupMetrics(registry *prometheus.Registry) {
	binlogBackupCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "binlog_backup_count",
		Help:      "The binlog backup operation count",
	}, []string{"cluster_name"})
	binlogBackupFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "binlog_backup_failure_count",
		Help:      "The failed binlog backup operation count",
	}, []string{"action", "cluster_name"})
	binlogBackupDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "binlog_backup_duration_seconds",
		Help:       "The time took to binlog backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})

	registry.MustRegister(
		binlogBackupCountMetrics,
		binlogBackupFailureCountMetrics,
		binlogBackupDurationSecondsMetrics,
	)
}

func registerCloneMetrics(registry *prometheus.Registry) {
	cloneCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_count",
		Help:      "The clone operation count",
	}, []string{"cluster_name"})
	cloneFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_failure_count",
		Help:      "The clone operation count",
	}, []string{"cluster_name"})
	cloneDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "clone_duration_seconds",
		Help:       "The time took to clone operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})

	registry.MustRegister(
		cloneCountMetrics,
		cloneFailureCountMetrics,
		cloneDurationSecondsMetrics,
	)
}

func registerLogrotationMetrics(registry *prometheus.Registry) {
	logRotationCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_count",
		Help:      "The log rotation operation count",
	}, []string{"cluster_name"})
	logRotationFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_failure_count",
		Help:      "The logRotation operation count",
	}, []string{"cluster_name"})
	logRotationDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "log_rotation_duration_seconds",
		Help:       "The time took to log rotation operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})

	registry.MustRegister(
		logRotationCountMetrics,
		logRotationFailureCountMetrics,
		logRotationDurationSecondsMetrics,
	)
}

func registerDumpBackupMetrics(registry *prometheus.Registry) {
	dumpBackupCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dump_backup_count",
		Help:      "The dump backup operation count",
	}, []string{"cluster_name"})
	dumpBackupFailureCountMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dump_backup_failure_count",
		Help:      "The failed dump backup operation count",
	}, []string{"action", "cluster_name"})
	dumpBackupDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "dump_backup_duration_seconds",
		Help:       "The time took to dump backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})

	registry.MustRegister(
		dumpBackupCountMetrics,
		dumpBackupFailureCountMetrics,
		dumpBackupDurationSecondsMetrics,
	)
}
