# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.14.0] - 2025-07-28

### Changed
- Manage CLI tools by aquaproj/aqua [#110](https://github.com/cybozu-go/moco-agent/pull/110)

### Added
- Add init-timezones flag to populate timezone data [#111](https://github.com/cybozu-go/moco-agent/pull/111)


## [0.13.1] - 2025-01-29

### Changed
- Upgrade go to 1.23, update dependencies and change test matrix [#106](https://github.com/cybozu-go/moco-agent/pull/106)

### ⚠️ End support for older versions
 - MySQL versions supported after this release are 8.0.28, 8.0.39, 8.0.40, 8.0.41 and 8.4.4

## [0.13.0] - 2024-11-29

### Added
- Add log-rotation-size option [#105](https://github.com/cybozu-go/moco-agent/pull/105)

### Changed
- Update golang version specified in Dockerfile [#106](https://github.com/cybozu-go/moco-agent/pull/106)

## [0.12.2] - 2024-09-10

### Changed
- Update dependencies [#102](https://github.com/cybozu-go/moco-agent/pull/102)

## [0.12.1] - 2024-06-20

### Changed
 - IsAccessDenied function is not determining correctly [#98](https://github.com/cybozu-go/moco-agent/issues/98)

## [0.12.0] - 2024-06-18

### ⚠️ Breaking Changes
 - MySQL Terminology Updates [#686](https://github.com/cybozu-go/moco/issues/686)

### Changed
 - Support MySQL 8.4.0 [#686](https://github.com/cybozu-go/moco/issues/686)

### ⚠️ End support for older versions
 - MySQL versions supported after this release are 8.0.28, 8.0.36, 8.0.37 and 8.4.0

## [0.11.0] - 2024-06-10

### Changed
- Replace custom-checker [#87](https://github.com/cybozu-go/moco-agent/pull/87)
- Upgrade go to 1.21 and update dependencies [#88](https://github.com/cybozu-go/moco-agent/pull/88)
- Upgrade direct dependencies, GitHub Actions and tools[#89](https://github.com/cybozu-go/moco-agent/pull/89)
- Fix moco#630 [#90](https://github.com/cybozu-go/moco-agent/pull/90)
- Migrate to ghcr.io[#91](https://github.com/cybozu-go/moco-agent/pull/91)
- Option to to use localhost instead of pod name[#92](https://github.com/cybozu-go/moco-agent/pull/92)
- Update dependencies [#93](https://github.com/cybozu-go/moco-agent/pull/93)

## [0.10.0] - 2023-08-04

### Changed
- Wait for transaction queueing on replica [#84](https://github.com/cybozu-go/moco-agent/pull/84)
- Expose replication delay metrics even if delay check is disabled [#85](https://github.com/cybozu-go/moco-agent/pull/85)

## [0.9.0] - 2023-03-07

### Added
- Support multi-platform [#79](https://github.com/cybozu-go/moco-agent/pull/79)

### Changed
- Build on Ubuntu 22.04 [#78](https://github.com/cybozu-go/moco-agent/pull/78)

## [0.8.0] - 2022-11-01

### Added
- Add moco-init and lower-case-table-names flags (#71)

### Changed
- Add description about --max-delay 0 (#70)
- Support MySQL 8.0.28 (#68)
- Update Go to 1.19 and some dependencies (#72)
- Ginkgo v2 (#73)
- Use gh command (#74)

## [0.7.1] - 2021-11-15

### Changed
- Update Go to 1.17 (#66)

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

[Unreleased]: https://github.com/cybozu-go/moco-agent/compare/v0.14.0...HEAD
[0.14.0]: https://github.com/cybozu-go/moco-agent/compare/v0.13.1...v0.14.0
[0.13.1]: https://github.com/cybozu-go/moco-agent/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/cybozu-go/moco-agent/compare/v0.12.2...v0.13.0
[0.12.2]: https://github.com/cybozu-go/moco-agent/compare/v0.12.1...v0.12.2
[0.12.1]: https://github.com/cybozu-go/moco-agent/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/cybozu-go/moco-agent/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/cybozu-go/moco-agent/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/cybozu-go/moco-agent/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/cybozu-go/moco-agent/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/cybozu-go/moco-agent/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/cybozu-go/moco-agent/compare/v0.7.0...v0.7.1
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
