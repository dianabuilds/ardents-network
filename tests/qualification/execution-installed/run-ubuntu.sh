#!/bin/sh
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
timeout_argument=${1--timeout=4m}
[ "$timeout_argument" = -timeout=4m ] || fail 'invalid environment: selected four-minute bound required'
timeout=${timeout_argument#-timeout=}
binary=${ARDENTS_EXECUTION_START_TEST-}
binary_pin=${ARDENTS_EXECUTION_START_TEST_SHA256-}
resources=${ARDENTS_EXECUTION_START_RESOURCES-}
inventory_pin=${ARDENTS_EXECUTION_START_INVENTORY_SHA256-}
base=${ARDENTS_EXECUTION_START_ROOT-}
log=${ARDENTS_EXECUTION_START_LOG-}
[ "$(id -u)" = 0 ] || fail 'invalid environment: root driver required'
for program in systemctl journalctl sha256sum stat uname grep getent mkdir; do
    command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] ||
    fail 'invalid environment: Ubuntu24.04 x86-64 required'
[ "$(systemctl show -p Version --value)" = 255.4-1ubuntu8.14 ] ||
    fail 'invalid environment: independently selected systemd255 build required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup-v2 required'
for value in "$binary" "$resources" "$base" "$log"; do
    case "$value" in /*) ;; *) fail 'invalid environment: absolute paths required' ;; esac
done
[ -n "$binary_pin" ] && [ -n "$inventory_pin" ] || fail 'invalid environment: independent pins required'
[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
    fail 'invalid environment: immutable root-owned test binary required'
[ -f "$resources/inventory.json" ] && [ ! -L "$resources/inventory.json" ] &&
    [ "$(stat -c %u:%g:%a "$resources/inventory.json")" = 0:0:444 ] ||
    fail 'invalid environment: immutable root-owned inventory required'
printf '%s  %s\n' "$binary_pin" "$binary" | sha256sum --check --status || fail 'invalid environment: test pin mismatch'
printf '%s  %s\n' "$inventory_pin" "$resources/inventory.json" | sha256sum --check --status || fail 'invalid environment: inventory pin mismatch'
for account in ardents-endpoint ardents-text; do
    if getent passwd "$account" >/dev/null || getent group "$account" >/dev/null; then
        fail 'invalid environment: Ardents account or group already exists'
    fi
done
for path in /etc/systemd/system/ardents-endpoint.service /run/systemd/system/ardents-endpoint.service \
    /etc/systemd/system/ardents-text-reader@.service /etc/systemd/system/ardents-text-publisher@.service \
    /etc/systemd/system/ardents-text-reader.socket /etc/systemd/system/ardents-text-publisher.socket \
    /usr/lib/ardents/text-worker-root /etc/ardents/text-worker-artifact.json \
    /usr/share/polkit-1/rules.d/50-ardents-text.rules /usr/lib/sysusers.d/ardents-text.conf /run/ardents-text; do
    [ ! -e "$path" ] && [ ! -L "$path" ] || fail 'invalid environment: fixed resource already exists'
done
[ ! -e "$base" ] && [ ! -L "$base" ] && [ ! -e "$log" ] && [ ! -L "$log" ] ||
    fail 'invalid environment: fresh root and retained new log required'
parent=${base%/*}
[ "$(stat -c %u:%g:%a "$parent")" = 0:0:755 ] && [ ! -L "$parent" ] ||
    fail 'invalid environment: direct trusted parent required'
umask 077
mkdir "$base"
chmod 0755 "$base"
mkdir "$base/private"
export TMPDIR="$base/private"
export ARDENTS_EXECUTION_START_ROOT="$base" ARDENTS_EXECUTION_START_RESOURCES="$resources"
set +e
"$binary" -test.run='^TestInstalledExecutionStartup$' -test.v "-test.timeout=$timeout" > "$log" 2>&1
result=$?
set -e
cat "$log"
[ "$result" = 0 ] || fail 'actual installed startup failed; retain this log and native residue'
[ "$(grep -c '^--- PASS: TestInstalledExecutionStartup (' "$log" || true)" = 1 ] ||
    fail 'actual installed startup lacks exact executed evidence'
if grep -q -- '--- SKIP:' "$log"; then fail 'passing skip is forbidden'; fi
printf '%s\n' 'execution-installed=passed; service-and-whole-host-qualification=not-established'
