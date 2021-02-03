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
	"sort"
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

		err = uploadBinaryLog(r.Context(), db, params)
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

func getBinaryLogNames(ctx context.Context, db *sqlx.DB) ([]string, error) {
	var binaryLogs []struct {
		LogName string `db:"Log_name"`
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
	_, err = db.ExecContext(ctx, `PURGE BINARY LOGS TO $1`, latest)
	return err
}

func uploadBinaryLog(ctx context.Context, db *sqlx.DB, params *BackupBinaryLogParams) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:           aws.String("neco"),
		Endpoint:         aws.String(fmt.Sprintf("%s:%d", params.BucketHost, params.BucketPort)),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(params.AccessKeyID, params.SecretAccessKey, ""),
	}))
	uploader := s3manager.NewUploader(sess)

	var binlogBasename string
	err := db.GetContext(ctx, &binlogBasename, `select @@log_bin_basename`)
	if err != nil {
		return err
	}
	binlogDir := filepath.Dir(binlogBasename)

	// Get binlog file names
	binlogNames, err := getBinaryLogNames(ctx, db)
	if err != nil {
		return err
	}
	binlogNames = binlogNames[:len(binlogNames)-1]

	// Upload the binary log files to the object Store.
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

	return nil
}
