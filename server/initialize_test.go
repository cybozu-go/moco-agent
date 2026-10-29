package server

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("initialize", func() {
	It("should create users", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")

		By("connecting with created users")
		for _, u := range Users {
			var pwd string
			switch u.name {
			case mocoagent.AdminUser:
				pwd = adminUserPassword
			case mocoagent.AgentUser:
				pwd = agentUserPassword
			case mocoagent.ReplicationUser:
				pwd = replicationUserPassword
			case mocoagent.CloneDonorUser:
				pwd = cloneDonorUserPassword
			case mocoagent.ExporterUser:
				pwd = exporterPassword
			case mocoagent.BackupUser:
				pwd = backupPassword
			case mocoagent.ReadOnlyUser:
				pwd = readOnlyPassword
			case mocoagent.WritableUser:
				pwd = writablePassword
			}
			db, err := GetMySQLConnLocalSocket(u.name, pwd, sockFile)
			Expect(err).NotTo(HaveOccurred(), "user %s cannot connect", u.name)
			err = db.Close()
			Expect(err).NotTo(HaveOccurred())
		}

		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())

		By("checking if super_read_only is 1")
		var superReadOnly bool
		err = db.Get(&superReadOnly, `SELECT @@super_read_only`)
		Expect(err).NotTo(HaveOccurred())
		Expect(superReadOnly).To(BeTrue())

		By("checking if executed gtid set is empty")
		var executedGTIDSet string
		err = db.Get(&executedGTIDSet, `SELECT @@gtid_executed`)
		Expect(err).NotTo(HaveOccurred())
		Expect(executedGTIDSet).To(BeEmpty())

		By("checking active plugins in information_schema")
		for _, p := range Plugins {
			var installed bool
			err := db.Get(&installed, "SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=? and PLUGIN_STATUS='ACTIVE'", p.name)
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue(), "plugin %s not found", p.name)
		}

		By("connecting with dropped root user")
		_, err = GetMySQLConnLocalSocket("root", "", sockFile)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("MigrateSemiSyncPlugins", func() {
	It("should be a no-op when only new plugins are installed", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")
		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		ctx := context.Background()

		By("verifying super_read_only is ON before migration")
		var superReadOnly bool
		Expect(db.GetContext(ctx, &superReadOnly, `SELECT @@global.super_read_only`)).To(Succeed())
		Expect(superReadOnly).To(BeTrue())

		By("running migration (expected no-op)")
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())

		By("verifying the new plugins are still ACTIVE")
		for _, name := range []string{"rpl_semi_sync_source", "rpl_semi_sync_replica"} {
			var status string
			Expect(db.GetContext(ctx, &status,
				`SELECT PLUGIN_STATUS FROM information_schema.plugins WHERE PLUGIN_NAME=?`, name,
			)).To(Succeed())
			Expect(status).To(Equal("ACTIVE"), "plugin %s should remain ACTIVE", name)
		}

		By("verifying super_read_only remains ON")
		Expect(db.GetContext(ctx, &superReadOnly, `SELECT @@global.super_read_only`)).To(Succeed())
		Expect(superReadOnly).To(BeTrue())
	})

	It("should install a new plugin missing after an interrupted migration", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")
		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		ctx := context.Background()

		By("removing the new replica plugin with no legacy plugin left")
		Expect(execAll(ctx, db,
			`SET sql_log_bin=OFF`,
			`SET GLOBAL super_read_only=OFF`,
			`UNINSTALL PLUGIN rpl_semi_sync_replica`,
			`SET GLOBAL super_read_only=ON`,
			`SET sql_log_bin=ON`,
		)).To(Succeed())

		By("running migration")
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())

		var status string
		Expect(db.GetContext(ctx, &status,
			`SELECT PLUGIN_STATUS FROM information_schema.plugins WHERE PLUGIN_NAME='rpl_semi_sync_replica'`,
		)).To(Succeed())
		Expect(status).To(Equal("ACTIVE"))
	})

	It("should migrate legacy plugins to new ones", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")
		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		ctx := context.Background()

		By("downgrading to legacy semi-sync plugins to simulate an instance initialized before the rename")
		// Init() has already installed the new plugins; remove them and install
		// the legacy ones in their place. sql_log_bin/super_read_only are
		// toggled in the same shape MigrateSemiSyncPlugins would handle.
		Expect(execAll(ctx, db,
			`SET sql_log_bin=OFF`,
			`SET GLOBAL super_read_only=OFF`,
			`UNINSTALL PLUGIN rpl_semi_sync_replica`,
			`UNINSTALL PLUGIN rpl_semi_sync_source`,
			`INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so'`,
			`INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so'`,
			`SET GLOBAL super_read_only=ON`,
			`SET sql_log_bin=ON`,
		)).To(Succeed())

		By("skipping migration while semi-sync is enabled")
		_, err = db.ExecContext(ctx, `SET GLOBAL rpl_semi_sync_master_enabled=ON`)
		Expect(err).NotTo(HaveOccurred())
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())
		var status string
		Expect(db.GetContext(ctx, &status,
			`SELECT PLUGIN_STATUS FROM information_schema.plugins WHERE PLUGIN_NAME='rpl_semi_sync_master'`,
		)).To(Succeed())
		Expect(status).To(Equal("ACTIVE"))
		_, err = db.ExecContext(ctx, `SET GLOBAL rpl_semi_sync_master_enabled=OFF`)
		Expect(err).NotTo(HaveOccurred())

		By("running migration")
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())

		By("verifying new plugins are ACTIVE")
		for _, name := range []string{"rpl_semi_sync_source", "rpl_semi_sync_replica"} {
			var status string
			Expect(db.GetContext(ctx, &status,
				`SELECT PLUGIN_STATUS FROM information_schema.plugins WHERE PLUGIN_NAME=?`, name,
			)).To(Succeed())
			Expect(status).To(Equal("ACTIVE"), "plugin %s should be ACTIVE after migration", name)
		}

		By("verifying legacy plugin rows are gone")
		for _, name := range []string{"rpl_semi_sync_master", "rpl_semi_sync_slave"} {
			var count int
			Expect(db.GetContext(ctx, &count,
				`SELECT COUNT(*) FROM information_schema.plugins WHERE PLUGIN_NAME=?`, name,
			)).To(Succeed())
			Expect(count).To(Equal(0), "legacy plugin %s should be uninstalled", name)
		}

		By("verifying super_read_only was restored to ON")
		var superReadOnly bool
		Expect(db.GetContext(ctx, &superReadOnly, `SELECT @@global.super_read_only`)).To(Succeed())
		Expect(superReadOnly).To(BeTrue())

		By("running migration again is a no-op")
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())
	})

	It("should carry over semi-sync variables set after the enabled check", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")
		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		ctx := context.Background()

		By("configuring the legacy plugins as moco-controller does")
		Expect(execAll(ctx, db,
			`SET sql_log_bin=OFF`,
			`SET GLOBAL super_read_only=OFF`,
			`UNINSTALL PLUGIN rpl_semi_sync_replica`,
			`UNINSTALL PLUGIN rpl_semi_sync_source`,
			`INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so'`,
			`INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so'`,
			`SET GLOBAL rpl_semi_sync_master_timeout=12345`,
			`SET GLOBAL rpl_semi_sync_master_wait_for_slave_count=2`,
			`SET GLOBAL rpl_semi_sync_master_enabled=ON`,
			`SET GLOBAL rpl_semi_sync_slave_enabled=ON`,
			`SET sql_log_bin=ON`,
		)).To(Succeed())

		By("replacing the plugins")
		for _, p := range Plugins {
			Expect(ensurePlugin(ctx, db, p, testLogger)).To(Succeed())
		}
		_, err = db.ExecContext(ctx, `SET GLOBAL super_read_only=ON`)
		Expect(err).NotTo(HaveOccurred())

		By("verifying the values are carried over")
		var vars struct {
			Timeout        int  `db:"timeout"`
			WaitCount      int  `db:"wait_count"`
			SourceEnabled  bool `db:"source_enabled"`
			ReplicaEnabled bool `db:"replica_enabled"`
		}
		Expect(db.GetContext(ctx, &vars,
			`SELECT @@rpl_semi_sync_source_timeout AS timeout,
			 @@rpl_semi_sync_source_wait_for_replica_count AS wait_count,
			 @@rpl_semi_sync_source_enabled AS source_enabled,
			 @@rpl_semi_sync_replica_enabled AS replica_enabled`,
		)).To(Succeed())
		Expect(vars.Timeout).To(Equal(12345))
		Expect(vars.WaitCount).To(Equal(2))
		Expect(vars.SourceEnabled).To(BeTrue())
		Expect(vars.ReplicaEnabled).To(BeTrue())
	})

	It("should make GetMySQLGlobalVariable follow the installed plugins", func() {
		By("starting MySQLd")
		StartMySQLD(donorHost, donorPort, donorServerID)
		defer StopAndRemoveMySQLD(donorHost)

		sockFile := filepath.Join(socketDir(donorHost), "mysqld.sock")
		conf := MySQLAccessorConfig{
			Host:              "localhost",
			Port:              donorPort,
			Password:          agentUserPassword,
			ConnMaxIdleTime:   30 * time.Minute,
			ConnectionTimeout: 3 * time.Second,
			ReadTimeout:       30 * time.Second,
		}
		agent, err := New(conf, testClusterName, sockFile, "", maxDelayThreshold, time.Second, testLogger)
		Expect(err).NotTo(HaveOccurred())
		defer agent.CloseDB()

		db, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		ctx := context.Background()

		By("reading the wait count with the source/replica plugins")
		_, err = db.ExecContext(ctx, `SET GLOBAL rpl_semi_sync_source_wait_for_replica_count=3`)
		Expect(err).NotTo(HaveOccurred())
		gv, err := agent.GetMySQLGlobalVariable(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(gv.RplSemiSyncWaitForReplicaCount).To(Equal(3))

		By("reading the wait count after switching to the master/slave plugins")
		Expect(execAll(ctx, db,
			`SET sql_log_bin=OFF`,
			`SET GLOBAL super_read_only=OFF`,
			`UNINSTALL PLUGIN rpl_semi_sync_replica`,
			`UNINSTALL PLUGIN rpl_semi_sync_source`,
			`INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so'`,
			`INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so'`,
			`SET GLOBAL rpl_semi_sync_master_wait_for_slave_count=2`,
			`SET GLOBAL super_read_only=ON`,
			`SET sql_log_bin=ON`,
		)).To(Succeed())
		gv, err = agent.GetMySQLGlobalVariable(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(gv.RplSemiSyncWaitForReplicaCount).To(Equal(2))

		By("reading the wait count after migrating back to the source/replica plugins")
		Expect(MigrateSemiSyncPlugins(ctx, db, testLogger)).To(Succeed())
		gv, err = agent.GetMySQLGlobalVariable(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(gv.RplSemiSyncWaitForReplicaCount).To(Equal(1), "variables are not carried over while semi-sync is disabled")
		_, err = db.ExecContext(ctx, `SET GLOBAL rpl_semi_sync_source_wait_for_replica_count=4`)
		Expect(err).NotTo(HaveOccurred())
		gv, err = agent.GetMySQLGlobalVariable(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(gv.RplSemiSyncWaitForReplicaCount).To(Equal(4))
	})
})

// execAll runs each query through a single pinned *sqlx.Conn so that
// session-scoped settings (e.g. SET sql_log_bin=OFF) reliably apply to
// the subsequent statements. Without pinning, *sqlx.DB may hop across
// pooled connections and silently lose those settings.
func execAll(ctx context.Context, db *sqlx.DB, queries ...string) error {
	conn, err := db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Close()
	for _, q := range queries {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to run %q: %w", q, err)
		}
	}
	return nil
}
