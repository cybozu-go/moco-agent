package server

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/moco/metrics"
	"github.com/cybozu-go/moco/test_utils"
	"github.com/cybozu-go/well"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testBackupBinaryLogs() {
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
		metrics.RegisterAgentMetrics(registry)

		By("initializing MySQL replica")
		test_utils.MySQLVersion = "8.0.20"
		err = test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("initializing MinIO object storage")
		stopMinIO("moco-test-minio")
		err = startMinIO("moco-test-minio", 9000)
		Expect(err).ShouldNot(HaveOccurred())

		By("setting environment variables for password")
		os.Setenv(moco.RootPasswordEnvName, test_utils.RootUserPassword)
	})

	AfterEach(func() {
		By("deleting MinIO container")
		os.RemoveAll(tmpDir)
	})

	It("should flush and backup binlog", func() {
		By("calling /backup-binlog API")
		req := httptest.NewRequest("GET", "http://"+replicaHost+"/flush-backup-binlog", nil)
		queries := url.Values{
			moco.AgentTokenParam:                       []string{token},
			mocoagent.BackupBinaryLogFilePrefixParam:   []string{"binlog-backup"},
			mocoagent.BackupBinaryLogBucketHostParam:   []string{"localhost"},
			mocoagent.BackupBinaryLogBucketPortParam:   []string{"9000"},
			mocoagent.BackupBinaryLogBucketNameParam:   []string{"moco-test-bucket"},
			mocoagent.BackupBinaryLogBucketRegionParam: []string{""},

			mocoagent.AccessKeyIDParam:     []string{"minioadmin"},
			mocoagent.SecretAccessKeyParam: []string{"minioadmin"},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.FlushAndBackupBinaryLogs(res, req)

		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		for {
			time.Sleep(20)
		}
	})
}

func startMinIO(name string, port int) error {
	ctx := context.Background()

	cmd := well.CommandContext(ctx,
		"docker", "run", "-d", "--restart=always",
		"--network=moco-test-net",
		"--name", name,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"minio/minio", "server", "/data",
	)
	return run(cmd)
}

func stopMinIO(name string) error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "stop", name)
	run(cmd)

	cmd = well.CommandContext(ctx, "docker", "rm", name)
	return run(cmd)
}

func run(cmd *well.LogCmd) error {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf

	err := cmd.Run()
	stdout := strings.TrimRight(outBuf.String(), "\n")
	if len(stdout) != 0 {
		fmt.Println("[test_utils/stdout] " + stdout)
	}
	stderr := strings.TrimRight(errBuf.String(), "\n")
	if len(stderr) != 0 {
		fmt.Println("[test_utils/stderr] " + stderr)
	}
	return err
}
