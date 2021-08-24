package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

var socketBaseDir = path.Join(os.TempDir(), "moco-agent-test-server")

var MySQLVersion = func() string {
	if ver := os.Getenv("MYSQL_VERSION"); ver == "" {
		os.Setenv("MYSQL_VERSION", "8.0.26")
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
	return filepath.Join(socketBaseDir, name)
}

func StartMySQLD(name string, port int, serverID int) {
	ctx := context.Background()

	if serverID == 0 {
		serverID = 1
	}

	dir := socketDir(name)
	ExpectWithOffset(1, os.MkdirAll(dir, 0755)).NotTo(HaveOccurred())
	err := os.Chmod(dir, 0777)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	args := []string{
		"run", "--name", name, "-d", "--restart=always",
		"--network=" + networkName,
		"-e", "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		"-e", "MYSQL_ROOT_HOST=localhost",
		"-p", fmt.Sprintf("%d:3306", port),
		"-v", dir + ":/var/run/mysqld",
		"mysql:" + MySQLVersion,
		fmt.Sprintf("--server-id=%d", serverID),
	}
	for k, v := range testMycnf {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}

	cmd := well.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	ExpectWithOffset(1, cmd.Run()).NotTo(HaveOccurred())

	socket := filepath.Join(dir, "mysqld.sock")
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
