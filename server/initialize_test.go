package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// legacySemiSyncAvailable reports whether the test MySQL has the legacy
// semi-sync plugin shared objects (semisync_master.so / semisync_slave.so).
// They were removed in MySQL 8.4, so we can only exercise the legacy-to-new
// migration on 8.0.x containers.
func legacySemiSyncAvailable() bool {
	parts := strings.SplitN(MySQLVersion, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major != 8 {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return minor < 4
}

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

	It("should migrate legacy plugins to new ones", func() {
		if !legacySemiSyncAvailable() {
			Skip("legacy semi-sync plugins are not available on MySQL " + MySQLVersion)
		}

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
})

func execAll(ctx context.Context, db *sqlx.DB, queries ...string) error {
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("failed to run %q: %w", q, err)
		}
	}
	return nil
}
