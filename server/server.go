package server

import (
	"sync"
	"time"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/proto"
	"github.com/go-logr/logr"
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

// New returns an Agent
func New(config MySQLAccessorConfig, clusterName, socket, logDir string, maxDelay, transactionQueueingWait time.Duration, logger logr.Logger) (*Agent, error) {
	db, err := getMySQLConn(config)
	if err != nil {
		return nil, err
	}

	return &Agent{
		config:                  config,
		db:                      db,
		logger:                  logger,
		mysqlSocketPath:         socket,
		logDir:                  logDir,
		maxDelayThreshold:       maxDelay,
		transactionQueueingWait: transactionQueueingWait,
		cloneLock:               make(chan struct{}, 1),
	}, nil
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	config                  MySQLAccessorConfig
	db                      *sqlx.DB
	logger                  logr.Logger
	mysqlSocketPath         string
	logDir                  string
	maxDelayThreshold       time.Duration
	transactionQueueingWait time.Duration

	cloneLock    chan struct{}
	registryLock sync.Mutex
	registered   bool
}

func (a *Agent) configureReplicationMetrics(enable bool) {
	a.registryLock.Lock()
	defer a.registryLock.Unlock()

	if enable {
		if a.registered {
			return
		}
		metrics.RegisterReplicationMetrics(prometheus.DefaultRegisterer)
		a.registered = true
		return
	}

	if !a.registered {
		return
	}
	metrics.UnregisterReplicationMetrics(prometheus.DefaultRegisterer)
	a.registered = false
}

type MySQLAccessorConfig struct {
	Host              string
	Port              int
	Password          string
	ConnMaxIdleTime   time.Duration
	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
}

func (a *Agent) CloseDB() error {
	return a.db.Close()
}
