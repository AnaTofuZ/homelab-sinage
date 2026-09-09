{
  description = "Home Signal — Go + BarefootJS household signage";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.go-overlay = {
    url = "git+https://github.com/purpleclay/go-overlay?ref=main";
    flake = false;
  };

  outputs =
    {
      self,
      nixpkgs,
      go-overlay,
      ...
    }:
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
          go = (pkgs.extend (import go-overlay)).go-bin.latestStable;
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
            npmDepsHash = "sha256-mByRR8egx6T7r50j/Otk1YdhgioDjs+jrdWcak5dD3o=";
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
          inherit go;
          golangci-lint = go.tools.golangci-lint.latest;
          default = (pkgs.buildGoModule.override { inherit go; }) {
            pname = "homelab-signage";
            version = "0.1.0";
            inherit src;
            vendorHash = "sha256-3UBKMXOOgRu8fZ65/ppTuneI8Q/ANeyZ/WmUmtkZpGo=";
            subPackages = [ "." ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            nativeCheckInputs = [ self.packages.${system}.golangci-lint ];
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
              self.packages.${system}.go
              self.packages.${system}.golangci-lint
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
