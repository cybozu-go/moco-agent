package server

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/initialize"
	"github.com/cybozu-go/moco-agent/server/proto"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/well"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewCloneService creates a new CloneServiceServer
func NewCloneService(agent *Agent) proto.CloneServiceServer {
	return &cloneService{
		agent: agent,
	}
}

type cloneService struct {
	proto.UnimplementedCloneServiceServer
	agent *Agent
}

type cloneParams struct {
	donorHostName string
	donorPort     int
	donorUser     string
	donorPassword string
	initUser      string
	initPassword  string
}

func (s *cloneService) Clone(ctx context.Context, req *proto.CloneRequest) (*proto.CloneResponse, error) {
	if req.GetToken() != s.agent.token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	params, err := gatherParams(req, s.agent)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if !s.agent.sem.TryAcquire(1) {
		return nil, status.Error(codes.ResourceExhausted, "another request is under processing")
	}

	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), moco.MiscUser, s.agent.miscUserPassword)
	if err != nil {
		log.Error("failed to connect to database before getting MySQL primary status", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to connect to database before getting MySQL primary status: hostname=%s, port=%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort)
	}

	primaryStatus, err := accessor.GetMySQLPrimaryStatus(ctx, db)
	if err != nil {
		s.agent.sem.Release(1)
		log.Error("failed to get MySQL primary status", map[string]interface{}{
			"hostname":  s.agent.mysqlAdminHostname,
			"port":      s.agent.mysqlAdminPort,
			log.FnError: err,
		})
		return nil, status.Errorf(codes.Internal, "failed to get MySQL primary status: %+v", err)
	}

	gtid := primaryStatus.ExecutedGtidSet
	if gtid != "" {
		s.agent.sem.Release(1)
		log.Error("recipient is not empty", map[string]interface{}{
			"gtid": gtid,
		})
		return nil, status.Errorf(codes.FailedPrecondition, "recipient is not empty: gtid=%s", gtid)
	}

	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {
		defer s.agent.sem.Release(1)
		err := s.agent.clone(ctx, params.donorUser, params.donorPassword, params.donorHostName, params.donorPort)
		if err != nil {
			return err
		}

		if req.GetExternal() {
			err := waitBootstrap(ctx, params.initUser, params.initPassword)
			if err != nil {
				log.Error("mysqld didn't boot up after cloning from external", map[string]interface{}{
					"hostname":  s.agent.mysqlAdminHostname,
					"port":      s.agent.mysqlAdminPort,
					log.FnError: err,
				})
				return err
			}
			err = initialize.RestoreUsers(ctx, passwordFilePath, miscConfPath, params.initUser, &params.initPassword, os.Getenv(moco.PodIPEnvName))
			if err != nil {
				log.Error("failed to initialize after clone", map[string]interface{}{
					"hostname":  s.agent.mysqlAdminHostname,
					"port":      s.agent.mysqlAdminPort,
					log.FnError: err,
				})
				return err
			}
			err = initialize.ShutdownInstance(ctx, passwordFilePath)
			if err != nil {
				log.Error("failed to shutdown mysqld after clone", map[string]interface{}{
					"hostname":  s.agent.mysqlAdminHostname,
					"port":      s.agent.mysqlAdminPort,
					log.FnError: err,
				})
				return err
			}
		}
		return nil
	})

	return &proto.CloneResponse{}, nil
}

func gatherParams(req *proto.CloneRequest, agent *Agent) (*cloneParams, error) {
	if !req.GetExternal() {
		donorHostName := req.GetDonorHost()
		if len(donorHostName) <= 0 {
			return nil, errors.New("invalid donor host name")
		}

		donorPort := req.GetDonorPort()
		if donorPort <= 0 {
			return nil, errors.New("invalid donor port")
		}

		return &cloneParams{
			donorHostName: donorHostName,
			donorPort:     int(donorPort),
			donorUser:     moco.CloneDonorUser,
			donorPassword: agent.donorUserPassword,
		}, nil
	}

	rawDonorHostName, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourcePrimaryHostKey)
	if err != nil {
		return nil, errors.New("cannot read donor host from Secret file")
	}

	rawDonorPort, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourcePrimaryPortKey)
	if err != nil {
		return nil, errors.New("cannot read donor port from Secret file")
	}
	donorPort, err := strconv.Atoi(string(rawDonorPort))
	if err != nil {
		return nil, errors.New("cannot convert donor port to int")
	}

	rawDonorUser, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourceCloneUserKey)
	if err != nil {
		return nil, errors.New("cannot read donor user from Secret file")
	}

	rawDonorPassword, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourceClonePasswordKey)
	if err != nil {
		return nil, errors.New("cannot read donor password from Secret file")
	}

	rawInitUser, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourceInitAfterCloneUserKey)
	if err != nil {
		return nil, errors.New("cannot read init user from Secret file")
	}

	rawInitPassword, err := ioutil.ReadFile(agent.replicationSourceSecretPath + "/" + moco.ReplicationSourceInitAfterClonePasswordKey)
	if err != nil {
		return nil, errors.New("cannot read init password from Secret file")
	}

	return &cloneParams{
		donorHostName: string(rawDonorHostName),
		donorPort:     donorPort,
		donorUser:     string(rawDonorUser),
		donorPassword: string(rawDonorPassword),
		initUser:      string(rawInitUser),
		initPassword:  string(rawInitPassword),
	}, nil
}
