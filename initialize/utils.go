package initialize

import (
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func GetMySQLConnLocalSocket(user, password, socket string, retryCount int) (*sqlx.DB, error) {
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

		if IsAccessDenied(err) {
			break
		}

		time.Sleep(time.Second * 3)
	}
	if err != nil {
		return nil, err
	}

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
