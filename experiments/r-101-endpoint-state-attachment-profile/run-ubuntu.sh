#!/usr/bin/env bash
set -euo pipefail

root=/tmp/ardents-r101-state-ipc-20260824
if [[ -e "$root" ]]; then
  echo "Refusing to reuse existing experiment root: $root" >&2
  exit 1
fi

cleanup() {
  if [[ -e "$root" ]] && [[ "$(realpath "$root")" == "$root" ]]; then
    rm -rf "$root"
  fi
}
trap cleanup EXIT

go run main.go -root "$root"
