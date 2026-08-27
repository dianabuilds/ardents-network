#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /path/to/r-102-linux-harness" >&2
  exit 2
fi

binary=$1
root=/tmp/a-r102
ready=/tmp/a-r102.ready
log=/tmp/a-r102.log
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

socket=$root/endpoint-state/runtime/attachment.sock
contender=$(
  "$binary" -root "$root" -expect-busy -recover-socket
)
test "$contender" = 'lock=busy'
printf '%s\n' "$contender"
test -S "$socket"
echo 'live_socket_preserved=true'

kill -KILL "$pid"
wait "$pid" || true
pid=

for _ in $(seq 1 100); do
  if recovered=$("$binary" -root "$root" -recover-socket 2>&1); then
    printf '%s\n' "$recovered"
    break
  fi
  sleep 0.1
done
if [[ "$recovered" != *"stale_socket_recovered=true"* ]]; then
  echo "post_kill_recovery_failed=$recovered" >&2
  exit 1
fi

rm -f "$socket"
printf preserve-regular >"$socket"
set +e
regular=$("$binary" -root "$root" -recover-socket 2>&1)
regular_code=$?
set -e
test "$regular_code" -ne 0
test "$regular" = 'failure=unexpected_runtime_entry_type'
test "$(cat "$socket")" = preserve-regular
echo 'unexpected_regular_preserved=true'

rm "$socket"
mkdir "$socket"
set +e
directory=$("$binary" -root "$root" -recover-socket 2>&1)
directory_code=$?
set -e
test "$directory_code" -ne 0
test "$directory" = 'failure=unexpected_runtime_entry_type'
test -d "$socket"
echo 'unexpected_directory_preserved=true'

rmdir "$socket"
target=$root/unexpected-target
mkdir "$target"
printf preserve-symlink >"$target/keep.txt"
ln -s "$target" "$socket"
set +e
symlink=$("$binary" -root "$root" -recover-socket 2>&1)
symlink_code=$?
set -e
test "$symlink_code" -ne 0
test "$symlink" = 'failure=unexpected_runtime_entry_type'
test -L "$socket"
test "$(cat "$target/keep.txt")" = preserve-symlink
rm "$socket"
echo 'unexpected_symlink_preserved=true'
