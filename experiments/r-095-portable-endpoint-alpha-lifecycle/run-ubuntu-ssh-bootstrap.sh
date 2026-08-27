#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'usage: run-ubuntu-ssh-bootstrap.sh ARTIFACT TUF_ROOT' >&2
  exit 64
fi
if [[ $(id -u) -eq 0 ]]; then
  echo 'refusing root bootstrap run' >&2
  exit 65
fi

source_artifact=$1
source_tuf_root=$2
root=/tmp/ardents-r095-ssh-bootstrap-20260824
publisher=$root/publisher
distributor=$root/distributor
participant=$root/participant
attacker=$root/attacker
replay=$root/replay
namespace=ardents-alpha-bootstrap-v1@ardents.network
principal=release@ardents.network
expected_release=ardents-alpha-0001
expected_platform=linux-amd64
cleanup_failed=0

if [[ -e $root || -L $root ]]; then
  echo "refusing occupied bootstrap root: $root" >&2
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

for command in ssh ssh-keygen sha256sum awk grep cmp; do
  command -v "$command" >/dev/null
done
mkdir -m 700 "$root" "$publisher" "$distributor" "$participant" "$attacker" "$replay"

ssh-keygen -q -t ed25519 -N '' -C 'R-095 bootstrap fixture' \
  -f "$publisher/bootstrap-key"
expected_fingerprint=$(ssh-keygen -lf "$publisher/bootstrap-key.pub" -E sha256 |
  awk '{print $2}')
public_fields=$(awk '{print $1 " " $2}' "$publisher/bootstrap-key.pub")

cp -- "$source_artifact" "$distributor/ardents-linux-amd64"
cp -- "$source_tuf_root" "$distributor/1.root.json"
chmod 600 "$distributor/ardents-linux-amd64" "$distributor/1.root.json"
printf '%s\n' \
  'schema=ardents-alpha-bootstrap-v1' \
  "release=$expected_release" \
  "platform=$expected_platform" \
  'artifact=ardents-linux-amd64' \
  'trusted_root=1.root.json' >"$distributor/RELEASE"
(
  cd "$distributor"
  LC_ALL=C sha256sum RELEASE ardents-linux-amd64 1.root.json >SHA256SUMS
)
ssh-keygen -Y sign -f "$publisher/bootstrap-key" -n "$namespace" \
  "$distributor/SHA256SUMS" >/dev/null 2>&1
cp -- "$publisher/bootstrap-key.pub" "$distributor/bootstrap.pub"

cp -- "$distributor/RELEASE" "$distributor/SHA256SUMS" \
  "$distributor/SHA256SUMS.sig" "$distributor/bootstrap.pub" \
  "$distributor/ardents-linux-amd64" "$distributor/1.root.json" "$participant/"
chmod 600 "$participant"/*
actual_fingerprint=$(ssh-keygen -lf "$participant/bootstrap.pub" -E sha256 |
  awk '{print $2}')
[[ $actual_fingerprint == "$expected_fingerprint" ]]
printf '%s namespaces="%s" %s\n' "$principal" "$namespace" "$public_fields" \
  >"$participant/allowed_signers"

verify_signature() {
  local directory=$1 identity=$2 signature_namespace=$3
  ssh-keygen -Y verify -f "$participant/allowed_signers" -I "$identity" \
    -n "$signature_namespace" -s "$directory/SHA256SUMS.sig" \
    <"$directory/SHA256SUMS" >/dev/null 2>&1
}

verify_signature "$participant" "$principal" "$namespace"
(
  cd "$participant"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
expected_descriptor=$(printf '%s\n' \
  'schema=ardents-alpha-bootstrap-v1' \
  "release=$expected_release" \
  "platform=$expected_platform" \
  'artifact=ardents-linux-amd64' \
  'trusted_root=1.root.json')
[[ $(cat "$participant/RELEASE") == "$expected_descriptor" ]]

cp -- "$participant/ardents-linux-amd64" "$root/artifact.good"
printf 'changed\n' >>"$participant/ardents-linux-amd64"
if (cd "$participant" && sha256sum --check --strict SHA256SUMS >/dev/null 2>&1); then
  echo 'changed artifact unexpectedly matched checksums' >&2
  exit 67
fi
mv -- "$root/artifact.good" "$participant/ardents-linux-amd64"

cp -- "$participant/1.root.json" "$root/root.good"
printf 'changed\n' >>"$participant/1.root.json"
if (cd "$participant" && sha256sum --check --strict SHA256SUMS >/dev/null 2>&1); then
  echo 'changed TUF root unexpectedly matched checksums' >&2
  exit 68
fi
mv -- "$root/root.good" "$participant/1.root.json"

cp -- "$participant/SHA256SUMS" "$root/sums.good"
printf '# changed signed bytes\n' >>"$participant/SHA256SUMS"
if verify_signature "$participant" "$principal" "$namespace"; then
  echo 'changed checksum bytes unexpectedly verified' >&2
  exit 69
fi
mv -- "$root/sums.good" "$participant/SHA256SUMS"

if verify_signature "$participant" 'wrong@ardents.network' "$namespace"; then
  echo 'wrong principal unexpectedly verified' >&2
  exit 70
fi
if verify_signature "$participant" "$principal" 'wrong@ardents.network'; then
  echo 'wrong namespace unexpectedly verified' >&2
  exit 71
fi

ssh-keygen -q -t ed25519 -N '' -C 'R-095 attacker fixture' \
  -f "$attacker/bootstrap-key"
cp -- "$participant/SHA256SUMS" "$attacker/SHA256SUMS"
ssh-keygen -Y sign -f "$attacker/bootstrap-key" -n "$namespace" \
  "$attacker/SHA256SUMS" >/dev/null 2>&1
if verify_signature "$attacker" "$principal" "$namespace"; then
  echo 'wrong signing key unexpectedly verified' >&2
  exit 72
fi
attacker_fingerprint=$(ssh-keygen -lf "$attacker/bootstrap-key.pub" -E sha256 |
  awk '{print $2}')
[[ $attacker_fingerprint != "$expected_fingerprint" ]]

cp -- "$participant/ardents-linux-amd64" "$replay/ardents-linux-amd64"
cp -- "$participant/1.root.json" "$replay/1.root.json"
printf '%s\n' \
  'schema=ardents-alpha-bootstrap-v1' \
  'release=ardents-alpha-0000' \
  "platform=$expected_platform" \
  'artifact=ardents-linux-amd64' \
  'trusted_root=1.root.json' >"$replay/RELEASE"
(
  cd "$replay"
  LC_ALL=C sha256sum RELEASE ardents-linux-amd64 1.root.json >SHA256SUMS
)
ssh-keygen -Y sign -f "$publisher/bootstrap-key" -n "$namespace" \
  "$replay/SHA256SUMS" >/dev/null 2>&1
verify_signature "$replay" "$principal" "$namespace"
(
  cd "$replay"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
if [[ $(cat "$replay/RELEASE") == "$expected_descriptor" ]]; then
  echo 'older signed release unexpectedly matched expected descriptor' >&2
  exit 73
fi

if find "$participant" -type f -name 'bootstrap-key' -print -quit | grep -q .; then
  echo 'private key entered participant directory' >&2
  exit 74
fi
artifact_digest=$(sha256sum "$participant/ardents-linux-amd64" | awk '{print $1}')
root_digest=$(sha256sum "$participant/1.root.json" | awk '{print $1}')
sums_digest=$(sha256sum "$participant/SHA256SUMS" | awk '{print $1}')
openssh_version=$(ssh -V 2>&1 | tr ' ' '_')

cleanup
trap - EXIT
[[ $cleanup_failed -eq 0 && ! -e $root && ! -L $root ]]
printf '{"schema":"ardents-r095-ubuntu-bootstrap-v1","passed":true,'
printf '"openssh":"%s","fingerprint":"%s","release":"%s","platform":"%s",' \
  "$openssh_version" "$expected_fingerprint" "$expected_release" "$expected_platform"
printf '"artifact_sha256":"%s","root_sha256":"%s","sums_sha256":"%s",' \
  "$artifact_digest" "$root_digest" "$sums_digest"
printf '"signature_valid":true,"hashes_valid":true,"descriptor_valid":true,'
printf '"changed_artifact_rejected":true,"changed_root_rejected":true,'
printf '"changed_signed_bytes_rejected":true,"wrong_principal_rejected":true,'
printf '"wrong_namespace_rejected":true,"wrong_key_rejected":true,'
printf '"substituted_key_fingerprint_rejected":true,"signed_replay_rejected":true,'
printf '"participant_private_key_absent":true,"cleanup_complete":true}\n'
