# Node architecture refactoring plan

Status: **current ownership plan**. The product scope, threat model, accepted
ADRs, [Network/Node technical owner](../technical/network-route-node.md), and
GitHub issues govern behavior and delivery. This document records code
boundaries, not a second task ledger. The source graph below was checked
against `dev` at `51b38337` on 2026-09-27; recheck it before each move.
The [C0 component reconstruction](c0-component-reconstruction.md) and
[Route boundary record](route-refactoring-boundary.md) retain older snapshots.

## Result and dependency direction

`internal/node` composes one process: validate public `Config`, admit one
State-selected duty, start that role, react to process pressure, emit events,
and report the joined terminal result. It does not retain a network role's
listener, connection handlers, spend ledger, or role-specific state. The
private probe remains under the process owner.

The intended children are `internal/node/outer`, `authority`, `forwarding`,
`issuer`, `resolution`, `introduction`, and `join`. A narrow shared hosting
reservation owner may be extracted when its exact caller values are known.
Each new package must have `doc.go`, behavior tests,
a non-test caller, and registered imports. A role never imports its parent or
receives `runtimeConfig`; the root adapts the role's small
`Done`/`Stop`/`Drain`/usage surface to process lifecycle.

```text
State accepted views             Resource ledger and measurements
        |                                      |
        v                                      v
Node process composer  -------->  selected role owner
        |                                      |
        |                                      +--> Node outer --> Route bridge/ARDP
        |                                      +--> Route receiving operation
        |                                      +--> Carrier (TCP/TLS or QUIC)
        |                                      +--> Replay / role-owned durable root
        +--> process event and terminal result
```

Route owns wire, authenticated Carrier, bridge state, and receiving operations;
it neither selects a Node duty nor closes a role's durable root. Credential
owns issuer key and token operations; the Node issuer role owns their listener
and late root close after workers join. State alone authenticates current
duty/profile/recipient views. Resource owns measurement and the shared Hosting
ledger; Node decides whether to protect or drain. No child imports the root.

## Source and resource map

At the checked base the Node root had 52 production files. `runtimeConfig`
embedded all of `Config`; every role could read unrelated local settings.
Five production callers used `serveClosedOuter` and its serialized writer.
The first boundary places that operation in `internal/node/outer`; the
accepted connection's final physical close result stays with each role's
accept loop.

| Owner | Inputs and state | Stop and final result |
| --- | --- | --- |
| Node root | Public process config, copied current duty, local role retention, event writer, process resource guard, hosting-period handle. | Stop selected duty, emit draining, await bounded joined Drain, then withdraw/release local role and close hosting period. Failed or unknown join is FAILED, never WITHDRAWN. |
| `outer` | Authenticated outer handshake, accepted connection, callback for inner lanes; private writer queues and child set. | Cancel children, interrupt physical I/O, close bridge, join children and interruption callback, then close handshake. Caller observes final accepted-connection close error. No admission or durable root moves here. |
| Forwarding | Selected receiver/peer facts, certificate, spend ledger, duty limits, host reservations, pool, producers and retained Carrier readers. | Stop listener and producers; join producers before outgoing readers; retire pool and reservations, then close spend root. Retain terminal result across repeated Drain and timeout. |
| Issuer | Selected issuer profile/receiver, certificate, issuer key root, spend root, token listener and accepted children. | Publish listener terminal cause, join children without releasing roots on caller timeout, close both roots once and retain close errors. |
| Resolution | Selected class-1 receiver, spend root, Descriptor store, listener and workers. | Close listener, join workers, then close store and spend root; retain connection and root errors. |
| Introduction | Selected class-3 receiver, spend/slot floors, registrations/deliveries, listener and workers. | Close listener, join workers before replay roots; retain close errors. |
| Join | Selected data-join receiver, spend root, pair owner, host reservation, listener and workers. | Stop listener and host monitor, join workers, close pairs and spend root, then host; retain terminal causes. |

The Node authority child borrows current profile/Route views, projects the
receiver and shared peer, and verifies class-1/2/3 tokens. The root still
supplies the copied duty and keeps process admission and role selection.
The child rechecks State at each existing admission
point and never accepts a plan-supplied digest, role, peer, or key. Forwarding's
class-2 host charge and control's class-1/3 lifetime charge remain distinct
policy callers. Hosting reservations use Resource's ledger with one Node owner
for allocation and release. Process pressure stays in the root; per-duty local
limits and JOIN host monitoring stay with the duty.

## Bounded extraction order

1. **Accepted outer connection.** Extract service and writer together.
   Preserve queue fairness, deadlines, cancellation, child join and the
   role-owned accepted-close result. Move owner tests with the implementation;
   keep role-level close and forwarding tests at their callers.
2. **Shared authority and hosting seams.** Replace role consumption of
   embedded `runtimeConfig` with explicit duty facts, callbacks for current
   State rechecks, and only that role's roots and certificate. Decide shared
   packages from actual callers; do not copy authority checks.
3. **Forwarding.** Move listener, receiving-resource group, admission,
   sessions, links, queue, bootstrap and shutdown as one duty. Keep startup
   rollback separate from transferred server resources. Verify both Carriers.
4. **Direct roles.** Extract issuer, resolution, introduction and join at
   listener/admitted-work/drain boundaries, one finished role at a time.
   Keep root adapters only for process dispatch. Credential's engine and
   Route's receiving operations stay in their existing packages.
5. **Process cleanup and review.** Delete confirmed dead declarations, repair
   comments and names, and leave root composition, admission, pressure,
   evidence and terminal lifecycle. File splits follow cohesion, not length.

For each slice, update the technical owner and package map with code, run
focused behavior tests and `make quick-check`, and commit a coherent result.
Before integration run `make check`, Linux build and available tests in Docker
on the combined tree. Installed systemd/cgroup operation and both Carriers
remain a later shared qualification gate; container tests cannot establish
that installed evidence.

## Coordination and integration

The Node owner works in the main checkout on `codex/node-decomposition`; the
Endpoint owner works in the existing architecture worktree. Both read the
same coordination directory outside their worktrees before a new slice, a
shared interface edit, or integration. Assign exactly one implementer to
each bounded Route, Credential, common-command, or shared-interface change.
Independent work does not wait for a reply. Integrate only named, locally
verified commits into `dev`; preserve unfinished work in its original tree
and resolve combined failures at their actual owner. A passing component
test does not establish full issue acceptance or installed readiness.
