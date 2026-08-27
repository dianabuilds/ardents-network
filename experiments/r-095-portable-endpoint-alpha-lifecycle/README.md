# R-095 — Portable verification and Ubuntu user lifecycle

## Question

Can an Ubuntu LTS alpha participant verify and run a Portable artifact as an
unprivileged `systemd --user` service, then stop, restart, and manually replace
it without losing protected state or activating unverified bytes? Separately,
can Windows reject a changed signed fixture without pretending that the first
Ardents Windows artifact is signed?

## Hypothesis

`gpgv` with an explicitly supplied keyring accepts the untouched signed fixture
and rejects the same signature after the artifact bytes change. A real Ubuntu
user unit can expose capability readiness through an owned Unix socket, retain
a synthetic durable state identity across stop/restart and a verified v1→v2
replacement, refuse replacement while active, and leave the current executable
unchanged after corrupt or pre-commit-interrupted candidates. Windows
PowerShell's `Get-AuthenticodeSignature` reports `Valid` for a known signed
system fixture and a non-`Valid` status after a copied fixture changes. The
closed-alpha comparison additionally expects one manifest digest independently
of distribution and rejects changed, substituted, or replayed bundles before
execution without introducing a signing key. The experiments do not establish
a real independent enrollment channel, TUF validity for an alpha artifact,
real Authority durability, logout/reboot behavior, or a public release-control
claim.

## Predeclared Ubuntu lifecycle oracle

The lifecycle runner uses one ordinary Ubuntu user and creates only uniquely
named disposable paths. Its fixture has no network route, Service Authority,
Vault, Local Grant, or real release key. Before the first run it must record the
Ubuntu/systemd version, UID, XDG roots, and current `Linger` value. It fails if
run as root, if the user manager is unavailable, if the fixed experiment unit
or any fixed fixture root already exists, or if `XDG_RUNTIME_DIR` is absent,
relative, not owned by the user, or not mode `0700`.

The passing matrix requires all of the following:

1. Cross-built v1 and v2 Linux/amd64 fixtures have different recorded SHA-256
   digests. An ephemeral experiment-only Ed25519 OpenPGP key signs both;
   `gpgv` accepts each untouched candidate and rejects a changed v2 copy.
2. The participant program, config, state, cache, and runtime roots are distinct
   owner-only directories. The service unit is a real file under the user's
   systemd configuration root, is explicitly enabled for `default.target`, and
   starts without `sudo`, a system service, or a `loginctl` mutation.
3. Readiness is not inferred from process state. After `systemctl --user start`,
   the same fixture performs an exact request/response over its owned Unix
   socket and reports build ID, stable synthetic state ID, monotonic start
   count, and fixed floor.
4. A normal stop removes only runtime readiness/socket state. Starting v1 again
   reports the same state ID/floor and a higher start count.
5. The checked replacement operation refuses while the unit is active. A
   corrupt candidate fails `gpgv`. A deliberate interruption after verified
   staging but before atomic rename leaves the installed v1 digest unchanged.
6. With the unit stopped, the same verified staging/rename operation activates
   v2. Its readiness proof reports v2, the original state ID/floor, and the next
   start count. The old build is not restarted accidentally.
7. Explicit stop/disable followed by removal of only the program root leaves
   synthetic protected state present. The runner then removes all of its own
   exact fixture paths and unit links and proves that `Linger` is unchanged.

The hypothesis is falsified by any elevation, implicit linger/autostart change,
unit-active replacement, accepted corrupt candidate, non-atomic partial
activation, changed/lost durable state, readiness inferred only from an active
PID, runtime residue after stop, or cleanup outside the declared paths.

## Predeclared closed-alpha enrollment-digest oracle

This slice compares the SSHSIG candidate below with a smaller trust input for
the first invited alpha cohort. The participant receives one exact SHA-256 of
`SHA256SUMS`, plus the expected cohort, release, platform, environment, network,
and target path, through an already authenticated contact with the Product
Owner that is independent of the GitHub Release download. The downloaded
`SHA256SUMS` in turn binds the exact `RELEASE` descriptor, Portable artifact,
and initial TUF-root bytes. No Ardents executable, downloaded script, public
key, certificate, or new package is trusted to establish this first fact.

The fixture models the independent input as values supplied directly to the
participant side rather than files copied through the distributor. It uses
only preinstalled `sha256sum` and shell comparison before the harmless artifact
could run. Its passing oracle requires:

1. the untouched manifest digest, strict file hashes, and exact descriptor all
   match;
2. a changed manifest, artifact, root, or descriptor fails before execution;
3. a completely substituted bundle with internally consistent hashes still
   fails the independently pinned manifest digest;
4. an internally consistent older release fails both the pinned manifest
   digest and the exact expected descriptor;
5. before content checking, the participant requires the exact four-entry
   fixture inventory as owner-only, single-link regular files and rejects an
   extra entry, a missing entry, or a same-content artifact symlink; and
6. the artifact is never executed and every exact fixture path is removed.

The hypothesis is falsified if any changed or self-consistent substituted
bundle satisfies the pinned input, if a same-content symlink or unexpected
inventory is accepted, if replay prevention relies on GitHub or on the bundle's
own claim, if verification needs a project binary, network service, package
installation, or elevation, or if cleanup escapes the exact fixture root.
Passing cannot prove that a real invitation channel is
independent or authenticated, that the TUF metadata is valid, that a first
artifact is non-malicious, or that this manual cohort flow scales beyond a
closed alpha. It tests whether a second reusable bootstrap signing key is
mechanically unnecessary for that bounded journey.

## Predeclared first-artifact bootstrap oracle

This separate slice asks whether tools already present on the selected Ubuntu
profile can authenticate the *first* Portable bundle before any Ardents binary
runs, while keeping the existing TUF-compatible Release Module authoritative
after bootstrap. It does not ask a downloaded Ardents verifier to authenticate
itself.

The publisher fixture creates a dedicated Ed25519 OpenSSH signing key, an exact
`RELEASE` descriptor, a Linux/amd64 fixture artifact, and a copy of the current
public TUF test root. It signs one `SHA256SUMS` file with the unambiguous
`ardents-alpha-bootstrap-v1@ardents.network` SSH signature namespace. The
participant fixture starts only with an expected key fingerprint, release ID,
and platform that model values delivered outside the release download channel.
It constructs its own namespace-limited `allowed_signers` entry, verifies the
signature, compares the exact descriptor, and runs GNU `sha256sum --check
--strict` before the artifact is executable.

The passing oracle requires:

1. untouched signature, exact principal, exact namespace, expected Ed25519 key
   fingerprint, descriptor, artifact, and TUF-root hashes all pass;
2. a changed artifact, changed descriptor/checksum bytes, wrong principal,
   wrong namespace, signature from another key, and substituted bootstrap key
   all fail before execution;
3. a fully valid older bundle signed by the same legitimate key passes its
   cryptographic/hash checks but fails the independently expected release-ID
   comparison, making the manual anti-replay input visible rather than implied;
4. no project executable is used as verifier, no private key enters the
   participant directory, and every exact fixture path is removed.

The slice is falsified if `ssh-keygen` accepts a wrong namespace/principal/key or
changed signed bytes, if `sha256sum` accepts changed target/root bytes, if an
older signed bundle can satisfy the expected-release check, or if verification
needs a project binary, network service, package installation, or elevation.
It cannot establish how the participant learns the initial fingerprint/release
ID, private-key custody, TUF metadata validity, independent build provenance,
or a Public Beta bootstrap.

## Run

From Ubuntu 24.04 or another declared Ubuntu LTS host with `gpg` and `gpgv`:

```sh
bash run-ubuntu-gpgv.sh
```

The script refuses to run if `/tmp/ardents-r095-gpgv-20260824` already exists.
It generates a fixture-only signing key without passphrase in that exact
directory, verifies an artifact, changes its bytes, verifies rejection, prints
the two result codes and SHA-256 digests, then removes only that exact directory.
It never reads or writes an Ardents Authority, release root, or Endpoint state.

The complete lifecycle slice is orchestrated from the repository's Windows
host and runs inside the declared `Ubuntu-24.04` WSL distribution whose user
manager is already running:

```powershell
pwsh -NoProfile -File .\run-ubuntu-lifecycle.ps1
```

The PowerShell wrapper cross-builds two build-ignored Linux/amd64 fixture
binaries with `CGO_ENABLED=0` and `-trimpath` into the host temporary directory,
passes them to the Ubuntu runner, and deletes only those exact binaries in
`finally`. The Ubuntu runner copies them into its private incoming area, creates
the experiment key/unit/profile, executes the predeclared matrix, prints one
`ardents-r095-ubuntu-lifecycle-v1` result, and cleans its exact paths in a trap.

The first-artifact bootstrap slice also runs in `Ubuntu-24.04` WSL. It
cross-builds the harmless v1 fixture, binds the current public TUF test-root
bytes, and leaves both host and guest clean:

```powershell
pwsh -NoProfile -File .\run-ubuntu-ssh-bootstrap.ps1
```

The simpler closed-alpha enrollment-digest comparison runs in the same profile
and binds the same artifact/root inputs without generating a signing key:

```powershell
pwsh -NoProfile -File .\run-ubuntu-cohort-digest.ps1
```

From Windows PowerShell, on a host that contains the Microsoft-signed
`%WINDIR%\System32\wsl.exe` fixture:

```powershell
pwsh -NoProfile -File .\run-windows-authenticode.ps1
```

The Windows script copies that fixture into the exact temporary directory
`$env:TEMP\ardents-r095-authenticode-20260824`, changes one byte only in the
copy, prints the original and changed signature status plus SHA-256 digests,
then removes that exact directory. It does not sign anything and never alters
the Windows binary or an Ardents artifact.

## Evidence

Capture the OS/version, command output, successful-verification status,
rejection status, artifact digests, unit state, readiness proofs, state identity
and counter, path modes, linger before/after, and cleanup result in R-095. A
successful run falsifies neither the bootstrap-key problem, real-release
metadata/recovery, logout/reboot behavior, Ubuntu Desktop qualification, nor
the Windows signing/reputation requirement.

## Captured Ubuntu lifecycle evidence — 2026-08-24

- The first wrapper invocation stopped before starting the Ubuntu matrix because
  WSL received a Windows path with its backslashes stripped. Its PowerShell
  `finally` removed both host temporary binaries. Passing slash-form paths to
  `wslpath` corrected this orchestration defect; it was not a systemd or
  Endpoint lifecycle failure.
- Three complete runs as UID 1000 on Ubuntu 24.04 WSL, systemd
  `255.4-1ubuntu8.14`, passed the exact matrix. `Linger` was `no` before and
  after each run. The user unit reported `enabled`; config, state, cache, and
  runtime fixture roots were `0700`, and the unit file was `0600`.
- All three runs used reproducible cross-built fixture digests
  `cd46e8c6ab957d2ae117445922a1a102f804202fe80b32cddc9e1eec94a7948e`
  for v1 and
  `d17c16b11e0cd8998fd881384f4bae1ca331d940bcd113850a0777ebf033d3e5`
  for v2. Each run generated a new synthetic state ID, then returned that same
  ID and floor 7 from socket readiness proofs with start counts 1, 2, and 3
  across same-build restart and verified v1→v2 replacement.
- Every run refused the checked replacement while the unit was active, rejected
  a mutated v2 under the untouched signature, and preserved the installed v1
  digest after an injected stop following verified staging but before rename.
  A stopped verified replacement activated v2. A `0755` state root failed with
  `state-root-permissions`. Removing only the stopped/disabled program root left
  the synthetic state file present.
- Each runner reported `cleanup_complete=true`; a separate post-run audit found
  no experiment unit, enablement link, Linux fixture root/profile roots, or host
  temporary binaries.

The measurement supports the small Ubuntu lifecycle shape, not a product
implementation. The replacement guard belongs to the checked participant flow;
Linux still permits a user to overwrite or unlink their own running executable
outside that flow. `systemctl active` is likewise not capability readiness—the
socket proof supplied that distinction. No logout/user-manager restart, reboot,
disk-full, kill during the atomic commit, actual Ardents state, release-key
bootstrap, or Ubuntu Desktop host was exercised.

## Captured closed-alpha enrollment-digest evidence — 2026-08-24

- The first host invocation could not enter WSL because the managed sandbox
  denied access to the WSL service. The wrapper consequently failed while
  translating its first path and removed its host temporary binary in
  `finally`; no guest experiment root was created. This is an execution-
  environment refusal, not an enrollment result.
- Three permitted runs first passed the predeclared content/substitution matrix
  without retry inside a run. Review then identified that matching content
  alone did not reject a symlink to the same artifact bytes or an unexpected
  neighboring entry. The inventory/type/owner/mode/link-count falsifiers above
  were recorded before those new cells ran. Three final runs passed the
  expanded matrix without retry. All six successful runs produced the same
  manifest SHA-256
  `0c799e4a573727197c71e56298a7c13551ce6a6b05d6937f21dde398fbf2ee88`,
  descriptor SHA-256
  `ed0dfeafd7591fb542b1664c4c35061320048dc013cb9b86d244075a9f819cdf`,
  artifact SHA-256
  `cd46e8c6ab957d2ae117445922a1a102f804202fe80b32cddc9e1eec94a7948e`,
  and initial test-root SHA-256
  `246c88b483ccb15982710fa661f7e456f9361f95c2529df9d60082c5c35c59fd`.
- Each final participant flow first compared the independently modelled
  manifest digest, required the exact four owner-only single-link regular-file
  inventory, then parsed that manifest with strict file-hash checking, and
  finally compared the exact cohort/release/platform/environment/network/target
  descriptor. An extra entry, missing root, and same-content artifact symlink
  failed, as did changed manifest, artifact, root, and descriptor bytes, before
  the artifact could run.
- A distributor-created substitute with changed artifact bytes and internally
  correct replacement hashes passed its own strict checks but failed the pinned
  manifest digest. A complete older-release bundle behaved the same way and
  also failed the expected descriptor. This is the critical distinction from a
  checksum copied from the release page: the trusted input was not supplied by
  the distributor.
- The artifact was never executed, no private key or project verifier existed
  in the flow, and all three runs reported exact guest and host cleanup.

The result removes a mechanical reason to introduce a reusable SSHSIG key for
the first closed cohort. Both candidates still require one authenticated input
outside GitHub; a fingerprint delegates authority over more bundle bytes and
adds private-key custody, while the manifest digest authorizes only one exact
bundle. The cost is deliberate: every newly invited cohort/release needs a new
independent pin. That does not scale to a public release, but it matches the
declared closed-alpha scope.

## Captured first-artifact bootstrap evidence — 2026-08-24

- On the same Ubuntu 24.04 environment, OpenSSH
  `9.6p1 Ubuntu-3ubuntu13.15`, `sha256sum`, and `gpgv` were already present;
  `gh`, `cosign`, `minisign`, and `signify-openbsd` were absent. This is one
  profile observation, not a promise about every Ubuntu installation.
- The first wrapper run stopped before creating a participant result because it
  named the maintained public TUF fixture `1.root.json` at its repository path,
  while the current source file is `root.json`. The guest trap and host
  `finally` removed their temporary roots. Correcting only that source path
  retained the bundle name `1.root.json`; this was an orchestration failure.
- Three subsequent runs generated fresh Ed25519 bootstrap keys and passed every
  predeclared result. The fixture artifact SHA-256 remained
  `cd46e8c6ab957d2ae117445922a1a102f804202fe80b32cddc9e1eec94a7948e`,
  the bound public TUF test-root SHA-256 was
  `246c88b483ccb15982710fa661f7e456f9361f95c2529df9d60082c5c35c59fd`,
  and the exact `SHA256SUMS` SHA-256 was
  `ff4cceb6534c22e746bda799117814e59e3cd6d09bb8f4b5580b31cdddee54ef`.
- Each participant verified an independently expected SHA-256 key fingerprint,
  exact principal, and namespace before strict checksum validation. Changed
  artifact bytes, changed TUF-root bytes, changed signed checksum bytes, wrong
  principal, wrong namespace, a different signing key, and a substituted public
  key all failed. No publisher private key entered the participant directory.
- A complete `ardents-alpha-0000` bundle signed by the same legitimate key
  passed SSH signature and checksum verification, then failed the independently
  expected `ardents-alpha-0001` descriptor comparison. The signature alone is
  therefore not an anti-replay rule; the alpha invitation/bootstrap input must
  name the exact expected release and platform.
- All three successful runs and the initial path failure cleaned the exact WSL
  fixture root and host binary. A separate post-run audit found no residue.

The experiment shows that a preinstalled-tool bootstrap can remain small, but
also exposes its authority: whoever controls the OpenSSH bootstrap private key
can authorize arbitrary first-run bytes. Signing a TUF-root digest beside the
artifact does not make the downloaded executable enforce TUF before it runs.
Using a separate bootstrap key therefore requires an explicit alpha bootstrap
decision or an external already-trusted TUF verifier; it cannot be described as
mere checksum convenience under ADR-0015.

## Result

The lifecycle hypothesis survives this local Ubuntu slice. A Portable artifact plus an
explicit user unit and a narrow lifecycle operation can implement the required
obtain/verify/start/stop/restart/manual-replacement journey without elevation,
linger mutation, installer, or protected-state placement in the program root.
The maintained design still needs one owner for verify/stage/active-state/
atomic-commit sequencing and must use Endpoint capability readiness rather than
systemd process state.

The first-artifact hypothesis also survives mechanically: OpenSSH SSHSIG plus
strict checksums can authenticate an exact release descriptor, artifact, and
initial TUF-root bytes using only selected-profile tools. It does **not** resolve
the trust bootstrap. The expected fingerprint, release ID, and platform must
arrive outside the GitHub release download. The Product Owner instead selected
the narrower one-bundle digest below, so the reusable SSHSIG authority remains
feasibility evidence only.

The narrower enrollment-digest hypothesis also survives and was selected for
the first closed cohort on 2026-08-24: the same necessary independent channel can pin one
exact manifest instead of delegating first-execution authority to a reusable
key. The first trusted Endpoint must still evaluate the bundled TUF root,
metadata, and its exact artifact through the maintained Release Decision Module
before reporting network readiness. That handoff cannot retroactively protect
against a malicious first artifact authorized by the pin; it makes TUF and
durable floors authoritative for successors.

## Disposition

Disposable research harness for a decided question. The Alpha Enrollment Pin,
Ubuntu lifecycle direction, and Endpoint/Release handoff are promoted. Retain
only until this unique measurement enters source history, then let maintained
H4-1A behavior and qualification tests supersede it. Do not turn the fixture,
ephemeral GPG/SSHSIG keys, or shell orchestration into the product lifecycle by
cleanup.
