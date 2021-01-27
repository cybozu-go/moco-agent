package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/server"
	"github.com/cybozu-go/moco/accessor"
	"github.com/cybozu-go/moco/metrics"
	"github.com/cybozu-go/well"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	addressFlag           = "address"
	connMaxLifetimeFlag   = "conn-max-lifetime"
	connectionTimeoutFlag = "connection-timeout"
	readTimeoutFlag       = "read-timeout"
)

type mysqlLogger struct{}

func (l mysqlLogger) Print(v ...interface{}) {
	log.Error("[mysql] "+fmt.Sprint(v...), nil)
}

type promhttpLogger struct{}

func (l promhttpLogger) Println(v ...interface{}) {
	log.Error("[promhttp] "+fmt.Sprint(v...), nil)
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Start MySQL agent service",
	Long:  `Start MySQL agent service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mux := http.NewServeMux()

		podName := os.Getenv(moco.PodNameEnvName)
		if podName == "" {
			return fmt.Errorf("%s is empty", moco.PodNameEnvName)
		}

		buf, err := ioutil.ReadFile(moco.MiscPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read misc password file at %s", moco.MiscPasswordPath)
		}
		miscPassword := strings.TrimSpace(string(buf))

		buf, err = ioutil.ReadFile(moco.DonorPasswordPath)
		if err != nil {
			return fmt.Errorf("cannot read donor password file at %s", moco.DonorPasswordPath)
		}
		donorPassword := strings.TrimSpace(string(buf))

		token := os.Getenv(moco.AgentTokenEnvName)
		if token == "" {
			return fmt.Errorf("%s is empty", moco.AgentTokenEnvName)
		}

		agent := server.New(podName, token,
			miscPassword, donorPassword, moco.ReplicationSourceSecretPath, moco.VarLogPath, moco.MySQLAdminPort,
			&accessor.MySQLAccessorConfig{
				ConnMaxLifeTime:   viper.GetDuration(connMaxLifetimeFlag),
				ConnectionTimeout: viper.GetDuration(connectionTimeoutFlag),
				ReadTimeout:       viper.GetDuration(readTimeoutFlag),
			})
		mux.HandleFunc("/rotate", agent.RotateLog)
		mux.HandleFunc("/clone", agent.Clone)
		mux.HandleFunc("/health", agent.Health)
		mysql.SetLogger(mysqlLogger{})

		registry := prometheus.NewRegistry()
		metrics.RegisterAgentMetrics(registry)
		mux.Handle("/metrics", promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorLog:      promhttpLogger{},
				ErrorHandling: promhttp.ContinueOnError,
			},
		))

		serv := &well.HTTPServer{
			Server: &http.Server{
				Addr:    viper.GetString(addressFlag),
				Handler: mux,
			},
		}

		err = serv.ListenAndServe()
		if err != nil {
			return err
		}
		err = well.Wait()

		if err != nil && !well.IsSignaled(err) {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().String(addressFlag, fmt.Sprintf(":%d", moco.AgentPort), "Listening address and port.")
	agentCmd.Flags().Duration(connMaxLifetimeFlag, 30*time.Minute, "The maximum amount of time a connection may be reused")
	agentCmd.Flags().Duration(connectionTimeoutFlag, 3*time.Second, "Dial timeout")
	agentCmd.Flags().Duration(readTimeoutFlag, 30*time.Second, "I/O read timeout")

	err := viper.BindPFlags(agentCmd.Flags())
	if err != nil {
		panic(err)
	}
}
