package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/proto"
	"github.com/cybozu-go/moco-agent/server"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/go-sql-driver/mysql"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	grpcDefaultAddr            = ":9080"
	probeDefaultAddr           = ":9081"
	metricsDefaultAddr         = ":8080"
	logRotationScheduleDefault = "*/5 * * * *"
	socketPathDefault          = "/run/mysqld.sock"
)

var config struct {
	address             string
	probeAddress        string
	metricsAddress      string
	connIdleTime        time.Duration
	connectionTimeout   time.Duration
	logRotationSchedule string
	readTimeout         time.Duration
	maxDelayThreshold   time.Duration
	socketPath          string
}

type mysqlLogger struct{}

func (l mysqlLogger) Print(v ...interface{}) {}

var grpcMetrics = grpc_prometheus.NewServerMetrics()

var rootCmd = &cobra.Command{
	Use:   "moco-agent",
	Short: "Agent for MySQL instances managed by MOCO",
	Long:  `Agent for MySQL instances managed by MOCO.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zapLogger, err := zap.NewProduction(zap.AddStacktrace(zapcore.DPanicLevel))
		if err != nil {
			return err
		}
		defer zapLogger.Sync()
		rLogger := zapr.NewLogger(zapLogger)

		// Read required values for agent from ENV
		podName := os.Getenv(mocoagent.PodNameEnvKey)
		if podName == "" {
			return fmt.Errorf("%s is empty", mocoagent.PodNameEnvKey)
		}
		agentPassword := os.Getenv(mocoagent.AgentPasswordEnvKey)
		if agentPassword == "" {
			return fmt.Errorf("%s is empty", mocoagent.AgentPasswordEnvKey)
		}
		clusterName := os.Getenv(mocoagent.ClusterNameEnvKey)
		if clusterName == "" {
			return fmt.Errorf("%s is empty", mocoagent.ClusterNameEnvKey)
		}

		ctx := context.Background()
		err = initializeMySQLForMOCO(ctx, config.socketPath, rLogger.WithName("init"))
		if err != nil {
			return err
		}

		conf := server.MySQLAccessorConfig{
			Host:              podName,
			Port:              mocoagent.MySQLAdminPort,
			Password:          agentPassword,
			ConnMaxIdleTime:   config.connIdleTime,
			ConnectionTimeout: config.connectionTimeout,
			ReadTimeout:       config.readTimeout,
		}
		agent, err := server.New(conf, clusterName, config.socketPath, mocoagent.VarLogPath,
			config.maxDelayThreshold, rLogger.WithName("agent"))
		if err != nil {
			return err
		}
		defer agent.CloseDB()

		mysql.SetLogger(mysqlLogger{})

		registry := prometheus.DefaultRegisterer
		metrics.RegisterMetrics(registry)

		c := cron.New(cron.WithLogger(rLogger.WithName("cron")))
		if _, err := c.AddFunc(config.logRotationSchedule, agent.RotateLog); err != nil {
			rLogger.Error(err, "failed to parse the cron spec", "spec", config.logRotationSchedule)
			return err
		}
		c.Start()
		defer func() {
			ctx := c.Stop()

			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
				rLogger.Info("log rotate job did not finish")
			}
		}()

		lis, err := net.Listen("tcp", config.address)
		if err != nil {
			return err
		}

		registry.MustRegister(grpcMetrics)
		grpcLogger := zapLogger.Named("grpc")
		grpcServer := grpc.NewServer(grpc.UnaryInterceptor(
			grpc_middleware.ChainUnaryServer(
				grpc_ctxtags.UnaryServerInterceptor(),
				grpcMetrics.UnaryServerInterceptor(),
				grpc_zap.UnaryServerInterceptor(grpcLogger),
			),
		))
		proto.RegisterAgentServer(grpcServer, server.NewAgentService(agent))

		// after all services are registered, initialize metrics.
		grpcMetrics.InitializeMetrics(grpcServer)

		// enable server reflection service
		// see https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md
		reflection.Register(grpcServer)

		well.Go(func(ctx context.Context) error {
			return grpcServer.Serve(lis)
		})
		well.Go(func(ctx context.Context) error {
			<-ctx.Done()
			grpcServer.GracefulStop()
			return nil
		})

		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		metricsServ := &well.HTTPServer{
			Server: &http.Server{
				Addr:    config.metricsAddress,
				Handler: metricsMux,
			},
		}
		err = metricsServ.ListenAndServe()
		if err != nil {
			return err
		}

		probeMux := http.NewServeMux()
		probeMux.HandleFunc("/healthz", agent.MySQLDHealth)
		probeMux.HandleFunc("/readyz", agent.MySQLDReady)
		probeServ := &well.HTTPServer{
			Server: &http.Server{
				Addr:    config.probeAddress,
				Handler: probeMux,
			},
		}
		err = probeServ.ListenAndServe()
		if err != nil {
			return err
		}

		if err := well.Wait(); err != nil && !well.IsSignaled(err) {
			return err
		}

		return nil
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&config.address, "address", grpcDefaultAddr, "Listening address and port for gRPC API.")
	fs.StringVar(&config.probeAddress, "probe-address", probeDefaultAddr, "Listening address and port for mysqld health probes.")
	fs.StringVar(&config.metricsAddress, "metrics-address", metricsDefaultAddr, "Listening address and port for metrics.")
	fs.DurationVar(&config.connIdleTime, "max-idle-time", 30*time.Second, "The maximum amount of time a connection may be idle")
	fs.DurationVar(&config.connectionTimeout, "connection-timeout", 5*time.Second, "Dial timeout")
	fs.StringVar(&config.logRotationSchedule, "log-rotation-schedule", logRotationScheduleDefault, "Cron format schedule for MySQL log rotation")
	fs.DurationVar(&config.readTimeout, "read-timeout", 30*time.Second, "I/O read timeout")
	fs.DurationVar(&config.maxDelayThreshold, "max-delay", time.Minute, "Acceptable max commit delay considering as ready")
	fs.StringVar(&config.socketPath, "socket-path", socketPathDefault, "Path of mysqld socket file.")
}

func initializeMySQLForMOCO(ctx context.Context, socketPath string, logger logr.Logger) error {
	var db *sqlx.DB
	st := time.Now()
	for {
		if time.Since(st) > 1*time.Minute {
			return errors.New("giving up connecting mysqld")
		}
		var err error
		db, err = server.GetMySQLConnLocalSocket("root", "", socketPath)
		if err == nil {
			break
		}
		if server.IsAccessDenied(err) {
			// There is no passwordless 'root'@'localhost' account.
			// It means the initialization has been completed.
			return nil
		}

		logger.Error(err, "connecting mysqld failed")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	defer db.Close()

	return server.Init(ctx, db, socketPath)
}
