<!--
SPDX-FileCopyrightText: 2026 wavever
SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Contributing

Thanks for helping improve `limitping`.

## Development Setup

Requirements:

- Go 1.25+
- The provider CLI or credentials required by the behavior you want to test

Build and test:

```sh
go mod download
gofmt -l .
go build ./...
go vet ./...
go test ./...
```

`gofmt -l .` should print nothing. If it prints file names, run `gofmt -w` on
those files before opening a pull request.

## Local Smoke Tests

```sh
go run ./cmd/limitping version
go run ./cmd/limitping config path
go run ./cmd/limitping ping --dry-run
go run ./cmd/limitping watch --dry-run
```

Avoid running non-dry-run `ping` or `watch` during development unless you intend
to consume a small amount of provider quota.

## Provider Changes

Providers are intentionally isolated:

- `internal/provider`: provider-specific usage reads and triggers
- `internal/auth`: credential loading and refresh
- `internal/config`: provider configuration defaults
- `internal/cli`: command wiring

When adding or changing a provider, include tests where the behavior can be
validated without real credentials or paid quota.

## Pull Request Checklist

- [ ] The change is focused and described clearly
- [ ] `gofmt -l .` prints nothing
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] README or config examples are updated when user-facing behavior changes
- [ ] No credentials, raw usage responses, or private account metadata are
      included in tests, fixtures, logs, screenshots, or docs

## Security Reports

Do not open public issues containing credentials, exploit details, or raw
provider responses. See `SECURITY.md`.
