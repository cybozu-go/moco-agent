package server

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/moco/test_utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testRotate() {
	var tmpDir string
	var agent *Agent
	var registry *prometheus.Registry

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "moco-test-agent-")
		Expect(err).ShouldNot(HaveOccurred())
		agent = New(test_utils.Host, token, test_utils.MiscUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, tmpDir, replicaPort,
			&accessor.MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)

		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("should rotate log files", func() {
		err := test_utils.StartMySQLD(donorHost, donorPort, donorServerID)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID)
		Expect(err).ShouldNot(HaveOccurred())

		err = test_utils.InitializeMySQL(donorPort)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		defer func() {
			test_utils.StopAndRemoveMySQLD(donorHost)
			test_utils.StopAndRemoveMySQLD(replicaHost)
		}()

		By("preparing log files for testing")
		slowFile := filepath.Join(tmpDir, moco.MySQLSlowLogName)
		errFile := filepath.Join(tmpDir, moco.MySQLErrorLogName)
		logFiles := []string{slowFile, errFile}

		for _, file := range logFiles {
			_, err := os.Create(file)
			Expect(err).ShouldNot(HaveOccurred())
		}

		agent.RotateLog()

		for _, file := range logFiles {
			_, err := os.Stat(file + ".0")
			Expect(err).ShouldNot(HaveOccurred())
		}
		rotationCount, err := getMetric(registry, metricsPrefix+"log_rotation_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*rotationCount.Counter.Value).Should(Equal(1.0))
		rotationFailureCount, err := getMetric(registry, metricsPrefix+"log_rotation_failure_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*rotationFailureCount.Counter.Value).Should(Equal(0.0))
		rotationDurationSeconds, err := getMetric(registry, metricsPrefix+"log_rotation_duration_seconds")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(rotationDurationSeconds.Summary.Quantile)).ShouldNot(Equal(0))

		By("creating the same name directory")
		for _, file := range logFiles {
			err := os.Rename(file+".0", file)
			Expect(err).ShouldNot(HaveOccurred())
			err = os.Mkdir(file+".0", 0777)
			Expect(err).ShouldNot(HaveOccurred())
		}

		agent.RotateLog()

		rotationCount, err = getMetric(registry, metricsPrefix+"log_rotation_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*rotationCount.Counter.Value).Should(Equal(2.0))
		rotationFailureCount, err = getMetric(registry, metricsPrefix+"log_rotation_failure_count")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(*rotationFailureCount.Counter.Value).Should(Equal(1.0))
	})
}
