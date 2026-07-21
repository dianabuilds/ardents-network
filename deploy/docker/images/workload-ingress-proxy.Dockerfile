FROM golang:1.26-bookworm AS build

ARG GO_BUILD_PARALLELISM=2

WORKDIR /src
COPY go.mod ./
COPY cmd/ardents-ingress-proxy ./cmd/ardents-ingress-proxy
COPY internal/ingressproxy ./internal/ingressproxy
RUN CGO_ENABLED=0 GOOS=linux GOMAXPROCS=${GO_BUILD_PARALLELISM} \
    go build -trimpath -buildvcs=false -p=${GO_BUILD_PARALLELISM} -ldflags="-s -w" \
    -o /out/ardents-ingress-proxy ./cmd/ardents-ingress-proxy

FROM scratch

ARG ARDENTS_VERSION=dev
ARG ARDENTS_COMMIT=unknown
ARG ARDENTS_BUILD_DATE=unknown
ARG ARDENTS_INGRESS_PROTOCOL

LABEL org.opencontainers.image.title="Ardents Workload Ingress Proxy" \
      org.opencontainers.image.version="${ARDENTS_VERSION}" \
      org.opencontainers.image.revision="${ARDENTS_COMMIT}" \
      org.opencontainers.image.created="${ARDENTS_BUILD_DATE}" \
      org.opencontainers.image.licenses="MIT" \
      io.ardents.ingress.protocol="${ARDENTS_INGRESS_PROTOCOL}"

USER 65534:65534
COPY --from=build /out/ardents-ingress-proxy /ardents-ingress-proxy
ENTRYPOINT ["/ardents-ingress-proxy"]
