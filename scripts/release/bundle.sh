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
    while IFS='|' read -r source destination; do
        [ -n "$source" ] || continue
        install -d -m 0755 "$stage/$(dirname "$destination")"
        cp "$source" "$stage/$destination"
    done < scripts/release/distribution-files.txt
    chmod 0755 "$stage/scripts/install/linux.sh" "$stage/scripts/install/smoke.sh" "$stage/scripts/install/systemd-smoke.sh"
    tar --sort=name --mtime="@${SOURCE_DATE_EPOCH}" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - . |
        gzip -n > "/out/ardents-${ARDENTS_VERSION}-linux-${arch}.tar.gz"
    cp "$stage/ardentsctl" "/out/ardentsctl-${ARDENTS_VERSION}-linux-${arch}"
    cp "$stage/ardentsd" "/out/ardentsd-${ARDENTS_VERSION}-linux-${arch}"
    rm -rf "$stage"
done

go list -m -f '{{.Path}}\t{{.Version}}\t{{.Sum}}' all > /out/modules.tsv
