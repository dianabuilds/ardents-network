SHELL := /bin/sh

# Fast product checks are container-free. Native live Route scenarios are not
# selected until M8/M11 provide their peer-facing runtime; historical Carrier
# Lab remains separate.

QUALITY_CACHE_ROOT ?= $(if $(TEMP),$(TEMP),/tmp)/ardents-network-quality
export GOENV := off
export GOTOOLCHAIN := local
export GOFLAGS := -mod=readonly
export GOCACHE := $(QUALITY_CACHE_ROOT)/go-build
export GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod
export STATICCHECK_CACHE := $(QUALITY_CACHE_ROOT)/staticcheck
H4_3B_MULTIHOST_TIMEOUT := -timeout=8m

.PHONY: architecture build check e2e format format-check fuzz mod-check package-ubuntu-deb prepare-h4-2-net-01a qualification qualification-h4-1a qualification-h4-1b qualification-h4-2-local-emulator qualification-h4-2-multihost qualification-h4-3b-docker qualification-h4-3b-multihost qualification-h4-3b-vps qualification-h4-4a-firefox qualification-h4-4-signed-firefox qualification-h4-4-signed-xpi qualification-h4-4-ubuntu-enrollment qualification-h4-4-windows-enrollment quick-check staticcheck test test-race tools-check tools-install unit vet vuln

define newline


endef
UNIT_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/deterministic-packages.txt))
PROCESS_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/process-packages.txt))
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
	go test $(PROCESS_PACKAGES) -shuffle=on -count=1

qualification-h4-1a:
	sh ./tests/qualification/h4-1a-ubuntu-portable/run-ubuntu.sh -timeout=2m

qualification-h4-1b:
	sh ./tests/qualification/h4-1b-ubuntu-replacement/run-ubuntu.sh -timeout=2m

qualification-h4-2-multihost:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-2-multihost/run-windows.ps1

qualification-h4-2-local-emulator:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-2-local-emulator/run-windows.ps1

qualification-h4-3b-docker:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-3b-docker/run-windows.ps1

qualification-h4-3b-multihost:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-3b-multihost/run-windows.ps1 -GoTestTimeout "$(H4_3B_MULTIHOST_TIMEOUT)"

qualification-h4-3b-vps:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-3b-vps/run-windows.ps1

prepare-h4-2-net-01a:
	sh ./tests/qualification/h4-2-net-01a/run-ubuntu.sh

qualification-h4-4a-firefox:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4a-firefox/run-windows.ps1

qualification-h4-4-signed-xpi:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4-signed-xpi/run-windows.ps1 -SignedXPI "$(ARDENTS_H4_4_SIGNED_XPI)"

qualification-h4-4-signed-firefox:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4-signed-firefox/run-windows.ps1 -Firefox "$(ARDENTS_REFERENCE_C2_FIREFOX)" -SignedXPI "$(ARDENTS_H4_4_SIGNED_XPI)"

qualification-h4-4-windows-enrollment:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4-windows-enrollment/run-windows.ps1 -SignedXPI "$(ARDENTS_H4_4_SIGNED_XPI)"

qualification-h4-4-ubuntu-enrollment:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-4-ubuntu-enrollment/run-windows-docker.ps1 -SignedXPI "$(ARDENTS_H4_4_SIGNED_XPI)"

qualification: qualification-h4-1a qualification-h4-1b

package-ubuntu-deb:
	sh ./packaging/ubuntu-deb/build.sh

fuzz:
	go test ./internal/network/state -run '^$$' -fuzz '^FuzzCanonicalParsers$$' -fuzztime=1m

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

tools-install:
	go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
