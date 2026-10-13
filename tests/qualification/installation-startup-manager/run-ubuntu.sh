#!/bin/sh
# Fresh manager mechanism only; it supplies no installed Startup acceptance.
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || fail 'invalid environment: root profile driver required'
[ "${1--timeout=20s}" = -timeout=20s ] || fail 'invalid environment: fixed twenty-second test bound required'
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] ||
    fail 'invalid environment: Ubuntu24.04 x86-64 required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup v2 required'
binary=/usr/lib/ardents/qualification/installation-systemd.test
unit=/run/systemd/system/ardents-endpoint.service
[ -n "${ARDENTS_STARTUP_MANAGER_TEST_SHA256-}" ] && [ -n "${ARDENTS_STARTUP_MANAGER_UNIT_SHA256-}" ] ||
    fail 'invalid environment: independent original binary/unit digests required'
[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
    fail 'invalid environment: immutable root-owned native test required'
[ -f "$unit" ] && [ ! -L "$unit" ] && [ "$(stat -c %u:%g:%a "$unit")" = 0:0:644 ] ||
    fail 'invalid environment: pinned root-owned qualification unit required'
printf '%s  %s\n' "$ARDENTS_STARTUP_MANAGER_TEST_SHA256" "$binary" | sha256sum --check --status ||
    fail 'invalid environment: original binary differs'
printf '%s  %s\n' "$ARDENTS_STARTUP_MANAGER_UNIT_SHA256" "$unit" | sha256sum --check --status ||
    fail 'invalid environment: original unit differs'
[ "$(systemctl show ardents-endpoint.service -p FragmentPath --value)" = "$unit" ] &&
    [ -z "$(systemctl show ardents-endpoint.service -p DropInPaths --value)" ] &&
    [ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] ||
    fail 'invalid environment: fixed Endpoint unit is not fresh and exact'
systemctl start ardents-endpoint.service
invocation=$(systemctl show ardents-endpoint.service -p InvocationID --value)
[ -n "$invocation" ] || fail 'native test has no original invocation'
attempt=0
while :; do
    state=$(systemctl show ardents-endpoint.service -p ActiveState --value)
    case "$state" in
        inactive|failed) break ;;
        active|activating|deactivating) ;;
        *) fail 'native test has unknown manager state' ;;
    esac
    attempt=$((attempt + 1))
    [ "$attempt" -le 300 ] || fail 'native manager mechanism exceeded its bound'
    sleep 0.1
done
journal=$(journalctl --no-pager -o cat "_SYSTEMD_INVOCATION_ID=$invocation")
printf '%s\n' "$journal"
[ "$state" = inactive ] && [ "$(systemctl show ardents-endpoint.service -p MainPID --value)" = 0 ] &&
    [ "$(systemctl show ardents-endpoint.service -p Result --value)" = success ] &&
    [ "$(systemctl show ardents-endpoint.service -p ExecMainStatus --value)" = 0 ] ||
    fail 'native manager observation failed or did not join'
[ "$(printf '%s\n' "$journal" | grep -c '^--- PASS: TestInstallationNativeFreshStartupProperties (' || true)" = 1 ] ||
    fail 'native manager mechanism lacks exact executed evidence'
printf 'profile=startup-manager-native-linux; result=passed; startup-acceptance=not-established\n'
