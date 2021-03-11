package server

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/initialize"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/well"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewCloneService creates a new CloneServiceServer
func NewCloneService(agent *Agent) agentrpc.CloneServiceServer {
	return &cloneService{
		agent: agent,
	}
}

type cloneService struct {
	agentrpc.UnimplementedCloneServiceServer
	agent *Agent
}

const timeoutDuration = 120 * time.Second

var (
	passwordFilePath = filepath.Join(moco.TmpPath, "moco-root-password")
	agentConfPath    = filepath.Join(mocoagent.MySQLDataPath, "agent.cnf")
)

type cloneParams struct {
	donorHostName string
	donorPort     int
	donorUser     string
	donorPassword string
	initUser      string
	initPassword  string
}

func (s *cloneService) Clone(ctx context.Context, req *agentrpc.CloneRequest) (*agentrpc.CloneResponse, error) {
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

	db, err := s.agent.acc.Get(fmt.Sprintf("%s:%d", s.agent.mysqlAdminHostname, s.agent.mysqlAdminPort), mocoagent.AgentUser, s.agent.agentUserPassword)
	if err != nil {
		s.agent.sem.Release(1)
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

	metrics.IncrementCloneCountMetrics(s.agent.clusterName)

	startTime := time.Now()
	metrics.SetCloneInProgressMetrics(s.agent.clusterName, true)
	env := well.NewEnvironment(context.Background())
	env.Go(func(ctx context.Context) error {

		defer func() {
			s.agent.sem.Release(1)
			metrics.SetCloneInProgressMetrics(s.agent.clusterName, false)
		}()

		err := clone(ctx, db, params.donorUser, params.donorPassword, params.donorHostName, params.donorPort, s.agent)
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
			err = initialize.RestoreUsers(ctx, passwordFilePath, agentConfPath, params.initUser, &params.initPassword)
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

		durationSeconds := time.Since(startTime).Seconds()
		metrics.UpdateCloneDurationSecondsMetrics(s.agent.clusterName, durationSeconds)
		return nil
	})

	return &agentrpc.CloneResponse{}, nil
}

func gatherParams(req *agentrpc.CloneRequest, agent *Agent) (*cloneParams, error) {
	var res *cloneParams

	if !req.GetExternal() {
		donorHostName := req.GetDonorHost()
		if len(donorHostName) <= 0 {
			return nil, errors.New("invalid donor host name")
		}

		donorPort := req.GetDonorPort()
		if donorPort <= 0 {
			return nil, errors.New("invalid donor port")
		}

		res = &cloneParams{
			donorHostName: donorHostName,
			donorPort:     int(donorPort),
			donorUser:     mocoagent.CloneDonorUser,
			donorPassword: agent.donorUserPassword,
		}
	} else {
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

		res = &cloneParams{
			donorHostName: string(rawDonorHostName),
			donorPort:     donorPort,
			donorUser:     string(rawDonorUser),
			donorPassword: string(rawDonorPassword),
			initUser:      string(rawInitUser),
			initPassword:  string(rawInitPassword),
		}
	}

	return res, nil
}

func clone(ctx context.Context, db *sqlx.DB, donorUser, donorPassword, donorHostName string, donorPort int, a *Agent) error {
	_, err := db.ExecContext(ctx, `CLONE INSTANCE FROM ?@?:? IDENTIFIED BY ?`, donorUser, donorHostName, donorPort, donorPassword)

	// After cloning, the recipient MySQL server instance is restarted (stopped and started) automatically.
	// And then the "ERROR 3707" (Restart server failed) occurs. This error does not indicate a cloning failure.
	// So checking the error number here.
	if err != nil && !strings.HasPrefix(err.Error(), "Error 3707") {
		metrics.IncrementCloneFailureCountMetrics(a.clusterName)

		log.Error("failed to exec mysql CLONE", map[string]interface{}{
			"donor_hostname": donorHostName,
			"donor_port":     donorPort,
			"hostname":       a.mysqlAdminHostname,
			"port":           a.mysqlAdminPort,
			log.FnError:      err,
		})
		return err
	}

	log.Info("success to exec mysql CLONE", map[string]interface{}{
		"donor_hostname": donorHostName,
		"donor_port":     donorPort,
		"hostname":       a.mysqlAdminHostname,
		"port":           a.mysqlAdminPort,
		log.FnError:      err,
	})
	return nil
}

func waitBootstrap(ctx context.Context, user, password string) error {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "unix"
	conf.Addr = "/var/run/mysqld/mysqld.sock"
	conf.InterpolateParams = true
	uri := conf.FormatDSN()

	ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			_, err := sqlx.Connect("mysql", uri)
			if err == nil {
				return nil
			}
		}
	}
}
