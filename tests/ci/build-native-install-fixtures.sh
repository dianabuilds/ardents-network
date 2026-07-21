#!/bin/sh
set -eu

build_pair() {
    version=$1
    commit=$2
    output=$3
    flags="-X ardents/internal/buildinfo.Version=$version -X ardents/internal/buildinfo.Commit=$commit -X ardents/internal/buildinfo.BuildDate=2026-07-21T00:00:00Z"
    go build -trimpath -buildvcs=false -ldflags "$flags" -o "$output/ardentsd" ./cmd/ardentsd
    go build -trimpath -buildvcs=false -ldflags "$flags" -o "$output/ardentsctl" ./cmd/ardentsctl
}

build_pair v0.1.0 1111111 /release/v1
build_pair v0.2.0 2222222 /release/v2
flags="-X ardents/internal/buildinfo.Version=v0.3.0 -X ardents/internal/buildinfo.Commit=3333333 -X ardents/internal/buildinfo.BuildDate=2026-07-21T00:00:00Z"
go build -trimpath -buildvcs=false -ldflags "$flags" -o /release/bad/ardentsd ./tests/fixtures/failing-ardentsd
go build -trimpath -buildvcs=false -ldflags "$flags" -o /release/bad/ardentsctl ./cmd/ardentsctl
