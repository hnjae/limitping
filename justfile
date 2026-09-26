#!/usr/bin/env -S just --justfile
# SPDX-FileCopyrightText: 2026 KIM Hyunjae
# SPDX-License-Identifier: AGPL-3.0-or-later

set unstable
set fallback := false
set lazy

_:
    @just --list

[group('ci')]
format:
    nix fmt

[group('build')]
build:
    nix build

[group('ci')]
test:
    go test ./...

[group('ci')]
vet:
    go vet ./...
