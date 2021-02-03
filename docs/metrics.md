Metrics
=======

MOCO agent expose the following metrics with the Prometheus format.  All these metrics are prefixed with `moco_agent_`

| Name                          | Description                      | Type    | Labels       |
| ----------------------------- | -------------------------------- | ------- | ------------ |
| clone_count                   | The clone operation count        | Counter | cluster_name |
| clone_failure_count           | The failed clone operation count | Counter | cluster_name |
| clone_duration_seconds        | The time took to clone operation | Summary | cluster_name |
| log_rotation_count            | The log rotation count           | Counter | cluster_name |
| log_rotation_failure_count    | The failed log rotation count    | Counter | cluster_name |
| log_rotation_duration_seconds | The time took to log rotation    | Summary | cluster_name |
