FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS build

ARG CARRIER_LAB_SOURCE_SHA256

RUN --network=none --mount=type=bind,from=go_archive,source=go1.26.5.linux-amd64.tar.gz,target=/input/go1.26.5.linux-amd64.tar.gz,ro \
    printf '%s  %s\n' \
      '5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053' \
      '/input/go1.26.5.linux-amd64.tar.gz' | sha256sum -c - \
    && mkdir -p /opt/toolchain \
    && tar -C /opt/toolchain -xzf /input/go1.26.5.linux-amd64.tar.gz

WORKDIR /workspace
COPY go.mod ./
COPY go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=go_vendor . ./vendor

ENV GOTOOLCHAIN=local \
    GOPROXY=off \
    GOSUMDB=off \
    GOWORK=off \
    GOFLAGS=-mod=vendor \
    CGO_ENABLED=0

RUN --network=none \
    --mount=type=tmpfs,target=/tmp/go-build \
    --mount=type=tmpfs,target=/tmp/go-mod \
    --mount=type=tmpfs,target=/tmp/go-tmp \
    GOCACHE=/tmp/go-build \
    GOMODCACHE=/tmp/go-mod \
    GOTMPDIR=/tmp/go-tmp \
    /opt/toolchain/go/bin/go build \
      -trimpath \
      -buildvcs=false \
      -ldflags=-buildid= \
      -o /out/carrier-lab \
      ./cmd/carrier-lab \
    && test "${#CARRIER_LAB_SOURCE_SHA256}" -eq 64 \
    && test -z "$(printf '%s' "$CARRIER_LAB_SOURCE_SHA256" | tr -d '0123456789abcdef')" \
    && printf '%s\n' "$CARRIER_LAB_SOURCE_SHA256" >/out/source.sha256

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS tools

COPY carrier-lab/tools.lock /input/carrier-lab-tools.lock
COPY --from=tool_bundle . /input/packages/

RUN --network=none set -eu; \
    expected_count="$(awk -F '\t' '$1 == "package" { count++ } END { print count+0 }' /input/carrier-lab-tools.lock)"; \
    actual_count="$(find /input/packages -mindepth 1 -maxdepth 1 -type f | wc -l)"; \
    test "$actual_count" = "$expected_count"; \
    find /input/packages -mindepth 1 -maxdepth 1 ! -type f -print -quit | grep -q . && exit 2 || true; \
    for package in /input/packages/*; do \
      filename="${package##*/}"; \
      awk -F '\t' -v filename="$filename" '$1 == "package" && $4 == filename { found=1 } END { exit !found }' /input/carrier-lab-tools.lock; \
    done; \
    awk -F '\t' '$1 == "package" { print $5 "  /input/packages/" $4 }' /input/carrier-lab-tools.lock >/tmp/package-sha256; \
    sha256sum -c /tmp/package-sha256; \
    mkdir -p /opt/toolroot; \
	while IFS="$(printf '\t')" read -r kind name version filename digest source license; do \
      test "$kind" = package || continue; \
      test "$(dpkg-deb --field "/input/packages/$filename" Package)" = "$name"; \
      test "$(dpkg-deb --field "/input/packages/$filename" Version)" = "$version"; \
      test "$(dpkg-deb --field "/input/packages/$filename" Architecture)" = amd64; \
      dpkg-deb --extract "/input/packages/$filename" /opt/toolroot; \
    done </input/carrier-lab-tools.lock; \
    awk -F '\t' '$1 == "tool" { print $5 "  /opt/toolroot" $4 }' /input/carrier-lab-tools.lock >/tmp/tool-sha256; \
    sha256sum -c /tmp/tool-sha256

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS tooling

LABEL org.opencontainers.image.title="Ardents Carrier Lab tooling" \
      org.opencontainers.image.description="Disposable tc/tcpdump laboratory roles; no product runtime or privacy claim" \
      org.opencontainers.image.base.name="ubuntu" \
      org.opencontainers.image.base.digest="sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960" \
      io.ardents.carrier-lab.target="tooling"

COPY --from=tools /opt/toolroot/ /
COPY --from=tools /input/carrier-lab-tools.lock /usr/share/ardents/carrier-lab-tools.lock
COPY --from=build --chown=65532:65532 /out/carrier-lab /usr/local/bin/carrier-lab
COPY --from=build --chown=65532:65532 /out/source.sha256 /usr/share/ardents/carrier-lab-source.sha256

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/carrier-lab"]

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS application

LABEL org.opencontainers.image.title="Ardents Carrier Lab" \
      org.opencontainers.image.description="Disposable Carrier Lab research binary; no privacy, anonymity, or production claim" \
      org.opencontainers.image.base.name="ubuntu" \
      org.opencontainers.image.base.digest="sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960" \
      io.ardents.carrier-lab.target="application"

COPY --from=build --chown=65532:65532 /out/carrier-lab /usr/local/bin/carrier-lab
COPY --from=build --chown=65532:65532 /out/source.sha256 /usr/share/ardents/carrier-lab-source.sha256

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/carrier-lab"]
