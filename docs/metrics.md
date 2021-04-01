Metrics
=======

MOCO agent exposes the following metrics in the Prometheus format.
All these metrics names are prefixed with `moco_agent_`

| Name                            | Description                                       | Type    | Labels    |
| ------------------------------- | ------------------------------------------------- | ------- | --------- |
| `clone_count`                   | The clone operation count                         | Counter | `cluster` |
| `clone_failure_count`           | The failed clone operation count                  | Counter | `cluster` |
| `clone_duration_seconds`        | The time took to clone operation                  | Summary | `cluster` |
| `clone_in_progress`             | Whether the clone operation is in progress or not | Gauge   | `cluster` |
| `log_rotation_count`            | The log rotation count                            | Counter | `cluster` |
| `log_rotation_failure_count`    | The failed log rotation count                     | Counter | `cluster` |
| `log_rotation_duration_seconds` | The time took to log rotation                     | Summary | `cluster` |

In addition to the above metrics, the following metrics are included:

- [Process metrics](https://github.com/prometheus/client_golang/blob/17e98a7e4fa630ca36cfbab6eea4e551290f819e/prometheus/process_collector.go#L75)
- [Go runtime metrics](https://github.com/prometheus/client_golang/blob/17e98a7e4fa630ca36cfbab6eea4e551290f819e/prometheus/go_collector.go#L65)
- [gRPC metrics](https://github.com/grpc-ecosystem/go-grpc-prometheus/blob/master/README.md#metrics)
