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
	grpcListenPort          = 9080
	probeListenPort         = 9081
	metricsListenPort       = 8080
)

type mysqlLogger struct{}

func (l mysqlLogger) Print(v ...interface{}) {
	log.Error("[mysql] "+fmt.Sprint(v...), nil)
}

var agentCmd = &cobra.Command{
	Use:   "server",
	Short: "Start MySQL agent service",
	Long:  `Start MySQL agent service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zapLogger, err := zap.NewDevelopment()
		if err != nil {
			return err
		}
		defer zapLogger.Sync()

		podName := os.Getenv(mocoagent.PodNameEnvKey)
		if podName == "" {
			return fmt.Errorf("%s is empty", mocoagent.PodNameEnvKey)
		}

		buf, err := os.ReadFile(mocoagent.AgentPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read agent password file at %s", mocoagent.AgentPasswordPath)
		}
		agentPassword := strings.TrimSpace(string(buf))

		buf, err = os.ReadFile(mocoagent.DonorPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read donor password file at %s", mocoagent.DonorPasswordPath)
		}
		donorPassword := strings.TrimSpace(string(buf))

		clusterName := os.Getenv(mocoagent.ClusterNameEnvKey)

		socketPath := os.Getenv(mocoagent.MySQLSocketPathEnvKey)
		if socketPath == "" {
			socketPath = mocoagent.MySQLSocketDefaultPath
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

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().String(addressFlag, fmt.Sprintf(":%d", grpcListenPort), "Listening address and port for gRPC API.")
	agentCmd.Flags().String(probeAddressFlag, fmt.Sprintf(":%d", probeListenPort), "Listening address and port for mysqld health probes.")
	agentCmd.Flags().String(metricsAddressFlag, fmt.Sprintf(":%d", metricsListenPort), "Listening address and port for metrics.")
	agentCmd.Flags().Duration(connMaxLifetimeFlag, 30*time.Minute, "The maximum amount of time a connection may be reused")
	agentCmd.Flags().Duration(connectionTimeoutFlag, 3*time.Second, "Dial timeout")
	agentCmd.Flags().String(logRotationScheduleFlag, "*/5 * * * *", "Cron format schedule for MySQL log rotation")
	agentCmd.Flags().Duration(readTimeoutFlag, 30*time.Second, "I/O read timeout")
	agentCmd.Flags().Duration(maxDelayThreshold, time.Minute, "Acceptable max commit delay considering as ready")

	err := viper.BindPFlags(agentCmd.Flags())
	if err != nil {
		panic(err)
	}
}
