# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.4.0] - 2021-03-02

### Changed

- Switch APIs from HTTP to gRPC server. (#24)

### Added

- Publish API proto file at GitHub release page (#23)

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

[Unreleased]: https://github.com/cybozu-go/moco-agent/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/cybozu-go/moco-agent/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/cybozu-go/moco-agent/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cybozu-go/moco-agent/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cybozu-go/moco-agent/compare/0913cef5607fd11e17ec2f5679059269fe4371fb...v0.1.0
