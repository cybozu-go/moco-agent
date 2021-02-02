package server

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/well"
)

// BackupBinaryLogParams is the paramters for backup binary logs
type BackupBinaryLogParams struct {
	FileName     string
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

		filepath, err := flushBinaryLogAndGetPath(false)
		if err != nil {

			return err
		}

		err = uploadBinaryLog(params, filepath)
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

		_, err := flushBinaryLogAndGetPath(deleteFlag)
		if err != nil {

			return err
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
		FileName:        v.Get(mocoagent.BackupBinaryLogFileNameParam),
		BucketHost:      v.Get(mocoagent.BackupBinaryLogBucketHostParam),
		BucketPort:      port,
		BucketName:      v.Get(mocoagent.BackupBinaryLogBucketNameParam),
		BucketRegion:    v.Get(mocoagent.BackupBinaryLogBucketRegionParam),
		AccessKeyID:     v.Get(mocoagent.AccessKeyIDParam),
		SecretAccessKey: v.Get(mocoagent.SecretAccessKeyParam),
	}, nil
}

func flushBinaryLogAndGetPath(deleteFlag bool) (string, error) {
	// TODO
	return "", nil
}

func uploadBinaryLog(params *BackupBinaryLogParams, binLogFilePath string) error {
	// TODO
	return nil
}
