package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cybozu-go/log"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/initialize"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/well"
	"github.com/go-sql-driver/mysql"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	addressFlag             = "address"
	probeAddressFlag        = "probe-address"
	metricsAddressFlag      = "metrics-address"
	connMaxLifetimeFlag     = "conn-max-lifetime"
	connectionTimeoutFlag   = "connection-timeout"
	logRotationScheduleFlag = "log-rotation-schedule"
	readTimeoutFlag         = "read-timeout"
	maxDelayThreshold       = "max-delay"
	grpcDefaultAddr         = ":9080"
	probeDefaultAddr        = ":9081"
	metricsDefaultAddr      = ":8080"
)

type mysqlLogger struct{}

func (l mysqlLogger) Print(v ...interface{}) {
	log.Error("[mysql] "+fmt.Sprint(v...), nil)
}

var (
	rootCmd = &cobra.Command{
		Use:   "moco-agent",
		Short: "Agent for MySQL instances managed by MOCO",
		Long:  `Agent for MySQL instances managed by MOCO.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			zapLogger, err := zap.NewDevelopment()
			if err != nil {
				return err
			}
			defer zapLogger.Sync()

			// Read required values for agent from ENV
			podName := os.Getenv(mocoagent.PodNameEnvKey)
			if podName == "" {
				return fmt.Errorf("%s is empty", mocoagent.PodNameEnvKey)
			}
			agentPassword := os.Getenv(mocoagent.AgentPasswordEnvKey)
			if agentPassword == "" {
				return fmt.Errorf("%s is empty", mocoagent.AgentPasswordEnvKey)
			}
			donorPassword := os.Getenv(mocoagent.CloneDonorPasswordEnvKey)
			if donorPassword == "" {
				return fmt.Errorf("%s is empty", mocoagent.CloneDonorPasswordEnvKey)
			}
			clusterName := os.Getenv(mocoagent.ClusterNameEnvKey)
			if clusterName == "" {
				return fmt.Errorf("%s is empty", mocoagent.PodNameEnvKey)
			}
			socketPath := os.Getenv(mocoagent.MySQLSocketPathEnvKey)
			if socketPath == "" {
				socketPath = mocoagent.MySQLSocketDefaultPath
			}

			// TODO: How should we handle the context?
			ctx := context.Background()
			err = initializeMySQLForMOCO(ctx, socketPath)
			if err != nil {
				return err
			}

			agent, err := server.New(podName, clusterName,
				agentPassword, donorPassword, mocoagent.ReplicationSourceSecretPath, socketPath, mocoagent.VarLogPath, mocoagent.MySQLAdminPort,
				server.MySQLAccessorConfig{
					ConnMaxLifeTime:   viper.GetDuration(connMaxLifetimeFlag),
					ConnectionTimeout: viper.GetDuration(connectionTimeoutFlag),
					ReadTimeout:       viper.GetDuration(readTimeoutFlag),
				}, viper.GetDuration(maxDelayThreshold))
			if err != nil {
				return err
			}
			defer agent.CloseDB()

			mysql.SetLogger(mysqlLogger{})

			registry := prometheus.DefaultRegisterer
			metrics.RegisterMetrics(registry)

			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())
			metricsServ := &well.HTTPServer{
				Server: &http.Server{
					Addr:    viper.GetString(metricsAddressFlag),
					Handler: metricsMux,
				},
			}
			err = metricsServ.ListenAndServe()
			if err != nil {
				return err
			}

			c := cron.New()
			if _, err := c.AddFunc(viper.GetString(logRotationScheduleFlag), agent.RotateLog); err != nil {
				log.Error("failed to parse the cron spec", map[string]interface{}{
					"spec":      viper.GetString(logRotationScheduleFlag),
					log.FnError: err,
				})
				return err
			}
			c.Start()
			defer func() {
				ctx := c.Stop()

				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					log.Error("log rotate job did not finish", nil)
				}
			}()

			lis, err := net.Listen("tcp", viper.GetString(addressFlag))
			if err != nil {
				return err
			}
			grpcLogger := zapLogger.Named("grpc")
			grpcServer := grpc.NewServer(grpc.UnaryInterceptor(
				grpc_middleware.ChainUnaryServer(
					grpc_ctxtags.UnaryServerInterceptor(),
					grpc_zap.UnaryServerInterceptor(grpcLogger),
				),
			))
			// TODO: gRPC server health check will be implemented
			// healthpb.RegisterHealthServer(grpcServer, server.NewHealthService(agent))
			agentrpc.RegisterCloneServiceServer(grpcServer, server.NewCloneService(agent))
			agentrpc.RegisterBackupBinlogServiceServer(grpcServer, server.NewBackupBinlogService(agent))

			well.Go(func(ctx context.Context) error {
				return grpcServer.Serve(lis)
			})
			well.Go(func(ctx context.Context) error {
				<-ctx.Done()
				grpcServer.GracefulStop()
				return nil
			})

			probeMux := http.NewServeMux()
			probeMux.HandleFunc("/healthz", agent.MySQLDHealth)
			probeMux.HandleFunc("/readyz", agent.MySQLDReady)
			probeServ := &well.HTTPServer{
				Server: &http.Server{
					Addr:    viper.GetString(probeAddressFlag),
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
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().String(addressFlag, grpcDefaultAddr, "Listening address and port for gRPC API.")
	rootCmd.Flags().String(probeAddressFlag, probeDefaultAddr, "Listening address and port for mysqld health probes.")
	rootCmd.Flags().String(metricsAddressFlag, metricsDefaultAddr, "Listening address and port for metrics.")
	rootCmd.Flags().Duration(connMaxLifetimeFlag, 30*time.Minute, "The maximum amount of time a connection may be reused")
	rootCmd.Flags().Duration(connectionTimeoutFlag, 3*time.Second, "Dial timeout")
	rootCmd.Flags().String(logRotationScheduleFlag, "*/5 * * * *", "Cron format schedule for MySQL log rotation")
	rootCmd.Flags().Duration(readTimeoutFlag, 30*time.Second, "I/O read timeout")
	rootCmd.Flags().Duration(maxDelayThreshold, time.Minute, "Acceptable max commit delay considering as ready")

	err := viper.BindPFlags(rootCmd.Flags())
	if err != nil {
		panic(err)
	}
}

func initConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func initializeMySQLForMOCO(ctx context.Context, socketPath string) error {
	db, err := initialize.GetMySQLConnLocalSocket("root", "", socketPath, 20)
	if err != nil {
		return err
	}
	defer db.Close()

	err = initialize.EnsureMOCOUsers(ctx, db)
	if err != nil {
		return err
	}
	// TODO: Install plugins here,
	// like initialize.InstallPlugins(ctx, initDB)
	return initialize.DropLocalRootUser(ctx, db)
}
