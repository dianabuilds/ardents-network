#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'usage: run-ubuntu-cohort-digest.sh ARTIFACT TUF_ROOT' >&2
  exit 64
fi
if [[ $(id -u) -eq 0 ]]; then
  echo 'refusing root enrollment run' >&2
  exit 65
fi

source_artifact=$1
source_tuf_root=$2
root=/tmp/ardents-r095-cohort-digest-20260824
publisher=$root/publisher
distributor=$root/distributor
participant=$root/participant
substitute=$root/substitute
replay=$root/replay
expected_cohort=ardents-alpha-cohort-0001
expected_release=ardents-alpha-0001
expected_platform=linux-amd64
expected_environment=alpha
expected_network=ardents-alpha-0001
expected_target=ardents/linux-amd64/endpoint
cleanup_failed=0

if [[ -e $root || -L $root ]]; then
  echo "refusing occupied enrollment root: $root" >&2
  exit 66
fi

cleanup() {
  if [[ ! -e $root ]]; then
    return
  fi
  if [[ -L $root ]]; then
    echo "refusing symlink cleanup target: $root" >&2
    cleanup_failed=1
    return
  fi
  local parent target actual
  parent=$(realpath "$(dirname "$root")")
  target=$parent/$(basename "$root")
  actual=$(realpath "$root")
  if [[ $actual != "$target" ]]; then
    echo "refusing unexpected cleanup target: $actual" >&2
    cleanup_failed=1
    return
  fi
  rm -rf -- "$actual"
}
trap cleanup EXIT

for command in sha256sum awk cmp find sort stat realpath; do
  command -v "$command" >/dev/null
done
mkdir -m 700 "$root" "$publisher" "$distributor" "$participant" \
  "$substitute" "$replay"

write_descriptor() {
  local directory=$1 release=$2
  printf '%s\n' \
    'schema=ardents-closed-alpha-enrollment-v1' \
    "cohort=$expected_cohort" \
    "release=$release" \
    "platform=$expected_platform" \
    "environment=$expected_environment" \
    "network=$expected_network" \
    "target_path=$expected_target" \
    'artifact=ardents-linux-amd64' \
    'trusted_root=1.root.json' >"$directory/RELEASE"
}

write_manifest() {
  local directory=$1
  (
    cd "$directory"
    LC_ALL=C sha256sum RELEASE ardents-linux-amd64 1.root.json >SHA256SUMS
  )
}

verify_inventory() {
  local directory=$1 expected actual entry facts expected_facts
  expected=$(printf '%s\n' \
    '1.root.json:f' \
    'RELEASE:f' \
    'SHA256SUMS:f' \
    'ardents-linux-amd64:f' | LC_ALL=C sort)
  actual=$(find "$directory" -mindepth 1 -maxdepth 1 -printf '%f:%y\n' |
    LC_ALL=C sort)
  [[ $actual == "$expected" ]] || return 1
  expected_facts="$(id -u):600:1"
  for entry in RELEASE SHA256SUMS ardents-linux-amd64 1.root.json; do
    [[ -f $directory/$entry && ! -L $directory/$entry ]] || return 1
    facts=$(stat -c '%u:%a:%h' "$directory/$entry")
    [[ $facts == "$expected_facts" ]] || return 1
  done
}

expected_descriptor=$(printf '%s\n' \
  'schema=ardents-closed-alpha-enrollment-v1' \
  "cohort=$expected_cohort" \
  "release=$expected_release" \
  "platform=$expected_platform" \
  "environment=$expected_environment" \
  "network=$expected_network" \
  "target_path=$expected_target" \
  'artifact=ardents-linux-amd64' \
  'trusted_root=1.root.json')

cp -- "$source_artifact" "$distributor/ardents-linux-amd64"
cp -- "$source_tuf_root" "$distributor/1.root.json"
chmod 600 "$distributor/ardents-linux-amd64" "$distributor/1.root.json"
write_descriptor "$distributor" "$expected_release"
write_manifest "$distributor"
expected_manifest_digest=$(sha256sum "$distributor/SHA256SUMS" | awk '{print $1}')

cp -- "$distributor/RELEASE" "$distributor/SHA256SUMS" \
  "$distributor/ardents-linux-amd64" "$distributor/1.root.json" "$participant/"
chmod 600 "$participant"/*
actual_manifest_digest=$(sha256sum "$participant/SHA256SUMS" | awk '{print $1}')
[[ $actual_manifest_digest == "$expected_manifest_digest" ]]
verify_inventory "$participant"
(
  cd "$participant"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
[[ $(cat "$participant/RELEASE") == "$expected_descriptor" ]]

printf 'unexpected\n' >"$participant/unknown-entry"
chmod 600 "$participant/unknown-entry"
if verify_inventory "$participant"; then
  echo 'extra inventory entry unexpectedly accepted' >&2
  exit 67
fi
rm -- "$participant/unknown-entry"

mv -- "$participant/1.root.json" "$root/root.missing"
if verify_inventory "$participant"; then
  echo 'missing inventory entry unexpectedly accepted' >&2
  exit 68
fi
mv -- "$root/root.missing" "$participant/1.root.json"

mv -- "$participant/ardents-linux-amd64" "$root/artifact.symlink-target"
ln -s -- "$root/artifact.symlink-target" "$participant/ardents-linux-amd64"
if verify_inventory "$participant"; then
  echo 'same-content artifact symlink unexpectedly accepted' >&2
  exit 69
fi
rm -- "$participant/ardents-linux-amd64"
mv -- "$root/artifact.symlink-target" "$participant/ardents-linux-amd64"
verify_inventory "$participant"

cp -- "$participant/SHA256SUMS" "$root/manifest.good"
printf '# changed manifest\n' >>"$participant/SHA256SUMS"
[[ $(sha256sum "$participant/SHA256SUMS" | awk '{print $1}') != \
  "$expected_manifest_digest" ]]
mv -- "$root/manifest.good" "$participant/SHA256SUMS"

cp -- "$participant/ardents-linux-amd64" "$root/artifact.good"
printf 'changed\n' >>"$participant/ardents-linux-amd64"
if (cd "$participant" && sha256sum --check --strict SHA256SUMS >/dev/null 2>&1); then
  echo 'changed artifact unexpectedly matched manifest' >&2
  exit 70
fi
mv -- "$root/artifact.good" "$participant/ardents-linux-amd64"

cp -- "$participant/1.root.json" "$root/root.good"
printf 'changed\n' >>"$participant/1.root.json"
if (cd "$participant" && sha256sum --check --strict SHA256SUMS >/dev/null 2>&1); then
  echo 'changed TUF root unexpectedly matched manifest' >&2
  exit 71
fi
mv -- "$root/root.good" "$participant/1.root.json"

cp -- "$participant/RELEASE" "$root/release.good"
printf 'changed=true\n' >>"$participant/RELEASE"
if (cd "$participant" && sha256sum --check --strict SHA256SUMS >/dev/null 2>&1); then
  echo 'changed descriptor unexpectedly matched manifest' >&2
  exit 72
fi
mv -- "$root/release.good" "$participant/RELEASE"

cp -- "$participant/RELEASE" "$participant/ardents-linux-amd64" \
  "$participant/1.root.json" "$substitute/"
printf 'substituted\n' >>"$substitute/ardents-linux-amd64"
write_manifest "$substitute"
(
  cd "$substitute"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
[[ $(sha256sum "$substitute/SHA256SUMS" | awk '{print $1}') != \
  "$expected_manifest_digest" ]]

cp -- "$participant/ardents-linux-amd64" "$participant/1.root.json" "$replay/"
write_descriptor "$replay" ardents-alpha-0000
write_manifest "$replay"
(
  cd "$replay"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
[[ $(sha256sum "$replay/SHA256SUMS" | awk '{print $1}') != \
  "$expected_manifest_digest" ]]
[[ $(cat "$replay/RELEASE") != "$expected_descriptor" ]]

artifact_digest=$(sha256sum "$participant/ardents-linux-amd64" | awk '{print $1}')
root_digest=$(sha256sum "$participant/1.root.json" | awk '{print $1}')
descriptor_digest=$(sha256sum "$participant/RELEASE" | awk '{print $1}')
cmp --silent "$participant/ardents-linux-amd64" "$source_artifact"
cmp --silent "$participant/1.root.json" "$source_tuf_root"

cleanup
trap - EXIT
[[ $cleanup_failed -eq 0 && ! -e $root && ! -L $root ]]
printf '{"schema":"ardents-r095-cohort-digest-v1","passed":true,'
printf '"cohort":"%s","release":"%s","platform":"%s",' \
  "$expected_cohort" "$expected_release" "$expected_platform"
printf '"manifest_sha256":"%s","descriptor_sha256":"%s",' \
  "$expected_manifest_digest" "$descriptor_digest"
printf '"artifact_sha256":"%s","root_sha256":"%s",' \
  "$artifact_digest" "$root_digest"
printf '"strict_hashes_valid":true,"descriptor_valid":true,'
printf '"changed_manifest_rejected":true,"changed_artifact_rejected":true,'
printf '"changed_root_rejected":true,"changed_descriptor_rejected":true,'
printf '"extra_entry_rejected":true,"missing_entry_rejected":true,'
printf '"same_content_symlink_rejected":true,'
printf '"self_consistent_substitution_rejected":true,'
printf '"self_consistent_replay_rejected":true,'
printf '"artifact_not_executed":true,"cleanup_complete":true}\n'
