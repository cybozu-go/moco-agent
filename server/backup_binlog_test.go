package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco-agent/test_utils"
	"github.com/cybozu-go/moco/accessor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	agentTestPrefix = "moco-agent-test-"
	binlogPrefix    = "binlog"
	backupID        = "test-backup-id"
	binlogDirPrefix = agentTestPrefix + "binlog-base-"
	bucketName      = agentTestPrefix + "bucket"
)

func testBackupBinlog() {
	var tmpDir string
	var binlogDir string
	var agent *Agent
	var registry *prometheus.Registry
	var sess *session.Session
	var gsrv agentrpc.BackupBinlogServiceServer

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", agentTestPrefix)
		Expect(err).ShouldNot(HaveOccurred())
		agent = New(test_utils.Host, clusterName, token, test_utils.AgentUserPassword, test_utils.CloneDonorUserPassword, replicationSourceSecretPath, tmpDir, replicaPort,
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
		err = test_utils.InitializeMySQL(replicaPort)
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
		os.Setenv(mocoagent.AdminPasswordEnvName, test_utils.AdminUserPassword)

		registry = prometheus.NewRegistry()
		metrics.RegisterMetrics(registry)

		gsrv = NewBackupBinlogService(agent)
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
		req := &agentrpc.FlushAndBackupBinlogRequest{
			Token:           token,
			BackupId:        backupID,
			BucketHost:      "localhost",
			BucketPort:      9000,
			BucketName:      bucketName,
			BucketRegion:    "neco",
			AccessKeyId:     "minioadmin",
			SecretAccessKey: "minioadmin",
		}
		_, err := gsrv.FlushAndBackupBinlog(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		By("checking in-progress metric is set")
		binlogBackupGauge, _ := getMetric(registry, metricsPrefix+"binlog_backup_in_progress")
		Expect(*binlogBackupGauge.Gauge.Value).Should(Equal(1.0))
		Expect(*binlogBackupGauge.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*binlogBackupGauge.Label[0].Value).Should(Equal(clusterName))

		By("wating binlog backup process is finished")
		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("process is still working")
		}, 30*time.Second).Should(Succeed())

		By("checking in-progress metric is cleared")
		binlogBackupGauge, _ = getMetric(registry, metricsPrefix+"binlog_backup_in_progress")
		Expect(*binlogBackupGauge.Gauge.Value).Should(Equal(0.0))

		By("checking the binlog file is uploaded")
		_, err = s3.New(sess).HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(backupID),
		})
		Expect(err).ShouldNot(HaveOccurred())

		objStr, err := getObjectAsString(sess, bucketName, backupID)
		Expect(err).ShouldNot(HaveOccurred())
		objNames := strings.Split(objStr, "\n")
		expectedObjNames := []string{backupID + "-000000"}
		Expect(objNames).Should(Equal(expectedObjNames))

		for _, objName := range objNames {
			_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(objName),
			})
			Expect(err).ShouldNot(HaveOccurred())
		}

		By("checking the uploaded binlog file is deleted")
		binlogName := binlogDir + "/" + binlogPrefix + ".000001"
		_, err = os.Stat(binlogName)
		Expect(os.IsNotExist(err)).Should(BeTrue())

		By("calling /flush-backup-binlog API with the same backup ID")
		_, err = gsrv.FlushAndBackupBinlog(context.Background(), req)
		Expect(err.Error()).Should(Equal(fmt.Sprintf("rpc error: code = InvalidArgument desc = the requested backup has already completed: BackupId=%s", backupID)))

		By("checking metrics")
		binlogBackupCount, _ := getMetric(registry, metricsPrefix+"binlog_backup_count")
		Expect(*binlogBackupCount.Counter.Value).Should(Equal(1.0))
		Expect(*binlogBackupCount.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*binlogBackupCount.Label[0].Value).Should(Equal(clusterName))

		binlogBackupFailureCount, _ := getMetric(registry, metricsPrefix+"binlog_backup_failure_count")
		Expect(binlogBackupFailureCount).Should(BeNil())

		binlogBackupDurationSeconds, _ := getMetric(registry, metricsPrefix+"binlog_backup_duration_seconds")
		Expect(*binlogBackupDurationSeconds.Label[0].Name).Should(Equal("cluster_name"))
		Expect(*binlogBackupDurationSeconds.Label[0].Value).Should(Equal(clusterName))
		for _, quantile := range binlogBackupDurationSeconds.Summary.Quantile {
			Expect(math.IsNaN(*quantile.Value)).Should(BeFalse())
		}
	})

	It("should backup multiple binlog files", func() {
		By("calling /flush-binlog API without delete flag")
		flushReq := &agentrpc.FlushBinlogRequest{
			Token: token,
		}
		_, err := gsrv.FlushBinlog(context.Background(), flushReq)
		Expect(err).ShouldNot(HaveOccurred())

		By("calling /flush-backup-binlog API")
		req := &agentrpc.FlushAndBackupBinlogRequest{
			Token:           token,
			BackupId:        backupID,
			BucketHost:      "localhost",
			BucketPort:      9000,
			BucketName:      bucketName,
			BucketRegion:    "neco",
			AccessKeyId:     "minioadmin",
			SecretAccessKey: "minioadmin",
		}
		_, err = gsrv.FlushAndBackupBinlog(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			if agent.sem.TryAcquire(1) {
				agent.sem.Release(1)
				return nil
			}
			return errors.New("backup process is still working")
		}, 30*time.Second).Should(Succeed())

		By("checking the multiple binlog files are uploaded")
		_, err = s3.New(sess).HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(backupID),
		})
		Expect(err).ShouldNot(HaveOccurred())

		objStr, err := getObjectAsString(sess, bucketName, backupID)
		Expect(err).ShouldNot(HaveOccurred())
		objNames := strings.Split(objStr, "\n")
		expectedObjNames := []string{backupID + "-000000", backupID + "-000001"}
		Expect(objNames).Should(Equal(expectedObjNames))

		for _, objName := range objNames {
			_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(objName),
			})
			Expect(err).ShouldNot(HaveOccurred())
		}

		By("checking the uploaded binlog files are deleted")
		binlogNames := []string{binlogDir + "/" + binlogPrefix + ".000001", binlogDir + "/" + binlogPrefix + ".000002"}
		for _, b := range binlogNames {
			_, err := os.Stat(b)
			Expect(os.IsNotExist(err)).Should(BeTrue())
		}
	})

	It("should only flush binlog", func() {
		By("calling /flush-binlog API without delete flag")
		req := &agentrpc.FlushBinlogRequest{
			Token: token,
		}
		_, err := gsrv.FlushBinlog(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		binlogSeqs := []string{"000001", "000002"}
		for _, s := range binlogSeqs {
			binlogName := binlogDir + "/" + binlogPrefix + "." + s
			_, err := os.Stat(binlogName)
			Expect(err).ShouldNot(HaveOccurred())
		}

		By("calling /flush-binlog API with delete flag")
		req = &agentrpc.FlushBinlogRequest{
			Token:  token,
			Delete: true,
		}
		_, err = gsrv.FlushBinlog(context.Background(), req)
		Expect(err).ShouldNot(HaveOccurred())

		binlogSeqs = []string{"000001", "000002"}
		for _, s := range binlogSeqs {
			binlogName := binlogDir + "/" + binlogPrefix + "." + s
			_, err := os.Stat(binlogName)
			Expect(os.IsNotExist(err)).Should(BeTrue())
		}
		binlogName := binlogDir + "/" + binlogPrefix + ".000003"
		_, err = os.Stat(binlogName)
		Expect(err).ShouldNot(HaveOccurred())
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
