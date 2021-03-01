package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/well"
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

	// TODO: change user
	rootPassword := os.Getenv(moco.RootPasswordEnvName)
	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), moco.RootUser, rootPassword)
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
	startTime := time.Now()

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
		defer s.agent.sem.Release(1)

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

	// TODO: change user
	rootPassword := os.Getenv(moco.RootPasswordEnvName)
	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), moco.RootUser, rootPassword)
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
