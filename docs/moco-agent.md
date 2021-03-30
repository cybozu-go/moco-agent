# moco-agent

This is the specification document of `moco-agent` command.

## Command-line options

```
Flags:
      --address string                 Listening address and port for gRPC API. (default ":9080")
      --connection-timeout duration    Dial timeout (default 5s)
  -h, --help                           help for moco-agent
      --log-rotation-schedule string   Cron format schedule for MySQL log rotation (default "*/5 * * * *")
      --logfile string                 Log filename
      --logformat string               Log format [plain,logfmt,json]
      --loglevel string                Log level [critical,error,warning,info,debug]
      --max-delay duration             Acceptable max commit delay considering as ready (default 1m0s)
      --max-idle-time duration         The maximum amount of time a connection may be idle (default 30s)
      --metrics-address string         Listening address and port for metrics. (default ":8080")
      --probe-address string           Listening address and port for mysqld health probes. (default ":9081")
      --read-timeout duration          I/O read timeout (default 30s)
      --socket-path string             Path of mysqld socket file. (default "/run/mysqld.sock")
```

## Environment variables

moco-agent requires the following environment variables to initialize MySQL users.
All of them are required.

| Name                   | Description                                      |
| ---------------------- | ------------------------------------------------ |
| `POD_NAME`             | The Kubernetes Pod name of this mysqld.          |
| `CLUSTER_NAME`         | `MySQLCluster` resource name that owns this Pod. |
| `ADMIN_PASSWORD`       | Password for `moco-admin` user.                  |
| `AGENT_PASSWORD`       | Password for `moco-agent` user.                  |
| `REPLICATION_PASSWORD` | Password for `moco-repl` user.                   |
| `CLONE_DONOR_PASSWORD` | Password for `moco-clone-donor` user.            |
| `READONLY_PASSWORD`    | Password for `moco-readonly` user.               |
| `WRITABLE_PASSWORD`    | Password for `moco-writable` user.               |
