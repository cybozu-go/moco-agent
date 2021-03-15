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
	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/moco-agent/metrics"
	"github.com/cybozu-go/moco-agent/server"
	"github.com/cybozu-go/moco-agent/server/agentrpc"
	"github.com/cybozu-go/moco/accessor"
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
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	addressFlag             = "address"
	metricsAddressFlag      = "metrics-address"
	connMaxLifetimeFlag     = "conn-max-lifetime"
	connectionTimeoutFlag   = "connection-timeout"
	logRotationScheduleFlag = "log-rotation-schedule"
	readTimeoutFlag         = "read-timeout"
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

		podName := os.Getenv(moco.PodNameEnvName)
		if podName == "" {
			return fmt.Errorf("%s is empty", moco.PodNameEnvName)
		}

		buf, err := os.ReadFile(mocoagent.AgentPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read agent password file at %s", mocoagent.AgentPasswordPath)
		}
		agentPassword := strings.TrimSpace(string(buf))

		buf, err = os.ReadFile(moco.DonorPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read donor password file at %s", moco.DonorPasswordPath)
		}
		donorPassword := strings.TrimSpace(string(buf))

		clusterName := os.Getenv(mocoagent.ClusterNameEnvKey)

		token := os.Getenv(moco.AgentTokenEnvName)
		if token == "" {
			return fmt.Errorf("%s is empty", moco.AgentTokenEnvName)
		}

		agent := server.New(podName, clusterName, token,
			agentPassword, donorPassword, moco.ReplicationSourceSecretPath, moco.VarLogPath, moco.MySQLAdminPort,
			&accessor.MySQLAccessorConfig{
				ConnMaxLifeTime:   viper.GetDuration(connMaxLifetimeFlag),
				ConnectionTimeout: viper.GetDuration(connectionTimeoutFlag),
				ReadTimeout:       viper.GetDuration(readTimeoutFlag),
			})

		mysql.SetLogger(mysqlLogger{})

		registry := prometheus.DefaultRegisterer
		metrics.RegisterMetrics(registry)

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		serv := &well.HTTPServer{
			Server: &http.Server{
				Addr:    viper.GetString(metricsAddressFlag),
				Handler: mux,
			},
		}
		err = serv.ListenAndServe()
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
		healthpb.RegisterHealthServer(grpcServer, server.NewHealthService(agent))
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

		if err := well.Wait(); err != nil && !well.IsSignaled(err) {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().String(addressFlag, fmt.Sprintf(":%d", moco.AgentPort), "Listening address and port for gRPC API.")
	agentCmd.Flags().String(metricsAddressFlag, fmt.Sprintf(":%d", mocoagent.MetricsPort), "Listening address and port for metrics.")
	agentCmd.Flags().Duration(connMaxLifetimeFlag, 30*time.Minute, "The maximum amount of time a connection may be reused")
	agentCmd.Flags().Duration(connectionTimeoutFlag, 3*time.Second, "Dial timeout")
	agentCmd.Flags().String(logRotationScheduleFlag, "*/5 * * * *", "Cron format schedule for MySQL log rotation")
	agentCmd.Flags().Duration(readTimeoutFlag, 30*time.Second, "I/O read timeout")

	err := viper.BindPFlags(agentCmd.Flags())
	if err != nil {
		panic(err)
	}
}
