package test_utils

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/cybozu-go/well"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const (
	Host             = "localhost"
	RootUser         = "root"
	RootUserPassword = "rootpassword"

	// Dummy user and password for clone from external.
	ExternalDonorUser         = "external-donor-user"
	ExternalDonorUserPassword = "externaldonorpassword"
	ExternalInitUser          = "external-init-user"
	ExternalInitUserPassword  = "externalinitpassword"

	// Dummy password for MySQL users which are managed by MOCO.
	AdminUserPassword       = "adminpassword"
	ReplicationUserPassword = "replpassword"
	CloneDonorUserPassword  = "clonepassword"
	AgentUserPassword       = "agentpassword"

	// Docker network name for test.
	networkName = "moco-agent-test-net"
)

var MysqlSocketDir = filepath.Join(os.TempDir(), "moco-agent-test-mysql-socket-dir")

var MySQLVersion = func() string {
	if ver := os.Getenv("MYSQL_VERSION"); ver == "" {
		os.Setenv("MYSQL_VERSION", "8.0.20")
	}
	return os.Getenv("MYSQL_VERSION")
}()

func StartMySQLD(name string, port int, serverID int, opt ...string) error {
	return StartMySQLDWithSockeDir(name, port, serverID, false, opt...)
}

func StartMySQLDWithSockeDir(name string, port int, serverID int, mountSocketDir bool, opt ...string) error {
	ctx := context.Background()

	var binlogBaseDir string
	var binlogPrefix string
	if len(opt) >= 2 {
		binlogBaseDir = opt[0]
		binlogPrefix = opt[1]
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// docker run options
	args := []string{
		"run", "--name", name, "-d", "--restart=always",
		"--network=" + networkName,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"-e", "MYSQL_ROOT_PASSWORD=" + RootUserPassword,
		"-v", filepath.Join(wd, "..", "my.cnf") + ":/etc/mysql/conf.d/my.cnf",
	}
	if mountSocketDir {
		args = append(args, "-v", MysqlSocketDir+":/var/run/mysqld")
	}
	if binlogBaseDir != "" {
		args = append(args, "-v", binlogBaseDir+":"+binlogBaseDir)
	}

	// mysqld options
	args = append(args,
		"mysql:"+MySQLVersion,
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--server-id=%d", serverID),
	)
	if binlogBaseDir != "" {
		args = append(args, "--log-bin="+binlogBaseDir+"/"+binlogPrefix)
	}

	cmd := well.CommandContext(ctx, "docker", args...)
	return run(cmd)
}

func StopAndRemoveMySQLD(name string) error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "stop", name)
	run(cmd)

	cmd = well.CommandContext(ctx, "docker", "rm", name)
	return run(cmd)
}

func CreateNetwork() error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "network", "create", networkName)
	run(cmd)

	cmd = well.CommandContext(ctx, "docker", "network", "inspect", networkName)
	return run(cmd)
}

func RemoveNetwork() error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "network", "rm", networkName)
	return run(cmd)
}

func CreateSocketDir() error {
	os.RemoveAll(MysqlSocketDir)
	if err := os.Mkdir(MysqlSocketDir, os.ModePerm); err != nil {
		return err
	}
	return os.Chmod(MysqlSocketDir, 0777)
}

func RemoveSocketDir() error {
	return os.RemoveAll(MysqlSocketDir)
}

func Connect(port, retryCount int) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = RootUser
	conf.Passwd = RootUserPassword
	conf.Net = "tcp"
	conf.Addr = Host + ":" + strconv.Itoa(port)
	conf.InterpolateParams = true

	var db *sqlx.DB
	var err error
	dataSource := conf.FormatDSN()
	for i := 0; i <= retryCount; i++ {
		fmt.Printf("[test_utils/connect] %d, %s\n", i, dataSource)
		db, err = sqlx.Connect("mysql", dataSource)
		if err == nil {
			break
		}
		time.Sleep(time.Second * 3)
	}
	return db, err
}

func InitializeMySQL(port int) error {
	db, err := Connect(port, 20)
	if err != nil {
		return err
	}

	users := []struct {
		name     string
		password string
	}{
		{
			name:     mocoagent.AdminUser,
			password: AdminUserPassword,
		},
		{
			name:     mocoagent.AgentUser,
			password: AgentUserPassword,
		},
		{
			name:     mocoagent.ReplicationUser,
			password: ReplicationUserPassword,
		},
		{
			name:     mocoagent.CloneDonorUser,
			password: CloneDonorUserPassword,
		},
	}
	for _, user := range users {
		_, err = db.Exec("CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED WITH mysql_native_password BY ?", user.name, user.password)
		if err != nil {
			return err
		}
		_, err = db.Exec("GRANT ALL ON *.* TO ?@'%' WITH GRANT OPTION", user.name)
		if err != nil {
			return err
		}
	}

	_, err = db.Exec("INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'rpl_semi_sync_master' already exists" {
			return err
		}
	}
	_, err = db.Exec("INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'rpl_semi_sync_slave' already exists" {
			return err
		}
	}
	_, err = db.Exec("INSTALL PLUGIN clone SONAME 'mysql_clone.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'clone' already exists" {
			return err
		}
	}

	buf := make([]byte, 256)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	_, err = db.Exec("CLONE LOCAL DATA DIRECTORY = ?", fmt.Sprintf("/tmp/%x", sha256.Sum256(buf)))
	if err != nil {
		return err
	}

	return ResetMaster(port)
}

func InitializeMySQLAsExternalDonor(port int) error {
	db, err := Connect(port, 20)
	if err != nil {
		return err
	}

	users := []struct {
		name     string
		password string
	}{
		{
			name:     ExternalDonorUser,
			password: ExternalDonorUserPassword,
		},
		{
			name:     ExternalInitUser,
			password: ExternalInitUserPassword,
		},
	}
	for _, user := range users {
		_, err = db.Exec("CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED BY ?", user.name, user.password)
		if err != nil {
			return err
		}
		_, err = db.Exec("GRANT ALL ON *.* TO ?@'%' WITH GRANT OPTION", user.name)
		if err != nil {
			return err
		}
	}

	_, err = db.Exec("INSTALL PLUGIN clone SONAME 'mysql_clone.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'clone' already exists" {
			return err
		}
	}

	buf := make([]byte, 256)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	_, err = db.Exec("CLONE LOCAL DATA DIRECTORY = ?", fmt.Sprintf("/tmp/%x", sha256.Sum256(buf)))
	if err != nil {
		return err
	}

	return ResetMaster(port)
}

func PrepareTestData(port int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS test")
	if err != nil {
		return err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS test.t1 (
    num bigint unsigned NOT NULL AUTO_INCREMENT,
    val0 varchar(100) DEFAULT NULL,
    val1 varchar(100) DEFAULT NULL,
    val2 varchar(100) DEFAULT NULL,
    val3 varchar(100) DEFAULT NULL,
    val4 varchar(100) DEFAULT NULL,
    UNIQUE KEY num (num)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
INSERT INTO test.t1 (val0, val1, val2, val3, val4)
WITH RECURSIVE t AS (
    SELECT 1 AS n
    UNION ALL
    SELECT n + 1 FROM t WHERE n < 10
)
SELECT MD5(RAND()), MD5(RAND()), MD5(RAND()), MD5(RAND()), MD5(RAND())
FROM t`)
	if err != nil {
		return err
	}

	_, err = db.Exec("COMMIT")
	return err
}

func SetValidDonorList(port int, donorHost string, donorPort int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("SET GLOBAL clone_valid_donor_list = ?", donorHost+":"+strconv.Itoa(donorPort))
	return err
}

func ResetMaster(port int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("RESET MASTER")
	return err
}

func SetReadonly(port int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("SET GLOBAL read_only=1")
	return err
}

func StartSlaveWithInvalidSettings(port int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("CHANGE MASTER TO MASTER_HOST = ?, MASTER_PORT = ?, MASTER_USER = ?, MASTER_PASSWORD = ?", "dummy", 3306, "dummy", "dummy")
	if err != nil {
		return err
	}
	_, err = db.Exec("START SLAVE")
	return err
}

func StartSlaveWithValidSettings(port int, targetHost string, targetPort int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("CHANGE MASTER TO MASTER_HOST = ?, MASTER_PORT = ?, MASTER_USER = ?, MASTER_PASSWORD = ?", targetHost, targetPort, mocoagent.ReplicationUser, ReplicationUserPassword)
	if err != nil {
		return err
	}
	_, err = db.Exec("START SLAVE")
	return err
}

func StopAndResetSlave(port int) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec("STOP SLAVE")
	if err != nil {
		return err
	}
	_, err = db.Exec("RESET SLAVE")
	return err
}

func ExecSQLCommand(port int, command string) error {
	db, err := Connect(port, 0)
	if err != nil {
		return err
	}

	_, err = db.Exec(command)
	return err
}

func ConnectMySQL(addr, user, password string) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "tcp"
	conf.Addr = addr
	conf.Timeout = 3 * time.Second
	conf.ReadTimeout = 30 * time.Second
	conf.InterpolateParams = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

func StartMySQLDForTestInit(name string) error {
	ctx := context.Background()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	args := []string{
		"run", "--name", name, "-d",
		"-e", "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		"-v", filepath.Join(wd, "..", "my.cnf") + ":/etc/mysql/conf.d/my.cnf",
		"-v", MysqlSocketDir + ":/var/run/mysqld",
		"mysql:" + MySQLVersion,
	}

	cmd := well.CommandContext(ctx, "docker", args...)
	return run(cmd)
}
