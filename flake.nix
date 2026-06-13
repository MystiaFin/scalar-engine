{
  description = "Scalar dev shell";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            golangci-lint
            delve
            air
            sqlite
						ollama
          ];

          shellHook = ''
            echo "scalar dev shell"
            echo "go $(go version | awk '{print $3}')"
          '';
        };
      });
}
