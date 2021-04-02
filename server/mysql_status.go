package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	LastIOErrno      int    `db:"Last_IO_Errno"`
	LastIOError      string `db:"Last_IO_Error"`
	LastSQLErrno     int    `db:"Last_SQL_Errno"`
	LastSQLError     string `db:"Last_SQL_Error"`
	MasterHost       string `db:"Master_Host"`
	RetrievedGtidSet string `db:"Retrieved_Gtid_Set"`
	ExecutedGtidSet  string `db:"Executed_Gtid_Set"`
	SlaveIORunning   string `db:"Slave_IO_Running"`
	SlaveSQLRunning  string `db:"Slave_SQL_Running"`

	// All of variables from here are NOT used in MOCO's reconcile
	SlaveIOState              string        `db:"Slave_IO_State"`
	MasterUser                string        `db:"Master_User"`
	MasterPort                int           `db:"Master_Port"`
	ConnectRetry              int           `db:"Connect_Retry"`
	MasterLogFile             string        `db:"Master_Log_File"`
	ReadMasterLogPos          int           `db:"Read_Master_Log_Pos"`
	RelayLogFile              string        `db:"Relay_Log_File"`
	RelayLogPos               int           `db:"Relay_Log_Pos"`
	RelayMasterLogFile        string        `db:"Relay_Master_Log_File"`
	ReplicateDoDB             string        `db:"Replicate_Do_DB"`
	ReplicateIgnoreDB         string        `db:"Replicate_Ignore_DB"`
	ReplicateDoTable          string        `db:"Replicate_Do_Table"`
	ReplicateIgnoreTable      string        `db:"Replicate_Ignore_Table"`
	ReplicateWildDoTable      string        `db:"Replicate_Wild_Do_Table"`
	ReplicateWildIgnoreTable  string        `db:"Replicate_Wild_Ignore_Table"`
	LastErrno                 int           `db:"Last_Errno"`
	LastError                 string        `db:"Last_Error"`
	SkipCounter               int           `db:"Skip_Counter"`
	ExecMasterLogPos          int           `db:"Exec_Master_Log_Pos"`
	RelayLogSpace             int           `db:"Relay_Log_Space"`
	UntilCondition            string        `db:"Until_Condition"`
	UntilLogFile              string        `db:"Until_Log_File"`
	UntilLogPos               int           `db:"Until_Log_Pos"`
	MasterSSLAllowed          string        `db:"Master_SSL_Allowed"`
	MasterSSLCAFile           string        `db:"Master_SSL_CA_File"`
	MasterSSLCAPath           string        `db:"Master_SSL_CA_Path"`
	MasterSSLCert             string        `db:"Master_SSL_Cert"`
	MasterSSLCipher           string        `db:"Master_SSL_Cipher"`
	MasterSSLKey              string        `db:"Master_SSL_Key"`
	SecondsBehindMaster       sql.NullInt64 `db:"Seconds_Behind_Master"`
	MasterSSLVerifyServerCert string        `db:"Master_SSL_Verify_Server_Cert"`
	ReplicateIgnoreServerIds  string        `db:"Replicate_Ignore_Server_Ids"`
	MasterServerID            int           `db:"Master_Server_Id"`
	MasterUUID                string        `db:"Master_UUID"`
	MasterInfoFile            string        `db:"Master_Info_File"`
	SQLDelay                  int           `db:"SQL_Delay"`
	SQLRemainingDelay         sql.NullInt64 `db:"SQL_Remaining_Delay"`
	SlaveSQLRunningState      string        `db:"Slave_SQL_Running_State"`
	MasterRetryCount          int           `db:"Master_Retry_Count"`
	MasterBind                string        `db:"Master_Bind"`
	LastIOErrorTimestamp      string        `db:"Last_IO_Error_Timestamp"`
	LastSQLErrorTimestamp     string        `db:"Last_SQL_Error_Timestamp"`
	MasterSSLCrl              string        `db:"Master_SSL_Crl"`
	MasterSSLCrlpath          string        `db:"Master_SSL_Crlpath"`
	AutoPosition              string        `db:"Auto_Position"`
	ReplicateRewriteDB        string        `db:"Replicate_Rewrite_DB"`
	ChannelName               string        `db:"Channel_Name"`
	MasterTLSVersion          string        `db:"Master_TLS_Version"`
	Masterpublickeypath       string        `db:"Master_public_key_path"`
	Getmasterpublickey        string        `db:"Get_master_public_key"`
	NetworkNamespace          string        `db:"Network_Namespace"`
}

type MySQLLastAppliedTransactionTimestamps struct {
	OriginalCommitTimestamp time.Time `db:"LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP"`
	EndApplyTimestamp       time.Time `db:"LAST_APPLIED_TRANSACTION_END_APPLY_TIMESTAMP"`
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

func (a *Agent) GetMySQLPrimaryStatus(ctx context.Context) (*MySQLPrimaryStatus, error) {
	rows, err := a.db.QueryxContext(ctx, `SHOW MASTER STATUS`)
	if err != nil {
		return nil, fmt.Errorf("failed to show master status: %w", err)
	}
	defer rows.Close()

	var status MySQLPrimaryStatus
	if rows.Next() {
		err = rows.StructScan(&status)
		if err != nil {
			return nil, err
		}
		return &status, nil
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("return value is empty")
}

func (a *Agent) GetMySQLReplicaStatus(ctx context.Context) (*MySQLReplicaStatus, error) {
	rows, err := a.db.QueryxContext(ctx, `SHOW SLAVE STATUS`)
	if err != nil {
		return nil, fmt.Errorf("failed to show slave status: %w", err)
	}
	defer rows.Close()

	var status MySQLReplicaStatus
	if rows.Next() {
		err = rows.StructScan(&status)
		if err != nil {
			return nil, err
		}
		return &status, nil
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("return value is empty")
}

func (a *Agent) GetMySQLLastAppliedTransactionTimestamps(ctx context.Context) (*MySQLLastAppliedTransactionTimestamps, error) {
	timestamps := &MySQLLastAppliedTransactionTimestamps{}
	err := a.db.GetContext(ctx, timestamps, `SELECT LAST_APPLIED_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP, LAST_APPLIED_TRANSACTION_END_APPLY_TIMESTAMP FROM performance_schema.replication_applier_status_by_worker`)
	if err != nil {
		return nil, fmt.Errorf("failed to get last applied transaction timestamps: %w", err)
	}
	return timestamps, nil
}
