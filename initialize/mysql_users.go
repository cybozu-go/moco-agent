package initialize

import (
	"bytes"
	"context"
	"os"
	"strings"
	"text/template"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const connRetryCount = 20

type userSetting struct {
	name                    string
	password                string
	privileges              []string
	revokePrivileges        map[string][]string
	withGrantOption         bool
	useNativePasswordPlugin bool
}

func EnsureMOCOUsers(ctx context.Context, user, password, socket string) error {
	db, err := getMySQLConnLocalSocket(user, password, socket, connRetryCount)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, "SET GLOBAL partial_revokes='ON'")
	if err != nil {
		return err
	}

	users := []userSetting{
		{
			name:     mocoagent.AdminUser,
			password: os.Getenv(mocoagent.AdminPasswordEnvKey),
			privileges: []string{
				"ALL",
			},
			withGrantOption: true,
		},
		{
			name:     mocoagent.AgentUser,
			password: os.Getenv(mocoagent.AgentPasswordEnvKey),
			privileges: []string{
				"SELECT",
				"RELOAD",
				"CLONE_ADMIN",
				"BINLOG_ADMIN",
				"SERVICE_CONNECTION_ADMIN",
				"REPLICATION CLIENT",
			},
		},
		{
			name:     mocoagent.ReplicationUser,
			password: os.Getenv(mocoagent.ReplicationPasswordEnvKey),
			privileges: []string{
				"REPLICATION SLAVE",
				"REPLICATION CLIENT",
			},
			useNativePasswordPlugin: true,
		},
		{
			name:     mocoagent.CloneDonorUser,
			password: os.Getenv(mocoagent.CloneDonorPasswordEnvKey),
			privileges: []string{
				"BACKUP_ADMIN",
				"SERVICE_CONNECTION_ADMIN",
			},
		},
		{
			name:     mocoagent.ReadOnlyUser,
			password: os.Getenv(mocoagent.ReadOnlyPasswordEnvKey),
			privileges: []string{
				"PROCESS",
				"SELECT",
				"SHOW DATABASES",
				"SHOW VIEW",
				"REPLICATION CLIENT",
				"REPLICATION SLAVE",
			},
		},
		{
			name:     mocoagent.WritableUser,
			password: os.Getenv(mocoagent.WritablePasswordEnvKey),
			privileges: []string{
				"ALTER",
				"ALTER ROUTINE",
				"CREATE",
				"CREATE ROLE",
				"CREATE ROUTINE",
				"CREATE TEMPORARY TABLES",
				"CREATE USER",
				"CREATE VIEW",
				"DELETE",
				"DROP",
				"DROP ROLE",
				"EVENT",
				"EXECUTE",
				"INDEX",
				"INSERT",
				"LOCK TABLES",
				"PROCESS",
				"REFERENCES",
				"REPLICATION CLIENT",
				"REPLICATION SLAVE",
				"SELECT",
				"SHOW DATABASES",
				"SHOW VIEW",
				"TRIGGER",
				"UPDATE",
			},
			revokePrivileges: map[string][]string{
				"mysql.*": {
					"CREATE",
					"CREATE ROUTINE",
					"CREATE TEMPORARY TABLES",
					"CREATE VIEW",
					"DELETE",
					"DROP",
					"EVENT",
					"EXECUTE",
					"INDEX",
					"INSERT",
					"LOCK TABLES",
					"REFERENCES",
					"TRIGGER",
					"UPDATE",
				},
			},
			withGrantOption: true,
		},
	}

	for _, u := range users {
		err := ensureMySQLUser(ctx, db, u)
		if err != nil {
			return err
		}
	}

	_, err = db.ExecContext(ctx, "DROP USER IF EXISTS 'root'@'localhost'")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	return err
}

func ensureMySQLUser(ctx context.Context, db *sqlx.DB, user userSetting) error {
	var count int
	err := db.GetContext(ctx, &count, `SELECT COUNT(*) FROM mysql.user WHERE user=? and host='%'`, user.name)
	if err != nil {
		return err
	}
	if count == 1 {
		// The user is already exists
		return nil
	}

	// TODO: When using encrypted connection, "WITH mysql_native_password" should be deleted.
	// See https://yoku0825.blogspot.com/2018/10/mysql-80cachingsha2password-ssl.html
	queryStr := `CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED`
	if user.useNativePasswordPlugin {
		queryStr = queryStr + " WITH mysql_native_password"
	}
	queryStr = queryStr + " BY ?"

	_, err = db.ExecContext(ctx, queryStr, user.name, user.password)
	if err != nil {
		return err
	}

	t := template.Must(template.New("sql").Parse(
		`GRANT {{ .Privileges }} ON *.* TO ?@'%'`))

	sql := new(bytes.Buffer)
	err = t.Execute(sql, struct {
		Privileges string
	}{strings.Join(user.privileges, ",")})
	if err != nil {
		return err
	}
	queryStr = sql.String()
	if user.withGrantOption {
		queryStr = queryStr + " WITH GRANT OPTION"
	}

	_, err = db.ExecContext(ctx, queryStr, user.name)
	if err != nil {
		return err
	}

	for target, privileges := range user.revokePrivileges {
		t := template.Must(template.New("sql").Parse(
			`REVOKE {{ .Privileges }} ON {{ .Target }} FROM ?@'%'`))

		sql := new(bytes.Buffer)
		err = t.Execute(sql, struct {
			Privileges string
			Target     string
		}{strings.Join(privileges, ","), target})
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, sql.String(), user.name)
		if err != nil {
			return err
		}
	}

	return nil
}

func getMySQLConnLocalSocket(user, password, socket string, retryCount int) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "unix"
	conf.Addr = socket
	conf.InterpolateParams = true

	var db *sqlx.DB
	var err error
	dataSource := conf.FormatDSN()
	for i := 0; i <= retryCount; i++ {
		db, err = sqlx.Connect("mysql", dataSource)
		if err == nil {
			break
		}
		time.Sleep(time.Second * 3)
	}
	if err != nil {
		return nil, err
	}

	db.SetMaxIdleConns(0)
	return db, nil
}
