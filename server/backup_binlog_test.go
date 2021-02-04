package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"

	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/moco/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

func testBackupBinaryLogs() {
	var tmpDir string
	var binlogDir string
	var agent *Agent
	var registry *prometheus.Registry

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "moco-agent-test-")
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
		binlogDir, err = ioutil.TempDir("", "moco-agent-test-binlog-base-")
		Expect(err).ShouldNot(HaveOccurred())
		fmt.Println(binlogDir)
		err = os.Chmod(binlogDir, 0777)
		Expect(err).ShouldNot(HaveOccurred())

		test_utils.MySQLVersion = "8.0.20"
		err = test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID, binlogDir)
		Expect(err).ShouldNot(HaveOccurred())
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("initializing MinIO object storage")
		test_utils.StopMinIO("moco-agent-test-minio")
		err = test_utils.StartMinIO("moco-agent-test-minio", 9000)
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
