#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
  echo 'usage: test.sh <platform> <ardents-browser-artifact> <ardents-browser-entry-artifact>' >&2
  exit 2
fi
platform=$1
adapter=$2
entry=$3
repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/ardents-browser-bundle-test.XXXXXX")
cleanup() {
  rm -rf "$scratch"
}
trap cleanup EXIT HUP INT TERM

build() {
  ARDENTS_BROWSER_BUNDLE_PLATFORM="$platform" \
  ARDENTS_BROWSER_BUNDLE_ADAPTER="$adapter" \
  ARDENTS_BROWSER_BUNDLE_ENTRY="$entry" \
  ARDENTS_BROWSER_BUNDLE_OUTPUT="$1" \
  SOURCE_DATE_EPOCH=0 \
  sh "$repository/packaging/browser-bundle/build.sh"
}

build "$scratch/first.tar.gz"
build "$scratch/second.tar.gz"
cmp -s "$scratch/first.tar.gz" "$scratch/second.tar.gz"

tar -xzf "$scratch/first.tar.gz" -C "$scratch"
case "$platform" in
  windows-*) executable_suffix=.exe ;;
  *) executable_suffix= ;;
esac
adapter_name="ardents-browser-$platform$executable_suffix"
entry_name="ardents-browser-entry-$platform$executable_suffix"
bundle_name="ardents-browser-companion-$platform"
bundle="$scratch/$bundle_name"

cmp -s "$adapter" "$bundle/$adapter_name"
cmp -s "$entry" "$bundle/$entry_name"
(
  cd "$bundle"
  sha256sum --strict --check SHA256SUMS
)
if [ "$(tar -tzf "$scratch/first.tar.gz")" != "$(printf '%s\n' \
  "$bundle_name/" \
  "$bundle_name/SHA256SUMS" \
  "$bundle_name/$entry_name" \
  "$bundle_name/$adapter_name")" ]; then
  echo 'Browser bundle archive inventory changed' >&2
  exit 1
fi
