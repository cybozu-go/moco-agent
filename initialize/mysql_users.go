package initialize

import (
	"context"
	"fmt"
	"os"
	"strings"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/jmoiron/sqlx"
)

type userSetting struct {
	name                    string
	password                string
	privileges              []string
	revokePrivileges        map[string][]string
	withGrantOption         bool
	useNativePasswordPlugin bool
}

var (
	users = []userSetting{
		{
			name:     mocoagent.AdminUser,
			password: os.Getenv(mocoagent.AdminPasswordEnvKey),
			privileges: []string{
				"ALL",
			},
			withGrantOption: true,
		},
		{
			name:     mocoagent.AgentUser,
			password: os.Getenv(mocoagent.AgentPasswordEnvKey),
			privileges: []string{
				"SELECT",
				"RELOAD",
				"CLONE_ADMIN",
				"BINLOG_ADMIN",
				"SERVICE_CONNECTION_ADMIN",
				"REPLICATION CLIENT",
			},
		},
		{
			name:     mocoagent.ReplicationUser,
			password: os.Getenv(mocoagent.ReplicationPasswordEnvKey),
			privileges: []string{
				"REPLICATION SLAVE",
				"REPLICATION CLIENT",
			},
			// TODO: When using encrypted connection, "WITH mysql_native_password" should be deleted.
			// See https://yoku0825.blogspot.com/2018/10/mysql-80cachingsha2password-ssl.html
			useNativePasswordPlugin: true,
		},
		{
			name:     mocoagent.CloneDonorUser,
			password: os.Getenv(mocoagent.CloneDonorPasswordEnvKey),
			privileges: []string{
				"BACKUP_ADMIN",
				"SERVICE_CONNECTION_ADMIN",
			},
		},
		{
			name:     mocoagent.ReadOnlyUser,
			password: os.Getenv(mocoagent.ReadOnlyPasswordEnvKey),
			privileges: []string{
				"PROCESS",
				"SELECT",
				"SHOW DATABASES",
				"SHOW VIEW",
				"REPLICATION CLIENT",
				"REPLICATION SLAVE",
			},
		},
		{
			name:     mocoagent.WritableUser,
			password: os.Getenv(mocoagent.WritablePasswordEnvKey),
			privileges: []string{
				"ALTER",
				"ALTER ROUTINE",
				"CREATE",
				"CREATE ROLE",
				"CREATE ROUTINE",
				"CREATE TEMPORARY TABLES",
				"CREATE USER",
				"CREATE VIEW",
				"DELETE",
				"DROP",
				"DROP ROLE",
				"EVENT",
				"EXECUTE",
				"INDEX",
				"INSERT",
				"LOCK TABLES",
				"PROCESS",
				"REFERENCES",
				"REPLICATION CLIENT",
				"REPLICATION SLAVE",
				"SELECT",
				"SHOW DATABASES",
				"SHOW VIEW",
				"TRIGGER",
				"UPDATE",
			},
			revokePrivileges: map[string][]string{
				"mysql.*": {
					"CREATE",
					"CREATE ROUTINE",
					"CREATE TEMPORARY TABLES",
					"CREATE VIEW",
					"DELETE",
					"DROP",
					"EVENT",
					"EXECUTE",
					"INDEX",
					"INSERT",
					"LOCK TABLES",
					"REFERENCES",
					"TRIGGER",
					"UPDATE",
				},
			},
			withGrantOption: true,
		},
	}
)

func EnsureMOCOUsers(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, "SET GLOBAL partial_revokes='ON'")
	if err != nil {
		return err
	}

	for _, u := range users {
		err := ensureMySQLUser(ctx, db, u)
		if err != nil {
			return err
		}
	}

	return nil
}

func DropLocalRootUser(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, "DROP USER IF EXISTS 'root'@'localhost'")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	return err
}

func ensureMySQLUser(ctx context.Context, db *sqlx.DB, user userSetting) error {
	var count int
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM mysql.user WHERE user=? and host='%'`, user.name)
	if err != nil {
		return err
	}
	if count == 1 {
		// The user is already exists
		return nil
	}

	queryStr := `CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED`
	if user.useNativePasswordPlugin {
		queryStr = queryStr + " WITH mysql_native_password"
	}
	queryStr = queryStr + " BY ?"
	_, err = db.ExecContext(ctx, queryStr, user.name, user.password)
	if err != nil {
		return err
	}

	queryStr = fmt.Sprintf(`GRANT %s ON *.* TO ?@'%%'`, strings.Join(user.privileges, ","))
	if user.withGrantOption {
		queryStr = queryStr + " WITH GRANT OPTION"
	}
	_, err = db.ExecContext(ctx, queryStr, user.name)
	if err != nil {
		return err
	}

	for target, privileges := range user.revokePrivileges {
		queryStr = fmt.Sprintf(`REVOKE %s ON %s FROM ?@'%%'`, strings.Join(privileges, ","), target)

		_, err = db.ExecContext(ctx, queryStr, user.name)
		if err != nil {
			return err
		}
	}

	return nil
}
