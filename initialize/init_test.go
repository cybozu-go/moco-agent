package initialize

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cybozu-go/moco"
	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/go-sql-driver/mysql"
	"github.com/google/go-cmp/cmp"
	"github.com/jmoiron/sqlx"
)

var (
	initOnceCompletedPath = filepath.Join(moco.MySQLDataPath, "init-once-completed")
	passwordFilePath      = filepath.Join("/tmp", "moco-root-password")
	rootPassword          = "testpassword"
	agentConfPath         = filepath.Join(moco.MySQLDataPath, "agent.cnf")
	initUser              = "init-user"
	initPassword          = "init-password"
	adminPassword         = "admin-password"
)

func testGenerateMySQLConfiguration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tempDir, moco.MySQLConfTemplatePath), 0777); err != nil {
		t.Fatalf("failed to craete temp dir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(tempDir, moco.MySQLConfPath), 0777); err != nil {
		t.Fatalf("failed to craete temp dir: %v", err)
	}

	template, err := ioutil.ReadFile("testdata/template-my.cnf")
	if err != nil {
		t.Fatalf("failed to load testdata: %v", err)
	}

	templateConfPath := filepath.Join(tempDir, moco.MySQLConfTemplatePath, moco.MySQLConfName)

	if err := ioutil.WriteFile(templateConfPath, template, 0644); err != nil {
		t.Fatalf("failed to create mysql configuration file template: %v", err)
	}

	if err := os.Setenv(moco.PodNameEnvName, "moco-mysqlcluster-0"); err != nil {
		t.Fatalf("failed to set env %s: %v", moco.PodNameEnvName, err)
	}
	defer os.Unsetenv(moco.PodNameEnvName)

	if err := generateMySQLConfiguration(ctx, 1000,
		filepath.Join(tempDir, moco.MySQLConfTemplatePath), filepath.Join(tempDir, moco.MySQLConfPath), moco.MySQLConfName); err != nil {
		t.Fatalf("failed to generate mysql configuration file: %v", err)
	}

	want, err := ioutil.ReadFile("testdata/my.cnf")
	if err != nil {
		t.Fatalf("failed to load testdata: %v", err)
	}

	got, err := ioutil.ReadFile(filepath.Join(tempDir, moco.MySQLConfPath, moco.MySQLConfName))
	if err != nil {
		t.Fatalf("failed to load generated mysql configration file: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("generated mysql configration file mismatch (-want +got):\n%s", diff)
	}
}

func testInitializeInstance(t *testing.T) {
	ctx := context.Background()

	err := os.MkdirAll(moco.MySQLConfPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	confPath := filepath.Join(moco.MySQLConfPath, moco.MySQLConfName)
	err = ioutil.WriteFile(confPath, []byte(`[client]
socket = /var/run/mysqld/mysqld.sock
loose_default_character_set = utf8mb4
[mysqld]
socket = /var/run/mysqld/mysqld.sock
datadir = /var/lib/mysql
log_error = /var/log/mysql/error.log
slow_query_log_file = /var/log/mysql/slow.log
pid_file = /var/run/mysqld/mysqld.pid
character_set_server = utf8mb4
collation_server = utf8mb4_unicode_ci
default_time_zone = +0:00
disabled_storage_engines = MyISAM
enforce_gtid_consistency = ON
gtid_mode = ON
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = initializeInstance(ctx)
	if err != nil {
		t.Fatal(err)
	}

}

func myAddress() net.IP {
	netInterfaceAddresses, _ := net.InterfaceAddrs()
	for _, netInterfaceAddress := range netInterfaceAddresses {
		networkIP, ok := netInterfaceAddress.(*net.IPNet)
		if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
			return networkIP.IP
		}
	}
	return net.IPv4zero
}

func testWaitInstanceBootstrap(t *testing.T) {
	ctx := context.Background()
	err := waitInstanceBootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func testInitializeAdminUser(t *testing.T) {
	ctx := context.Background()
	adminPassword := "admin-password"
	err := initializeAdminUser(ctx, passwordFilePath, "root", nil, adminPassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-admin';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-admin")
	}
	out, err = execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='root';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "0") {
		t.Fatal("root user isn't dropped")
	}

	err = initializeAdminUser(ctx, passwordFilePath, "moco-admin", &adminPassword, adminPassword)
	if err != nil {
		t.Fatal(err)
	}
	out, err = execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-admin';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-admin")
	}
}

func testInitializeDonorUser(t *testing.T) {
	ctx := context.Background()
	donorPassword := "donor-password"
	err := initializeDonorUser(ctx, passwordFilePath, donorPassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-clone-donor';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-clone-donor")
	}
}

func testInitializeReplicationUser(t *testing.T) {
	ctx := context.Background()
	replicationPassword := "replication-password"
	err := initializeReplicationUser(ctx, passwordFilePath, replicationPassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-repl';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-repl")
	}
}

func testInitializeAgentUser(t *testing.T) {
	ctx := context.Background()
	agentPassword := "agent-password"
	err := initializeAgentUser(ctx, passwordFilePath, agentConfPath, agentPassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-agent';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatalf("cannot find user: moco-agent")
	}
	_, err = os.Stat(agentConfPath)
	if err != nil {
		t.Fatal(err)
	}
}

func testInitializeReadOnlyUser(t *testing.T) {
	ctx := context.Background()
	readOnlyPassword := "readonly-password"
	err := initializeReadOnlyUser(ctx, passwordFilePath, readOnlyPassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-readonly';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-readonly")
	}
}

func testInitializeWritableUser(t *testing.T) {
	ctx := context.Background()
	writablePassword := "writable-password"
	err := initializeWritableUser(ctx, passwordFilePath, writablePassword)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SELECT count(*) FROM mysql.user WHERE user='moco-writable';"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1") {
		t.Fatal("cannot find user: moco-writable")
	}
}

func testInstallPlugins(t *testing.T) {
	ctx := context.Background()
	err := installPlugins(ctx, passwordFilePath)
	if err != nil {
		t.Fatal(err)
	}

	out, err := execSQL(ctx, passwordFilePath, []byte("SHOW PLUGINS;"), "")
	if err != nil {
		t.Fatal(err)
	}

	semiSyncMasterFound := false
	semiSyncSlaveFound := false
	cloneFound := false
	for _, plugin := range strings.Split(string(out), "\n") {
		if strings.Contains(plugin, "rpl_semi_sync_master") {
			semiSyncMasterFound = true
		}
		if strings.Contains(plugin, "rpl_semi_sync_slave") {
			semiSyncSlaveFound = true
		}
		if strings.Contains(plugin, "clone") {
			cloneFound = true
		}
	}
	if !semiSyncMasterFound {
		t.Fatal("cannot find plugin: rpl_semi_sync_master")
	}
	if !semiSyncSlaveFound {
		t.Fatal("cannot find plugin: rpl_semi_sync_slave")
	}
	if !cloneFound {
		t.Fatal("cannot find plugin: clone")
	}
}

func testRestoreUsers(t *testing.T) {
	ctx := context.Background()

	if err := os.Setenv(moco.OperatorPasswordEnvName, adminPassword); err != nil {
		t.Fatalf("failed to set env %s: %v", moco.OperatorPasswordEnvName, err)
	}
	defer os.Unsetenv(moco.PodNameEnvName)
	err := RestoreUsers(ctx, passwordFilePath, agentConfPath, "moco-admin", &adminPassword)
	if err != nil {
		t.Error(err)
	}

	conf := mysql.NewConfig()
	conf.User = "moco-admin"
	conf.Passwd = adminPassword
	conf.Net = "unix"
	conf.Addr = "/var/run/mysqld/mysqld.sock"
	conf.InterpolateParams = true

	var db *sqlx.DB
	for i := 0; i < 20; i++ {
		db, err = sqlx.Connect("mysql", conf.FormatDSN())
		if err == nil {
			break
		}
		fmt.Printf("%+v", err)
		time.Sleep(time.Second * 3)
	}
	defer db.Close()

	for _, k := range []string{
		moco.OperatorAdminUser,
		moco.CloneDonorUser,
		moco.ReplicationUser,
		mocoagent.AgentUser,
	} {
		sqlRows, err := db.Query("SELECT user FROM mysql.user WHERE (user = ? AND host = '%')", k)
		if err != nil {
			t.Fatal(err)
		}
		if !sqlRows.Next() {
			t.Errorf("user '%s' should be created, but not exist", k)
		}
	}
}

func testShutdownInstance(t *testing.T) {
	ctx := context.Background()
	err := ShutdownInstance(ctx, passwordFilePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = doExec(ctx, nil, "mysqladmin", "ping")
	if err == nil {
		t.Fatal("cannot shutdown instance")
	}
}

func testTouchInitOnceCompleted(t *testing.T) {
	ctx := context.Background()
	err := touchInitOnceCompleted(ctx, initOnceCompletedPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(initOnceCompletedPath)
	if err != nil {
		t.Fatal(err)
	}
}

func testRetryInitializeOnce(t *testing.T) {
	ctx := context.Background()
	err := os.Remove(initOnceCompletedPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Setenv(moco.PodNameEnvName, "moco-mysqlcluster-0"); err != nil {
		t.Fatalf("failed to set env %s: %v", moco.PodNameEnvName, err)
	}
	defer os.Unsetenv(moco.PodNameEnvName)

	err = InitializeOnce(ctx, initOnceCompletedPath, passwordFilePath, agentConfPath, 1000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInit(t *testing.T) {
	_, err := os.Stat(filepath.Join("/", ".dockerenv"))
	if err != nil {
		t.Skip("These tests should be run on docker")
	}
	t.Run("generateMySQLConfiguration", testGenerateMySQLConfiguration)
	t.Run("initializeInstance", testInitializeInstance)
	t.Run("waitInstanceBootstrap", testWaitInstanceBootstrap)
	t.Run("initializeAdminUser", testInitializeAdminUser)
	t.Run("initializeDonorUser", testInitializeDonorUser)
	t.Run("initializeReplicationUser", testInitializeReplicationUser)
	t.Run("initializeAgentUser", testInitializeAgentUser)
	t.Run("initializeReadOnlyUser", testInitializeReadOnlyUser)
	t.Run("initializeWritableUser", testInitializeWritableUser)
	t.Run("installPlugins", testInstallPlugins)
	t.Run("restoreUsers", testRestoreUsers)
	t.Run("shutdownInstance", testShutdownInstance)
	t.Run("touchInitOnceCompleted", testTouchInitOnceCompleted)
	t.Run("retryInitialization", testRetryInitializeOnce)
}
