API Reference of `moco-agent server`
====================================

## Table of Contents

- [Table of Contents](#table-of-contents)
- [gRPC Health Checking](#grpc-health-checking)
- [agentrpc.proto](#agentrpcproto)
  - [CloneService](#cloneservice)
  - [CloneRequest](#clonerequest)
  - [CloneResponse](#cloneresponse)
  - [BackupBinlogService](#backupbinlogservice)
  - [FlushAndBackupBinlogRequest](#flushandbackupbinlogrequest)
  - [FlushAndBackupBinlogResponse](#flushandbackupbinlogresponse)
  - [FlushBinlogRequest](#flushbinlogrequest)
  - [FlushBinlogResponse](#flushbinlogresponse)


## gRPC Health Checking
`moco-agent server` exposes the health checking service through [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).

You can use the gRPC service defined in [gRPC Health Checking Protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md) except for `Watch` service. Besides, [`grpc_health_probe`](https://github.com/grpc-ecosystem/grpc-health-probe) can be used to check the health of gRPC server.

Each response code means:
- `0 (HealthCheckResponse_UNKNOWN)`: Cannot obtain the status of the MySQL instance.
- `1 (HealthCheckResponse_SERVING)`: The MySQL instance can provide service.
- `2 (HealthCheckResponse_NOT_SERVING)`: The MySQL instance cannot provide service because it has IO/SQL thread error and/or is under cloning from another instance.
  - The response message includes the reason like `hasIOThreadError=%t, hasSQLThreadError=%t, isUnderCloning=%t`


## agentrpc.proto

The `agentrpc.proto` file is located at [here](../server/agentrpc/agentrpc.proto).

### CloneService
CloneService is a service for cloning MySQL instance.

| Method Name | Request Type                  | Response Type                   | Description                          |
| ----------- | ----------------------------- | ------------------------------- | ------------------------------------ |
| Clone       | [CloneRequest](#CloneRequest) | [CloneResponse](#CloneResponse) | Clone invokes MySQL `CLONE` command. |

### CloneRequest

CloneRequest is the request message to invoke MySQL `CLONE` command.

| Field      | Type   | Description                                                                                                                                                                  |
| ---------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| external   | bool   | external is a flag whether the donor is in the outside of the own cluster or not (default: false). If external=true, the MySQL users for MOCO will be automatically restored |
| donor_host | string | donor_host is the donor host in the own cluster (only has meaning if external=false)                                                                                         |
| donor_port | int32  | donor_port is the port number where the donor host is listening (only has meaning if external=false)                                                                         |

### CloneResponse
CloneResponse is a response message against to CloneRequest.

### BackupBinlogService
BackupBinlogService is a service for flushing binlogs and backup them.

| Method Name          | Request Type                                                | Response Type                                                 | Description                                                                        |
| -------------------- | ----------------------------------------------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| FlushAndBackupBinlog | [FlushAndBackupBinlogRequest](#FlushAndBackupBinlogRequest) | [FlushAndBackupBinlogResponse](#FlushAndBackupBinlogResponse) | FlushAndBackupBinlog invokes MySQL `FLUSH BINARY LOGS` and backup process of them. |
| FlushBinlog          | [FlushBinlogRequest](#FlushBinlogRequest)                   | [FlushBinlogResponse](#FlushBinlogResponse)                   | FlushBinlog invokes MySQL `FLUSH BINARY LOGS`                                      |

### FlushAndBackupBinlogRequest
FlushAndBackupBinlogRequest is the request message to invoke MySQL `FLUSH BINARY LOGS` command
and upload the flushed binlog files to the given object storage bucket.


| Field             | Type   | Description                                                          |
| ----------------- | ------ | -------------------------------------------------------------------- |
| backup_id         | string | backup_id is the unique id of this backup process                    |
| bucket_host       | string | backet_host is the host address of the object storage                |
| bucket_port       | int32  | bucket_port is the port number where the object storage is listening |
| bucket_name       | string | bucket_name is the bucket name where the backup files are uploaded   |
| bucket_region     | string | bucket_region is the region name of the bucket                       |
| access_key_id     | string | access_key_id is used for authentication on the object storage       |
| secret_access_key | string | secret_access_key is used for authentication on the object storage   |

### FlushAndBackupBinlogResponse
FlushAndBackupBinlogResponse is a response message against to FlushANdBackupBinlogRequest.

### FlushBinlogRequest
FlushBinlogRequest is the request message to invoke MySQL `FLUSH BINARY LOGS` command.

| Field  | Type | Description                                                                                 |
| ------ | ---- | ------------------------------------------------------------------------------------------- |
| delete | bool | delete is the flag whether the flushed binlog files will be deleted or not (default: false) |

### FlushBinlogResponse
FlushBinlogResponse is a response message against to FlushBinlogRequest.
