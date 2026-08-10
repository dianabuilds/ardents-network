FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS build

ARG GATEC_SOURCE_SHA256

RUN --network=none --mount=type=bind,from=go_archive,source=go1.26.5.linux-amd64.tar.gz,target=/input/go1.26.5.linux-amd64.tar.gz,ro \
    printf '%s  %s\n' \
      '5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053' \
      '/input/go1.26.5.linux-amd64.tar.gz' | sha256sum -c - \
    && mkdir -p /opt/toolchain \
    && tar -C /opt/toolchain -xzf /input/go1.26.5.linux-amd64.tar.gz

WORKDIR /workspace
COPY go.mod go.sum ./
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
    GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod GOTMPDIR=/tmp/go-tmp \
    /opt/toolchain/go/bin/go build -trimpath -buildvcs=false -ldflags=-buildid= \
      -o /out/reference-site-lab ./cmd/reference-site-lab \
    && test "${#GATEC_SOURCE_SHA256}" -eq 64 \
    && test -z "$(printf '%s' "$GATEC_SOURCE_SHA256" | tr -d '0123456789abcdef')" \
    && printf '%s\n' "$GATEC_SOURCE_SHA256" >/out/source.sha256

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 AS reference

LABEL org.opencontainers.image.title="Ardents Gate C Reference Site Lab" \
      org.opencontainers.image.description="Disposable Named Unlisted Site experiment roles; no product or privacy claim" \
      io.ardents.gate-c.target="reference"

COPY --from=build --chown=65532:65532 /out/reference-site-lab /usr/local/bin/reference-site-lab
COPY --from=build --chown=65532:65532 /out/source.sha256 /usr/share/ardents/gate-c-source.sha256

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/reference-site-lab"]
