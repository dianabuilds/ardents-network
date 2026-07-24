FROM docker.io/library/golang:1.26.5-bookworm@sha256:3f6236bd765f898a2a3c2946112b04097814c4529d44534674700cd07b9c6b4c AS build

ARG GO_BUILD_PARALLELISM=2
ARG ARDENTS_VERSION=dev
ARG ARDENTS_COMMIT=unknown
ARG ARDENTS_BUILD_DATE=unknown

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN export LDFLAGS="-s -w -X ardents/internal/buildinfo.Version=${ARDENTS_VERSION} -X ardents/internal/buildinfo.Commit=${ARDENTS_COMMIT} -X ardents/internal/buildinfo.BuildDate=${ARDENTS_BUILD_DATE}" && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ardentsd ./cmd/ardentsd && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ardentsctl ./cmd/ardentsctl && \
    GOMAXPROCS=${GO_BUILD_PARALLELISM} go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags "$LDFLAGS" -o /out/ard-store-probe ./tests/tooling/store-probe

FROM docker.io/library/debian:bookworm-slim@sha256:63a496b5d3b99214b39f5ed70eb71a61e590a77979c79cbee4faf991f8c0783e

ARG ARDENTS_VERSION=dev
ARG ARDENTS_COMMIT=unknown
ARG ARDENTS_BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Ardents Network" \
      org.opencontainers.image.version="${ARDENTS_VERSION}" \
      org.opencontainers.image.revision="${ARDENTS_COMMIT}" \
      org.opencontainers.image.created="${ARDENTS_BUILD_DATE}" \
      org.opencontainers.image.licenses="MIT"

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /out/ardentsd /usr/local/bin/ardentsd
COPY --from=build /out/ardentsctl /usr/local/bin/ardentsctl
COPY --from=build /out/ard-store-probe /usr/local/bin/ard-store-probe
COPY LICENSE /usr/share/doc/ardents/LICENSE

EXPOSE 8080 9090 61001 61002 61003

ENTRYPOINT ["ardentsd"]
