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
verify "$ARDENTS_NODE_RESOURCE_TEST" "$ARDENTS_NODE_RESOURCE_TEST_SHA256"

# The root driver and its immutable artifacts stay outside every measured leaf.
# Exact discovery rejects zero, missing, extra or duplicate undispatched tests.
expected_resources='TestOriginalNativeDescriptorEvidenceLossRetainsActualCloseFailure
TestOriginalNativeDescriptorManagedMemoryPressureRetires
TestOriginalNativeDescriptorProcessProfile'
discovered_resources=$("$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNativeDescriptor' | grep '^TestOriginalNativeDescriptor' | sort)
test "$discovered_resources" = "$expected_resources" || { echo 'Descriptor resource discovery differs' >&2; exit 2; }
expected='TestNativeDescriptorOperationBoundariesBothCarriers
TestNativeDescriptorOriginalAuthorityAndJoinBothCarriers
TestNativeDescriptorOriginalCommandFailureBothCarriers
TestNativeDescriptorPartialStartupRefusalsBothCarriers
TestNativeDescriptorPublishLookupAndReopenBothCarriers
TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerBothCarriers
TestNativeDescriptorUndercoverageBeforeSpendBothCarriers'
discovered=$("$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeDescriptor' | grep '^TestNativeDescriptor' | sort)
test "$discovered" = "$expected" || { echo 'Descriptor workload discovery differs' >&2; exit 2; }

unit=ardents-descriptor-resource-native-$$.service
systemd-run --quiet --wait --pipe --collect --unit="$unit" \
    --property=CPUQuota=100% --property=CPUQuotaPeriodSec=100ms \
    --property=MemoryMax=1G --property=MemoryHigh=768M --property=TasksMax=256 \
    --property=LimitNOFILE=1024 --property=RuntimeMaxSec=120s \
    --setenv=GOMAXPROCS=1 --setenv=GOMEMLIMIT=512MiB \
    -- "$ARDENTS_NODE_RESOURCE_TEST" -test.v=true -test.run='^TestOriginalNativeDescriptor' -test.timeout=90s
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptor(PublishLookupAndReopenBothCarriers|OriginalCommandFailureBothCarriers)$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorOriginalAuthorityAndJoinBothCarriers$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorOperationBoundariesBothCarriers$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerBothCarriers$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorPartialStartupRefusalsBothCarriers$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeDescriptorUndercoverageBeforeSpendBothCarriers$' -test.timeout=3m
