package server

import (
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/proto"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
)

// NewAgentService creates a new AgentServer
func NewAgentService(agent *Agent) proto.AgentServer {
	return agentService{agent: agent}
}

type agentService struct {
	agent *Agent
	proto.UnimplementedAgentServer
}

// New returns a Agent
func New(podName, clusterName, agentUserPassword, mysqlSocketPath, logDir string, mysqlAdminPort int, config MySQLAccessorConfig, maxDelayThreshold time.Duration) (*Agent, error) {
	db, err := getMySQLConn(mocoagent.AgentUser, agentUserPassword, podName, mysqlAdminPort, config)
	if err != nil {
		return nil, err
	}

	return &Agent{
		db:                         db,
		mysqlSocketPath:            mysqlSocketPath,
		logDir:                     logDir,
		maxDelayThreshold:          maxDelayThreshold,
		cloneLock:                  make(chan struct{}, 1),
		cloneCount:                 metrics.CloneCount.WithLabelValues(clusterName),
		cloneFailureCount:          metrics.CloneFailureCount.WithLabelValues(clusterName),
		cloneDurationSeconds:       metrics.CloneDurationSeconds.WithLabelValues(clusterName),
		cloneInProgress:            metrics.CloneInProgress.WithLabelValues(clusterName),
		logRotationCount:           metrics.LogRotationCount.WithLabelValues(clusterName),
		logRotationFailureCount:    metrics.LogRotationFailureCount.WithLabelValues(clusterName),
		logRotationDurationSeconds: metrics.LogRotationDurationSeconds.WithLabelValues(clusterName),
	}, nil
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	db                *sqlx.DB
	mysqlSocketPath   string
	logDir            string
	maxDelayThreshold time.Duration

	cloneLock                  chan struct{}
	cloneCount                 prometheus.Counter
	cloneFailureCount          prometheus.Counter
	cloneDurationSeconds       prometheus.Observer
	cloneInProgress            prometheus.Gauge
	logRotationCount           prometheus.Counter
	logRotationFailureCount    prometheus.Counter
	logRotationDurationSeconds prometheus.Observer
}

type MySQLAccessorConfig struct {
	ConnMaxIdleTime   time.Duration
	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
}

func (a *Agent) CloseDB() error {
	return a.db.Close()
}
