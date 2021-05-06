package server

import (
	"path/filepath"

	mocoagent "github.com/cybozu-go/moco-agent"
	. "github.com/onsi/ginkgo"
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
