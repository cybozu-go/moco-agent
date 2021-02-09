package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/moco/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	agentTestPrefix = "moco-agent-test-"
	binlogPrefix    = "binlog"
	binlogDirPrefix = agentTestPrefix + "binlog-base-"
	bucketName      = agentTestPrefix + "bucket"
)

func testBackupBinaryLogs() {
	var tmpDir string
	var binlogDir string
	var agent *Agent
	var registry *prometheus.Registry
	var sess *session.Session

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", agentTestPrefix)
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

		By("creating MySQL and MinIO containers")
		binlogDir, err = ioutil.TempDir("", binlogDirPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		fmt.Println(binlogDir)
		err = os.Chmod(binlogDir, 0777|os.ModeSetgid)
		Expect(err).ShouldNot(HaveOccurred())

		test_utils.MySQLVersion = "8.0.20"
		test_utils.StopAndRemoveMySQLD(replicaHost)
		err = test_utils.StartMySQLD(replicaHost, replicaPort, replicaServerID, binlogDir, binlogPrefix)
		Expect(err).ShouldNot(HaveOccurred())

		test_utils.StopMinIO(agentTestPrefix + "minio")
		err = test_utils.StartMinIO(agentTestPrefix+"minio", 9000)
		Expect(err).ShouldNot(HaveOccurred())

		By("initializing MySQL replica")
		err = test_utils.InitializeMySQL(replicaPort)
		Expect(err).ShouldNot(HaveOccurred())

		By("initializing MinIO")
		sess = session.Must(session.NewSession(&aws.Config{
			Region:           aws.String("neco"),
			Endpoint:         aws.String(fmt.Sprintf("%s:%d", "localhost", 9000)),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
			Credentials:      credentials.NewStaticCredentials("minioadmin", "minioadmin", ""),
		}))

		Eventually(func() error {
			return createBucket(sess, bucketName)
		}, 10*time.Second).Should(Succeed())

		By("setting environment variables for password")
		os.Setenv(moco.RootPasswordEnvName, test_utils.RootUserPassword)
	})

	AfterEach(func() {
		By("deleting MinIO container")
		os.RemoveAll(tmpDir)
	})

	It("should flush and backup binlog", func() {
		By("calling /flush-backup-binlog API")
		req := httptest.NewRequest("GET", "http://"+replicaHost+"/flush-backup-binlog", nil)
		queries := url.Values{
			moco.AgentTokenParam:                       []string{token},
			mocoagent.BackupBinaryLogFilePrefixParam:   []string{binlogPrefix},
			mocoagent.BackupBinaryLogBucketHostParam:   []string{"localhost"},
			mocoagent.BackupBinaryLogBucketPortParam:   []string{"9000"},
			mocoagent.BackupBinaryLogBucketNameParam:   []string{bucketName},
			mocoagent.BackupBinaryLogBucketRegionParam: []string{""},

			mocoagent.AccessKeyIDParam:     []string{"minioadmin"},
			mocoagent.SecretAccessKeyParam: []string{"minioadmin"},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.FlushAndBackupBinaryLogs(res, req)

		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		type BinlogFileListObjectKey struct {
			BinlogFileListObjectKey string `json:"BinlogFileListObjectKey"`
		}
		var objKey BinlogFileListObjectKey
		err := json.Unmarshal(res.Body.Bytes(), &objKey)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			return checkObjectExistence(sess, bucketName, binlogPrefix)
		}, 10*time.Second).Should(Succeed())

		objStr, err := getObjectAsString(sess, bucketName, binlogPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		objNames := strings.Split(objStr, "\n")
		expectedObjNames := []string{binlogPrefix + "-000000"}
		Expect(objNames).Should(Equal(expectedObjNames))

		Eventually(func() error {
			for _, objName := range objNames {
				err := checkObjectExistence(sess, bucketName, objName)
				if err != nil {
					return err
				}
			}
			return nil
		}).Should(Succeed())

		Eventually(func() error {
			binlogName := binlogDir + "/" + binlogPrefix + ".000001"
			_, err := os.Stat(binlogName)
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("file: %s should be deleted, but exists", binlogName)
		}).Should(Succeed())
	})

	It("should only flush binlog", func() {
		By("calling /flush-binlog API without delete flag")
		req := httptest.NewRequest("GET", "http://"+replicaHost+"/flush-binlog", nil)
		queries := url.Values{
			moco.AgentTokenParam: []string{token},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.FlushBinaryLogs(res, req)

		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		Eventually(func() error {
			binlogSeqs := []string{"000001", "000002"}
			for _, s := range binlogSeqs {
				binlogName := binlogDir + "/" + binlogPrefix + "." + s
				_, err := os.Stat(binlogName)
				if err != nil {
					return fmt.Errorf("file: %s should exist, but be deleted", binlogName)
				}
			}
			return nil
		}, 10*time.Second).Should(Succeed())

		By("calling /flush-binlog API with delete flag")
		req = httptest.NewRequest("GET", "http://"+replicaHost+"/flush-binlog", nil)
		queries = url.Values{
			moco.AgentTokenParam:                []string{token},
			mocoagent.FlushBinaryLogDeleteparam: []string{"true"},
		}
		req.URL.RawQuery = queries.Encode()

		res = httptest.NewRecorder()
		agent.FlushBinaryLogs(res, req)

		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		Eventually(func() error {
			binlogSeqs := []string{"000001", "000002"}
			for _, s := range binlogSeqs {
				binlogName := binlogDir + "/" + binlogPrefix + "." + s
				_, err := os.Stat(binlogName)
				if !os.IsNotExist(err) {
					return fmt.Errorf("file: %s should be deleted, but exists", binlogName)
				}
			}
			binlogName := binlogDir + "/" + binlogPrefix + ".000003"
			_, err := os.Stat(binlogName)
			if err != nil {
				return fmt.Errorf("file: %s should exist, but not", binlogName)
			}
			return nil
		}, 10*time.Second).Should(Succeed())
	})
}

func createBucket(sess *session.Session, bucketName string) error {
	svc := s3.New(sess)
	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	return err
}

func getObjectAsString(sess *session.Session, bucketName, objectName string) (string, error) {
	svc := s3.New(sess)
	res, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, res.Body)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
