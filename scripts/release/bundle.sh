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
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/ardentsctl" ./cmd/ardentsctl
    CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/ardentsd" ./cmd/ardentsd
    cp README.md "$stage/README.md"
    cp LICENSE "$stage/LICENSE"
    install -d -m 0755 "$stage/docker" "$stage/docs" "$stage/scripts/deploy" "$stage/scripts/install" "$stage/systemd"
    cp deploy/docker/README.md deploy/docker/compose/docker-compose.multinode.yml deploy/docker/compose/docker-compose.production.yml \
        "$stage/docker/"
    cp ardents.ps1 "$stage/ardents.ps1"
    cp scripts/deploy/cluster.ps1 scripts/deploy/data.ps1 scripts/deploy/rollout.ps1 \
        "$stage/scripts/deploy/"
    cp scripts/install/linux.sh scripts/install/smoke.sh scripts/install/systemd-smoke.sh "$stage/scripts/install/"
    chmod 0755 "$stage/scripts/install/linux.sh" "$stage/scripts/install/smoke.sh" "$stage/scripts/install/systemd-smoke.sh"
    cp deploy/systemd/ardentsd.service "$stage/systemd/"
    cp docs/operations/deployment-contract.md docs/operations/operator-runbook.md docs/operations/upgrade-migration.md \
        docs/operations/incident-response.md docs/operations/native-linux-installation.md \
        docs/security/persistent-state-security.md docs/product/supported-platforms.md \
        "$stage/docs/"
    tar --sort=name --mtime="@${SOURCE_DATE_EPOCH}" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - . |
        gzip -n > "/out/ardents-${ARDENTS_VERSION}-linux-${arch}.tar.gz"
    cp "$stage/ardentsctl" "/out/ardentsctl-linux-${arch}"
    cp "$stage/ardentsd" "/out/ardentsd-linux-${arch}"
done

go list -m -f '{{.Path}}\t{{.Version}}\t{{.Sum}}' all > /out/modules.tsv
