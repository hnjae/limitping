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

        perSystem = { config, pkgs, ... }: {
          treefmt = {
            projectRootFile = "flake.nix";
            programs.gofmt.enable = true;
            programs.nixfmt.enable = true;
            programs.taplo.enable = true;
            settings.formatter.rumdl = {
              command = "${pkgs.rumdl}/bin/rumdl";
              options = [ "fmt" ];
              includes = [ "*.md" ];
            };
            programs.yamlfmt.enable = true;
          };

          pre-commit.settings = {
            package = pkgs.prek;
            hooks.treefmt.enable = true;
          };

          devShells.default = pkgs.mkShellNoCC {
            inputsFrom = [ config.treefmt.build.devShell ];
            packages = [
              pkgs.go
              pkgs.prek
            ]
            ++ config.pre-commit.settings.enabledPackages;

            shellHook = config.pre-commit.shellHook + ''
              if [ ! -e treefmt.toml ] || [ -L treefmt.toml ]; then
                ln -sfn ${config.treefmt.build.configFile} treefmt.toml
              fi
            '';
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
