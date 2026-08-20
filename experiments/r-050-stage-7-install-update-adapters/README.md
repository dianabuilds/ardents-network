# R-050 Stage 7 install/update Adapter experiment

This disposable experiment answers
[R-050](../../docs/research/records/r-050-stage-7-install-update-adapters.md):
which Ubuntu and Windows Installed package/bootstrap/filesystem/activation
Adapters can implement the managed Stage 7 lifecycle, and whether the same
platform executable can be released directly as a minimal Portable artifact,
without making either delivery form the release authority?

It is not maintained Ardents code and does not authorize S7.1–S7.4. Generated
packages, signing keys, repositories, VM disks/snapshots, binaries, state,
crash images, caches, and raw evidence stay in an owned external scratch root.
The repository root module is unchanged; build-ignored Go files are copied to a
temporary module before execution.

## Frozen candidates and pre-result decision rule

Inputs were frozen before candidate execution on 2026-08-20:

| Concern | O1 candidate | Comparison/rejection candidate |
|---|---|---|
| Ubuntu bootstrap package | minimal `amd64` `.deb`, no maintainer scripts, obtained through an exact-key `Signed-By` APT repository | APT owns every application payload update |
| Windows bootstrap package | signed per-machine MSI; a user-context Ardents lifecycle entry owns normal remove/preserve/purge before the elevated package operation | full MSIX owns application installation and update |
| executable payload | common immutable per-environment version directory, admitted by the R-049 Release Decision | platform-package payload mutation |
| Portable artifact | the exact authenticated platform executable contained in the Installed package, plus only companions proven unavoidable; no bootstrap/lifecycle Adapter | second installer/update stack disguised as Portable |
| activation | one same-filesystem record replaced after file flush; Linux `renameat` plus directory `fsync`, Windows `MoveFileExW(REPLACE_EXISTING|WRITE_THROUGH)` | in-place overwrite, copy/delete, delayed-reboot activation, cross-volume move |

The package signature or APT archive signature is additional delivery evidence.
It cannot authorize a release target, lower a release floor, or replace the
R-049 threshold decision.

H1/O1 is selected only if both required platforms reach the same lifecycle
outcomes. It is falsified if any measured run executes unaccepted bytes, exposes
ordinary runtime to elevation, mutates an active payload in place, guesses a
commit after interruption, loses the last committed activation, accepts a link,
reparse point, unsupported volume, or cross-volume copy, changes Vault/floors,
silently removes a non-empty Vault, or leaves an object outside the
precommitted residue inventory. It is also falsified if Installed and Portable
executable digests, runtime features, Interfaces, resources, state
compatibility, or claims differ. Package-lifecycle convenience is intentionally
not a Portable parity requirement. A platform-specific weaker runtime success
is failure, not a score to average.

## Support and measurement profiles

The required qualification families remain those accepted by R-023:
fully-patched supported Windows 11 `x86-64` and Ubuntu LTS `x86-64` images
frozen with each candidate. R-054 owns the final image digest and campaign
identity. R-050 narrows storage support to:

- a local fixed NTFS volume on Windows, with state and staging on the same
  volume, no reparse component, and a protected owner-only DACL; and
- a local ext4 filesystem on Ubuntu with state and staging on the same mount,
  no symlink component, owner UID, directories `0700`, files `0600`, and a
  successful file and parent-directory `fsync`.

SMB/NFS, OverlayFS, tmpfs, FUSE, ReFS, FAT/exFAT, removable media, cloud-synced
folders, and cross-mount/cross-volume activation are explicit
`activation-unsupported` results. Their behavior is not inferred from the
required profiles.

The 2026-08-20 development probes are deliberately non-qualifying:

| Probe | Frozen identity | Permitted conclusion |
|---|---|---|
| Windows activation smoke | Windows 11 25H2, build `26200.9168`, `amd64`; local filesystem identity recorded by the harness | API, open-handle, link/reparse, permission, and process-interruption behavior only |
| Ubuntu package smoke | OCI image `ubuntu:26.04@sha256:2260313b31c8c011cd2eebe728008efac1b3982be73eb71348ea2648d2c0e09b`; `dpkg-deb 1.23.7`, APT `3.2.0` | package construction/install/reinstall/remove and residue only |
| Ubuntu activation smoke | the same container on named Docker volume `ardents_r050_ext4_20260820`, reported as ext4; WSL2 kernel `6.6.87.2-microsoft-standard-WSL2` | syscall and process-interruption behavior only |

Container OverlayFS is never activation evidence. Neither development host can
simulate loss of acknowledged device-cache writes, so reboot and hard-power
interruption stay open until controlled VM/block-device runs.

## Owned layout and privilege boundary under test

The final candidate package owns only the stable bootstrap, immutable initial
Endpoint executable target, and registration:

- Ubuntu: `/usr/lib/ardents/bootstrap/ardents-bootstrap`,
  `/usr/lib/ardents/seed/ardents`, and `/usr/bin/ardents`; and
- Windows: `%ProgramFiles%\Ardents\bootstrap\ardents-bootstrap.exe`, the
  immutable `%ProgramFiles%\Ardents\seed\ardents.exe`, the explicit uninstall/
  repair registration, and no service or automatic-start entry.

The seed executable is the exact Portable target digest. The bootstrap verifies
its release metadata before materializing the unprivileged initial payload; the
package never owns mutable Endpoint state. The existing Ubuntu `.deb` smoke is
only a bootstrap fixture and does not yet establish this final seed layout.

The unprivileged owner creates one environment beneath
`${XDG_DATA_HOME:-$HOME/.local/share}/ardents/environments/<environment-id>` on
Ubuntu or `%LOCALAPPDATA%\Ardents\environments\<environment-id>` on Windows.
Within that root, `payloads/` holds exactly the current committed payload, one
verified rollback payload, and at most one `staging/` payload. `activation`
selects the next launch. Mutable copy-on-write state, Authority Vault, recovery
state, and all monotonic floors are siblings with separate owners; they are not
inside a payload directory and are never package repair inputs.

Normal execution and all state transitions are unprivileged. Elevation is
allowed only to install, repair, or remove the enumerated platform package and
registration. Elevated code receives no Vault, Bundle secret, Local Grant,
Service credential, Name/Target, or Application Data. A direct administrator
package removal may remove the bootstrap, but it must leave user state and a
non-empty Vault intact; the supported Ardents lifecycle performs the
user-context empty-Vault cleanup or preserve/export decision before invoking
the package Adapter. Purge is a separate owner-confirmed operation and is never
a package-manager side effect.

## Minimal Portable contract

Portable adds no activation or lifecycle Adapter. On each supported platform the
experiment publishes the exact Endpoint executable target embedded in the
Installed package and only companions that a reproducible run proves
unavoidable. It then checks:

- identical executable digest between package payload and Portable target;
- trusted release verification before first execution and an exact digest recheck
  after copying/replacement but before execution;
- direct first start from an Owner-chosen path with no installer or elevation;
- no implicit package, service, startup, URI/browser/native-host, proxy, DNS,
  route, or VPN registration;
- protected state outside the program artifact and an exclusive mutable-state
  lock;
- stopped replacement by a newer authenticated executable, with existing
  release/rollback floors applied on its first run; and
- direct executable deletion leaving Vault, roots, floors, Grants, Endpoint
  identity, and network state untouched.

This is a small artifact/startup/state-separation check. It does not repeat the
Installed crash-durable bootstrap, automatic activation, repair, or uninstall
campaign. An unchecked raw executable cannot self-authenticate after it starts;
executing one is outside the experiment's Ardents claim.

## Activation interruption matrix

For `activation.next` in the same directory as `activation`, run every row once
from old record `A` to new record `B`, restart the harness, and accept only the
listed recovery:

| Stop point | Required recovery |
|---|---|
| before create | `A`; no temp file |
| after exclusive temp create | `A`; bounded temp removed on recovery |
| after complete write, before file flush | `A`; bounded temp removed |
| after file flush, before replace | `A`; verified `B` temp may be reused or removed |
| after atomic replace, before Linux directory flush / Windows return | `A` or complete `B`, never missing/partial/other; transaction journal decides commit |
| after durability primitive returns | complete `B` on first restart |

Each profile also runs: concurrent reads across 100 activation attempts, where
each successful replacement is whole and any Windows open race is the explicit
unchanged `activation-busy` result; held old executable/record handles; Windows
delete-denying antivirus-equivalent handle; full-disk failure before replace;
owner/mode/DACL denial; unexpected
symlink/reparse/hardlink; path component and total-length boundaries; corrupted
record; unsupported/cross-volume state; repeated rollback/current/staging
cleanup; and a complete pre/post residue inventory. The activation Adapter
does not retry locks internally: a sharing violation is the explicit bounded
`activation-busy` outcome and leaves the committed record unchanged.

## Running the build-ignored harness

Use Go `1.26.x` and `golang.org/x/sys v0.45.0`. Copy only the matching platform
files to a scratch module outside Git:

```powershell
$scratch = Join-Path $env:TEMP 'ardents-r050-install-update'
New-Item -ItemType Directory -Force -Path $scratch
Copy-Item ./experiments/r-050-stage-7-install-update-adapters/activation.go $scratch
Copy-Item ./experiments/r-050-stage-7-install-update-adapters/activation_test.go $scratch
Copy-Item ./experiments/r-050-stage-7-install-update-adapters/activation_windows.go $scratch
Copy-Item ./experiments/r-050-stage-7-install-update-adapters/activation_windows_fixture.go $scratch
Push-Location $scratch
go mod init ardents.local/r050
go get golang.org/x/sys@v0.45.0
go test ./activation.go ./activation_windows.go ./activation_windows_fixture.go ./activation_test.go -count=10 -v
Pop-Location
```

For Ubuntu, copy `activation_linux.go` instead and run the temporary module on
the ext4 mount. The harness prints an exact host/filesystem manifest and refuses
the unsupported profiles before mutation. Raw output is retained externally.

## Package lifecycle smoke

`ubuntu-package-smoke.sh` builds a fixture bootstrap in external scratch,
constructs a script-free `.deb`, verifies its archive members, publishes it in
an ephemeral signed local APT repository, then exercises install, reinstall,
remove, and file-residue checks in the manifest-bound Ubuntu container. The signing key
is experiment-only and destroyed with scratch. The procedure demonstrates APT
archive authentication and package idempotency; it is not an Ardents release
signature or a production repository design.

Windows MSI build/repair/remove remains a separate required run. The Product
Owner accepted WiX Toolset v7.0.0 binary OSMF EULA/tool governance for this
disposable experiment on 2026-08-20; that acceptance does not select MSI for the
product or extend to another WiX version/licence. The signed MSI lifecycle
matrix remains blocked until a separate Product Owner command names the
artifact and authorized mutations on the current Windows machine. Pristine-host
and power-loss qualification remain deferred.

## Development-smoke results

Executed 2026-08-20. All generated state remained in system temporary storage
or the external Docker volume.

- The Windows harness passed ten complete repetitions on Windows 11 25H2 build
  `26200.9168`, local fixed NTFS, Go `1.26.6`, and `x/sys v0.45.0`. Every
  process-interruption, owner-only DACL, junction, hardlink, bounded input,
  held-delete-denying-handle, and post-reader replacement check passed.
  Continuous concurrent opens produced `98–100` explicit
  `activation-busy` outcomes per 100 attempts across the verbose repetitions;
  the committed record never disappeared or became partial, and replacement
  succeeded immediately after readers stopped.
- The Linux harness passed ten complete repetitions on the external ext4
  volume under the manifest-bound Ubuntu container, Go `1.26.6`, and `x/sys v0.45.0`.
  All 100 concurrent activation attempts succeeded in each repetition with
  zero partial/missing reads; process interruption, mode/owner, symlink,
  hardlink, bounded input, open-handle, file `fsync`, rename, and directory
  `fsync` checks passed.
- Two independent Ubuntu package scratch roots produced the same fixture `.deb`
  SHA-256
  `d15e7b1ef1b602c05cf0894baa10b60eab536fc7072ec8e9d1761d0b095a6976`
  with `SOURCE_DATE_EPOCH=1787184000`. Each ephemeral exact-key APT repository
  passed authenticated update, install, reinstall, remove, and the declared
  system-path residue scan. The control archive contained only `control`; no
  maintainer script, service, startup entry, state root, or Authority path was
  created.

These are smoke results supporting the R-050 development decision, not native
qualification. Hard power interruption,
filesystem-full/inode-full injection, real executable-in-use behavior, signed
MSI install/repair/remove, and complete VM reboot/residue campaigns remain open.
The WSL2-backed ext4 volume and a process exit cannot establish device-cache
power-loss durability.

## Evidence and disposition rule

Retain in Git only the human-authored harness, source identities, summarized
measurements, and honest limitations. The Product Owner selected R-050 O1 for
development and accepted the unavailable native durability/residue cells as
deferred, not passed. Windows MSI execution remains authorization-pending. Any
missing scheduled observer fact is `invalid`; a candidate-caused escape or
residue is `fail` and reopens R-050/ADR-0015.
