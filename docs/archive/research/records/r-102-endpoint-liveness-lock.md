---
id: R-102
title: Endpoint liveness lock and stale local attachment recovery
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-102 — Which minimal cross-platform mechanism proves one live Endpoint state-root owner and safely recovers a stale local attachment after abrupt termination?

## Decision this unlocks

Select the H4-1A state-root owner/liveness and local attachment restart rule
for Windows 11 and Ubuntu LTS. It may authorize narrow platform adapters below
the Endpoint state owner; it does not claim H4-7 malicious-sibling isolation,
multi-user tenancy, or installed-service supervision.

## Current contract

- H4-1 keeps Vault, Grants, floors, cache, live state, diagnostics, and runtime
  attachment outside the replaceable artifact directory.
- R-101 measured that an `O_EXCL` owner residue and a filesystem Unix socket
  remain after forced termination on both selected platforms.
- A generic local attachment must not become an ambient administration port or
  a same-user isolation claim.

## Hypotheses

- **H1:** platform-held advisory/exclusive file locks, released by process
  termination, can establish one state-root owner. Only after acquiring that
  lock can a new owner classify and remove its bounded stale socket, then bind
  a fresh local attachment.
- **H2:** a different OS-owned primitive can provide equivalent liveness and
  recovery with lower maintenance cost while retaining the same ordering.
- **H0:** no small cross-platform mechanism distinguishes live ownership from
  residue safely enough; H4-1A must narrow its attachment or platform scope.

## Evaluation criteria

- a second ordinary-user process cannot become state-root owner while the first
  is live; after forced termination it can acquire ownership without manual
  deletion;
- stale attachment removal is permitted only by the new owner, only beneath
  its verified runtime directory, and only after it has established liveness;
- restart never deletes Vault, Grants, floors, cache, diagnostics, arbitrary
  files, or a live listener; all path and error classes are explicit;
- Windows 11 and Ubuntu LTS use supported primitives with bounded adapters and
  no unreviewed persistent daemon or third-party lock service;
- the result does not imply protection against an administrator or malicious
  process sharing the account.

## Evidence plan

### Primary sources

- Official Windows locking and filesystem access documentation.
- Linux/POSIX locking documentation and Go/platform implementation boundaries.
- R-101 state/IPC measurements and the current Endpoint technical contract.

### Experiment

For each candidate, run two ordinary-user processes against one empty synthetic
state root. Capture first-owner acquisition, concurrent-owner refusal, forced
termination, second-owner acquisition, socket residue classification, bounded
recovery/rebind, and all files before/after. Repeat on Windows 11 and Ubuntu
LTS. Synthetic state contains no Authority or live Endpoint data.

### Failure scenarios

- Two processes both report ownership or a second process deletes a live
  socket.
- A crash leaves permanent refusal or recovery deletes durable state.
- A path substitution or unexpected file type is treated as a stale socket.
- A platform adapter silently falls back to loopback, a pipe, or an unbounded
  global path.

## Findings

- **Measurement:** R-101 has already falsified using create-exclusive durable
  residue as a liveness lock: both Windows and Ubuntu left `owner.lock` and
  socket paths after forced termination.
- **Sourced fact:** Windows `LockFileEx` can request an exclusive lock with an
  immediate-failure flag; when a process terminates or closes the relevant file
  handle, Windows unlocks its outstanding locks. Unlock timing can be delayed by
  system resources, so an owner must retain a bounded retry/error outcome.
  [Microsoft LockFileEx documentation](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
  (accessed 2026-08-24).
- **Sourced fact:** Linux `flock` supports a nonblocking exclusive advisory
  lock. It is associated with an open file description and is released once all
  associated descriptors close; it is advisory, so it is not a defence against
  a process that deliberately ignores it. [Linux `flock(2)`](https://man7.org/linux/man-pages/man2/flock.2.html)
  (accessed 2026-08-24).
- **Implementation fact:** the current Go standard library exposes
  `syscall.Flock` for Linux. The already reviewed direct dependency
  `golang.org/x/sys v0.47.0` exposes Windows `LockFileEx` and `UnlockFileEx`.
  This permits a disposable platform-adapter experiment without adding a new
  module. It does not authorize a new maintained Endpoint dependency surface.
- **Inference:** option 1 is technically plausible but needs one exact order:
  acquire the process-lifetime state lock; only then inspect a socket below the
  fixed runtime directory; remove it only if it has the expected socket type;
  bind the new attachment; retain the lock until listener shutdown. A `busy`
  result after crash is retryable only within a stated limit; it is never a
  license to delete the owner file or arbitrary runtime entry.
- **Measurement:** on the current Windows host, the R-102 two-process harness
  reported `lock=busy` for a concurrent contender. After forced termination of
  only the first harness process, its replacement reported `lock=acquired`,
  `stale_socket_type=socket`, and `stale_socket_recovered=true` on 2026-08-24.
  Its wrapper verified removal of its exact temporary root, binary, markers,
  and logs.
- **Measurement:** the same harness cross-compiled for Linux/amd64 and run in
  local Ubuntu 24.04 WSL produced the same concurrent refusal and post-`SIGKILL`
  recovery fields on 2026-08-24. Its wrapper verified removal of its exact
  `/tmp` root, marker, and log. These two local platform runs are functional
  semantics evidence only; they do not measure an NFS/SMB root, power loss,
  a hostile same-user process, or a maintained Endpoint implementation.
- **Inference:** H1 is supported for a local per-user state root: a held
  OS-level file lock plus type-checked, post-lock stale-socket recovery meets
  the tested exclusive-owner/restart outcome on both declared alpha platforms.
  The lock file may persist as non-secret residue; it cannot be interpreted as
  proof that an Endpoint is still live.
- **Measurement:** the strengthened Windows and Ubuntu wrappers required a
  recovery-requesting contender to remain `lock=busy` while the first owner was
  live and verified `live_socket_preserved=true`. After forced termination,
  both replacements acquired ownership, classified the stale socket, rebound,
  and closed it. A regular file, directory, and Windows junction or Ubuntu
  symlink placed at `attachment.sock` were each rejected as
  `unexpected_runtime_entry_type` and preserved.
- **Measurement:** deterministic substitution of the entire runtime directory
  initially falsified the unrefined path-based candidate on both platforms: it
  followed a junction/symlink and created the socket in the outside sentinel.
  The refined harness `Lstat`s every exact owned directory before taking the
  lock, uses Linux `O_NOFOLLOW` for the owner file, and rejects unexpected
  entry types. Repeated Windows 11 and Ubuntu 24.04 WSL runs then reported
  `substituted_runtime_accepted=false`, `outside_socket_created=false`, and a
  preserved sentinel.
- **Measurement:** the final repeated runs also enforced owner-only directory
  and attachment policy before readiness: protected sole-current-user DACLs on
  Windows and effective-user `0700` directories/`0600` socket on Ubuntu. An
  actual second-account denial remains unmeasured, and the result deliberately
  excludes a malicious process sharing the account.

## Options

1. Platform-held file locks plus a guarded stale-socket cleanup sequence.
2. Socket existence/bind as the sole ownership mechanism.
3. A local loopback listener with a bearer token.
4. Narrow H4-1A to a platform with a selected supervisor.

## Recommendation

**Product decision (2026-08-24): option 1 is selected as the liveness and
recovery submechanism of R-101's H4-1A composition.** Its maintained form must
use thin Windows
`LockFileEx` and Ubuntu `flock` adapters; acquire the lock before inspecting
runtime state; remove only an expected socket below the owned runtime root;
and return explicit `owner-busy`, `lock-error`, `unexpected-runtime-entry`,
`stale-recovery-failed`, or `attachment-bind-failed` results. It must not
support network-mounted roots or make a same-user adversary claim.

**Confidence:** medium-high for the declared local alpha scope. **Strongest
argument against the recommendation:** the two platform primitives have
different semantics and delayed-release behaviour; a maintained implementation
still needs bounded retry, platform tests, and correct access control before it
can be promoted as a participant lifecycle.

## Disposition

Selected for local per-user H4-1A integration on both declared platforms. The
experiment supports concurrent refusal, abrupt-loss acquisition, guarded stale
recovery, unexpected-entry preservation, deterministic path-substitution
rejection, and permission-policy round-trip. Maintained Endpoint integration,
bounded delayed-release retry, cross-account tests, and lifecycle qualification
remain; network-mounted roots and malicious-same-user protection remain outside
the claim. A consequential dependency or unsupported platform divergence
requires ADR review. The disposable harness was retired by the pre-H4-8
baseline after its measured outcomes were retained here and its maintained
Portable-runtime behavior tests were identified.
