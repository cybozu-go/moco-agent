package mocoagent

const (
	MetricsPort       = 8080
	ClusterNameEnvKey = "CLUSTER_NAME"
)

// MySQL user names for MOCO
const (
	// AdminUser is a name of MOCO operator-admin user.
	// This user is a super user especially for creating and granting privileges to other users.
	AdminUser = "moco-admin"

	// AgentUser is a name of MOCO agent user.
	AgentUser = "moco-agent"

	// ReplicationUser is a name of MOCO replicator user.
	ReplicationUser = "moco-repl"

	// CloneDonorUser is a name of MOCO clone-donor user.
	CloneDonorUser = "moco-clone-donor"

	// ReadOnlyUser is a name of MOCO predefined human user with wide read-only rights used for manual operation.
	ReadOnlyUser = "moco-readonly"

	// WritableUser is a name of MOCO predefined human user with wide read/write rights used for manual operation.
	WritableUser = "moco-writable"
)

// ENV names for initialize MySQL users
const (
	// AdminPasswordEnvName is a name of the environment variable of a password for both operator and operator-admin.
	AdminPasswordEnvName = "ADMIN_PASSWORD"

	// AgentPasswordEnvName is a name of the environment variable of a password for the misc user.
	AgentPasswordEnvName = "AGENT_PASSWORD"

	// ReplicationPasswordEnvName is a name of the environment variable of a password for replication user.
	ReplicationPasswordEnvName = "REPLICATION_PASSWORD"

	// ClonePasswordEnvName is a name of the environment variable of a password for donor user.
	ClonePasswordEnvName = "CLONE_DONOR_PASSWORD"

	// ReadOnlyPasswordEnvName is a name of the environment variable of a password for moco-readonly.
	ReadOnlyPasswordEnvName = "READONLY_PASSWORD"

	// WritablePasswordEnvName is a name of the environment variable of a password for moco-writable.
	WritablePasswordEnvName = "WRITABLE_PASSWORD"
)

const (
	// MySQLDataPath is a path for MySQL data dir.
	MySQLDataPath = "/var/lib/mysql"

	// AgentPasswordPath is a path for the password file for agent
	AgentPasswordPath = MySQLDataPath + "/agent-password"

	// MySQLSocketPathEnvName is a name of the environment variable of a path for MySQL unix domain socket file
	MySQLSocketPathEnvName = "MYSQL_SOCKET_PATH"

	// MySQLSocketDefaultPath is the default path for MySQL unix domain socket file
	MySQLSocketDefaultPath = "/var/run/mysqld/mysqld.sock"
)
