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
func New(podName, clusterName, agentUserPassword, donorUserPassword, replicationSourceSecretPath, mysqlSocketPath, logDir string, mysqlAdminPort int, config MySQLAccessorConfig) *Agent {
	return &Agent{
		sem:                         semaphore.NewWeighted(int64(maxCloneWorkers)),
		mysqlAdminHostname:          podName,
		mysqlAdminPort:              mysqlAdminPort,
		agentUserPassword:           agentUserPassword,
		donorUserPassword:           donorUserPassword,
		replicationSourceSecretPath: replicationSourceSecretPath,
		mysqlSocketPath:             mysqlSocketPath,
		clusterName:                 clusterName,
		logDir:                      logDir,
	}
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	sem                         *semaphore.Weighted
	accConfig                   MySQLAccessorConfig
	mysqlAdminHostname          string
	mysqlAdminPort              int
	agentUserPassword           string
	donorUserPassword           string
	replicationSourceSecretPath string
	mysqlSocketPath             string
	clusterName                 string
	logDir                      string
}

type MySQLAccessorConfig struct {
	ConnMaxLifeTime   time.Duration
	ConnectionTimeout time.Duration
	ReadTimeout       time.Duration
}

func (a *Agent) getMySQLConn() (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = mocoagent.AgentUser
	conf.Passwd = a.agentUserPassword
	conf.Net = "tcp"
	conf.Addr = fmt.Sprintf("%s:%d", a.mysqlAdminHostname, a.mysqlAdminPort)
	conf.Timeout = a.accConfig.ConnectionTimeout
	conf.ReadTimeout = a.accConfig.ReadTimeout
	conf.InterpolateParams = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(a.accConfig.ConnMaxLifeTime)

	return db, nil
}
