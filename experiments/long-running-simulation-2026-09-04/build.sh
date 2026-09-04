#!/bin/bash
# Disposable build script for the R-138 long-running simulation
# experiment. Cross-compiles the maintained ardents + ardents-node
# commands, the slice 2 pilot's test-driver (for the one-shot
# prebake step), and the new sim-driver (for the tick loop) for
# linux/amd64 into /workspace/artifacts, which the compose file
# bind-mounts from the host evidence dir. The prebake, source, and
# sim-driver services all read the binaries from /workspace/artifacts,
# so the build output path must match that bind mount exactly —
# writing to /workspace/evidence/artifacts would put the binaries
# in the builder container's writable layer, where they would be
# lost on container exit and invisible to every downstream service.
#
# S3.1 (this slice) only needs the sim-driver + the slice 2
# prebake + the maintained ardents/ardents-node binaries; later
# slices will add the Adversary and Auditor roles here without
# changing the bind-mount contract.

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

# Reuse the slice 2 pilot's test-driver for the prebake step. The
# full source list is reproduced here (not imported as a Go package)
# because the test-driver is a build-ignored standalone binary
# that depends on its sibling files. The list MUST stay in sync
# with the slice 2 build.sh + cmd/test-driver/; if the slice 2
# driver gains a file, this list must gain it too.
test_driver_dir=./experiments/multi-node-network-2026-09-04/cmd/test-driver
test_driver_sources=(
    "$test_driver_dir/doc.go"
    "$test_driver_dir/main.go"
    "$test_driver_dir/prebake.go"
    "$test_driver_dir/verify.go"
    "$test_driver_dir/selftest.go"
    "$test_driver_dir/convergence.go"
    "$test_driver_dir/encoding.go"
    "$test_driver_dir/epoch.go"
    "$test_driver_dir/fixtures.go"
    "$test_driver_dir/record.go"
    "$test_driver_dir/sourceplan.go"
    "$test_driver_dir/verify_adversary.go"
)
test_driver_out="$ARTIFACTS/test-driver-linux-amd64"
go build $CANONICAL_FLAGS -o "$test_driver_out" "${test_driver_sources[@]}"
echo "built $test_driver_out"
sha256sum "$test_driver_out" >> "$ARTIFACTS/SHA256SUMS"

# Build the S3.1 sim-driver. Like the test-driver, the sim-driver
# is a build-ignored standalone binary; the source list mirrors
# cmd/sim-driver/. S3.1 adds 7 files (doc, main, timekeeper,
# observer, tripwires, fixtures, selftest); S3.6 appends 3 more
# (credentials, personas, useractor) for the UserActor role.
# Later slices will append to this list when they add the
# DriftInjector / Adversary / Auditor files.
sim_driver_dir=./experiments/long-running-simulation-2026-09-04/cmd/sim-driver
sim_driver_sources=(
    "$sim_driver_dir/doc.go"
    "$sim_driver_dir/main.go"
    "$sim_driver_dir/timekeeper.go"
    "$sim_driver_dir/observer.go"
    "$sim_driver_dir/tripwires.go"
    "$sim_driver_dir/fixtures.go"
    "$sim_driver_dir/selftest.go"
    "$sim_driver_dir/credentials.go"
    "$sim_driver_dir/personas.go"
    "$sim_driver_dir/useractor.go"
)
sim_driver_out="$ARTIFACTS/sim-driver-linux-amd64"
go build $CANONICAL_FLAGS -o "$sim_driver_out" "${sim_driver_sources[@]}"
echo "built $sim_driver_out"
sha256sum "$sim_driver_out" >> "$ARTIFACTS/SHA256SUMS"

echo "== builder done =="
