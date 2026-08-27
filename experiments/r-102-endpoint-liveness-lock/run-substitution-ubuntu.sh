#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /path/to/r-102-linux-harness" >&2
  exit 2
fi

binary=$1
parent=/tmp/a-r102-sub
root=$parent/root
state=$root/endpoint-state
runtime=$state/runtime
outside=$parent/outside
ready=/tmp/a-r102-sub.ready
log=/tmp/a-r102-sub.log
pid=

for path in "$parent" "$ready" "$log"; do
  if [[ -e "$path" || -L "$path" ]]; then
    echo "refusing_existing_path=$path" >&2
    exit 1
  fi
done

cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" || true
    wait "$pid" || true
  fi
  if [[ -L "$runtime" ]]; then
    if [[ "$(readlink -f "$runtime")" != "$outside" ]]; then
      echo "refusing_unexpected_runtime_link=$runtime" >&2
      exit 1
    fi
    rm "$runtime"
  fi
  if [[ -f "$outside/keep.txt" ]]; then
    if [[ "$(cat "$outside/keep.txt")" == preserve ]]; then
      echo "sentinel_preserved_after_link_removal=true"
    else
      echo "sentinel_preserved_after_link_removal=false"
    fi
  fi
  if [[ -e "$parent" ]]; then
    if [[ "$(realpath "$parent")" != "$parent" ]]; then
      echo "refusing_unexpected_experiment_parent=$parent" >&2
      exit 1
    fi
    rm -rf "$parent"
  fi
  rm -f "$ready" "$log"
}
trap cleanup EXIT

mkdir -p "$state/live" "$state/vault" "$outside"
printf preserve >"$outside/keep.txt"
ln -s "$outside" "$runtime"

"$binary" -root "$root" -hold-ready "$ready" >"$log" 2>&1 &
pid=$!
for _ in $(seq 1 100); do
  if [[ -e "$ready" ]]; then
    break
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

accepted=false
[[ -e "$ready" ]] && accepted=true
exited=false
exit_code=
if ! kill -0 "$pid" 2>/dev/null; then
  exited=true
  set +e
  wait "$pid"
  exit_code=$?
  set -e
  pid=
fi
outside_socket=false
[[ -S "$outside/attachment.sock" ]] && outside_socket=true

echo "substituted_runtime_accepted=$accepted"
echo "child_exited=$exited"
echo "exit_code=$exit_code"
echo "outside_socket_created=$outside_socket"
echo "stderr=$(tr '\n' ' ' <"$log")"
