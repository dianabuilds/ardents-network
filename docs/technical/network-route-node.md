# Network State, Entry, Route, and Node

Status: **current maintained technical contract.** This document describes the
implemented closed-test-network Modules and their current Interfaces. It does
not claim public network operation, independent operators, public discovery,
supported Node hosting, or Route qualification.

The [selected successor architecture](common-privacy-architecture.md) under
[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md) has a separate
target contract. It does not change the maintained native protocol below.

The selected generation-3 successor is defined by [ADR-0081](../adr/0081-select-closed-protected-service-contract.md)
and the [protected forwarding contract](protected-route-protocol.md). The
implemented behavior described here remains the migration input until its
owning change promotes the replacement; it is not a second accepting privacy path.

The Node command connects an exclusive `closed_forwarding` local reservation
to the implemented generation-3 forwarding receiver. The reservation contains
only the existing receiving-spend root and finite connection/drain limits.
It selects the closed State profile only with its already State-pinned signer;
State's accepted profile and duty projection continue to select the listener,
Carrier, adjacent/interior assignment and next peers. A legacy duty, issuer
reservation or unrelated profile signer cannot be combined with that plan.
This command binding does not implement the remaining Endpoint composition or
establish whole-route qualification.
For a generation-3 TCP Node Carrier, terminal retirement closes the owned
physical socket once and retains its actual close result. It interrupts the
multiplexed transport instead of initiating another TLS notification after a
peer reset. Lane owners still join readers, writers and children before
releasing their roots. This terminal abort is distinct from directional ARDP
EOF and authenticated Service completion; direct role and inner TLS closure
keep their existing semantics.

The shared successor listener gives each arriving connection its own bounded
handshake/first-stream interval. Waiting without a peer does not consume that
interval or make the next valid peer inherit an expired deadline. Issuer, forwarding and resolution consumers use this interface; current State classification and
subsequent HELLO/admission deadlines remain separate checks.

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
## Module ownership

| Module | Interface responsibility | Excluded responsibility |
|---|---|---|
| internal/network/state | Authenticate source input, verify Epoch/View material, publish one immutable current or pending View through its exclusive durable root, and supply narrow read-only views. | Source authority, public wire selection, Node lifecycle, Route selection, or private naming control. |
| internal/network/source | Obtain one finite selected Direct-Origin source input with its credential, TLS transport, material selector, ordering, and exposure identity. | Accepting State or selecting a peer protocol. |
| internal/network/duty | Persist the Endpoint-local Role Domain generation, watermark, expiry, conflict truth, and receiving-Node one-use Transit Grant spend ledger. | Network State publication, assignment creation, Route ownership, issuer custody, or Node process lifecycle. |
| internal/resource | Resolve the current process's own cgroup-v2 directory, measure selected Linux process limits, and make the finite NORMAL, PROTECT, or DRAIN pressure decision. | Admission, listener shutdown, or a claim for unsupported platforms. |
| internal/entry | Import and admit a signed State-referenced Entry Invite, maintain its bounded durable replay/replacement set, and open an adjacent contact lifecycle. Local import verifies the signed recipient against its retained Entry-root recipient identity before consuming a slot or replacing a predecessor; receiving admission independently verifies it against the presenting TLS key. | Complete Route selection, carrier choice, or User identity. |
| internal/route | Compose and hold one native Interactive User-route attachment from authenticated State, Entry, caller-owned resource facts, private reachability, Descriptor-selected peers, and the exact selected TCP/TLS or QUIC-v1 carrier. | Candidate ranking, carrier policy/fallback, H3 compatibility, peer runtime, Node profile, or durable State/Duty/credential-journal writing. |
| internal/node | Run one bounded Contributor duty from authenticated admission through listener readiness, pressure reaction, drain, withdrawal, and a bounded terminal cleanup outcome. | State-root authority, assignment creation, or a separate probe runtime. |
| internal/contributor | Own the one pinned-bundle, fixed-path systemd lifecycle for the dedicated Rendezvous installation. | Duty selection, Network State authority, public admission, co-residence, arbitrary service control, or capacity claims. |

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

## Native Route profile

The selected Route profile is ardents-interactive-route-v2. EntryBinding binds
one signed v2 Invite to a fresh User-to-Initiator TLS attempt key. Node-to-Node
LegBinding and SealedIntroduction have fixed binary records; State/publication
select supported generations, not a Node or peer value. The profile has no H3
reader, direct fallback, generic record map, or version-negotiation path.

Route owns volatile User-route composition and cleanup. It reserves caller
capacity before reading State, then obtains only State's exact Gateway,
Initiator, and issuer facts; carries private reachability through Entry;
verifies the Descriptor against the authenticated Target; and uses its exact
Introduction/Rendezvous slot to return an opaque Attachment and immutable
evidence to Service Connection. Endpoint owns Service-Link/capability binding
and its durable credential journal through a narrow callback; it cannot choose
a Route carrier or peer. Service Connection, not Route, decides whether an
attachment must be replaced. Entry may retain replay and adjacent-contact state
but cannot construct a complete Route from that state. Caller and Route
shutdown may race to close one active Attachment; all closers join the same
terminal cleanup, receive the same result, and cannot reuse the carrier after
close begins.

Entry owns every carrier/attachment cleanup lease returned by `Acquire`. Its
owner rejects new acquisition as soon as close begins, cancels and joins an
in-flight opener, closes each active attachment exactly once, and durably
records the terminal cleanup outcome before releasing the exclusive Entry
root. Concurrent caller cleanup and owner close share the same lease result;
a cleanup error is returned and cannot be represented as a clean attachment.

### Adjacent-Node Carrier profiles

`internal/route.Carrier` is the transport-neutral reliable ordered byte lane
used by native Node duties. The release maintains exactly
`ardents-carrier-tcp-tls-v1` and `ardents-carrier-quic-v1`. Both require TLS
1.3, the native Route ALPN, the State-pinned Ed25519 peer, and reciprocal
`LegBinding`. QUIC uses one bidirectional stream, an initial packet size of
1200, no 0-RTT, no datagrams, and bounded keepalive inside its idle timeout.
Failed post-open authentication aborts rather than masquerading as a graceful
close. Carrier errors have stable transport-neutral failure classes. Transport
sockets, QUIC connection IDs,
migration operations, and cleanup mechanics remain private to each Adapter.

Network State owns the supported choice. Signed Node Record v1 canonically
means TCP/TLS; v2 contains one signed explicit Carrier Profile. Unknown
profiles are rejected before assignment. Rendezvous listens with its own
record's profile; Initiator and Responder use the selected Rendezvous
candidate's profile. `OpenNodeLeg` and `ListenNodeCarrier` accept exactly one
profile and never race or fall back. A State successor drains and withdraws the
old duty; it does not rewrite an active attachment.

One optional Rendezvous-only operational seam admits a literal loopback listen
address on the same numeric port as that signed candidate. It exists so a
host-owned, byte-transparent Carrier relay can bind the State-advertised
address while the exact product Rendezvous binds loopback behind it. The seam
cannot change any advertised candidate, Node identity, State digest or Epoch,
or Carrier profile; hostname, unspecified, public, and port-divergent overrides
fail before listener startup. With no override, Rendezvous binds the State
endpoint exactly as before.

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
Each new Entry or Transit Grant admission re-reads the current authenticated
duty facts and requires the exact same generation, Network, Epoch, digest,
Node, assignment, and assignment digest to remain fresh and unconflicted. An
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

The sole accepted native resource profile is
`ardents-rendezvous-dedicated-host-v1`. It is accepted only for an exact Rendezvous-only
Node plan and rejected for Initiator, Introduction, Responder, mixed duties,
or arbitrary native configurations. Its 1-CPU, 192/256-MiB, 128-MiB Go,
64-task, and 256-FD placement is qualified only for the project-operated
dedicated-host Functional Alpha. The retained `h3-*` guard profiles may still
support their retired-role tests, but native Route code cannot inherit them.
Readers accept the historical `h4-5-rendezvous-alpha-v1` identity only to
reopen already pinned bundles, Node plans, and installation records; runtime
state, new records, and reports normalize to the canonical product identity.

The Linux Contributor command writes only the bounded last lifecycle and resource
events into its private diagnostic directory. Its Contributor Module verifies
an independently pinned closed bundle before parsing, owns fixed host paths and
one hardened systemd unit, requires exact generation successors, rolls back a
failed or interrupted update, and exposes only diagnose/restart/drain/withdraw
and confirmed removal. The operator contract is the
[Rendezvous Contributor runbook](../reference/rendezvous-contributor.md).

## Verification and decisions

- Focused Network State, Duty, Resource, Entry, Route, and Node behavior tests
  cover durable reopen, corruption, replay, invitation replacement, successor-
  State admission rejection, active attachment cancellation and exactly-once
  cleanup, pressure, listener drain, cleanup fault propagation, and withdrawal.
- The maintained Carrier cells cover exact TCP/TLS and QUIC peer/binding
  authentication, pending-admission reservation before QUIC authentication,
  signed v1/v2 State projection and unknown-profile rejection, both directions
  of no-fallback behavior, and the same authenticated native Route attachment
  journey over each profile. The restricted local Docker campaign repeats
  those cells from cross-built Linux bytes at 1 vCPU/1 GiB with no external
  network. Its recurring QUIC UDP-buffer warning forbids a throughput or
  capacity conclusion.
- Process tests cover authenticated source-to-State and Node lifecycles; the
  selected multi-host cells additionally put the exact product Rendezvous behind a
  test-owned raw TCP Carrier relay, retain both PIDs/lifecycles, and inject
  Carrier-reset and exact product-Node-kill faults without a fixture
  Rendezvous or transit fallback. They remain bounded functional evidence, not
  a public network or native host profile.
- Product-command tests now start separate Initiator, Introduction, Rendezvous,
  and Responder processes from one signed native Route Epoch, verify their
  exact State assignments, and carry one local Service Connection journey through those
  commands. The Linux Docker route test uses `SIGTERM` and requires
  `DRAINING` then `WITHDRAWN` after the completed journey; a linked signed
  State successor also withdraws all four commands. Its product-transit
  offline case produces `service unavailable` without opening an Application
  Connection, and a Linux Rendezvous process test drains a held authenticated
  pair on `SIGTERM`.
  Its neighbouring Route roles remain fixtures. This does not prove a full Route
  active-work drain, multi-host operation, or a host profile. The Windows
  compatibility harness retains forced cleanup.
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
limits both admission and actual dispatch to four per second. The original
1 MiB registration allowance includes delivery OPERATION, RESULT, CLOSE and
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