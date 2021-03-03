Metrics
=======

MOCO agent expose the following metrics with the Prometheus format.  All these metrics are prefixed with `moco_agent_`

| Name                           | Description                                               | Type    | Labels               |
| ------------------------------ | --------------------------------------------------------- | ------- | -------------------- |
| binlog_backup_count            | The binlog backup operation count                         | Counter | cluster_name         |
| binlog_backup_failure_count    | The failed binlog backup operation count                  | Counter | cluster_name, action |
| binlog_backup_duration_seconds | The time took to binlog backup operation                  | Summary | cluster_name         |
| binlog_backup_in_progress      | Whether the binlog backup operation is in progress or not | Gauge   | cluster_name         |
| clone_count                    | The clone operation count                                 | Counter | cluster_name         |
| clone_failure_count            | The failed clone operation count                          | Counter | cluster_name         |
| clone_duration_seconds         | The time took to clone operation                          | Summary | cluster_name         |
| clone_in_progress              | Whether the clone operation is in progress or not         | Gauge   | cluster_name         |
| log_rotation_count             | The log rotation count                                    | Counter | cluster_name         |
| log_rotation_failure_count     | The failed log rotation count                             | Counter | cluster_name         |
| log_rotation_duration_seconds  | The time took to log rotation                             | Summary | cluster_name         |
| dump_backup_count              | The dump backup operation count                           | Counter | cluster_name         |
| dump_backup_failure_count      | The failed dump backup operation count                    | Counter | cluster_name, action |
| dump_backup_duration_seconds   | The time took to dump backup operation                    | Summary | cluster_name         |
| dump_backup_in_progress        | Whether the dump backup operation is in progress or not   | Gauge   | cluster_name         |

The `action` label indicates the operation in which the error occurred.
