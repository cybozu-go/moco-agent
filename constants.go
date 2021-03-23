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
	// MySQLDataPath is a path for MySQL data dir.
	MySQLDataPath = "/var/lib/mysql"

	// MySQLSocketPathEnvKey is the ENV key of a path for MySQL unix domain socket file
	MySQLSocketPathEnvKey = "MYSQL_SOCKET_PATH"

	// MySQLSocketDefaultPath is the default path for MySQL unix domain socket file
	MySQLSocketDefaultPath = "/run/mysqld.sock"

	// AgentPasswordPath is a path for the password file for agent
	AgentPasswordPath = MySQLDataPath + "/agent-password"

	// DonorPasswordPath is the path to donor user passsword file
	DonorPasswordPath = MySQLDataPath + "/donor-password"

	// MySQLPasswordFilePath includes the embed users' password of MOCO (used for restoring users)
	MySQLPasswordFilePath = "/tmp/moco-root-password"

	// MySQLPingConfFilePath is the file path of credential used for `moco-agent ping`
	MySQLPingConfFilePath = MySQLDataPath + "/agent.cnf"

	// MySQLConfigFileName is the file key of MySQL config
	MySQLConfFileName = "my.cnf"

	// ReplicationSourceSecretPath is the path to replication source secret file
	ReplicationSourceSecretPath = MySQLDataPath + "/replication-source-secret"

	// VarLogPath is a path for /var/log/mysql.
	VarLogPath = "/var/log/mysql"

	// MySQLAdminPort is a port number for MySQL Admin
	MySQLAdminPort = 33062

	// MySQLConfTemplatePath is
	MySQLConfTemplatePath = "/etc/mysql_template"

	// MySQLConfPath is a path for MySQL conf dir.
	MySQLConfPath = "/etc/mysql"

	// MySQLErrorLogName is a filekey of error log for MySQL.
	MySQLErrorLogName = "mysql.err"

	// MySQLSlowLogName is a filekey of slow query log for MySQL.
	MySQLSlowLogName = "mysql.slow"

	// ReplicationSourcePrimaryHostKey etc. are Secret key for replication source secret
	ReplicationSourcePrimaryHostKey            = "PRIMARY_HOST"
	ReplicationSourcePrimaryUserKey            = "PRIMARY_USER"
	ReplicationSourcePrimaryPasswordKey        = "PRIMARY_PASSWORD"
	ReplicationSourcePrimaryPortKey            = "PRIMARY_PORT"
	ReplicationSourceCloneUserKey              = "CLONE_USER"
	ReplicationSourceClonePasswordKey          = "CLONE_PASSWORD"
	ReplicationSourceInitAfterCloneUserKey     = "INIT_AFTER_CLONE_USER"
	ReplicationSourceInitAfterClonePasswordKey = "INIT_AFTER_CLONE_PASSWORD"
)

// The status strings of MySQL status
const (
	ReplicaRunConnect    = "Yes"
	ReplicaNotRun        = "No"
	ReplicaRunNotConnect = "Connecting"

	CloneStatusNotStarted = "Not Started"
	CloneStatusInProgress = "In Progress"
	CloneStatusCompleted  = "Completed"
	CloneStatusFailed     = "Failed"
)
