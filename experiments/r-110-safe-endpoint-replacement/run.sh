#!/bin/sh
set -eu

root=$(mktemp -d "${TMPDIR:-/tmp}/ardents-r110.XXXXXX")
trap 'rm -rf -- "$root"' EXIT HUP INT TERM
program="$root/program"
state="$root/state"
stage="$root/stage"
mkdir -m 700 "$program" "$state" "$stage" "$state/vault" "$state/floors"
printf 'v1\n' >"$program/ardents"
printf 'vault-must-not-move\n' >"$state/vault/authority"
printf 'floor=7\n' >"$state/floors/release"

show() {
  label=$1
  printf '\n[%s]\n' "$label"
  printf 'program=%s stage=%s journal=%s vault=%s floor=%s\n' \
    "$(cat "$program/ardents")" "$(cat "$stage/candidate" 2>/dev/null || printf absent)" \
    "$(cat "$state/journal" 2>/dev/null || printf absent)" "$(cat "$state/vault/authority")" \
    "$(cat "$state/floors/release")"
}

replace() {
  candidate=$1 mode=$2 active=$3
  if [ "$active" = yes ]; then
    printf 'refused-active\n' >"$state/journal"
    show "refused while endpoint active"
    return
  fi
  printf '%s\n' "$candidate" >"$stage/candidate"
  printf 'staged\n' >"$state/journal"
  if [ "$mode" = before-rename ]; then
    show "interrupted after stage"
    return
  fi
  mv "$stage/candidate" "$program/ardents"
  printf 'self-test-required\n' >"$state/journal"
  if [ "$mode" = after-rename ]; then
    show "interrupted after atomic activation"
    return
  fi
  if [ "$mode" = self-test-failed ]; then
    printf 'rollback-authorization-required\n' >"$state/journal"
    show "candidate self-test failed"
    return
  fi
  printf 'committed-restart-permitted\n' >"$state/journal"
  show "candidate self-test passed"
}

show "initial"
replace v2 normal yes
replace v2 before-rename no
rm -f "$stage/candidate"
printf 'v1\n' >"$program/ardents"
replace v2 after-rename no
printf 'v1\n' >"$program/ardents"
replace v2 self-test-failed no
printf 'v1\n' >"$program/ardents"
replace v2 committed no
