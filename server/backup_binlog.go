package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cybozu-go/log"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/well"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewBackupBinlogService creates a new BackupBinlogServiceServer
func NewBackupBinlogService(agent *Agent) agentrpc.BackupBinlogServiceServer {
	return &backupBinlogService{
		agent: agent,
	}
}

type backupBinlogService struct {
	agentrpc.UnimplementedBackupBinlogServiceServer
	agent *Agent
}

type BackupBinaryLogsParams struct {
	BackupID     string
	BucketHost   string
	BucketPort   int
	BucketName   string
	BucketRegion string

	AccessKeyID     string
	SecretAccessKey string
}

// FlushAndBackupBinaryLogs executes "FLUSH BINARY LOGS;"
// and upload it to the object storage, then delete it
func (s *backupBinlogService) FlushAndBackupBinlog(ctx context.Context, req *agentrpc.FlushAndBackupBinlogRequest) (*agentrpc.FlushAndBackupBinlogResponse, error) {
	if req.Token != s.agent.token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if !s.agent.sem.TryAcquire(1) {
		return nil, status.Error(codes.ResourceExhausted, "another request is under processing")
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:           aws.String(req.BucketRegion),
		Endpoint:         aws.String(fmt.Sprintf("%s:%d", req.BucketHost, req.BucketPort)),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(req.AccessKeyId, req.SecretAccessKey, ""),
	}))

	// prevent re-execution of the same request
	_, err := s3.New(sess).HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(req.BucketName),
		Key:    aws.String(req.BackupId),
	})

	if err == nil {
		s.agent.sem.Release(1)
		return nil, status.Errorf(codes.InvalidArgument, "the requested backup has already completed: BackupId=%s", req.BackupId)
	}

	awsErr, ok := err.(awserr.RequestFailure)
	if !ok {
		s.agent.sem.Release(1)
		log.Error("unknown response from object storage", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "unknown response from object storage: err=%+v", err)
	}
	if awsErr.StatusCode() != http.StatusNotFound {
		s.agent.sem.Release(1)
		log.Error("failed to get objects", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to get object: err=%+v", err)
	}

	// TODO: change user to AgentUser (need to add appropriate privileges to AgentUser)
	adminPassword := os.Getenv(mocoagent.AdminPasswordEnvName)
	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), mocoagent.AdminUser, adminPassword)
	if err != nil {
		s.agent.sem.Release(1)
		log.Error("failed to connect to database before flush binary logs", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to connect to database before flush binary logs: err=%+v", err)
	}

	metrics.IncrementBinlogBackupCountMetrics(s.agent.clusterName)

	err = flushBinaryLogs(ctx, db)
	if err != nil {
		s.agent.sem.Release(1)
		log.Error("failed to flush binary logs", map[string]interface{}{
			log.FnError: err,
		})
		metrics.IncrementBinlogBackupFailureCountMetrics(s.agent.clusterName, "flush")
		return nil, status.Errorf(codes.Internal, "failed to flush binary logs: err=%+v", err)
	}

	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {
		startTime := time.Now()
		metrics.SetBinlogBackupInProgressMetrics(s.agent.clusterName, true)
		defer func() {
			s.agent.sem.Release(1)
			metrics.SetBinlogBackupInProgressMetrics(s.agent.clusterName, false)
		}()

		err = uploadBinaryLog(ctx, sess, db, convertProtoReqToParams(req))
		if err != nil {
			// Need not output error log here, because the errors are logged in the function.
			metrics.IncrementBinlogBackupFailureCountMetrics(s.agent.clusterName, "upload")
			return err
		}

		err = deleteBinaryLog(ctx, db)
		if err != nil {
			log.Error("failed to delete binary logs", map[string]interface{}{
				log.FnError: err,
			})
			metrics.IncrementBinlogBackupFailureCountMetrics(s.agent.clusterName, "delete")
			return err
		}

		durationSeconds := time.Since(startTime).Seconds()
		metrics.UpdateBinlogBackupDurationSecondsMetrics(s.agent.clusterName, durationSeconds)

		return nil
	})

	return &agentrpc.FlushAndBackupBinlogResponse{}, nil
}

// FlushBinlog executes "FLUSH BINARY LOGS;" and delete file if required
func (s *backupBinlogService) FlushBinlog(ctx context.Context, req *agentrpc.FlushBinlogRequest) (*agentrpc.FlushBinlogResponse, error) {
	if req.Token != s.agent.token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if !s.agent.sem.TryAcquire(1) {
		return nil, status.Error(codes.ResourceExhausted, "another request is under processing")
	}
	defer s.agent.sem.Release(1)

	// TODO: change user to AgentUser (need to add appropriate privileges to AgentUser)
	adminPassword := os.Getenv(mocoagent.AdminPasswordEnvName)
	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), mocoagent.AdminUser, adminPassword)
	if err != nil {
		log.Error("failed to connect to database before flush binary logs", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to connect to database before flush binary logs: err=%+v", err)
	}

	err = flushBinaryLogs(ctx, db)
	if err != nil {
		log.Error("failed to flush binary logs", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to flush binary logs: err=%+v", err)
	}

	if req.Delete {
		err = deleteBinaryLog(ctx, db)
		if err != nil {
			log.Error("failed to delete binary logs", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Errorf(codes.Internal, "failed to delete binary logs: err=%+v", err)
		}
	}

	return &agentrpc.FlushBinlogResponse{}, nil
}

func convertProtoReqToParams(req *agentrpc.FlushAndBackupBinlogRequest) *BackupBinaryLogsParams {
	return &BackupBinaryLogsParams{
		BucketHost:      req.BucketHost,
		BackupID:        req.BackupId,
		BucketPort:      int(req.BucketPort),
		BucketName:      req.BucketName,
		BucketRegion:    req.BucketRegion,
		AccessKeyID:     req.AccessKeyId,
		SecretAccessKey: req.SecretAccessKey,
	}
}

func flushBinaryLogs(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `FLUSH BINARY LOGS`)
	return err
}

func getBinaryLogNames(ctx context.Context, db *sqlx.DB) ([]string, error) {
	var binaryLogs []struct {
		LogName   string `db:"Log_name"`
		FileSize  int64  `db:"File_size"`
		Encrypted string `db:"Encrypted"`
	}
	err := db.SelectContext(ctx, &binaryLogs, "SHOW BINARY LOGS")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, binlog := range binaryLogs {
		names = append(names, binlog.LogName)
	}
	sort.Strings(names)
	return names, nil
}

func deleteBinaryLog(ctx context.Context, db *sqlx.DB) error {
	binlogNames, err := getBinaryLogNames(ctx, db)
	if err != nil {
		return err
	}
	latest := binlogNames[len(binlogNames)-1]
	_, err = db.ExecContext(ctx, `PURGE BINARY LOGS TO ?`, latest)
	return err
}

func uploadBinaryLog(ctx context.Context, sess *session.Session, db *sqlx.DB, params *BackupBinaryLogsParams) error {
	uploader := s3manager.NewUploader(sess)

	var binlogBasename string
	err := db.GetContext(ctx, &binlogBasename, `select @@log_bin_basename`)
	if err != nil {
		log.Error("failed to get binary log basename", map[string]interface{}{
			log.FnError: err,
		})
		return err
	}
	binlogDir := filepath.Dir(binlogBasename)

	// Get binlog file names
	binlogNames, err := getBinaryLogNames(ctx, db)
	if err != nil {
		log.Error("failed to get binary log names", map[string]interface{}{
			log.FnError: err,
		})
		return err
	}
	binlogNames = binlogNames[:len(binlogNames)-1]

	// Upload the binary log files to the object Store.
	var objectKeys []string
	for i, binlog := range binlogNames {
		file, err := os.Open(filepath.Join(binlogDir, binlog))
		if err != nil {
			log.Error("failed to open binary log", map[string]interface{}{
				"filename":  binlog,
				log.FnError: err,
			})
			return err
		}

		reader, writer := io.Pipe()
		go func() {
			gw := gzip.NewWriter(writer)
			defer func() {
				file.Close()
				gw.Close()
				writer.Close()
			}()

			// We need not error handling here.
			// If this copy will fail, the following uploader.Upload() get failed.
			io.Copy(gw, file)
		}()

		objectKey := getBinlogFileObjectKey(params.BackupID, i)
		objectKeys = append(objectKeys, objectKey)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(params.BucketName),
			Key:    aws.String(objectKey),
			Body:   reader,
		})
		if err != nil {
			log.Error("failed to upload binary log", map[string]interface{}{
				"filename":  binlog,
				log.FnError: err,
			})
			return err
		}
		log.Info("success to upload binary log", map[string]interface{}{
			"filename":   binlog,
			"object_url": aws.StringValue(&result.Location),
		})
	}

	// Upload object list
	objectKey := params.BackupID
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(params.BucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader([]byte(strings.Join(objectKeys, "\n"))),
	})
	if err != nil {
		log.Error("failed to upload binary log file list", map[string]interface{}{
			"filename":  objectKey,
			log.FnError: err,
		})
		return err
	}
	log.Info("success to upload binary log file list", map[string]interface{}{
		"filename":   objectKey,
		"object_url": aws.StringValue(&result.Location),
	})

	return nil
}

func getBinlogFileObjectKey(prefix string, index int) string {
	return fmt.Sprintf("%s-%06d", prefix, index)
}
