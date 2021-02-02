package server

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/well"
	"github.com/jmoiron/sqlx"
)

// BackupBinaryLogParams is the paramters for backup binary logs
type BackupBinaryLogParams struct {
	FilePrefix   string
	BucketHost   string
	BucketPort   int
	BucketName   string
	BucketRegion string

	AccessKeyID     string
	SecretAccessKey string
}

// BackupBinaryLog executes "FLUSH BINARY LOGS;"
// and upload it to the object storage, then delete it
func (a *Agent) BackupBinaryLog(w http.ResponseWriter, r *http.Request) {
	var err error

	if r.Method != http.MethodGet {
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

	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {
		defer a.sem.Release(1)

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
			return err
		}

		err = flushBinaryLog(r.Context(), db)
		if err != nil {

			return err
		}

		err = uploadBinaryLog(params)
		if err != nil {
			return err
		}

		err = deleteBinaryLog(r.Context(), db)
		if err != nil {
			return err
		}

		return nil
	})
}

// FlushBinaryLog executes "FLUSH BINARY LOGS;" and delete file if required
func (a *Agent) FlushBinaryLog(w http.ResponseWriter, r *http.Request) {
	var err error

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get(moco.AgentTokenParam)
	if token != a.token {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	deleteFlag, err := strconv.ParseBool(r.URL.Query().Get(mocoagent.FlushBinaryLogDeleteparam))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !a.sem.TryAcquire(1) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {
		defer a.sem.Release(1)

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
			return err
		}

		err = flushBinaryLog(r.Context(), db)
		if err != nil {
			return err
		}

		if deleteFlag {
			err = deleteBinaryLog(r.Context(), db)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func parseBackupBinLogParams(v url.Values) (*BackupBinaryLogParams, error) {
	port, err := strconv.Atoi(mocoagent.BackupBinaryLogBucketPortParam)
	if err != nil {
		return nil, err
	}

	return &BackupBinaryLogParams{
		FilePrefix:      v.Get(mocoagent.BackupBinaryLogFilePrefixParam),
		BucketHost:      v.Get(mocoagent.BackupBinaryLogBucketHostParam),
		BucketPort:      port,
		BucketName:      v.Get(mocoagent.BackupBinaryLogBucketNameParam),
		BucketRegion:    v.Get(mocoagent.BackupBinaryLogBucketRegionParam),
		AccessKeyID:     v.Get(mocoagent.AccessKeyIDParam),
		SecretAccessKey: v.Get(mocoagent.SecretAccessKeyParam),
	}, nil
}

func flushBinaryLog(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `FLUSH BINARY LOGS`)
	return err
}

func deleteBinaryLog(ctx context.Context, db *sqlx.DB) error {
	// TODO: specify binary log names
	_, err := db.ExecContext(ctx, `PURGE BINARY LOGS`)
	return err
}

func uploadBinaryLog(params *BackupBinaryLogParams) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:           aws.String("neco"),
		Endpoint:         aws.String(fmt.Sprintf("%s:%d", params.BucketHost, params.BucketPort)),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(params.AccessKeyID, params.SecretAccessKey, ""),
	}))
	uploader := s3manager.NewUploader(sess)

	// Get binlog dir
	// show variables like log_bin_basename
	binlogDir := "/var/lib/mysql/"

	// Get binlog file names
	// show binary logs
	binaryLogs := []string{}

	// Upload the binary log files to the object Store.
	for i, filename := range binaryLogs {

		file, err := os.Open(filepath.Join(binlogDir, filename))
		if err != nil {
			log.Error("failed to open binary log", map[string]interface{}{
				"filename":  filename,
				log.FnError: err,
			})
			return err
		}

		reader, writer := io.Pipe()
		go func() {
			gw := gzip.NewWriter(writer)
			io.Copy(gw, file)
			file.Close()
			gw.Close()
			writer.Close()
		}()

		objectKey := fmt.Sprintf("%s-%d", params.FilePrefix, i)
		result, err := uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(params.BucketName),
			Key:    aws.String(objectKey),
			Body:   reader,
		})
		if err != nil {
			log.Error("failed to upload binary log", map[string]interface{}{
				"filename":  filename,
				log.FnError: err,
			})
			return err
		}
		log.Info("success to upload binary log", map[string]interface{}{
			"filename":   filename,
			"object_url": aws.StringValue(&result.Location),
		})
	}

	return nil
}
