# Node architecture refactoring plan

Status: **current ownership plan**. The product scope, threat model, accepted
ADRs, [Network/Node technical owner](../technical/network-route-node.md), and
GitHub issues govern behavior and delivery. This document records code
boundaries, not a second task ledger. The initial source graph was checked
against `dev` at `51b38337` on 2026-09-27. The current ownership, shutdown
order, and test layout below were rechecked against `dev` at `8063040b` on
2026-09-28; individual integration receipts remain in Git history.
Installed systemd/cgroup evidence remains a later shared qualification
milestone.
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

Route owns wire, authenticated Carrier, bridge state, and receiving admission,
queue, credit, and JOIN operations. Its client and credential packages also
recheck copied State authority at their use points. Route neither selects a
Node duty nor closes a role's durable root. Credential owns issuer key and
token operations; the Node issuer role owns their listener and late root close
after workers join. State alone authenticates current duty/profile/recipient
views. Resource owns measurement and the provider-period ledger primitive;
`node/hosting` owns the opened ledger handle and shared reservation policy.
Node decides whether to protect or drain. No child imports the root.

## Source and resource map

At the initial checked base the Node root had 52 production files.
`runtimeConfig` embedded all of `Config`, and the roles could read unrelated
process settings. Five production callers used `serveClosedOuter` and its
serialized writer. That accepted-connection lifetime now belongs to
`internal/node/outer`; the final physical close result remains with each
receiving role's accept loop.

The current root keeps process composition, admission, pressure, events and
terminal cleanup. `duty_server.go` holds role dispatch and its narrow
supervision handle; `process_config.go` keeps public inputs beside the
process's retained runtime state; `lifecycle_event.go` keeps event/result
values beside their bounded emitter. Platform-specific event writers remain
separate files. File counts are navigation evidence, not an extraction target.

| Owner | Inputs and state | Stop and final result |
| --- | --- | --- |
| Node root | Public process config, copied current duty, local role retention, event writer and process resource guard. | Stop selected duty, emit draining, await bounded joined Drain, close shared Hosting and remove the local-role record before publishing WITHDRAWN. Failed or unknown join is FAILED; an unjoined role retains its conflict record to authenticated expiry and delays Hosting close until join. |
| `hosting` | Opened provider-period handle, shared sample cache, class-1/2/3 reservation policy and retained late-close lifetime. | Release reservations at their callers; close the concrete ledger only when joined borrowers can no longer use it. Node chooses when protection or drain is required. |
| `outer` | Authenticated outer handshake, accepted connection, callback for inner lanes; private writer queues and child set. | Cancel children, interrupt physical I/O, close bridge, join children and interruption callback, then close handshake. Return the first physical close result to the receiving role, including closure after a partial write; that role retains the final accepted-connection result. No admission or durable root moves here. |
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
Route's receiving channel owns queued-frame accounting and credit. The Node
forwarding link drains those channels into child lanes and owns its downstream
writers. A change to either side of that boundary needs one agreed contract
and one implementer per affected package so that admission and retained
resources remain accounted for across the handoff.
The root files named `forwarding.go`, `issuer.go`, `resolution.go`,
`introduction.go`, and `join.go` are process adapters: they validate the selected
local profile, choose its listen address and map each child handle to lifecycle
supervision. `hosting.go` selects the shared period and interprets pressure;
`state_authority.go` projects current State; `listen_address.go` validates the
private bind override. The private `closed_*` protocol identifiers remain where
they are part of the accepted contract, but the root file names no longer use
that prefix as a substitute for an owner.
The process composer retains `runtimeConfig`, including its copied public
`Config`. Each role adapter receives only the selected local profile, copied
State duty, and a private `roleInputs` projection of current State authority,
bounded duty refresh, clock and listener override. The issuer, resolution and
introduction adapters receive the shared Hosting handle separately for their
control admission; forwarding and JOIN do not receive that handle through the
common projection.
The refresh callback retains only the public `Current` function. Admission
validation receives an authority projection with its one-poll State view
rather than the full process configuration; no child role receives process
pressure or event state.
Admission and startup use one private ordered role selector. Admission checks
the captured State view at its single poll time; startup rechecks the current
State view with its live clock before transferring resources to the selected
role. Profile validation and process failure decisions remain in the root.
Each role's `Start` checks the borrowed callbacks it will invoke before
opening its own roots or listener. Forwarding receives a Host handle already
opened by Node; an incomplete dependency set closes that handle and returns
both the prerequisite error and any close error. The other roles acquire their
own resources only after this check. These are role-local contracts rather
than a shared generic validator.
Within the forwarding package, the listener owns accepted producers; the
session set owns each retained outgoing Carrier reader and its child frame
queues; a link joins one child lane to that session. They share one shutdown
order, so a separate nested package would expose more mutable session and link
state than the current private types. Source files follow these owners:
`listener.go` includes accepted close and shutdown, `session.go` includes the
queue, `bootstrap.go` includes bootstrap admission, and `recipient.go` includes
the relay dial rule. Tests of session ordering, queue and terminal outcomes
share one `session_test.go`; Linux-specific tests retain their tags when they
need the installed resource/Carrier fixture. Outgoing Carrier opening outcomes,
deadline and same-key reuse share `open_linux_test.go`. The parent connection's
progress during a blocked child open or write is covered together in
`parent_progress_linux_test.go`. The package-local recipient and relay cases now
share `recipient_selection_test.go`; bootstrap admission and exact adjacency
share `bootstrap_accept_test.go`. Session cancellation and parent expiry share
`session_lifetime_test.go`. Accepted close, a delayed session reader and the
fake shared-Hosting sampler share `listener_lifecycle_test.go`, because each
checks the listener's joined shutdown. The portable role projection fixture
supports those tests and the Linux Carrier scenarios.
At the process boundary, forwarding tests check the recipient owner directly.
Root network tests call `Run` or a private process adapter and
exercise combinations of roles; moving them to a role package would transfer
process authority or require a new exported test seam. Related scenarios and
their single-purpose fixtures now share files: bootstrap exchange/forwarding,
forwarding lifetime/restriction, issuer lifecycle, Introduction observation,
recovery and heap-process delivery, and Resolution's admitted-network and Host
release behavior. The cross-role recipient fixture stays in one file, while
its pure window calculation runs on both platforms. Linux-only tests use the
`_linux_test.go` suffix for real Carrier, role-root or permission-sensitive
network work. Process lifecycle, admission, identity, pressure and event tests
remain beside their production owners; component behavior tests live in the
role packages.
The root's Linux-tagged tests exercise its journal Unix socket, actual
TCP/TLS and QUIC processes, selected role roots and Hosting fixtures.
Forwarding, issuer and hosting keep their Linux-only Carrier, root and ledger
tests beside those component owners. The pure root
admission projection case now lives in untagged `admission_test.go`; the
direct bootstrap adjacency table lives with forwarding. Forwarding's
in-memory bootstrap acceptance, projection fixture and fake Host/listener
reaper run on Windows as well as Linux. Linux-only fixture helpers for actual
network scenarios stay in a tagged file because Windows has no callers.
Its remaining Linux tests use actual Carriers, replay/spend roots or
permission-sensitive paths. Hosting samples
its Linux ledger; issuer tests late root close with the Linux fixture. Other
fixture-only Linux files have solely Linux callers. The source filename
matches each Linux build tag. Windows `make check` covers the portable
assertions; the read-only Linux Docker Node run covers the selected Linux
scenarios. Neither run qualifies the installed systemd/cgroup startup profile.
The isolated four-reader Endpoint scenario passed with the existing two-second
Introduction drain bound under Docker quotas of 2, 1 and 0.25 CPUs. CPU quota
alone did not reproduce the earlier host-load Node 6 cleanup deadline. A
deterministic Introduction lifetime test holds one accepted worker after Stop:
Drain times out, the spend root stays open while that worker owns it, and the
role joins and closes the root after the worker returns. This proves the late
resource-release boundary, but does not identify which worker or operation
exceeded the bound in the four-reader run. The remaining diagnostic need is a
goroutine dump and joined/active observation at that exact deadline if it
recurs; a later Source CLOSE/CREDIT EOF under host load is a separate failure
shape. No timeout or protocol contract was changed on the basis of these runs.
Process pressure remains in the root; per-duty local limits and JOIN host
monitoring remain with their duty.

## Extraction sequence and verification

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

The earlier integration completed the five original slices. The follow-up audit
extracted the private probe, moved the concrete Hosting ledger and class-1/3
reservation policy to `hosting`, and placed the issuer child release errors in
`issuer`. The root's process adapters now use role names rather than the
`closed_*` filename prefix. Forwarding's listener, session, bootstrap and
recipient files follow their resource owners. Small related tests were grouped,
one direct recipient test moved to the forwarding owner, and Linux-only
integration scenarios now carry explicit platform suffixes. The root has no
`closed_*` source files; retained `Closed*` identifiers are protocol or public
configuration names, not file grouping.

The subsequent test-layout audit removed isolated files for one scenario's
fixture and grouped complete process-network scenarios by role and failure
mode. The production root keeps process composition and observations; role
packages still own listeners, admitted work and durable resources.

The completed Node ownership and Endpoint qualification slices entered `dev`
at `5a757e49`. The root-file audit and Endpoint publication consolidation
entered `dev` at `59026c64`. Windows `make check` passed on the latter combined
revision, including E2E and race tests. A read-only Linux Docker build plus
Node, qualification, worker, replacement and targeted Endpoint tests also
passed. The full Endpoint Linux battery for the publication slice was run by
its owner before integration; the combined run used targeted Endpoint tests.
Installed systemd/cgroup evidence remains a later shared qualification gate.

For each slice, update the technical owner and package map with code, run
focused behavior tests and `make quick-check`, and commit a coherent result.
Before integration run `make check`, Linux build and available tests in Docker
on the combined tree. Installed systemd/cgroup operation and both Carriers
remain a later shared qualification gate; container tests cannot establish
that installed evidence.

## Coordination and integration

The Node owner integrates ready slices in the main `dev` checkout. Endpoint,
Route and Network owners work in their separately registered worktrees;
their current paths and active slices are recorded in
`C:\Users\vitek\code\ardents-coordination\`. All owners read that shared
directory outside their worktrees before a new slice, a shared interface edit,
or integration. Assign exactly one implementer to each bounded Route,
Credential, common-command, or shared-interface change.
Independent work does not wait for a reply. Integrate only named, locally
verified commits into `dev`; preserve unfinished work in its original tree
and resolve combined failures at their actual owner. A passing component
test does not establish full issue acceptance or installed readiness.
