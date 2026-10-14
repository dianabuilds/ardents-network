#!/bin/sh
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "${1-}" = -timeout=8m ] && [ "$#" = 1 ] || fail 'invalid environment: selected eight-minute controller bound required'
[ "$(id -u)" = 0 ] || fail 'invalid environment: root controller required'
for program in nsenter sha256sum stat uname mkdir grep cat; do
    command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
[ "$(uname -s):$(uname -m)" = Linux:x86_64 ] || fail 'invalid environment: Linux x86-64 controller required'
binary=${ARDENTS_EXECUTION_START_TEST-}
resources=${ARDENTS_EXECUTION_START_RESOURCES-}
base=${ARDENTS_EXECUTION_START_ROOT-}
log=${ARDENTS_EXECUTION_START_LOG-}
for value in "$binary" "$resources" "$base" "$log"; do
    case "$value" in /*) ;; *) fail 'invalid environment: absolute fixture paths required' ;; esac
done
[ -n "${ARDENTS_EXECUTION_START_TEST_SHA256-}" ] && [ -n "${ARDENTS_EXECUTION_START_INVENTORY_SHA256-}" ] ||
    fail 'invalid environment: independent artifact pins required'
[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
    fail 'invalid environment: immutable root-owned test binary required'
[ -f "$resources/inventory.json" ] && [ ! -L "$resources/inventory.json" ] &&
    [ "$(stat -c %u:%g:%a "$resources/inventory.json")" = 0:0:444 ] ||
    fail 'invalid environment: immutable root-owned inventory required'
printf '%s  %s\n' "$ARDENTS_EXECUTION_START_TEST_SHA256" "$binary" | sha256sum --check --status || fail 'invalid environment: test pin mismatch'
printf '%s  %s\n' "$ARDENTS_EXECUTION_START_INVENTORY_SHA256" "$resources/inventory.json" | sha256sum --check --status || fail 'invalid environment: inventory pin mismatch'
case "${ARDENTS_EXECUTION_START_CARRIER-}" in
    ardents-carrier-tcp-tls-v2|ardents-carrier-quic-v2) ;;
    *) fail 'invalid environment: independently selected Carrier required' ;;
esac
[ -n "${ARDENTS_EXECUTION_PAIR_MANAGERS-}" ] || fail 'invalid environment: two original manager PIDs required'
[ ! -e "$base" ] && [ ! -L "$base" ] && [ ! -e "$log" ] && [ ! -L "$log" ] ||
    fail 'invalid environment: fresh controller root and retained new log required'
parent=${base%/*}
[ "$(stat -c %u:%g:%a "$parent")" = 0:0:755 ] && [ ! -L "$parent" ] || fail 'invalid environment: trusted parent required'
umask 077
mkdir "$base"
export TMPDIR="$base"
set +e
"$binary" '-test.run=^TestInstalledServicePair$' -test.v -test.timeout=8m > "$log" 2>&1
result=$?
set -e
cat "$log"
[ "$result" = 0 ] || fail 'installed pair failed; retain controller and both participant logs'
[ "$(grep -c '^--- PASS: TestInstalledServicePair (' "$log" || true)" = 1 ] || fail 'installed pair lacks exact executed evidence'
if grep -q -- '--- SKIP:' "$log"; then fail 'passing skip is forbidden'; fi
printf '%s\n' 'execution-installed-pair=passed; full-interruption-and-whole-host-qualification=not-established'
