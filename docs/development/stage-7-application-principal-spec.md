# Stage 7 Application Principal specification

Status: **accepted for Stage 7 development on 2026-08-20 under ADR-0016.**
Scheduled R-051/R-052 evidence and explicit native qualification gates remain.
The only first-party `unsafe` exception is the exact bounded Windows bridge in
ADR-0016 and it requires dedicated risk tests.

This document freezes the exact accepted R-051 O2 profile to implement and falsify. It separates a
claim-bearing launcher session from named generic attachment and from the
co-resident direct-binary Adapter. It does not select an Application wire
protocol, network-isolation mechanism, or public claim.

## 1. Principal classes

| Class | Binding | Permitted claim |
|---|---|---|
| `direct-invocation` | One fresh invocation of the same Ardents executable; Adapter and Broker boundary are co-resident and no external Application IPC peer exists | `none`; explicit direct-binary operation remains first-class in Installed and Portable |
| `generic-peer` | Owner-scoped named local endpoint plus OS user/session facts | One coarse same-user trust domain; `application-networking-unverified` |
| `launcher-bound` | Broker-created private inherited channel plus one non-reusable process handle, complete non-breakaway tree owner, Local Grant, Isolation Context, resource parent, broker start, and deadline | Malicious-sibling separation only after R-051 passes; network-isolated claim only after the same tree also passes R-052 |

No class is inferred from executable path, package installation, URI
registration, PID, UID/SID, desktop user, loopback port, pipe/socket name,
process image, or copyable capability alone. Installed and Portable use the
same executable and the same three classes; installation creates no stronger
principal.

`direct-invocation` has a fresh operation resource parent and Local Grant but
does not pretend that a process authenticated itself to itself. If a direct
command uses an external background Broker, that hop is `generic-peer` or
`launcher-bound` and must satisfy this specification. The direct command never
depends on a browser, desktop registration, daemon, or installer.

## 2. Exact launcher-bound identity

One principal is the conjunction of all of these facts:

1. a broker-generated, unpredictable `128-bit` start identity created before
   any untrusted Application code runs;
2. the exact launcher event and frozen launch-policy identity;
3. one private bidirectional channel whose child endpoint was created by the
   Broker and inherited only by the launched root Adapter;
4. one non-reusable OS process handle for that root from creation through
   termination;
5. one OS-owned, non-breakaway process-tree container created before launch or
   joined while the root is suspended;
6. expected platform user/session/token facts observed before activation;
7. one Local Grant with an exact Connection or Service Administration
   operation set;
8. one Isolation Context and hierarchical resource parent;
9. the current Broker and Endpoint start identities; and
10. one finite monotonic deadline.

Changing, losing, or failing to observe any required fact before activation is
`principal-denied`. Losing it after activation starts revocation. All
descendants intentionally share the root principal and never obtain a separate
grant merely by receiving or copying a handle.

The start identity and any channel challenge are defense in depth. They are
accepted only on the already inherited channel, are never placed in argv, an
environment variable, a filesystem object, desktop registration, or result,
and are invalid after channel close, Application/Broker/Endpoint restart,
deadline, or grant revocation.

## 3. Lifecycle and failure order

The shared lifecycle remains:

```text
declared -> launching -> os-bound -> channel-bound -> active
active -> draining -> revoked
active -> revoked | expired
launching | os-bound | channel-bound -> denied
```

The required order is exact:

1. validate launch policy, requested operation, Local Grant, resource parent,
   and deadline without starting an Application;
2. create start identity, private channel, and empty tree owner;
3. create the root stopped or atomically inside the tree owner with only
   declared child descriptors/handles;
4. obtain the non-reusable process handle and platform facts, prove tree
   membership, and close the Broker's duplicate of every child endpoint;
5. start/resume the root, complete one bounded fresh challenge on the exact
   channel, and activate the Local Grant only after all facts agree;
6. admit requests only from that channel and only within the operation,
   context, resource, sequence, and deadline bounds; and
7. on EOF, process exit, observer failure, revoke, expiry, or Broker shutdown,
   deny new work, apply the selected finite drain rule, terminate the complete
   tree, close all channel/process/container objects, and prove cleanup within
   `5 s`.

No attach-after-launch path can become `launcher-bound`. There is no reconnect,
session transfer, capability export, hidden relaunch, or generic fallback.
Persistent Local Grant policy may survive restart; the principal and every
session fact do not.

One principal admits at most `32` processes, `1,024` descriptors/handles,
`256` queued IPC frames, and one active channel. One Broker admits at most `64`
concurrent principals. One frame is at most `64 KiB`. Pre-admission/frame bounds
reject before work; the first observed runtime inventory excess fails the whole
principal closed and starts immediate tree revocation. Ubuntu also applies a
hard `512`-task cgroup ceiling because its PID controller counts kernel task
IDs, including threads, rather than only processes. These are R-051 candidate
bounds, not permission to implement a placeholder protocol.

Connection, Service Administration, and Authority Custody remain disjoint.
The private channel never grants Authority export/import, recovery, release,
Route, update, registration, or another context.

## 4. Ubuntu candidate

The native qualification candidate requires an Ubuntu `x86-64` host with cgroup v2,
`clone3(CLONE_INTO_CGROUP)`, `pidfd`, `cgroup.kill`, and an owner-delegated
domain cgroup whose control files are not writable or visible to the launched
tree.

- The Broker creates an unnamed `AF_UNIX SOCK_STREAM` `socketpair` before
  `exec`, retains one endpoint, and passes exactly the other through
  `os/exec.Cmd.ExtraFiles`. The child endpoint and unrelated descriptors are
  close-on-exec everywhere except the exact root handoff; descendants do not
  inherit the Broker channel.
- `SO_PEERCRED` on this pre-created pair is **not** child identity: Linux fixes
  it at `socketpair` creation, when both endpoints belong to the Broker. The
  first child message may carry kernel-checked `SCM_CREDENTIALS` as a
  supplemental observation, never as the principal.
- Go `syscall.SysProcAttr{UseCgroupFD:true, CgroupFD:...}` creates the root
  atomically in the prepared cgroup. Post-start PID migration is forbidden.
- The cgroup is a non-threaded domain, has `pids.max=512`, exposes recursive
  `populated` observation, and is owned outside the Application. Linux counts
  task IDs in this controller, so an independent `cgroup.procs` observer also
  fails/revokes the candidate at the first process inventory above `32`. The
  launched tree cannot write `cgroup.procs`, create an escapable child cgroup,
  or access the parent delegation.
- Go `os.Process.WithHandle` must yield a pidfd; PID is diagnostic only. Missing
  pidfd/clone3/cgroup controls, an unexpected descriptor, writable tree
  ownership, or an observation failure denies launch.
- Revoke writes `1` to `cgroup.kill`, waits for recursive `populated=0`, reaps
  the root through its process handle, closes both channel ends, and removes
  the empty cgroup within the shared cleanup deadline.

R-052 freezes non-setuid bubblewrap `v0.11.2` and the exact
[`ubuntu-bwrap-native-v1`](stage-7-application-isolation-spec.md) namespace/
storage policy around this tree. If the frozen unprivileged host cannot supply
that boundary
without a permanent broad privileged service, the Ubuntu claim-bearing
candidate fails; a pathname Unix socket or UID check does not replace it.

## 5. Windows candidate

The native qualification candidate requires a Windows 11 `x86-64` host and stable
Job Object, process-token, inherited-handle, and process-handle APIs.

- The Broker creates two anonymous one-way pipes as one bidirectional private
  channel. Only the child's read and write handles plus explicitly declared
  standard handles are inheritable; Broker ends and all unrelated handles are
  non-inheritable. A launch lock covers the inheritable-handle inventory.
- The root is created with `CREATE_SUSPENDED`. Before `ResumeThread`, the Broker
  assigns its stable process handle to a fresh unnamed Job Object configured
  with no breakaway flags, `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` plus
  `ActiveProcessLimit=32`, and `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`.
- Before resume, the Broker opens and freezes the expected user SID, logon SID,
  session, integrity, and exact zero-capability AppContainer facts required by
  R-052
  [`windows-appcontainer-native-v1`](stage-7-application-isolation-spec.md).
  PID and executable path are diagnostic only.
- The Broker retains the Job and process handles; an independent host observer
  inventories membership and limit failures. The Broker terminates the Job on
  revoke. Job empty, root termination, all handle closes, and zero survivor
  inventory are required within `5 s`.

A named pipe remains a generic/coarse candidate only. If used there, it must be
local-only, owner/logon-SID DACL protected, and every impersonation result must
be checked and followed by `RevertToSelf`; failure denies the request. A named
pipe opened by the Broker before child inheritance reports the Broker as its
client, so `GetNamedPipeClientProcessId` must not be treated as child identity.

Go 1.26 exposes creation flags and explicit inherited handles; `x/sys/windows`
v0.45.0 exposes process, token, pipe, and Job calls. Its current
`SetInformationJobObject` signature nevertheless requires a first-party
`unsafe.Pointer` bridge for the fixed Job limit structure. No such code is
authorized now. R-051 must either prove a maintained safe surface or propose in
ADR-0016 one minimal Windows-only bridge, exact struct/version checks, no stored
pointer, and dedicated race, layout, failure, and cleanup tests. Rejection of
that exception selects H0 for this Windows candidate rather than dropping Job
limits.

## 6. Browser and native Application mapping

A native Application that speaks the local Interface receives the private
channel directly and is the Stage 7 claim-bearing candidate on each platform.
R-052 found no Stage 7 profile that gives an unmodified visible Firefox tree
the required Ardents-only IPC boundary. The reverse-HTTP browser Adapter is
therefore generic in Stage 7, and `isolated-browser-v1` returns
`isolation-unsupported` without launching a browser.

Generic browsing uses the reverse-HTTP shape without containment and remains
`generic-peer`/`application-networking-unverified`. A loopback token, Firefox
preference, URI association, inherited channel, or Job/cgroup never upgrades
the claim. A future isolated-browser profile needs separate research and an
accepted scope decision.

## 7. Required evidence and decision

R-051 expands S7E1 cells E0-E9 into one exact tuple per platform, principal
class, operation class, and candidate build. It must include positive and
negative observer controls plus:

- correct launch, direct invocation, generic attachment, native private
  channel, and the exact unsupported isolated-browser result;
- hostile same-user endpoint discovery, channel/handle/FD theft and
  duplication, copied challenge, PID reuse, process replacement, debugger/
  ptrace access, child/grandchild/shell/helper escape, and restart replay;
- wrong user/session/token/context/grant/operation, failed credential/token/
  impersonation/Job/cgroup queries, and Broker-privilege crossing;
- every exact process/handle/frame/principal bound and slow/closed peer;
- immediate revoke, finite drain, root crash, Broker crash, Endpoint restart,
  concurrent fork, and zero-residue cleanup; and
- independent process/container/channel/handle observations whose controls
  prove present, absent, or invalidate the episode as unobservable.

One unauthorized operation, escaped/surviving descendant, accepted stale
session, ignored identity failure, unbounded resource, or cleanup miss is
`fail`. A missed control or unavailable required observation is `invalid`, not
pass. The scheduled Ubuntu-Docker/current-Windows subset may establish only the
facts those surfaces expose; unavailable native facts stay `environment-deferred`.
R-051 O2 is selected by ADR-0016 for implementation. Its scheduled subset must
pass before the development handoff, and unavailable native facts remain
qualification gates. Windows install-associated cells still need separate
authorization. R-052 must pass the same launcher tree before any
network-isolated claim is reported as qualified.
