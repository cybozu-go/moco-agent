package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

type testData struct {
	primaryHost            string
	primaryPort            int
	primaryUser            string
	primaryPassword        string
	cloneUser              string
	clonePassword          string
	initAfterCloneUser     string
	initAfterClonePassword string
}

func testClone() {
	var agent *Agent
	var registry *prometheus.Registry
	var gsrv agentrpc.CloneServiceServer

	BeforeEach(func() {
		// The configuration of the donor MySQL is different for each test case.
		// So the donor is not initialized here. The initialization will do at the beginning of each test case.
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)

		err := test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.SetValidDonorList(replicaPort, donorHost, donorPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.ResetMaster(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)

		agent = New(test_utils.Host, clusterName, token, test_utils.AgentUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, test_utils.MysqlSocketDir+"/mysqld.sock", "", replicaPort,
			MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)

		gsrv = NewCloneService(agent)
	})

	AfterEach(func() {
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)
	})

	It("should return error with bad requests", func() {
		initializeDonorMySQL(false)

		testcases := []struct {
			title string
			req   *agentrpc.CloneRequest
		}{
			{
				title: "passing invalid token",
				req: &agentrpc.CloneRequest{
					Token:     "invalid-token",
					DonorHost: donorHost,
					DonorPort: donorPort,
				},
			},
			{
				title: "passing empty token",
				req: &agentrpc.CloneRequest{
					DonorHost: donorHost,
					DonorPort: donorPort,
				},
			},
			{
				title: "passing empty donorHostName",
				req: &agentrpc.CloneRequest{
					Token:     token,
					DonorPort: donorPort,
				},
			},
			{
				title: "passing empty donorPort",
				req: &agentrpc.CloneRequest{
					Token:     token,
					DonorHost: donorHost,
				},
			},
		}

		for _, tt := range testcases {
			By(tt.title)
			_, err := gsrv.Clone(context.Background(), tt.req)
			Expect(err).Should(HaveOccurred())
		}

		By("checking metrics")
		// In these test cases, the clone will not start actually. So the metrics will not change.
		_, err := getMetric(registry, metricsPrefix+"clone_count")
		Expect(err).Should(HaveOccurred())

		_, err = getMetric(registry, metricsPrefix+"clone_failure_count")
		Expect(err).Should(HaveOccurred())
	})

	It("should clone from donor successfully", func() {
		initializeDonorMySQL(false)

		By("cloning from donor")
		req := &agentrpc.CloneRequest{
			Token:     token,
			DonorHost: donorHost,
			DonorPort: donorPort,
		}

		_, err := gsrv.Clone(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		By("checking in-progress metric is set")
		cloneGauge, _ := getMetric(registry, metricsPrefix+"clone_in_progress")
		Expect(*cloneGauge.Gauge.Value).Should(Equal(1.0))
		Expect(*cloneGauge.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*cloneGauge.Label[0].Value).Should(Equal(clusterName))

		By("cloning from donor (second time)")
		_, err = gsrv.Clone(context.Background(), req)
		Expect(err).Should(HaveOccurred())
		Expect(err.Error()).Should(Equal("rpc error: code = ResourceExhausted desc = another request is under processing"))

		By("wating clone process is finished")
		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("clone process is still working")
		}).Should(Succeed())

		By("checking in-progress metric is cleared")
		cloneGauge, _ = getMetric(registry, metricsPrefix+"clone_in_progress")
		Expect(*cloneGauge.Gauge.Value).Should(Equal(0.0))

		By("checking clone status")
		Eventually(func() error {
			db, err := test_utils.ConnectMySQL(test_utils.Host+":"+strconv.Itoa(replicaPort), mocoagent.AgentUser, test_utils.AgentUserPassword)
			if err != nil {
				return err
			}
			cloneStatus, err := GetMySQLCloneStateStatus(context.Background(), db)
			if err == nil && cloneStatus.State.Valid && cloneStatus.State.String == "Completed" {
				return nil
			}
			return fmt.Errorf("CLONE should be completed: state=%+v, err=%+v", cloneStatus, err)
		}).Should(Succeed())

		By("checking metrics")
		cloneCount, err := getMetric(registry, metricsPrefix+"clone_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*cloneCount.Counter.Value).Should(Equal(1.0))
		Expect(*cloneCount.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*cloneCount.Label[0].Value).Should(Equal(clusterName))

		_, err = getMetric(registry, metricsPrefix+"clone_failure_count")
		Expect(err).Should(HaveOccurred())

		cloneDurationSeconds, err := getMetric(registry, metricsPrefix+"clone_duration_seconds")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*cloneDurationSeconds.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*cloneDurationSeconds.Label[0].Value).Should(Equal(clusterName))
		for _, quantile := range cloneDurationSeconds.Summary.Quantile {
			Expect(math.IsNaN(*quantile.Value)).Should(BeFalse())
		}
	})

	It("should not clone if recipient has some data", func() {
		initializeDonorMySQL(false)

		By("write data to recipient")
		err := test_utils.PrepareTestData(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("cloning from donor")
		req := &agentrpc.CloneRequest{
			Token:     token,
			DonorHost: donorHost,
			DonorPort: donorPort,
		}

		_, err = gsrv.Clone(context.Background(), req)
		Expect(err).Should(HaveOccurred())
	})

	It("should not clone from external MySQL with invalid donor settings", func() {
		initializeDonorMySQL(true)

		testcases := []struct {
			title         string
			donorHost     string
			donorPort     int
			cloneUser     string
			clonePassword string
		}{
			{
				title:         "invalid donorHostName",
				donorHost:     "invalid-host",
				donorPort:     donorPort,
				cloneUser:     test_utils.ExternalDonorUser,
				clonePassword: test_utils.ExternalDonorUserPassword,
			},
			{
				title:         "invalid donorPort",
				donorHost:     donorHost,
				donorPort:     10000,
				cloneUser:     test_utils.ExternalDonorUser,
				clonePassword: test_utils.ExternalDonorUserPassword,
			},
			{
				title:         "invalid cloneUser",
				donorHost:     donorHost,
				donorPort:     donorPort,
				cloneUser:     "invalid-user",
				clonePassword: test_utils.ExternalDonorUserPassword,
			},
			{
				title:         "invalid clonePassword",
				donorHost:     donorHost,
				donorPort:     donorPort,
				cloneUser:     test_utils.ExternalDonorUser,
				clonePassword: "invalid-password",
			},
		}

		for _, tt := range testcases {
			By(fmt.Sprintf("(%s) %s", tt.title, "preparing test data"))
			data := &testData{
				primaryHost:            tt.donorHost,
				primaryPort:            tt.donorPort,
				cloneUser:              tt.cloneUser,
				clonePassword:          tt.clonePassword,
				initAfterCloneUser:     test_utils.ExternalInitUser,
				initAfterClonePassword: test_utils.ExternalInitUserPassword,
			}
			writeTestData(data)

			By(fmt.Sprintf("(%s) %s", tt.title, "setting  clone_valid_donor_list"))
			err := test_utils.SetValidDonorList(replicaPort, tt.donorHost, tt.donorPort)
			Expect(err).ShouldNot(HaveOccurred())

			By(fmt.Sprintf("(%s) %s", tt.title, "cloning from external MySQL"))
			req := &agentrpc.CloneRequest{
				Token:    token,
				External: true,
			}

			_, err = gsrv.Clone(context.Background(), req)
			Expect(err).ShouldNot(HaveOccurred())

			// Just in case, wait for the clone to be started.
			time.Sleep(3 * time.Second)

			By(fmt.Sprintf("(%s) %s", tt.title, "checking after clone status"))
			Eventually(func() error {
				db, err := test_utils.ConnectMySQL(test_utils.Host+":"+strconv.Itoa(replicaPort), mocoagent.AgentUser, test_utils.AgentUserPassword)
				if err != nil {
					return err
				}

				cloneStatus, err := GetMySQLCloneStateStatus(context.Background(), db)
				if err != nil {
					return err
				}

				expected := sql.NullString{Valid: true, String: "Failed"}
				if !cmp.Equal(cloneStatus.State, expected) {
					return fmt.Errorf("doesn't reach failed state: %+v", cloneStatus.State)
				}
				return nil
			}).Should(Succeed())
		}

		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("clone process is still working")
		}).Should(Succeed())

		By("checking metrics")
		// In these test cases, the clone will start and fail. So the metrics will change.
		cloneCount, err := getMetric(registry, metricsPrefix+"clone_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*cloneCount.Counter.Value).Should(Equal(float64(len(testcases))))
		Expect(*cloneCount.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*cloneCount.Label[0].Value).Should(Equal(clusterName))

		cloneFailureCount, err := getMetric(registry, metricsPrefix+"clone_failure_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*cloneFailureCount.Counter.Value).Should(Equal(float64(len(testcases))))
		Expect(*cloneFailureCount.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*cloneFailureCount.Label[0].Value).Should(Equal(clusterName))

		_, err = getMetric(registry, metricsPrefix+"clone_duration_seconds")
		Expect(err).Should(HaveOccurred())
	})

	It("should clone from external MySQL", func() {
		initializeDonorMySQL(true)

		By("preparing test data")
		data := &testData{
			primaryHost:            donorHost,
			primaryPort:            donorPort,
			cloneUser:              test_utils.ExternalDonorUser,
			clonePassword:          test_utils.ExternalDonorUserPassword,
			initAfterCloneUser:     test_utils.ExternalInitUser,
			initAfterClonePassword: test_utils.ExternalInitUserPassword,
		}
		writeTestData(data)

		By("cloning from external MySQL")
		req := &agentrpc.CloneRequest{
			Token:    token,
			External: true,
		}

		_, err := gsrv.Clone(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		By("wating clone process is finished")
		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("clone process is still working")
		}).Should(Succeed())

		By("confirming clone by init user")
		Eventually(func() error {
			db, err := test_utils.ConnectMySQL(test_utils.Host+":"+strconv.Itoa(replicaPort), test_utils.ExternalInitUser, test_utils.ExternalInitUserPassword)
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
		}).Should(Succeed())

		By("getting error when secret files doesn't exist")
		pwd, err := os.Getwd()
		rightPath := agent.replicationSourceSecretPath
		Expect(err).ShouldNot(HaveOccurred())
		agent.replicationSourceSecretPath = pwd

		req = &agentrpc.CloneRequest{
			Token:    token,
			External: true,
		}

		_, err = gsrv.Clone(context.Background(), req)
		Expect(err).Should(HaveOccurred())

		agent.replicationSourceSecretPath = rightPath

		// The initialization(*) after cloning from the external donor does not succeed in this test.
		// In the initialization, the agent tries to connect to the MySQL server via the Unix domain socket. But the connection will not be succeeded.
		// *) htps://github.com/cybozu-go/moco/blob/v0.3.1/agent/clone.go#L169-L197
		Skip("MySQL users for MOCO don't be created")

		By("confirming clone by restored agent user")
		restoredAgentUserPassword := "dummy"
		Eventually(func() error {
			db, err := test_utils.ConnectMySQL(test_utils.Host+":"+strconv.Itoa(replicaPort), mocoagent.AgentUser, restoredAgentUserPassword)
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
		}).Should(Succeed())
	})
}

func writeTestData(data *testData) {
	writeFile := func(filename, data string) error {
		return os.WriteFile(path.Join(replicationSourceSecretPath, filename), []byte(data), 0664)
	}

	var err error
	err = writeFile("PRIMARY_HOST", data.primaryHost)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("PRIMARY_PORT", strconv.Itoa(data.primaryPort))
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("PRIMARY_USER", data.primaryUser)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("PRIMARY_PASSWORD", data.primaryPassword)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("CLONE_USER", data.cloneUser)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("CLONE_PASSWORD", data.clonePassword)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("INIT_AFTER_CLONE_USER", data.initAfterCloneUser)
	Expect(err).ShouldNot(HaveOccurred())
	err = writeFile("INIT_AFTER_CLONE_PASSWORD", data.initAfterClonePassword)
	Expect(err).ShouldNot(HaveOccurred())
}

func initializeDonorMySQL(isExternal bool) {
	By("initializing MySQL donor")
	err := test_utils.StartMySQLDWithSockeDir(donorHost, donorPort, donorServerID, isExternal)
	Expect(err).ShouldNot(HaveOccurred())
	if isExternal {
		err = test_utils.InitializeMySQLAsExternalDonor(donorPort)
		Expect(err).ShouldNot(HaveOccurred())
	} else {
		err = test_utils.InitializeMySQL(donorPort)
		Expect(err).ShouldNot(HaveOccurred())
	}
	err = test_utils.PrepareTestData(donorPort)
	Expect(err).ShouldNot(HaveOccurred())
	err = test_utils.ResetMaster(donorPort)
	Expect(err).ShouldNot(HaveOccurred())
}
