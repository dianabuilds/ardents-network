---
id: R-051
title: Which local-channel and process facts bind a Stage 7 Application Principal?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-051 — Stage 7 Application Principal and local IPC

## Decision this unlocks

Select Ubuntu and Windows local IPC and launcher/process-tree identity Adapters
for the Application Broker in S7.5. Freeze which OS facts jointly constitute a
claim-bearing Application Principal and which generic attachments remain one
coarse trust domain.

## Current contract

R-024, R-048, ADR-0007, the
[Application-principal decision proposal](../../development/stage-7-launcher-bound-application-principals-proposal.md),
the exact
[Application Principal specification](../../development/stage-7-application-principal-spec.md),
and the lifecycle specification
reject PID, desktop user, loopback/pipe/socket path, and copyable bearer alone.
The broker must bind one fresh launcher-born process tree/session, exact channel,
Local Grant, Isolation Context, resources, broker start, and deadline. Connection,
Service Administration, and Authority Custody remain disjoint.

## Hypotheses

- **H1:** a launcher-created named/scoped endpoint plus joint OS/process-tree
  facts is sufficient for a claim-bearing principal.
- **H2:** one private inherited channel, non-reusable root process handle, and
  complete launcher-owned tree are jointly required for claim-bearing
  principals, while named endpoints remain generic only.
- **H0:** a supported platform cannot distinguish hostile same-user Applications
  without the full isolation mechanism; its non-isolated profile must remain
  coarse/unqualified.

## Evaluation criteria

- binding occurs before untrusted Application work;
- exact server and client ownership, session, channel, start identity, and
  complete process tree are observable and non-reusable;
- no PID-reuse, process-replacement, endpoint-name, symlink/reparse, remote-pipe,
  inherited-handle, or bearer-only acceptance;
- failed peer/token/impersonation/Job/ownership query fails closed without
  continuing under broker privilege;
- fresh post-restart binding and replay protection;
- hierarchical resource admission/backpressure and revoke/drain semantics;
- Windows/Ubuntu parity, supported Go/library surface, license/advisory closure,
  and no cgo, driver, or permanent broad privileged broker; any unavoidable
  fixed Win32 `unsafe` bridge requires an explicit ADR exception and dedicated
  risk tests before code; and
- bounded paths, frames, sessions, processes, handles/FDs, queues, deadlines,
  diagnostics, and cleanup.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- Linux [`unix(7)` peer credentials](https://man7.org/linux/man-pages/man7/unix.7.html);
- Linux kernel [cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
  process membership, delegation, recursive populated state, `pids.max`, and
  `cgroup.kill`;
- Go 1.26 [`os.Process.WithHandle`](https://go.dev/pkg/os/?m=all%2Cold)
  and Linux
  [`UseCgroupFD`/`PidFD`](https://go.dev/src/syscall/exec_linux.go);
- Microsoft [CreateProcess](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw)
  and [process creation flags](https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags);
- Microsoft [named-pipe security](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights);
- Microsoft [GetNamedPipeClientProcessId](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getnamedpipeclientprocessid);
- Microsoft [ImpersonateNamedPipeClient](https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-impersonatenamedpipeclient);
- Microsoft [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects),
  [AssignProcessToJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-assignprocesstojobobject),
  and [Job limits](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information); and
- Go 1.26 `os/exec`/`syscall` source and the already pinned
  `golang.org/x/sys/windows` v0.45.0 source for exact candidate surfaces.

### Experiment

Create `experiments/r-051-stage-7-application-principal/` only after the exact
host-image and observer-binary manifest values required by the
[Stage 7 host-campaign specification](../../development/stage-7-host-campaign-spec.md)
are frozen. On Ubuntu, falsify an inherited
unnamed Unix socketpair plus atomic cgroup-v2 placement, pidfd, process cap, and
recursive kill/empty observation. On Windows, falsify two inherited anonymous
pipes plus suspended root, token/session facts, non-breakaway Job/process cap,
kill-on-close, and stable process handle. Also run direct-invocation and named
generic controls. Attempt channel discovery/substitution, credential/bearer
copy/replay, PID reuse, child replacement/breakaway, handle/FD inheritance,
theft and duplication, failed identity query, broker/Application/Endpoint
restart, cross-grant operations, pressure, revoke/drain, crash, and cleanup.
Retain OS-observed process/channel facts independently of candidate reports.

### Failure scenarios

Identity query after request processing; impersonation failure ignored; broker
acts with its own privilege; PID/path/username accepted alone; server spoofing;
old session accepted after restart; child escapes ownership; grant family
crossing; other context diagnostics/state visible; and incomplete object/process
cleanup.

## Falsification criteria

Freeze the launcher/channel/process corpus before running a candidate. H1/H2 is
falsified on a host if any hostile sibling, copied bearer, PID reuse, process
replacement, channel substitution, breakaway, inherited-handle, restart, or
failed identity-query case obtains one unauthorized operation; if binding occurs
after untrusted work starts; if the broker continues under its own privilege;
or if any descendant/session/object survives its cleanup deadline.

The research envelope is `32` processes and `1,024` handles/FDs per Application
tree, `64` concurrent principals, `256` queued frames and one private channel
per principal, `64 KiB` per frame, a Linux hard `512`-task ceiling, and a `5 s`
revoke/termination deadline. Pre-admission/frame excess rejects before work;
the first observed runtime-inventory excess fails the principal closed and
starts immediate revocation. Every required attack passes exactly; averaging
is forbidden. If a required host exposes no stable facts sufficient for the
claim-bearing profile, select O0 for that profile rather than accepting PID,
user, path, or bearer alone.

## Findings

- **Sourced fact:** Linux connected Unix sockets can expose credentials fixed at
  connection/listen/socketpair time. Therefore `SO_PEERCRED` on a socketpair
  created before launch describes the Broker at creation, not the later child;
  one UID also commonly identifies several same-user Applications.
- **Sourced fact:** Go 1.26 can expose a stable Linux pidfd/Windows process
  handle. Its Linux launch surface can request pidfd and atomically place a
  child into a cgroup-v2 FD through `clone3(CLONE_INTO_CGROUP)`.
- **Sourced fact:** cgroup v2 births descendants in the parent's cgroup, exposes
  recursive populated state and a hard task-ID limit, and `cgroup.kill` handles
  concurrent forks and migration while killing the tree. The PID controller
  counts threads as tasks, not only processes; delegation and write exposure
  still determine whether the Application can escape ownership.
- **Sourced fact:** Windows named-pipe APIs expose client PID and can impersonate
  the last client message, but documentation warns that failure leaves the server
  in its own context and must be checked.
- **Sourced fact:** Windows Job Objects normally associate CreateProcess
  children, can prohibit breakaway, cap processes, and kill the tree when the
  last Job handle closes. A suspended root lets the Broker assign the Job and
  inspect token/session facts before Application work.
- **Measured source-surface fact:** Go 1.26 exposes creation flags and explicit
  inherited handles, and pinned `x/sys/windows` exposes the required Job calls.
  Its `SetInformationJobObject` wrapper still accepts the limit structure as a
  `uintptr`; setting the mandatory Job flags from first-party Go therefore
  currently needs a narrowly reviewed `unsafe.Pointer` bridge or a different
  maintained safe surface.
- **Inference:** OS peer facts are necessary but not sufficient; launcher start
  identity and owned non-breakaway tree must be joined before a Local Grant is
  active.
- **Inference:** R-052 selects native claim-bearing Applications for Stage 7.
  The generic reverse-HTTP browser Adapter remains coarse/unverified, while an
  isolated-browser request is explicitly unsupported rather than being treated
  as an R-051 principal merely because its processes share a tree.

## Options

- **O1:** launcher-created named/scoped channel plus joint OS/process-tree facts.
- **O2:** private inherited channel as the claim-bearing Interface; named channel
  is generic/coarse only.
- **O0:** keep all same-user Applications one generic trust domain on a failing
  platform and stop the claim-bearing profile.

## Recommendation

Advance O2 as the exact falsification candidate on both platforms. Keep named
Unix sockets/named pipes as O1 generic attachment only. Keep direct binary as a
co-resident per-invocation Adapter with claim `none`, not as a degraded IPC
path. Do not accept a Windows implementation until a maintained safe Job-limit
surface is proved or ADR-0016 explicitly authorizes and bounds the minimal
fixed-structure bridge. Confidence: medium-low before host experiment.

## Disposition

- State: `decided`; the Product Owner selected O2 for Stage 7 development on
  2026-08-20 under ADR-0016.
- The [Application Principal specification](../../development/stage-7-application-principal-spec.md)
  freezes principal classes, joint identity, lifecycle order, Ubuntu/Windows
  candidates, numeric envelope, browser mapping, and failure rules.
- The scheduled Ubuntu-Docker/current-Windows experiments and observer controls
  remain S7.5 evidence gates. Unobservable native qualification remains
  explicitly deferred.
- ADR-0016 accepts only the minimal fixed-structure Windows `unsafe.Pointer`
  bridge with dedicated layout/lifetime/race/failure/cleanup tests.
- R-052 separately owns network confinement; principal success alone carries no
  Application-network privacy claim.
