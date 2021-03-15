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
	binlogBackupInProgressMetrics      *prometheus.GaugeVec

	cloneCountMetrics           *prometheus.CounterVec
	cloneFailureCountMetrics    *prometheus.CounterVec
	cloneDurationSecondsMetrics *prometheus.SummaryVec
	cloneInProgressMetrics      *prometheus.GaugeVec

	logRotationCountMetrics           *prometheus.CounterVec
	logRotationFailureCountMetrics    *prometheus.CounterVec
	logRotationDurationSecondsMetrics *prometheus.SummaryVec

	dumpBackupCountMetrics           *prometheus.CounterVec
	dumpBackupFailureCountMetrics    *prometheus.CounterVec
	dumpBackupDurationSecondsMetrics *prometheus.SummaryVec
	dumpBackupInProgressMetrics      *prometheus.GaugeVec
)

// RegisterMetrics registers MOCO's metrics to the registry
func RegisterMetrics(registry prometheus.Registerer) {
	registerBinlogBackupMetrics(registry)
	registerCloneMetrics(registry)
	registerLogrotationMetrics(registry)
	registerDumpBackupMetrics(registry)
}

func registerBinlogBackupMetrics(registry prometheus.Registerer) {
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
	}, []string{"cluster_name", "action"})
	binlogBackupDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "binlog_backup_duration_seconds",
		Help:       "The time took to binlog backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})
	binlogBackupInProgressMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "binlog_backup_in_progress",
		Help:      "Whether the binlog backup operation is in progress or not",
	}, []string{"cluster_name"})

	registry.MustRegister(
		binlogBackupCountMetrics,
		binlogBackupFailureCountMetrics,
		binlogBackupDurationSecondsMetrics,
		binlogBackupInProgressMetrics,
	)
}

func registerCloneMetrics(registry prometheus.Registerer) {
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
	cloneInProgressMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_in_progress",
		Help:      "Whether the clone operation is in progress or not",
	}, []string{"cluster_name"})

	registry.MustRegister(
		cloneCountMetrics,
		cloneFailureCountMetrics,
		cloneDurationSecondsMetrics,
		cloneInProgressMetrics,
	)
}

func registerLogrotationMetrics(registry prometheus.Registerer) {
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

func registerDumpBackupMetrics(registry prometheus.Registerer) {
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
	}, []string{"cluster_name", "action"})
	dumpBackupDurationSecondsMetrics = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "dump_backup_duration_seconds",
		Help:       "The time took to dump backup operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster_name"})
	dumpBackupInProgressMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dump_backup_in_progress",
		Help:      "Whether the dump backup operation is in progress or not",
	}, []string{"cluster_name"})

	registry.MustRegister(
		dumpBackupCountMetrics,
		dumpBackupFailureCountMetrics,
		dumpBackupDurationSecondsMetrics,
		dumpBackupInProgressMetrics,
	)
}
