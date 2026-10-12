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
: "${ARDENTS_NODE_NATIVE_TEST:?native command test binary is required}"
: "${ARDENTS_NODE_NATIVE_TEST_SHA256:?independent test digest is required}"
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
verify "$ARDENTS_NODE_NATIVE_TEST" "$ARDENTS_NODE_NATIVE_TEST_SHA256"
verify "$ARDENTS_NODE_RESOURCE_TEST" "$ARDENTS_NODE_RESOURCE_TEST_SHA256"

# Discovery is part of the selected Issuer profile; zero-test runs fail.
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerBootstrapOrdinaryAndFreshSpendBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerOriginalAuthorityAndJoinBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerOriginalFailureBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerPartialStartupRefusalsBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerUnavailableCompletionRetainsOriginalProducerBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerOperationJoinAndExactRetryBothCarriers$' >/dev/null
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuerStorageUncertaintyBothCarriers$' >/dev/null
"$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNativeIssuer' | grep '^TestOriginalNativeIssuerProcessProfile$' >/dev/null
"$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNativeIssuer' | grep '^TestOriginalNativeIssuerEvidenceLossRetainsActualCloseFailure$' >/dev/null
"$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNativeIssuer' | grep '^TestOriginalNativeIssuerManagedMemoryPressureRetires$' >/dev/null
unit=ardents-issuer-resource-native-$$.service
systemd-run --quiet --wait --pipe --collect --unit="$unit" \
    --property=CPUQuota=100% --property=CPUQuotaPeriodSec=100ms \
    --property=MemoryMax=1G --property=MemoryHigh=768M --property=TasksMax=256 \
    --property=LimitNOFILE=1024 --property=RuntimeMaxSec=120s \
    --setenv=GOMAXPROCS=1 --setenv=GOMEMLIMIT=512MiB \
    -- "$ARDENTS_NODE_RESOURCE_TEST" -test.v=true -test.run='^TestOriginalNativeIssuer' -test.timeout=90s

# Independent lifecycle and issuing-operation workloads each retain the same
# registered process timeout. Both are mandatory and execute sequentially under
# the identical native envelope; no failed case is omitted or given more time.
# Exact discovery rejects any added, missing or duplicate undispatched test.
expected='TestNativeIssuerBootstrapOrdinaryAndFreshSpendBothCarriers
TestNativeIssuerOperationJoinAndExactRetryBothCarriers
TestNativeIssuerOriginalAuthorityAndJoinBothCarriers
TestNativeIssuerOriginalFailureBothCarriers
TestNativeIssuerPartialStartupRefusalsBothCarriers
TestNativeIssuerStorageUncertaintyBothCarriers
TestNativeIssuerUnavailableCompletionRetainsOriginalProducerBothCarriers'
discovered=$("$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeIssuer' | grep '^TestNativeIssuer' | sort)
test "$discovered" = "$expected" || { echo 'Issuer workload discovery differs' >&2; exit 2; }
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeIssuer(BootstrapOrdinaryAndFreshSpendBothCarriers|OriginalAuthorityAndJoinBothCarriers|OriginalFailureBothCarriers|PartialStartupRefusalsBothCarriers|UnavailableCompletionRetainsOriginalProducerBothCarriers)$' -test.timeout=3m
"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeIssuer(OperationJoinAndExactRetryBothCarriers|StorageUncertaintyBothCarriers)$' -test.timeout=3m
