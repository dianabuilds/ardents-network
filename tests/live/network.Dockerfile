FROM golang:1.26.6-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents-route ./cmd/ardents-route \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-service ./cmd/ardents-service \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-stream-app ./cmd/ardents-stream-app \
    && CGO_ENABLED=0 go build -trimpath -o /out/ardents-publish-app ./cmd/ardents-publish-app \
    && CGO_ENABLED=0 go build -trimpath -o /out/carrier-lab ./cmd/carrier-lab

FROM scratch
COPY --from=build /out/ardents-route /usr/local/bin/ardents-route
COPY --from=build /out/ardents-service /usr/local/bin/ardents-service
COPY --from=build /out/ardents-stream-app /usr/local/bin/ardents-stream-app
COPY --from=build /out/ardents-publish-app /usr/local/bin/ardents-publish-app
COPY --from=build /out/carrier-lab /usr/local/bin/carrier-lab
