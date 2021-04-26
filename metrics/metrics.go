package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "moco"
	subsystem = "instance"
)

// moco-agent metrics
var (
	ReplicationDelay           prometheus.Gauge
	CloneCount                 prometheus.Counter
	CloneFailureCount          prometheus.Counter
	CloneDurationSeconds       prometheus.Summary
	CloneInProgress            prometheus.Gauge
	LogRotationCount           prometheus.Counter
	LogRotationFailureCount    prometheus.Counter
	LogRotationDurationSeconds prometheus.Summary
)

// Init initializes and registers MOCO's metrics to the registry
func Init(registry prometheus.Registerer, name string, index int) {
	labels := prometheus.Labels{
		"name":  name,
		"index": strconv.Itoa(index),
	}
	ReplicationDelay = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "replication_delay_seconds",
		Help:        "The seconds how much delay to replicate data",
		ConstLabels: labels,
	})
	CloneCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "clone_count",
		Help:        "The number of clone operations",
		ConstLabels: labels,
	})
	CloneFailureCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "clone_failure_count",
		Help:        "The number of times clone operation failed",
		ConstLabels: labels,
	})
	CloneDurationSeconds = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "clone_duration_seconds",
		Help:        "The time took to clone operation",
		ConstLabels: labels,
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	CloneInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "clone_in_progress",
		Help:        "Whether the clone operation is in progress or not",
		ConstLabels: labels,
	})
	LogRotationCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "log_rotation_count",
		Help:      "The log rotation operation count",
	})
	LogRotationFailureCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "log_rotation_failure_count",
		Help:        "The logRotation operation count",
		ConstLabels: labels,
	})
	LogRotationDurationSeconds = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        "log_rotation_duration_seconds",
		Help:        "The time took to log rotation operation",
		ConstLabels: labels,
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	registry.MustRegister(
		CloneCount,
		CloneFailureCount,
		CloneDurationSeconds,
		CloneInProgress,
		LogRotationCount,
		LogRotationFailureCount,
		LogRotationDurationSeconds,
	)
}

func RegisterReplicationMetrics(registry prometheus.Registerer) {
	UnregisterReplicationMetrics(registry)
	registry.MustRegister(ReplicationDelay)
}

func UnregisterReplicationMetrics(registry prometheus.Registerer) {
	registry.Unregister(ReplicationDelay)
}
