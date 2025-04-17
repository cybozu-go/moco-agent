[![GitHub release](https://img.shields.io/github/release/cybozu-go/moco-agent.svg?maxAge=60)][releases]
[![CI](https://github.com/cybozu-go/moco-agent/actions/workflows/ci.yaml/badge.svg)](https://github.com/cybozu-go/moco-agent/actions/workflows/ci.yaml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/cybozu-go/moco-agent?tab=overview)](https://pkg.go.dev/github.com/cybozu-go/moco-agent?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/moco-agent)](https://goreportcard.com/report/github.com/cybozu-go/moco-agent)

# MOCO Agent

MOCO Agent is a sidecar program of [MOCO][].

## Documentation

[docs](docs/) directory contains documents about designs and specifications.

## Docker images

Docker images are available on [GitHub Container Registry](https://github.com/orgs/cybozu-go/packages/container/package/moco-agent)

[releases]: https://github.com/cybozu-go/moco-agent/releases
[MOCO]: https://github.com/cybozu-go/moco

## Development

### Prerequisites

This project uses [aqua](https://aquaproj.github.io/) to manage development tools. Before starting development, please install aqua by following the instructions at https://aquaproj.github.io/docs/install.

Once aqua is installed, the required development tools will be automatically installed when you run make commands.
