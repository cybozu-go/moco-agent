package server

import (
	"context"
	"fmt"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/server/proto"
	"github.com/cybozu-go/moco/accessor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewHealthService creates a new HealthServiceServer
func NewHealthService(agent *Agent) proto.HealthServiceServer {
	return &healthService{
		agent: agent,
	}
}

type healthService struct {
	proto.UnimplementedHealthServiceServer
	agent *Agent
}

func (s *healthService) Health(ctx context.Context, req *proto.HealthRequest) (*proto.HealthResponse, error) {
	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), moco.MiscUser, s.agent.miscUserPassword)
	if err != nil {
		log.Error("failed to connect to database before health check", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to connect to database before health check: err=%v", err)
	}

	replicaStatus, err := accessor.GetMySQLReplicaStatus(ctx, db)
	if err != nil {
		log.Error("failed to get replica status", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to get replica status: err=%v", err)
	}

	cloneStatus, err := accessor.GetMySQLCloneStateStatus(ctx, db)
	if err != nil {
		log.Error("failed to get clone status", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to get clone status: err=%v", err)
	}

	var res proto.HealthResponse

	if replicaStatus != nil && replicaStatus.LastIoErrno != 0 {
		res.IsOutOfSynced = true
	}

	if cloneStatus.State.Valid && cloneStatus.State.String != moco.CloneStatusCompleted {
		res.IsUnderCloning = true
	}

	res.Ok = !res.IsOutOfSynced && !res.IsUnderCloning
	return &res, nil
}
