package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testHealthHTTP() {
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

	It("should return 200 if no errors or cloning is not in progress", func() {
		By("getting health")
		res := getHealth(agent)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))
	})

	It("should return 500 if cloning process is in progress", func() {
		By("executing cloning")
		err := test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health")
		Eventually(func() error {
			res := getHealth(agent)
			if res.Result().StatusCode != http.StatusInternalServerError {
				return fmt.Errorf("doesn't occur internal server error: %+v", res.Result().Status)
			}
			return nil
		}, 5*time.Second).Should(Succeed())

		By("wating cloning is completed")
		Eventually(func() error {
			db, err := agent.getMySQLConn()
			if err != nil {
				return err
			}

			cloneStatus, err := GetMySQLCloneStateStatus(context.Background(), db)
			if err != nil {
				return err
			}

			expected := sql.NullString{Valid: true, String: "Completed"}
			if !cmp.Equal(cloneStatus.State, expected) {
				return fmt.Errorf("doesn't reach completed state: %+v", cloneStatus.State)
			}
			return nil
		}, 30*time.Second).Should(Succeed())
	})

	It("should return 500 if replica status has IO error", func() {
		By("executing START SLAVE with invalid parameters")
		err := test_utils.StartSlaveWithInvalidSettings(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health")
		Eventually(func() error {
			res := getHealth(agent)
			if res.Result().StatusCode != http.StatusInternalServerError {
				return fmt.Errorf("doesn't occur internal server error: %+v", res.Result().Status)
			}
			return nil
		}, 10*time.Second).Should(Succeed())
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
