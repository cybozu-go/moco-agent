package server

import (
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type options struct {
	connMaxIdleTime   time.Duration
	connMaxLifeTime   time.Duration
	readTimeout       time.Duration
	connectionTimeout time.Duration
}

type Option interface {
	apply(opts *options)
}

type accessorConfigOption MySQLAccessorConfig

func (ac accessorConfigOption) apply(opts *options) {
	opts.connMaxIdleTime = ac.ConnMaxIdleTime
	opts.readTimeout = ac.ReadTimeout
	opts.connectionTimeout = ac.ConnectionTimeout
}

func WithAccessorConfig(ac MySQLAccessorConfig) Option {
	return accessorConfigOption(ac)
}

type connMaxLifeTimeOption time.Duration

func (t connMaxLifeTimeOption) apply(opts *options) {
	opts.connMaxLifeTime = time.Duration(t)
}

func WithConnMaxLifeTime(t time.Duration) Option {
	return connMaxLifeTimeOption(t)
}

func GetMySQLConnLocalSocket(user, password, socket string, opts ...Option) (*sqlx.DB, error) {
	options := options{
		connMaxIdleTime: 30 * time.Second,
	}
	for _, o := range opts {
		o.apply(&options)
	}

	conf := mysql.NewConfig()
	conf.User = user
	conf.Passwd = password
	conf.Net = "unix"
	conf.Addr = socket
	conf.InterpolateParams = true
	conf.ParseTime = true
	if options.connectionTimeout > 0 {
		conf.Timeout = options.connectionTimeout
	}
	if options.readTimeout > 0 {
		conf.ReadTimeout = options.readTimeout
	}

	db, err := sqlx.Connect("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}

	if options.connMaxLifeTime > 0 {
		db.SetConnMaxIdleTime(options.connMaxIdleTime)
	}
	if options.connMaxLifeTime > 0 {
		db.SetConnMaxLifetime(options.connMaxLifeTime)
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
