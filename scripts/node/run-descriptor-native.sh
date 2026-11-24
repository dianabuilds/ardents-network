#!/bin/sh
set -eu
test "$#" = 1 && test "$1" = '-timeout=3m' || { echo 'Descriptor native profile requires its registered timeout' >&2; exit 2; }
test "$(uname -s)" = Linux || { echo 'Descriptor native profile requires Linux' >&2; exit 2; }
test "$(id -u)" = 0 || { echo 'Descriptor native profile requires a root driver' >&2; exit 2; }
command -v systemd-run >/dev/null
command -v systemctl >/dev/null
command -v taskset >/dev/null
test -r /sys/fs/cgroup/cgroup.controllers

: "${ARDENTS_NODE_NATIVE_COMMAND:?canonical Node command is required}"
: "${ARDENTS_NODE_NATIVE_COMMAND_SHA256:?independent command digest is required}"
: "${ARDENTS_NODE_NATIVE_TEST:?native command test binary is required}"
: "${ARDENTS_NODE_NATIVE_TEST_SHA256:?independent test digest is required}"
: "${ARDENTS_NODE_OWNER_TEST:?native Node owner test binary is required}"
: "${ARDENTS_NODE_OWNER_TEST_SHA256:?independent Node owner test digest is required}"
: "${ARDENTS_NODE_E2E_TEST:?Node e2e test binary is required}"
: "${ARDENTS_NODE_E2E_TEST_SHA256:?independent Node e2e digest is required}"
: "${ARDENTS_NODE_RESOURCE_TEST:?native resource test binary is required}"
: "${ARDENTS_NODE_RESOURCE_TEST_SHA256:?independent resource digest is required}"
verify() {
    case "$1" in /*) ;; *) echo 'native artifact path must be absolute' >&2; exit 2;; esac
    test -f "$1" && test -x "$1"
    test "${#2}" = 64
    actual=$(sha256sum "$1")
    test "${actual%% *}" = "$2" || { echo 'native artifact digest differs' >&2; exit 2; }
}
verify "$ARDENTS_NODE_NATIVE_COMMAND" "$ARDENTS_NODE_NATIVE_COMMAND_SHA256"
verify "$ARDENTS_NODE_NATIVE_TEST" "$ARDENTS_NODE_NATIVE_TEST_SHA256"
verify "$ARDENTS_NODE_OWNER_TEST" "$ARDENTS_NODE_OWNER_TEST_SHA256"
"$ARDENTS_NODE_OWNER_TEST" -test.list='^TestNativeNodeDescriptorOperationProcess$' | grep -Fx TestNativeNodeDescriptorOperationProcess >/dev/null
verify "$ARDENTS_NODE_E2E_TEST" "$ARDENTS_NODE_E2E_TEST_SHA256"
verify "$ARDENTS_NODE_RESOURCE_TEST" "$ARDENTS_NODE_RESOURCE_TEST_SHA256"

# The root driver and its immutable artifacts stay outside every measured leaf.
# Exact discovery rejects zero, missing, extra or duplicate undispatched tests.
expected_resources='TestOriginalNativeDescriptorEvidenceLossRetainsActualCloseFailure
TestOriginalNativeDescriptorManagedMemoryPressureRetires
TestOriginalNativeDescriptorProcessProfile'
discovered_resources=$("$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNativeDescriptor' | grep '^TestOriginalNativeDescriptor' | sort)
test "$discovered_resources" = "$expected_resources" || { echo 'Descriptor resource discovery differs' >&2; exit 2; }
expected='TestNativeDescriptorOperationBoundariesTCP'
discovered=$("$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeDescriptor' | grep '^TestNativeDescriptor' | sort)
test "$discovered" = "$expected" || { echo 'Descriptor workload discovery differs' >&2; exit 2; }

"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorOriginalCommandFailureTCP$' | grep -Fx TestNativeDescriptorOriginalCommandFailureTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorPublishLookupAndReopenTCP$' | grep -Fx TestNativeDescriptorPublishLookupAndReopenTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorUndercoverageBeforeSpendTCP$' | grep -Fx TestNativeDescriptorUndercoverageBeforeSpendTCP >/dev/null

unit=ardents-descriptor-resource-native-$$.service
systemd-run --quiet --wait --pipe --collect --unit="$unit" \
    --property=CPUQuota=100% --property=CPUQuotaPeriodSec=100ms \
    --property=MemoryMax=1G --property=MemoryHigh=768M --property=TasksMax=256 \
    --property=LimitNOFILE=1024 --property=RuntimeMaxSec=120s \
    --setenv=GOMAXPROCS=1 --setenv=GOMEMLIMIT=512MiB \
    -- "$ARDENTS_NODE_RESOURCE_TEST" -test.v=true -test.run='^TestOriginalNativeDescriptor' -test.timeout=90s
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNativeDescriptor(PublishLookupAndReopenTCP|OriginalCommandFailureTCP)$' -test.timeout=3m
"$ARDENTS_NODE_OWNER_TEST" -test.list='^TestNativeDescriptorOriginalAuthorityAndJoinTCP$' | grep -Fx TestNativeDescriptorOriginalAuthorityAndJoinTCP >/dev/null
"$ARDENTS_NODE_OWNER_TEST" -test.v=true -test.run='^TestNativeDescriptorOriginalAuthorityAndJoinTCP$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorOperationBoundariesTCP$' -test.timeout=3m
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP$' | grep -Fx TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP$' -test.timeout=3m
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorPartialStartupRefusalsTCP$' | grep -Fx TestNativeDescriptorPartialStartupRefusalsTCP >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNativeDescriptorPartialStartupRefusalsTCP$' -test.timeout=3m
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNativeDescriptorUndercoverageBeforeSpendTCP$' -test.timeout=3m
"$ARDENTS_NODE_E2E_TEST" -test.list='^TestNativeDescriptorRetainedLifecycle$' | grep -Fx 'TestNativeDescriptorRetainedLifecycle' >/dev/null
"$ARDENTS_NODE_E2E_TEST" -test.v=true -test.run='^TestNativeDescriptorRetainedLifecycle$' -test.timeout=3m
