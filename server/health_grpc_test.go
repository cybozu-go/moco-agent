package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/proto"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/cybozu-go/moco/accessor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testHealthgRPC() {
	var agent *Agent
	var registry *prometheus.Registry
	var gsrv proto.HealthServiceServer

	BeforeEach(func() {
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)

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

		agent = New(test_utils.Host, clusterName, token, test_utils.MiscUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, "", replicaPort,
			&accessor.MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)

		gsrv = NewHealthService(agent)
	})

	AfterEach(func() {
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)
	})

	It("should return OK=true if no errors and cloning is not in progress", func() {
		By("getting health")
		res, err := gsrv.Health(context.Background(), &proto.HealthRequest{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.Ok).Should(BeTrue())
	})

	It("should return IsUnderCloning=true if cloning process is in progress", func() {
		By("executing cloning")
		err := test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		req := httptest.NewRequest("GET", "http://"+replicaHost+"/clone", nil)
		queries := url.Values{
			moco.CloneParamDonorHostName: []string{donorHost},
			moco.CloneParamDonorPort:     []string{strconv.Itoa(donorPort)},
			moco.AgentTokenParam:         []string{token},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.Clone(res, req)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		By("getting health expecting IsUnderCloning=true")
		Eventually(func() error {
			res, err := gsrv.Health(context.Background(), &proto.HealthRequest{})
			if err == nil && !res.Ok && res.IsUnderCloning {
				return nil
			}
			return fmt.Errorf("should become IsUnderCloning=true: res=%s", res.String())
		}, 5*time.Second, 200*time.Millisecond).Should(Succeed())

		By("wating cloning is completed")
		Eventually(func() error {
			res, err := gsrv.Health(context.Background(), &proto.HealthRequest{})
			if err == nil && res.Ok {
				return nil
			}
			return fmt.Errorf("should become Ok=true: res=%s", res.String())
		}, 30*time.Second, time.Second).Should(Succeed())
	})

	It("should return IsOutOfSynced=true if replica status has IO error", func() {
		By("executing START SLAVE with invalid parameters")
		err := test_utils.StartSlaveWithInvalidSettings(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health expecting IsOutOfSync=true")
		Eventually(func() error {
			res, err := gsrv.Health(context.Background(), &proto.HealthRequest{})
			if err == nil && !res.Ok && res.IsOutOfSynced {
				return nil
			}
			return fmt.Errorf("should become IsOutOfSynced=true: res=%s", res.String())
		}, 5*time.Second, 200*time.Millisecond).Should(Succeed())
	})
}
