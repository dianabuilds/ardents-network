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

ifeq ($(OS),Windows_NT)
RACE_TEST_PREFIX :=
else
RACE_TEST_PREFIX := umask 077;
endif

.PHONY: architecture build check e2e format format-check fuzz headless-build headless-check headless-e2e mod-check package-ubuntu-deb prepare-h4-2-net-01a prepare-h4-5-rendezvous qualification qualification-h4-1a qualification-h4-1b qualification-h4-2-local-emulator qualification-h4-2-multihost qualification-h4-3b-docker qualification-h4-3b-multihost qualification-h4-3b-vps qualification-h4-4a-firefox qualification-h4-4-signed-firefox qualification-h4-4-signed-xpi qualification-h4-4-ubuntu-enrollment qualification-h4-4-windows-enrollment qualification-h4-5-rendezvous qualification-h4-6a-two-endpoints qualification-h4-8-a11 quick-check staticcheck test test-race tools-check tools-install unit vet vuln

define newline


endef
UNIT_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/deterministic-packages.txt))
PROCESS_PACKAGES := $(subst $(newline), ,$(file <tests/profiles/process-packages.txt))
HEADLESS_COMMANDS := $(subst $(newline), ,$(file <tests/profiles/headless-commands.txt))
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

headless-build:
	go build $(HEADLESS_COMMANDS)

headless-e2e:
	go test ./internal/endpoint/enrollment -run '^(TestVerifyReturnsV3ControlArtifactOutsideReleaseMetadata|TestVerifyRejectsUnknownInventoryAndExecutableSubstitution)$$' -count=1
	go test ./tests/e2e/endpoint -run '^TestEnrollmentCheckAcceptsExactRunningBundleAndRejectsChangedManifest$$' -count=1
	go test ./tests/e2e/network-source -run '^TestFiniteSourceCommandsAsBlackBoxProcesses$$' -count=1
	go test ./tests/e2e/node -run '^TestNativeDutyProcessesUseTheirExactStateAssignments$$' -count=1
	go test ./tests/e2e/service -run '^TestServiceCommandReadinessTimeoutAndCleanup$$' -count=1

headless-check: headless-build headless-e2e

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

qualification-h4-6a-two-endpoints:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-6a-two-endpoints/run-windows.ps1 -CandidateArchive "$(H4_6A_ARCHIVE)" -ArchiveSHA256 "$(H4_6A_ARCHIVE_SHA256)" -ManifestPin "$(H4_6A_MANIFEST_PIN)" -EndpointSHA256 "$(H4_6A_ENDPOINT_SHA256)" -ControlSHA256 "$(H4_6A_CONTROL_SHA256)" -Cohort "$(H4_6A_COHORT)" -Release "$(H4_6A_RELEASE)" -At "$(H4_6A_AT)" -VPS "$(H4_6A_VPS)" -SSHKey "$(H4_6A_SSH_KEY)" -User "$(H4_6A_VPS_USER)" -EvidenceOutput "$(H4_6A_EVIDENCE)"

qualification-h4-8-a11:
	C:\Windows\Sysnative\WindowsPowerShell\v1.0\powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-8-a11/invoke-windows.ps1 -SourceRevision "$(H4_8_A11_SOURCE_REVISION)" -CandidateRepository "$(H4_8_A11_CANDIDATE_REPOSITORY)" -ReleaseTag "$(H4_8_A11_RELEASE_TAG)" -CandidateArchive "$(H4_8_A11_ARCHIVE)" -ArchiveSHA256 "$(H4_8_A11_ARCHIVE_SHA256)" -ManifestPin "$(H4_8_A11_MANIFEST_PIN)" -EndpointSHA256 "$(H4_8_A11_ENDPOINT_SHA256)" -ControlSHA256 "$(H4_8_A11_CONTROL_SHA256)" -Cohort "$(H4_8_A11_COHORT)" -At "$(H4_8_A11_AT)" -VPS "$(H4_8_A11_VPS)" -SSHKey "$(H4_8_A11_SSH_KEY)" -User "$(H4_8_A11_VPS_USER)" -BasePort "$(H4_8_A11_BASE_PORT)" -RemoteImageID "$(H4_8_A11_IMAGE_ID)" -EvidenceOutput "$(H4_8_A11_EVIDENCE)" -CampaignTimeout "-timeout=125m"

prepare-h4-2-net-01a:
	sh ./tests/qualification/h4-2-net-01a/run-ubuntu.sh

prepare-h4-5-rendezvous:
	sh ./tests/qualification/h4-5-rendezvous/run-ubuntu.sh

qualification-h4-5-rendezvous:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./tests/qualification/h4-5-rendezvous/run-windows.ps1 -PrimaryVPS "$(H4_5_PRIMARY_VPS)" -PrimarySSHKey "$(H4_5_PRIMARY_SSH_KEY)" -PrimaryUser "$(H4_5_PRIMARY_USER)" -PrimaryBasePort "$(H4_5_PRIMARY_BASE_PORT)" -SecondaryVPS "$(H4_5_SECONDARY_VPS)" -SecondarySSHKey "$(H4_5_SECONDARY_SSH_KEY)" -SecondaryPasswordFile "$(H4_5_SECONDARY_PASSWORD_FILE)" -SecondaryHostKey "$(H4_5_SECONDARY_HOST_KEY)" -SecondaryUser "$(H4_5_SECONDARY_USER)" -EvidenceOutput "$(H4_5_EVIDENCE)" -OperatorHistoryBase64 "$(H4_5_OPERATOR_HISTORY_BASE64)"

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
