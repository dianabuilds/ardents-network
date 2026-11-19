# Closed-alpha Ubuntu Portable enrollment

Status: **closed-alpha portable Endpoint instruction, exercised on 2026-08-28 by the
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
Do not add a file to this directory: the manifest inventory is exact.

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

## Start (per-user route retired)

The former per-user `endpoint user-unit` / `endpoint enroll` start procedure is
retired (#617 S3.5); `ardents endpoint user-unit/enroll/enroll-installed/
installed-user-unit/replace/rollback` return typed refusals before effects. The
external verification above still applies before running a downloaded program.

First execution now uses platform delivery: `ardents-next platform unit` with
`endpoint construct-plan` and `prepare-history`, then `ardents-next endpoint
start-installed` under the platform system unit (see
[Endpoint service runtime](../technical/endpoint-service-runtime.md) and
[commands](commands.md)). The successor Endpoint lifecycle is forward-only;
there is no per-user session unit, automatic replacement, repair, or destructive
state removal. Existing per-user roots and floors remain retained evidence, not
authority for an automatic migration.
