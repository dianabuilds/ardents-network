#!/bin/sh
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/ardents-alpha-bundle-test.XXXXXX")
cleanup() {
  rm -rf "$scratch"
}
trap cleanup EXIT HUP INT TERM

static_root="$scratch/static"
mkdir -m 700 "$static_root"
printf 'endpoint\n' > "$scratch/ardents-linux-amd64"
printf 'control\n' > "$scratch/ardents-control-linux-amd64"
chmod 700 "$scratch/ardents-linux-amd64" "$scratch/ardents-control-linux-amd64"
for name in 1.root.json 1.snapshot.json 1.targets.json catalog.ac1 catalog.pub compatibility.ac1 compatibility.pub corpus.pub network.ac1 network.pub release.ac1 release.pub timestamp.json; do
  printf '%s\n' "$name" > "$static_root/$name"
done
cat > "$static_root/RELEASE" <<'EOF'
schema=ardents-closed-alpha-enrollment-v3
cohort=alpha-test
release=h4-alpha-test-1
platform=linux-amd64
environment=alpha
network=alpha-test
target_path=ardents/linux-amd64/endpoint
artifact=ardents-linux-amd64
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
control_artifact=ardents-control-linux-amd64
EOF

build() {
  ARDENTS_ALPHA_BUNDLE_COHORT=alpha-test \
  ARDENTS_ALPHA_BUNDLE_RELEASE=h4-alpha-test-1 \
  ARDENTS_ALPHA_BUNDLE_ENDPOINT="$scratch/ardents-linux-amd64" \
  ARDENTS_ALPHA_BUNDLE_CONTROL="$scratch/ardents-control-linux-amd64" \
  ARDENTS_ALPHA_BUNDLE_STATIC_ROOT="$static_root" \
  ARDENTS_ALPHA_BUNDLE_OUTPUT="$1" \
  SOURCE_DATE_EPOCH=0 \
  sh "$repository/packaging/alpha-bundle/build.sh"
}

build "$scratch/first.tar.gz"
build "$scratch/second.tar.gz"
cmp -s "$scratch/first.tar.gz" "$scratch/second.tar.gz"

tar -xzf "$scratch/first.tar.gz" -C "$scratch"
bundle="$scratch/ardents-alpha-h4-alpha-test-1-linux-amd64"
(
  cd "$bundle"
  sha256sum --strict --check SHA256SUMS
)

# Keep the pre-execution inventory check deterministic even when the
# participant's locale collates uppercase and lowercase names differently.
expected_names="$scratch/expected-names"
actual_names="$scratch/actual-names"
LC_ALL=C sed -n 's/^[0-9a-f]\{64\}  //p' "$bundle/SHA256SUMS" >"$expected_names"
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
  'ardents-alpha-h4-alpha-test-1-linux-amd64/' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/1.root.json' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/1.snapshot.json' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/1.targets.json' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/RELEASE' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/SHA256SUMS' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/ardents-control-linux-amd64' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/ardents-linux-amd64' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/catalog.ac1' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/catalog.pub' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/compatibility.ac1' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/compatibility.pub' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/corpus.pub' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/network.ac1' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/network.pub' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/release.ac1' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/release.pub' \
  'ardents-alpha-h4-alpha-test-1-linux-amd64/timestamp.json')" ]; then
  echo 'alpha bundle archive inventory changed' >&2
  exit 1
fi

mv "$static_root/1.snapshot.json" "$static_root/2.snapshot.json"
mv "$static_root/1.targets.json" "$static_root/2.targets.json"
build "$scratch/successor.tar.gz"
if tar -tzf "$scratch/successor.tar.gz" | grep -Fqx 'ardents-alpha-h4-alpha-test-1-linux-amd64/1.snapshot.json' ||
  ! tar -tzf "$scratch/successor.tar.gz" | grep -Fqx 'ardents-alpha-h4-alpha-test-1-linux-amd64/2.snapshot.json' ||
  ! tar -tzf "$scratch/successor.tar.gz" | grep -Fqx 'ardents-alpha-h4-alpha-test-1-linux-amd64/2.targets.json'; then
  echo 'alpha bundle did not retain exactly the selected successor metadata version' >&2
  exit 1
fi

printf 'unexpected\n' > "$static_root/unexpected"
if build "$scratch/rejected.tar.gz" >/dev/null 2>&1; then
  echo 'alpha bundle accepted an unlisted static input' >&2
  exit 1
fi
test ! -e "$scratch/rejected.tar.gz"
