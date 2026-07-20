#!/bin/sh
set -eu

: "${ARDENTS_VERSION:?ARDENTS_VERSION is required}"
: "${ARDENTS_COMMIT:?ARDENTS_COMMIT is required}"
: "${ARDENTS_BUILD_DATE:?ARDENTS_BUILD_DATE is required}"
: "${SOURCE_DATE_EPOCH:?SOURCE_DATE_EPOCH is required}"

ldflags="-s -w -X ardents/internal/buildinfo.Version=${ARDENTS_VERSION} -X ardents/internal/buildinfo.Commit=${ARDENTS_COMMIT} -X ardents/internal/buildinfo.BuildDate=${ARDENTS_BUILD_DATE}"

for arch in amd64; do
    stage="/out/stage-${arch}"
    install -d -m 0755 "$stage"
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/ard" ./cmd/ard
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/ardd" ./cmd/ardd
    cp README.md "$stage/README.md"
    cp LICENSE "$stage/LICENSE"
    install -d -m 0755 "$stage/docker" "$stage/docs" "$stage/scripts/deploy"
    cp docker/README.md docker/docker-compose.multinode.yml docker/docker-compose.production.yml \
        "$stage/docker/"
    cp ardents.ps1 "$stage/ardents.ps1"
    cp scripts/deploy/cluster.ps1 scripts/deploy/data.ps1 scripts/deploy/rollout.ps1 \
        "$stage/scripts/deploy/"
    cp docs/deployment-contract.md docs/operator-runbook.md docs/upgrade-migration.md \
        docs/incident-response.md docs/persistent-state-security.md docs/supported-platforms.md \
        "$stage/docs/"
    tar --sort=name --mtime="@${SOURCE_DATE_EPOCH}" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - . |
        gzip -n > "/out/ardents-${ARDENTS_VERSION}-linux-${arch}.tar.gz"
    cp "$stage/ard" "/out/ard-linux-${arch}"
    cp "$stage/ardd" "/out/ardd-linux-${arch}"
done

go list -m -f '{{.Path}}\t{{.Version}}\t{{.Sum}}' all > /out/modules.tsv
