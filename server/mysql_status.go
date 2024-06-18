package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// MySQLGlobalVariablesStatus defines the observed global variable state of a MySQL instance
type MySQLGlobalVariablesStatus struct {
	ReadOnly                           bool           `db:"@@read_only"`
	SuperReadOnly                      bool           `db:"@@super_read_only"`
	RplSemiSyncMasterWaitForSlaveCount int            `db:"@@rpl_semi_sync_master_wait_for_slave_count"`
	CloneValidDonorList                sql.NullString `db:"@@clone_valid_donor_list"`
}

// MySQLCloneStateStatus defines the observed clone state of a MySQL instance
type MySQLCloneStateStatus struct {
	State sql.NullString `db:"state"`
}

// MySQLPrimaryStatus defines the observed state of a primary
type MySQLPrimaryStatus struct {
	ExecutedGtidSet string `db:"Executed_Gtid_Set"`

	// All of variables from here are NOT used in MOCO's reconcile
	File           string `db:"File"`
	Position       string `db:"Position"`
	BinlogDoDB     string `db:"Binlog_Do_DB"`
	BinlogIgnoreDB string `db:"Binlog_Ignore_DB"`
}

// MySQLReplicaStatus defines the observed state of a replica
type MySQLReplicaStatus struct {
	LastIOErrno       int    `db:"Last_IO_Errno"`
	LastIOError       string `db:"Last_IO_Error"`
	LastSQLErrno      int    `db:"Last_SQL_Errno"`
	LastSQLError      string `db:"Last_SQL_Error"`
	SourceHost        string `db:"Source_Host"`
	RetrievedGtidSet  string `db:"Retrieved_Gtid_Set"`
	ExecutedGtidSet   string `db:"Executed_Gtid_Set"`
	ReplicaIORunning  string `db:"Replica_IO_Running"`
	ReplicaSQLRunning string `db:"Replica_SQL_Running"`

	// All of variables from here are NOT used in MOCO's reconcile
	ReplicaIOState            string        `db:"Replica_IO_State"`
	SourceUser                string        `db:"Source_User"`
	SourcePort                int           `db:"Source_Port"`
	ConnectRetry              int           `db:"Connect_Retry"`
	SourceLogFile             string        `db:"Source_Log_File"`
	ReadSourceLogPos          int           `db:"Read_Source_Log_Pos"`
	RelayLogFile              string        `db:"Relay_Log_File"`
	RelayLogPos               int           `db:"Relay_Log_Pos"`
	RelaySourceLogFile        string        `db:"Relay_Source_Log_File"`
	ReplicateDoDB             string        `db:"Replicate_Do_DB"`
	ReplicateIgnoreDB         string        `db:"Replicate_Ignore_DB"`
	ReplicateDoTable          string        `db:"Replicate_Do_Table"`
	ReplicateIgnoreTable      string        `db:"Replicate_Ignore_Table"`
	ReplicateWildDoTable      string        `db:"Replicate_Wild_Do_Table"`
	ReplicateWildIgnoreTable  string        `db:"Replicate_Wild_Ignore_Table"`
	LastErrno                 int           `db:"Last_Errno"`
	LastError                 string        `db:"Last_Error"`
	SkipCounter               int           `db:"Skip_Counter"`
	ExecSourceLogPos          int           `db:"Exec_Source_Log_Pos"`
	RelayLogSpace             int           `db:"Relay_Log_Space"`
	UntilCondition            string        `db:"Until_Condition"`
	UntilLogFile              string        `db:"Until_Log_File"`
	UntilLogPos               int           `db:"Until_Log_Pos"`
	SourceSSLAllowed          string        `db:"Source_SSL_Allowed"`
	SourceSSLCAFile           string        `db:"Source_SSL_CA_File"`
	SourceSSLCAPath           string        `db:"Source_SSL_CA_Path"`
	SourceSSLCert             string        `db:"Source_SSL_Cert"`
	SourceSSLCipher           string        `db:"Source_SSL_Cipher"`
	SourceSSLKey              string        `db:"Source_SSL_Key"`
	SecondsBehindSource       sql.NullInt64 `db:"Seconds_Behind_Source"`
	SourceSSLVerifyServerCert string        `db:"Source_SSL_Verify_Server_Cert"`
	ReplicateIgnoreServerIds  string        `db:"Replicate_Ignore_Server_Ids"`
	SourceServerID            int           `db:"Source_Server_Id"`
	SourceUUID                string        `db:"Source_UUID"`
	SourceInfoFile            string        `db:"Source_Info_File"`
	SQLDelay                  int           `db:"SQL_Delay"`
	SQLRemainingDelay         sql.NullInt64 `db:"SQL_Remaining_Delay"`
	ReplicaSQLRunningState    string        `db:"Replica_SQL_Running_State"`
	SourceRetryCount          int           `db:"Source_Retry_Count"`
	SourceBind                string        `db:"Source_Bind"`
	LastIOErrorTimestamp      string        `db:"Last_IO_Error_Timestamp"`
	LastSQLErrorTimestamp     string        `db:"Last_SQL_Error_Timestamp"`
	SourceSSLCrl              string        `db:"Source_SSL_Crl"`
	SourceSSLCrlpath          string        `db:"Source_SSL_Crlpath"`
	AutoPosition              string        `db:"Auto_Position"`
	ReplicateRewriteDB        string        `db:"Replicate_Rewrite_DB"`
	ChannelName               string        `db:"Channel_Name"`
	SourceTLSVersion          string        `db:"Source_TLS_Version"`
	Sourcepublickeypath       string        `db:"Source_public_key_path"`
	GetSourcepublickey        string        `db:"Get_Source_public_key"`
	NetworkNamespace          string        `db:"Network_Namespace"`
}

func (a *Agent) GetMySQLGlobalVariable(ctx context.Context) (*MySQLGlobalVariablesStatus, error) {
	status := &MySQLGlobalVariablesStatus{}
	err := a.db.GetContext(ctx, status, `SELECT @@read_only, @@super_read_only, @@rpl_semi_sync_master_wait_for_slave_count, @@clone_valid_donor_list`)
	if err != nil {
		return nil, fmt.Errorf("failed to get global variable: %w", err)
	}
	return status, nil
}

func (a *Agent) GetMySQLCloneStateStatus(ctx context.Context) (*MySQLCloneStateStatus, error) {
	status := &MySQLCloneStateStatus{}
	err := a.db.GetContext(ctx, status, `SELECT state FROM performance_schema.clone_status`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &MySQLCloneStateStatus{}, nil
		}
		return nil, err
	}
	return status, nil
}

func (a *Agent) IsMySQL84(ctx context.Context) (bool, error) {
	var version string
	err := a.db.GetContext(ctx, &version, `SELECT SUBSTRING_INDEX(VERSION(), '.', 2)`)
	if err != nil {
		return false, fmt.Errorf("failed to get version: %w", err)
	}
	return version == "8.4", nil
}

func (a *Agent) GetMySQLPrimaryStatus(ctx context.Context) (*MySQLPrimaryStatus, error) {
	status := &MySQLPrimaryStatus{}
	isMySQL84, err := a.IsMySQL84(ctx)
	if err != nil {
		return nil, err
	}
	if isMySQL84 {
		if err := a.db.GetContext(ctx, status, `SHOW BINARY LOG STATUS`); err != nil {
			return nil, fmt.Errorf("failed to show binary log status: %w", err)
		}
	} else {
		if err := a.db.GetContext(ctx, status, `SHOW MASTER STATUS`); err != nil {
			return nil, fmt.Errorf("failed to show master status: %w", err)
		}
	}
	return status, nil
}

func (a *Agent) GetMySQLReplicaStatus(ctx context.Context) (*MySQLReplicaStatus, error) {
	status := &MySQLReplicaStatus{}
	if err := a.db.GetContext(ctx, status, `SHOW REPLICA STATUS`); err != nil {
		return nil, fmt.Errorf("failed to show replica status: %w", err)
	}
	return status, nil
}

func (a *Agent) GetTransactionTimestamps(ctx context.Context) (queued, applied time.Time, uptime time.Duration, err error) {
	err = a.db.GetContext(ctx, &queued, `
SELECT MAX(LAST_QUEUED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP)
FROM performance_schema.replication_connection_status`)
	if err != nil {
		return
	}
	err = a.db.GetContext(ctx, &applied, `
SELECT MAX(LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP)
FROM performance_schema.replication_applier_status_by_worker`)
	if err != nil {
		return
	}
	var uptime_seconds_string string
	err = a.db.GetContext(ctx, &uptime_seconds_string, `
SELECT VARIABLE_VALUE
FROM performance_schema.global_status
WHERE VARIABLE_NAME='Uptime'`)
	if err != nil {
		return
	}
	uptime_seconds, err := strconv.Atoi(uptime_seconds_string)
	if err != nil {
		return
	}
	uptime = time.Second * time.Duration(uptime_seconds)
	return
}
