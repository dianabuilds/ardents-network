#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /path/to/r-101-linux-harness" >&2
  exit 2
fi

binary=$1
root=/tmp/a-r101c
ready=/tmp/a-r101c.ready
log=/tmp/a-r101c.log
pid=

for path in "$root" "$ready" "$log"; do
  if [[ -e "$path" ]]; then
    echo "refusing_existing_path=$path" >&2
    exit 1
  fi
done

cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" || true
    wait "$pid" || true
  fi
  if [[ -e "$root" ]] && [[ "$(realpath "$root")" == "$root" ]]; then
    rm -rf "$root"
  fi
  rm -f "$ready" "$log"
}
trap cleanup EXIT

"$binary" -root "$root" -hold-ready "$ready" >"$log" 2>&1 &
pid=$!
for _ in $(seq 1 100); do
  if [[ -e "$ready" ]]; then
    break
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    cat "$log" >&2
    exit 1
  fi
  sleep 0.1
done
test -e "$ready"

kill -KILL "$pid"
wait "$pid" || true
pid=

test -e "$root/endpoint-state/live/owner.lock" && owner=true || owner=false
test -S "$root/endpoint-state/runtime/attachment.sock" && socket=true || socket=false
test -d "$root/endpoint-state/vault" && vault=true || vault=false
printf 'owner_file_after_kill=%s\nsocket_after_kill=%s\nvault_after_kill=%s\n' "$owner" "$socket" "$vault"
