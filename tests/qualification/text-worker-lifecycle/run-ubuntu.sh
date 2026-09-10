#!/bin/sh
# Runs only on the dedicated, preinstalled qualification Endpoint service.
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || fail 'invalid environment: root installed-profile driver required'
maximum_polls=1500
case "${1-lifecycle}" in
    lifecycle) test_root=TestInstalledTextWorkerLifecycle ;;
    network) test_root=TestInstalledTextWorkersReadTargetThroughJoinedNetwork; maximum_polls=10000 ;;
    tree)
        test_root=TestInstalledTextWorkerHostileTree
        worker=/usr/lib/ardents/text-worker-root/ardents-text
        [ -n "${ARDENTS_TEXT_HOSTILE_WORKER_SHA256-}" ] || fail 'hostile worker independent digest required'
        [ -f "$worker" ] && [ ! -L "$worker" ] && [ "$(stat -c %u:%g:%a "$worker")" = 0:0:555 ] ||
            fail 'invalid environment: pinned hostile artifact required'
        printf '%s  %s\n' "$ARDENTS_TEXT_HOSTILE_WORKER_SHA256" "$worker" | sha256sum --check --status ||
            fail 'invalid environment: hostile artifact differs from declared candidate'
        ;;
    *) fail 'unknown installed text-worker profile' ;;
esac
for program in systemctl journalctl sha256sum stat uname sleep grep; do
    command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] ||
    fail 'invalid environment: Ubuntu 24.04 x86-64 required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup v2 required'
binary=/usr/lib/ardents/qualification/endpoint.test
unit=/run/systemd/system/ardents-endpoint.service
[ -n "$ARDENTS_TEXT_LIFECYCLE_SHA256" ] && [ -n "$ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256" ] ||
    fail 'invalid environment: independent binary and unit digests required'
[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
    fail 'invalid environment: immutable root-owned test binary required'
[ -f "$unit" ] && [ ! -L "$unit" ] && [ "$(stat -c %u:%g:%a "$unit")" = 0:0:644 ] ||
    fail 'invalid environment: root-owned temporary qualification unit required'
printf '%s  %s\n' "$ARDENTS_TEXT_LIFECYCLE_SHA256" "$binary" | sha256sum --check --status ||
    fail 'invalid environment: test binary differs from declared candidate'
printf '%s  %s\n' "$ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256" "$unit" | sha256sum --check --status ||
    fail 'invalid environment: test unit differs from declared candidate'
[ "$(systemctl show ardents-endpoint.service -p FragmentPath --value)" = "$unit" ] ||
    fail 'invalid environment: a different Endpoint unit is loaded'
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] ||
    fail 'invalid environment: Endpoint is not fresh and inactive'
[ -z "$(systemctl show ardents-endpoint.service -p DropInPaths --value)" ] ||
    fail 'invalid environment: qualification unit has drop-ins'
systemctl start ardents-endpoint.service
invocation=$(systemctl show ardents-endpoint.service -p InvocationID --value)
[ -n "$invocation" ] || fail 'installed test has no exact running invocation'
attempt=0
while :; do
    state=$(systemctl show ardents-endpoint.service -p ActiveState --value)
    case "$state" in
        inactive|failed) break ;;
        active|activating|deactivating) ;;
        *) fail 'installed lifecycle has an unknown service state' ;;
    esac
    attempt=$((attempt + 1))
    [ "$attempt" -le "$maximum_polls" ] || fail 'installed lifecycle exceeded its declared bound'
    sleep 0.1
done
journal=$(journalctl --no-pager -o cat "_SYSTEMD_INVOCATION_ID=$invocation")
printf '%s\n' "$journal"
[ "$state" = inactive ] && [ "$(systemctl show ardents-endpoint.service -p MainPID --value)" = 0 ] &&
    [ "$(systemctl show ardents-endpoint.service -p Result --value)" = success ] &&
    [ "$(systemctl show ardents-endpoint.service -p ExecMainStatus --value)" = 0 ] ||
    fail 'installed text-worker lifecycle failed or did not terminate'
set -- "$test_root" "$test_root/reader" "$test_root/publisher"
if [ "$test_root" = TestInstalledTextWorkersReadTargetThroughJoinedNetwork ]; then
    set -- "$test_root"
    for carrier in ardents-carrier-tcp-tls-v2 ardents-carrier-quic-v2; do
        set -- "$@" "$test_root/$carrier"
        for size in empty reference maximum refresh; do
            set -- "$@" "$test_root/$carrier/$size"
        done
    done
fi
for test do
    [ "$(printf '%s\n' "$journal" | grep -c "^[[:space:]]*--- PASS: $test (" || true)" = 1 ] ||
        fail 'installed lifecycle lacks exact executed test evidence'
done
printf 'installed-profile=%s; result=passed; whole-host-qualification=not-established\n' "$test_root"
