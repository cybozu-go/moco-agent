# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.7.0] - 2021-11-15

### Added
- `boot_timeout` parameter to CloneRequest (#64)

## [0.6.9] - 2021-07-29

### Changed
- Change LICENSE from MIT to Apache2 (#59)

### Fixed
- Grant PROXY privilege to AdminUser (#58)

## [0.6.8] - 2021-06-24

### Changed
- Complete clone operation even if a cancel signal is received (#55)

## [0.6.7] - 2021-06-22

### Changed
- Set KeepaliveEnforcementPolicy (#53)

## [0.6.6] - 2021-06-04

### Fixed
- Timeout error when cloning large data (#52).

## [0.6.5] - 2021-05-17

### Changed
- Improve replication lag detection (#51).

## [0.6.4] - 2021-05-14

### Changed
- Grant `REPLICATION SLAVE` privilege to `moco-backup` (#50).

## [0.6.3] - 2021-05-09

### Changed
- Implement mTLS for gRPC. (#49)

## [0.6.2] - 2021-05-06

### Added
- `moco-exporter` and `moco-backup` users. (#47)

## [0.6.1] - 2021-04-26

### Added
- New metric `moco_instance_replication_delay_seconds` (#45)

### Changed
- Reset user passwords after clone (#45)
- Change metrics names (#45)

## [0.6.0] - 2021-04-02

### Changed
- Redesign and re-implement moco-agent (#41)

## [0.5.0] - 2021-03-15

### Changed

- Set state as not ready if MySQL thread has an error. (#25)
- Drop root user. (#28)
- Update Go to 1.16 (#29)

### Added

- Add metrics representing in-progress state. (#20)

## [0.4.0] - 2021-03-02

### Changed

- Switch APIs from HTTP to gRPC server. (#24)

### Added

- Publish API proto file at GitHub release page (#24)

## [0.3.0] - 2021-02-26

### Added

- Export metrics of backup binlog API. (#9)
- Move the config generation feature from the moco-conf-gen. (#14)

## [0.2.1] - 2021-02-22

### Changed

- Remove files in data directory before initialization. (#16)

## [0.2.0] - 2021-02-18

### Added

- Add backup binlog API. (#6)
- Deprecate the rotate API and use goroutine to periodically run cron job. (#10)

## [0.1.0] - 2021-02-01

### Added

- Move moco agent code from cybozu-go/moco repo. (#1)
- Move ping function from shellscript to moco-agent. (#4)

[Unreleased]: https://github.com/cybozu-go/moco-agent/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/cybozu-go/moco-agent/compare/v0.6.9...v0.7.0
[0.6.9]: https://github.com/cybozu-go/moco-agent/compare/v0.6.8...v0.6.9
[0.6.8]: https://github.com/cybozu-go/moco-agent/compare/v0.6.7...v0.6.8
[0.6.7]: https://github.com/cybozu-go/moco-agent/compare/v0.6.6...v0.6.7
[0.6.6]: https://github.com/cybozu-go/moco-agent/compare/v0.6.5...v0.6.6
[0.6.5]: https://github.com/cybozu-go/moco-agent/compare/v0.6.4...v0.6.5
[0.6.4]: https://github.com/cybozu-go/moco-agent/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/cybozu-go/moco-agent/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/cybozu-go/moco-agent/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/cybozu-go/moco-agent/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/cybozu-go/moco-agent/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/cybozu-go/moco-agent/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/cybozu-go/moco-agent/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/cybozu-go/moco-agent/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/cybozu-go/moco-agent/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cybozu-go/moco-agent/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cybozu-go/moco-agent/compare/0913cef5607fd11e17ec2f5679059269fe4371fb...v0.1.0
