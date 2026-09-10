#!/bin/sh
# Local authorization mechanism test. This is not whole-host qualification.
set -eu
fail() { printf '%s\n' "$*" >&2; exit 1; }
[ "$(id -u)" = 0 ] || fail 'invalid environment: root policy-query driver required'
for program in pkcheck setpriv stat awk sleep timeout id systemctl cmp uname dirname; do
    command -v "$program" >/dev/null || fail "invalid environment: $program unavailable"
done
[ -r /etc/os-release ] || fail 'invalid environment: OS identity unavailable'
. /etc/os-release
[ "$ID" = ubuntu ] && [ "$VERSION_ID" = 24.04 ] && [ "$(uname -m)" = x86_64 ] || fail 'invalid environment: Ubuntu 24.04 x86-64 required'
[ "$(systemctl --version | awk 'NR == 1 {print $2}')" = 255 ] || fail 'invalid environment: systemd 255 required'
rule=/usr/share/polkit-1/rules.d/50-ardents-text.rules
source_rule=$(dirname "$0")/../../../packaging/text-worker/50-ardents-text.rules
[ -f "$rule" ] && [ ! -L "$rule" ] && [ "$(stat -c %u:%g:%a "$rule")" = 0:0:644 ] || fail 'invalid environment: root-owned mode 0644 installed rule required'
cmp -s "$source_rule" "$rule" || fail 'invalid environment: installed rule differs from this source candidate'
endpoint_uid=$(id -u ardents-endpoint) || fail 'invalid environment: Endpoint account unavailable'
foreign_uid=$(id -u nobody) || fail 'invalid environment: foreign test account unavailable'
[ "$endpoint_uid" != 0 ] && [ "$endpoint_uid" != "$foreign_uid" ] || fail 'invalid environment: distinct unprivileged accounts required'
subject_pid=
cleanup() {
    if [ -n "$subject_pid" ]; then
        kill "$subject_pid" 2>/dev/null || :
        wait "$subject_pid" 2>/dev/null || :
        subject_pid=
    fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
check_case() {
    verb=$1
    unit=$2
    expected=$3
    status=0
    timeout 5s pkcheck --action-id org.freedesktop.systemd1.manage-units --process "$subject" \
        --detail unit "$unit" --detail verb "$verb" >/dev/null 2>&1 || status=$?
    if [ "$expected" = denied ]; then
        [ "$status" = 1 ] || [ "$status" = 2 ] || fail "authorization failure: foreign subject status=$status"
    else
        [ "$status" = "$expected" ] || fail "authorization failure: verb=$verb unit=$unit status=$status expected=$expected"
    fi
    cases=$((cases + 1))
}
for account in ardents-endpoint nobody; do
    uid=$(id -u "$account")
    gid=$(id -g "$account")
    setpriv --reuid "$uid" --regid "$gid" --clear-groups sleep 120 &
    subject_pid=$!
    attempt=0
    while [ "$(stat -c %u "/proc/$subject_pid")" != "$uid" ]; do
        attempt=$((attempt + 1))
        [ "$attempt" -le 100 ] || fail 'invalid environment: test subject did not drop privileges'
        sleep 0.01
    done
    # The fixed sleep process has a space-free comm, so field 22 is unambiguous.
    started=$(awk '{print $22}' "/proc/$subject_pid/stat")
    subject="$subject_pid,$started,$uid"
    allow=0
    deny=1
    if [ "$uid" != "$endpoint_uid" ]; then allow=denied; deny=denied; fi
    cases=0
    name="ardents-text-publisher@1-123-$endpoint_uid.service"
    check_case stop "$name" "$allow"
    check_case stop "ardents-text-reader@0-123-$endpoint_uid.service" "$allow"
    for verb in start restart reload kill; do check_case "$verb" "$name" "$deny"; done
    check_case stop ardents-text-publisher.socket "$deny"
    check_case stop ardents-text-publisher@.service "$deny"
    check_case stop "ardents-text-publisher@01-123-$endpoint_uid.service" "$deny"
    check_case stop "ardents-text-publisher@1-0-$endpoint_uid.service" "$deny"
    check_case stop ardents-text-publisher@1-123-0.service "$deny"
    check_case stop "ardents-text-publisher@4294967296-123-$endpoint_uid.service" "$deny"
    check_case stop "ardents-text-other@1-123-$endpoint_uid.service" "$deny"
    check_case stop "$name
" "$deny"
    for unit in dbus.service polkit.service ardents-endpoint.service; do check_case stop "$unit" "$deny"; done
    cleanup
    printf 'authorization-cases=%s uid=%s exact-stop-only=true\n' "$cases" "$uid"
done
