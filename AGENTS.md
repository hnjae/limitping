# Repository Guidelines

## Project Overview

`limitping` is a Go CLI that keeps Claude Code and Codex rolling usage windows continuous. `ping` triggers a small billable request through the provider’s official CLI; `watch` waits for resets and triggers automatically. `status` reads usage and local transcript-derived daily token/cost totals. Usage reads are separate from quota-consuming triggers; Codex reset-credit redemption is an explicit irreversible operation.

## Architecture & Data Flow

The executable in `cmd/limitping` delegates to the Cobra command layer in `internal/cli`. The CLI loads TOML configuration, selects providers, and wires provider implementations into scheduler targets. Providers read their respective usage endpoints and normalize responses to `internal/usage`; triggers use the official `claude` or `codex` CLI rather than usage APIs. `internal/scheduler` runs cancellable per-provider watch loops, applying reset and weekly-limit policy before triggering. `status` combines provider usage with local transcript totals from `internal/spend` and model costs from `internal/pricing`.

Keep provider-specific parsing and API details in `internal/provider`; preserve `usage.Usage` as the shared model. Keep CLI construction centralized in `internal/cli`, and keep optional provider capabilities (such as reset-credit redemption) behind narrow optional interfaces. Status JSON has its own projection in `internal/cli/status.go`; do not serialize internal usage structs directly.

## Key Directories

- `cmd/limitping/` — executable entry point.
- `internal/cli/`, `internal/config/`, `internal/usage/` — command behavior and wiring, TOML defaults/path resolution, and provider-neutral usage types/policies.
- `internal/provider/`, `internal/auth/` — provider API/CLI integrations and reuse/refresh of provider-owned credentials.
- `internal/scheduler/` — cancellable watch orchestration and reset/weekly-limit decisions.
- `internal/spend/`, `internal/pricing/`, `internal/notify/` — local transcript accounting, cost lookup, and platform notifications.
- `nix/partitions/dev/` — Nix development environment and tooling configuration.

`README.md` is the primary user guide; it documents install, commands, configuration, provider prerequisites, and operational/privacy caveats.

## Development Commands

Run from the repository root. `just` lists the available recipes.

```sh
just build       # nix build
just test        # go test ./...
just vet         # go vet ./...
just format      # nix fmt
nix flake check  # CI checks, including vet and race/coverage tests
```

For a safe CLI smoke run after building/installing, use `limitping config init`, `limitping status`, `limitping ping --dry-run`, or `limitping watch --dry-run`. See `README.md` for provider setup and Nix installation instructions.

## Code Conventions & Common Patterns

- Use idiomatic Go and format Go changes with `gofmt` via `nix fmt`.
- Thread `context.Context` through network, provider, and long-running scheduler operations; timers and loops must remain cancellable.
- Wrap errors with `%w` when callers need classification, and preserve HTTP status details used by scheduler policy. Use typed errors and `errors.As` where behavior depends on an error kind.
- Keep provider-specific behavior behind `provider.Provider` and normalize results at the provider boundary. Use a narrow optional interface for capabilities not shared by every provider.
- Keep shared mutable auth and pricing cache state synchronized; follow existing mutex patterns.
- Follow `internal/auth` for reuse, refresh, and persistence of official CLI credentials; route quota-consuming triggers through provider CLIs. Do not add a separate login flow or use read-only usage endpoints for triggering.

## Important Files

- `cmd/limitping/main.go` — process entry point and top-level error handling.
- `internal/cli/cli.go`, `internal/cli/status.go` — Cobra commands, provider wiring, and status output contract.
- `internal/provider/provider.go`, `internal/usage/usage.go`, `internal/scheduler/scheduler.go` — provider boundary, shared usage model, and watch orchestration.
- `internal/config/config.go`, `internal/spend/spend.go`, `internal/pricing/pricing.go` — configuration, local spend aggregation, and pricing.
- `go.mod`, `flake.nix`, `justfile`, `.github/workflows/ci.yml` — Go module/toolchain, Nix build/checks, contributor commands, and CI.

## Runtime/Tooling Preferences

The module declares Go 1.26.7. Nix flakes are the repository-native build and development environment; `just` provides convenience aliases, while dependencies are managed as a Go module. Triggering requires the relevant official provider CLI (`claude` and/or `codex`) and its existing credentials. `README.md` documents supported platforms and provider-specific setup.

## Testing & QA

Tests use Go’s standard `testing` package in package-local `*_test.go` files. Follow existing table-driven and `t.Run` patterns; isolate filesystem/environment state with `t.TempDir` and `t.Setenv`, and stub HTTP behavior rather than depending on live provider services. For asynchronous scheduler behavior, use controlled stubs and cancellation.

Run `go test ./...` (or `just test`) for the suite and `go vet ./...` (or `just vet`) for vetting. `nix flake check` is the CI-level check and runs vet plus race-enabled tests with coverage. Add or update tests for observable behavior and boundary changes; avoid assertions on incidental wording, formatting, or private implementation details.
