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
