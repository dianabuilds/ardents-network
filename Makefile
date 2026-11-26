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

.PHONY: architecture artifact-representation-check build check deadcode e2e format format-check fuzz headless-build headless-check headless-evidence heapdump-capture heapdump-role-map installed-tag-compile-check platform-installed-compile-check mod-check package-e2e package-ubuntu-deb qualification-service-credential-response-linux quick-check staticcheck test test-race tools-check tools-install unit vet vuln

define newline


endef
read-profile-list = $(or $(strip $(file <$1)),$(error Cannot read $1. Check the inventory and select the registered GNU Make))
# Read inventories only when the selected target uses them. Tool bootstrap
# can run before the registered Make is available.
UNIT_PACKAGES = $(call read-profile-list,tests/profiles/deterministic-packages.txt)
PROCESS_PACKAGES = $(call read-profile-list,tests/profiles/process-packages.txt)
HEADLESS_COMMANDS = $(call read-profile-list,tests/profiles/headless-commands.txt)
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
# The Linux supplement may be empty when all packages also build on other hosts.
UNIT_PACKAGES += $(strip $(file <tests/profiles/deterministic-linux-packages.txt))
endif
HEADLESS_SUFFIX := $(if $(filter windows,$(HEADLESS_GOOS)),.exe,)
HEADLESS_ARTIFACT_ROOT ?= $(QUALITY_CACHE_ROOT)/headless-artifacts/$(HEADLESS_PLATFORM)
HEADLESS_ENDPOINT_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
HEADLESS_CUSTODY_ARTIFACT := $(HEADLESS_ARTIFACT_ROOT)/ardents-custody-$(HEADLESS_PLATFORM)$(HEADLESS_SUFFIX)
# The document_worker_installed surface only runs on dedicated Ubuntu hosts, so no
# runtime gate compiles it. This target cross-compiles the tagged Linux test
# packages from any host; the test binaries land in the quality cache, never in
# the repository.
INSTALLED_TAG_COMPILE_ROOT := $(QUALITY_CACHE_ROOT)/installed-tag-compile
INSTALLED_TAG_COMPILE_PACKAGES := ./internal/domain/execution/runtime ./internal/domain/execution/worker ./internal/application/document ./internal/endpoint

ifeq ($(OS),Windows_NT)
INSTALLED_TAG_COMPILE_MKDIR = powershell -NoProfile -Command "[System.IO.Directory]::CreateDirectory('$(INSTALLED_TAG_COMPILE_ROOT)') | Out-Null"
else
INSTALLED_TAG_COMPILE_MKDIR = mkdir -p "$(INSTALLED_TAG_COMPILE_ROOT)"
endif

# Installed integrity checks freshly compare the whole signed executable. Omit
# DWARF from distributed commands while retaining symbols for stacks/profiles.
override CANONICAL_GO_BUILD_FLAGS := -trimpath -buildvcs=false -ldflags=-w
QUICK_CHECK_TARGETS := vet unit build mod-check artifact-representation-check installed-tag-compile-check platform-installed-compile-check release-operation-compile-check installation-native-compile-check network-preparation-compile-check network-signing-compile-check

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
	go test ./internal/architecture -run '^(TestRepositoryArchitecture|TestMakeProfileSelectorsHaveTests)$$' -count=1

vet:
	go vet ./...

UNIT_TEST_FLAGS := -short -shuffle=on -count=1 -timeout=15m
unit:
	go test -p 1 $(UNIT_PACKAGES) $(UNIT_TEST_FLAGS)

# Pure launch policy/Release handoff and independently retained Route history.
# This target starts no system manager, runtime listener or installed process.
.PHONY: platform-launch-unit
platform-launch-unit:
	@test "$(HEADLESS_GOOS)" = linux || (echo "platform-launch-unit requires Linux"; exit 2)
	go test ./internal/release/infrastructure/launch -run '^Test(Platform|IssuerProjectionRetainsKeyLeaseOnly$$)' -count=1 -timeout=3m
	go test ./internal/release/infrastructure -run '^TestPlatform(RuntimeRelease|SourceForwardProgram)' -count=1 -timeout=3m
	go test ./tests/e2e/node -run '^TestNodeRolesRefuseMissingNetworkHistoryWithoutInitializing$$' -count=1 -timeout=3m
	go test ./internal/network/source/command -run '^TestSourcePreparationRefusesOtherPrincipalBeforeInputEffects$$' -count=1 -timeout=3m
	go test ./internal/network/source/serving -run '^TestSourceCancellationJoinsWaitCloseAndInputsBeforeCompletion$$' -count=1 -timeout=3m
	go test ./internal/domain/node/provisioning -run '^TestSourceInitialNetworkPlanKeepsDistinctPurposeAndPublicPins$$' -count=1 -timeout=3m
	go test ./internal/platform/artifacts -run '^TestArtifactInputs' -count=1 -timeout=3m
	go test ./internal/platform/process -run '^TestDedicatedAccountID$$' -count=1 -timeout=3m
	go test ./internal/network/state -run '^Test(Source(Participation|Completion|InitialPreparation|PreparationConfig).*|FreshStateAuthenticatesOnceAndRetainedOpeningKeepsHistory|RetainedStateReopensRealSignedHistoryAndKeepsSuccessorFloor|RetainedStateDoesNotInitializeAbsentOrEmptiedRoot)$$' -count=1 -timeout=3m
	go test ./internal/network/duty -run '^TestSourceCompletionInspectionPolicyRefusesUnjoinedProducer$$' -count=1 -timeout=3m
	go test ./internal/route/introduction -run '^TestRetainedReceivingRegistryPreservesClaimAndRejectsErasedHistory$$' -count=1 -timeout=3m
	go test ./internal/platform/systemd -run '^TestPlatform(SiblingVocabularyPreservesFixedNodeExclusion|PrestartRejectsDifferentLoadedInvocationAndPolicy)$$' -count=1 -timeout=3m

heapdump-capture:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_REPORT)" || (echo "ARDENTS_HEAPDUMP_REPORT is required"; exit 2)
	go test -tags heapdumpcapture ./tests/diagnostics/heapdump -run '^TestHeapDumpObservation$$' -count=1

heapdump-role-map:
	@test -n "$(ARDENTS_HEAPDUMP_INPUT_ROOT)" || (echo "ARDENTS_HEAPDUMP_INPUT_ROOT is required"; exit 2)
	@test -n "$(ARDENTS_HEAPDUMP_ROLE_MAP)" || (echo "ARDENTS_HEAPDUMP_ROLE_MAP is required"; exit 2)
	go test -tags heapdumpcapture ./tests/diagnostics/heapdump -run '^TestHeapDumpRoleMapObservation$$' -count=1 -timeout=5m

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
	"$(HEADLESS_ARTIFACT_SHELL)" ./packaging/alpha-bundle/test.sh "$(HEADLESS_PLATFORM)" "$(abspath $(HEADLESS_ENDPOINT_ARTIFACT))"
	go test ./internal/domain/enrollment -run '^(TestBundleMetadataProjectionKeepsStaticInventoryWithItsOwner|TestBundleRefusesForeignCompanions|TestBundleRefusalStagesReturnNoProvenance|TestBundleActualInventoryAndByteChangesRefuse)$$' -count=1
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

RACE_TEST_FLAGS := -short -race -shuffle=on -count=1 -timeout=15m
test-race:
	$(RACE_TEST_PREFIX) go test -p 1 $(UNIT_PACKAGES) $(RACE_TEST_FLAGS)

.PHONY: race-shard-packages test-race-shard
# Each shard remains serial and runs on its own CI host. Keep the two most
# expensive packages apart while distributing the rest of the full inventory.
race-shard-packages:
	@set -eu; \
	case "$(RACE_SHARD)" in 0|1|2|3) ;; *) echo "RACE_SHARD must be 0, 1, 2 or 3" >&2; exit 2;; esac; \
	printf '%s\n' $(UNIT_PACKAGES) | awk -v shard="$(RACE_SHARD)" '{ assigned = ($$0 ~ /\/internal\/domain\/custody$$/) ? 3 : (NR-1)%4; if (assigned == shard) print }'

test-race-shard:
	@set -eu; \
	packages=$$($(MAKE) --no-print-directory race-shard-packages RACE_SHARD=$(RACE_SHARD)); \
	test -n "$$packages"; \
	$(RACE_TEST_PREFIX) go test -p 1 $$packages $(RACE_TEST_FLAGS)

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
	staticcheck ./scripts/prepare-qualification-release-keys.go
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
	$(foreach package,$(INSTALLED_TAG_COMPILE_PACKAGES),go test -c -tags document_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/$(subst /,_,$(package)).test" $(package)$(newline))

# Platform-installed gates are compile-checked only: their executed proof is a
# manual root-driver run on an admitted Ubuntu/systemd host (see testing.md), not
# an automated make lane. This target keeps their test binaries compiling so the
# compile-checked-only surface is an explicit gate, not a deadcode side effect.
platform-installed-compile-check: export GOOS := linux
platform-installed-compile-check: export GOARCH := amd64
platform-installed-compile-check: export CGO_ENABLED := 0
platform-installed-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags endpoint_platform_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-platform-installed.test" ./tests/e2e/endpoint
	go test -c -tags endpoint_launch_native,document_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-launch-installed.test" ./internal/endpoint
	go test -c -tags endpoint_pair_native,document_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-pair-installed.test" ./internal/endpoint
	go test -c -tags endpoint_pair_native,document_worker_installed -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-manager-pair.test" ./tests/e2e/endpoint
	go test -c -tags source_installed_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-installed.test" ./tests/e2e/node
	go test -c -tags join_platform_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/join-platform-installed.test" ./tests/e2e/node
	go test -c -tags forwarding_platform_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/forwarding-platform-installed.test" ./tests/e2e/node
	go test -c -tags descriptor_platform_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/descriptor-platform-installed.test" ./tests/e2e/node
	go test -c -tags introduction_repair_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/introduction-repair-installed.test" ./tests/e2e/node
	go test -c -tags endpoint_launch_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-latency-participant.test" ./internal/endpoint
	go test -c -tags endpoint_launch_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/endpoint-idle-observation.test" ./tests/e2e/endpoint

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

check: check-core
	$(MAKE) --output-sync=target test-race

.PHONY: check-core
check-core:
	$(MAKE) --output-sync=target -j 4 $(QUICK_CHECK_TARGETS) staticcheck vuln deadcode
	$(MAKE) --output-sync=target e2e
ifeq ($(HEADLESS_GOOS),linux)
	$(MAKE) --output-sync=target offline-admission-e2e-race
	$(MAKE) --output-sync=target package-e2e
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
	sh ./tests/qualification/document-worker-policy/run-ubuntu.sh

.PHONY: text-worker-lifecycle-check
text-worker-lifecycle-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh

.PHONY: text-worker-network-check
text-worker-network-check:
	sh ./tests/qualification/document-worker-network/run-ubuntu.sh

.PHONY: execution-lifecycle-check
execution-lifecycle-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh execution

.PHONY: execution-route-check
execution-route-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh execution-route

.PHONY: connection-stream-check
.PHONY: startup-manager-check
startup-manager-check:
	sh ./tests/qualification/installation-startup-manager/run-ubuntu.sh -timeout=20s

connection-stream-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-stream

.PHONY: connection-request-recovery-check
connection-request-recovery-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-request-recovery

.PHONY: connection-control-check connection-control-received-check connection-control-delivery-check connection-authority-check

connection-authority-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-authority

connection-control-delivery-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-control-delivery

connection-control-received-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-control-received
.PHONY: connection-reopen-check
connection-reopen-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-reopen

connection-control-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-control

.PHONY: connection-refresh-check
connection-refresh-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-refresh

.PHONY: connection-withdrawal-check
connection-withdrawal-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-withdrawal

.PHONY: connection-nonce-check
connection-nonce-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh connection-nonce

.PHONY: publication-timing-check
publication-timing-check:
	sh ./tests/qualification/worker-lifecycle/run-ubuntu.sh publication-timing -timeout=15m

.PHONY: execution-tree-check
execution-tree-check:
	sh ./tests/qualification/execution-tree/run-ubuntu.sh

.PHONY: execution-escape-check
execution-escape-check:
	sh ./tests/qualification/execution-escape/run-ubuntu.sh

.PHONY: execution-recovery-check
execution-recovery-check:
	sh ./tests/qualification/execution-recovery/run-ubuntu.sh -timeout=2m

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
	go test -race -tags installation_native ./internal/endpoint/launch ./internal/endpoint/plan ./internal/endpoint/generation ./internal/platform/cgroup ./internal/platform/process ./internal/platform/systemd -run '^TestInstallationNative' -count=1 -timeout=1m

installation-native-compile-check: export GOOS := linux
installation-native-compile-check: export GOARCH := amd64
installation-native-compile-check: export CGO_ENABLED := 0
installation-native-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-endpoint-native.test" ./internal/endpoint/launch
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-request-native.test" ./internal/endpoint/plan
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-cgroup-native.test" ./internal/platform/cgroup
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-systemd-native.test" ./internal/platform/systemd
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-process-native.test" ./internal/platform/process
	go test -c -tags installation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/installation-generation-native.test" ./internal/endpoint/generation

# Hosting measures whole interfaces shared by these packages.
.PHONY: route-check
route-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -p 1 -race ./internal/network/... ./internal/admission/... ./internal/hosting ./internal/publication/... ./internal/connection/... ./internal/reachability/... ./internal/domain/enrollment/... ./internal/release/... ./internal/platform/... ./internal/domain/execution/... ./internal/route/... ./internal/endpoint/... ./internal/domain/node ./tests/networkfixture -count=1 -timeout=5m,$(error route-check requires Linux))
	go test -race ./tests/e2e/node -run '^TestRoute' -count=1 -timeout=5m
	go test ./internal/architecture -run '^(TestRouteMigration(ImportIsolation|IsolationPolicy)|TestSuccessor.*|TestPackageProfileMembershipIsComplete|TestProfilePackageEntriesAreCurrent|TestLinuxOnlyProfileNamesActualPlatformPackages|TestTestProfileRegistryIsFactualAndWired|TestMakeProfileSelectorsHaveTests)$$' -count=1

hosting-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/hosting ./internal/admission/receiving ./internal/domain/node -count=1 -timeout=3m,$(error hosting-check requires Linux))
	go test -race ./tests/e2e/node -run '^Test(AdmissionHostingReservationAndWork|HostingPlanExactFields|HostingCompiledCLIInitializationAndObservation|HostingOTLPContainsOnlyFiniteAttributes|HostingCollectorFailurePreservesCommittedResult|CanceledSocketChildRetainsReservationUntilJoined)' -count=1 -timeout=3m

.PHONY: admission-check
admission-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission/... ./internal/endpoint/session ./internal/domain/node ./tests/networkfixture -count=1 -timeout=5m,$(error admission-check requires Linux))
	$(MAKE) --no-print-directory offline-admission-e2e-race

# Keep compiled offline command scenarios in the race inventory after their move
# out of the command's unit-test package.
.PHONY: offline-admission-e2e-race
offline-admission-e2e-race:
	go test -race ./tests/e2e/node -run '^Test(Admission(Fresh|Initialization|OTLP|StandaloneCommands|Consoles|ConsoleInput|HolderInterrupt|HostingReservationAndWork)|Issuance|ConfirmedIssuance|IssuerProfileCLI|ProfileCommandStrict)' -count=1 -timeout=3m

.PHONY: issuance-check
issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission/issuance ./internal/admission/operator ./internal/endpoint/session ./internal/domain/node ./tests/networkfixture -count=1 -timeout=3m,$(error issuance-check requires Linux))
	$(MAKE) --no-print-directory offline-admission-e2e-race

.PHONY: token-issuance-check
token-issuance-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission ./internal/admission/issuerprofile ./internal/admission/issuance ./internal/admission/issuer ./internal/admission/operator ./internal/endpoint/session ./internal/domain/node ./tests/networkfixture -count=1 -timeout=5m,$(error token-issuance-check requires Linux))

.PHONY: issuer-profile-check
issuer-profile-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/admission ./internal/admission/issuerprofile ./internal/admission/issuerprofile/signing ./internal/admission/issuance ./internal/admission/issuer ./internal/admission/operator ./internal/domain/node ./tests/networkfixture -count=1 -timeout=5m,$(error issuer-profile-check requires Linux))
	go test ./internal/admission/issuerprofile -run '^$$' -fuzz '^FuzzIssuerProfile$$' -fuzztime=30s -parallel=2 -timeout=90s
	go test ./internal/admission/issuerprofile/signing -run '^$$' -fuzz '^FuzzNodeIdentityPEM$$' -fuzztime=30s -parallel=2 -timeout=90s

.PHONY: custody-check
custody-check:
	$(if $(filter linux,$(shell go env GOOS)),go test -race ./internal/domain/custody ./internal/domain/custody/command ./tests/e2e/service -count=1 -timeout=3m,$(error custody-check requires Linux))
	go test golang.org/x/crypto/argon2 -run '^(TestArgon2|TestVectors)$$' -count=1

.PHONY: source-process-check
source-process-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "source-process-check requires Linux"; exit 2)
	sh scripts/node/run-source-native.sh -timeout=3m

.PHONY: network-preparation-check network-preparation-compile-check
network-preparation-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "network-preparation-check requires Linux and native ext4 TMPDIR"; exit 2)
	go test -race ./internal/platform/artifacts -run '^TestWriteReportJoinsCanceledFullPipe$$' -count=1 -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_preparation_native ./tests/e2e/node -run '^TestNetworkPreparation' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_preparation_native ./internal/domain/node/provisioning -run '^TestNetworkPreparation' -count=1 -v -timeout=3m

network-preparation-compile-check: export GOOS := linux
network-preparation-compile-check: export GOARCH := amd64
network-preparation-compile-check: export CGO_ENABLED := 0
network-preparation-compile-check:
	$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags network_preparation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/network-preparation.test" ./tests/e2e/node
	go test -c -tags network_preparation_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/network-publication.test" ./internal/domain/node/provisioning

.PHONY: network-signing-check network-signing-compile-check

network-signing-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "network-signing-check requires Linux root and root-controlled native ext4 TMPDIR"; exit 2)
	go test -race ./internal/platform/artifacts -run '^TestWriteReportJoinsCanceledFullPipe$$' -count=1 -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/endpoint -run '^TestSourcePreparationNative(CommitmentsAndCustody|OriginalProcessCuts|PartialWrite|ReferenceExpiry|Child)$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure -run '^Test(NetworkSigning|SourcePreparationNative)' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./tests/e2e/node -run '^Test(NetworkSigning(NodeCertificate(Command|Receipt)|StaticNativeRefusals|CommandsGenuineStateChain|Infrastructure(Preparation|RoleInventories)|InitialKeys(Command|StateChain|ReceiptFailure|BlockedReceipt|StaticRefusals))|SourcePreparationNative(PublicOnlyCommand|ReceiptFailure))$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/state -run '^TestSource(InspectionNative|PreparationNativeStateCommand)' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/state -run '^TestIntroductionRepairRootsNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/network/duty ./internal/network/state -run '^TestNodeInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/admission/spending -run '^TestSpendInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/hosting -run '^TestHostingInspectionNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/platform/systemd -run '^TestSourceReferenceLibraryCloseCause' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/platform/artifacts -run '^TestSourcePreparationNative(OutputSubstitutionRefuses|PublicMountRefusal)$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/domain/node/provisioning -run '^Test(NetworkSigning(InitialKeys|OriginalCancellationBeforeEffects|OriginalTimeAfterIO|DurableInventory|ConcurrentOutput|UnsupportedMount|OriginalInputCustody|OriginalRefusalCannotRevive|InterruptedChild|ActualPartialAndProcessCuts|Profile(NativeCommit|ActualExpiry|ReceiptExpiry|Contention)|NodeCertificate(LateTime|FinalOutputAndPhysicalFailure|Contention))|ControlProvisioningKeysNativeRoleAndPurpose|Role(NodeProvision|EpochProvision|Profile|TokenMaterial)Native|Forwarding(TokenAuthority|TokenMaterial)Native)' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^TestControlReleaseKeyReceiptsNative' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^Test(Forwarding|Join)PreparationNative(InitialReleasePurposeAndKeyCustody|ForwardReleasePreservesRootAndRaisesFloors)$$|^TestDescriptorPreparationNativeInitialReleasePurposeAndKeyCustody$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^TestSourcePreparationNativeInitialReleasePurposeAndKeyCustody$$' -count=1 -v -timeout=3m
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/domain/node/provisioning -run '^TestSourcePreparationNativeInitialNetworkAndNonrootState$$' -count=1 -v -timeout=3m

network-signing-compile-check: export GOOS := linux
network-signing-compile-check: export GOARCH := amd64
network-signing-compile-check: export CGO_ENABLED := 0
network-signing-compile-check:
	@$(INSTALLED_TAG_COMPILE_MKDIR)
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/infrastructure-provisioning.test" ./internal/domain/node/provisioning
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/infrastructure-release-signing.test" ./internal/release/infrastructure/signing
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/operator-artifacts.test" ./internal/platform/artifacts
	go build $(CANONICAL_GO_BUILD_FLAGS) -o "$(INSTALLED_TAG_COMPILE_ROOT)/ardents-infrastructure" ./cmd/ardents-next
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-preflight-manager.test" ./internal/platform/systemd
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-preflight-owner.test" ./internal/release/infrastructure
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-state-inspection.test" ./internal/network/state
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-trust-owner.test" ./internal/endpoint
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/source-public-preparation.test" ./tests/e2e/node
	go test -c -tags network_signing_native -o "$(INSTALLED_TAG_COMPILE_ROOT)/descriptor-store-inspection.test" ./internal/reachability

.PHONY: descriptor-store-inspection-check
descriptor-store-inspection-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "descriptor-store-inspection-check requires explicit Linux root/ext4 stand"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/reachability -run '^TestDescriptorStoreInspectionNative' -count=1 -v -timeout=3m

.PHONY: descriptor-forward-release-check
descriptor-forward-release-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "descriptor-forward-release-check requires explicit root/ext4 canonical artifact stand"; exit 2)
	go test -race -trimpath -buildvcs=false -tags network_signing_native ./internal/release/infrastructure/signing -run '^TestDescriptorPreparationNativeForwardReleasePreservesRootAndRaisesFloors$$' -count=1 -v -timeout=3m

.PHONY: worker-descriptor-audit-check
worker-descriptor-audit-check:
	@test "$(HEADLESS_GOOS)" = linux || (echo "worker-descriptor-audit-check requires Linux unified cgroup v2"; exit 2)
	go test -race -trimpath -buildvcs=false -tags document_worker_installed ./internal/application/document -run '^TestVerifyWorkerDescriptors' -count=1 -v -timeout=1m
