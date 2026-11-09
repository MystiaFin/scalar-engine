#!/usr/bin/env bash
# ./run.sh        start scalar
# ./run.sh test   run the tests
# ./run.sh pull   read older emails
set -e

cd "$(dirname "$0")"

# the nix dev shell has go and ollama
if ! command -v ollama >/dev/null && command -v nix >/dev/null && [ -z "$IN_NIX_SHELL" ]; then
	exec nix develop -c ./run.sh "$@"
fi

if ! command -v go >/dev/null; then
	echo "go not found, install it from https://go.dev/dl"
	exit 1
fi

case "$1" in
	test) go test ./... ;;
	pull) go run ./cmd/pull ;;
	"") go run ./cmd/main.go ;;
	*)
		echo "usage: ./run.sh [test|pull]"
		exit 1
		;;
esac
