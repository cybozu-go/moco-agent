package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco-agent/test_utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func testHealth() {
	var agent *Agent
	var registry *prometheus.Registry
	var gsrv healthpb.HealthServer
	var cloneSrv agentrpc.CloneServiceServer

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

		agent = New(test_utils.Host, clusterName, token, test_utils.AgentUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, "", "", replicaPort,
			MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)

		gsrv = NewHealthService(agent)
		cloneSrv = NewCloneService(agent)
	})

	AfterEach(func() {
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)
	})

	It("should return OK=true if no errors and cloning is not in progress", func() {
		By("getting health")
		res, err := gsrv.Check(context.Background(), &healthpb.HealthCheckRequest{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.Status).Should(Equal(healthpb.HealthCheckResponse_SERVING))
	})

	It("should return IsUnderCloning=true if cloning process is in progress", func() {
		By("executing cloning")
		err := test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())

		req := &agentrpc.CloneRequest{
			Token:     token,
			DonorHost: donorHost,
			DonorPort: donorPort,
		}
		_, err = cloneSrv.Clone(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health expecting IsUnderCloning=true")
		Eventually(func() error {
			res, err := gsrv.Check(context.Background(), &healthpb.HealthCheckRequest{})
			if res.Status == healthpb.HealthCheckResponse_NOT_SERVING && strings.HasSuffix(err.Error(), "hasIOThreadError=false, hasSQLThreadError=false, isUnderCloning=true") {
				return nil
			}
			return fmt.Errorf("should become NOT_SERVING and IsUnderCloning=true: res=%s, err=%+v", res.Status, err)
		}, 5*time.Second, 200*time.Millisecond).Should(Succeed())

		By("wating clone process is finished")
		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("clone process is still working")
		}).Should(Succeed())

		By("wating cloning is completed")
		Eventually(func() error {
			res, err := gsrv.Check(context.Background(), &healthpb.HealthCheckRequest{})
			if err == nil {
				return nil
			}
			return fmt.Errorf("should return without error: res=%s", res.String())
		}).Should(Succeed())
	})

	It("should return hasIOThreadError=true if replica status has IO error", func() {
		By("executing START SLAVE with invalid parameters")
		err := test_utils.StartSlaveWithInvalidSettings(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health expecting IsOutOfSync=true")
		Eventually(func() error {
			res, err := gsrv.Check(context.Background(), &healthpb.HealthCheckRequest{})
			if res.Status == healthpb.HealthCheckResponse_NOT_SERVING && strings.HasSuffix(err.Error(), "hasIOThreadError=true, hasSQLThreadError=false, isUnderCloning=false") {
				return nil
			}
			return fmt.Errorf("should become NOT_SERVING and hasIOThreadError=true: res=%s, err=%+v", res.Status, err)
		}, 5*time.Second, 200*time.Millisecond).Should(Succeed())

	})

	It("should return healthy in primary mode", func() {
		By("executing START, STOP, and RESET SLAVE (simulating switching to primary")
		err := test_utils.StartSlaveWithInvalidSettings(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.StopAndResetSlave(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("getting health")
		res, err := gsrv.Check(context.Background(), &healthpb.HealthCheckRequest{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.Status).Should(Equal(healthpb.HealthCheckResponse_SERVING))
	})
}
