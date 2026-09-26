# SPDX-FileCopyrightText: 2026 KIM Hyunjae
# SPDX-License-Identifier: AGPL-3.0-or-later

{
  inputs = {
    nixpkgs.url = "https://flakehub.com/f/DeterminateSystems/nixpkgs-weekly/0";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };
  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      imports = [ inputs.flake-parts.flakeModules.partitions ];

      partitions.dev.extraInputsFlake = ./nix/partitions/dev;
      partitions.dev.module = { inputs, ... }: {
        imports = [
          inputs.git-hooks.flakeModule
          inputs.treefmt-nix.flakeModule
        ];

        perSystem =
          {
            config,
            pkgs,
            lib,
            ...
          }:
          {
            treefmt.projectRootFile = "flake.nix";
            treefmt.programs = {
              gofmt.enable = true;
              nixfmt.enable = true;
              rumdl-format.enable = true;
              taplo.enable = true;
              yamlfmt.enable = true;
              just.enable = true;
            };

            pre-commit.settings.package = pkgs.prek;
            pre-commit.settings.hooks = {
              cocogitto = {
                enable = true;
                name = "cog verify";
                description = "Lint commit messages with Cocogitto.";
                package = pkgs.cocogitto;
                entry = "${lib.getExe pkgs.cocogitto} verify --file";
                stages = [ "commit-msg" ];
              };
              detect-private-keys.enable = true;
              treefmt.enable = true;
              typos.enable = true;
              reuse.enable = true;
            };
            checks = {
              limitping = pkgs.buildGoModule {
                pname = "limitping";
                version = "unstable";
                src = ./.;
                vendorHash = "sha256-M6lE7Dk/f8+PLY+8uS5lbEhPnez0OUDmyWdZnwnIQ+Y=";
                subPackages = [ "cmd/limitping" ];
                env.CGO_ENABLED = "0";
                nativeBuildInputs = [ pkgs.stdenv.cc ];
                doCheck = true;
                checkPhase = ''
                  runHook preCheck
                  go vet ./...
                  CGO_ENABLED=1 go test -race -coverprofile=coverage.out -covermode=atomic ./...
                  go tool cover -func=coverage.out
                  runHook postCheck
                '';
              };
            };

            devShells.default = pkgs.mkShellNoCC {
              inputsFrom = [ config.treefmt.build.devShell ];
              packages = [
                pkgs.go
              ]
              ++ config.pre-commit.settings.enabledPackages;

              shellHook = lib.concatLines [
                config.pre-commit.shellHook
                # sh
                ''
                  if [ ! -e treefmt.toml ] || [ -L treefmt.toml ]; then
                    ln -sfn ${config.treefmt.build.configFile} treefmt.toml
                  fi
                ''
              ];
            };
          };
      };

      partitionedAttrs = {
        checks = "dev";
        devShells = "dev";
        formatter = "dev";
      };

      perSystem = { pkgs, ... }: {
        packages.default = pkgs.buildGoModule {
          pname = "limitping";
          version = "unstable";
          src = ./.;
          vendorHash = "sha256-M6lE7Dk/f8+PLY+8uS5lbEhPnez0OUDmyWdZnwnIQ+Y=";
          subPackages = [ "cmd/limitping" ];
          env.CGO_ENABLED = "0";
        };
      };
    };
}
