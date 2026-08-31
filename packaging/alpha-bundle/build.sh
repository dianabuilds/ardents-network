#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
artifact_name() {
  (cd "$repository" && go run ./scripts/enrollment-artifact-name.go "$1" "$2")
}

: "${ARDENTS_ALPHA_BUNDLE_COHORT:?ARDENTS_ALPHA_BUNDLE_COHORT is required}"
: "${ARDENTS_ALPHA_BUNDLE_RELEASE:?ARDENTS_ALPHA_BUNDLE_RELEASE is required}"
: "${ARDENTS_ALPHA_BUNDLE_PLATFORM:?ARDENTS_ALPHA_BUNDLE_PLATFORM is required}"
: "${ARDENTS_ALPHA_BUNDLE_ENDPOINT:?ARDENTS_ALPHA_BUNDLE_ENDPOINT is required}"
: "${ARDENTS_ALPHA_BUNDLE_NODE:?ARDENTS_ALPHA_BUNDLE_NODE is required}"
: "${ARDENTS_ALPHA_BUNDLE_CONTROL:?ARDENTS_ALPHA_BUNDLE_CONTROL is required}"
: "${ARDENTS_ALPHA_BUNDLE_CUSTODY:?ARDENTS_ALPHA_BUNDLE_CUSTODY is required}"
: "${ARDENTS_ALPHA_BUNDLE_STATIC_ROOT:?ARDENTS_ALPHA_BUNDLE_STATIC_ROOT is required}"
: "${ARDENTS_ALPHA_BUNDLE_OUTPUT:?ARDENTS_ALPHA_BUNDLE_OUTPUT is required}"
: "${SOURCE_DATE_EPOCH:?SOURCE_DATE_EPOCH is required}"

platform=$ARDENTS_ALPHA_BUNDLE_PLATFORM
endpoint_name=$(artifact_name ardents "$platform")
node_name=$(artifact_name ardents-node "$platform")
control_name=$(artifact_name ardents-control "$platform")
custody_name=$(artifact_name ardents-custody "$platform")
bundle_name="ardents-alpha-${ARDENTS_ALPHA_BUNDLE_RELEASE}-${platform}"

case "$ARDENTS_ALPHA_BUNDLE_COHORT" in
  *[!A-Za-z0-9._-]* | '') echo 'invalid alpha cohort' >&2; exit 2 ;;
esac
case "$ARDENTS_ALPHA_BUNDLE_RELEASE" in
  *[!A-Za-z0-9._-]* | '') echo 'invalid alpha release' >&2; exit 2 ;;
esac
case "$platform" in
  *[!A-Za-z0-9._-]* | '') echo 'invalid alpha platform' >&2; exit 2 ;;
esac
case "$SOURCE_DATE_EPOCH" in
  *[!0-9]* | '') echo 'SOURCE_DATE_EPOCH must be a non-negative Unix timestamp' >&2; exit 2 ;;
esac
case "$ARDENTS_ALPHA_BUNDLE_OUTPUT" in
  /*) ;;
  *) echo 'ARDENTS_ALPHA_BUNDLE_OUTPUT must be an absolute path' >&2; exit 2 ;;
esac

for input in "$ARDENTS_ALPHA_BUNDLE_ENDPOINT" "$ARDENTS_ALPHA_BUNDLE_NODE" "$ARDENTS_ALPHA_BUNDLE_CONTROL" "$ARDENTS_ALPHA_BUNDLE_CUSTODY"; do
  if [ ! -f "$input" ] || [ -L "$input" ]; then
    echo 'alpha bundle executable input must be a direct regular file' >&2
    exit 2
  fi
done
if [ ! -d "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT" ] || [ -L "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT" ]; then
  echo 'alpha bundle static root must be a direct directory' >&2
  exit 2
fi
if [ -e "$ARDENTS_ALPHA_BUNDLE_OUTPUT" ]; then
  echo 'refusing to overwrite alpha bundle output' >&2
  exit 2
fi

metadata_names=$(for source in "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT"/*.snapshot.json "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT"/*.targets.json; do
  [ -f "$source" ] || continue
  basename "$source"
done | LC_ALL=C sort)
metadata_count=$(printf '%s\n' "$metadata_names" | sed '/^$/d' | wc -l | tr -d ' ')
if [ "$metadata_count" -ne 2 ]; then
  echo 'alpha bundle static root must contain exactly one versioned snapshot/targets pair' >&2
  exit 2
fi
snapshot_name=$(printf '%s\n' "$metadata_names" | sed -n '/\.snapshot\.json$/p')
targets_name=$(printf '%s\n' "$metadata_names" | sed -n '/\.targets\.json$/p')
snapshot_version=${snapshot_name%.snapshot.json}
targets_version=${targets_name%.targets.json}
case "$snapshot_version" in
  *[!0-9]* | '' | 0 | 0*) echo 'alpha bundle versioned metadata generation is invalid' >&2; exit 2 ;;
esac
if [ -z "$snapshot_name" ] || [ -z "$targets_name" ] || [ "$snapshot_version" != "$targets_version" ]; then
  echo 'alpha bundle versioned snapshot/targets pair is invalid' >&2
  exit 2
fi

static_names="1.root.json
$snapshot_name
$targets_name
RELEASE
catalog.ac1
catalog.pub
compatibility.ac1
compatibility.pub
corpus.pub
network.ac1
network.pub
release.ac1
release.pub
timestamp.json"

for name in $static_names; do
  source="$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT/$name"
  if [ ! -f "$source" ] || [ -L "$source" ]; then
    echo "alpha bundle static root lacks direct regular $name" >&2
    exit 2
  fi
done
actual_names=$(find "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)
if [ "$actual_names" != "$(printf '%s\n' "$static_names" | LC_ALL=C sort)" ]; then
  echo 'alpha bundle static root has an unexpected inventory' >&2
  exit 2
fi
release_descriptor="$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT/RELEASE"
for entry in \
  "schema=ardents-closed-alpha-enrollment-v3" \
  "cohort=$ARDENTS_ALPHA_BUNDLE_COHORT" \
  "release=$ARDENTS_ALPHA_BUNDLE_RELEASE" \
  "platform=$platform" \
  'environment=alpha' \
  "artifact=$endpoint_name" \
  "control_artifact=$control_name" \
  'trusted_root=1.root.json'; do
  if ! grep -F -x -- "$entry" "$release_descriptor" >/dev/null; then
    echo "alpha bundle RELEASE lacks required entry: $entry" >&2
    exit 2
  fi
done

output_parent=$(dirname "$ARDENTS_ALPHA_BUNDLE_OUTPUT")
if [ ! -d "$output_parent" ] || [ -L "$output_parent" ]; then
  echo 'alpha bundle output parent must be a direct existing directory' >&2
  exit 2
fi

umask 077
stage=$(mktemp -d "${TMPDIR:-/tmp}/ardents-alpha-bundle.XXXXXX")
archive_stage=$(mktemp "$output_parent/.${bundle_name}.XXXXXX")
cleanup() {
  rm -rf "$stage"
  rm -f "$archive_stage"
}
trap cleanup EXIT HUP INT TERM

bundle="$stage/$bundle_name"
mkdir -m 700 "$bundle"
install -m 700 "$ARDENTS_ALPHA_BUNDLE_ENDPOINT" "$bundle/$endpoint_name"
install -m 700 "$ARDENTS_ALPHA_BUNDLE_NODE" "$bundle/$node_name"
install -m 700 "$ARDENTS_ALPHA_BUNDLE_CONTROL" "$bundle/$control_name"
install -m 700 "$ARDENTS_ALPHA_BUNDLE_CUSTODY" "$bundle/$custody_name"
for name in $static_names; do
  install -m 600 "$ARDENTS_ALPHA_BUNDLE_STATIC_ROOT/$name" "$bundle/$name"
done
(
  cd "$bundle"
  LC_ALL=C find . -mindepth 1 -maxdepth 1 -type f ! -name SHA256SUMS -printf '%f\n' | LC_ALL=C sort | while IFS= read -r name; do
    sha256sum "$name"
  done > SHA256SUMS
  chmod 600 SHA256SUMS
)

(
  cd "$stage"
  tar --format=posix --pax-option=delete=atime,delete=ctime --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -cf - "$bundle_name" |
    gzip -n > "$archive_stage"
)
if ! ln "$archive_stage" "$ARDENTS_ALPHA_BUNDLE_OUTPUT"; then
  echo 'alpha bundle output appeared during publication' >&2
  exit 2
fi
rm -f "$archive_stage"
archive_stage=''
sha256sum "$ARDENTS_ALPHA_BUNDLE_OUTPUT"
