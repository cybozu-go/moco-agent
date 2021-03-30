package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func getMySQLConn(user, password, host string, port int, config MySQLAccessorConfig) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "tcp"
	conf.Addr = fmt.Sprintf("%s:%d", host, port)
	conf.Timeout = config.ConnectionTimeout
	conf.ReadTimeout = config.ReadTimeout
	conf.InterpolateParams = true
	conf.ParseTime = true

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
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

func IsAccessDenied(err error) bool {
	var merr *mysql.MySQLError
	if errors.As(err, &merr) && merr.Number == 1045 {
		return true
	}

	return false
}
