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
manifest=/etc/ardents/text-worker-artifact.json
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
[ -f "$manifest" ] && [ ! -L "$manifest" ] && [ "$(stat -c %u:%g:%a "$manifest")" = 0:0:644 ] ||
	fail 'invalid environment: root-owned worker manifest required'
manifest_pair=$(printf '"%s":"%s"' "$worker" "$ARDENTS_TEXT_COMMAND_WORKER_SHA256")
grep -Fq "$manifest_pair" "$manifest" ||
	fail 'invalid environment: worker manifest differs from declared candidate'
endpoint_uid=$(id -u ardents-endpoint)
endpoint_gid=$(id -g ardents-endpoint)
for role in reader publisher; do
	socket_unit="ardents-text-$role.socket"
	socket_path="/run/ardents-text/$role.sock"
	[ "$(systemctl show "$socket_unit" -p ActiveState --value)" = active ] &&
		[ -S "$socket_path" ] &&
		[ "$(stat -c %u:%g:%a "$socket_path")" = "$endpoint_uid:$endpoint_gid:600" ] ||
		fail "invalid environment: $role worker socket is not active for Endpoint"
done

[ "$(systemctl show ardents-endpoint.service -p FragmentPath --value)" = "$unit" ] ||
	fail 'invalid environment: a different Endpoint unit is loaded'
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] ||
	fail 'invalid environment: Endpoint is not fresh and inactive'
[ -z "$(systemctl show ardents-endpoint.service -p DropInPaths --value)" ] ||
	fail 'invalid environment: qualification unit has drop-ins'

if [ -n "${ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT-}" ]; then
    case "$ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT" in
        /*) ;;
        *) fail 'invalid environment: command evidence root must be absolute' ;;
    esac
    [ -d "$ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT" ] && [ ! -L "$ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT" ] &&
        [ "$(stat -c %u:%g:%a "$ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT")" = 0:0:700 ] ||
        fail 'invalid environment: owner-private command evidence root required'
    run_log=$(umask 077; mktemp "$ARDENTS_TEXT_COMMAND_EVIDENCE_ROOT/command-journey.XXXXXXXX") ||
        fail 'invalid environment: command evidence log unavailable'
    printf 'command-evidence-log=%s\n' "$run_log" >&2
else
    run_log=$(mktemp /var/tmp/ardents-text-command-network.XXXXXX) || fail 'invalid environment: command evidence log unavailable'
    trap 'rm -f "$run_log"' EXIT HUP INT TERM
fi
result=0
ARDENTS_TEXT_COMMAND_QUALIFICATION=1 ARDENTS_E2E_COMMAND_ROOT="$command_root" \
    timeout --signal=TERM --kill-after=30s 3060s "$binary" -test.run='^TestInstalledClosedTextCommandsThroughNodeProcesses$' -test.v -test.timeout=49m >"$run_log" 2>&1 || result=$?
cat "$run_log"
[ "$result" -eq 0 ] || fail "installed command journey test failed with exit status $result"
[ "$(systemctl show ardents-endpoint.service -p ActiveState --value)" = inactive ] &&
	[ "$(systemctl show ardents-endpoint.service -p MainPID --value)" = 0 ] ||
	fail 'installed command journey retained the temporary Endpoint'

root=TestInstalledClosedTextCommandsThroughNodeProcesses
[ "$(grep -c "^[[:space:]]*--- PASS: $root (" "$run_log" || true)" = 1 ] ||
	fail 'installed command journey lacks root test evidence'
for carrier in ardents-carrier-tcp-tls-v2 ardents-carrier-quic-v2; do
	[ "$(grep -c "^[[:space:]]*--- PASS: $root/$carrier (" "$run_log" || true)" = 1 ] ||
		fail 'installed command journey lacks exact Carrier evidence'
	for size in empty 64KiB 4MiB; do
		[ "$(grep -c "^[[:space:]]*--- PASS: $root/$carrier/$size (" "$run_log" || true)" = 1 ] ||
			fail 'installed command journey lacks exact document evidence'
	done
done
printf 'installed-profile=text-command-network; result=passed; whole-host-qualification=not-established\n'
