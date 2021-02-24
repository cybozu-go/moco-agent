package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco-agent/initialize"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	serverIDBaseFlag = "server-id-base"

	errorLogOutputFlag = "error-log-output"
	slowLogOutputFlag  = "slow-query-log-output"
)

type logOutputType string

const (
	defaultLogOutput logOutputType = "default"
	stdErrLogOutput  logOutputType = "stderr"
	stdOutLogOutput  logOutputType = "stdout"
)

func (t logOutputType) String() string {
	return string(t)
}

func (t logOutputType) SymlinkTarget() string {
	switch t {
	case stdErrLogOutput:
		return "/dev/stderr"
	case stdOutLogOutput:
		return "/dev/stdout"
	case defaultLogOutput:
		return ""
	default:
		return ""
	}
}

var (
	passwordFilePath = filepath.Join(moco.TmpPath, "moco-root-password")
	miscConfPath     = filepath.Join(moco.MySQLDataPath, "misc.cnf")
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize MySQL instance",
	Long: fmt.Sprintf(`Initialize MySQL instance managed by MOCO.
	If %s already exists, this command does nothing.
	`, initOnceCompletedPath),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := validateLogOutputType(viper.GetString(errorLogOutputFlag)); err != nil {
			return fmt.Errorf("%s options validation error: %w", errorLogOutputFlag, err)
		}

		if err := validateLogOutputType(viper.GetString(slowLogOutputFlag)); err != nil {
			return fmt.Errorf("%s options validation error: %w", slowLogOutputFlag, err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		well.Go(func(ctx context.Context) error {
			log.Info("start initialization", nil)

			errLogOutput := logOutputType(viper.GetString(errorLogOutputFlag))
			if len(errLogOutput.SymlinkTarget()) != 0 {
				err := initialize.CreateSymlink(ctx, errLogOutput.SymlinkTarget(), filepath.Join(moco.VarLogPath, moco.MySQLErrorLogName))
				if err != nil {
					log.Error("failed to crate symlink", map[string]interface{}{
						log.FnError: err,
						"source":    filepath.Join(moco.VarLogPath, moco.MySQLErrorLogName),
						"target":    errLogOutput.SymlinkTarget(),
					})
				}
			}

			slowLogOutput := logOutputType(viper.GetString(slowLogOutputFlag))
			if len(slowLogOutput.SymlinkTarget()) != 0 {
				err := initialize.CreateSymlink(ctx, slowLogOutput.SymlinkTarget(), filepath.Join(moco.VarLogPath, moco.MySQLSlowLogName))
				if err != nil {
					log.Error("failed to crate symlink", map[string]interface{}{
						log.FnError: err,
						"source":    filepath.Join(moco.VarLogPath, moco.MySQLSlowLogName),
						"target":    errLogOutput.SymlinkTarget(),
					})
				}
			}

			serverIDBase := viper.GetUint32(serverIDBaseFlag)

			err := initialize.InitializeOnce(ctx, initOnceCompletedPath, passwordFilePath, miscConfPath, serverIDBase)
			if err != nil {
				f, err2 := ioutil.ReadFile("/var/log/mysql/mysql.err")
				if err2 != nil {
					log.Error("failed to read mysql.err", map[string]interface{}{
						log.FnError: err2,
					})
					// original error is more important, so return it
					return err
				}

				fmt.Println(string(f))
				return err
			}

			// Put preparation steps which should be executed at every startup.

			return nil
		})

		well.Stop()
		err := well.Wait()
		if err != nil {
			log.ErrorExit(err)
		}

		return nil
	},
}

// validateLogOutputType verifies that the output is a supported type.
func validateLogOutputType(s string) error {
	switch logOutputType(s) {
	case defaultLogOutput:
	case stdOutLogOutput:
	case stdErrLogOutput:
	default:
		return fmt.Errorf("unsupported types: %s", s)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)

	// ordinal should be increased by 1000 as default because the case server-id is 0 is not suitable for the replication purpose
	initCmd.Flags().Uint32(serverIDBaseFlag, 1000, "Base value of server-id.")
	initCmd.Flags().String(moco.PodNameFlag, "", "Pod Name created by StatefulSet")
	initCmd.Flags().String(moco.PodIPFlag, "", "Pod IP address")
	initCmd.Flags().String(errorLogOutputFlag, "default",
		"Error logs output destination. If default is specified, use the settings in the mysql configuration file. One of: [default|stdout|stderr]")
	initCmd.Flags().String(slowLogOutputFlag, "default",
		"Slow query logs output destination. If default is specified, use the settings in the mysql configuration file. One of: [default|stdout|stderr]")
	err := viper.BindPFlags(initCmd.Flags())
	if err != nil {
		panic(err)
	}
}
