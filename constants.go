package mocoagent

// MySQL user names for MOCO
const (
	// AdminUser is the admin user name used by MOCO operator.
	// This user is a super user especially for creating and granting privileges to other users.
	AdminUser = "moco-admin"

	// AgentUser is the user name used by MOCO Agent.
	AgentUser = "moco-agent"

	// ReplicationUser is the user name used for replication.
	ReplicationUser = "moco-repl"

	// CloneDonorUser is the user name used for clone donor.
	CloneDonorUser = "moco-clone-donor"

	// ReadOnlyUser is the readonly user name used for human operator
	ReadOnlyUser = "moco-readonly"

	// WritableUser is the writable user name used for human operator
	WritableUser = "moco-writable"
)

// ENV keys for initialize MySQL users
const (
	// AdminPasswordEnvKey is the ENV key of moco-admin's password
	AdminPasswordEnvKey = "ADMIN_PASSWORD"

	// AgentPasswordEnvKey is the ENV key of moco-agent's password
	AgentPasswordEnvKey = "AGENT_PASSWORD"

	// ReplicationPasswordEnvKey is the ENV key of moco-repl's password
	ReplicationPasswordEnvKey = "REPLICATION_PASSWORD"

	// CloneDonorPasswordEnvKey is the ENV key of moco-clone-donor's password
	CloneDonorPasswordEnvKey = "CLONE_DONOR_PASSWORD"

	// ReadOnlyPasswordEnvKey is the ENV key of moco-readonly's passowrd
	ReadOnlyPasswordEnvKey = "READONLY_PASSWORD"

	// WritablePasswordEnvKey is the ENV key of moco-writable's password
	WritablePasswordEnvKey = "WRITABLE_PASSWORD"
)

// ENV keys for the values propergated by Kubernetes resource
const (
	// PodNameEnvKey is the ENV key of the own pod name
	PodNameEnvKey = "POD_NAME"

	// ClusterNameEnvKey is the ENV key of the cluster where the agent located
	ClusterNameEnvKey = "CLUSTER_NAME"
)

const (
	// VarLogPath is a path for /var/log/mysql.
	VarLogPath = "/var/log/mysql"

	// MySQLAdminPort is a port number for MySQL Admin
	MySQLAdminPort = 33062

	// MySQLErrorLogName is a filekey of error log for MySQL.
	MySQLErrorLogName = "mysql.err"

	// MySQLSlowLogName is a filekey of slow query log for MySQL.
	MySQLSlowLogName = "mysql.slow"
)
