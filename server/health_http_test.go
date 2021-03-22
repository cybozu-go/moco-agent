package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco-agent/test_utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testHealthHTTP() {
	var agent *Agent
	var registry *prometheus.Registry

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)

		agent = New(test_utils.Host, clusterName, test_utils.AgentUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, "", "", replicaPort,
			MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)
	})

	It("should return 200 if the agent can execute a query", func() {
		err := test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health")
		res := getHealth(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		test_utils.StopAndRemoveMySQLD(replicaHost)
	})

	It("should return 503 if the agent cannot connect the own mysqld", func() {
		By("getting health")
		res := getHealth(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusServiceUnavailable))
	})
}

func testReadyHTTP() {
	var agent *Agent
	var registry *prometheus.Registry

	BeforeEach(func() {
		err := test_utils.StartMySQLD(donorHost, donorPort, donorServerID)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID)
		Expect(err).ShouldNot(HaveOccurred())

		err = test_utils.InitializeMySQL(donorPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		err = test_utils.PrepareTestData(donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		err = test_utils.ResetMaster(donorPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)

		agent = New(test_utils.Host, clusterName, test_utils.AgentUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, "", "", replicaPort,
			MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)
	})

	AfterEach(func() {
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)
	})

	It("should return 200 if not under cloning and read_only=false", func() {
		By("getting readiness (should be 200)")
		res := getReady(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))
	})

	It("should return 503 if cloning process is in progress", func() {
		By("executing cloning")
		err := test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())
		gsrv := NewCloneService(agent)
		req := &agentrpc.CloneRequest{
			DonorHost: donorHost,
			DonorPort: donorPort,
		}
		_, err = gsrv.Clone(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting readiness (should be 503)")
		res := getReady(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusServiceUnavailable))

		By("wating cloning is completed")
		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("clone process is still working")
		}).Should(Succeed())
	})

	FIt("should return appropriate status code when working as replica instance", func() {
		By("setting read_only=true, but not works as replica")
		test_utils.SetReadonly(replicaPort)

		By("getting readiness (should be 503)")
		res := getReady(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusServiceUnavailable))

		By("executing START SLAVE with invalid parameters")
		err := test_utils.StartSlaveWithInvalidSettings(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting readiness (should be 503)")
		Eventually(func() error {
			res := getReady(agent)
			if res.Result().StatusCode != http.StatusServiceUnavailable {
				return fmt.Errorf("doesn't return 503: %+v", res.Result().Status)
			}
			return nil
		}).Should(Succeed())

		By("executing START SLAVE with valid parameters")
		err = test_utils.StopAndResetSlave(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.StartSlaveWithValidSettings(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting readiness (should be 200)")
		Eventually(func() error {
			res := getReady(agent)
			if res.Result().StatusCode != http.StatusOK {
				return fmt.Errorf("doesn't return 200: %+v", res.Result().Status)
			}
			return nil
		}).Should(Succeed())

		By("making delay between the original commit and the apply end time apply at the replica")
		err = test_utils.ExecSQLCommand(replicaPort, "STOP SLAVE SQL_THREAD")
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.ExecSQLCommand(donorPort, "CREATE DATABASE health_test_db")
		Expect(err).ShouldNot(HaveOccurred())
		time.Sleep(maxDelayThreshold + time.Second)
		err = test_utils.ExecSQLCommand(replicaPort, "START SLAVE SQL_THREAD")
		Expect(err).ShouldNot(HaveOccurred())

		By("getting readiness (should be 503)")
		Eventually(func() error {
			res := getReady(agent)
			if res.Result().StatusCode != http.StatusServiceUnavailable {
				return fmt.Errorf("doesn't return 503: %+v", res.Result().Status)
			}
			return nil
		}).Should(Succeed())
	})
}

func getHealth(agent *Agent) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "http://"+replicaHost+"/healthz", nil)
	res := httptest.NewRecorder()
	agent.Health(res, req)
	return res
}

func getReady(agent *Agent) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "http://"+replicaHost+"/readyz", nil)
	res := httptest.NewRecorder()
	agent.Ready(res, req)
	return res
}
