{
  description = "Home Signal — Go + BarefootJS household signage";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-darwin"
      ];
      eachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter =
              path: type:
              let
                name = baseNameOf path;
              in
              !builtins.elem name [
                ".git"
                "dist"
                "node_modules"
              ];
          };
          frontend = pkgs.buildNpmPackage {
            pname = "homelab-signage-ui";
            version = "0.1.0";
            inherit src;
            npmDepsHash = "sha256-BEeyo3NMtjE1kWRBrnTthehkl9BsIqvp03xPtQTJiWA=";
            npmBuildScript = "build:ui";
            doCheck = true;
            checkPhase = ''
              npm run format:check
              npm run lint
            '';
            installPhase = ''
              mkdir -p $out
              cp -r dist components.go $out/
            '';
          };
        in
        {
          default = pkgs.buildGo127Module {
            pname = "homelab-signage";
            version = "0.1.0";
            inherit src;
            vendorHash = "sha256-US6OBsxN7jAxfC3z3kJCH3sxeGbEQQXalFeHul14Zkk=";
            subPackages = [ "." ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            nativeCheckInputs = [ pkgs.golangci-lint ];
            preBuild = ''
              cp -r ${frontend}/dist .
              cp ${frontend}/components.go .
            '';
            checkPhase = ''
              runHook preCheck
              export GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint"
              golangci-lint run ./...
              go test ./...
              runHook postCheck
            '';
            postInstall = ''
              mkdir -p $out/share/homelab-signage
              cp -r dist public $out/share/homelab-signage/
              wrapProgram $out/bin/homelab-signage \
                --chdir $out/share/homelab-signage \
                --set-default APP_ENV production
            '';
            meta.mainProgram = "homelab-signage";
          };
        }
      );

      checks = eachSystem (system: {
        inherit (self.packages.${system}) default;
      });
      devShells = eachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go_1_27
              pkgs.golangci-lint
              pkgs.nodejs_22
            ];
            shellHook = ''
              unset GOROOT
              export GOCACHE="$PWD/.cache/go-build"
              export GOLANGCI_LINT_CACHE="$PWD/.cache/golangci-lint"
            '';
          };
        }
      );
      formatter = eachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);
    };
}
