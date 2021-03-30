package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco-agent/proto"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const cloneBootstrapTimeout = 120 * time.Second

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

	primaryStatus, err := a.GetMySQLPrimaryStatus(ctx)
	if err != nil {
		log.Error("failed to get MySQL primary status", map[string]interface{}{
			log.FnError: err,
		})
		return status.Errorf(codes.Internal, "failed to get MySQL primary status: %+v", err)
	}

	gtid := primaryStatus.ExecutedGtidSet
	if gtid != "" {
		log.Error("recipient is not empty", map[string]interface{}{
			"gtid": gtid,
		})
		return status.Errorf(codes.FailedPrecondition, "recipient is not empty: gtid=%s", gtid)
	}

	startTime := time.Now()
	a.cloneCount.Inc()
	a.cloneInProgress.Set(1)
	defer func() {
		a.cloneInProgress.Set(0)
		a.cloneDurationSeconds.Observe(time.Since(startTime).Seconds())
	}()

	donorAddr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	if _, err := a.db.ExecContext(ctx, `SET GLOBAL clone_valid_donor_list = ?`, donorAddr); err != nil {
		return status.Errorf(codes.Internal, "failed to set clone_valid_donor_list: %+v", err)
	}

	_, err = a.db.ExecContext(ctx, `CLONE INSTANCE FROM ?@?:? IDENTIFIED BY ?`, req.User, req.Host, req.Port, req.Password)
	if err != nil && !IsRestartFailed(err) {
		a.cloneFailureCount.Inc()

		log.Error("failed to exec CLONE INSTANCE", map[string]interface{}{
			"donor":     donorAddr,
			log.FnError: err,
		})
		return err
	}

	log.Info("cloning finished successfully", map[string]interface{}{
		"donor": donorAddr,
	})

	time.Sleep(100 * time.Millisecond)

	if err := waitBootstrap(ctx, req.InitUser, req.InitPassword, a.mysqlSocketPath, cloneBootstrapTimeout); err != nil {
		log.Error("mysqld didn't boot up after cloning from external", map[string]interface{}{
			log.FnError: err,
		})
		return err
	}

	initDB, err := GetMySQLConnLocalSocket(req.InitUser, req.InitPassword, a.mysqlSocketPath)
	if err != nil {
		log.Error("failed to connect to mysqld after bootstrap", map[string]interface{}{
			log.FnError: err,
		})
		return err
	}

	if err := InitExternal(ctx, initDB); err != nil {
		log.Error("failed to initialize after clone", map[string]interface{}{
			log.FnError: err,
		})
		return err
	}

	return nil
}

func waitBootstrap(ctx context.Context, user, password, socket string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
			log.Info("connection failed", map[string]interface{}{
				log.FnError: err,
			})
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
