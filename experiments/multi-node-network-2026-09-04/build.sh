#!/bin/bash
# Disposable build script for the multi-node pilot. Cross-compiles the four
# maintained commands plus the experiment's test-driver binary for linux/amd64
# into /workspace/artifacts, which the compose file bind-mounts from the host
# evidence dir. The prebake and node services all read the binaries from
# /workspace/artifacts, so the build output path must match that bind mount
# exactly — writing to /workspace/evidence/artifacts would put the binaries
# in the builder container's writable layer, where they would be lost on
# container exit and invisible to every downstream service.

set -eo pipefail

ARTIFACTS=/workspace/artifacts
mkdir -p "$ARTIFACTS"

cd /src

echo "== go version =="
go version

echo "== canonical build flags =="
CANONICAL_FLAGS="-trimpath -buildvcs=false"
echo "$CANONICAL_FLAGS"

for cmd in ./cmd/ardents ./cmd/ardents-node; do
    base=$(basename "$cmd")
    out="$ARTIFACTS/${base}-linux-amd64"
    go build $CANONICAL_FLAGS -o "$out" "$cmd"
    echo "built $out"
    sha256sum "$out" >> "$ARTIFACTS/SHA256SUMS"
done

test_driver_dir=./experiments/multi-node-network-2026-09-04/cmd/test-driver
test_driver_sources=(
    "$test_driver_dir/doc.go"
    "$test_driver_dir/main.go"
    "$test_driver_dir/convergence.go"
    "$test_driver_dir/encoding.go"
    "$test_driver_dir/epoch.go"
    "$test_driver_dir/fixtures.go"
    "$test_driver_dir/record.go"
    "$test_driver_dir/sourceplan.go"
)
test_driver_out="$ARTIFACTS/test-driver-linux-amd64"
go build $CANONICAL_FLAGS -o "$test_driver_out" "${test_driver_sources[@]}"
echo "built $test_driver_out"
sha256sum "$test_driver_out" >> "$ARTIFACTS/SHA256SUMS"

echo "== builder done =="
