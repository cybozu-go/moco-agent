package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/well"
	"github.com/jmoiron/sqlx"
)

// BackupBinaryLogsParams is the paramters for backup binary logs
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
func (a *Agent) FlushAndBackupBinaryLogs(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get(moco.AgentTokenParam)
	if token != a.token {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	params, err := parseBackupBinLogParams(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !a.sem.TryAcquire(1) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:           aws.String(params.BucketRegion),
		Endpoint:         aws.String(fmt.Sprintf("%s:%d", params.BucketHost, params.BucketPort)),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(params.AccessKeyID, params.SecretAccessKey, ""),
	}))

	// prevent re-execution of the same request
	_, err = s3.New(sess).HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(params.BucketName),
		Key:    aws.String(params.BackupID),
	})
	if err == nil {
		a.sem.Release(1)
		w.WriteHeader(http.StatusConflict)
		return
	}
	if strings.HasPrefix(err.Error(), s3.ErrCodeNoSuchKey) {
		a.sem.Release(1)
		internalServerError(w, fmt.Errorf("failed to get objects: %w", err))
		log.Error("failed to get objects", map[string]interface{}{
			log.FnError: err,
		})
		return
	}

	// TODO: change user
	rootPassword := os.Getenv(moco.RootPasswordEnvName)
	db, err := a.acc.Get(fmt.Sprintf("%s:%d", a.mysqlAdminHostname, a.mysqlAdminPort), moco.RootUser, rootPassword)
	if err != nil {
		a.sem.Release(1)
		internalServerError(w, fmt.Errorf("failed to connect to database before flush binary logs: %w", err))
		log.Error("failed to connect to database before flush binary logs", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
		return
	}

	err = flushBinaryLogs(r.Context(), db)
	if err != nil {
		a.sem.Release(1)
		internalServerError(w, fmt.Errorf("failed to flush binary logs: %w", err))
		log.Error("failed to flush binary logs", map[string]interface{}{
			log.FnError: err,
		})
		return
	}

	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {
		defer a.sem.Release(1)

		err = uploadBinaryLog(ctx, sess, db, params)
		if err != nil {
			// Need not output error log here, because the errors are logged in the function.
			return err
		}

		err = deleteBinaryLog(ctx, db)
		if err != nil {
			internalServerError(w, fmt.Errorf("failed to delete binary logs: %w", err))
			log.Error("failed to delete binary logs", map[string]interface{}{
				log.FnError: err,
			})
			return err
		}

		return nil
	})
}

// FlushBinaryLogs executes "FLUSH BINARY LOGS;" and delete file if required
func (a *Agent) FlushBinaryLogs(w http.ResponseWriter, r *http.Request) {
	var err error

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get(moco.AgentTokenParam)
	if token != a.token {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var deleteFlag bool
	if flag := r.URL.Query().Get(mocoagent.FlushBinaryLogDeleteparam); len(flag) > 0 {
		deleteFlag, err = strconv.ParseBool(flag)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if !a.sem.TryAcquire(1) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	defer a.sem.Release(1)

	// TODO: change user
	rootPassword := os.Getenv(moco.RootPasswordEnvName)
	db, err := a.acc.Get(fmt.Sprintf("%s:%d", a.mysqlAdminHostname, a.mysqlAdminPort), moco.RootUser, rootPassword)
	if err != nil {
		internalServerError(w, fmt.Errorf("failed to connect to database before flush binary logs: %w", err))
		log.Error("failed to connect to database before flush binary logs", map[string]interface{}{
			"hostname":  a.mysqlAdminHostname,
			"port":      a.mysqlAdminPort,
			log.FnError: err,
		})
	}

	err = flushBinaryLogs(r.Context(), db)
	if err != nil {
		internalServerError(w, fmt.Errorf("failed to flush binary logs: %w", err))
		log.Error("failed to flush binary logs", map[string]interface{}{
			log.FnError: err,
		})
	}

	if deleteFlag {
		err = deleteBinaryLog(r.Context(), db)
		if err != nil {
			internalServerError(w, fmt.Errorf("failed to delete binary logs: %w", err))
			log.Error("failed to delete binary logs", map[string]interface{}{
				log.FnError: err,
			})
		}
	}
}

func parseBackupBinLogParams(v url.Values) (*BackupBinaryLogsParams, error) {
	port, err := strconv.Atoi(v.Get(mocoagent.BackupBinaryLogBucketPortParam))
	if err != nil {
		return nil, err
	}

	return &BackupBinaryLogsParams{
		BackupID:        v.Get(mocoagent.BackupBinaryLogBackupIDParam),
		BucketHost:      v.Get(mocoagent.BackupBinaryLogBucketHostParam),
		BucketPort:      port,
		BucketName:      v.Get(mocoagent.BackupBinaryLogBucketNameParam),
		BucketRegion:    v.Get(mocoagent.BackupBinaryLogBucketRegionParam),
		AccessKeyID:     v.Get(mocoagent.AccessKeyIDParam),
		SecretAccessKey: v.Get(mocoagent.SecretAccessKeyParam),
	}, nil
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
