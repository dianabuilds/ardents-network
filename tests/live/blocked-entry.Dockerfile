ARG ARDENTS_SOURCE_SHA256=0000000000000000000000000000000000000000000000000000000000000000
ARG ARDENTS_GO_BUILDER_IMAGE=golang:1.26.6-bookworm
FROM ${ARDENTS_GO_BUILDER_IMAGE} AS build
ARG ARDENTS_SOURCE_SHA256
ARG ARDENTS_GO_ARCHIVE_SHA256
ARG ARDENTS_GO_RECIPE_SHA256
ARG ARDENTS_GO_MODULE_SHA256

WORKDIR /src
ENV GOTOOLCHAIN=local \
    GOPROXY=off \
    GOSUMDB=off \
    GOWORK=off \
    GOFLAGS=-mod=readonly \
    CGO_ENABLED=0
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY tests ./tests
RUN sha256sum -c /go/pkg/mod/.ardents-stage5/source.sha256 \
    && go list -m all >/tmp/modules.txt \
    && cmp /tmp/modules.txt /go/pkg/mod/.ardents-stage5/modules.txt \
    && go mod verify \
    && rm /tmp/modules.txt \
    && go build -trimpath -o /out/ardents-route ./cmd/ardents-route \
    && go build -trimpath -o /out/ardents-bridge ./cmd/ardents-bridge \
    && go build -trimpath -o /out/ardents-service ./cmd/ardents-service \
    && go build -trimpath -o /out/ardents-stream-app ./cmd/ardents-stream-app \
    && go build -trimpath -o /out/ardents-publish-app ./cmd/ardents-publish-app \
    && go test -tags=live -c -o /out/network-live.test ./tests/live/network \
    && go test -c -o /out/camouflage.test ./internal/camouflage \
    && test "${#ARDENTS_SOURCE_SHA256}" -eq 64 \
    && test -z "$(printf '%s' "$ARDENTS_SOURCE_SHA256" | tr -d '0123456789abcdef')" \
    && mkdir -p /receipt \
    && printf '%s\n' "$ARDENTS_SOURCE_SHA256" >/receipt/stage5-source.sha256 \
    && test "$(cat /usr/share/ardents/go-archive.sha256)" = "$ARDENTS_GO_ARCHIVE_SHA256" \
    && test "$(cat /usr/share/ardents/go-builder-recipe.sha256)" = "$ARDENTS_GO_RECIPE_SHA256" \
    && test "$(cat /usr/share/ardents/go-module-cache.sha256)" = "$ARDENTS_GO_MODULE_SHA256" \
    && cd /out \
    && sha256sum ardents-route ardents-bridge ardents-service ardents-stream-app \
        ardents-publish-app network-live.test camouflage.test >/receipt/stage5-binaries.sha256

FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960
ARG ARDENTS_SOURCE_SHA256
ARG ARDENTS_GO_BUILDER_ID
ARG ARDENTS_GO_ARCHIVE_SHA256
ARG ARDENTS_GO_RECIPE_SHA256
ARG ARDENTS_GO_MODULE_SHA256
LABEL io.ardents.stage5.target="product" \
      io.ardents.stage5.source.sha256="${ARDENTS_SOURCE_SHA256}" \
      io.ardents.stage5.builder.image="${ARDENTS_GO_BUILDER_ID}" \
      io.ardents.stage5.go.archive.sha256="${ARDENTS_GO_ARCHIVE_SHA256}" \
      io.ardents.stage5.go.recipe.sha256="${ARDENTS_GO_RECIPE_SHA256}" \
      io.ardents.stage5.go.module-cache.sha256="${ARDENTS_GO_MODULE_SHA256}" \
      org.opencontainers.image.base.name="ubuntu" \
      org.opencontainers.image.base.digest="sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
COPY --from=build /out/ /usr/local/bin/
COPY --from=build /receipt/stage5-source.sha256 /usr/share/ardents/stage5-source.sha256
COPY --from=build /receipt/stage5-binaries.sha256 /usr/share/ardents/stage5-binaries.sha256
COPY --from=build /usr/share/ardents/go-archive.sha256 /usr/share/ardents/go-archive.sha256
COPY --from=build /usr/share/ardents/go-builder-recipe.sha256 /usr/share/ardents/go-builder-recipe.sha256
COPY --from=build /usr/share/ardents/go-module-cache.sha256 /usr/share/ardents/go-module-cache.sha256
RUN mkdir -p /run/ardents/client-route /run/ardents/publisher-route \
        /run/ardents/client-app /run/ardents/publisher-app /run/ardents/admin \
        /run/ardents/publication /run/ardents/introduction-ack /run/ardents/lifecycle \
        /run/evidence /run/secure /run/state \
    && chown -R 65532:65532 /run/ardents /run/evidence /run/secure /run/state
USER 65532:65532
