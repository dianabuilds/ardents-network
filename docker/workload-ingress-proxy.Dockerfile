FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ardents-ingress-proxy ./cmd/ardents-ingress-proxy
COPY internal/workload/ingressproxy ./internal/workload/ingressproxy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ardents-ingress-proxy ./cmd/ardents-ingress-proxy

FROM scratch
USER 65534:65534
COPY --from=build /out/ardents-ingress-proxy /ardents-ingress-proxy
ENTRYPOINT ["/ardents-ingress-proxy"]
