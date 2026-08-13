FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents-route ./cmd/ardents-route && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-route-qualify ./cmd/ardents-route-qualify

FROM scratch
ARG ARDENTS_SOURCE_DIGEST
LABEL org.opencontainers.image.revision=$ARDENTS_SOURCE_DIGEST
COPY --from=build /out/ardents-route /usr/local/bin/ardents-route
COPY --from=build /out/ardents-route-qualify /usr/local/bin/ardents-route-qualify
