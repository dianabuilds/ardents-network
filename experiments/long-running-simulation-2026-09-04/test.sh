#!/bin/bash
# Local pre-flight for the S3.1 sim-driver. Runs the same focused
# build + vet + self-test cycle as the slice 2 test.sh, but on
# the S3.1 source files. The Docker-backed 100-tick smoke is run
# via `docker compose --profile build up` and is a separate
# acceptance gate (AC4); this script is the AC1/AC2/AC3/AC5 gate.
#
# Windows host notes:
# - `go test -race` is intentionally skipped. mingw gcc on the
#   Windows dev host is missing -ldl, so the Go race detector
#   cannot link. Carrier Lab on Linux is where race tests run.
# - The build-ignored //go:build tag means the files are not part
#   of the maintained root module; they MUST be passed to
#   `go` as explicit file paths, not as a package import path.

set -eo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DRIVER="$ROOT/experiments/long-running-simulation-2026-09-04/cmd/sim-driver"

GO=go
if ! command -v "$GO" >/dev/null 2>&1; then
    if [ -x "/c/Program Files/Go/bin/go.exe" ]; then
        GO="/c/Program Files/Go/bin/go.exe"
    else
        echo "sim-driver: go executable not found" >&2
        exit 2
    fi
fi

SOURCES=(
    "$DRIVER/doc.go"
    "$DRIVER/main.go"
    "$DRIVER/timekeeper.go"
    "$DRIVER/observer.go"
    "$DRIVER/tripwires.go"
    "$DRIVER/fixtures.go"
    "$DRIVER/selftest.go"
    "$DRIVER/credentials.go"
    "$DRIVER/personas.go"
    "$DRIVER/useractor.go"
)

echo "== AC1 build =="
"$GO" build -o NUL "${SOURCES[@]}"
echo "build OK"

echo "== AC2 vet =="
"$GO" vet "${SOURCES[@]}"
echo "vet OK"

echo "== AC3 + AC5 self-test =="
"$GO" run "${SOURCES[@]}" self-test
echo "self-test OK"

echo "== sim-driver pre-flight done =="
