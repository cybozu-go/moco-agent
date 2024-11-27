package server

import (
	"bytes"
	"os"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var _ = Describe("log rotation", Ordered, func() {
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
		agent, err := New(conf, testClusterName, sockFile, tmpDir, maxDelayThreshold, time.Second, testLogger)
		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		By("preparing log file for testing")
		logFile := filepath.Join(tmpDir, mocoagent.MySQLSlowLogName)

		_, err = os.Create(logFile)
		Expect(err).ShouldNot(HaveOccurred())

		agent.RotateLog()

		_, err = os.Stat(logFile + ".0")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 1))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 0))

		By("creating the same name directory")
		err = os.Rename(logFile+".0", logFile)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Mkdir(logFile+".0", 0777)
		Expect(err).ShouldNot(HaveOccurred())

		agent.RotateLog()

		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 2))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 1))
	})

	It("should rotate logs by RotateLogIfSizeExceeded if size exceeds", func() {
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
		agent, err := New(conf, testClusterName, sockFile, tmpDir, maxDelayThreshold, time.Second, testLogger)
		Expect(err).ShouldNot(HaveOccurred())
		defer agent.CloseDB()

		By("preparing log file for testing")
		logFile := filepath.Join(tmpDir, mocoagent.MySQLSlowLogName)

		logDataSize := 512
		data := bytes.Repeat([]byte("a"), logDataSize)
		f, err := os.Create(logFile)
		Expect(err).ShouldNot(HaveOccurred())
		f.Write(data)

		agent.RotateLogIfSizeExceeded(int64(logDataSize) + 1)

		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 2))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 1))

		agent.RotateLogIfSizeExceeded(int64(logDataSize) - 1)

		_, err = os.Stat(logFile + ".0")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 3))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 1))

		By("creating the same name directory")
		err = os.Rename(logFile+".0", logFile)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Mkdir(logFile+".0", 0777)
		Expect(err).ShouldNot(HaveOccurred())

		agent.RotateLogIfSizeExceeded(int64(logDataSize) - 1)

		Expect(testutil.ToFloat64(metrics.LogRotationCount)).To(BeNumerically("==", 4))
		Expect(testutil.ToFloat64(metrics.LogRotationFailureCount)).To(BeNumerically("==", 2))
	})
})
