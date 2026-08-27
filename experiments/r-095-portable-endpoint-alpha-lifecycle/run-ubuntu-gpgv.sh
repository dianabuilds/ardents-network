#!/usr/bin/env bash
set -euo pipefail

root=/tmp/ardents-r095-gpgv-20260824

if [ -e "$root" ]; then
  echo "refusing occupied experiment root: $root" >&2
  exit 64
fi

cleanup() {
  gpgconf --homedir "$root/gnupg" --kill all >/dev/null 2>&1 || true
  if [ "$(realpath "$root")" = "$root" ]; then
    rm -rf -- "$root"
  fi
}
trap cleanup EXIT

mkdir -m 700 "$root"
mkdir -m 700 "$root/gnupg"
export GNUPGHOME="$root/gnupg"

printf 'R-095 harmless portable artifact v1\n' >"$root/artifact.bin"
gpg --batch --passphrase '' --quick-generate-key \
  'R-095 Fixture <r095@example.invalid>' ed25519 sign 0
fingerprint=$(gpg --batch --with-colons --list-keys 'r095@example.invalid' |
  awk -F: '$1 == "fpr" { print $10; exit }')

gpg --batch --export "$fingerprint" >"$root/trustedkeys.gpg"
gpg --batch --armor --detach-sign --local-user "$fingerprint" \
  --output "$root/artifact.bin.asc" "$root/artifact.bin"

gpgv --keyring "$root/trustedkeys.gpg" "$root/artifact.bin.asc" \
  "$root/artifact.bin"
verified_status=$?
before_digest=$(sha256sum "$root/artifact.bin" | awk '{ print $1 }')

printf 'R-095 harmless portable artifact v1 -- changed\n' >"$root/artifact.bin"
if gpgv --keyring "$root/trustedkeys.gpg" "$root/artifact.bin.asc" \
  "$root/artifact.bin"; then
  echo 'changed artifact unexpectedly verified' >&2
  exit 65
else
  rejected_status=$?
fi
after_digest=$(sha256sum "$root/artifact.bin" | awk '{ print $1 }')

printf 'gpgv_version=%s\n' "$(gpgv --version | head -n 1)"
printf 'verified_status=%s\n' "$verified_status"
printf 'changed_rejection_status=%s\n' "$rejected_status"
printf 'before_sha256=%s\n' "$before_digest"
printf 'after_sha256=%s\n' "$after_digest"

test "$verified_status" -eq 0
test "$rejected_status" -ne 0
