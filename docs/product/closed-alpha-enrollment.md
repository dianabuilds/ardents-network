# Closed-alpha Ubuntu Portable enrollment

Status: **H4-1A participant instruction, exercised on 2026-08-28 by the
Product Owner's own authenticated first-enrollment walkthrough.** This is a
closed-alpha procedure, not an independent-participant, public-release,
platform-support, or anonymous-onboarding claim.

## What must arrive independently

The Product Owner sends the invited participant, through an already
authenticated contact that is independent of the artifact download, one Alpha
Enrollment Pin:

- cohort;
- release;
- platform (`linux-amd64` for this profile); and
- lowercase SHA-256 digest of the bundle's `SHA256SUMS` file.

The invitation also names the artifact download and the expected bundle
directory. A digest copied only from a release page is not an Enrollment Pin.
The Product Owner must name the actual contact class before the first real
invite. For the current alpha candidate, the approved class is an
**authenticated direct message from the Product Owner to the participant**;
the release page, GitHub, HTTPS, and downloaded executable are not sources of
first-install trust. This declaration is not evidence that an independent
participant has received or enacted an invitation.

## Verify before execution

These commands use Ubuntu's preinstalled POSIX shell, `sha256sum`, `find`,
`sed`, `sort`, and `cmp`; they do not execute the downloaded binary. Substitute
the independently received digest and the participant's absolute bundle path.
Do not add the Enrollment Input to this directory: the manifest inventory is
exact.

```sh
set -eu
bundle=/absolute/path/to/unpacked-bundle
pin=64-lowercase-hex-characters-from-the-invitation
cd "$bundle"

actual=$(sha256sum SHA256SUMS | cut -d ' ' -f1)
test "$actual" = "$pin"

expected=$(mktemp)
actual_names=$(mktemp)
trap 'rm -f "$expected" "$actual_names"' EXIT HUP INT TERM

# Reject a directory, symlink, device, or other non-regular top-level entry.
if find . -mindepth 1 -maxdepth 1 ! -type f -print -quit | grep -q .; then
  echo 'bundle has a non-regular top-level entry' >&2
  exit 1
fi

# The bundle must contain exactly the manifest entries plus SHA256SUMS itself.
LC_ALL=C sed -n 's/^[0-9a-f]\{64\}  //p' SHA256SUMS >"$expected"
printf '%s\n' SHA256SUMS >>"$expected"
LC_ALL=C sort -o "$expected" "$expected"
LC_ALL=C find . -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort >"$actual_names"
cmp -s "$expected" "$actual_names"

# Only now parse and check every declared byte.
sha256sum --strict --check SHA256SUMS
```

Any non-zero result stops the journey. Do not mark the artifact executable, run
`ardents`, start a unit, or seek a workaround. The built-in
`ardents endpoint enrollment-check` is deliberately absent from this procedure:
it can recheck the inventory before Endpoint readiness only after the artifact
has already executed.

## Create the local enrollment input

After the preceding verification succeeds, read the non-secret values in the
verified `RELEASE` descriptor and create the input outside the bundle. The
invitation's cohort/release/platform/manifest digest must be copied exactly;
the descriptor's `environment`, `network`, and `target_path` must agree with
the values below. This file contains no Authority and no private key.

```sh
set -eu
umask 077
input_home="$HOME/.local/state/ardents-alpha/declared-release"
mkdir -p "$input_home"
chmod 700 "$input_home"
cat >"$input_home/alpha-enrollment.json" <<'EOF'
{
  "schema": "ardents-alpha-enrollment-input-v1",
  "bundle_root": "/absolute/path/to/unpacked-bundle",
  "cohort": "declared-cohort",
  "release": "declared-release",
  "platform": "linux-amd64",
  "manifest_sha256": "64-lowercase-hex-characters-from-the-invitation",
  "environment": "verified-environment",
  "network": "verified-network",
  "target_path": "ardents/linux-amd64/endpoint"
}
EOF
chmod 600 "$input_home/alpha-enrollment.json"
```

The input is a one-bundle local transcription, not a release root and not
authority to accept successors. It cannot be stored inside the bundle without
causing the exact inventory check to fail.

## Explicit Ubuntu user-session start

Only after the external check succeeds, mark the exact artifact executable and
render the unit. Replace the command path with the verified artifact path.

```sh
chmod 700 /absolute/path/to/unpacked-bundle/ardents-linux-amd64
mkdir -p ~/.config/systemd/user
/absolute/path/to/unpacked-bundle/ardents-linux-amd64 endpoint user-unit \
  "$HOME/.local/state/ardents-alpha/declared-release/alpha-enrollment.json" \
  > ~/.config/systemd/user/ardents-endpoint.service
chmod 600 ~/.config/systemd/user/ardents-endpoint.service
systemctl --user daemon-reload
systemctl --user enable --now ardents-endpoint.service
```

This is a per-user session unit. It neither enables linger nor creates a system
service, startup task, system proxy, DNS/route/VPN setting, browser integration,
remote listener, or Authority. `journalctl --user -u ardents-endpoint.service`
must show `endpoint-lifecycle` `starting`, then `release-decision`, then
`endpoint-lifecycle` `ready`. A terminal `blocked` or `incompatible` event is
an explicit failed start, not a partly ready Endpoint.

Routine stop and restart are explicit:

```sh
systemctl --user stop ardents-endpoint.service
systemctl --user start ardents-endpoint.service
```

Deleting the stopped bundle directory removes only program bytes. It does not
remove the per-user Vault, release floors, grants, diagnostics, cache, or live
state roots. There is no supported automatic replacement, repair, or destructive
state removal in H4-1A; those operations belong to H4-1B and H4-1C.
