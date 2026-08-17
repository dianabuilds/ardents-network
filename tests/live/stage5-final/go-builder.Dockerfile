FROM ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960

ARG GO_ARCHIVE=go1.26.6.linux-amd64.tar.gz
ARG GO_ARCHIVE_SHA256
ARG BUILDER_RECIPE_SHA256
ARG MODULE_CACHE=gomodcache.tar.gz
ARG MODULE_CACHE_SHA256

COPY ${GO_ARCHIVE} /tmp/go.tar.gz
COPY ${MODULE_CACHE} /tmp/gomodcache.tar.gz
RUN test "${#GO_ARCHIVE_SHA256}" -eq 64 \
    && test "${#BUILDER_RECIPE_SHA256}" -eq 64 \
    && test "${#MODULE_CACHE_SHA256}" -eq 64 \
    && printf '%s  %s\n' "$GO_ARCHIVE_SHA256" /tmp/go.tar.gz | sha256sum -c - \
    && printf '%s  %s\n' "$MODULE_CACHE_SHA256" /tmp/gomodcache.tar.gz | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && mkdir -p /go/pkg/mod \
    && tar -C /go/pkg/mod -xzf /tmp/gomodcache.tar.gz \
    && rm /tmp/go.tar.gz /tmp/gomodcache.tar.gz \
    && mkdir -p /usr/share/ardents \
    && printf '%s\n' "$GO_ARCHIVE_SHA256" >/usr/share/ardents/go-archive.sha256 \
    && printf '%s\n' "$BUILDER_RECIPE_SHA256" >/usr/share/ardents/go-builder-recipe.sha256 \
    && printf '%s\n' "$MODULE_CACHE_SHA256" >/usr/share/ardents/go-module-cache.sha256 \
    && test "$(/usr/local/go/bin/go version)" = "go version go1.26.6 linux/amd64"

ENV PATH=/usr/local/go/bin:/usr/bin:/bin \
    GOPATH=/go \
    GOTOOLCHAIN=local \
    GOPROXY=off \
    GOSUMDB=off \
    GOWORK=off \
    GOFLAGS=-mod=readonly \
    CGO_ENABLED=0

LABEL io.ardents.stage5.target="go-builder" \
      io.ardents.stage5.go.archive.sha256="${GO_ARCHIVE_SHA256}" \
      io.ardents.stage5.go.recipe.sha256="${BUILDER_RECIPE_SHA256}" \
      io.ardents.stage5.go.module-cache.sha256="${MODULE_CACHE_SHA256}" \
      org.opencontainers.image.base.digest="sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
