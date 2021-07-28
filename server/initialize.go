package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/jmoiron/sqlx"
)

const initTimeout = 1 * time.Minute

// UserSetting represents settings for a MySQL user.
type UserSetting struct {
	name             string
	privileges       []string
	proxyAdmin       bool
	revokePrivileges map[string][]string
	withGrantOption  bool
}

var Users = []UserSetting{
	{
		name: mocoagent.AdminUser,
		privileges: []string{
			"ALL",
		},
		proxyAdmin:      true,
		withGrantOption: true,
	},
	{
		name: mocoagent.AgentUser,
		privileges: []string{
			"BINLOG_ADMIN",
			"CLONE_ADMIN",
			"RELOAD",
			"REPLICATION CLIENT",
			"SELECT",
			"SERVICE_CONNECTION_ADMIN",
			"SYSTEM_VARIABLES_ADMIN",
		},
	},
	{
		name: mocoagent.ReplicationUser,
		privileges: []string{
			"REPLICATION CLIENT",
			"REPLICATION SLAVE",
		},
	},
	{
		name: mocoagent.CloneDonorUser,
		privileges: []string{
			"BACKUP_ADMIN",
			"SERVICE_CONNECTION_ADMIN",
		},
	},
	{
		name: mocoagent.ExporterUser,
		privileges: []string{
			"PROCESS",
			"REPLICATION CLIENT",
			"SELECT",
		},
	},
	{
		name: mocoagent.BackupUser,
		privileges: []string{
			"BACKUP_ADMIN",
			"EVENT",
			"RELOAD",
			"SELECT",
			"SHOW VIEW",
			"TRIGGER",
			"REPLICATION CLIENT",
			"REPLICATION SLAVE",
			"SERVICE_CONNECTION_ADMIN",
		},
	},
	{
		name: mocoagent.ReadOnlyUser,
		privileges: []string{
			"PROCESS",
			"REPLICATION CLIENT",
			"REPLICATION SLAVE",
			"SELECT",
			"SHOW DATABASES",
			"SHOW VIEW",
		},
	},
	{
		name: mocoagent.WritableUser,
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

// Plugin represents a plugin for mysqld.
type Plugin struct {
	name   string
	soName string
}

var Plugins = []Plugin{
	{
		name:   "rpl_semi_sync_master",
		soName: "semisync_master.so",
	},
	{
		name:   "rpl_semi_sync_slave",
		soName: "semisync_slave.so",
	},
	{
		name:   "clone",
		soName: "mysql_clone.so",
	},
}

func ensureMOCOUsers(ctx context.Context, db *sqlx.DB, reset bool) error {
	_, err := db.ExecContext(ctx, "SET GLOBAL partial_revokes='ON'")
	if err != nil {
		return fmt.Errorf("failed to set global partial_revokes=ON: %w", err)
	}

	passwords := map[string]string{
		mocoagent.AdminUser:       os.Getenv(mocoagent.AdminPasswordEnvKey),
		mocoagent.AgentUser:       os.Getenv(mocoagent.AgentPasswordEnvKey),
		mocoagent.ReplicationUser: os.Getenv(mocoagent.ReplicationPasswordEnvKey),
		mocoagent.CloneDonorUser:  os.Getenv(mocoagent.CloneDonorPasswordEnvKey),
		mocoagent.ExporterUser:    os.Getenv(mocoagent.ExporterPasswordKey),
		mocoagent.BackupUser:      os.Getenv(mocoagent.BackupPasswordKey),
		mocoagent.ReadOnlyUser:    os.Getenv(mocoagent.ReadOnlyPasswordEnvKey),
		mocoagent.WritableUser:    os.Getenv(mocoagent.WritablePasswordEnvKey),
	}

	for k, v := range passwords {
		if v == "" {
			return fmt.Errorf("no password for %s is given via envvar", k)
		}
	}

	for _, u := range Users {
		err := ensureMySQLUser(ctx, db, u, passwords[u.name], reset)
		if err != nil {
			return err
		}
	}

	return nil
}

func dropLocalRootUser(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, "DROP USER IF EXISTS 'root'@'localhost'")
	if err != nil {
		return fmt.Errorf("failed to drop root user: %w", err)
	}
	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	if err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}
	return nil
}

func ensureMySQLUser(ctx context.Context, db *sqlx.DB, user UserSetting, pwd string, reset bool) error {
	var count int
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM mysql.user WHERE user=? and host='%'`, user.name)
	if err != nil {
		return fmt.Errorf("failed to select from mysql.user: %w", err)
	}
	if count == 1 {
		// The user already exists
		if reset {
			_, err := db.ExecContext(ctx, `ALTER USER ?@'%' IDENTIFIED BY ?`, user.name, pwd)
			if err != nil {
				return fmt.Errorf("failed to reset password for %s: %w", user.name, err)
			}
		}
		return nil
	}

	queryStr := `CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED BY ?`
	_, err = db.ExecContext(ctx, queryStr, user.name, pwd)
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", user.name, err)
	}

	queryStr = fmt.Sprintf(`GRANT %s ON *.* TO ?@'%%'`, strings.Join(user.privileges, ","))
	if user.withGrantOption {
		queryStr = queryStr + " WITH GRANT OPTION"
	}
	_, err = db.ExecContext(ctx, queryStr, user.name)
	if err != nil {
		return fmt.Errorf("failed to grant to %s: %w", user.name, err)
	}

	if user.proxyAdmin {
		queryStr = fmt.Sprintf(`GRANT PROXY ON ''@'' TO ?@'%%' WITH GRANT OPTION`)
		_, err = db.ExecContext(ctx, queryStr, user.name)
		if err != nil {
			return fmt.Errorf("failed to grant to %s: %w", user.name, err)
		}
	}

	for target, privileges := range user.revokePrivileges {
		queryStr = fmt.Sprintf(`REVOKE %s ON %s FROM ?@'%%'`, strings.Join(privileges, ","), target)

		_, err = db.ExecContext(ctx, queryStr, user.name)
		if err != nil {
			return fmt.Errorf("failed to revoke from %s: %w", user.name, err)
		}
	}

	return nil
}

func ensureMOCOPlugins(ctx context.Context, db *sqlx.DB) error {
	for _, p := range Plugins {
		err := ensurePlugin(ctx, db, p)
		if err != nil {
			return err
		}
	}

	return nil
}

func ensurePlugin(ctx context.Context, db *sqlx.DB, plugin Plugin) error {
	var installed bool
	err := db.GetContext(ctx, &installed, "SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=? and PLUGIN_STATUS='ACTIVE'", plugin.name)
	if err != nil {
		return fmt.Errorf("failed to select from information_schema.plugins: %w", err)
	}

	if !installed {
		queryStr := fmt.Sprintf(`INSTALL PLUGIN %s SONAME ?`, plugin.name)
		_, err = db.ExecContext(ctx, queryStr, plugin.soName)
		if err != nil {
			return fmt.Errorf("failed to install plugin %s: %w", plugin.name, err)
		}
	}

	return nil
}

func Init(ctx context.Context, db *sqlx.DB, socket string) error {
	if _, err := db.ExecContext(ctx, "SET GLOBAL read_only=OFF"); err != nil {
		return fmt.Errorf("failed to disable read_only: %w", err)
	}
	if err := ensureMOCOUsers(ctx, db, false); err != nil {
		return err
	}
	if err := ensureMOCOPlugins(ctx, db); err != nil {
		return err
	}

	st := time.Now()
	for {
		var err error
		db, err = GetMySQLConnLocalSocket(mocoagent.AdminUser, os.Getenv(mocoagent.AdminPasswordEnvKey), socket)
		if err == nil {
			break
		}

		if time.Since(st) > initTimeout {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	defer db.Close()

	if err := dropLocalRootUser(ctx, db); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, "RESET MASTER"); err != nil {
		return fmt.Errorf("failed to reset master: %w", err)
	}
	if _, err := db.ExecContext(ctx, "SET GLOBAL super_read_only=ON"); err != nil {
		return fmt.Errorf("failed to enable super_read_only: %w", err)
	}
	return nil
}

func InitExternal(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, "SET sql_log_bin=OFF"); err != nil {
		return fmt.Errorf("failed to disable binary logging: %w", err)
	}
	if _, err := db.ExecContext(ctx, "SET GLOBAL read_only=OFF"); err != nil {
		return fmt.Errorf("failed to disable read_only: %w", err)
	}
	if err := ensureMOCOUsers(ctx, db, true); err != nil {
		return err
	}
	if err := ensureMOCOPlugins(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "SET GLOBAL super_read_only=ON"); err != nil {
		return fmt.Errorf("failed to enable super_read_only: %w", err)
	}
	if _, err := db.ExecContext(ctx, "SET sql_log_bin=ON"); err != nil {
		return fmt.Errorf("failed to enable binary logging: %w", err)
	}
	return nil
}
