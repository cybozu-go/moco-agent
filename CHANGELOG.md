# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/cybozu-go/moco-agent/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/cybozu-go/moco-agent/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cybozu-go/moco-agent/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cybozu-go/moco-agent/compare/0913cef5607fd11e17ec2f5679059269fe4371fb...v0.1.0
