<!--
SPDX-FileCopyrightText: 2026 wavever
SPDX-FileCopyrightText: 2026 KIM Hyunjae
SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Contributing

Thanks for helping improve `limitping`.

## Development Setup

Nix with flakes enabled is required. The supported source build is `nix build`;
run development commands inside `nix develop`. Provider-specific tests
may also require the provider CLI or credentials.

Build the CLI, then enter the development shell for checks:

```sh
nix build
nix develop
prek run --all-files
go vet ./...
go test ./...
```

## Local Smoke Tests

Inside `nix develop`, after `nix build`:

```sh
./result/bin/limitping version
./result/bin/limitping config path
./result/bin/limitping ping --dry-run
./result/bin/limitping watch --dry-run
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
- [ ] `prek run --all-files` passes
- [ ] `nix build` passes
- [ ] `go vet ./...` passes inside `nix develop`
- [ ] `go test ./...` passes inside `nix develop`
- [ ] README or config examples are updated when user-facing behavior changes
- [ ] No credentials, raw usage responses, or private account metadata are
      included in tests, fixtures, logs, screenshots, or docs

## Security Reports

Do not open public issues containing credentials, exploit details, or raw
provider responses. See `SECURITY.md`.
