package server

import (
	"github.com/cybozu-go/moco/accessor"
	"golang.org/x/sync/semaphore"
)

const maxCloneWorkers = 1

// New returns a Agent
func New(podName, clusterName, token, agentUserPassword, donorUserPassword, replicationSourceSecretPath, logDir string, mysqlAdminPort int, config *accessor.MySQLAccessorConfig) *Agent {
	return &Agent{
		sem:                         semaphore.NewWeighted(int64(maxCloneWorkers)),
		acc:                         accessor.NewMySQLAccessor(config),
		mysqlAdminHostname:          podName,
		mysqlAdminPort:              mysqlAdminPort,
		agentUserPassword:           agentUserPassword,
		donorUserPassword:           donorUserPassword,
		replicationSourceSecretPath: replicationSourceSecretPath,
		clusterName:                 clusterName,
		token:                       token,
		logDir:                      logDir,
	}
}

// Agent is the agent to executes some MySQL commands of the own Pod
type Agent struct {
	sem                         *semaphore.Weighted
	acc                         *accessor.MySQLAccessor
	mysqlAdminHostname          string
	mysqlAdminPort              int
	agentUserPassword           string
	donorUserPassword           string
	replicationSourceSecretPath string
	clusterName                 string
	token                       string
	logDir                      string
}
