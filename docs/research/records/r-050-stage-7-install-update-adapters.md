---
id: R-050
title: Which Ubuntu/Windows Installed Adapters and minimal Portable artifacts meet the Stage 7 proposal?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-050 — Stage 7 distribution and update Adapters

## Decision this unlocks

Select Ubuntu/Windows Installed lifecycle Adapters plus the minimal Portable
artifact contract for S7.2–S7.4. Freeze Installed stable bootstrap, immutable
payload layout, atomic activation, crash durability, repair, uninstall and
purge; freeze Portable executable/companion contents, stopped replacement,
state separation, feature/claim parity, and absence of implicit integration
before the release-activation proposal can become ADR-0015.

## Current contract

[R-048](r-048-h3-stage-7-contract.md), the
[lifecycle specification](../../development/stage-7-lifecycle-spec.md), and the
[release-activation proposal](../../development/stage-7-versioned-release-activation-proposal.md)
require common release/state-safety behavior, thin platform Installed Adapters,
immutable versioned Installed payloads, an atomically replaced activation
record, copy-on-write mutable state, safe authenticated rollback only,
Authority/floor separation, and unprivileged runtime. Portable reuses the
release, state compatibility, and floor rules through stopped executable
replacement rather than duplicating the activation Adapter.

The Product Owner accepted two relatively equivalent Distribution Profiles on
2026-08-20, then clarified that Portable is deliberately small: the authenticated
platform executable plus only unavoidable authenticated non-secret static
configuration templates/resources. Mutable configuration remains protected
Endpoint state outside the artifact. The Installed
package MUST wrap the same platform executable digest for a release. Both expose
the same direct-binary behavior, Application Interfaces, runtime capabilities,
resource limits, state compatibility, and claim ceilings; package-managed
lifecycle convenience is not a parity requirement. Portable has no installer,
stable bootstrap, or lifecycle Adapter and no implicit system integration. Its
exact digest MUST be authenticated by the Owner or an already trusted verifier
before first execution and after stopped replacement; raw bytes cannot
self-authenticate once executed.

R-023 fixes supported, fully patched Windows 11 and Ubuntu LTS `x86-64` images
at candidate freeze time; R-054 owns the exact final image and campaign
identity. R-049 proposes the release verifier and its durable floor handoff.
R-050 may select only package/bootstrap/storage mechanics. An OS package or
archive signature is additional delivery evidence, never Ardents release
authority.

## Hypotheses

- **H1:** two Installed stable-bootstrap Adapters can wrap the exact Portable
  platform executable target, while direct Portable execution needs no lifecycle
  Adapter and exposes no feature, trust, or privacy-claim skew.
- **H2:** fully OS-package-managed payload updates are required per platform,
  while a shared Release Decision still constrains accepted package identity.
- **H0:** safe crash-consistent activation/rollback requires in-place mutation,
  broad permanent privilege, hidden OS trust, or unequal platform guarantees.

## Evaluation criteria

Evidence is conjunctive across both required platforms. Before measurement,
the candidate must declare:

- ordinary signed Installed-profile install/repair/uninstall and direct
  unprivileged Portable execution on supported Ubuntu LTS and Windows 11
  `x86-64`; the current development surfaces and their claim ceiling are bound
  separately in the campaign manifest;
- package and raw-executable build/source/tool/version/license identity and
  reproducibility inputs, with the same platform executable digest inside the
  Installed package and as the Portable release target;
- exact elevated Installed package/registration operations and unprivileged
  runtime in both profiles; Portable creates no registration by default;
- owned Installed package/program paths, Portable executable and unavoidable
  companions, state-root selection/lock, DACL/modes, link/reparse policy, path
  bounds, and complete residue inventory;
- same-filesystem activation, file-data flush, atomic replacement, metadata
  durability, open-handle/lock, disk-full, and restart recovery behavior;
- exactly current committed payload plus one verified rollback payload and at
  most one staging payload;
- Installed repair/uninstall, direct administrator package deletion, Portable
  stopped replacement/deletion, explicit state purge, same-state concurrency
  rejection, and cancellation semantics without Authority/floor mutation; and
- feature, resource, Application Interface, state-compatibility, and claim
  parity; package-managed update/repair/uninstall ergonomics may differ, but a
  Portable-only weaker or stronger network/security result is forbidden; and
- one shared outcome taxonomy: `activation-unsupported`, `activation-busy`,
  `resource-denied`, `uninstall-blocked`, `uninstalled`, `purged`,
  `cleanup-incomplete`, or the lifecycle's existing transaction outcomes.

Development smoke uses a maximum `16 KiB` opaque activation fixture, maximum
`64` bytes per generated path component, maximum `512` path bytes overall, and
maximum `240` UTF-16 code units for the Windows activation path. R-054 must bind
the final canonical record schema within these ceilings; R-050 does not invent
that serialization.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [Ubuntu 26.04 LTS release notes](https://documentation.ubuntu.com/release-notes/26.04/)
  and [official image index](https://releases.ubuntu.com/releases/26.04/);
- [APT archive authentication](https://manpages.debian.org/testing/apt/apt-secure.8.en.html),
  which distinguishes signed repository metadata from per-package signatures;
- [Debian Policy package lifecycle and idempotency](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html);
- Linux [`rename(2)`](https://man7.org/linux/man-pages/man2/rename.2.html) and
  [`fsync(2)`](https://man7.org/linux/man-pages/man2/fsync.2.html);
- [Microsoft's Windows distribution choice](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/choose-distribution-path),
  [MSIX packaged-desktop state/uninstall behavior](https://learn.microsoft.com/en-us/windows/msix/desktop/desktop-to-uwp-behind-the-scenes),
  and [Windows 11 lifecycle](https://learn.microsoft.com/lifecycle/products/windows-11-home-and-pro);
- [Microsoft signed MSI authoring](https://learn.microsoft.com/en-us/windows/win32/msi/authoring-a-fully-verified-signed-installation)
  and [Windows Installer repair](https://learn.microsoft.com/en-us/windows/win32/msi/searching-for-a-broken-feature-or-component);
- Microsoft [`MoveFileExW`](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw),
  [`CreateFile`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea),
  and [`FlushFileBuffers`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers);
- [WiX Toolset releases](https://github.com/wixtoolset/wix/releases/) and
  [source license](https://github.com/wixtoolset/wix/blob/main/LICENSE.TXT); and
- Go `1.26.6` plus the already reviewed `golang.org/x/sys v0.45.0` operating
  system surfaces. The experiment changes neither root `go.mod` nor the
  dependency register.

### Experiment

[The build-ignored R-050 experiment](../../../experiments/r-050-stage-7-install-update-adapters/README.md)
precommits candidate identities, layout, falsifiers, interruption points, and
qualification limits. Generated packages, keys, local repositories,
toolchains, module caches, state, VM disks, snapshots, binaries, and raw output
remain outside Git.

All development runs use the immutable axes, coverage partitions, observer
controls, and episode-authority rules in the
[Stage 7 development-host campaign specification](../../development/stage-7-host-campaign-spec.md).

The development phase runs:

1. an owner-mode/DACL, link/reparse/hardlink, held-handle, bounded-path,
   concurrent-reader, and five-point process-interruption activation harness on
   Windows fixed NTFS and Linux ext4;
2. a deterministic script-free Ubuntu `.deb` published through an ephemeral
   exact-key `Signed-By` APT repository, followed by install, reinstall, remove,
   and declared-residue checks; and
3. the exact Portable executable target on both platforms, followed by trusted
   pre-execution verification, digest/
   package-payload equality, no-install first start, direct-binary use, zero
   implicit registration, protected-state separation, same-state lock, stopped
   replacement, direct deletion, and program-object residue checks; and
4. later controlled Windows/Ubuntu VM runs that inject disk/inode exhaustion,
   executable/file locks, reboot, acknowledged-write power loss, repair,
   uninstall, purge cancellation, and a complete external residue scan.

The Windows MSI run begins only after its exact authoring/signing tool and
license are accepted. A container/process-exit smoke result cannot substitute
for a VM/block-device durability result.

### Failure scenarios

Wrong package/executable/release/profile identity; partial deployment;
non-idempotent lifecycle action; Installed and Portable payload or feature skew;
activation across unsupported storage; lost acknowledged durability; ACL/mode
inheritance change; bootstrap/payload mismatch; unsafe or revoked rollback;
Authority/floor mutation; Portable replacement while running; implicit state
move or merge; same-state concurrent processes; service/startup/URI/browser
residue; direct administrator/package/executable removal with a Vault; cancellation
during purge; and a platform/profile result that cannot map to the shared
Interface.

## Falsification criteria

H1/H2 is falsified on a required platform if any valid required A/C/D lifecycle
case cannot reach the shared outcome; unauthorized bytes execute; activation
needs in-place payload mutation or cross-volume atomicity; a crash loses the
last committed activation; ordinary runtime retains elevation; repair/update/
rollback changes Vault or a monotonic floor; supported uninstall silently
erases a non-empty Vault; or a declared object/process/registration survives
successful cleanup.

H1 is also falsified if the Installed package and Portable target on the same
platform contain different Endpoint executable digests for one release, expose
different runtime capabilities or claims, require different Application
behavior, permit concurrent use of one mutable state root, or if replacing/
deleting the Portable executable implicitly mutates protected state, or the
contract claims an unchecked raw executable can authenticate itself after
execution.

Every precommitted interruption point must recover the expected state on first
restart. Unsupported filesystem, lock, privilege, or durability behavior is an
explicit unsupported/busy result, not grounds to weaken the common Interface.
Failure on either required host selects O0 unless the supported-platform
contract is explicitly changed.

## Findings

### Sourced platform facts

- **Sourced fact:** Ubuntu 26.04 is an LTS release supported through 2031 and
  publishes an official `amd64` image and signed image checksums.
- **Sourced fact:** APT authenticates the signed `Release`/`InRelease` metadata
  and package hashes; it does not verify an individual `.deb` signature by
  default. Archive trust therefore remains a delivery root distinct from the
  R-049 release root.
- **Sourced fact:** Debian maintainer scripts may run across failure recovery
  and must be idempotent. The O1 bootstrap package avoids them entirely, so no
  elevated script receives or migrates user Authority state.
- **Sourced fact:** Linux same-mount `rename` atomically replaces an existing
  destination and leaves open file descriptors referring to the old file;
  cross-mount rename returns `EXDEV`. File `fsync` alone does not persist the
  containing directory entry, so the Adapter must also `fsync` the opened
  parent directory.
- **Sourced fact:** MSIX package files are read-only and container-captured
  state is removed with the package, but writes outside the package are allowed
  when permissions permit and are outside that cleanup boundary. A separately
  retained Vault/version/floor tree therefore prevents MSIX alone from owning
  the required uninstall contract.
- **Sourced fact:** Microsoft documents signed MSI installation and Windows
  Installer component repair. MSI/EXE distribution leaves application update
  responsibility with the publisher, which fits a stable package plus shared
  Release Decision better than platform-owned payload updates.
- **Sourced fact:** `MoveFileExW` can replace an existing file and offers
  `MOVEFILE_WRITE_THROUGH`; cross-volume behavior becomes copy/delete only when
  `MOVEFILE_COPY_ALLOWED` is requested. Its documentation explicitly describes
  flush guarantees for copy/delete but is not, by itself, proof of same-volume
  NTFS survival after power loss. R-050 therefore keeps the hard-power test.
- **Sourced fact:** WiX Toolset v7.0.0 at commit `b8977d6` is the current MSI
  authoring candidate. Its binary release requires OSMF EULA acceptance; its
  source is MS-RL.
- **Product Owner decision (2026-08-20):** use of the WiX Toolset v7.0.0 binary
  under the OSMF EULA is accepted for the disposable signed-MSI experiment.
  This acceptance neither selects MSI as the final product installation
  contract nor applies to a future WiX version or a materially changed
  license/maintenance-fee obligation.
- **Product Owner decision (2026-08-20):** Installed and Portable are both
  supported Distribution Profiles with relative feature/security parity, and
  direct binary operation is first-class in both. Portable is not a reduced
  developer build; browser integration remains optional.

### Development measurements

- **Measurement:** the current Windows development host was Windows 11 25H2 build
  `26200.9168`, `amd64`, local fixed NTFS. Ten complete Go `1.26.6`/
  `x/sys v0.45.0` repetitions passed successful replacement, five process-exit
  boundaries, protected owner-only DACL, junction rejection, hardlink
  rejection, bounded input/path, held-delete-denying handle, and post-reader
  recovery.
- **Measurement:** under continuous concurrent Windows opens, `98–100` of 100
  activation attempts per verbose repetition returned explicit
  `activation-busy`; no reader observed a missing or partial record, the old
  record remained committed on busy, and replacement succeeded after readers
  stopped. Activation must therefore follow stop-new-work/drain; the Adapter
  performs no hidden retry.
- **Measurement:** ten equivalent Linux repetitions passed on an ext4 Docker
  volume with Ubuntu 26.04 userspace and WSL2 kernel
  `6.6.87.2-microsoft-standard-WSL2`. All 100 concurrent attempts per
  repetition succeeded; file `fsync`, atomic rename, parent-directory `fsync`,
  process-exit, `0700`/`0600` owner checks, link/hardlink rejection, and bounded
  inputs passed.
- **Measurement:** two new Ubuntu package scratch roots using
  `dpkg-deb 1.23.7`, APT/`apt-ftparchive 3.2.0`, GnuPG `2.4.8`, and
  `SOURCE_DATE_EPOCH=1787184000` produced the same fixture `.deb` SHA-256
  `d15e7b1ef1b602c05cf0894baa10b60eab536fc7072ec8e9d1761d0b095a6976`.
  Exact-key authenticated APT update, install, reinstall, remove, and declared
  system-path residue checks passed. The control archive contained only
  `control`; it created no service/startup/state/Authority object.
- **Measurement limitation:** Docker Desktop's container root is OverlayFS and
  the named volume is ext4 behind WSL2. Process exit is not reboot or power
  loss. These runs prove only Adapter/API behavior; they do not qualify Ubuntu
  ext4 or Windows NTFS durability.

### Candidate boundary refined by evidence

- **Inference:** O1 is the only retained architecture. Installed uses a minimal
  script-free `.deb` from an exact-key signed APT archive on Ubuntu and a signed
  per-machine MSI on Windows. Portable is the same authenticated platform
  executable payload plus only companions proven unavoidable; it has no stable
  bootstrap or Install Lifecycle Adapter.
- **Inference:** platform package removal by a direct administrator is external
  force, not Authority authorization. It may remove the stable bootstrap but
  must not inspect or erase per-user state; a non-empty Vault and monotonic
  floors remain in place. Supported empty-Vault uninstall first removes the
  declared user runtime state, then invokes the package Adapter.
- **Inference:** the package owns only the stable bootstrap, the immutable
  authenticated initial Endpoint executable target, and explicit registration.
  The initial target has the exact Portable digest and is materialized only
  after release verification; it is not mutable state. Ubuntu candidates use
  `/usr/lib/ardents/bootstrap/`, `/usr/lib/ardents/seed/ardents`, and
  `/usr/bin/ardents`; Windows candidates use
  `%ProgramFiles%\Ardents\bootstrap\`, `%ProgramFiles%\Ardents\seed\ardents.exe`,
  plus the exact repair/uninstall registration. No service, listener,
  auto-start, remote admin path, account, state root, or Authority is installed.
- **Inference:** the Portable release target is Owner-copied and directly run.
  Its default path has no package, service, startup, URI, native-host, proxy,
  DNS, route, VPN, or machine-wide registration. Direct binary use requires none
  of them; optional desktop integration belongs to R-056, not Portable delivery.
- **Inference:** unprivileged environment state lives below
  `${XDG_DATA_HOME:-$HOME/.local/share}/ardents/environments/<environment-id>`
  or `%LOCALAPPDATA%\Ardents\environments\<environment-id>`. Payloads,
  activation, copy-on-write mutable state, Vault/recovery state, and monotonic
  floors are distinct children/owners; none is package repair input.
- **Inference:** the Ubuntu activation Adapter uses a same-directory exclusive
  temp, `0600`, complete write, file `fsync`, close, same-open-directory
  `renameat`, parent `fsync`, then handle/path/mode/owner verification. The
  Windows Adapter uses an owner-only protected root DACL, inheriting exclusive
  temp, complete write and `FlushFileBuffers`, close, same-fixed-NTFS-volume
  `MoveFileExW(REPLACE_EXISTING|WRITE_THROUGH)` without `COPY_ALLOWED`, then
  handle/volume/DACL verification.
- **Inference:** SMB/NFS, OverlayFS, tmpfs, FUSE, ReFS, FAT/exFAT, removable or
  cloud-synced storage, reparse/symlink paths, and cross-mount/volume activation
  are explicit `activation-unsupported`. Windows sharing/access denial maps to
  `activation-busy` with no mutation; the Update Transaction decides whether a
  later bounded episode retries.

## Options

### O1 — Installed stable bootstraps plus direct Portable executable

Retained as the decision candidate. It preserves one deep Release Decision and
Update Transaction, keeps package trust additional, permits authenticated
rollback, and limits elevated code to enumerated Installed-profile objects.
Portable reuses the authorized Endpoint executable and adds no lifecycle Module
or Adapter. Exact unavoidable companion/config contents, stopped replacement,
and state-root locking remain open.

### O2 — OS package manager owns every payload update

Rejected by current evidence. APT, MSI, and MSIX have different rollback,
in-use, user-state, and uninstall semantics; making them own application
payloads would either duplicate the shared state machine or weaken one
platform. Package version/signature also cannot express Ardents threshold,
compatibility, revocation, and durable-floor decisions.

### O3 — Full MSIX application/package lifecycle

Rejected for this contract. MSIX is attractive for immutable package bytes and
clean package uninstall, but the required external Vault/floors/versioned
payloads are outside the package cleanup boundary. Adding a second lifecycle
owner would remove the claimed simplification and still require the O1 shared
transaction.

### O0 — Stop

Selected if signed MSI governance/lifecycle or either controlled durability
campaign fails. Do not substitute in-place update, cross-volume copy, broad
runtime privilege, or an undocumented platform-only success.

## Recommendation

Continue O1 and reject O2/O3. Confidence is **medium** for the architecture and
**low** for final selection until the remaining platform gates run. The
strongest argument against O1 is that Windows showed frequent explicit busy
under continuous opens and neither development activation smoke proves
acknowledged-write durability after power loss. R-056 now freezes only an
explicit per-user Ardents URI association, trusted local browser-mode choice,
and matching cleanup; no extension, native host, proxy, or browser payload is
selected. Those objects remain absent from this package-lifecycle experiment
until the R-056 Windows/Ubuntu platform falsification passes.

Next decision actions, in order:

1. Freeze the disposable WiX v7.0.0 MSI, Ubuntu `.deb`, exact Portable targets,
   and the 392-episode coverage partition outside Git. No Windows installation,
   repair, removal, purge, or installer-owned registration runs at this step.
2. Run the authorized Ubuntu 26.04 Docker package smoke and the available
   Portable contract: package-payload digest equality where a package exists,
   no-install direct start, zero implicit integration, state-root lock, stopped
   replacement, deletion, feature/claim parity, and protected-state
   preservation. Record container/native claim limits.
3. Stop before every Windows Installed cell. After a separate Product Owner
   command identifies the artifact and mutation scope, run only those named
   install/reinstall/repair/remove/Vault-preserve/purge/residue cells on the
   current Windows machine. Optional R-056 per-user URI objects require the same
   explicit authorization; browser payload, extension, native host, proxy, DNS,
   route, and VPN integration remain forbidden.
4. Keep pristine-first-install, reboot, disk/inode-full, acknowledged-write
   power-loss, and VM/block-device durability episodes visibly
   `environment-deferred` until a suitable native surface is supplied.
5. Treat the scheduled subset as slice evidence. A failure reopens R-050 and
   ADR-0015 or requires an explicit product-contract change.

## Disposition

- State: `decided`; the Product Owner selected O1 for Stage 7 development and
  accepted the documented native/durability qualification deferrals on
  2026-08-20.
- Retain the build-ignored experiment through the controlled campaigns; raw
  artifacts remain external and disposable.
- ADR-0015 records the accepted lifecycle architecture. Maintained packages and
  commands still require real implementation, tests, callers, exact imports,
  and package-map entries in their owning slices.
- The environment decision does not authorize a Windows installation. Exact
  artifact, surface, coverage-partition, and later Product Owner authorization
  digests remain R-054 inputs.
