#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  printf 'usage: %s LOCK OUTPUT_DIRECTORY\n' "$0" >&2
  exit 64
fi

lock=$(realpath "$1")
output=$2
mkdir -p "$output"
output=$(realpath "$output")
[[ "$output" != / && "$output" != "$PWD" ]] || exit 64

while IFS=$'\t' read -r kind name version filename digest url license; do
  [[ "$kind" == package ]] || continue
  curl --fail --location --proto '=https' --tlsv1.2 --output "$output/$filename" "$url"
  printf '%s  %s\n' "$digest" "$output/$filename" | sha256sum --check --status
  [[ $(dpkg-deb --field "$output/$filename" Package) == "$name" ]]
  [[ $(dpkg-deb --field "$output/$filename" Version) == "$version" ]]
  [[ $(dpkg-deb --field "$output/$filename" Architecture) == amd64 ]]
done < "$lock"

expected=$(awk -F '\t' '$1 == "package" { count++ } END { print count+0 }' "$lock")
actual=$(find "$output" -mindepth 1 -maxdepth 1 -type f | wc -l)
[[ "$actual" == "$expected" ]]
printf 'prepared %s pinned Carrier Lab tool packages in %s\n' "$actual" "$output"
