#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo 'usage: test.sh <platform> <ardents-artifact>' >&2
  exit 2
fi
ARDENTS_HEADLESS_PLATFORM=$1
ARDENTS_HEADLESS_ENDPOINT=$2

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
artifact_name() {
  (cd "$repository" && go run ./cmd/ardents-next enrollment artifact-name "$1" "$2")
}
scratch=$(mktemp -d "${TMPDIR:-/tmp}/ardents-alpha-bundle-test.XXXXXX")
cleanup() {
  rm -rf "$scratch"
}
trap cleanup EXIT HUP INT TERM

static_root="$scratch/static"
mkdir "$static_root"
chmod 700 "$static_root"
platform=$ARDENTS_HEADLESS_PLATFORM
endpoint_name=$(artifact_name ardents "$platform")
test -f "$ARDENTS_HEADLESS_ENDPOINT"
for name in 1.root.json 1.snapshot.json 1.targets.json timestamp.json; do
  printf '%s\n' "$name" > "$static_root/$name"
done
cat > "$static_root/RELEASE" <<EOF
schema=ardents-closed-alpha-enrollment-v6
cohort=alpha-test
release=usable-alpha-test-1
platform=$platform
environment=alpha
network=alpha-test
target_path=ardents/linux-amd64/endpoint
artifact=$endpoint_name
trusted_root=1.root.json
EOF

build() {
  ARDENTS_ALPHA_BUNDLE_COHORT=alpha-test \
  ARDENTS_ALPHA_BUNDLE_RELEASE=usable-alpha-test-1 \
  ARDENTS_ALPHA_BUNDLE_PLATFORM="$platform" \
  ARDENTS_ALPHA_BUNDLE_ENDPOINT="$ARDENTS_HEADLESS_ENDPOINT" \
  ARDENTS_ALPHA_BUNDLE_TEXT="${protected_text:-}" \
  ARDENTS_ALPHA_BUNDLE_STATIC_ROOT="$static_root" \
  ARDENTS_ALPHA_BUNDLE_OUTPUT="$1" \
  SOURCE_DATE_EPOCH=0 \
  sh "$repository/packaging/alpha-bundle/build.sh"
}

build "$scratch/first.tar.gz"
build "$scratch/second.tar.gz"
cmp -s "$scratch/first.tar.gz" "$scratch/second.tar.gz"

cp "$static_root/RELEASE" "$scratch/current-release"
sed '1s/enrollment-v6$/enrollment-v5/' "$scratch/current-release" > "$static_root/RELEASE"
if build "$scratch/retired.tar.gz"; then
  echo 'alpha bundle accepted a retired descriptor' >&2
  exit 1
fi
test ! -e "$scratch/retired.tar.gz"
cp "$scratch/current-release" "$static_root/RELEASE"

tar -xzf "$scratch/first.tar.gz" -C "$scratch"
bundle="$scratch/ardents-alpha-usable-alpha-test-1-$platform"

cmp -s "$ARDENTS_HEADLESS_ENDPOINT" "$bundle/$endpoint_name"

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
  "$repository/docs/reference/portable-enrollment.md"; then
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
  "ardents-alpha-usable-alpha-test-1-$platform/$endpoint_name" \
  "ardents-alpha-usable-alpha-test-1-$platform/timestamp.json" | LC_ALL=C sort)" ]; then
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

if [ "$platform" = linux-amd64 ]; then
  # Packaging controls use synthetic resources and an existing binary as the
  # text-byte fixture. They do not execute or qualify a confined text worker.
  protected_text=$ARDENTS_HEADLESS_ENDPOINT
  resource_names="50-ardents-text.rules
ardents-endpoint.service
ardents-text-publisher.socket
ardents-text-publisher@.service
ardents-text-reader.socket
ardents-text-reader@.service
ardents-text.conf"
  for name in $resource_names; do
    printf 'resource: %s\n' "$name" > "$static_root/$name"
  done
  {
    printf '{"schema":"ardents-protected-endpoint-artifact-v1","platform":"linux-amd64","release_identity":"usable-alpha-test-1","release_version":1,"files":{'
    separator=''
    for name in $(printf '%s\n' "$resource_names" ardents-linux-amd64 ardents-text-linux-amd64 | LC_ALL=C sort); do
      case "$name" in
        ardents-linux-amd64) source=$ARDENTS_HEADLESS_ENDPOINT ;;
        ardents-text-linux-amd64) source=$protected_text ;;
        *) source="$static_root/$name" ;;
      esac
      digest=$(sha256sum "$source" | cut -d ' ' -f 1)
      printf '%s"%s":"%s"' "$separator" "$name" "$digest"
      separator=','
    done
    printf '}}\n'
  } > "$static_root/protected-endpoint.json"
  build "$scratch/protected-first.tar.gz"
  build "$scratch/protected-second.tar.gz"
  cmp -s "$scratch/protected-first.tar.gz" "$scratch/protected-second.tar.gz"
  protected_unpack="$scratch/protected-unpack"
  mkdir "$protected_unpack"
  tar -xzf "$scratch/protected-first.tar.gz" -C "$protected_unpack"
  protected_bundle="$protected_unpack/ardents-alpha-usable-alpha-test-1-$platform"
  test "$(wc -l < "$protected_bundle/SHA256SUMS" | tr -d ' ')" = 16
  (cd "$protected_bundle" && sha256sum --strict --check SHA256SUMS)
  cmp -s "$static_root/protected-endpoint.json" "$protected_bundle/protected-endpoint.json"
  printf 'substitution\n' > "$static_root/ardents-text.conf"
  if build "$scratch/protected-substitution.tar.gz"; then
    echo 'producer accepted a substituted resource against unchanged descriptor' >&2
    exit 1
  fi
  test ! -e "$scratch/protected-substitution.tar.gz"
  rm "$static_root/ardents-text.conf"
  if build "$scratch/protected-partial.tar.gz"; then
    echo 'producer accepted a partial protected inventory' >&2
    exit 1
  fi
  test ! -e "$scratch/protected-partial.tar.gz"
fi

printf 'retired catalog\n' > "$static_root/catalog.ac1"
if build "$scratch/rejected.tar.gz" >/dev/null 2>&1; then
  echo 'alpha bundle accepted the retired catalog' >&2
  exit 1
fi
test ! -e "$scratch/rejected.tar.gz"
