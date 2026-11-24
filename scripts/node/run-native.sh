#!/bin/sh
set -eu
test "$#" = 1 && test "$1" = '-timeout=3m' || { echo 'Node native profile requires its registered timeout' >&2; exit 2; }

# The actual canonical binaries are built from the checked source separately.
# This root driver owns only transient services; it is outside Node cgroups.
test "$(uname -s)" = Linux || { echo 'Node native profile requires Linux' >&2; exit 2; }
test "$(id -u)" = 0 || { echo 'Node native profile requires a root driver' >&2; exit 2; }
command -v systemd-run >/dev/null
command -v systemctl >/dev/null
test -r /sys/fs/cgroup/cgroup.controllers

: "${ARDENTS_NODE_NATIVE_COMMAND:?canonical Node command is required}"
: "${ARDENTS_NODE_NATIVE_COMMAND_SHA256:?independent command digest is required}"
: "${ARDENTS_NODE_OWNER_TEST:?native Node owner test binary is required}"
: "${ARDENTS_NODE_OWNER_TEST_SHA256:?independent Node owner test digest is required}"
: "${ARDENTS_NODE_E2E_TEST:?Node e2e test binary is required}"
: "${ARDENTS_NODE_E2E_TEST_SHA256:?independent Node e2e digest is required}"
: "${ARDENTS_NODE_RESOURCE_TEST:?native resource test binary is required}"
: "${ARDENTS_NODE_RESOURCE_TEST_SHA256:?independent resource test digest is required}"

verify() {
    case "$1" in /*) ;; *) echo 'native artifact path must be absolute' >&2; exit 2;; esac
    test -f "$1" && test -x "$1"
    test "${#2}" = 64
    actual=$(sha256sum "$1")
    test "${actual%% *}" = "$2" || { echo 'native artifact digest differs' >&2; exit 2; }
}
verify "$ARDENTS_NODE_NATIVE_COMMAND" "$ARDENTS_NODE_NATIVE_COMMAND_SHA256"
verify "$ARDENTS_NODE_OWNER_TEST" "$ARDENTS_NODE_OWNER_TEST_SHA256"
verify "$ARDENTS_NODE_E2E_TEST" "$ARDENTS_NODE_E2E_TEST_SHA256"
verify "$ARDENTS_NODE_RESOURCE_TEST" "$ARDENTS_NODE_RESOURCE_TEST_SHA256"

# Verify discovery before executing; a selected profile can never pass zero tests.
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeNodeForwardingTCP$' | grep -Fx TestNativeNodeForwardingTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeNodeProvisionedCertificateTCP$' | grep -Fx TestNativeNodeProvisionedCertificateTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeNodePartialStartupRefusalsTCP$' | grep -Fx TestNativeNodePartialStartupRefusalsTCP >/dev/null
"$ARDENTS_NODE_OWNER_TEST" -test.list='^TestNativeNode' | grep '^TestNativeNodeParentRefillAndReopenTCP$' >/dev/null
"$ARDENTS_NODE_OWNER_TEST" -test.list='^TestNativeNode' | grep '^TestNativeNodeOriginalAuthorityAndJoinTCP$' >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeNodeUnavailableCompletionRetainsOriginalProducerTCP$' | grep -Fx TestNativeNodeUnavailableCompletionRetainsOriginalProducerTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeNodeRepairRefusalKeepsBoundedReport$' | grep -Fx TestNativeNodeRepairRefusalKeepsBoundedReport >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeForwardingRetainedLifecycle$' | grep '^TestNativeForwardingRetainedLifecycle$' >/dev/null
"$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNative' | grep '^TestOriginalNativeProcessProfile$' >/dev/null
expected_resources='TestMeasuredBriefDevelopmentStallsPreserveAdmission
TestNativeMemoryEventsSinglePassEquivalence
TestNativePressureAndUnavailableIntervalDenyAdmission
TestOriginalNativeEvidenceLossRetainsActualCloseFailure
TestOriginalNativeManagedMemoryPressureRetires
TestOriginalNativePlacementAndCallerRetire
TestOriginalNativeProcessProfile
TestProfileAndOriginalCallerRefuseBeforeNativeEffects
TestRequiredNativeCounterIsNotAnUnknownZero
TestRequiredPressureRejectsMissingNonfiniteAndAmbiguous'
# Source has its own exact discovery/execution in run-source-native.sh, just as
# the other receiving profiles below. Copied failure projections are exercised
# by the portable deterministic profile.
discovered_resources=$("$ARDENTS_NODE_RESOURCE_TEST" -test.list='^Test' | grep '^Test' | grep -Ev '^Test(OriginalNative(Source|Issuer|Descriptor|Introduction|Join)|Failure(CopiesDetachedFactsThroughWrappedErrors|MissingObservationStaysUnavailable|IgnoresForeignErrors)$)' | sort)
test "$discovered_resources" = "$expected_resources" || { echo 'Forwarding resource discovery differs' >&2; exit 2; }
unit=ardents-forwarding-resource-native-$$.service
systemd-run --quiet --wait --pipe --collect --unit="$unit" \
    --property=CPUQuota=100% --property=CPUQuotaPeriodSec=100ms \
    --property=MemoryMax=1G --property=MemoryHigh=768M --property=TasksMax=256 \
    --property=LimitNOFILE=1024 --property=RuntimeMaxSec=120s \
    --setenv=GOMAXPROCS=1 --setenv=GOMEMLIMIT=512MiB \
    -- "$ARDENTS_NODE_RESOURCE_TEST" -test.v=true -test.run='^Test(MeasuredBriefDevelopmentStallsPreserveAdmission|NativeMemoryEventsSinglePassEquivalence|ProfileAndOriginalCallerRefuseBeforeNativeEffects|NativePressureAndUnavailableIntervalDenyAdmission|RequiredNativeCounterIsNotAnUnknownZero|RequiredPressureRejectsMissingNonfiniteAndAmbiguous|OriginalNative(ProcessProfile|EvidenceLossRetainsActualCloseFailure|ManagedMemoryPressureRetires|PlacementAndCallerRetire))$' -test.timeout=90s

"$ARDENTS_NODE_OWNER_TEST" -test.v=true -test.run='^TestNativeNodeParentRefillAndReopenTCP$' -test.timeout=3m
"$ARDENTS_NODE_OWNER_TEST" -test.v=true -test.run='^TestNativeNodeOriginalAuthorityAndJoinTCP$' -test.timeout=3m
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNative(ForwardingRetainedLifecycle|NodePartialStartupRefusalsTCP|NodeUnavailableCompletionRetainsOriginalProducerTCP|NodeRepairRefusalKeepsBoundedReport|NodeForwardingTCP|NodeProvisionedCertificateTCP)$' -test.timeout=3m
