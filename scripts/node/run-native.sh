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

# Verify discovery before executing; a selected profile can never pass zero tests.
"$ARDENTS_NODE_NATIVE_TEST" -test.list='^TestNativeNode' | grep '^TestNativeNodeForwardingBothCarriers$' >/dev/null
"$ARDENTS_NODE_RESOURCE_TEST" -test.list='^TestOriginalNative' | grep '^TestOriginalNativeProcessProfile$' >/dev/null
unit=ardents-forwarding-resource-native-$$.service
systemd-run --quiet --wait --pipe --collect --unit="$unit" \
    --property=CPUQuota=100% --property=CPUQuotaPeriodSec=100ms \
    --property=MemoryMax=1G --property=MemoryHigh=768M --property=TasksMax=256 \
    --property=LimitNOFILE=1024 --property=RuntimeMaxSec=120s \
    --setenv=GOMAXPROCS=1 --setenv=GOMEMLIMIT=512MiB \
    -- "$ARDENTS_NODE_RESOURCE_TEST" -test.v=true -test.timeout=90s

"$ARDENTS_NODE_NATIVE_TEST" -test.v=true -test.run='^TestNativeNode' -test.timeout=3m
