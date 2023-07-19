package server

import (
	"sync"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
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
func New(config MySQLAccessorConfig, clusterName, agentPassword, socket, logDir string, maxDelay time.Duration, logger logr.Logger) (*Agent, error) {
	db, err := GetMySQLConnLocalSocket(mocoagent.AgentUser, agentPassword, socket,
		WithAccessorConfig(config), WithConnMaxLifeTime(5*time.Minute))
	if err != nil {
		return nil, err
	}

	return &Agent{
		agentUserPassword: agentPassword,
		db:                db,
		logger:            logger,
		mysqlSocketPath:   socket,
		logDir:            logDir,
		maxDelayThreshold: maxDelay,
		cloneLock:         make(chan struct{}, 1),
	}, nil
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	agentUserPassword string
	db                *sqlx.DB
	logger            logr.Logger
	mysqlSocketPath   string
	logDir            string
	maxDelayThreshold time.Duration

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
	ConnMaxIdleTime   time.Duration
	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
}

func (a *Agent) CloseDB() error {
	return a.db.Close()
}
