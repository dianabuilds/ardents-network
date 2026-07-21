FROM golang:1.26-bookworm AS build

ARG GO_BUILD_PARALLELISM=2
ARG ARDENTS_VERSION=dev
ARG ARDENTS_COMMIT=unknown
ARG ARDENTS_BUILD_DATE=unknown

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export LDFLAGS="-s -w -X ardents/internal/buildinfo.Version=${ARDENTS_VERSION} -X ardents/internal/buildinfo.Commit=${ARDENTS_COMMIT} -X ardents/internal/buildinfo.BuildDate=${ARDENTS_BUILD_DATE}" && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ardd ./cmd/ardd && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ard ./cmd/ard && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ard-store-probe ./tests/tooling/store-probe

FROM debian:bookworm-slim

ARG ARDENTS_VERSION=dev
ARG ARDENTS_COMMIT=unknown
ARG ARDENTS_BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Ardents Network" \
      org.opencontainers.image.version="${ARDENTS_VERSION}" \
      org.opencontainers.image.revision="${ARDENTS_COMMIT}" \
      org.opencontainers.image.created="${ARDENTS_BUILD_DATE}" \
      org.opencontainers.image.licenses="MIT"

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /out/ardd /usr/local/bin/ardd
COPY --from=build /out/ard /usr/local/bin/ard
COPY --from=build /out/ard-store-probe /usr/local/bin/ard-store-probe
COPY LICENSE /usr/share/doc/ardents/LICENSE

EXPOSE 8080 9090 61001 61002 61003

ENTRYPOINT ["ardd"]
