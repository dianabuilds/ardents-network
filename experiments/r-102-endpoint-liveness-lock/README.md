# R-102 — Endpoint liveness lock experiment

## Question

Can a platform-held file lock establish exactly one live synthetic Endpoint
state-root owner and allow a replacement process to reclaim an expected stale
Unix socket after the owner is forcibly terminated?

## Hypothesis

Held `flock`/`LockFileEx` ownership acquired before runtime classification,
combined with exact no-follow path checks and removal of only the expected stale
socket type, permits one owner and bounded crash recovery on Ubuntu and Windows.
Any simultaneous owner, followed substitution, unexpected-entry mutation, or
out-of-root deletion falsifies the sequence.

## Contract under test

The first process creates a synthetic `live/owner.lock`, holds a platform
exclusive lock, binds `runtime/attachment.sock`, and writes a readiness marker.
A concurrent process must report `lock=busy`. After the wrapper kills only the
first process, the replacement must acquire the lock, confirm that the existing
runtime entry is a socket, remove that one expected socket, rebind it, and
close it. The lock file itself is retained as harmless state residue.

Before selecting the sequence, run two additional falsifiers:

1. While the first owner is live, a contender requesting recovery must still
   return `lock=busy`, and the live socket must remain reachable and unchanged.
   After ownership is released, ordinary files, directories, and supported
   symlink/reparse forms at `attachment.sock` must be rejected and preserved.
2. Pre-create `endpoint-state/runtime` as a symlink or junction to a distinct
   caller-owned sentinel directory. If the harness reports ready or creates its
   socket in that sentinel, path-based ownership is falsified. The refined
   candidate must inspect each owned directory without following a link or
   reparse point and return `unexpected-runtime-root` before locking, deleting,
   or binding anything outside the exact state root.
3. Enforce and inspect the declared cross-user access boundary before reporting
   readiness. Ubuntu directories must be owned by the effective user at `0700`
   and the live socket at `0600`. Windows directories and the live socket must
   have a protected DACL whose access entries name only the current user.
   Windows may normalize an inheritable directory rule into separate effective
   and inherit-only entries, so the invariant is the sole trustee rather than
   an exact ACE count. Failure to apply or read back that policy rejects the
   shared filesystem-socket candidate on that platform.

The second falsifier is not a claim against a malicious same-user process racing
after validation. It checks deterministic startup substitution and makes the
normal-user state-root boundary explicit.

The refined candidate checks every exact Endpoint-owned directory with
`Lstat` before acquiring the owner lock and rejects a link/reparse point or
non-directory. Its Linux lock open uses `O_NOFOLLOW` and verifies a regular
`0600` file owned by the effective Endpoint user; Windows rejects a preexisting
link/non-regular lock entry before `LockFileEx`. These checks close deterministic
startup substitution only; R-101's owner-only directory access is still needed
to exclude races by another OS user, and no malicious-same-user claim is made.

This experiment has no Authority material and does not establish malicious
same-user isolation, cross-user ACLs, a maintained Go interface, or a service
supervisor.

## Run

Windows:

```powershell
pwsh -NoProfile -File .\run-windows.ps1
```

For Ubuntu, compile the Linux binary with the selected Go toolchain, then run:

```sh
bash run-ubuntu.sh /path/to/r-102-linux-harness
```

The deterministic startup-substitution probes use the same binary inputs:

```powershell
pwsh -NoProfile -File .\run-substitution-windows.ps1
```

```sh
bash run-substitution-ubuntu.sh /path/to/r-102-linux-harness
```

The wrappers use short fixed temporary paths, refuse reuse, terminate only the
harness child they started, and remove only their exact experiment root and
logs after reporting output.

## Evidence

Record platform, Go/tooling version, first-owner readiness, concurrent-owner
result, post-kill recovery result, socket type check, and cleanup result in
R-102. Any simultaneous owner, missing post-kill acquisition, unexpected
runtime entry type, followed runtime-root substitution, mutation of an
unexpected entry, or deletion outside the expected socket fails the candidate.

## Disposition

Disposable research code for the decided R-102 question. The state-owner rule
is promoted to H4-1; retain the harness only until its unique evidence enters
source history, then let maintained Endpoint tests supersede it.
