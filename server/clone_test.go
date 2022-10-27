package server

import (
	"context"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/proto"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	// Dummy user and password for clone from external.
	externalDonorUser     = "external-donor-user"
	externalDonorPassword = "externaldonorpassword"
	externalInitUser      = "external-init-user"
	externalInitPassword  = "externalinitpassword"
)

var _ = Describe("clone", func() {
	It("should successfully complete cloning", func() {
		By("setting up the donor instance")
		StartMySQLD(donorHost, donorPort, donorServerID)
		defer StopAndRemoveMySQLD(donorHost)

		sockFile := filepath.Join(socketDir(donorHost), "mysqld.sock")

		donorDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer donorDB.Close()

		_, err = donorDB.Exec(`SET GLOBAL read_only=0`)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`CREATE DATABASE foo`)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`CREATE TABLE foo.bar (i INT PRIMARY KEY) ENGINE=InnoDB`)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("INSERT INTO foo.bar (i) VALUES (100), (101), (102), (103)")
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`RESET MASTER`)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("INSERT INTO foo.bar (i) VALUES (200), (800), (10000), (-3)")
		Expect(err).NotTo(HaveOccurred())

		_, err = donorDB.Exec(`CREATE USER ?@'%' IDENTIFIED BY ?`, externalDonorUser, externalDonorPassword)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`CREATE USER ?@'localhost' IDENTIFIED BY ?`, externalInitUser, externalInitPassword)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`DROP USER ?@'%'`, mocoagent.ReadOnlyUser)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`GRANT BACKUP_ADMIN, REPLICATION SLAVE ON *.* TO ?@'%'`, externalDonorUser)
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec(`GRANT ALL ON *.* TO ?@'localhost' WITH GRANT OPTION`, externalInitUser)
		Expect(err).NotTo(HaveOccurred())

		By("preparing an empty replica instance")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile = filepath.Join(socketDir(replicaHost), "mysqld.sock")
		conf := MySQLAccessorConfig{
			Host:              "localhost",
			Port:              replicaPort,
			Password:          agentUserPassword,
			ConnMaxIdleTime:   30 * time.Minute,
			ConnectionTimeout: 3 * time.Second,
			ReadTimeout:       30 * time.Second,
		}
		agent, err := New(conf, testClusterName, sockFile, "", 100*time.Millisecond, testLogger)

		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		replicaDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer replicaDB.Close()

		By("executing CLONE INSTANCE")
		err = agent.Clone(context.Background(), &proto.CloneRequest{
			Host:         donorHost,
			Port:         3306,
			User:         externalDonorUser,
			Password:     externalDonorPassword,
			InitUser:     externalInitUser,
			InitPassword: externalInitPassword,
			BootTimeout:  durationpb.New(2 * time.Minute),
		})
		Expect(err).NotTo(HaveOccurred())

		By("checking the cloned data")
		var count int
		err = replicaDB.Get(&count, `SELECT COUNT(*) FROM foo.bar`)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(8))

		By("starting replication")
		_, err = donorDB.Exec(`INSERT INTO foo.bar (i) VALUES (9), (999)`)
		Expect(err).NotTo(HaveOccurred())
		_, err = replicaDB.Exec(`CHANGE MASTER TO MASTER_HOST=?, MASTER_PORT=3306, MASTER_USER=?, MASTER_PASSWORD=?, GET_MASTER_PUBLIC_KEY=1`,
			donorHost, mocoagent.ReplicationUser, replicationUserPassword)
		Expect(err).NotTo(HaveOccurred())
		_, err = replicaDB.Exec(`START SLAVE`)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			var count int
			replicaDB.Get(&count, `SELECT COUNT(*) FROM foo.bar`)
			return count
		}).Should(Equal(10))

		By("checking if errant transactions exist")
		rs, err := agent.GetMySQLReplicaStatus(context.Background())
		Expect(err).NotTo(HaveOccurred())
		var isSubset bool
		err = replicaDB.Get(&isSubset, `SELECT GTID_SUBSET(?, ?)`, rs.ExecutedGtidSet, rs.RetrievedGtidSet)
		Expect(err).NotTo(HaveOccurred())
		Expect(isSubset).To(BeTrue())
	})
})
