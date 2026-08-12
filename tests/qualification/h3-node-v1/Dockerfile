FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents ./cmd/ardents && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-node ./cmd/ardents-node && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-qualify ./cmd/ardents-qualify

FROM scratch
COPY --from=build /out/ardents /usr/local/bin/ardents
COPY --from=build /out/ardents-node /usr/local/bin/ardents-node
COPY --from=build /out/ardents-qualify /usr/local/bin/ardents-qualify
