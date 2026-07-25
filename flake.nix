{
  description = "pw, the CLI of the Popcorn Wave web application framework";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      # requirement:cli-distribution. Bumped in the release preparation commit
      # that precedes the tag; see flow:cli-release.
      baseVersion = "0.0.0";

      version = if self ? rev then baseVersion else "${baseVersion}-${self.dirtyShortRev or "dirty"}";

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forEachSystem = nixpkgs.lib.genAttrs systems;
      pkgsFor = system: nixpkgs.legacyPackages.${system};
    in
    {
      overlays.default = final: _prev: {
        pw = final.callPackage ./nix/pw.nix {
          src = self;
          inherit version;
        };
      };

      packages = forEachSystem (
        system:
        let
          pw = (pkgsFor system).callPackage ./nix/pw.nix {
            src = self;
            inherit version;
          };
        in
        {
          inherit pw;
          default = pw;
        }
      );

      apps = forEachSystem (system: rec {
        pw = {
          type = "app";
          program = "${self.packages.${system}.pw}/bin/pw";
        };
        default = pw;
      });

      devShells = forEachSystem (
        system:
        let
          pkgs = pkgsFor system;
        in
        {
          default = pkgs.mkShell {
            # Host tools of decision:host-tools-target-runtime. Tailwind stays
            # with Devbox per decision:tailwind-host-toolchain.
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gotools
              pkgs.tinygo
            ];
          };
        }
      );

      formatter = forEachSystem (system: (pkgsFor system).nixfmt-rfc-style);
    };
}
