#!/usr/bin/env bash
# Build the host-native Runs library and run the Go test suite on Linux.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
proto_dir="$repo_root/prototypes/mygo-run"
mygo_pin="v0.8.4-0.20261108220316-c4197b7fe562"

cd "$proto_dir"

echo "Building host-native Runs library for $(go env GOOS)/$(go env GOARCH)..."
go run "github.com/egoist/mygo/cmd/mygo@${mygo_pin}" native build -target host ./modules/runs

echo "Running Go tests..."
go test ./...
