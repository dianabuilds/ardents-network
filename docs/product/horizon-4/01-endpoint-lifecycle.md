# H4-1 — Endpoint lifecycle and distributable profiles

Status: **H4-1A functional-alpha gate passed on 2026-08-28 for one immutable
Ubuntu Portable prerelease and the Product Owner's own authenticated
first-enrollment lifecycle. Foreground replacement, custody, and the direct
Ubuntu `.deb` retain their existing qualification. Independent-participant,
Windows, and Public Beta release claims remain explicitly unclaimed.**

## Decision

H4-1 starts with one small delivery result, not a full installer, updater, and
recovery product:

> A participant can obtain one authenticated Ardents alpha build, run an unprivileged
> local Endpoint, stop it, restart it with its protected local state intact,
> and understand that it is an alpha build rather than a supported Public Beta
> release.

This is the distribution prerequisite for a usable network alpha. It does not
authorize automatic updates, repair, destructive uninstall, background
contribution, browser integration, or a public release claim.

## Accepted alpha decisions

### Ubuntu-first real Portable profile

H4-1A first delivers an authenticated Portable artifact for Ubuntu LTS
`x86-64`. This is the release-gating alpha cell. It is a real Endpoint profile,
not a developer checkout or a feature-reduced edition. The artifact contains
the exact executable and only unavoidable non-secret static resources; it
carries no pre-created Authority, Local Grant, Node identity, or hidden network
state.

A Windows 11 `x86-64` binary may be published beside it as a visibly
best-effort experimental artifact. It does not block the Ubuntu alpha and does
not become a selected Windows platform claim merely because it compiles or can
be launched. The first Windows artifact is explicitly unsigned: the project
will not purchase an OV, EV, or hosted commercial code-signing service for an
alpha without users. A no-cost OSS Authenticode service may be evaluated later,
but release does not wait for it. Windows participants must be told that the
binary can trigger SmartScreen or Smart App Control and may be unrunnable under
local policy. Installing a self-signed project CA, weakening Windows security,
or instructing a participant to bypass an enforced policy is not an accepted
workaround.

The participant explicitly verifies the artifact before first run and after
replacement with a verifier that was already trusted before the downloaded
bytes execute. The Portable executable cannot authenticate its own first
execution: `ardents endpoint enrollment-check` is only a diagnostic that a
running process will perform the same pinned-inventory check before Endpoint
readiness. It is not a bootstrap verifier and must not be presented as one.
Alpha release identity and first-enrollment material are
provisional and visible; they are not threshold public-release control or
evidence of an independent build process. The intended first network
distribution channel is GitHub Releases, accompanied by immutable digests and
build provenance where the selected release workflow supports it. GitHub
availability, HTTPS, or a checksum copied only from the Release page is not the
release-verification rule.

The Product Owner accepted R-095's narrower first-enrollment profile instead of
a reusable signing key on 2026-08-24. For the first closed cohort, the Product
Owner independently sends
the invited participant one exact **Alpha Enrollment Pin**: manifest SHA-256,
cohort, release, and platform. The participant compares that digest before
parsing the downloaded manifest, then uses the manifest to verify the exact
descriptor, Portable artifact, initial TUF root, complete metadata, and every
declared static companion. Three Ubuntu fixture runs rejected changed bytes,
an internally self-consistent distributor substitution, and an older complete
bundle without running the artifact.

The first trusted Endpoint must then pass the same bytes and its executable to
the Release Decision Module and durably establish root/floors before reporting
network readiness. Only Release Decision authorizes successors. The enrollment
pin protects exact first-bundle bytes from a compromised distributor only while
the participant's existing Product Owner contact remains authentic; it does not
protect against a malicious pinned artifact or claim independent/public release
control. The actual independent contact class must be named before the first
participant is enrolled, but it is not a prerequisite for implementing the
bounded verification and Release Decision handoff. OpenSSH SSHSIG remains feasibility evidence, not
the default: its fingerprint needs the same independent delivery but
additionally delegates reusable signing power and creates key-custody and
rotation work.

### Unprivileged, per-user execution

The normal User/Publisher Endpoint never runs as root, a Windows Service, or a
machine-wide network daemon.

- **Ubuntu alpha:** the intended steady-state endpoint is a `systemd --user`
  service under the participant's own account. Enabling that unit is an explicit
  post-unpack action, not an implicit side effect of downloading or running a
  Portable artifact. It runs only for the user's session by default; no
  `linger`, system service, remote administration, or public listener is
  enabled by H4-1A.
- **Windows experimental companion:** where Windows permits execution, the
  participant explicitly launches the unprivileged Ardents binary in their
  user session. It remains a visible running process; no Windows Service,
  scheduled startup, elevation, background autostart, security-policy bypass,
  or trust-store mutation is part of this profile.

R-095's disposable Ubuntu 24.04 WSL slice established the intended
user-service shape. The maintained H4-1A/H4-1B qualifiers then ran under a
clean non-lingering Ubuntu VPS user session against real `systemd --user`:
Portable start/stop/restart and the successful/recovery replacement paths use
an Endpoint-owned Unix-socket readiness proof rather than systemd `active`.
This qualifies the stated Ubuntu alpha paths, not Windows, arbitrary Desktop
distributions, or an OS security boundary against the owning user overwriting
their own files.

H4-1D adds explicit, reversible Ubuntu package integration beside the Portable
profile; it wraps the same authenticated Endpoint rather than creating another
runtime product. Its direct `.deb` and explicit package enrollment do not
change the unprivileged lifecycle rule.

The Portable profile has one narrow lifecycle owner. Its supported replacement
operation checks Release verification, stops the fixed user unit, stages beside
the current executable, commits atomically only while stopped, self-tests the
exact successor, and only then starts it. These checks describe the supported
participant path, not an OS security boundary against the owning user manually
overwriting their own files.

### Protected state is outside the program artifact

The executable directory is replaceable. Per-user protected state is not. The
platform profile must keep these classes materially separate:

| State class | H4-1A rule |
|---|---|
| Executable and static resources | Replaced only as a complete verified artifact; deleting them does not delete user state. |
| Authority Vault and authority watermarks | Never created in the artifact and never erased by program replacement, disable, or uninstall. |
| Endpoint configuration, Local Grants, Instance Keys, and public Credentials | Per-user state; no artifact copy/import silently merges it. |
| Release, epoch, Namespace, freshness, generation, and rollback floors | Per-user protected state; no replacement may lower them. |
| Network and descriptor cache | Disposable only when its removal cannot erase an Authority, Grant, or floor. |
| Live Route and connection state | Ends at process stop; no alpha claim of session survival. |
| Diagnostics | Finite, local, and removable separately from Authority material. |

Exact Windows known folders, Ubuntu XDG paths, permissions, and native key
protection are platform decisions to record before implementation. They are not
left to the installer or a library default.

The selected H4-1A composition follows one logical layout with thin platform
adapters. Ubuntu places configuration under `XDG_CONFIG_HOME`, durable Endpoint
state under `XDG_STATE_HOME`, cache under `XDG_CACHE_HOME`, and live state plus
the attachment only under the absolute, owner-only `XDG_RUNTIME_DIR`; a missing
or unsuitable runtime directory fails rather than falling back to global
`/tmp`. Windows resolves `FOLDERID_LocalAppData` at runtime, separates
`config`, `vault`, `floors`, `diagnostics`, and `cache`, and uses a deliberately
short `Endpoint\r` live directory to preserve the declared `AF_UNIX` path
budget. Program bytes remain outside all of these roots.

Both platforms use the same generic local-attachment protocol over a filesystem
Unix socket. Ubuntu requires effective-user `0700` directories and a `0600`
socket; Windows requires protected current-user-only DACLs on the owned roots
and socket. A held `flock` on Ubuntu or `LockFileEx` on Windows establishes the
single live state owner. Only after acquiring it may restart classify and remove
the exact expected stale socket, then bind and retain the lock until shutdown.
Unexpected files, directories, links, reparse points, roots, or lock errors fail
explicitly and are not removed. There is no silent named-pipe or loopback
fallback.

R-101/R-102 passed ordinary start, concurrent refusal, forced termination,
guarded recovery, deterministic path-substitution, unexpected-entry
preservation, and access-policy round-trip on Windows 11 and Ubuntu 24.04 WSL.
The decision is a local per-user composition, not H4-7 isolation: it makes no
claim against an administrator or a malicious process sharing the account. A
purpose-owned Endpoint implementation, actual second-account denial, bounded
delayed-lock retry, and complete lifecycle qualification remain before a
supported platform claim.

### One release per alpha cohort

H4-1A is a fixed-release alpha. A participant may stop the Endpoint and replace
the Portable artifact only after verifying the new artifact, but H4-1A makes no
claim of in-place migration, automatic download, automatic rollback, repair,
or compatibility across arbitrary release generations. A selected alpha run
uses one declared artifact and profile; a required replacement is an explicit
new test condition.

This intentionally avoids exposing the existing technical Update module as a
premature end-user updater. It also prevents a failed release mechanism from
being hidden inside a browser, package manager, or background service.

### Authority loss is explicit

H4-1A may create or import Authority material only through the existing
Authority Custody boundary. It never sends root material to a package service,
browser, Node, or local Application Interface. The participant receives an
explicit alpha warning that loss of an unbacked Service Authority loses its
Target. An Authority Recovery Bundle, restore/reconciliation workflow, and
destructive Vault purge remain later H4-1 work unless separately selected and
tested.

## H4-1A: alpha distribution slice

### Entry conditions

- A declared alpha network configuration or bounded readiness fixture exists.
  It may point to the H4-2 live profile when that profile is selected, but
  H4-1A does not require H4-2 completion.
- The exact artifact inputs, Alpha Enrollment Pin, Release identity, target
  platform, network identity, and initial Release inputs are declared.
- The Endpoint has explicit `starting`, capability readiness, blocked, stale,
  incompatible, and stopped outcomes. A running process is not `ready` merely
  because a local listener exists.

### Participant journey

1. Obtain the declared Portable artifact and its independent verification
   material.
2. Before giving the artifact execute permission or invoking `ardents`, unpack
   it into a participant-chosen program directory and use the host's
   preinstalled `sha256sum` to compare the independently received Alpha
   Enrollment Pin with `SHA256SUMS`. Only after that exact manifest digest
   matches may the participant run `sha256sum --strict --check SHA256SUMS` and
   confirm that the top-level directory contains only the checked manifest
   entries and `SHA256SUMS`; a symlink, subdirectory, missing entry, or extra
   entry fails the supported path. This external check is the first-execution
   authorization. The subsequent Endpoint check repeats the bounded inventory
   and binds its own executable plus the same Root/metadata bytes to Release
   Decision before readiness.
3. Create an owner-only Alpha Enrollment Input **outside** the exact bundle.
   It is a participant-local transcription of the independently received
   cohort/release/platform/manifest digest plus the verified descriptor facts:

   ```json
   {
     "schema": "ardents-alpha-enrollment-input-v1",
     "bundle_root": "/absolute/path/to/unpacked-bundle",
     "cohort": "declared-cohort",
     "release": "declared-release",
     "platform": "linux-amd64",
     "manifest_sha256": "64-lowercase-hex-characters",
     "environment": "declared-environment",
     "network": "declared-network",
     "target_path": "ardents/linux-amd64/endpoint"
   }
   ```

   The input contains no Authority or secret. It must be `0600`, must not be
   copied into the bundle (which would invalidate its exact inventory), and is
   not a successor-release authorization. The invitation declares the Pin; the
   verified `RELEASE` descriptor supplies the remaining matching facts.
4. On Ubuntu, explicitly enable/start the per-user service; on Windows,
   explicitly start the binary.

   On Ubuntu the Portable binary renders, but never writes or enables, the
   exact unit for its absolute enrollment input:

   ```sh
   mkdir -p ~/.config/systemd/user
   /absolute/path/to/ardents endpoint user-unit /absolute/path/to/alpha-enrollment.json \
     > ~/.config/systemd/user/ardents-endpoint.service
   systemctl --user daemon-reload
   systemctl --user enable --now ardents-endpoint.service
   ```

   `systemctl --user stop ardents-endpoint.service` is the normal explicit
   stop. The unit has no `User=`, `linger`, restart loop, elevated permission,
   system-wide proxy/DNS, or automatic replacement action.
5. Observe the named Endpoint readiness or an explicit failure. No browser,
   URI handler, system proxy, DNS, route, VPN, Service publication, or
   Contributor role is created implicitly.
6. Stop the Endpoint and start the same artifact again. Protected state is
   retained; live work is closed and is not resumed invisibly.
7. Remove the executable directory if desired. Per-user protected state remains
   until a future explicitly confirmed cleanup operation exists.

### Done when

On Ubuntu, and on each later selected alpha platform, a clean user account can repeat the journey
from declared inputs. The evidence records artifact identity, verification
result, state-root ownership, first start, routine restart, stop, and the
observed readiness/failure result. Attempts to run an unverified artifact,
reuse another profile's protected state, make the Endpoint privileged, or erase
the Vault by deleting the program must fail or remain visibly outside the
supported path.

### Functional-alpha execution record — 2026-08-28

The exact source revision
`70bf425eec937edcc22e8f0534db992aa2002a16` produced Endpoint SHA-256
`33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1`.
The deterministic archive
`ardents-alpha-h4-alpha-1-rc-1-linux-amd64.tar.gz`, SHA-256
`e7ff0b26257978fd14bc3583c5de7d36eb7626bac7b43586bcb9442c53f7dba7`,
is the immutable GitHub prerelease
[`h4-alpha-1-rc-1`](https://github.com/dianabuilds/ardents-network/releases/tag/h4-alpha-1-rc-1).
Its Alpha Enrollment Pin is
`8ed0fd25c60a6988fcc8938baf86547c7c646744f57fb0c39186f184d13afefd`.

The Pin was delivered in the authenticated one-to-one Product Owner Codex task,
separately from GitHub, then enacted by the Product Owner on a clean
unprivileged Ubuntu `24.04.3 LTS` account. The pre-execution inventory, built-in
enrollment check, non-lingering user-unit start, retained-state stop/restart,
and final disabled/inactive cleanup all passed. This is the permitted Product
Owner walkthrough for a bounded tracer; it is not independent participant,
novice-usability, external-security, or Public Beta validation. Exact retained
receipts and their digests are owned by the
[H4-8A matrix](08a-alpha-1-readiness-matrix.md).

### Stop conditions

Stop this slice and revisit its profile if it requires a privileged daemon,
hidden autostart, a system-wide proxy/DNS/route change, copying Authority
material into the artifact, or an unsupported state migration merely to let a
participant start the Endpoint.

## Later H4-1 promotion slices

These remain part of the complete H4-1/Public Beta lifecycle, but are not
hidden prerequisites for H4-1A:

| Slice | Required result before promotion |
|---|---|
| H4-1B — safe replacement | Explicitly authorized staging, drain/stop, atomic activation, self-test, interrupted-update recovery, and rollback protection using the Release/Update boundary. |
| H4-1C — recovery and removal | Supported Authority Bundle export, isolated restore/reconciliation, repair, retained watermarks, and separately confirmed destructive purge. |
| H4-1D — Installed profiles | Native signed packages, optional explicit OS integration, repair, update, uninstall, and supported platform durability behavior, one platform at a time. |

No later slice may change the H4-1A rule that ordinary Endpoint runtime is
unprivileged and protected state is distinct from program bytes.

### H4-1B Ubuntu foreground replacement contract

H4-1B is a deliberate local operation, not a background updater. A participant
who has already completed H4-1A obtains one local replacement-bundle directory
and explicitly invokes:

```sh
/absolute/path/to/current/ardents endpoint replace \
  /absolute/path/to/replacement-bundle
```

The bundle has a canonical `REPLACEMENT` descriptor, the candidate executable,
the candidate root and TUF metadata needed by Release Decision, and no
Authority material. It has no URL, polling interval, repository setting, or
unit-name field. `endpoint replace` first proves that the currently executing
binary matches the durable selected-successor record established at successful
first enrollment; it then evaluates the supplied bytes against existing Release
floors. A public-looking Release decision, an unbound current executable, an
older floor, a foreign platform, or an artifact digest mismatch stops before
the user unit is stopped.

For the H4-1A alpha runtime there is no admitted Endpoint network work to
drain. The operation stages the authenticated candidate beside the program,
stops only `ardents-endpoint.service`, retains the predecessor in the
Endpoint-owned replacement state root, atomically renames the candidate over
the program path, and runs `endpoint replacement-self-test` from that new
binary. This self-test has no network route or Authority input: it proves the
candidate bytes match the durable **prepared** record. Only then is the record
promoted to **current** and `ardents-endpoint.service` explicitly started.

The state root retains `current`, `prepared`, an interruption journal, and the
immediate predecessor bytes separately from Release floors and Authority. A
self-test failure, a crash after activation, or a failed restart does not
infer a rollback from locally retained bytes. On a failed self-test the command
returns its exact owner-private `recovery_program`; it is the retained direct
predecessor executable, not a generic helper or a new daemon. The Owner gets a
new local bundle whose Release Decision freshly authorizes those exact retained
predecessor bytes, then invokes:

```sh
/absolute/path/from/recovery_program endpoint rollback \
  /absolute/path/to/fresh-predecessor-bundle
```

The recovery copy proves its own exact journal-bound location, restores only
the journal-bound original program path, and repeats stop, atomic activation,
no-network prepared-record self-test, and explicit start. A stale Bundle,
different recovery executable, arbitrary target path, or locally retained
bytes without the new Release authorization fails. Interrupted recovery stays
classified by the explicit `ardents endpoint replacement-recovery` observation
as `rollback-self-test-required`, `repair-required`, or an explicit committed
result; it never starts or rolls back automatically. H4-1B does not
add a downloader, daemon, linger, system service, Windows profile, package
repair, or public-release claim.

Only one completed immediate predecessor is retained. Before a later explicit
successor replacement, the foreground operation may retire that reserve only
when the ordinary current bytes and the completed journal agree exactly; any
failed or interrupted transaction remains fail-closed and cannot be retired by
starting another replacement.

### H4-1D Ubuntu direct package contract

H4-1D adds one Ubuntu `amd64` Installed alpha profile: a directly downloaded
`.deb`, initially distributed beside the Portable artifact rather than through
an APT repository. The participant independently checks the package digest and
provenance before the explicit `dpkg -i` action. That check is package
transport/bootstrap evidence, not Release authorization.

The package has no maintainer scripts, conffiles, system service, user unit,
XDG/home payload, package-controlled Endpoint state, automatic update policy,
or repository key. It owns only root-owned, non-user-writable program bytes at
`/usr/lib/ardents/ardents`, a `/usr/bin/ardents` launcher, and readable static
enrollment files at `/usr/share/ardents/enrollment/<package-version>/`.
Install, reinstall,
remove, and purge therefore cannot create a privileged runtime, enable linger,
or remove the Authority Vault, Authority floors, Release floors, cache, or
runtime state.

Before endpoint readiness, the participant creates an owner-only package
enrollment input outside `/usr` that names the static directory, exact direct
program path, cohort/release/platform, independently delivered manifest pin,
and local Release binding. They explicitly render/enable a user-session unit
with:

```sh
/usr/bin/ardents endpoint installed-user-unit \
  /absolute/path/to/package-enrollment.json \
  > ~/.config/systemd/user/ardents-endpoint.service
```

`endpoint enroll-installed` verifies the static manifest pin before parsing,
requires the installed direct executable to be root-owned and non-writable,
then passes its exact bytes to Release Decision. A wrong package payload,
user-writable artifact, substituted executable, unrecognized static inventory,
or Release rejection never reaches `ready`. Package replacement/upgrade does
not itself authorize Endpoint activation; the explicit enrollment command must
again establish a Release-authorized program record; when a root-owned package
program differs from the prior current record, only that command can bind its
new exact bytes after a fresh accepted Release Decision. H4-1D does not claim a
repository, unattended package upgrade, package repair across interrupted
versions, Windows package, or Public Beta supply-chain process.

### H4-1C custody recovery and removal contract

Custody remains a separate terminal-only process. `ardents-custody
export-recovery-bundle` requires the exact record ID and public Authority
commitments, unlocks the encrypted record interactively, requires a distinct
Bundle password, and test-restores the new Bundle before reporting success.
`restore-recovery-bundle` accepts the same commitments and a Bundle password
but writes only an `authority-locked` quarantine record in an empty destination
Vault. A Bundle is therefore exportable but cannot sign or silently take over
an existing Authority.

`purge-record` is the separately confirmed destructive action. It first
authenticates the exact encrypted record and matching public binding with the
Vault password, asks for terminal confirmation, and then removes only that
record. It deliberately retains the Authority floor, so a stale Bundle or
older local record cannot regain authority after purge. None of these commands
accepts a password or root material in flags, environment, configuration, or
Endpoint IPC.

Activating a recovered Name Authority remains unavailable from this command:
it needs the maintained opaque, fresh Namespace reconciliation witness. That
witness cannot be safely reconstructed from a Bundle, a local JSON file, or a
participant-supplied generation number. A participant-facing activation route
therefore waits for the H4-4 Namespace input boundary rather than claiming
repair is complete.

## Current technical inputs

- [Closed-alpha Ubuntu Portable enrollment](../closed-alpha-enrollment.md)
  owns the exact external first-execution check, local Enrollment Input, and
  explicit user-session unit instructions. It deliberately makes no public
  release or future-version authorization claim.
- [Release, Update, and Authority Custody](../../technical/release-update-custody.md)
  owns current bounded release/update/custody behavior. It does not yet select
  installer, automatic updater, platform durability, or a complete operator
  journey.
- [Operating model](../operating-model.md) owns the accepted Portable/Installed
  distribution, state separation, release-safety, and Authority-recovery
  contract.
- [R-095](../../research/records/r-095-portable-endpoint-alpha-lifecycle.md)
  records the selected Ubuntu-first/unsigned-Windows direction and the current
  disposable verification and user-service lifecycle evidence.
- [H4 scope](../scope.md) owns the complete H4-1 and Public Beta expectation.

## Open decisions deliberately deferred

- Exact package formats and repositories for later Installed Windows profiles,
  and any APT repository/update policy beyond the selected direct Ubuntu `.deb`.
- Whether observed Portable-user friction justifies an installer, native
  package, or additional client-facing integration after the first alpha.
- Threshold release-control operations and independent reproducible-release
  evidence; these belong with H4-6.
- Exact update transport, mirror behavior, automatic-update policy, and
  cross-version data migration; these belong to H4-1B after a live alpha makes
  their real needs observable.
- Platform-native key-protection, actual cross-account access, and power-loss qualification beyond the
  declared H4-1A alpha profile.
