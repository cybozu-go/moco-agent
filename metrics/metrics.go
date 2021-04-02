package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "moco"
	metricsSubsystem = "agent"
)

var (
	CloneCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_count",
		Help:      "The clone operation count",
	}, []string{"cluster"})
	CloneFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_failure_count",
		Help:      "The clone operation count",
	}, []string{"cluster"})
	CloneDurationSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "clone_duration_seconds",
		Help:       "The time took to clone operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster"})
	CloneInProgress = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "clone_in_progress",
		Help:      "Whether the clone operation is in progress or not",
	}, []string{"cluster"})

	LogRotationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_count",
		Help:      "The log rotation operation count",
	}, []string{"cluster"})
	LogRotationFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "log_rotation_failure_count",
		Help:      "The logRotation operation count",
	}, []string{"cluster"})
	LogRotationDurationSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  metricsNamespace,
		Subsystem:  metricsSubsystem,
		Name:       "log_rotation_duration_seconds",
		Help:       "The time took to log rotation operation",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"cluster"})
)

// RegisterMetrics registers MOCO's metrics to the registry
func RegisterMetrics(registry prometheus.Registerer) {
	registry.MustRegister(
		CloneCount,
		CloneFailureCount,
		CloneDurationSeconds,
		CloneInProgress,
	)
	registry.MustRegister(
		LogRotationCount,
		LogRotationFailureCount,
		LogRotationDurationSeconds,
	)
}
