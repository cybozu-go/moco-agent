package server

import (
	"fmt"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/semaphore"
)

const maxCloneWorkers = 1

// New returns a Agent
func New(podName, clusterName, agentUserPassword, donorUserPassword, replicationSourceSecretPath, mysqlSocketPath, logDir string, mysqlAdminPort int, config MySQLAccessorConfig, maxDelayThreshold time.Duration) (*Agent, error) {
	db, err := getMySQLConn(mocoagent.AgentUser, agentUserPassword, podName, mysqlAdminPort, config)
	if err != nil {
		return nil, err
	}

	return &Agent{
		db:                          db,
		sem:                         semaphore.NewWeighted(int64(maxCloneWorkers)),
		donorUserPassword:           donorUserPassword,
		replicationSourceSecretPath: replicationSourceSecretPath,
		mysqlSocketPath:             mysqlSocketPath,
		clusterName:                 clusterName,
		logDir:                      logDir,
		maxDelayThreshold:           maxDelayThreshold,
	}, nil
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	db                          *sqlx.DB
	sem                         *semaphore.Weighted
	donorUserPassword           string
	replicationSourceSecretPath string
	mysqlSocketPath             string
	clusterName                 string
	logDir                      string
	maxDelayThreshold           time.Duration
}

type MySQLAccessorConfig struct {
	ConnMaxLifeTime   time.Duration
	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
}

func (a *Agent) CloseDB() error {
	return a.db.Close()
}

func getMySQLConn(user, password, host string, port int, config MySQLAccessorConfig) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "tcp"
	conf.Addr = fmt.Sprintf("%s:%d", host, port)
	conf.Timeout = config.ConnectionTimeout
	conf.ReadTimeout = config.ReadTimeout
	conf.InterpolateParams = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(config.ConnMaxLifeTime)
	db.SetMaxIdleConns(0)

	return db, nil
}
