---
id: R-051
title: Which local-channel and process facts bind a Stage 7 Application Principal?
status: open
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
and the lifecycle specification
reject PID, desktop user, loopback/pipe/socket path, and copyable bearer alone.
The broker must bind one fresh launcher-born process tree/session, exact channel,
Local Grant, Isolation Context, resources, broker start, and deadline. Connection,
Service Administration, and Authority Custody remain disjoint.

## Hypotheses

- **H1:** launcher-created scoped Unix-domain IPC plus peer credentials and
  process ownership on Ubuntu, and launcher-created named/inherited pipe plus
  token/session/process/Job facts on Windows, meet the same Interface.
- **H2:** a private inherited anonymous channel is required for claim-bearing
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
  and no cgo/unsafe or permanent privileged broker; and
- bounded paths, frames, sessions, processes, handles/FDs, queues, deadlines,
  diagnostics, and cleanup.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- Linux [`unix(7)` peer credentials](https://man7.org/linux/man-pages/man7/unix.7.html);
- Microsoft [GetNamedPipeClientProcessId](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getnamedpipeclientprocessid);
- Microsoft [ImpersonateNamedPipeClient](https://learn.microsoft.com/en-us/windows/win32/api/namedpipeapi/nf-namedpipeapi-impersonatenamedpipeclient);
- Microsoft [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects); and
- Go `net`, `os/exec`, and selected `x/sys` source for exact candidates.

### Experiment

Create `experiments/r-051-stage-7-application-principal/`. On each frozen host,
launch authorized client/publisher/admin trees and hostile same-user siblings.
Attempt channel discovery/substitution, bearer copy/replay, PID reuse, child
replacement/breakaway, handle inheritance/duplication, remote pipe use, failed
identity query, broker/Application/Endpoint restart, cross-grant operations,
pressure, revoke/drain, crash, and cleanup. Retain OS-observed process/channel
facts independently of candidate reports.

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

The research envelope is `32` processes per Application tree, `64` concurrent
principals, `256` queued frames per principal, and a `5 s` revoke/termination
deadline. The candidate must reject the first observation beyond each bound and
must pass every required attack exactly; averaging is forbidden. If a required
host exposes no stable facts sufficient for the claim-bearing profile, select
O0 for that profile rather than accepting PID, user, path, or bearer alone.

## Findings

- **Sourced fact:** Linux connected Unix sockets can expose credentials fixed at
  connection/listen/socketpair time; one UID still commonly identifies several
  same-user Applications.
- **Sourced fact:** Windows named-pipe APIs expose client PID and can impersonate
  the last client message, but documentation warns that failure leaves the server
  in its own context and must be checked.
- **Sourced fact:** Windows Job Objects normally associate CreateProcess children,
  but breakaway flags and creation paths affect complete-tree coverage.
- **Inference:** OS peer facts are necessary but not sufficient; launcher start
  identity and owned non-breakaway tree must be joined before a Local Grant is
  active.

## Options

- **O1:** launcher-created named/scoped channel plus joint OS/process-tree facts.
- **O2:** private inherited channel as the claim-bearing Interface; named channel
  is generic/coarse only.
- **O0:** keep all same-user Applications one generic trust domain on a failing
  platform and stop the claim-bearing profile.

## Recommendation

Compare O1 and O2 per platform. Prefer the smallest Interface that survives all
hostile cases; do not force platform symmetry by dropping a required fact.
Confidence: medium before experiment.

## Disposition

- State: `open`; no IPC, dependency, or OS identity profile is selected.
- Required before the proposal can be promoted to ADR-0016 and before S7.5.
- R-052 separately owns network confinement; principal success alone carries no
  Application-network privacy claim.
