package mocoagent

import "github.com/cybozu-go/moco"

const (
	MetricsPort       = 8080
	ClusterNameEnvKey = "CLUSTER_NAME"
)

const (
	// AgentUser is a name of MOCO agent user in the MySQL context.
	AgentUser = "moco-agent"

	AgentPasswordEnvName = "AGENT_PASSWORD"

	AgentPasswordPath = moco.MySQLDataPath + "/agent-password"
)
