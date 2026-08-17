FROM golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY tests ./tests
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents-route ./cmd/ardents-route \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-bridge ./cmd/ardents-bridge \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-service ./cmd/ardents-service \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-stream-app ./cmd/ardents-stream-app \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-publish-app ./cmd/ardents-publish-app \
    && CGO_ENABLED=0 go test -tags=live -c -o /out/network-live.test ./tests/live/network \
    && CGO_ENABLED=0 go test -c -o /out/camouflage.test ./internal/camouflage

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960
COPY --from=build /out/ /usr/local/bin/
RUN mkdir -p /run/ardents/client-route /run/ardents/publisher-route \
        /run/ardents/client-app /run/ardents/publisher-app /run/ardents/admin \
        /run/ardents/publication /run/ardents/introduction-ack /run/ardents/lifecycle \
        /run/evidence /run/secure /run/state \
    && chown -R 65532:65532 /run/ardents /run/evidence /run/secure /run/state
USER 65532:65532
