package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

const (
	defaultBaseDir = "/usr/local/mysql"
	defaultDataDir = "/var/mysql"
	defaultConfDir = "/etc/mysql-conf.d"

	initializedFile = "moco-initialized"
)

var config struct {
	baseDir string
	dataDir string
	confDir string

	lowerCaseTableNames        int
	visitedLowerCaseTableNames *int

	mysqldLocalhost bool

	podName  string
	baseID   uint32
	podIndex uint32
}

//go:embed my.cnf
var mycnfTmpl string

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "moco-init SERVER_ID_BASE",
	Short: "initialize MySQL",
	Long: `moco-init initializes MySQL data directory and create a
configuration snippet to give instance specific configuration values
such as server_id and admin_address.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		cmd.Flags().Visit(func(f *pflag.Flag) {
			if f.Name == "lower-case-table-names" {
				config.visitedLowerCaseTableNames = &config.lowerCaseTableNames
			}
		})
		return subMain(args[0])
	},
}

func subMain(serverIDBase string) error {
	mysqld, err := exec.LookPath("mysqld")
	if err != nil {
		return err
	}

	config.podName = os.Getenv("POD_NAME")
	if len(config.podName) == 0 {
		return fmt.Errorf("no POD_NAME environment variable")
	}

	fields := strings.Split(config.podName, "-")
	if len(fields) < 2 {
		return fmt.Errorf("bad POD_NAME: %s", config.podName)
	}

	indexUint64, err := strconv.ParseUint(fields[len(fields)-1], 10, 32)
	if err != nil {
		return fmt.Errorf("bad POD_NAME %s", config.podName)
	}
	config.podIndex = uint32(indexUint64)

	baseUint64, err := strconv.ParseUint(serverIDBase, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid server base ID: %s: %w", os.Args[1], err)
	}
	config.baseID = uint32(baseUint64)

	if err := validateFlags(); err != nil {
		return err
	}

	_, err = os.Stat(filepath.Join(config.dataDir, initializedFile))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := initMySQL(mysqld); err != nil {
			return err
		}
	}

	return createConf()
}

func initMySQL(mysqld string) error {
	dataDir := filepath.Join(config.dataDir, "data")
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("failed to remove dir %s: %w", dataDir, err)
	}

	var args []string
	args = append(args, "--basedir="+config.baseDir)
	args = append(args, "--datadir="+dataDir)
	args = append(args, "--initialize-insecure")

	// Set only if lower-case-table-names flag is set.
	if config.visitedLowerCaseTableNames != nil {
		args = append(args, fmt.Sprintf("--lower_case_table_names=%d", config.lowerCaseTableNames))
	}

	cmd := exec.Command(mysqld, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	dotFile := filepath.Join(config.dataDir, "."+initializedFile)
	if err := os.Remove(dotFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	f, err := os.Create(dotFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := unix.Syncfs(int(f.Fd())); err != nil {
		return fmt.Errorf("failed to sync fs: %w", err)
	}

	if err := os.Rename(dotFile, filepath.Join(config.dataDir, initializedFile)); err != nil {
		return err
	}

	g, err := os.OpenFile(config.dataDir, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer g.Close()
	return g.Sync()
}

func createConf() error {
	tmpl := template.Must(template.New("my.cnf").Parse(mycnfTmpl))
	adminAddress := config.podName
	if config.mysqldLocalhost {
		adminAddress = "localhost"
	}
	v := struct {
		ServerID     uint32
		AdminAddress string
	}{
		ServerID:     config.baseID + config.podIndex,
		AdminAddress: adminAddress,
	}

	f, err := os.OpenFile(filepath.Join(config.confDir, "my.cnf"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create my.cnf file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, v); err != nil {
		return err
	}
	return f.Sync()
}

func validateFlags() error {
	// refs: https://dev.mysql.com/doc/refman/8.0/en/identifier-case-sensitivity.html
	switch config.lowerCaseTableNames {
	case 0, 1, 2:
		// noop
	default:
		return errors.New("the value of lower-case-table-names flag must be 0, 1 or 2")
	}

	return nil
}

func init() {
	rootCmd.Flags().StringVar(&config.baseDir, "base-dir", defaultBaseDir, "The base directory for MySQL.")
	rootCmd.Flags().StringVar(&config.dataDir, "data-dir", defaultDataDir, "The data directory for MySQL.  Data will be stored in a subdirectory named 'data'")
	rootCmd.Flags().StringVar(&config.confDir, "conf-dir", defaultConfDir, "The directory where configuration file is created.")
	// On Unix, the default value of lower_case_table_names is 0.
	// https://dev.mysql.com/doc/refman/8.0/en/identifier-case-sensitivity.html
	rootCmd.Flags().IntVar(&config.lowerCaseTableNames, "lower-case-table-names", 0, "The value to pass to the '--lower-case-table-names' flag.")
	rootCmd.Flags().BoolVar(&config.mysqldLocalhost, "mysqld-localhost", false, "If true, bind mysqld admin to localhost instead of pod name")
}
