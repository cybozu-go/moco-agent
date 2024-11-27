package mocoagent

// MySQL user names for MOCO
const (
	AdminUser       = "moco-admin"
	AgentUser       = "moco-agent"
	ReplicationUser = "moco-repl"
	CloneDonorUser  = "moco-clone-donor"
	ExporterUser    = "moco-exporter"
	BackupUser      = "moco-backup"
	ReadOnlyUser    = "moco-readonly"
	WritableUser    = "moco-writable"
)

// ENV keys for getting MySQL user passwords
const (
	AdminPasswordEnvKey       = "ADMIN_PASSWORD"
	AgentPasswordEnvKey       = "AGENT_PASSWORD"
	ReplicationPasswordEnvKey = "REPLICATION_PASSWORD"
	CloneDonorPasswordEnvKey  = "CLONE_DONOR_PASSWORD"
	ExporterPasswordKey       = "EXPORTER_PASSWORD"
	BackupPasswordKey         = "BACKUP_PASSWORD"
	ReadOnlyPasswordEnvKey    = "READONLY_PASSWORD"
	WritablePasswordEnvKey    = "WRITABLE_PASSWORD"
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

	// MySQLSlowLogName is a filekey of slow query log for MySQL.
	MySQLSlowLogName = "mysql.slow"
)
