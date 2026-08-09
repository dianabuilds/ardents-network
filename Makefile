SHELL := /bin/sh

# Development quality gates are intentionally container-free. Carrier Lab
# Docker qualification runs explicitly after a source freeze; see
# docs/development/carrier-lab-preflight.md.

QUALITY_CACHE_ROOT ?= $(if $(TEMP),$(TEMP),/tmp)/ardents-network-quality
export GOENV := off
export GOTOOLCHAIN := local
export GOFLAGS := -mod=readonly
export GOCACHE := $(QUALITY_CACHE_ROOT)/go-build
export GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod
export STATICCHECK_CACHE := $(QUALITY_CACHE_ROOT)/staticcheck

.PHONY: architecture build check format format-check mod-check quick-check staticcheck test test-race tools-check tools-install vet vuln

format:
	go fmt ./...
	gofmt -w ./scripts/check-tools.go

format-check architecture:
	go test ./internal/architecture -run TestRepositoryArchitecture -count=1

vet:
	go vet ./...

test:
	go test ./... -shuffle=on -count=1

test-race:
	go test ./... -race -shuffle=on -count=1

build:
	go build ./...

mod-check:
	go mod tidy -diff

tools-check:
	go run ./scripts/check-tools.go

staticcheck: tools-check
	staticcheck ./...

vuln: tools-check
	govulncheck ./...

quick-check: format-check vet test build mod-check

check: tools-check quick-check test-race staticcheck vuln

tools-install:
	go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
