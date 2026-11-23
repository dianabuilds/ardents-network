#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)

: "${ARDENTS_PACKAGE_VERSION:?ARDENTS_PACKAGE_VERSION is required}"
: "${ARDENTS_PACKAGE_PROGRAM:?ARDENTS_PACKAGE_PROGRAM is required}"
: "${ARDENTS_PACKAGE_STATIC_ROOT:?ARDENTS_PACKAGE_STATIC_ROOT is required}"
: "${ARDENTS_PACKAGE_OUTPUT:?ARDENTS_PACKAGE_OUTPUT is required}"

case "$ARDENTS_PACKAGE_VERSION" in
  *[!0-9A-Za-z.+:~-]* | '') echo "invalid Debian package version" >&2; exit 2 ;;
esac
case "$ARDENTS_PACKAGE_OUTPUT" in
  /*) ;;
  *) echo "ARDENTS_PACKAGE_OUTPUT must be absolute" >&2; exit 2 ;;
esac

if [ ! -f "$ARDENTS_PACKAGE_PROGRAM" ] || [ -L "$ARDENTS_PACKAGE_PROGRAM" ]; then
  echo "package program must be a direct regular file" >&2
  exit 2
fi
if [ ! -d "$ARDENTS_PACKAGE_STATIC_ROOT" ] || [ -L "$ARDENTS_PACKAGE_STATIC_ROOT" ]; then
  echo "package static root must be a direct directory" >&2
  exit 2
fi
if [ ! -f "$ARDENTS_PACKAGE_STATIC_ROOT/SHA256SUMS" ] || [ ! -f "$ARDENTS_PACKAGE_STATIC_ROOT/RELEASE" ]; then
  echo "package static root lacks SHA256SUMS or RELEASE" >&2
  exit 2
fi
if [ -e "$ARDENTS_PACKAGE_OUTPUT" ]; then
  echo "refusing to overwrite package output" >&2
  exit 2
fi

package_stage=$(mktemp -d "${TMPDIR:-/tmp}/ardents-ubuntu-deb.XXXXXX")
cleanup() {
  rm -rf "$package_stage"
}
trap cleanup EXIT HUP INT TERM

static_destination="$package_stage/usr/share/ardents/enrollment/$ARDENTS_PACKAGE_VERSION"
mkdir -p "$package_stage/DEBIAN" "$package_stage/usr/bin" "$package_stage/usr/lib/ardents" "$static_destination"
printf 'Package: ardents\nVersion: %s\nSection: net\nPriority: optional\nArchitecture: amd64\nMaintainer: Ardents Network <noreply@invalid>\nDescription: Ardents Endpoint (closed alpha)\n' "$ARDENTS_PACKAGE_VERSION" > "$package_stage/DEBIAN/control"
endpoint_artifact=$(cd "$repository" && go run -buildvcs=false ./cmd/ardents-next enrollment artifact-name ardents linux-amd64)
for source in "$ARDENTS_PACKAGE_STATIC_ROOT"/*; do
  name=$(basename "$source")
  case "$name" in
    '' | *[!A-Za-z0-9._-]*) echo "package static root has an invalid entry name" >&2; exit 2 ;;
  esac
  if [ "$name" = "$endpoint_artifact" ] || [ ! -f "$source" ] || [ -L "$source" ]; then
    echo "package static root has an invalid entry" >&2
    exit 2
  fi
  install -m 0600 "$source" "$static_destination/$name"
done

# Verify the exact staged bytes as a private, single-link Enrollment snapshot.
# Only after validation assign the package's installation paths and modes.
program="$static_destination/$endpoint_artifact"
install -m 0700 "$ARDENTS_PACKAGE_PROGRAM" "$program"
(cd "$repository" && go run -buildvcs=false ./cmd/ardents-next enrollment check-candidate "$static_destination")
mv "$program" "$package_stage/usr/lib/ardents/ardents"
chmod 0755 "$package_stage/usr/lib/ardents/ardents"
chmod 0644 "$static_destination"/*
printf '%s\n' '#!/bin/sh' 'exec /usr/lib/ardents/ardents "$@"' > "$package_stage/usr/bin/ardents"
chmod 0755 "$package_stage/usr/bin/ardents"

dpkg-deb --build --root-owner-group "$package_stage" "$ARDENTS_PACKAGE_OUTPUT"
