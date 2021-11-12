package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/proto"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/go-sql-driver/mysql"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const cloneBootstrapTimeout = 10 * time.Minute

func (s agentService) Clone(ctx context.Context, req *proto.CloneRequest) (*proto.CloneResponse, error) {
	if err := s.agent.Clone(ctx, req); err != nil {
		return nil, err
	}
	return &proto.CloneResponse{}, nil
}

func (a *Agent) Clone(ctx context.Context, req *proto.CloneRequest) error {
	select {
	case a.cloneLock <- struct{}{}:
	default:
		return status.Error(codes.ResourceExhausted, "another request is undergoing")
	}
	defer func() { <-a.cloneLock }()

	logger := zapr.NewLogger(ctxzap.Extract(ctx))

	primaryStatus, err := a.GetMySQLPrimaryStatus(ctx)
	if err != nil {
		logger.Error(err, "failed to get MySQL primary status")
		return status.Errorf(codes.Internal, "failed to get MySQL primary status: %+v", err)
	}

	gtid := primaryStatus.ExecutedGtidSet
	if gtid != "" {
		logger.Error(err, "recipient is not empty")
		return status.Errorf(codes.FailedPrecondition, "recipient is not empty: gtid=%s", gtid)
	}

	startTime := time.Now()
	metrics.CloneCount.Inc()
	metrics.CloneInProgress.Set(1)
	defer func() {
		metrics.CloneInProgress.Set(0)
		metrics.CloneDurationSeconds.Observe(time.Since(startTime).Seconds())
	}()

	donorAddr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	if _, err := a.db.ExecContext(ctx, `SET GLOBAL clone_valid_donor_list = ?`, donorAddr); err != nil {
		return status.Errorf(codes.Internal, "failed to set clone_valid_donor_list: %+v", err)
	}

	// To clone, the connection should not set timeout values.
	cloneDB, err := GetMySQLConnLocalSocket(mocoagent.AgentUser, a.config.Password, a.mysqlSocketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to mysqld through %s: %w", a.mysqlSocketPath, err)
	}
	defer cloneDB.Close()

	logger.Info("start cloning instance", "donor", donorAddr)
	_, err = cloneDB.Exec(`CLONE INSTANCE FROM ?@?:? IDENTIFIED BY ?`, req.User, req.Host, req.Port, req.Password)
	if err != nil && !IsRestartFailed(err) {
		metrics.CloneFailureCount.Inc()

		logger.Error(err, "failed to exec CLONE INSTANCE", "donor", donorAddr)
		return err
	}

	logger.Info("cloning finished successfully", "donor", donorAddr)

	time.Sleep(100 * time.Millisecond)

	if err := waitBootstrap(req.InitUser, req.InitPassword, a.mysqlSocketPath, cloneBootstrapTimeout, logger); err != nil {
		logger.Error(err, "mysqld didn't boot up after cloning from external")
		return err
	}

	initDB, err := GetMySQLConnLocalSocket(req.InitUser, req.InitPassword, a.mysqlSocketPath)
	if err != nil {
		logger.Error(err, "failed to connect to mysqld after bootstrap")
		return err
	}
	defer initDB.Close()

	if err := InitExternal(context.Background(), initDB); err != nil {
		logger.Error(err, "failed to initialize after clone")
		return err
	}

	return nil
}

func waitBootstrap(user, password, socket string, timeout time.Duration, logger logr.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		db, err := GetMySQLConnLocalSocket(user, password, socket)
		if err == nil {
			db.Close()
			return nil
		}

		if err != nil {
			logger.Error(err, "connection failed")
		}
	}
}

func IsRestartFailed(err error) bool {
	var merr *mysql.MySQLError
	if errors.As(err, &merr) && merr.Number == 3707 {
		return true
	}

	return false
}
