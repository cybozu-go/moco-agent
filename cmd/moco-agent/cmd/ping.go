package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	credentialConfPathFlag = "credential-conf-path"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Send ping to a MySQL instance",
	Long:  `Send ping to a MySQL instance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := os.Stat(initOnceCompletedPath)
		if err != nil {
			return err
		}

		bin := "mysqladmin"
		args = []string{
			"--defaults-extra-file=" + viper.GetString(credentialConfPathFlag),
			"ping",
		}
		command := exec.Command(bin, args...)
		return command.Run()
	},
}

func init() {
	rootCmd.AddCommand(pingCmd)

	pingCmd.Flags().String(credentialConfPathFlag, agentConfPath, "MySQL config path that including credential to access MySQL instance")
	err := viper.BindPFlags(pingCmd.Flags())
	if err != nil {
		panic(err)
	}
}
