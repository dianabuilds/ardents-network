#!/bin/sh
# Runs the root command journey only on the selected installed Ubuntu host.
set -eu

fail() { printf '%s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || fail 'invalid environment: root installed-command driver required'
for program in systemctl sha256sum stat uname grep timeout awk cmp mktemp rm cat; do
	command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] ||
	fail 'invalid environment: Ubuntu 24.04 x86-64 required'
[ "$(systemctl --version | awk 'NR == 1 {print $2}')" = 255 ] || fail 'invalid environment: systemd 255 required'
[ "$(stat -fc %T /sys/fs/cgroup)" = cgroup2fs ] || fail 'invalid environment: cgroup v2 required'

binary=/usr/lib/ardents/qualification/closed-text-commands.test
unit=/run/systemd/system/ardents-endpoint.service
command_root=/usr/lib/ardents/qualification/commands
worker=/usr/lib/ardents/text-worker-root/ardents-text
[ -n "${ARDENTS_TEXT_COMMAND_TEST_SHA256-}" ] && [ -n "${ARDENTS_TEXT_COMMAND_UNIT_SHA256-}" ] &&
	[ -n "${ARDENTS_TEXT_COMMAND_ARDENTS_SHA256-}" ] && [ -n "${ARDENTS_TEXT_COMMAND_CUSTODY_SHA256-}" ] &&
	[ -n "${ARDENTS_TEXT_COMMAND_NODE_SHA256-}" ] && [ -n "${ARDENTS_TEXT_COMMAND_CONTROL_SHA256-}" ] &&
	[ -n "${ARDENTS_TEXT_COMMAND_TEXT_SHA256-}" ] && [ -n "${ARDENTS_TEXT_COMMAND_WORKER_SHA256-}" ] ||
	fail 'invalid environment: independent command candidate digests required'

[ -f "$binary" ] && [ ! -L "$binary" ] && [ "$(stat -c %u:%g:%a "$binary")" = 0:0:555 ] ||
	fail 'invalid environment: immutable root-owned command test binary required'
[ -f "$unit" ] && [ ! -L "$unit" ] && [ "$(stat -c %u:%g:%a "$unit")" = 0:0:644 ] ||
	fail 'invalid environment: root-owned temporary qualification unit required'
printf '%s  %s\n' "$ARDENTS_TEXT_COMMAND_TEST_SHA256" "$binary" | sha256sum --check --status ||
	fail 'invalid environment: command test binary differs from declared candidate'
printf '%s  %s\n' "$ARDENTS_TEXT_COMMAND_UNIT_SHA256" "$unit" | sha256sum --check --status ||
	fail 'invalid environment: command unit differs from declared candidate'

check_command() {
	name=$1
	digest=$2
	path="$command_root/$name"
	[ -f "$path" ] && [ ! -L "$path" ] && [ "$(stat -c %u:%g:%a "$path")" = 0:0:555 ] ||
		fail "invalid environment: pinned $name command required"
	printf '%s  %s\n' "$digest" "$path" | sha256sum --check --status ||
		fail "invalid environment: $name command differs from declared candidate"
}
check_command ardents "$ARDENTS_TEXT_COMMAND_ARDENTS_SHA256"
check_command ardents-custody "$ARDENTS_TEXT_COMMAND_CUSTODY_SHA256"
check_command ardents-node "$ARDENTS_TEXT_COMMAND_NODE_SHA256"
check_command ardents-control "$ARDENTS_TEXT_COMMAND_CONTROL_SHA256"
check_command ardents-text "$ARDENTS_TEXT_COMMAND_TEXT_SHA256"
[ -f "$worker" ] && [ ! -L "$worker" ] && [ "$(stat -c %u:%g:%a "$worker")" = 0:0:555 ] ||
	fail 'invalid environment: pinned ordinary worker required'
printf '%s  %s\n' "$ARDENTS_TEXT_COMMAND_WORKER_SHA256" "$worker" | sha256sum --check --status ||
	fail 'invalid environment: worker differs from declared candidate'
cmp -s "$command_root/ardents-text" "$worker" || fail 'invalid environment: UI and worker do not share the candidate artifact'

[ "$(systemctl show ardents-endpoint.service -p FragmentPath --value)" = "$unit" ] ||
	fail 'invalid environment: a different Endpoint unit is loaded'
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] ||
	fail 'invalid environment: Endpoint is not fresh and inactive'
[ -z "$(systemctl show ardents-endpoint.service -p DropInPaths --value)" ] ||
	fail 'invalid environment: qualification unit has drop-ins'

run_log=$(mktemp /var/tmp/ardents-text-command-network.XXXXXX) || fail 'invalid environment: command evidence log unavailable'
trap 'rm -f "$run_log"' EXIT HUP INT TERM
if ! ARDENTS_TEXT_COMMAND_QUALIFICATION=1 ARDENTS_E2E_COMMAND_ROOT="$command_root" \
	timeout --signal=TERM --kill-after=30s 2100s "$binary" -test.run='^TestInstalledClosedTextCommandsThroughNodeProcesses$' -test.v -test.timeout=34m >"$run_log" 2>&1; then
	cat "$run_log"
	fail 'installed command journey test failed'
fi
test_output=$(cat "$run_log")
printf '%s\n' "$test_output"
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] &&
	[ "$(systemctl show ardents-endpoint.service -p MainPID --value)" = 0 ] ||
	fail 'installed command journey retained the temporary Endpoint'

root=TestInstalledClosedTextCommandsThroughNodeProcesses
[ "$(printf '%s\n' "$test_output" | grep -c "^[[:space:]]*--- PASS: $root (" || true)" = 1 ] ||
	fail 'installed command journey lacks root test evidence'
for carrier in ardents-carrier-tcp-tls-v2 ardents-carrier-quic-v2; do
	[ "$(printf '%s\n' "$test_output" | grep -c "^[[:space:]]*--- PASS: $root/$carrier (" || true)" = 1 ] ||
		fail 'installed command journey lacks exact Carrier evidence'
	for size in empty 64KiB 4MiB; do
		[ "$(printf '%s\n' "$test_output" | grep -c "^[[:space:]]*--- PASS: $root/$carrier/$size (" || true)" = 1 ] ||
			fail 'installed command journey lacks exact document evidence'
	done
done
printf 'installed-profile=text-command-network; result=passed; whole-host-qualification=not-established\n'
