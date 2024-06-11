package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("health", func() {
	It("should reply for probes as Primary", func() {
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

		By("getting health for running Primary")
		res := getHealth(agent)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		By("getting readiness for read-only Primary")
		res = getReady(agent)
		Expect(res).NotTo(HaveHTTPStatus(http.StatusOK))

		By("getting readiness for working Primary")
		_, err = db.Exec("SET GLOBAL read_only=0")
		Expect(err).NotTo(HaveOccurred())
		res = getReady(agent)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		By("getting health for stopped Primary")
		StopAndRemoveMySQLD(donorHost)
		res = getHealth(agent)
		Expect(res).NotTo(HaveHTTPStatus(http.StatusOK))

		By("getting readiness for stopped Primary")
		res = getReady(agent)
		Expect(res).NotTo(HaveHTTPStatus(http.StatusOK))
	})

	It("should reply for probes as Replica", func() {
		By("starting primary/replica MySQLds")
		StartMySQLD(donorHost, donorPort, donorServerID)
		defer StopAndRemoveMySQLD(donorHost)

		sockFile := filepath.Join(socketDir(donorHost), "mysqld.sock")

		donorDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer donorDB.Close()

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
		agent, err := New(conf, testClusterName, sockFile, "", 100*time.Millisecond, time.Second, testLogger)
		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		replicaDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer replicaDB.Close()

		By("checking readiness before starting replication")
		res := getReady(agent)
		Expect(res).NotTo(HaveHTTPStatus(http.StatusOK))

		By("setting up donor")
		_, err = replicaDB.Exec("SET GLOBAL clone_valid_donor_list = ?", donorHost+":3306")
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("SET GLOBAL read_only=0")
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("CREATE DATABASE foo")
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("CREATE TABLE foo.bar (i INT PRIMARY KEY) ENGINE=InnoDB")
		Expect(err).NotTo(HaveOccurred())
		items := []interface{}{100, 299, 993, 9292}
		_, err = donorDB.Exec("INSERT INTO foo.bar (i) VALUES (?), (?), (?), (?)", items...)
		Expect(err).NotTo(HaveOccurred())
		
		if strings.hasPrefix(MYSQL_VERSION, "8.4") {
			_, err = replicaDB.Exec(`CHANGE REPLICATION SOURCE TO SOURCE_HOST=?, SOURCE_PORT=3306, SOURCE_USER=?, SOURCE_PASSWORD=?, GET_SOURCE_PUBLIC_KEY=1`,
				donorHost, mocoagent.ReplicationUser, replicationUserPassword)
			Expect(err).NotTo(HaveOccurred())
		} else {
			_, err = replicaDB.Exec(`CHANGE MASTER TO MASTER_HOST=?, MASTER_PORT=3306, MASTER_USER=?, MASTER_PASSWORD=?, GET_MASTER_PUBLIC_KEY=1`,
				donorHost, mocoagent.ReplicationUser, replicationUserPassword)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = replicaDB.Exec(`START REPLICA`)
		Expect(err).NotTo(HaveOccurred())

		By("checking readiness")
		Eventually(func() interface{} {
			return getReady(agent)
		}).Should(HaveHTTPStatus(http.StatusOK))

		By("locking replica")
		_, err = replicaDB.Exec(`LOCK INSTANCE FOR BACKUP`)
		Expect(err).NotTo(HaveOccurred())

		By("checking lag")
		time.Sleep(200 * time.Millisecond)
		_, err = donorDB.Exec("ALTER TABLE foo.bar ADD COLUMN hoge TEXT")
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() interface{} {
			return getReady(agent)
		}).Should(HaveHTTPStatus(http.StatusServiceUnavailable))

		By("unlocking replica")
		_, err = replicaDB.Exec(`UNLOCK INSTANCE`)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() interface{} {
			return getReady(agent)
		}).Should(HaveHTTPStatus(http.StatusOK))
	})

	It("checks transactionQueueingWait works", func() {
		By("starting primary/replica MySQLds")
		StartMySQLD(donorHost, donorPort, donorServerID)
		defer StopAndRemoveMySQLD(donorHost)

		sockFile := filepath.Join(socketDir(donorHost), "mysqld.sock")

		donorDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer donorDB.Close()

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
		agent, err := New(conf, testClusterName, sockFile, "", 100*time.Millisecond, time.Second*60, testLogger)
		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		replicaDB, err := GetMySQLConnLocalSocket(mocoagent.AdminUser, adminUserPassword, sockFile)
		Expect(err).NotTo(HaveOccurred())
		defer replicaDB.Close()

		By("setting up donor")
		_, err = replicaDB.Exec("SET GLOBAL clone_valid_donor_list = ?", donorHost+":3306")
		Expect(err).NotTo(HaveOccurred())
		_, err = donorDB.Exec("SET GLOBAL read_only=0")
		Expect(err).NotTo(HaveOccurred())

		if strings.hasPrefix(MYSQL_VERSION, "8.4") {
			_, err = replicaDB.Exec(`CHANGE REPLICATION SOURCE TO SOURCE_HOST=?, SOURCE_PORT=3306, SOURCE_USER=?, SOURCE_PASSWORD=?, GET_SOURCE_PUBLIC_KEY=1`,
				donorHost, mocoagent.ReplicationUser, replicationUserPassword)
			Expect(err).NotTo(HaveOccurred())
		} else {
			_, err = replicaDB.Exec(`CHANGE MASTER TO MASTER_HOST=?, MASTER_PORT=3306, MASTER_USER=?, MASTER_PASSWORD=?, GET_MASTER_PUBLIC_KEY=1`,
				donorHost, mocoagent.ReplicationUser, replicationUserPassword)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = replicaDB.Exec(`START REPLICA`)
		Expect(err).NotTo(HaveOccurred())

		By("checking readiness")
		// The uptime observed by the agent is about 15s smaller than the process uptime reported by kernel
		// The test flow takes about 35s from the process start to this point.
		// We set Consistently timeout to 60(transactionQueueingWait) - (35 - 15) - 5(margin) = 35
		Consistently(func() interface{} {
			return getReady(agent)
		}).WithPolling(time.Second).WithTimeout(time.Second * 35).ShouldNot(HaveHTTPStatus(http.StatusOK))
		Eventually(func() interface{} {
			return getReady(agent)
		}).WithPolling(time.Second).WithTimeout(time.Second * 10).Should(HaveHTTPStatus(http.StatusOK))
	})
})

func getHealth(agent *Agent) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "http://"+replicaHost+"/healthz", nil)
	res := httptest.NewRecorder()
	agent.MySQLDHealth(res, req)
	return res
}

func getReady(agent *Agent) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "http://"+replicaHost+"/readyz", nil)
	res := httptest.NewRecorder()
	agent.MySQLDReady(res, req)
	return res
}
