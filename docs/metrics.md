Metrics
=======

MOCO agent exposes the following metrics in the Prometheus format.
All these metrics have `moco_instance_` as a prefix of their names and have `name` and `index` labels.

`name` indicates the name of MySQLCluster.  `index` is the index of the instance such as `0`, `1`, or `2`.

| Name                            | Description                                                   | Type    |
| ------------------------------- | ------------------------------------------------------------- | ------- |
| `replication_delay_seconds`     | The seconds how much delay to replicate data from the primary | Gauge   |
| `clone_count`                   | The clone operation count                                     | Counter |
| `clone_failure_count`           | The failed clone operation count                              | Counter |
| `clone_duration_seconds`        | The time took to clone operation                              | Summary |
| `clone_in_progress`             | Whether the clone operation is in progress or not             | Gauge   |
| `log_rotation_count`            | The log rotation count                                        | Counter |
| `log_rotation_failure_count`    | The failed log rotation count                                 | Counter |
| `log_rotation_duration_seconds` | The time took to log rotation                                 | Summary |

In addition to the above metrics, the following metrics are included:

- [Process metrics](https://github.com/prometheus/client_golang/blob/17e98a7e4fa630ca36cfbab6eea4e551290f819e/prometheus/process_collector.go#L75)
- [Go runtime metrics](https://github.com/prometheus/client_golang/blob/17e98a7e4fa630ca36cfbab6eea4e551290f819e/prometheus/go_collector.go#L65)
- [gRPC metrics](https://github.com/grpc-ecosystem/go-grpc-prometheus/blob/master/README.md#metrics)
