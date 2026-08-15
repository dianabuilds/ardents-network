FROM golang:1.26.6-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents-route ./cmd/ardents-route

FROM scratch
COPY --from=build /out/ardents-route /usr/local/bin/ardents-route
