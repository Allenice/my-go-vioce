#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
[[ "$(uname -s)" == Linux ]] || { echo 'Run Linux validation on Linux.' >&2; exit 1; }
export CGO_ENABLED=1
if [[ -n "$(gofmt -l main.go internal cmd)" ]]; then echo 'Run gofmt before validation.' >&2; exit 1; fi
go test -p 1 ./...
go vet ./...
./scripts/build-linux.sh
