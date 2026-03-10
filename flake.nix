{
  description = "Mattermost multi-agent platform dev shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_24
            nodejs_24
            docker
            docker-compose
            golangci-lint
            postgresql_16
            jq
            git
            gh
          ];
          shellHook = ''
            export GOPATH="$HOME/.go"
            export PATH="$GOPATH/bin:$PATH"
          '';
        };
      });
}
