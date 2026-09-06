{
  description = "Home Signal — Go + BarefootJS household signage";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

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
            npmDepsHash = "sha256-aEN5gRRu7uhoBMqdFeZ3B8hfbIdlFVhw0AIFv9UNJ0E=";
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
          default = pkgs.buildGoModule {
            pname = "homelab-signage";
            version = "0.1.0";
            inherit src;
            vendorHash = "sha256-eL9aO7pUEre/S55JODRRg0RkjlIjN+wdgBdfK+4aOgI=";
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
              pkgs.go
              pkgs.golangci-lint
              pkgs.nodejs_22
            ];
          };
        }
      );
      formatter = eachSystem (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);
    };
}
