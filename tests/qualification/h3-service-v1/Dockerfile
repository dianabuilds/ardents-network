FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/ardents-route ./cmd/ardents-route && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-service ./cmd/ardents-service && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-stream-app ./cmd/ardents-stream-app && \
    CGO_ENABLED=0 go build -trimpath -o /out/ardents-publish-app ./cmd/ardents-publish-app && \
	CGO_ENABLED=0 go build -trimpath -o /out/ardents-service-negative ./cmd/ardents-service-negative && \
	CGO_ENABLED=0 go build -trimpath -o /out/ardents-service-volume-init ./cmd/ardents-service-volume-init && \
	CGO_ENABLED=0 go build -trimpath -o /out/ardents-service-qualify ./cmd/ardents-service-qualify && \
	CGO_ENABLED=0 go build -trimpath -o /out/ardents-recovery-qualify ./cmd/ardents-recovery-qualify

FROM scratch
ARG ARDENTS_SOURCE_COMMIT
LABEL org.opencontainers.image.revision=$ARDENTS_SOURCE_COMMIT
COPY --from=build /out/ /usr/local/bin/
