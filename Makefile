SHELL := /bin/sh

# Fast product checks are container-free. Selected qualification scenarios use
# explicit targets and prerequisites; ordinary checks never infer a live
# environment.

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

.PHONY: architecture artifact-representation-check build check deadcode e2e fixture-network-test format format-check fuzz headless-build headless-check headless-evidence heapdump-capture heapdump-role-map installed-tag-compile-check issue60-checks mod-check package-e2e package-ubuntu-deb qualification qualification-endpoint-portable-ubuntu qualification-endpoint-replacement-ubuntu qualification-service-credential-response-linux quick-check staticcheck test test-race text-role-durable-state-capture tools-check tools-install unit vet vuln

define newline


endef
UNIT_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/deterministic-packages.txt))
PROCESS_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/process-packages.txt))
NODE_PROCESS_PACKAGES := $(filter %/tests/e2e/node,$(PROCESS_PACKAGES))
# Only the Node process suite may select an elevated system-scope runner.
NODE_PROCESS_TEST_COMMAND ?= go
HEADLESS_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/headless-commands.txt))
HEADLESS_GOOS := $(shell go env GOOS)
HEADLESS_GOARCH := $(shell go env GOARCH)
HEADLESS_PLATFORM := $(HEADLESS_GOOS)-$(HEADLESS_GOARCH)
ifeq ($(HEADLESS_GOOS),linux)
UNIT_PACKAGES += $(subst $(newline), ,$(file <tests/profiles/deterministic-linux-packages.txt))
endif
HEADLESS_SUFFIX := $(if $(filter windows,$(HEADLESS_GOOS)),.exe,)
HEADLESS_ARTIFACT_ROOT ?= $(QUALITY_CACHE_ROOT)/headless-artifacts/$(HEADLESS_PLATFORM)
HEADLESS_ENDPOINT_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_NODE_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-node-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CONTROL_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-control-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CUSTODY_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-custody-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
# The text_worker_installed surface only runs on dedicated Ubuntu hosts, so no
# runtime gate compiles it. This target cross-compiles the tagged Linux test
# packages from any host; the test binaries land in the quality cache, never in
# the repository.
INSTALLED_TAG_COMPILE_ROOT := $(QUALITY_CACHE_ROOT)/installed-tag-compile
INSTALLED_TAG_COMPILE_PACKAGES := ./internal/endpoint ./tests/e2e/node

ifeq ($(OS),Windows_NT)
INSTALLED_TAG_COMPILE_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(INSTALLED_TAG_COMPILE_ROOT)') | Out-Null"
else
INSTALLED_TAG_COMPILE_MKDIR = mkdir -p "$(INSTALLED_TAG_COMPILE_ROOT)"
endif

override CANONICAL_GO_BUILD_FLAGS := -trimpath -buildvcs=false
QUICK_CHECK_TARGETS := vet unit build mod-check artifact-representation-check installed-tag-compile-check release-operation-compile-check

ifeq ($(OS),Windows_NT)
HEADLESS_ARTIFACT_SHELL ?= C:/Program Files/Git/bin/bash.exe
HEADLESS_ARTIFACT_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(HEADLESS_ARTIFACT_ROOT)') | Out-Null"
else
HEADLESS_ARTIFACT_SHELL ?= sh
HEADLESS_ARTIFACT_MKDIR = mkdir -p "$(HEADLESS_ARTIFACT_ROOT)"
endif

format:
	go fmt ./...
	gofmt -w ./scripts/check-tools.go ./scripts/check-deadcode.go ./scripts/run-fuzz-targets.go

format-check architecture:
	go test ./internal/architecture -run TestRepositoryArchitecture -count=1

vet:
	go vet ./...

unit:
	go test -p 1 $(UNIT_PACKAGES) -short -shuffle=on -count=1 -timeout=15m

heapdump-capture:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_REPORT)" || (echo "ARDENTS_HEAPDUMP_REPORT is required"; exit 2)
	go test -tags heapdumpcapture ./internal/endpoint -run '^TestHeapDumpObservation$$' -count=1

heapdump-role-map:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_ROLE_MAP)" || (echo "ARDENTS_HEAPDUMP_ROLE_MAP is required"; exit 2)
	go test -tags heapdumpcapture ./internal/endpoint -run '^TestHeapDumpRoleMapObservation$$' -count=1 -timeout=5m

e2e:
	go test -p 1 $(filter-out $(NODE_PROCESS_PACKAGES),$(PROCESS_PACKAGES)) -shuffle=on -count=1
	$(NODE_PROCESS_TEST_COMMAND) test -p 1 $(NODE_PROCESS_PACKAGES) -shuffle=on -count=1

fixture-network-test:
	@test "$(HEADLESS_GOOS)" = linux || (echo "fixture-network-test requires Linux"; exit 2)
	go test ./tests/qualification/stream-network-two-host/fixturecommand/qualification-network -count=1

package-e2e:
	sudo env "PATH=$$PATH" "GOTOOLCHAIN=$(GOTOOLCHAIN)" "GOENV=$(GOENV)" "GOFLAGS=$(GOFLAGS)" "GOCACHE=$(GOCACHE)" "GOMODCACHE=$(GOMODCACHE)" go test -tags packagee2e ./tests/e2e/endpoint -run '^TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes$$' -shuffle=on -count=1

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
	@test "$(HEADLESS_GOOS)" = linux && test "$(HEADLESS_GOARCH)" = amd64 || (echo "headless-evidence requires Linux x86-64"; exit 2)
	"$(HEADLESS_ARTIFACT_SHELL)" ./packaging/alpha-bundle/test.sh "$(HEADLESS_PLATFORM)" "$(abspath $(HEADLESS_ENDPOINT_ARTIFACT))" "$(abspath $(HEADLESS_NODE_ARTIFACT))" "$(abspath $(HEADLESS_CONTROL_ARTIFACT))" "$(abspath $(HEADLESS_CUSTODY_ARTIFACT))"
	go test ./internal/enrollment -run '^(TestVerifyReturnsV3HeadlessArtifactsOutsideReleaseMetadata|TestVerifyRejectsUnknownInventoryAndExecutableSubstitution)$$' -count=1
	go test ./tests/e2e/endpoint -run '^(TestEnrollmentCheckAcceptsExactRunningBundleAndRejectsChangedManifest|TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart)$$' -count=1
	go test ./tests/e2e/network-source -run '^TestFiniteSourceCommandsAsBlackBoxProcesses$$' -count=1
	go test ./tests/e2e/node -run '^TestClosedTextTopologyProvisioningAcrossProcesses$$' -count=1
	go test ./tests/e2e/service -run '^TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses$$' -count=1

headless-check: export ARDENTS_EXTRACTION_OWNER := network
headless-check: headless-evidence
	go test ./internal/architecture -run '^TestNetworkExtractionRehearsal$$' -count=1

qualification-endpoint-portable-ubuntu:
	sh ./tests/qualification/endpoint-portable-ubuntu/run-ubuntu.sh -timeout=2m

qualification-endpoint-replacement-ubuntu:
	sh ./tests/qualification/endpoint-replacement-ubuntu/run-ubuntu.sh -timeout=2m

qualification-service-credential-response-linux:
	sh ./tests/qualification/service-credential-response-linux/run-ubuntu.sh -timeout=2m

qualification: qualification-endpoint-portable-ubuntu qualification-endpoint-replacement-ubuntu

package-ubuntu-deb:
	sh ./packaging/ubuntu-deb/build.sh

fuzz:
	go run ./scripts/run-fuzz-targets.go

test: unit e2e

test-race:
	$(RACE_TEST_PREFIX) go test -p 1 $(UNIT_PACKAGES) -short -race -shuffle=on -count=1 -timeout=15m

build:
	go build ./...

mod-check:
	go mod tidy -diff

tools-check:
	go run ./scripts/check-tools.go

staticcheck: tools-check standalone-staticcheck
	staticcheck ./...

.PHONY: standalone-staticcheck standalone-staticcheck-linux
standalone-staticcheck: tools-check
standalone-staticcheck-linux: export GOOS := linux
standalone-staticcheck-linux: export GOARCH := amd64
standalone-staticcheck-linux: export CGO_ENABLED := 0
standalone-staticcheck:
	$(MAKE) --output-sync=target standalone-staticcheck-linux

standalone-staticcheck-linux:
# Ignored entrypoints are separate programs; select-pr owns two files together.
	staticcheck ./scripts/check-deadcode.go
	staticcheck ./scripts/check-tools.go
	staticcheck ./scripts/enrollment-artifact-name.go
	staticcheck ./scripts/prepare-qualification-alpha-catalog.go
	staticcheck ./scripts/prepare-qualification-alpha-evidence.go
	staticcheck ./scripts/prepare-qualification-alpha-keys.go
	staticcheck ./scripts/prepare-qualification-network-keys.go
	staticcheck ./scripts/prepare-qualification-release-keys.go
	staticcheck ./scripts/protected-generation-check.go
	staticcheck ./scripts/run-fuzz-targets.go
	staticcheck ./scripts/run-issue60-checks.go
	staticcheck ./scripts/sign-qualification-alpha.go
	staticcheck ./scripts/sign-qualification-network.go
	staticcheck ./scripts/sign-qualification-release.go
	staticcheck ./scripts/select-pr-checks.go ./scripts/select-pr-check-registry.go

vuln: tools-check
	govulncheck ./...

deadcode: tools-check
	go run ./scripts/check-deadcode.go

installed-tag-compile-check: export GOOS := linux
installed-tag-compile-check: export GOARCH := amd64
installed-tag-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	$(foreach package,$(INSTALLED_TAG_COMPILE_PACKAGES),go test -c -tags text_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/$(subst /,_,$(package)).test" $(package)$(newline))

quick-check:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS)

.PHONY: release-operation-compile-check
release-operation-compile-check: export GOOS := linux
release-operation-compile-check: export GOARCH := amd64
release-operation-compile-check: export CGO_ENABLED := 0
release-operation-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go vet ./scripts/prepare-qualification-release-keys.go
	go vet ./scripts/sign-qualification-release.go
	go vet ./scripts/prepare-qualification-alpha-evidence.go
	go vet ./scripts/prepare-qualification-alpha-catalog.go
	go vet ./scripts/prepare-qualification-alpha-keys.go
	go vet ./scripts/prepare-qualification-network-keys.go
	go vet ./scripts/sign-qualification-network.go
	go vet ./scripts/sign-qualification-alpha.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-release-keys" ./scripts/prepare-qualification-release-keys.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/sign-qualification-release" ./scripts/sign-qualification-release.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-alpha-evidence" ./scripts/prepare-qualification-alpha-evidence.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-alpha-catalog" ./scripts/prepare-qualification-alpha-catalog.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-alpha-keys" ./scripts/prepare-qualification-alpha-keys.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-network-keys" ./scripts/prepare-qualification-network-keys.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/sign-qualification-network" ./scripts/sign-qualification-network.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/sign-qualification-alpha" ./scripts/sign-qualification-alpha.go

issue60-checks:
	@test -n "$(ARDENTS_ISSUE60_REPORT)" || (echo "ARDENTS_ISSUE60_REPORT is required"; exit 2)
	go run ./scripts/run-issue60-checks.go -report "$(ARDENTS_ISSUE60_REPORT)"

check:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS) staticcheck vuln deadcode
	$(MAKE) --output-sync=target e2e
ifeq ($(HEADLESS_GOOS),linux)
	$(MAKE) --output-sync=target fixture-network-test
	$(MAKE) --output-sync=target package-e2e
endif
	$(MAKE) --output-sync=target test-race

DIAGNOSTIC_PARSER_ROOT ?= $(if $(GOBIN),$(GOBIN),$(shell go env GOPATH)/bin)

tools-install:
	go build -trimpath -buildvcs=false -o "$(DIAGNOSTIC_PARSER_ROOT)/pprof$(HEADLESS_SUFFIX)" cmd/pprof
	go build -trimpath -buildvcs=false -o "$(DIAGNOSTIC_PARSER_ROOT)/trace$(HEADLESS_SUFFIX)" cmd/trace
ifneq ($(DIAGNOSTIC_PARSERS_ONLY),1)
	go install honnef.co/go/tools/cmd/staticcheck@2026.2.1
	go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
	go install golang.org/x/tools/cmd/deadcode@v0.50.0
ifeq ($(DIAGNOSTIC_TOOLS),1)
	go install github.com/go-delve/delve/cmd/dlv@v1.27.2
	go install github.com/kisielk/errcheck@v1.20.0
	sh ./scripts/diagnostics/install-powershell.sh
endif
endif

.PHONY: text-worker-policy-check
text-worker-policy-check:
	sh ./tests/qualification/text-worker-policy/run-ubuntu.sh

.PHONY: text-worker-lifecycle-check
text-worker-lifecycle-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh

.PHONY: text-worker-tree-check
text-worker-tree-check:
	sh ./tests/qualification/text-worker-tree/run-ubuntu.sh

.PHONY: text-worker-network-check text-worker-recovery-check text-command-network-build text-command-network-check text-worker-escape-check
text-worker-network-check:
	sh ./tests/qualification/text-worker-network/run-ubuntu.sh

text-worker-recovery-check:
	sh ./tests/qualification/text-worker-recovery/run-ubuntu.sh

text-command-network-build:
	sh ./tests/qualification/text-command-network/build-candidate.sh

text-command-network-check:
	sh ./tests/qualification/text-command-network/run-ubuntu.sh

text-worker-escape-check:
	sh ./tests/qualification/text-worker-escape/run-ubuntu.sh

.PHONY: text-role-durable-state-capture
text-role-durable-state-capture:
	@test "$$(go env GOOS)" = linux || (echo "text-role-durable-state-capture requires Linux"; exit 1)
	@test -n "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)" || (echo "ARDENTS_TEXT_ROLE_OBSERVATIONS must name an absolute writable capture directory"; exit 1)
	@test "$$(dirname "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)")" != "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)" || (echo "ARDENTS_TEXT_ROLE_OBSERVATIONS must not be a filesystem root"; exit 1)
	@test -d "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)" && test -w "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)" && test ! -L "$(ARDENTS_TEXT_ROLE_OBSERVATIONS)" || (echo "ARDENTS_TEXT_ROLE_OBSERVATIONS must name an existing writable non-symlink directory"; exit 1)
	go test ./internal/endpoint -run '^TestTextPublicationIsolatedRoleObservations$$' -count=1 -timeout=4m

# Explicit local diagnostics; ordinary checks stay Docker-free.
.PHONY: diagnostics-check
diagnostics-check:
	@test "$$(go env GOOS)" = linux || (echo "diagnostics-check requires Linux"; exit 2)
	go test ./scripts/diagnostics/diagnostic-command.go ./scripts/diagnostics/diagnostic-capture.go ./scripts/diagnostics/diagnostic-view.go ./scripts/diagnostics/diagnostic-report.go ./scripts/diagnostics/diagnostic-monitor.go scripts/diagnostics/diagnostic-monitor-view.go scripts/diagnostics/diagnostic-monitor-collector.go scripts/diagnostics/diagnostic-monitor_test.go ./scripts/diagnostics/diagnostic-log-retention.go ./scripts/diagnostics/diagnostic-log-retention_test.go ./scripts/diagnostics/diagnostic-capture_test.go ./scripts/diagnostics/diagnostic-report_test.go ./scripts/diagnostics/diagnostic-evidence.go ./scripts/diagnostics/diagnostic-evidence_test.go -count=1 -timeout=1m

.PHONY: hosting-check
hosting-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/successor/hosting ./cmd/ardents-next -count=1 -timeout=3m,$(error hosting-check requires Linux))

.PHONY: admission-check
admission-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/successor/admission ./internal/successor/admission/issuerprofile ./cmd/ardents-next -count=1 -timeout=3m,$(error admission-check requires Linux))

.PHONY: issuance-check
issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/successor/admission/issuance ./cmd/ardents-next -count=1 -timeout=3m,$(error issuance-check requires Linux))

.PHONY: token-issuance-check
token-issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/successor/admission ./internal/successor/admission/issuerprofile ./internal/successor/admission/issuance ./internal/successor/admission/issuer ./cmd/ardents-next -count=1 -timeout=3m,$(error token-issuance-check requires Linux))

.PHONY: issuer-profile-check
issuer-profile-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/successor/admission ./internal/successor/admission/issuerprofile ./internal/successor/nodeidentity ./internal/successor/admission/issuance ./internal/successor/admission/issuer ./cmd/ardents-next -count=1 -timeout=5m,$(error issuer-profile-check requires Linux))
	go test ./internal/successor/admission/issuerprofile -run '^$$' -fuzz '^FuzzIssuerProfile$$' -fuzztime=30s -parallel=2 -timeout=90s
	go test ./internal/successor/nodeidentity -run '^$$' -fuzz '^FuzzNodeIdentityPEM$$' -fuzztime=30s -parallel=2 -timeout=90s
