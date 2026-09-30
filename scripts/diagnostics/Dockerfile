# syntax=docker/dockerfile:1
FROM golang:1.26.8-bookworm@sha256:9fdc884aacc3bec89b20ffc69f4bb369c78210e3e4f600387b5128b12c199f81
ENV GOENV=off GOTOOLCHAIN=local GOTELEMETRY=off GOFLAGS=-mod=readonly
RUN apt-get update && apt-get install -y --no-install-recommends \
    make git ca-certificates gcc libc6-dev pkg-config dpkg util-linux \
    strace iproute2 tcpdump procps graphviz sudo python3 libicu72 libssl3 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY Makefile go.mod go.sum ./
COPY tests/profiles/ ./tests/profiles/
COPY scripts/diagnostics/install-powershell.sh ./scripts/diagnostics/install-powershell.sh
RUN make tools-install DIAGNOSTIC_TOOLS=1
RUN go mod download && go mod verify
COPY scripts/diagnostics/diagnostic-command.go scripts/diagnostics/diagnostic-capture.go scripts/diagnostics/diagnostic-view.go scripts/diagnostics/diagnostic-report.go scripts/diagnostics/diagnostic-monitor.go scripts/diagnostics/diagnostic-monitor-view.go scripts/diagnostics/diagnostic-log-retention.go scripts/diagnostics/diagnostic-evidence.go ./
RUN go build -trimpath -buildvcs=false -o /usr/local/bin/ardents-diagnostics diagnostic-command.go diagnostic-capture.go diagnostic-view.go diagnostic-report.go diagnostic-monitor.go diagnostic-monitor-view.go diagnostic-log-retention.go diagnostic-evidence.go \
    && mkdir -p /opt/ardents-diagnostics \
    && (go version; staticcheck -version; govulncheck -version; dlv version; pwsh --version; dpkg-query -W; go version -m /go/bin/dlv; go version -m /go/bin/errcheck) > /opt/ardents-diagnostics/inventory.txt
RUN groupadd -g 10001 diagnostic && useradd -m -u 10001 -g diagnostic diagnostic \
    && mkdir -p /cache /state && chown diagnostic:diagnostic /cache /state \
    && printf "diagnostic ALL=(root) NOPASSWD: ALL\n" > /etc/sudoers.d/diagnostic \
    && chmod 440 /etc/sudoers.d/diagnostic
USER diagnostic
WORKDIR /source
ENTRYPOINT ["ardents-diagnostics"]
CMD ["doctor"]
