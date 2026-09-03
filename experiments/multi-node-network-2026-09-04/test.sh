#!/bin/bash
# Run the build-ignored disposable driver's focused tests without making the
# experiment a package in the maintained root module.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DRIVER="$ROOT/experiments/multi-node-network-2026-09-04/cmd/test-driver"
GO=go
if ! command -v "$GO" >/dev/null 2>&1; then
    if [ -x "/c/Program Files/Go/bin/go.exe" ]; then
        GO="/c/Program Files/Go/bin/go.exe"
    else
        echo "test-driver: go executable not found" >&2
        exit 2
    fi
fi
SOURCES=(
    "$DRIVER/doc.go"
    "$DRIVER/main.go"
    "$DRIVER/convergence.go"
    "$DRIVER/encoding.go"
    "$DRIVER/epoch.go"
    "$DRIVER/fixtures.go"
    "$DRIVER/record.go"
    "$DRIVER/sourceplan.go"
)

"$GO" test "${SOURCES[@]}" "$DRIVER/sourceplan_test.go"
"$GO" run "${SOURCES[@]}" self-test
