package server

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
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
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/cybozu-go/moco/accessor"
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
		agent = New(test_utils.Host, clusterName, token, test_utils.MiscUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, tmpDir, replicaPort,
			&accessor.MySQLAccessorConfig{
				ConnMaxLifeTime:   30 * time.Minute,
				ConnectionTimeout: 3 * time.Second,
				ReadTimeout:       30 * time.Second,
			},
		)

		By("creating MySQL and MinIO containers")
		binlogDir, err = ioutil.TempDir("", binlogDirPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		fmt.Println(binlogDir)
		err = os.Chmod(binlogDir, 0777|os.ModeSetgid)
		Expect(err).ShouldNot(HaveOccurred())

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

		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)

		backupBinlogCount, _ := getMetric(registry, metricsPrefix+"backup_binlog_count")
		Expect(backupBinlogCount).Should(BeNil())

		backupBinlogFailureCount, _ := getMetric(registry, metricsPrefix+"backup_binlog_failure_count")
		Expect(backupBinlogFailureCount).Should(BeNil())

		backupBinlogDurationSeconds, _ := getMetric(registry, metricsPrefix+"backup_binlog_duration_seconds")
		Expect(backupBinlogDurationSeconds).Should(BeNil())
	})

	AfterEach(func() {
		By("deleting MySQL containers")
		test_utils.StopAndRemoveMySQLD(donorHost)
		test_utils.StopAndRemoveMySQLD(replicaHost)

		By("deleting MinIO container")
		test_utils.StopMinIO(agentTestPrefix + "minio")
		os.RemoveAll(tmpDir)
		os.RemoveAll(binlogDir)
	})

	It("should flush and backup binlog", func() {
		By("calling /flush-backup-binlog API")
		req := httptest.NewRequest("POST", "http://"+replicaHost+"/flush-backup-binlog", nil)
		queries := url.Values{
			moco.AgentTokenParam:                       []string{token},
			mocoagent.BackupBinaryLogBackupIDParam:     []string{binlogPrefix},
			mocoagent.BackupBinaryLogBucketHostParam:   []string{"localhost"},
			mocoagent.BackupBinaryLogBucketPortParam:   []string{"9000"},
			mocoagent.BackupBinaryLogBucketNameParam:   []string{bucketName},
			mocoagent.BackupBinaryLogBucketRegionParam: []string{"neco"},

			mocoagent.AccessKeyIDParam:     []string{"minioadmin"},
			mocoagent.SecretAccessKeyParam: []string{"minioadmin"},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.FlushAndBackupBinaryLogs(res, req)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		By("checking the binlog file is uploaded")
		Eventually(func() error {
			_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(binlogPrefix),
			})
			return err
		}, 10*time.Second).Should(Succeed())

		objStr, err := getObjectAsString(sess, bucketName, binlogPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		objNames := strings.Split(objStr, "\n")
		expectedObjNames := []string{binlogPrefix + "-000000"}
		Expect(objNames).Should(Equal(expectedObjNames))

		Eventually(func() error {
			for _, objName := range objNames {
				_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(objName),
				})
				if err != nil {
					return err
				}
			}
			return nil
		}).Should(Succeed())

		By("checking the uploaded binlog file is deleted")
		Eventually(func() error {
			binlogName := binlogDir + "/" + binlogPrefix + ".000001"
			_, err := os.Stat(binlogName)
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("file: %s should be deleted, but exists", binlogName)
		}).Should(Succeed())

		By("calling /flush-backup-binlog API with the same prefix")
		req = httptest.NewRequest("POST", "http://"+replicaHost+"/flush-backup-binlog", nil)
		queries = url.Values{
			moco.AgentTokenParam:                       []string{token},
			mocoagent.BackupBinaryLogBackupIDParam:     []string{binlogPrefix},
			mocoagent.BackupBinaryLogBucketHostParam:   []string{"localhost"},
			mocoagent.BackupBinaryLogBucketPortParam:   []string{"9000"},
			mocoagent.BackupBinaryLogBucketNameParam:   []string{bucketName},
			mocoagent.BackupBinaryLogBucketRegionParam: []string{"neco"},

			mocoagent.AccessKeyIDParam:     []string{"minioadmin"},
			mocoagent.SecretAccessKeyParam: []string{"minioadmin"},
		}
		req.URL.RawQuery = queries.Encode()

		res = httptest.NewRecorder()
		agent.FlushAndBackupBinaryLogs(res, req)
		Expect(res).Should(HaveHTTPStatus(http.StatusConflict))

		By("checking metrics")
		Eventually(func() error {
			binlogBackupCount, _ := getMetric(registry, metricsPrefix+"binlog_backup_count")
			if binlogBackupCount == nil || *binlogBackupCount.Counter.Value != 1.0 {
				return fmt.Errorf("binlog_backup_count isn't incremented yet: value=%f", *binlogBackupCount.Counter.Value)
			}

			binlogBackupFailureCount, _ := getMetric(registry, metricsPrefix+"binlog_backup_failure_count")
			if binlogBackupFailureCount != nil && *binlogBackupFailureCount.Counter.Value != 0.0 {
				return fmt.Errorf("binlog_backup_failure_count should not be incremented: value=%f", *binlogBackupFailureCount.Counter.Value)
			}

			binlogBackupDurationSeconds, _ := getMetric(registry, metricsPrefix+"binlog_backup_duration_seconds")
			for _, quantile := range binlogBackupDurationSeconds.Summary.Quantile {
				if math.IsNaN(*quantile.Value) {
					return fmt.Errorf("binlog_backup_duration_seconds should have values: quantile=%f, value=%f", *quantile.Quantile, *quantile.Value)
				}
			}

			return nil
		}, 30*time.Second).Should(Succeed())

		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("backup process is still working")
		}).Should(Succeed())
	})

	It("should backup multiple binlog files", func() {
		By("calling /flush-binlog API without delete flag")
		req := httptest.NewRequest("POST", "http://"+replicaHost+"/flush-binlog", nil)
		queries := url.Values{
			moco.AgentTokenParam: []string{token},
		}
		req.URL.RawQuery = queries.Encode()

		res := httptest.NewRecorder()
		agent.FlushBinaryLogs(res, req)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		By("calling /flush-backup-binlog API")
		req = httptest.NewRequest("POST", "http://"+replicaHost+"/flush-backup-binlog", nil)
		queries = url.Values{
			moco.AgentTokenParam:                       []string{token},
			mocoagent.BackupBinaryLogBackupIDParam:     []string{binlogPrefix},
			mocoagent.BackupBinaryLogBucketHostParam:   []string{"localhost"},
			mocoagent.BackupBinaryLogBucketPortParam:   []string{"9000"},
			mocoagent.BackupBinaryLogBucketNameParam:   []string{bucketName},
			mocoagent.BackupBinaryLogBucketRegionParam: []string{"neco"},

			mocoagent.AccessKeyIDParam:     []string{"minioadmin"},
			mocoagent.SecretAccessKeyParam: []string{"minioadmin"},
		}
		req.URL.RawQuery = queries.Encode()

		res = httptest.NewRecorder()
		agent.FlushAndBackupBinaryLogs(res, req)
		Expect(res).Should(HaveHTTPStatus(http.StatusOK))

		By("checking the multiple binlog files are uploaded")
		Eventually(func() error {
			_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(binlogPrefix),
			})
			return err
		}, 10*time.Second).Should(Succeed())

		objStr, err := getObjectAsString(sess, bucketName, binlogPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		objNames := strings.Split(objStr, "\n")
		expectedObjNames := []string{binlogPrefix + "-000000", binlogPrefix + "-000001"}
		Expect(objNames).Should(Equal(expectedObjNames))

		Eventually(func() error {
			for _, objName := range objNames {
				_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(objName),
				})
				if err != nil {
					return err
				}
			}
			return nil
		}).Should(Succeed())

		By("checking the uploaded binlog files are deleted")
		Eventually(func() error {
			binlogNames := []string{binlogDir + "/" + binlogPrefix + ".000001", binlogDir + "/" + binlogPrefix + ".000002"}
			for _, b := range binlogNames {
				_, err := os.Stat(b)
				if !os.IsNotExist(err) {
					return fmt.Errorf("file: %s should be deleted, but exists", b)
				}
			}
			return nil
		}).Should(Succeed())

		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("backup process is still working")
		}).Should(Succeed())
	})

	It("should only flush binlog", func() {
		By("calling /flush-binlog API without delete flag")
		req := httptest.NewRequest("POST", "http://"+replicaHost+"/flush-binlog", nil)
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
		req = httptest.NewRequest("POST", "http://"+replicaHost+"/flush-binlog", nil)
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
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, res.Body)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
