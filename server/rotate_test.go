package server

import (
	"os"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var _ = Describe("log rotation", func() {
	It("should rotate logs", func() {
		By("starting MySQLd")
		StartMySQLD(replicaHost, replicaPort, replicaServerID)
		defer StopAndRemoveMySQLD(replicaHost)

		sockFile := filepath.Join(socketDir(replicaHost), "mysqld.sock")
		tmpDir, err := os.MkdirTemp("", "moco-test-agent-")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := MySQLAccessorConfig{
			Host:              "localhost",
			Port:              replicaPort,
			Password:          agentUserPassword,
			ConnMaxIdleTime:   30 * time.Minute,
			ConnectionTimeout: 3 * time.Second,
			ReadTimeout:       30 * time.Second,
		}
		agent, err := New(conf, testClusterName, sockFile, tmpDir, maxDelayThreshold, testLogger)
		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		By("preparing log files for testing")
		slowFile := filepath.Join(tmpDir, mocoagent.MySQLSlowLogName)
		errFile := filepath.Join(tmpDir, mocoagent.MySQLErrorLogName)
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
		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 1))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 0))

		By("creating the same name directory")
		for _, file := range logFiles {
			err := os.Rename(file+".0", file)
			Expect(err).ShouldNot(HaveOccurred())
			err = os.Mkdir(file+".0", 0777)
			Expect(err).ShouldNot(HaveOccurred())
		}

		agent.RotateLog()

		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 2))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 1))
	})
})
