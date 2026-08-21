SHELL := /bin/sh

# Fast product checks are container-free. Long live container scenarios remain
# an explicit independent test surface; historical Carrier Lab is separate.

QUALITY_CACHE_ROOT ?= $(if $(TEMP),$(TEMP),/tmp)/ardents-network-quality
export GOENV := off
export GOTOOLCHAIN := local
export GOFLAGS := -mod=readonly
export GOCACHE := $(QUALITY_CACHE_ROOT)/go-build
export GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod
export STATICCHECK_CACHE := $(QUALITY_CACHE_ROOT)/staticcheck

.PHONY: architecture build check e2e format format-check lab-test live mod-check prototype-r053 quick-check staticcheck test test-race tools-check tools-install unit vet vuln

ALL_PACKAGES := $(shell go list ./cmd/... ./internal/...)
LAB_PACKAGES := $(shell go list ./cmd/carrier-lab ./cmd/named-site-lab ./internal/lab/...)
UNIT_PACKAGES := $(filter-out $(LAB_PACKAGES),$(ALL_PACKAGES))
QUICK_CHECK_TARGETS := format-check vet unit build mod-check

format:
	go fmt ./...
	gofmt -w ./scripts/check-tools.go

format-check architecture:
	go test ./internal/architecture -run TestRepositoryArchitecture -count=1

vet:
	go vet ./...

unit:
	go test $(UNIT_PACKAGES) -short -shuffle=on -count=1

e2e:
	go test ./tests/e2e/... -shuffle=on -count=1

lab-test:
	go test $(LAB_PACKAGES) -short -shuffle=on -count=1

live:
	go test -tags=live ./tests/live/... -count=1 -timeout=80m

test: unit e2e

test-race:
	go test $(UNIT_PACKAGES) -short -race -shuffle=on -count=1

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

quick-check:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS)

check:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS) staticcheck vuln
	$(MAKE) --output-sync=target e2e
	$(MAKE) --output-sync=target test-race

prototype-r053:
	go run ./experiments/r-053-stage-7-authority-recovery/profile.go ./experiments/r-053-stage-7-authority-recovery/main.go

tools-install:
	go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
