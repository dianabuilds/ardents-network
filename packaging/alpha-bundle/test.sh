#!/bin/sh
set -eu

if [ "$#" -ne 5 ]; then
  echo 'usage: test.sh <platform> <ardents-artifact> <ardents-node-artifact> <ardents-control-artifact> <ardents-custody-artifact>' >&2
  exit 2
fi
ARDENTS_HEADLESS_PLATFORM=$1
ARDENTS_HEADLESS_ENDPOINT=$2
ARDENTS_HEADLESS_NODE=$3
ARDENTS_HEADLESS_CONTROL=$4
ARDENTS_HEADLESS_CUSTODY=$5

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/ardents-alpha-bundle-test.XXXXXX")
cleanup() {
  rm -rf "$scratch"
}
trap cleanup EXIT HUP INT TERM

static_root="$scratch/static"
mkdir "$static_root"
chmod 700 "$static_root"
platform=$ARDENTS_HEADLESS_PLATFORM
case "$platform" in
  windows-*) executable_suffix=.exe ;;
  *) executable_suffix= ;;
esac
endpoint_name="ardents-$platform$executable_suffix"
node_name="ardents-node-$platform$executable_suffix"
control_name="ardents-control-$platform"
custody_name="ardents-custody-$platform$executable_suffix"
test -f "$ARDENTS_HEADLESS_ENDPOINT"
test -f "$ARDENTS_HEADLESS_NODE"
test -f "$ARDENTS_HEADLESS_CONTROL"
test -f "$ARDENTS_HEADLESS_CUSTODY"
for name in 1.root.json 1.snapshot.json 1.targets.json catalog.ac1 catalog.pub compatibility.ac1 compatibility.pub corpus.pub network.ac1 network.pub release.ac1 release.pub timestamp.json; do
  printf '%s\n' "$name" > "$static_root/$name"
done
cat > "$static_root/RELEASE" <<EOF
schema=ardents-closed-alpha-enrollment-v3
cohort=alpha-test
release=usable-alpha-test-1
platform=$platform
environment=alpha
network=alpha-test
target_path=ardents/linux-amd64/endpoint
artifact=$endpoint_name
trusted_root=1.root.json
control_catalog=catalog.ac1
disclosure_root=catalog.pub
control_release=release.ac1
control_network=network.ac1
control_compatibility=compatibility.ac1
control_release_root=release.pub
control_network_root=network.pub
control_compatibility_root=compatibility.pub
corpus_authority=corpus.pub
control_artifact=$control_name
EOF

build() {
  ARDENTS_ALPHA_BUNDLE_COHORT=alpha-test \
  ARDENTS_ALPHA_BUNDLE_RELEASE=usable-alpha-test-1 \
  ARDENTS_ALPHA_BUNDLE_PLATFORM="$platform" \
  ARDENTS_ALPHA_BUNDLE_ENDPOINT="$ARDENTS_HEADLESS_ENDPOINT" \
  ARDENTS_ALPHA_BUNDLE_NODE="$ARDENTS_HEADLESS_NODE" \
  ARDENTS_ALPHA_BUNDLE_CONTROL="$ARDENTS_HEADLESS_CONTROL" \
  ARDENTS_ALPHA_BUNDLE_CUSTODY="$ARDENTS_HEADLESS_CUSTODY" \
  ARDENTS_ALPHA_BUNDLE_STATIC_ROOT="$static_root" \
  ARDENTS_ALPHA_BUNDLE_OUTPUT="$1" \
  SOURCE_DATE_EPOCH=0 \
  sh "$repository/packaging/alpha-bundle/build.sh"
}

build "$scratch/first.tar.gz"
build "$scratch/second.tar.gz"
cmp -s "$scratch/first.tar.gz" "$scratch/second.tar.gz"

tar -xzf "$scratch/first.tar.gz" -C "$scratch"
bundle="$scratch/ardents-alpha-usable-alpha-test-1-$platform"

cmp -s "$ARDENTS_HEADLESS_ENDPOINT" "$bundle/$endpoint_name"
cmp -s "$ARDENTS_HEADLESS_NODE" "$bundle/$node_name"
cmp -s "$ARDENTS_HEADLESS_CONTROL" "$bundle/$control_name"
cmp -s "$ARDENTS_HEADLESS_CUSTODY" "$bundle/$custody_name"

if grep -Eq '^browser_entry_(artifact|extension)=' "$bundle/RELEASE" ||
  tar -tzf "$scratch/first.tar.gz" | grep -Eq '(ardents-browser-entry|\.xpi)$'; then
  echo 'headless bundle contains a Browser companion' >&2
  exit 1
fi
(
  cd "$bundle"
  sha256sum --strict --check SHA256SUMS
)

# Keep the pre-execution inventory check deterministic even when the
# participant's locale collates uppercase and lowercase names differently.
expected_names="$scratch/expected-names"
actual_names="$scratch/actual-names"
LC_ALL=C sed -n 's/^[0-9a-f]\{64\} [ *]//p' "$bundle/SHA256SUMS" >"$expected_names"
printf '%s\n' SHA256SUMS >>"$expected_names"
LC_ALL=C sort -o "$expected_names" "$expected_names"
LC_ALL=C find "$bundle" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' |
  LC_ALL=C sort >"$actual_names"
cmp -s "$expected_names" "$actual_names"

if ! grep -Fqx 'LC_ALL=C find . -mindepth 1 -maxdepth 1 -type f -printf '\''%f\n'\'' | LC_ALL=C sort >"$actual_names"' \
  "$repository/docs/product/closed-alpha-enrollment.md"; then
  echo 'participant inventory instruction lost locale-independent sorting' >&2
  exit 1
fi

if [ "$(tar -tzf "$scratch/first.tar.gz")" != "$(printf '%s\n' \
  "ardents-alpha-usable-alpha-test-1-$platform/" \
  "ardents-alpha-usable-alpha-test-1-$platform/1.root.json" \
  "ardents-alpha-usable-alpha-test-1-$platform/1.snapshot.json" \
  "ardents-alpha-usable-alpha-test-1-$platform/1.targets.json" \
  "ardents-alpha-usable-alpha-test-1-$platform/RELEASE" \
  "ardents-alpha-usable-alpha-test-1-$platform/SHA256SUMS" \
  "ardents-alpha-usable-alpha-test-1-$platform/$control_name" \
  "ardents-alpha-usable-alpha-test-1-$platform/$custody_name" \
  "ardents-alpha-usable-alpha-test-1-$platform/$node_name" \
  "ardents-alpha-usable-alpha-test-1-$platform/$endpoint_name" \
  "ardents-alpha-usable-alpha-test-1-$platform/catalog.ac1" \
  "ardents-alpha-usable-alpha-test-1-$platform/catalog.pub" \
  "ardents-alpha-usable-alpha-test-1-$platform/compatibility.ac1" \
  "ardents-alpha-usable-alpha-test-1-$platform/compatibility.pub" \
  "ardents-alpha-usable-alpha-test-1-$platform/corpus.pub" \
  "ardents-alpha-usable-alpha-test-1-$platform/network.ac1" \
  "ardents-alpha-usable-alpha-test-1-$platform/network.pub" \
  "ardents-alpha-usable-alpha-test-1-$platform/release.ac1" \
  "ardents-alpha-usable-alpha-test-1-$platform/release.pub" \
  "ardents-alpha-usable-alpha-test-1-$platform/timestamp.json")" ]; then
  echo 'alpha bundle archive inventory changed' >&2
  exit 1
fi

mv "$static_root/1.snapshot.json" "$static_root/2.snapshot.json"
mv "$static_root/1.targets.json" "$static_root/2.targets.json"
build "$scratch/successor.tar.gz"
if tar -tzf "$scratch/successor.tar.gz" | grep -Fqx "ardents-alpha-usable-alpha-test-1-$platform/1.snapshot.json" ||
  ! tar -tzf "$scratch/successor.tar.gz" | grep -Fqx "ardents-alpha-usable-alpha-test-1-$platform/2.snapshot.json" ||
  ! tar -tzf "$scratch/successor.tar.gz" | grep -Fqx "ardents-alpha-usable-alpha-test-1-$platform/2.targets.json"; then
  echo 'alpha bundle did not retain exactly the selected successor metadata version' >&2
  exit 1
fi

printf 'unexpected\n' > "$static_root/unexpected"
if build "$scratch/rejected.tar.gz" >/dev/null 2>&1; then
  echo 'alpha bundle accepted an unlisted static input' >&2
  exit 1
fi
test ! -e "$scratch/rejected.tar.gz"
