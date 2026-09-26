<!--
SPDX-FileCopyrightText: 2026 wavever
SPDX-FileCopyrightText: 2026 KIM Hyunjae
SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Contributing

Thanks for helping improve `limitping`.

## Development Setup

Nix with flakes enabled is required. `nix flake check` is the canonical CI command; it builds the CLI and runs formatting, Go analysis/tests, and release-config validation. Use `nix develop` for interactive Go and repository-hook commands. Provider-specific tests may also require the provider CLI or credentials.

Run the full CI check set from the repository root:

```sh
nix flake check
```

Inside `nix develop`, run repository hooks with `prek run --all-files`.

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
- [ ] `nix flake check` passes
- [ ] README or config examples are updated when user-facing behavior changes
- [ ] No credentials, raw usage responses, or private account metadata are included in tests, fixtures, logs, screenshots, or docs

## Security Reports

Do not open public issues containing credentials, exploit details, or raw
provider responses. See `SECURITY.md`.
