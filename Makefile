SHELL := /bin/sh

# Fast product checks are container-free. Selected qualification scenarios use
# explicit targets and prerequisites; ordinary checks never infer a live
# environment.

QUALITY_CACHE_ROOT ?= $(if $(TEMP),$(TEMP),/tmp)/ardents-network-quality
# A successful full gate trims the shared Go build cache back to this budget.
QUALITY_CACHE_BUILD_BUDGET_GIB ?= 8
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

.PHONY: architecture artifact-representation-check build check deadcode e2e format format-check fuzz headless-build headless-check headless-evidence heapdump-capture heapdump-role-map installed-tag-compile-check mod-check package-e2e package-ubuntu-deb qualification-service-credential-response-linux quality-cache-trim quick-check staticcheck test test-race text-role-durable-state-capture tools-check tools-install unit vet vuln

define newline


endef
UNIT_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/deterministic-packages.txt))
PROCESS_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/process-packages.txt))
HEADLESS_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/headless-commands.txt))
ifneq ($(filter source-process-check node-development-check node-process-check node-issuer-check node-descriptor-check node-introduction-check node-join-check,$(MAKECMDGOALS)),)
# This native driver consumes independently built, pinned artifacts. Its host
# needs the actual manager/kernel, not a second Go installation.
HEADLESS_GOOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HEADLESS_GOARCH := $(shell uname -m)
else
HEADLESS_GOOS := $(shell go env GOOS)
HEADLESS_GOARCH := $(shell go env GOARCH)
endif
HEADLESS_PLATFORM := $(HEADLESS_GOOS)-$(HEADLESS_GOARCH)
ifeq ($(HEADLESS_GOOS),linux)
UNIT_PACKAGES += $(subst $(newline), ,$(file <tests/profiles/deterministic-linux-packages.txt))
endif
HEADLESS_SUFFIX := $(if $(filter windows,$(HEADLESS_GOOS)),.exe,)
HEADLESS_ARTIFACT_ROOT ?= $(QUALITY_CACHE_ROOT)/headless-artifacts/$(HEADLESS_PLATFORM)
HEADLESS_ENDPOINT_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CUSTODY_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-custody-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
# The text_worker_installed surface only runs on dedicated Ubuntu hosts, so no
# runtime gate compiles it. This target cross-compiles the tagged Linux test
# packages from any host; the test binaries land in the quality cache, never in
# the repository.
INSTALLED_TAG_COMPILE_ROOT := $(QUALITY_CACHE_ROOT)/installed-tag-compile
INSTALLED_TAG_COMPILE_PACKAGES := ./internal/execution/runtime ./internal/execution/worker ./cmd/ardents-next

ifeq ($(OS),Windows_NT)
INSTALLED_TAG_COMPILE_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(INSTALLED_TAG_COMPILE_ROOT)') | Out-Null"
else
INSTALLED_TAG_COMPILE_MKDIR = mkdir -p "$(INSTALLED_TAG_COMPILE_ROOT)"
endif

# Installed integrity checks freshly compare the whole signed executable. Omit
# DWARF from distributed commands while retaining symbols for stacks/profiles.
override CANONICAL_GO_BUILD_FLAGS := -trimpath -buildvcs=false -ldflags=-w
QUICK_CHECK_TARGETS := vet unit build mod-check artifact-representation-check installed-tag-compile-check release-operation-compile-check installation-native-compile-check network-preparation-compile-check network-signing-compile-check

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

# Pure launch policy/Release handoff and independently retained Route history.
# This target starts no system manager, runtime listener or installed process.
.PHONY: platform-launch-unit
platform-launch-unit:
	@test "$(HEADLESS_GOOS)" = linux || (echo "platform-launch-unit requires Linux"; exit 2)
	go test ./cmd/ardents-next -run '^Test(Platform.*|SourceCancellationJoinsWaitCloseAndInputsBeforeCompletion|SourceInitialPreparation.*|SourcePreparationConfig.*|SourcePreparationRefusesOtherPrincipal.*|NodeRolesRefuseMissingNetworkHistoryWithoutInitializing)$$' -count=1 -timeout=3m
	go test ./internal/node/provisioning ./internal/platform/artifacts -run '^Test(SourceInitialPreparation|SourcePreparation|ArtifactInputs)' -count=1 -timeout=3m
	go test ./internal/network/state -run '^Test(Source(Participation|Completion).*|FreshStateAuthenticatesOnceAndRetainedOpeningKeepsHistory|RetainedStateReopensRealSignedHistoryAndKeepsSuccessorFloor|RetainedStateDoesNotInitializeAbsentOrEmptiedRoot)$$' -count=1 -timeout=3m
	go test ./internal/network/duty -run '^TestSourceCompletionInspectionPolicyRefusesUnjoinedProducer$$' -count=1 -timeout=3m
	go test ./internal/route/introduction -run '^TestRetainedReceivingRegistryPreservesClaimAndRejectsErasedHistory$$' -count=1 -timeout=3m
	go test ./internal/platform/systemd -run '^TestPlatformSiblingVocabularyPreservesFixedNodeExclusion$$' -count=1 -timeout=3m

heapdump-capture:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_REPORT)" || (echo "ARDENTS_HEAPDUMP_REPORT is required"; exit 2)
	go test -tags heapdumpcapture ./internal/diagnostics/process -run '^TestHeapDumpObservation$$' -count=1

heapdump-role-map:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_ROLE_MAP)" || (echo "ARDENTS_HEAPDUMP_ROLE_MAP is required"; exit 2)
	go test -tags heapdumpcapture ./internal/diagnostics/process -run '^TestHeapDumpRoleMapObservation$$' -count=1 -timeout=5m

e2e:
	go test -p 1 $(PROCESS_PACKAGES) -shuffle=on -count=1

package-e2e:
	sudo env "PATH=$$PATH" "GOTOOLCHAIN=$(GOTOOLCHAIN)" "GOENV=$(GOENV)" "GOFLAGS=$(GOFLAGS)" "GOCACHE=$(GOCACHE)" "GOMODCACHE=$(GOMODCACHE)" go test -tags packagee2e ./tests/e2e/endpoint -run '^TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes$$' -shuffle=on -count=1

artifact-representation-check: export ARDENTS_CANONICAL_BUILD_REPRESENTATIONS := 1
artifact-representation-check:
	go test ./internal/architecture -run '^TestCanonicalCommandBuildIsRepositoryRepresentationIndependent$$' -count=1

headless-artifact-name = $(if $(filter ./cmd/ardents-next,$(1)),ardents,$(notdir $(1)))

headless-build:
	$(HEADLESS_ARTIFACT_MKDIR)
	$(foreach command,$(HEADLESS_COMMANDS),go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(HEADLESS_ARTIFACT_ROOT)/$(call headless-artifact-name,$(command))-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)" $(command)$(newline))

headless-evidence: export ARDENTS_E2E_COMMAND := $(abspath $(HEADLESS_ENDPOINT_ARTIFACT))
headless-evidence: export ARDENTS_E2E_PRODUCT_ARDENTS_NEXT := $(abspath $(HEADLESS_ENDPOINT_ARTIFACT))
headless-evidence: export ARDENTS_E2E_PRODUCT_ARDENTS_CUSTODY := $(abspath $(HEADLESS_CUSTODY_ARTIFACT))
headless-evidence: headless-build
	@test "$(HEADLESS_GOOS)" = linux && test "$(HEADLESS_GOARCH)" = amd64 || (echo "headless-evidence requires Linux x86-64"; exit 2)
	"$(HEADLESS_ARTIFACT_SHELL)" ./packaging/alpha-bundle/test.sh "$(HEADLESS_PLATFORM)" "$(abspath $(HEADLESS_ENDPOINT_ARTIFACT))" "$(abspath $(HEADLESS_CUSTODY_ARTIFACT))"
	go test ./internal/enrollment -run '^(TestBundleMetadataProjectionKeepsStaticInventoryWithItsOwner|TestBundleRequiresCustodyAndRefusesForeignCompanions|TestBundleRefusalStagesReturnNoProvenance|TestBundleActualInventoryAndByteChangesRefuse)$$' -count=1
	go test ./tests/e2e/endpoint -run '^(TestHeadlessEnrollmentAcceptsExactRunningBundleAndRejectsChangedManifest)$$' -count=1
	go test ./tests/e2e/service -run '^TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses$$' -count=1

headless-check: export ARDENTS_EXTRACTION_OWNER := network
headless-check: headless-evidence
	go test ./internal/architecture -run '^TestNetworkExtractionRehearsal$$' -count=1

qualification-service-credential-response-linux:
	sh ./tests/qualification/service-credential-response-linux/run-ubuntu.sh -timeout=2m

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
	staticcheck ./scripts/install-quality-analyzer.go
	staticcheck ./scripts/enrollment-artifact-name.go
	staticcheck ./scripts/prepare-qualification-release-keys.go
	staticcheck ./scripts/protected-generation-check.go
	staticcheck ./scripts/run-fuzz-targets.go
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
	go test -c -tags startup_manager_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-systemd-startup-native.test" ./internal/platform/systemd
	$(foreach package,$(INSTALLED_TAG_COMPILE_PACKAGES),go test -c -tags text_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/$(subst /,_,$(package)).test" $(package)$(newline))

.PHONY: node-process-check
.PHONY: node-development-check
node-development-check:
	python3 -I -S ./scripts/node/run-development.py baseline "$(ARDENTS_DEVELOPMENT_LOCK_PARENT)" "$(ARDENTS_NODE_DEVELOPMENT_OUTPUT)" -timeout=90m

node-process-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "node-process-check requires Linux"; exit 2)
	sh ./scripts/node/run-native.sh -timeout=3m

.PHONY: node-issuer-check
node-issuer-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "node-issuer-check requires Linux"; exit 2)
	sh ./scripts/node/run-issuer-native.sh -timeout=3m

.PHONY: node-descriptor-check
node-descriptor-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "node-descriptor-check requires Linux"; exit 2)
	sh ./scripts/node/run-descriptor-native.sh -timeout=3m

.PHONY: node-introduction-check
node-introduction-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "node-introduction-check requires Linux"; exit 2)
	sh ./scripts/node/run-introduction-native.sh -timeout=3m

.PHONY: node-join-check
node-join-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "node-join-check requires Linux"; exit 2)
	sh ./scripts/node/run-join-native.sh -timeout=3m

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
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/prepare-qualification-release-keys" ./scripts/prepare-qualification-release-keys.go
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/sign-qualification-release" ./scripts/sign-qualification-release.go

check:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS) staticcheck vuln deadcode
	$(MAKE) --output-sync=target e2e
ifeq ($(HEADLESS_GOOS),linux)
	$(MAKE) --output-sync=target package-e2e
endif
	$(MAKE) --output-sync=target test-race
	$(MAKE) quality-cache-trim

# The build cache is the only unbounded member of the quality cache; the module
# cache, analyzer caches and compiled artifacts stay warm. A failed gate keeps
# the cache for diagnosis, and the session removes its own disposable workdirs.
.PHONY: quality-cache-trim
quality-cache-trim:
ifeq ($(OS),Windows_NT)
	@echo "quality-cache-trim: POSIX-only; trim $(QUALITY_CACHE_ROOT)/go-build manually on Windows"
else
	sh ./scripts/trim-quality-cache.sh "$(QUALITY_CACHE_ROOT)" "$(CURDIR)" "$(QUALITY_CACHE_BUILD_BUDGET_GIB)"
endif

DIAGNOSTIC_PARSER_ROOT ?= $(if $(GOBIN),$(GOBIN),$(shell go env GOPATH)/bin)

tools-install:
ifeq ($(NODE_NATIVE_HOST_TOOLS_ONLY),1)
ifeq ($(OS),Windows_NT)
	wsl --distribution Ubuntu-24.04 --user root --cd "$(CURDIR)" -- env "ARDENTS_NODE_NATIVE_MAKE_PACKAGE=$(NODE_NATIVE_MAKE_PACKAGE)" sh ./scripts/node/install-host-tools.sh
else
	ARDENTS_NODE_NATIVE_MAKE_PACKAGE="$(NODE_NATIVE_MAKE_PACKAGE)" sh ./scripts/node/install-host-tools.sh
endif
else
	go build -trimpath -buildvcs=false -o "$(DIAGNOSTIC_PARSER_ROOT)/pprof$(HEADLESS_SUFFIX)" cmd/pprof
	go build -trimpath -buildvcs=false -o "$(DIAGNOSTIC_PARSER_ROOT)/trace$(HEADLESS_SUFFIX)" cmd/trace
ifneq ($(DIAGNOSTIC_PARSERS_ONLY),1)
	go run ./scripts/install-quality-analyzer.go -tool staticcheck -cache "$(QUALITY_CACHE_ROOT)" -output "$(DIAGNOSTIC_PARSER_ROOT)"
	go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
	go install golang.org/x/tools/cmd/deadcode@v0.50.0
ifeq ($(DIAGNOSTIC_TOOLS),1)
	go install github.com/go-delve/delve/cmd/dlv@v1.27.2
	go run ./scripts/install-quality-analyzer.go -tool errcheck -cache "$(QUALITY_CACHE_ROOT)" -output "$(DIAGNOSTIC_PARSER_ROOT)"
	sh ./scripts/diagnostics/install-powershell.sh
	go run ./scripts/check-tools.go -diagnostic
endif
endif
endif

.PHONY: text-worker-policy-check
text-worker-policy-check:
	sh ./tests/qualification/text-worker-policy/run-ubuntu.sh

.PHONY: text-worker-lifecycle-check
text-worker-lifecycle-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh

.PHONY: text-worker-network-check
text-worker-network-check:
	sh ./tests/qualification/text-worker-network/run-ubuntu.sh

.PHONY: execution-lifecycle-check
execution-lifecycle-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh execution

.PHONY: execution-route-check
execution-route-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh execution-route

.PHONY: connection-stream-check
.PHONY: startup-manager-check
startup-manager-check:
	sh ./tests/qualification/installation-startup-manager/run-ubuntu.sh -timeout=20s

connection-stream-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-stream

.PHONY: connection-request-recovery-check
connection-request-recovery-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-request-recovery

.PHONY: connection-control-check connection-control-received-check connection-control-delivery-check connection-authority-check

connection-authority-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-authority

connection-control-delivery-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-control-delivery

connection-control-received-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-control-received
.PHONY: connection-reopen-check
connection-reopen-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-reopen

connection-control-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-control

.PHONY: connection-refresh-check
connection-refresh-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-refresh

.PHONY: connection-withdrawal-check
connection-withdrawal-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-withdrawal

.PHONY: connection-nonce-check
connection-nonce-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh connection-nonce

.PHONY: publication-timing-check
publication-timing-check:
	sh ./tests/qualification/text-worker-lifecycle/run-ubuntu.sh publication-timing -timeout=15m

.PHONY: execution-tree-check
execution-tree-check:
	sh ./tests/qualification/execution-tree/run-ubuntu.sh

.PHONY: execution-escape-check
execution-escape-check:
	sh ./tests/qualification/execution-escape/run-ubuntu.sh

.PHONY: execution-recovery-check
execution-recovery-check:
	sh ./tests/qualification/execution-recovery/run-ubuntu.sh -timeout=2m

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
.PHONY: installation-native-check installation-native-compile-check
installation-native-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "installation-native-check requires Linux"; exit 2)
	@test "$$(id -u)" = 0 || (echo "installation-native-check requires root"; exit 2)
	@test -n "$(ARDENTS_INSTALLATION_NATIVE_ROOT)" || (echo "ARDENTS_INSTALLATION_NATIVE_ROOT must name a trusted root-owned temporary parent"; exit 2)
	go test -race -tags installation_native ./internal/endpoint/launch ./internal/endpoint/plan ./internal/endpoint/generation ./internal/endpoint/unit ./internal/platform/cgroup ./internal/platform/process ./internal/platform/systemd ./internal/release/endpoint ./internal/release/infrastructure -run '^TestInstallationNative' -count=1 -timeout=1m

installation-native-compile-check: export GOOS := linux
installation-native-compile-check: export GOARCH := amd64
installation-native-compile-check: export CGO_ENABLED := 0
installation-native-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-native.test" ./internal/release/endpoint
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-endpoint-native.test" ./internal/endpoint/launch
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-request-native.test" ./internal/endpoint/plan
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-cgroup-native.test" ./internal/platform/cgroup
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-systemd-native.test" ./internal/platform/systemd
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-process-native.test" ./internal/platform/process
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-generation-native.test" ./internal/endpoint/generation

# Hosting measures whole interfaces shared by these packages.
.PHONY: route-check
route-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -p 1 -race ./internal/network/... ./internal/admission/... ./internal/hosting ./internal/publication/... ./internal/connection/... ./internal/reachability/... ./internal/enrollment/... ./internal/release/... ./internal/platform/... ./internal/execution/... ./internal/route/... ./internal/endpoint/... ./cmd/ardents-next -count=1 -timeout=5m,$(error route-check requires Linux))
	go test ./internal/architecture -run '^(TestRouteMigration(ImportIsolation|IsolationPolicy)|TestSuccessor.*|TestPackageProfileMembershipIsComplete|TestProfilePackageEntriesAreCurrent|TestLinuxOnlyProfileNamesActualPlatformPackages|TestTestProfileRegistryIsFactualAndWired)$$' -count=1

hosting-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/hosting ./cmd/ardents-next -count=1 -timeout=3m,$(error hosting-check requires Linux))

.PHONY: admission-check
admission-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission/... ./cmd/ardents-next -count=1 -timeout=5m,$(error admission-check requires Linux))

.PHONY: issuance-check
issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission/issuance ./cmd/ardents-next -count=1 -timeout=3m,$(error issuance-check requires Linux))

.PHONY: token-issuance-check
token-issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission ./internal/admission/issuerprofile ./internal/admission/issuance ./internal/admission/issuer ./cmd/ardents-next -count=1 -timeout=5m,$(error token-issuance-check requires Linux))

.PHONY: issuer-profile-check
issuer-profile-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission ./internal/admission/issuerprofile ./internal/admission/issuerprofile/signing ./internal/admission/issuance ./internal/admission/issuer ./cmd/ardents-next -count=1 -timeout=5m,$(error issuer-profile-check requires Linux))
	go test ./internal/admission/issuerprofile -run '^$$' -fuzz '^FuzzIssuerProfile$$' -fuzztime=30s -parallel=2 -timeout=90s
	go test ./internal/admission/issuerprofile/signing -run '^$$' -fuzz '^FuzzNodeIdentityPEM$$' -fuzztime=30s -parallel=2 -timeout=90s

.PHONY: custody-check
custody-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/custody ./cmd/ardents-custody ./tests/e2e/service -count=1 -timeout=3m,$(error custody-check requires Linux))
	go test golang.org/x/crypto/argon2 -run '^(TestArgon2|TestVectors)$$' -count=1

.PHONY: source-process-check
source-process-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "source-process-check requires Linux"; exit 2)
	sh scripts/node/run-source-native.sh -timeout=3m

.PHONY: network-preparation-check network-preparation-compile-check
network-preparation-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "network-preparation-check requires Linux and native ext4 TMPDIR"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_preparation_native ./cmd/ardents-next -run '^TestNetworkPreparation' -count=1 -v -timeout=3m

network-preparation-compile-check: export GOOS := linux
network-preparation-compile-check: export GOARCH := amd64
network-preparation-compile-check: export CGO_ENABLED := 0
network-preparation-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags network_preparation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/network-preparation.test" ./cmd/ardents-next

.PHONY: network-signing-check network-signing-compile-check

network-signing-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "network-signing-check requires Linux root and root-controlled native ext4 TMPDIR"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./cmd/ardents-next -run '^Test(NetworkSigning|SourcePreparationNative)' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/state -run '^TestSourceInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/state -run '^TestIntroductionRepairRootsNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/duty ./internal/network/state -run '^TestNodeInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/admission/spending -run '^TestSpendInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/hosting -run '^TestHostingInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/platform/systemd -run '^TestSourceReferenceLibraryCloseCause' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/endpoint/plan -run '^TestDeliveryFilesNative' -count=1 -v -timeout=3m

	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/platform/artifacts -run '^TestSourcePreparationNativeOutputSubstitutionRefuses$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/node/provisioning -run '^Test(ControlProvisioningKeysNativeRoleAndPurpose|Role(NodeProvision|EpochProvision|Profile|TokenMaterial)Native|Forwarding(TokenAuthority|TokenMaterial)Native)' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^TestControlReleaseKeyReceiptsNative' -count=1 -v -timeout=3m

network-signing-compile-check: export GOOS := linux
network-signing-compile-check: export GOARCH := amd64
network-signing-compile-check: export CGO_ENABLED := 0
network-signing-compile-check:
	@$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/network-signing.test" ./cmd/ardents-next
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/infrastructure-provisioning.test" ./internal/node/provisioning
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/infrastructure-release-signing.test" ./internal/release/infrastructure/signing
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/operator-artifacts.test" ./internal/platform/artifacts
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/ardents-infrastructure" ./cmd/ardents-next
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-preflight-manager.test" ./internal/platform/systemd
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-preflight-owner.test" ./internal/release/infrastructure
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-state-inspection.test" ./internal/network/state
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/delivery-files.test" ./internal/endpoint/plan
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/descriptor-store-inspection.test" ./internal/reachability

.PHONY: descriptor-store-inspection-check
descriptor-store-inspection-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "descriptor-store-inspection-check requires explicit Linux root/ext4 stand"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/reachability -run '^TestDescriptorStoreInspectionNative' -count=1 -v -timeout=3m

.PHONY: descriptor-forward-release-check
descriptor-forward-release-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "descriptor-forward-release-check requires explicit root/ext4 canonical artifact stand"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^TestDescriptorPreparationNativeForwardReleasePreservesRootAndRaisesFloors$$' -count=1 -v -timeout=3m
