package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/well"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/gomega"
)

const (
	// Dummy password for MySQL users which are managed by MOCO.
	adminUserPassword       = "adminpassword"
	agentUserPassword       = "agentpassword"
	replicationUserPassword = "replpassword"
	cloneDonorUserPassword  = "clonepassword"
	exporterPassword        = "exporter"
	backupPassword          = "backup"
	readOnlyPassword        = "readonly"
	writablePassword        = "writable"

	// Docker network name for test.
	networkName = "moco-agent-test-net"
)

var tmpBaseDir = path.Join(os.TempDir(), "moco-agent-test-server")

var MySQLVersion = func() string {
	if ver := os.Getenv("MYSQL_VERSION"); ver == "" {
		os.Setenv("MYSQL_VERSION", "8.0.28")
	}
	return os.Getenv("MYSQL_VERSION")
}()

var testMycnf = map[string]string{
	"character_set_server":     "utf8mb4",
	"collation_server":         "utf8mb4_unicode_ci",
	"default_time_zone":        "+0:00",
	"disabled_storage_engines": "MyISAM",
	"enforce_gtid_consistency": "ON",
	"gtid_mode":                "ON",
}

func socketDir(name string) string {
	return filepath.Join(tmpBaseDir, name, "socket")
}

func dataDir(name string) string {
	return filepath.Join(tmpBaseDir, name, "data")
}

func StartMySQLD(name string, port int, serverID int) {
	ctx := context.Background()

	if serverID == 0 {
		serverID = 1
	}

	// In order to delete MySQL's data directory after testing, run MySQL as the current user.
	// If a user is not specified, the data files are created as 10000:10000 (the default user of
	// ghcr.io/cybozu-go/moco/mysql), and the current user cannot delete the files.
	currentUser, err := user.Current()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	socketDir := socketDir(name)
	ExpectWithOffset(1, os.MkdirAll(socketDir, 0755)).NotTo(HaveOccurred())
	ExpectWithOffset(1, os.Chmod(socketDir, 0777)).NotTo(HaveOccurred())

	dataDir := dataDir(name)
	ExpectWithOffset(1, os.RemoveAll(dataDir)).NotTo(HaveOccurred()) // If files exist in data dir, initialization fails.
	ExpectWithOffset(1, os.MkdirAll(dataDir, 0755)).NotTo(HaveOccurred())
	ExpectWithOffset(1, os.Chmod(dataDir, 0777)).NotTo(HaveOccurred())

	initArgs := []string{
		"run", "--name", name + "-init", "--rm",
		"--user", currentUser.Uid,
		"-v", dataDir + ":/var/lib/mysql",
		"ghcr.io/cybozu-go/moco/mysql:" + MySQLVersion,
		"--initialize-insecure",
		"--datadir", "/var/lib/mysql",
	}

	cmd := well.CommandContext(ctx, "docker", initArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	ExpectWithOffset(1, cmd.Run()).NotTo(HaveOccurred())

	args := []string{
		"run", "--name", name, "-d", "--restart=always",
		"--user", currentUser.Uid,
		"--network=" + networkName,
		"-e", "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		"-e", "MYSQL_ROOT_HOST=localhost",
		"-p", fmt.Sprintf("%d:3306", port),
		"-v", socketDir + ":/var/run/mysqld",
		"-v", dataDir + ":/var/lib/mysql",
		"ghcr.io/cybozu-go/moco/mysql:" + MySQLVersion,
		fmt.Sprintf("--server-id=%d", serverID),
		"--socket", "/var/run/mysqld/mysqld.sock",
		"--datadir", "/var/lib/mysql",
	}
	for k, v := range testMycnf {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}

	cmd = well.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	ExpectWithOffset(1, cmd.Run()).NotTo(HaveOccurred())

	socket := filepath.Join(socketDir, "mysqld.sock")
	var db *sqlx.DB
	EventuallyWithOffset(1, func() error {
		var err error
		db, err = GetMySQLConnLocalSocket("root", "", socket)
		if err != nil {
			return err
		}
		ts := time.Now()
		for {
			if time.Since(ts) > 20*time.Second {
				return nil
			}

			_, err := db.Exec(`SELECT @@super_read_only`)
			if err != nil {
				db.Close()
				return err
			}
			time.Sleep(1 * time.Second)
		}
	}).Should(Succeed())
	defer db.Close()

	_, err = db.Exec("SET GLOBAL read_only=ON")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	_, err = db.Exec("SET GLOBAL super_read_only=ON")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	envPasswords := map[string]string{
		mocoagent.AdminPasswordEnvKey:       adminUserPassword,
		mocoagent.AgentPasswordEnvKey:       agentUserPassword,
		mocoagent.ReplicationPasswordEnvKey: replicationUserPassword,
		mocoagent.CloneDonorPasswordEnvKey:  cloneDonorUserPassword,
		mocoagent.ExporterPasswordKey:       exporterPassword,
		mocoagent.BackupPasswordKey:         backupPassword,
		mocoagent.ReadOnlyPasswordEnvKey:    readOnlyPassword,
		mocoagent.WritablePasswordEnvKey:    writablePassword,
	}
	for k, v := range envPasswords {
		os.Setenv(k, v)
	}

	err = Init(context.Background(), db, socket)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func StopAndRemoveMySQLD(name string) {
	err := exec.Command("docker", "inspect", name).Run()
	if err != nil {
		return
	}
	cmd := exec.Command("docker", "kill", name)
	ExpectWithOffset(1, cmd.Run()).NotTo(HaveOccurred())
	cmd = exec.Command("docker", "rm", name)
	ExpectWithOffset(1, cmd.Run()).NotTo(HaveOccurred())
}

func CreateNetwork() {
	err := exec.Command("docker", "network", "create", networkName).Run()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	EventuallyWithOffset(1, func() error {
		cmd := exec.Command("docker", "network", "inspect", networkName)
		return cmd.Run()
	}).Should(Succeed())
}

func RemoveNetwork() {
	err := exec.Command("docker", "network", "inspect", networkName).Run()
	if err != nil {
		return
	}

	err = exec.Command("docker", "network", "rm", networkName).Run()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}
