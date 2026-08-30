#!/bin/sh
set -eu

: "${ARDENTS_BROWSER_BUNDLE_PLATFORM:?ARDENTS_BROWSER_BUNDLE_PLATFORM is required}"
: "${ARDENTS_BROWSER_BUNDLE_ADAPTER:?ARDENTS_BROWSER_BUNDLE_ADAPTER is required}"
: "${ARDENTS_BROWSER_BUNDLE_ENTRY:?ARDENTS_BROWSER_BUNDLE_ENTRY is required}"
: "${ARDENTS_BROWSER_BUNDLE_OUTPUT:?ARDENTS_BROWSER_BUNDLE_OUTPUT is required}"
: "${SOURCE_DATE_EPOCH:?SOURCE_DATE_EPOCH is required}"

platform=$ARDENTS_BROWSER_BUNDLE_PLATFORM
case "$platform" in
  *[!A-Za-z0-9._-]* | '') echo 'invalid Browser bundle platform' >&2; exit 2 ;;
  windows-*) executable_suffix=.exe ;;
  *) executable_suffix= ;;
esac
case "$SOURCE_DATE_EPOCH" in
  *[!0-9]* | '') echo 'SOURCE_DATE_EPOCH must be a non-negative Unix timestamp' >&2; exit 2 ;;
esac
case "$ARDENTS_BROWSER_BUNDLE_OUTPUT" in
  /*) ;;
  *) echo 'ARDENTS_BROWSER_BUNDLE_OUTPUT must be an absolute path' >&2; exit 2 ;;
esac

adapter_name="ardents-browser-$platform$executable_suffix"
entry_name="ardents-browser-entry-$platform$executable_suffix"
bundle_name="ardents-browser-companion-$platform"
for input in "$ARDENTS_BROWSER_BUNDLE_ADAPTER" "$ARDENTS_BROWSER_BUNDLE_ENTRY"; do
  if [ ! -f "$input" ] || [ -L "$input" ]; then
    echo 'Browser bundle executable input must be a direct regular file' >&2
    exit 2
  fi
done
if [ "$(basename "$ARDENTS_BROWSER_BUNDLE_ADAPTER")" != "$adapter_name" ] ||
  [ "$(basename "$ARDENTS_BROWSER_BUNDLE_ENTRY")" != "$entry_name" ]; then
  echo 'Browser bundle executable input has a non-canonical artifact name' >&2
  exit 2
fi
output_parent=$(dirname "$ARDENTS_BROWSER_BUNDLE_OUTPUT")
if [ ! -d "$output_parent" ] || [ -L "$output_parent" ]; then
  echo 'Browser bundle output parent must be a direct existing directory' >&2
  exit 2
fi
if [ -e "$ARDENTS_BROWSER_BUNDLE_OUTPUT" ]; then
  echo 'refusing to overwrite Browser bundle output' >&2
  exit 2
fi

umask 077
stage=$(mktemp -d "${TMPDIR:-/tmp}/ardents-browser-bundle.XXXXXX")
archive_stage=$(mktemp "$output_parent/.${bundle_name}.XXXXXX")
cleanup() {
  rm -rf "$stage"
  rm -f "$archive_stage"
}
trap cleanup EXIT HUP INT TERM

bundle="$stage/$bundle_name"
mkdir -m 700 "$bundle"
install -m 700 "$ARDENTS_BROWSER_BUNDLE_ADAPTER" "$bundle/$adapter_name"
install -m 700 "$ARDENTS_BROWSER_BUNDLE_ENTRY" "$bundle/$entry_name"
(
  cd "$bundle"
  sha256sum "$adapter_name" "$entry_name" > SHA256SUMS
  chmod 600 SHA256SUMS
)
(
  cd "$stage"
  tar --format=posix --pax-option=delete=atime,delete=ctime --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -cf - "$bundle_name" |
    gzip -n > "$archive_stage"
)
if ! ln "$archive_stage" "$ARDENTS_BROWSER_BUNDLE_OUTPUT"; then
  echo 'Browser bundle output appeared during publication' >&2
  exit 2
fi
rm -f "$archive_stage"
archive_stage=''
sha256sum "$ARDENTS_BROWSER_BUNDLE_OUTPUT"
