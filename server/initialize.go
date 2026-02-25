package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/go-logr/logr"
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
		name:   "rpl_semi_sync_source",
		soName: "semisync_source.so",
	},
	{
		name:   "rpl_semi_sync_replica",
		soName: "semisync_replica.so",
	},
	{
		name:   "clone",
		soName: "mysql_clone.so",
	},
}

// legacySemiSyncPlugins maps new plugin names to their legacy counterparts.
// Used during migration to detect and replace old plugins.
var legacySemiSyncPlugins = map[string]string{
	"rpl_semi_sync_source":  "rpl_semi_sync_master",
	"rpl_semi_sync_replica": "rpl_semi_sync_slave",
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
		queryStr = `GRANT PROXY ON ''@'' TO ?@'%' WITH GRANT OPTION`
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
	logger := logr.Discard()
	for _, p := range Plugins {
		if err := ensurePlugin(ctx, db, p, logger); err != nil {
			return err
		}
	}
	return nil
}

// MigrateSemiSyncPlugins replaces legacy semi-sync plugins (master/slave)
// with the new ones (source/replica) on already-initialized instances.
func MigrateSemiSyncPlugins(ctx context.Context, db *sqlx.DB, logger logr.Logger) error {
	// Check if any legacy plugins need migration
	var needsMigration bool
	for _, p := range Plugins {
		oldName, ok := legacySemiSyncPlugins[p.name]
		if !ok {
			continue
		}
		oldStatus, err := getPluginStatus(ctx, db, oldName)
		if err != nil {
			return err
		}
		if oldStatus != "" {
			needsMigration = true
			break
		}
	}
	if !needsMigration {
		return nil
	}

	// Disable binlog and super_read_only only for actual plugin changes
	if _, err := db.ExecContext(ctx, "SET sql_log_bin=OFF"); err != nil {
		return fmt.Errorf("failed to disable sql_log_bin: %w", err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, "SET sql_log_bin=ON"); err != nil {
			logger.Error(err, "failed to re-enable sql_log_bin")
		}
	}()

	var readOnly int
	if err := db.GetContext(ctx, &readOnly, "SELECT @@global.super_read_only"); err != nil {
		return fmt.Errorf("failed to get super_read_only: %w", err)
	}
	if readOnly == 1 {
		if _, err := db.ExecContext(ctx, "SET GLOBAL super_read_only=OFF"); err != nil {
			return fmt.Errorf("failed to disable super_read_only: %w", err)
		}
		defer func() {
			if _, err := db.ExecContext(ctx, "SET GLOBAL super_read_only=ON"); err != nil {
				logger.Error(err, "failed to re-enable super_read_only")
			}
		}()
	}

	for _, p := range Plugins {
		if _, ok := legacySemiSyncPlugins[p.name]; !ok {
			continue
		}
		if err := ensurePlugin(ctx, db, p, logger); err != nil {
			return err
		}
	}
	return nil
}

func ensurePlugin(ctx context.Context, db *sqlx.DB, plugin Plugin, logger logr.Logger) error {
	// Check if the plugin is already installed and active
	status, err := getPluginStatus(ctx, db, plugin.name)
	if err != nil {
		return err
	}
	if status == "ACTIVE" {
		return nil
	}

	// For semi-sync plugins, check if the legacy version is installed and migrate
	if oldName, ok := legacySemiSyncPlugins[plugin.name]; ok {
		oldStatus, err := getPluginStatus(ctx, db, oldName)
		if err != nil {
			return err
		}
		if oldStatus != "" {
			logger.Info("migrating semi-sync plugin", "from", oldName, "to", plugin.name, "oldStatus", oldStatus)
			if _, err := db.ExecContext(ctx, fmt.Sprintf("UNINSTALL PLUGIN %s", oldName)); err != nil {
				return fmt.Errorf("failed to uninstall legacy plugin %s: %w", oldName, err)
			}
		}
	}

	// Install the plugin
	queryStr := fmt.Sprintf(`INSTALL PLUGIN %s SONAME ?`, plugin.name)
	if _, err := db.ExecContext(ctx, queryStr, plugin.soName); err != nil {
		return fmt.Errorf("failed to install plugin %s: %w", plugin.name, err)
	}
	logger.Info("installed plugin", "name", plugin.name)
	return nil
}

// getPluginStatus returns the PLUGIN_STATUS for the given plugin name.
// Returns "" if the plugin is not installed, or the status string ("ACTIVE", "INACTIVE", "DISABLED", etc.).
func getPluginStatus(ctx context.Context, db *sqlx.DB, name string) (string, error) {
	var status string
	err := db.GetContext(ctx, &status,
		"SELECT PLUGIN_STATUS FROM information_schema.plugins WHERE PLUGIN_NAME=?", name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to check plugin %s: %w", name, err)
	}
	return status, nil
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

	var version string
	err := db.GetContext(ctx, &version, `SELECT SUBSTRING_INDEX(VERSION(), '.', 2)`)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}
	if version == "8.4" {
		if _, err := db.ExecContext(ctx, "RESET BINARY LOGS AND GTIDS"); err != nil {
			return fmt.Errorf("failed to reset binary logs and gtids: %w", err)
		}

	} else {
		if _, err := db.ExecContext(ctx, "RESET MASTER"); err != nil {
			return fmt.Errorf("failed to reset master: %w", err)
		}
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
