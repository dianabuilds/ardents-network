# Node architecture refactoring plan

Status: **current ownership plan**. The product scope, threat model, accepted
ADRs, [Network/Node technical owner](../technical/network-route-node.md), and
GitHub issues govern behavior and delivery. This document records code
boundaries, not a second task ledger. The initial source graph was checked
against `dev` at `51b38337` on 2026-09-27. The result below describes the
extracted Node packages integrated into `dev` at `a2db90bb` after the combined
`make check` and Linux Docker build/tests. Installed systemd/cgroup evidence
remains a later shared qualification milestone.
The [C0 component reconstruction](c0-component-reconstruction.md) and
[Route boundary record](route-refactoring-boundary.md) retain older snapshots.

## Current boundary and remaining work

`internal/node` composes one process: validate public `Config`, admit one
State-selected duty, start that role, react to process pressure, emit events,
and report the joined terminal result. It does not retain a network role's
listener, connection handlers, spend ledger, or role-specific state. The
private probe has its own owner because it also owns a listener, replay
memory, accepted connections and joined drain.

The children are `internal/node/outer`, `authority`, `hosting`, `resolution`,
`forwarding`, `issuer`, `introduction`, `join`, and `probe`. `hosting` owns the
concrete provider-period handle, shared sampler, late close, class-2
reserve-before-spend order and class-1/3 control envelopes. Node selects its
period and decides process pressure.
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
| Node root | Public process config, copied current duty, local role retention, event writer and process resource guard. | Stop selected duty, emit draining, await bounded joined Drain, then withdraw/release local role and ask Hosting to close its period. Failed or unknown join is FAILED, never WITHDRAWN. |
| `hosting` | Opened provider-period handle, shared sample cache, class-1/2/3 reservation policy and retained late-close lifetime. | Release reservations at their callers; close the concrete ledger only when joined borrowers can no longer use it. Node chooses when protection or drain is required. |
| `outer` | Authenticated outer handshake, accepted connection, callback for inner lanes; private writer queues and child set. | Cancel children, interrupt physical I/O, close bridge, join children and interruption callback, then close handshake. Caller observes final accepted-connection close error. No admission or durable root moves here. |
| Forwarding | Selected receiver/peer facts, certificate, spend ledger, duty limits, host reservations, pool, producers and retained Carrier readers. | Stop listener and producers; join producers before outgoing readers; retire pool and reservations, then close spend root. Retain terminal result across repeated Drain and timeout. |
| `issuer` | Selected issuer profile/receiver, certificate, issuer key root, spend root, token listener, accepted children and their release-error accumulator. | Publish listener terminal cause, join children without releasing roots on caller timeout, record unexpected child release errors, close both roots once and retain close errors. Root Node supplies current State and Hosting reservation callbacks. |
| `resolution` | Selected class-1 receiver, spend root, Descriptor store, listener and workers. | Close listener, join workers, then close store and spend root; retain connection and root errors. Root Node supplies current State callbacks and maps its small handle to process lifecycle. |
| `introduction` | Selected class-3 receiver, spend/slot floors, registrations/deliveries, listener and workers. | Close listener, join workers before replay roots; retain close errors. Qualification evidence observes the actual registration and delivery protocol without a test-only state inspection method in production. |
| `join` | Selected data-join receiver, spend root, pair owner, leased host handle, listener and workers. | Stop listener and host monitor, join workers, close pairs and spend root, then host; retain terminal causes. Root Node supplies current State and class-2 Hosting policy. |
| `probe` | Validated TLS material, fixed private probe request/response, replay memory, listener and accepted connections. | Stop admission, join accepted connections within the drain bound, force close lingering connections, and retain physical close failure. Node selects its authenticated duty and supervises the handle. |

The forwarding child now owns its listener, session set, receiving resources,
pool, accepted handlers and joined shutdown. Node retains profile validation,
address choice, process admission and
role selection. It opens the Host handle and transfers its late close to the
child. `internal/node/hosting` owns the common class-2 reserve-before-spend
policy used by forwarding and JOIN; the distinct class-1/3 control envelopes
also belong to Hosting.
The child borrows current State, authority, token policy and endpoint validation
through explicit dependencies, never `runtimeConfig`. It rechecks State at each
existing admission point and cannot accept a plan-supplied peer or key.
The root files named `forwarding.go`, `issuer.go`, `resolution.go`,
`introduction.go`, and `join.go` are process adapters: they validate the selected
local profile, choose its listen address and map each child handle to lifecycle
supervision. `hosting.go` selects the shared period and interprets pressure;
`state_authority.go` projects current State; `listen_address.go` validates the
private bind override. The private `closed_*` protocol identifiers remain where
they are part of the accepted contract, but the root file names no longer use
that prefix as a substitute for an owner.
Within the forwarding package, the listener owns accepted producers; the
session set owns each retained outgoing Carrier reader and its child frame
queues; a link joins one child lane to that session. They share one shutdown
order, so a separate nested package would expose more mutable session and link
state than the current private types. Source files follow these owners:
`listener.go` includes accepted close and shutdown, `session.go` includes the
queue, `bootstrap.go` includes bootstrap admission, and `recipient.go` includes
the relay dial rule. Tests of session ordering, queue and terminal outcomes
share one `session_test.go`; Linux-specific tests retain their tags when they
need the installed resource/Carrier fixture.
Process pressure remains in the root; per-duty local limits and JOIN host
monitoring remain with their duty.

## Extraction sequence and open audit

1. **Accepted outer connection.** The service and writer moved together while
   queue fairness, deadlines, cancellation, child join and the role-owned
   accepted-close result remained covered at their callers.
2. **Shared authority and hosting seams.** Roles now borrow explicit duty
   facts and current-State callbacks. Authority owns current-role projection;
   Hosting owns the shared class-2 reserve-before-spend rule.
3. **Forwarding.** Its listener, receiving resources, admission, sessions,
   links, queue, bootstrap and joined shutdown moved as one duty. Startup
   rollback remains separate from transferred server resources.
4. **Direct roles.** Resolution, issuer, introduction and join own their
   listeners, admitted work, durable roots and joined drain. Root adapters
   retain only process dispatch, admission and address selection. Credential's
   engine and Route's receiving operations remain with their owners.
5. **Process cleanup.** Confirmed dead declarations and stale comments were
   removed; the root retains composition, admission, pressure, evidence and
   terminal lifecycle. File boundaries follow responsibility, not length.

The earlier integration completed those five slices, but did not satisfy the
whole target. The private probe now has its own listener owner. The issuer role
now retains its own child release errors; root no longer passes release-state
callbacks into the role. Remaining work is to audit the root listener adapters
and review the forwarding package's internal responsibilities and Linux-tagged
test layout.
The `closed_*` file family includes both process adapters and role integration
tests; organize it by actual owner and test purpose rather than preserving a
file for each isolated assertion. Re-run combined checks before integration.

For each slice, update the technical owner and package map with code, run
focused behavior tests and `make quick-check`, and commit a coherent result.
Before integration run `make check`, Linux build and available tests in Docker
on the combined tree. Installed systemd/cgroup operation and both Carriers
remain a later shared qualification gate; container tests cannot establish
that installed evidence.

## Coordination and integration

The Node owner works in the main checkout on `codex/node-decomposition`; the
Endpoint owner works in its separately registered worktree (current path in
`C:\Users\vitek\code\ardents-coordination\endpoint.md`). Both read the
same coordination directory outside their worktrees before a new slice, a
shared interface edit, or integration. Assign exactly one implementer to
each bounded Route, Credential, common-command, or shared-interface change.
Independent work does not wait for a reply. Integrate only named, locally
verified commits into `dev`; preserve unfinished work in its original tree
and resolve combined failures at their actual owner. A passing component
test does not establish full issue acceptance or installed readiness.
