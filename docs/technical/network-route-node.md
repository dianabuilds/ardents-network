# Network State, Entry, Route, and Node

Status: **current maintained technical contract.** This document describes the
implemented closed-test-network Modules and their current Interfaces. It does
not claim public network operation, independent operators, public discovery,
supported Node hosting, or Route qualification.

The [selected architecture](common-privacy-architecture.md) under
[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md) and
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) uses the
[protected forwarding contract](protected-route-protocol.md) for the current
closed v3 path. This document describes both implemented closed duties and
retained generation-2 grammar. The latter remains migration or refusal input,
not a second accepting privacy path.

The Node command connects an exclusive `closed_forwarding` local reservation
to the implemented generation-3 forwarding receiver. The reservation contains
only the existing receiving-spend root and finite connection/drain limits.
It selects the closed State profile only with its already State-pinned signer;
State's accepted profile and duty projection continue to select the listener,
Carrier, adjacent/interior assignment and next peers. A legacy duty, issuer
reservation or unrelated profile signer cannot be combined with that plan.
This command binding does not implement the remaining Endpoint composition or
establish whole-route qualification.
Forwarding startup constructs the spend ledger, duty limits and bootstrap
controller as one private concrete Node owner before it creates or transfers a
server. Until that group is complete, its builder owns rollback and closes the
exact spend-root lease once; an initialization failure retains both its initial
cause and any cleanup cause. The listener, outgoing pool and borrowed-or-local
host remain separate composition owners, so this grouping neither relocates
their policy nor adds a hidden host close.
For a generation-3 TCP Node Carrier, terminal retirement closes the owned
physical socket once and retains its actual close result. It interrupts the
multiplexed transport instead of initiating another TLS notification after a
peer reset. Lane owners still join readers, writers and children before
releasing their roots. This terminal abort is distinct from directional ARDP
EOF and authenticated Service completion; direct role and inner TLS closure
keep their existing semantics.
On State loss or an accepted successor, the Node lifecycle stops the old
forwarding duty, closes its listener and outgoing pool, and waits for every
accepted handler before the outgoing-session owner performs the final wait for
its retained Carrier readers and returns their joined cleanup result. The server
retains the pool interruption and spend-root lifetime, so a timed-out Drain
cannot release the root or turn a later physical close failure into success.
While one child is pending downstream HELLO/ACCEPT, the parent reader still
serves lane-zero control and independently selected children. A pending child's
frames remain in Route's bounded accounted queues; CLOSE cancels and joins only
that child opener before its reservation is released.
An old reader can invalidate only its exact Carrier lease incarnation, so a
late terminal result cannot close a replacement with the same public key.

The shared successor listener gives each arriving connection its own bounded
handshake/first-stream interval. Waiting without a peer does not consume that
interval or make the next valid peer inherit an expired deadline. Issuer, forwarding and resolution consumers use this interface; current State classification and
subsequent HELLO/admission deadlines remain separate checks.
For a retained forwarding Carrier, one exact-key creator owns outer HELLO/ACCEPT
I/O; same-key callers wait for that terminal result and receive the same live
session only when their leases name the same incarnation. A blocked creator
does not hold the session map lock, so an unrelated ready Carrier continues to
open and carry child work. A waiter can cancel without canceling that creator;
the creator publishes only after its cancellation close callback has joined.

The exclusive `closed_resolution` reservation similarly connects the selected
resolution duty to private Descriptor publication and lookup. It supplies
separate Descriptor and admission-spend roots plus finite connection/drain
limits; State owns its identity, endpoint, Carrier and role. Only current
State-authorized outer Node Carriers may open the confidential recipient
channel. Its class-1 token is spent before the operation; the actual Store
verifies the proof and persists its floors before success. Current profile and
Introduction assignment are checked before accepting or returning a proof.
Shutdown cancels children and joins handlers before releasing either root;
a timed-out Drain leaves the roots held. No plan callback can supply a
successful publication or bypass verification.
Short local-role transactions coordinate with concurrent Source exposure
retention. `duty.OpenOperation` waits only for an occupied exclusive lease,
for at most one second or the caller's earlier cancellation. It then verifies
the current durable generation under that lease; a busy, corrupt, expired or
unavailable root never becomes a no-conflict result. Source exposure updates
and one-shot conflict reads use this bounded acquisition. The existing
`duty.Open` remains non-waiting for retained owners. Acquisition cancellation
does not release another owner's lease, and every successful caller still
closes its own store. This local coordination does not extend any Route,
permission, registration or protocol deadline.

## Old start retirement

[ADR-0089](../adr/0089-retire-old-node-starts-preserve-owned-shutdown.md)
selects an effect-free refusal for every new old execution selection:

- Node reservations `rendezvous`, `initiator`, `introduction`, `responder`,
  and `transit_issuer`, including their old runtime assignments;
- Source `native_rendezvous_profile`;
- `ardents-transit-issuer-initialize-v1` and issuer serve selected by the old
  Transit issuer reservation; and
- Contributor `apply` and `restart`, for canonical and historical profile
  inputs.

Each adapter must identify and refuse its old selection before opening or
creating a state root or key, binding a listener, invoking a supervisor, or
starting Network work. Refusal cannot select another profile or fall back to a
closed duty. The plan and command schemas are not retired wholesale: probe
behavior is outside this decision, and `closed_issuer`, `closed_forwarding`,
`closed_resolution`, `closed_introduction`, `closed_data_join`, the explicit
closed Source profile, and closed issuer initialize/serve retain their exact
existing authority checks.

Existing old roots, keys, floors, plans, and installation records remain
unchanged evidence. Historical profile recognition may authenticate an already
pinned owned installation for retirement only; it cannot authorize execution
of an old duty or rewrite persisted identity. The old Rendezvous, Initiator,
Responder, Introduction, and Transit-issuance engines and their command
composition have been deleted after the command refusal became current. Their
old plan stanzas remain only at that typed refusal boundary. The Initiator
Entry-admission adapter is also absent. The old Transit-issuance
signer/listener/root-mutation engine is absent;
the typed command refusal and signed-profile decoder remain. ADR-0092 also
removed the uncomposed Endpoint Transit acquisition client; neither side
provides a selected old-Transit receiving path.
The adjacent production-dead Initiator Entry admission, receiving relay grammar,
and direct OHTTP forwarding adapters have been deleted by their dedicated
closure audit. The later User Route closure audit also removed the uncalled
Open/Attach owner, its private reachability exchange, and its exclusive relay
and Introduction sender orchestration. Shared credential-relay grammar and the
standalone reachability Relay remain with their actual consumers; test-local
reciprocal fixtures do not restore a production receiving path.
The old Node-leg dial and client confirmation entrypoint are also absent.
The v1 `ListenNodeCarrier` and its exclusive helpers are absent. Current Node duties use
`ListenClosedSharedCarrier`; the direct role issuer uses
`ListenClosedRoleCarrier`. Shared byte-lane and TLS/QUIC mechanics remain
with those closed consumers. The v1 State/profile readers and reciprocal
codec are separate compatibility questions and are not retired by this
listener disposition.
No retirement path inherits a duty,
regenerates a key, resets a root or floor, converts state, or adopts foreign
files. Compatibility readers and separately owned retirement surfaces remain
until later bounded changes prove their accepting callers and other consumers
absent.

The Node-plan gate is integrated: all five old reservations return the stable
`old Node duty reservation is retired` outcome immediately after bounded plan
decoding and schema/completeness recognition, before key, certificate, Source
root, State root, listener, resource, or duty construction. A mixed old and
closed plan receives that same refusal and cannot use the closed reservation as
a fallback. The Contributor start gate is also integrated: recognized `apply`
and `restart` command shapes return `old Contributor start is retired` before
platform, bundle, installation, root, supervisor, output, or Network effects.
Contributor pre-Control recovery now authenticates and reconciles only owned
interrupted-update evidence: it may Stop an active predecessor and clean exact
residue, but never Starts, Restarts, or Enables either generation. An inactive
installation remains inactive, while ambiguous or foreign evidence fails and
is retained. The Source gate is also integrated: a recognized
`native_rendezvous_profile` returns `old Source profile is retired` after
bounded Source-plan recognition and before trust-map, root, key, listener or
Network work. Mixing that selector with the closed profile receives the same
retirement outcome and cannot fall back. The explicit closed profile and its
already pinned authority retain their previous State and Source behavior.

## Module ownership

| Module | Interface responsibility | Excluded responsibility |
|---|---|---|
| internal/network/state | Authenticate source input, verify Epoch/View material, publish one immutable current or pending View through its exclusive durable root, and supply narrow read-only views. | Source authority, public wire selection, Node lifecycle, Route selection, or private naming control. |
| internal/network/source | Obtain one finite selected Direct-Origin source input with its credential, TLS transport, material selector, ordering, and exposure identity. | Accepting State or selecting a peer protocol. |
| internal/network/duty | Persist the Endpoint-local Role Domain generation, watermark, expiry, and current conflict Duties. Its version-1 root still decodes and carries the historical receiving one-use Transit Grant spend ledger through `Replace`; the `SpendTransitGrant` operation was retired with the Route v2 execution closure (ADR-0093), and no current receiving-Node admission path exists. The persisted schema needs an explicit old-root migration or refusal decision before its decoder is removed. | Network State publication, assignment creation, Route ownership, issuer custody, or Node process lifecycle. |
| internal/resource | Check selected process placement and measure process/cgroup pressure through a process-local Guard. Separately own the initialized durable shared Hosting period, interface-counter charging and work/termination reservations. | State or Node authority, admission, listener shutdown, forgiving an outstanding reservation on handle close, or a claim for unsupported platforms. |
| internal/entry | Own the protected Endpoint's durable closed Entry sets: select exactly two State-current members per adjacent Role Domain before use, revalidate a selected member, retain the generation floor, and refuse legacy-root substitution. Separately, the retained `entry recipient/import` operator command opens the older Invite root, validates one recipient-bound signed Invite and records its replay/contact history; its Route attachment has no selected C0 caller after ADR-0092. | Complete Route selection, receiving Entry admission, carrier choice, User identity, or treating the old Invite command as a second closed-Route path. |
| internal/route | Implement the closed v3 Node Carrier/wire used by `internal/node` through `OpenClosedNodeCarrier`. The former aggregate Interactive User Route v2 runtime and its whole v2 execution closure (Attachment, EndpointTransitBinding, EntryBinding, credential-relay, Introduction slot/outcome, LegBinding, and Transit Grant verifier files) are absent under ADR-0093. What remains is the byte-exact sealed Introduction v1 grammar pending its superseding ADR-0035 decision (F-42) and the retired v2 `Profile` identity used only for Node typed refusals; neither provides a second supported Route. The [package map](../development/package-map.md) records the current consumer boundary. | Reintroducing the removed User-route composition as a maintained product path or treating its removal as successor-network readiness; candidate ranking, carrier policy/fallback, H3 compatibility, peer runtime, Node profile, or durable State/Duty/credential-journal writing. |
| internal/node | Run one bounded current closed Node duty from authenticated admission through listener readiness, pressure reaction, drain, withdrawal, and a bounded terminal cleanup outcome. All five old native duty engines are absent; their plan stanzas remain only at the command refusal boundary. | State-root authority, assignment creation, an old native duty listener, or a separate probe runtime. |
| internal/contributor | Authenticate and retire an already owned dedicated-host Rendezvous installation. The only dispatchable actions are diagnose, drain, withdraw and confirmed remove; interrupted-update recovery may reconcile the exact current/predecessor generation but cannot Start, Restart or Enable either. | New installation or update, duty selection, Network State authority, public admission, co-residence, arbitrary service control, or capacity claims. |

Each Module exposes one consumer-relevant Interface while retaining codec,
storage, replay, socket, and cleanup details privately. State readers receive
immutable snapshots only after durable publication. A source, clock, or
resource uncertainty prevents fresh State publication rather than creating a
fallback truth.

### State transition admissibility

State alone decides whether a verified Epoch can become current or pending.
Offline acceptance, Source selection, and pending activation apply one durable
current/pending/conflict invariant; the command and Source adapters only supply
verified candidate bytes. A normal exact successor becomes current, a future
successor becomes the one pending Epoch, and that exact pending digest may
become current only in its validity window after a complete Source wave has
retained the pending identity for comparison and rechecked trusted completion
time. A second digest for the pending
Epoch number records a persistent conflict, preserves the current and pending
evidence, and refuses later admission or automatic winner selection. Reopen
recovers the same current/pending/conflict relation before State-dependent work
can proceed.

State also owns the one active Source wave across bootstrap, caller-requested,
and automatic refresh. An automatic tick that arrives while that wave is active
is non-terminal and leaves the existing wave and scheduler live; it neither
publishes a second result nor records a State failure. A completed or rejected
wave still follows the normal availability, clock-confidence, and durable
admission rules, and an actual terminal automatic-refresh failure remains
visible to `Current` and `Wait`.

## Retained generation-2 Route grammar

`ardents-interactive-route-v2` identifies the former native Route grammar, not
an accepting C0 Node duty. The selected closed Route uses `ardents-route-v3`;
old Node selections are refused before Network effects under ADR-0089, and
Node compares the exact stale `Profile` only to refuse it without side
effects (ADR-0093). The v2 EntryBinding and the reciprocal LegBinding
grammars are retired with the whole v2 execution closure (ADR-0093);
LegBinding was wire-only and never persisted. Only SealedIntroduction keeps a
fixed historical record: its bytes and vectors are compatibility evidence
for the ADR-0034 publication keys, not a fallback or version-negotiation
path for the closed Route.
The production-dead Interactive User Route v2 Open/Attach owner and its
EntryBinding, private reachability, relay-setup, sealed-Introduction sender,
credential-relay grammar, Endpoint-transit binding, and volatile composition
paths are absent. This removes no shared listener, closed Source prefix, or
current Node Carrier consumer and selects no successor wire.

The retained Attachment is a small shared authenticated-connection value, not
a complete Route plan or a composition owner. It delegates the existing
`net.Conn` contract, exposes immutable evidence, and publishes one cleanup
result to concurrent closers. No production constructor or accepting startup
for the removed User Route is retained. Endpoint continues to own its durable
credential journal, Entry retains replay and adjacent-contact state, and
Service Connection owns replacement decisions through its own attachment type.

Entry owns every carrier/attachment cleanup lease returned by `Acquire`. Its
owner rejects new acquisition as soon as close begins, cancels and joins an
in-flight opener, closes each active attachment exactly once, and durably
records the terminal cleanup outcome before releasing the exclusive Entry
root. Concurrent caller cleanup and owner close share the same lease result;
a cleanup error is returned and cannot be represented as a clean attachment.

### Adjacent-Node Carrier profiles

`internal/route.Carrier` is the transport-neutral reliable ordered byte lane
used by current Node duties. The maintained closed Node path selects exactly
`ardents-carrier-tcp-tls-v2` or `ardents-carrier-quic-v2`, TLS 1.3,
`ardents-route-v3` ALPN, and the State-pinned Ed25519 peer. It does not
exchange the old reciprocal `LegBinding`, whose codec is retired (ADR-0093).
QUIC uses one bidirectional
stream, an initial packet size of 1200, no 0-RTT or datagrams, and bounded
keepalive inside its idle timeout. Transport sockets, QUIC connection IDs,
migration operations, and cleanup mechanics stay private to the adapters.
The old v1 Carrier identifiers remain historical State and refusal inputs,
not an accepting closed Node Carrier path.

Network State owns the supported choice. Historical signed Node Record v1
canonically means TCP/TLS; v2 contains one signed explicit Carrier Profile.
Unknown profiles are rejected before assignment. No old native duty listener
has a production caller. The deleted Initiator and Responder engines previously
used the selected-candidate rule, and their old `OpenNodeLeg` dialer is absent.
Current `OpenClosedNodeCarrier`, `ListenClosedSharedCarrier`, and
`ListenClosedRoleCarrier` accept one selected closed profile without fallback.
A State successor drains and withdraws the current duty; it does not rewrite
an active attachment.

### Old native listener closure

The closure removed `node_carrier_listener.go`
(`ListenNodeCarrier`, `CarrierListener`, `PendingCarrier`, its private
TCP/QUIC adapters, v1 server TLS and QUIC configuration) and
`node_carrier_listener_test.go`, their exclusive `nodeQUICConfig` from
`node_carrier_quic.go`, and the otherwise uncalled failure-class wrapper in
`node_carrier_failure.go`. The removed symbols had no production caller or
separately retained test. This is a source closure,
not a change to accepted Carrier selection or wire behavior.

The #252 listener deletion retained `Carrier` and the v1 profile constants in
`node_carrier.go`; ADR-0093 has since retired the `CarrierTCP`/`CarrierQUIC`
constants (F-59). The byte-lane Interface remains live in the closed path.
The closed-opener rejection test keeps the exact literal old profile value,
and the v1 profile *values* remain signed-record interpretation and refusal
inputs in Network State. Retain
`quicNodeCarrier` and its deadline and
close behavior for `OpenClosedNodeCarrier`. Retain
`closedNodeQUICConfig`, the closed Node TLS verifier, and both closed
listeners for current Node and direct-role callers. The v1 reciprocal
`LegBinding` codec and its canonical vectors are retired by ADR-0093 as
their compatibility disposition. Current
behavior checks are the TCP/TLS and QUIC cases in
`closed_node_carrier_test.go`, `closed_shared_carrier_test.go`, and
`closed_role_carrier_test.go`, including peer rejection and QUIC handshake
reservation. The old listener test does not substitute for these checks.

One qualification-only operational seam admits a literal loopback or private
IPv4 listen address for a State-selected closed Route duty. A host-owned,
byte-transparent Carrier relay binds the public State endpoint and forwards to
that private address. One forwarding duty may likewise dial a declared relay
address while authenticating the exact peer, key, Carrier and duty selected by
State. These adapters change socket placement only; they cannot select a Route
peer or create a second network identity. With no adapter, the Node binds and
dials the State endpoints exactly as before.

## TLS material boundaries

Native Route, Entry, and Node TLS use local certificates for their bounded
attempts and selected peer keys from authenticated State and binding evidence.
Route creates its self-signed client certificate for one attachment (currently
16 minutes); its public-key digest is bound into the Entry or Transit record.
Node duties receive their local certificate in the bounded plan. The effective
attempt deadline comes from the authenticated Entry, Transit, or selected duty
fact. Their trust does not depend on an external CA issuance service: the peer
verifies TLS 1.3, the selected Route ALPN, the State-pinned Ed25519 key, and
the reciprocal binding.

Direct-Origin Source has a distinct X.509 transport boundary. Its client
accepts only the configured CA, hostname, and server leaf-key pin. Its server
requires a CA-verified client certificate and an authorized client leaf-key
pin. `ardents` and `ardents-node` read the declared PEM key pairs and roots
while constructing the bounded Source configuration; `internal/network/source`
then owns copies for its one configured TLS client or listener. Replacing a PEM
file does not alter a running Source process: there is no hot reload or
Source-side certificate issuer in the maintained surface. A changed certificate
therefore needs a separately checked new configuration and lifecycle action;
the X.509 `NotBefore`/`NotAfter` limits are checked during a new handshake.
The current contract does not promise seamless rotation or that an already
established TLS connection is immediately interrupted when a certificate
expires.

## Node and Resource lifecycle

Node consumes narrow authenticated State and Duty facts, then moves a local
role through admission, readiness, pressure protection or drain, withdrawal,
and terminal cleanup. The former standalone probe package is intentionally
private Node implementation; the command does not compose an independent
probe runtime.

The accepted duty projection captured at listener start identifies only the
duty for which that listener was created.
Each new closed Route admission must re-read the current authenticated duty
facts and require the exact same generation, Network, Epoch, digest, Node,
assignment, and assignment digest to remain fresh and unconflicted. The State
owner must join the profile's numeric Role Domain to that Epoch assignment
before providing a usable duty view. An
accepted successor, expiry, conflict, or withdrawal therefore closes the old
admission authority without a polling grace period.

Terminal success is conditional on known cleanup. Role drain, listener or HTTP
shutdown, and owned issuer/Entry root close errors propagate to the lifecycle
result. Node may emit `DRAINING` while cleanup is attempted, but it must move to
`FAILED` and must not publish `WITHDRAWN` if any required cleanup fails or its
result is unavailable.

Resource measurement is Linux-only until another native Adapter is selected
and measured. Unsupported platforms refuse rather than silently reporting
capacity. Resource has no authority over a consumer's lifecycle: Endpoint,
Node, and Route own their respective readiness, admission, drain, and shutdown
reaction.
When a READY Node cannot obtain required pressure evidence, it fails closed.
Its lifecycle event uses the fixed reason `resource pressure sampling timed out`
for a measurement deadline and the existing `resource pressure evidence is
unavailable` reason for other errors. The event never copies raw measurement,
filesystem, cgroup, or provider error text; the `Run` error retains the
underlying cause for local diagnosis.

## Current limits and limitations

The implemented system is a project-controlled Closed Test Network. A local
test or development-host process does not establish independently operated
capacity, anonymity, availability, censorship resistance, public deployment,
or a supported platform profile. Private source and Route bytes do not create
a public protocol promise.

The current direct-origin source and native Route code are selected technical
tracers. Any new source transport, peer announcement, public bootstrap,
directory, carrier fallback, or supported Node operating profile requires its
own decision, compatibility rule, and Qualification evidence.

The retained native resource profile identity is
`ardents-rendezvous-dedicated-host-v1`. Its 1-CPU, 192/256-MiB, 128-MiB Go,
64-task, and 256-FD bounds remain evidence for an already owned dedicated-host
Contributor installation; they do not authorize a new Rendezvous Node start.
The Node command refuses an old reservation before resource-profile validation
and no longer normalizes the historical `h4-5-rendezvous-alpha-v1` identity
into a runnable plan. Contributor ownership readers may recognize either
identity only to authenticate pinned evidence and perform the separately
bounded retirement operations. Retained engine tests and `h3-*` guard profiles
are compatibility evidence pending their own deletion slices, not accepting
command routes.

The Linux Contributor command exposes only diagnose, drain, withdraw and
confirmed removal for an already owned installation. `apply` and explicit
`restart` refuse before opening that installation or creating a supervisor.
The Contributor Module has no Apply installation/update operation. It retains
authenticated installation/update-record readers and no-start interrupted
update recovery for the four dispatchable retirement actions.
The operator contract is the
[Rendezvous Contributor runbook](../reference/rendezvous-contributor.md).
Under the selected retirement transition, pre-Control update recovery
authenticates and reconciles only the current or predecessor generation. It
may inspect, Stop, Disable, and remove owned state but does not Start, Restart,
Enable, or finish an update by executing either generation. Ambiguous or
foreign evidence fails without adoption or cleanup.

## Verification and decisions

- Focused Network State, Duty, Resource, Entry, Route, and Node behavior tests
  cover durable reopen, corruption, replay, invitation replacement, successor-
  State admission rejection, active attachment cancellation and exactly-once
  cleanup, pressure, listener drain, cleanup fault propagation, and withdrawal.
  Forwarding startup tests fail each initialization step after opening the
  spend root, require its exact lease to be released once, and retain the
  combined initialization and cleanup causes before any server exists.
  The forwarding shutdown regression joins a producer that completes a late
  successful outer handshake before waiting on its delayed session reader; the
  spend root remains held through both joins and repeated Drain retains the
  physical close result. Linux race checks additionally exercise State-successor
  drain, cancellation racing a late outer ACCEPT, and late Carrier invalidation
  against a replacement incarnation.
- The maintained Carrier cells cover exact TCP/TLS and QUIC peer/binding
  authentication, pending-admission reservation before QUIC authentication,
  signed v1/v2 State projection and unknown-profile rejection, both directions
  of no-fallback behavior, and the same authenticated native Route attachment
  journey over each profile. The restricted local Docker campaign repeats
  those cells from cross-built Linux bytes at 1 vCPU/1 GiB with no external
  network. Its recurring QUIC UDP-buffer warning forbids a throughput or
  capacity conclusion.
- Current process tests cover authenticated Source-to-State and closed Node
  lifecycles. The superseded positive old-role command and multi-host
  qualification procedures are absent; their historical receipts do not make
  an old duty runnable. All five old native duty engines and their direct
  behavior tests are absent. Transit credential tests now cover only its
  retained signed-profile/client grammar and the separately owned Endpoint
  acquisition path.
- A [historical mixed-host run](https://github.com/dianabuilds/ardents-network/blob/f82a52dde912e975df0b23bdcf459f1e5b71def3/docs/technical/network-route-node.md#verification-and-decisions)
  retains bounded functional integration evidence for its exact candidate.
  It supplies no current Route, privacy, host-profile, or public-operation
  qualification.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md),
	[ADR-0070](../adr/0070-own-volatile-user-route-orchestration.md),
  [ADR-0025](../adr/0025-state-referenced-entry-invites.md),
  [ADR-0072](../adr/0072-adopt-offline-enrollment-route-v2.md), which
  supersedes their C0 Route/Entry selection,
  [ADR-0048](../adr/0048-maintain-tcp-and-quic-carriers.md), and
  [ADR-0049](../adr/0049-defer-blocked-entry-profile.md) define the selected
  native Route and Carrier facts.
- [R-092](../research/records/r-092-native-node-operating-profile.md) retains the
  measured dedicated-host Rendezvous Functional Alpha result for its original
  recorded candidate. That historical result does not qualify the current C0
  candidate, select another duty, or establish public capacity, availability,
  co-resident, permissionless, or independent-operation claims.

## Closed Introduction registration receiver

The `closed_introduction` reservation binds one current Introduction delivery
duty to its State-selected shared TCP/TLS or QUIC listener and separate durable
admission-spend root. A State-authorized Node Carrier provides only bounded
inner TLS allocation. Each registration still needs its own class-3 token,
verified against the exact inner HELLO and TLS exporter before the receiving
lease can extend the child's deadline.

REGISTER and WITHDRAW use the selected 4,096-byte operation grammar and
16,384-byte results. A slot belongs to the exact admitted terminal channel;
duplicates refuse, withdrawal must match its slot/revision and use a fresh
request nonce, and channel loss invalidates the live registration. Before a
successful registration result, the duty durably retains the slot's SHA-256
hash and original expiry under the same exclusive admission-root lease. The
bounded snapshot contains at most 1,024 entries, is bound to the exact Network,
profile, Node and duty generation, and uses file synchronization, replacement
and directory synchronization. A fresh token after restart cannot reclaim a
slot before that expiry; expiry permits its floor to be pruned. The snapshot
also retains the pruning time: a clock below that floor refuses new claims,
including after restart. No live channel
is restored. Missing slot storage after prior admission, malformed snapshots
and ambiguous writes refuse registration. The spend root remains held until
all Carrier readers and handlers join; released owners cannot reopen or write
its ledgers. The shared QUIC listener owns and joins its transport and UDP
socket explicitly, so completed shutdown releases the selected address.

A separate class-1 submission is admitted for the same Introduction duty.
Its receiver forwards only the sealed capsule over the already owned class-3
registration, using a fresh channel-local request nonce and ordered even child
IDs. It reserves at most 16 pending deliveries, including writer waiters, and
limits both admission and actual dispatch to four per second. The bounded
8 MiB registration allowance includes delivery OPERATION, RESULT, CLOSE and
reserved withdrawal; another submission token cannot enlarge it. Publisher
acknowledgement is bounded by the capsule and original registration expiry.
Failure that cannot finish a child retires that registration without reclaiming
its slot. Publisher refresh and complete command publication readiness remain
unconnected. These component checks do not qualify the complete journey.
## Closed data JOIN receiver

The exclusive `closed_data_join` reservation binds the State-selected Rendezvous
DataJoin duty to one receiving-spend root, certificate, finite connection limit
and drain timeout. The shared TCP/TLS or QUIC listener accepts current Node
Carriers and fresh inner role TLS; HELLO and genuine class-2 admission precede
exactly one lane-1 JOIN. Route rechecks the original TLS exporter before matching.

The Route pairing owner retains both original admissions, matches the approved
secret/context/profile and opposite side values, and emits each local RESULT
only after pairing. Both result writes precede framed data forwarding. Directional
credit is bounded to 64 KiB, EOF preserves the reverse direction, and CLOSE
joins both readers/writers before admission and queue release. A completed JOIN
retains its successful outer terminal status through TLS closure. Fixed request,
result, headers, data and control traffic consume the original class-2 budget;
parsing and stream buffers are reserved from the receiving-duty aggregate first.
Listener drain joins handlers and timer callbacks before releasing the spend root.

This receiver does not construct Source/Responder prefixes or authenticate the
end-to-end Service session. Those remain Endpoint and Route client obligations.
