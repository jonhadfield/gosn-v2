# gosn-v2

[![Build Status](https://www.travis-ci.org/jonhadfield/gosn-v2.svg?branch=master)](https://www.travis-ci.org/jonhadfield/gosn-v2) [![CircleCI](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master.svg?style=svg)](https://circleci.com/gh/jonhadfield/gosn-v2/tree/master) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/jonhadfield/gosn-v2/) [![Go Report Card](https://goreportcard.com/badge/github.com/jonhadfield/gosn-v2)](https://goreportcard.com/report/github.com/jonhadfield/gosn-v2)

`gosn-v2` is a Go library for building Standard Notes clients. It wraps authentication, sync, encryption, and caching flows while letting you integrate with the official or a self-hosted Standard Notes server.

## Highlights
- Handles the Standard Notes v004 encryption protocol out of the box.
- Ships caching helpers for delta syncs and reducing server load.
- Includes schema validators, CLI utilities, and fixtures for reproducible tests.

## Requirements
- Go 1.25.1 or later (see `go.mod`).
- Access to a Standard Notes server account for live integration tests.
- macOS, Linux, or Windows with the Go toolchain installed.

## Installation
```bash
GO111MODULE=on go get github.com/jonhadfield/gosn-v2
```

## Quick Start
```go
import "github.com/jonhadfield/gosn-v2"

// Sign in and establish a session
sio, err := gosn.SignIn(gosn.SignInInput{Email: "user@example.com", Password: "topsecret"})
if err != nil {
    log.Fatal(err)
}

// Perform an initial sync
so, err := gosn.Sync(gosn.SyncInput{Session: &sio.Session})
if err != nil {
    log.Fatal(err)
}

// Decrypt and work with your items
items, err := so.Items.DecryptAndParse(&sio.Session)
```

## Project Layout
- `auth/`, `session/`, `items/` — domain packages for authentication, session lifecycle, and note models.
- `crypto/` — key derivation, encryption, and signing helpers.
- `cache/` — tooling for encrypted sync snapshots and cache persistence.
- `docs/` — user guides and reference material; start with `docs/index.md`.
- `schemas/`, `test.json` — JSON schemas and fixtures for validation and integration tests.
- `bin/` — utility scripts for development and troubleshooting.

## Development Workflow
- `go build ./...` verifies every package compiles.
- `go test ./...` runs unit tests across the repository. Set `SN_SKIP_SESSION_TESTS=true` to skip live server checks.
- `make test` aggregates coverage into `coverage.txt`; `make fmt` applies `gofmt` and `goimports` to all Go files.
- `make lint` runs `golangci-lint` with the configured rule set; `make critic` enables additional `gocritic` analysis.

## Documentation & Support
- Browse the guides in `docs/` or the Go API reference on [pkg.go.dev](https://pkg.go.dev/github.com/jonhadfield/gosn-v2).
- For issues or feature requests, open a GitHub issue with reproduction steps or context about your Standard Notes setup.

## Contributing
Review the [Repository Guidelines](AGENTS.md) before opening a pull request. They cover project structure, testing expectations, and the commit/PR workflow.

## License
This project is distributed under the [MIT License](LICENSE).
