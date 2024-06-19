package server

import (
	"errors"
	"fmt"
	"net"
	"time"

	mocoagent "github.com/cybozu-go/moco-agent"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func getMySQLConn(config MySQLAccessorConfig) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = mocoagent.AgentUser
	conf.Passwd = config.Password
	conf.Net = "tcp"
	conf.Addr = net.JoinHostPort(config.Host, fmt.Sprint(config.Port))
	conf.Timeout = config.ConnectionTimeout
	conf.ReadTimeout = config.ReadTimeout
	conf.InterpolateParams = true
	conf.ParseTime = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(1)

	return db, nil
}

func GetMySQLConnLocalSocket(user, password, socket string) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "unix"
	conf.Addr = socket
	conf.InterpolateParams = true
	conf.ParseTime = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(30 * time.Second)
	db.SetMaxIdleConns(1)
	return db, nil
}

func UserNotExists(err error) bool {
	// For security reason, error messages are randomly output when a user does not exist.
	//   https://github.com/mysql/mysql-server/commit/b40001faf6229dca668c9d03ba75c451f999c9f5
	// This function assumes the user does not exist when the following message is output:
	//   ERROR 1045 (28000): Access denied for user 'root'@'localhost' (using password: NO)
	//   ERROR 1524 (HY000): Plugin 'mysql_native_password' is not loaded
	var merr *mysql.MySQLError
	if errors.As(err, &merr) && (merr.Number == 1045 || merr.Number == 1524) {
		return true
	}

	return false
}
