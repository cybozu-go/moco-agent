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
)

var (
	passwordFilePath = filepath.Join(moco.TmpPath, "moco-root-password")
	agentConfPath    = filepath.Join(moco.MySQLDataPath, "agent.cnf")
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize MySQL instance",
	Long: fmt.Sprintf(`Initialize MySQL instance managed by MOCO.
	If %s already exists, this command does nothing.
	`, initOnceCompletedPath),
	RunE: func(cmd *cobra.Command, args []string) error {
		well.Go(func(ctx context.Context) error {
			log.Info("start initialization", nil)
			serverIDBase := viper.GetUint32(serverIDBaseFlag)

			err := initialize.InitializeOnce(ctx, initOnceCompletedPath, passwordFilePath, agentConfPath, serverIDBase)
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

func init() {
	rootCmd.AddCommand(initCmd)

	// ordinal should be increased by 1000 as default because the case server-id is 0 is not suitable for the replication purpose
	initCmd.Flags().Uint32(serverIDBaseFlag, 1000, "Base value of server-id.")
	initCmd.Flags().String(moco.PodNameFlag, "", "Pod Name created by StatefulSet")
	err := viper.BindPFlags(initCmd.Flags())
	if err != nil {
		panic(err)
	}
}
