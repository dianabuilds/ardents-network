SHELL := /bin/sh

# Fast product checks are container-free. Selected native Rendezvous scenarios
# use explicit qualification targets and prerequisites; ordinary checks never
# infer a live environment.

QUALITY_CACHE_ROOT ?= $(if $(TEMP),$(TEMP),/tmp)/ardents-network-quality
export GOENV := off
export GOTOOLCHAIN := local
export GOFLAGS := -mod=readonly
export GOCACHE := $(QUALITY_CACHE_ROOT)/go-build
export GOMODCACHE := $(QUALITY_CACHE_ROOT)/go-mod
export STATICCHECK_CACHE := $(QUALITY_CACHE_ROOT)/staticcheck

ifeq ($(OS),Windows_NT)
RACE_TEST_PREFIX :=
else
RACE_TEST_PREFIX := umask 077;
endif

.PHONY: architecture artifact-representation-check browser-build browser-check build check e2e format format-check fuzz headless-build headless-check headless-evidence mod-check package-ubuntu-deb prepare-native-rendezvous-host qualification qualification-alpha-control-two-endpoints qualification-browser-entry-ubuntu qualification-browser-entry-windows qualification-browser-signed-xpi qualification-endpoint-portable-ubuntu qualification-endpoint-replacement-ubuntu qualification-native-rendezvous-multihost quick-check staticcheck test test-race tools-check tools-install unit vet vuln

define newline


endef
UNIT_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/deterministic-packages.txt))
PROCESS_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/process-packages.txt))
HEADLESS_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/headless-commands.txt))
BROWSER_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/browser-commands.txt))
HEADLESS_GOOS := $(shell go env GOOS)
HEADLESS_GOARCH := $(shell go env GOARCH)
HEADLESS_PLATFORM := $(HEADLESS_GOOS)-$(HEADLESS_GOARCH)
HEADLESS_SUFFIX := $(if $(filter windows,$(HEADLESS_GOOS)),.exe,)
HEADLESS_ARTIFACT_ROOT ?= $(QUALITY_CACHE_ROOT)/headless-artifacts/$(HEADLESS_PLATFORM)
HEADLESS_ENDPOINT_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_NODE_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-node-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CONTROL_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-control-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CUSTODY_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-custody-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
BROWSER_ARTIFACT_ROOT ?= $(QUALITY_CACHE_ROOT)/browser-artifacts/$(HEADLESS_PLATFORM)
BROWSER_ADAPTER_ARTIFACT := $(BROWSER_ARTIFACT_ROOT)/ardents-browser-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
BROWSER_ENTRY_ARTIFACT := $(BROWSER_ARTIFACT_ROOT)/ardents-browser-entry-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
override CANONICAL_GO_BUILD_FLAGS := -trimpath -buildvcs=false
QUICK_CHECK_TARGETS := format-check vet unit build mod-check browser-check artifact-representation-check

ifeq ($(OS),Windows_NT)
HEADLESS_ARTIFACT_SHELL ?= C:/Program Files/Git/bin/bash.exe
HEADLESS_ARTIFACT_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(HEADLESS_ARTIFACT_ROOT)') | Out-Null"
BROWSER_ARTIFACT_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(BROWSER_ARTIFACT_ROOT)') | Out-Null"
else
HEADLESS_ARTIFACT_SHELL ?= sh
HEADLESS_ARTIFACT_MKDIR = mkdir -p "$(HEADLESS_ARTIFACT_ROOT)"
BROWSER_ARTIFACT_MKDIR = mkdir -p "$(BROWSER_ARTIFACT_ROOT)"
endif

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

browser-build:
	$(BROWSER_ARTIFACT_MKDIR)
	$(foreach command,$(BROWSER_COMMANDS),go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(BROWSER_ARTIFACT_ROOT)/$(notdir $(command))-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)" $(command)$(newline))

browser-check: export ARDENTS_EXTRACTION_OWNER := application-browser
browser-check: export ARDENTS_EXTRACTION_SHELL := $(HEADLESS_ARTIFACT_SHELL)
browser-check: browser-build
	"$(HEADLESS_ARTIFACT_SHELL)" ./packaging/browser-bundle/test.sh "$(HEADLESS_PLATFORM)" "$(abspath $(BROWSER_ADAPTER_ARTIFACT))" "$(abspath $(BROWSER_ENTRY_ARTIFACT))"
	go test ./internal/architecture -run '^TestApplicationExtractionRehearsal$$' -count=1
	go test ./cmd/ardents-browser-entry -run '^TestParticipantInstallAuthenticatesARealV4Bundle$$' -count=1

artifact-representation-check: export ARDENTS_CANONICAL_BUILD_REPRESENTATIONS := 1
artifact-representation-check:
	go test ./internal/architecture -run '^TestCanonicalCommandBuildIsRepositoryRepresentationIndependent$$' -count=1

headless-build:
	$(HEADLESS_ARTIFACT_MKDIR)
	$(foreach command,$(HEADLESS_COMMANDS),go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(HEADLESS_ARTIFACT_ROOT)/$(notdir $(command))-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)" $(command)$(newline))

headless-evidence: export ARDENTS_E2E_COMMAND := $(abspath $(HEADLESS_ENDPOINT_ARTIFACT))
headless-evidence: export ARDENTS_E2E_PRODUCT_ARDENTS := $(abspath $(HEADLESS_ENDPOINT_ARTIFACT))
headless-evidence: export ARDENTS_E2E_PRODUCT_ARDENTS_NODE := $(abspath $(HEADLESS_NODE_ARTIFACT))
headless-evidence: export ARDENTS_E2E_PRODUCT_ARDENTS_CUSTODY := $(abspath $(HEADLESS_CUSTODY_ARTIFACT))
headless-evidence: export ARDENTS_E2E_CONTROL := $(abspath $(HEADLESS_CONTROL_ARTIFACT))
headless-evidence: headless-build
	"$(HEADLESS_ARTIFACT_SHELL)" ./packaging/alpha-bundle/test.sh "$(HEADLESS_PLATFORM)" "$(abspath $(HEADLESS_ENDPOINT_ARTIFACT))" "$(abspath $(HEADLESS_NODE_ARTIFACT))" "$(abspath $(HEADLESS_CONTROL_ARTIFACT))" "$(abspath $(HEADLESS_CUSTODY_ARTIFACT))"
	go test ./internal/enrollment -run '^(TestVerifyReturnsV3HeadlessArtifactsOutsideReleaseMetadata|TestVerifyRejectsUnknownInventoryAndExecutableSubstitution)$$' -count=1
	go test ./tests/e2e/endpoint -run '^(TestEnrollmentCheckAcceptsExactRunningBundleAndRejectsChangedManifest|TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart)$$' -count=1
	go test ./tests/e2e/network-source -run '^TestFiniteSourceCommandsAsBlackBoxProcesses$$' -count=1
	go test ./tests/e2e/node -run '^TestNativeDutyProcessesUseTheirExactStateAssignments$$' -count=1
	go test ./tests/e2e/service -run '^TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses$$' -count=1

headless-check: export ARDENTS_EXTRACTION_OWNER := network
headless-check: headless-evidence
	go test ./internal/architecture -run '^TestNetworkExtractionRehearsal$$' -count=1

qualification-endpoint-portable-ubuntu:
	sh ./tests/qualification/endpoint-portable-ubuntu/run-ubuntu.sh -timeout=2m

qualification-endpoint-replacement-ubuntu:
	sh ./tests/qualification/endpoint-replacement-ubuntu/run-ubuntu.sh -timeout=2m

qualification-native-rendezvous-multihost:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/native-rendezvous-multihost/run-windows.ps1

qualification-alpha-control-two-endpoints:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/alpha-control-two-endpoints/run-windows.ps1 -CandidateArchive "$(ALPHA_CONTROL_ARCHIVE)" -ArchiveSHA256 "$(ALPHA_CONTROL_ARCHIVE_SHA256)" -ManifestPin "$(ALPHA_CONTROL_MANIFEST_PIN)" -EndpointSHA256 "$(ALPHA_CONTROL_ENDPOINT_SHA256)" -ControlSHA256 "$(ALPHA_CONTROL_CONTROL_SHA256)" -Cohort "$(ALPHA_CONTROL_COHORT)" -Release "$(ALPHA_CONTROL_RELEASE)" -At "$(ALPHA_CONTROL_AT)" -VPS "$(ALPHA_CONTROL_VPS)" -SSHKey "$(ALPHA_CONTROL_SSH_KEY)" -User "$(ALPHA_CONTROL_VPS_USER)" -EvidenceOutput "$(ALPHA_CONTROL_EVIDENCE)"

prepare-native-rendezvous-host:
	sh ./tests/qualification/native-rendezvous-host/run-ubuntu.sh

qualification-browser-signed-xpi:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/browser-signed-xpi/run-windows.ps1 -SignedXPI "$(ARDENTS_BROWSER_SIGNED_XPI)"

qualification-browser-entry-windows:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/browser-entry-windows/run-windows.ps1 -SignedXPI "$(ARDENTS_BROWSER_SIGNED_XPI)"

qualification-browser-entry-ubuntu:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/browser-entry-ubuntu/run-windows-docker.ps1 -SignedXPI "$(ARDENTS_BROWSER_SIGNED_XPI)"

qualification: qualification-endpoint-portable-ubuntu qualification-endpoint-replacement-ubuntu

package-ubuntu-deb:
	sh ./packaging/ubuntu-deb/build.sh

fuzz:
	go test ./internal/network/state -run '^$$' -fuzz '^FuzzCanonicalParsers$$' -fuzztime=1m

test: unit e2e

test-race:
	$(RACE_TEST_PREFIX) go test $(UNIT_PACKAGES) -short -race -shuffle=on -count=1

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
