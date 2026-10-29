# Network State, Entry, Route, and Node

## Selected closed-release Carrier

[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS alone
for the nearest closed release, superseding dual-Carrier admission and positive
qualification clauses below. Known QUIC identities retain canonical meaning but
grant no eligible selection, local duty or transport execution; refuse before
effects without fallback or signed-State rewriting. Historical both-Carrier
receipts remain source-specific evidence. All role/authority, capacity, deadline,
durable floor, process and joined-cleanup requirements remain binding on TCP/TLS.

Status: **current maintained technical contract.** This document describes the
implemented closed-test-network Modules and their current Interfaces. It does
not claim public network operation, independent operators, public discovery,
supported Node hosting, or Route qualification.

Under the [bounded qualification operation](../operations/qualification-release.md)
and [ADR-0120](../adr/0120-authorize-operator-prepared-qualification-release.md),
`ardents-control prepare-qualification-node-record` prepares only existing
unsigned generation-one ARNR-v2 bytes for explicit closed Node facts.
`prepare-qualification-epoch` checks supplied signed closed Records with the
ordinary Epoch owner, computes actual commitments/assignments and prepares
only existing unsigned initial AREP-v3 bytes. State authorities remain distinct
from Node identity keys; any rejected or colliding initial Record refuses
preparation. Neither command signs, grants a duty, accepts State, modifies a
floor or starts a Node. Existing State signature, closed-profile and Source
verification contracts remain binding.

The isolated new public composition exposes the corresponding canonical owners
through `ardents-next network prepare-initial-node-record`,
`prepare-initial-epoch` and `prepare-closed-profile`, under the
[new Network boundary](successor-network-state.md) and
[command reference](../reference/commands.md#public-network-preparation-in-the-isolated-command).
It prepares only public unsigned bytes with fresh native output custody and
TCP/TLS-only admission. It imports no predecessor command/runtime and implements
no private signing or installed operator provisioning journey.

The [selected architecture](common-privacy-architecture.md) under
[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md) and
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) uses the
[protected forwarding contract](protected-route-protocol.md) for the current
closed v3 path. This document describes the implemented closed duties. The retained
generation-2 grammar it once cited is deleted: ADR-0109 (F-32) refuses a
stored legacy record with a typed error, so no second accepting privacy path
remains.

The Node command connects an exclusive `closed_forwarding` local reservation
to the implemented generation-3 forwarding receiver. The reservation contains
only the existing receiving-spend root and finite connection/drain limits.
It selects the closed State profile only with its already State-pinned signer;
State's accepted profile and duty projection continue to select the listener,
Carrier, adjacent/interior assignment and next peers. A legacy duty, issuer
reservation or unrelated profile signer cannot be combined with that plan.
This command binding does not implement the remaining Endpoint composition or
establish whole-route qualification.
The forwarding role constructs the spend ledger, duty limits and bootstrap
controller as one private receiving group before it opens the outgoing pool
and listener. Until that group is complete, its builder owns rollback and
closes the exact spend-root lease once; an initialization failure retains both
its initial cause and any cleanup cause. Node validates the local profile and
address, then opens and transfers a shared Hosting handle. The role owns that
handle's late close after its accepted producers and Carrier readers join.
The forwarding Start boundary checks both current Route and current Profile
State callbacks, plus its admission, clock and
endpoint callbacks before opening receiving resources. If those dependencies
are incomplete, it closes the transferred Hosting handle and retains its
close result. Issuer, Resolution, Introduction and JOIN likewise check their
borrowed callbacks before opening their own roots or listeners.
The Node Hosting adapter supplies class-2 reservation policy without giving
the role process pressure or the provider-period ledger's global ownership.
For a generation-3 TCP Node Carrier, terminal retirement closes the owned
physical socket once and retains its actual close result. It interrupts the
multiplexed transport instead of initiating another TLS notification after a
peer reset. Lane owners still join readers, writers and children before
releasing their roots. This terminal abort is distinct from directional ARDP
EOF and authenticated Service completion; direct role and inner TLS closure
keep their existing semantics.
Terminal forwarding cleanup expires the read deadline before closing an
accepted Carrier or direct-role connection. The close operation owns write
interruption; concurrent cancellation must not expire the write deadline while
TLS sends its closure alert. Actual close failures remain retained for drain.
On State loss or an accepted successor, the Node lifecycle stops the old
forwarding duty, closes its listener and outgoing pool, and waits for every
accepted handler before the outgoing-session owner performs the final wait for
its retained Carrier readers and returns their joined cleanup result. The server
retains the pool interruption and spend-root lifetime, so a timed-out Drain
cannot release the root or turn a later physical close failure into success.
Accepted Forwarding Carrier close failures, including capacity refusals, join
the final `Drain` result after all handlers finish. A repeated close reporting
`net.ErrClosed` is benign; `Done` remains the accept-loop result rather than
the joined cleanup result.
While one child is pending downstream HELLO/ACCEPT, the parent reader still
serves lane-zero control and independently selected children. A pending child's
frames remain in Route's bounded accounted queues; CLOSE cancels and joins only
that child opener before its reservation is released.
Outgoing Source child retirement removes its queued payload, then joins an
already active DATA or CREDIT frame before sending CLOSE. That frame retains
its original write deadline, additionally bounded by the same one-second total
cleanup window. No queued payload or new work is admitted by retirement. A
stalled or failed active frame retires the physical parent, and its error remains
in the child cleanup result even if CLOSE never reached the writer.
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
Resolution retains non-benign close failures from accepted Carriers, including
direct refusals and capacity refusals, in the final joined drain result;
`net.ErrClosed` from an already closed Carrier is benign.
Introduction follows the same accepted-Carrier close accounting through its
joined drain result, including admitted children and both refusal paths.
The Issuer's Credential listener also retains accepted-Carrier close failures
after joining its children. Node releases the issuer and admission roots when
that join completed, even if the listener reports a physical close failure;
an incomplete join keeps both roots held.
Resolution, Introduction, and Issuer control admissions retain a shared
Hosting reservation until their admitted child has completed. A successful
protocol reply does not turn a later reservation-release failure into a failed
reply; that failure is instead part of the joined duty cleanup result. If a
bounded drain expires before the child joins, Node returns the failed cleanup
outcome and transfers shared Hosting-handle closure to that eventual join, so a
late release cannot run against a closed handle.
Resolution and Introduction transfer each successful admission to the complete
inner child owner, including later operation failures. That owner keeps the
reservation through TLS close-notify, the Outer terminal write and any required
failed-writer physical join before releasing it exactly once. A submission
retains its own class-1 reservation independently of the owning registration's
class-3 reservation. The registration first retires its slot, then joins its
channel termination; its retained slot floor is not refunded by withdrawal.
Each Carrier joins its physical outcome with its first complete child cleanup
failure. The duty retains the first non-benign complete Carrier result, bounding
error retention across successive connections; durable-root close failures are
joined separately during drain. Independent causes inside that retained result
survive even when joined or wrapped with an already-closed sentinel. Writing to an already retired Outer lane reports
`net.ErrClosed`; an in-flight physical writer failure keeps its original cause.
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

[ADR-0089](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0089-retire-old-node-starts-preserve-owned-shutdown.md)
selects an effect-free refusal for every new old execution selection:

- Node reservations `rendezvous`, `initiator`, `introduction`, `responder`,
  and `transit_issuer`, including their old runtime assignments;
- Source `native_rendezvous_profile`;
- `ardents-transit-issuer-initialize-v1` and issuer serve selected by the old
  Transit issuer reservation.

The old dedicated-host Contributor `apply` and `restart` selectors were
subsequently removed entirely together with the whole retirement mechanism
(ADR-0114); an unrecognized `contributor` command shape fails with the
standard usage error before any effect.

Each adapter must identify and refuse its old selection before opening or
creating a state root or key, binding a listener, invoking a supervisor, or
starting Network work. Refusal cannot select another profile or fall back to a
closed duty. The plan and command schemas are not retired wholesale: probe
behavior is outside this decision, and `closed_issuer`, `closed_forwarding`,
`closed_resolution`, `closed_introduction`, `closed_data_join`, the explicit
closed Source profile, and closed issuer initialize/serve retain their exact
existing authority checks.

Existing old roots, keys, floors, plans, and installation records remain
unchanged evidence. No maintained reader authenticates an old installation
record: the owned-installation retirement surface was removed together with
its mechanism (ADR-0114). Historical profile recognition cannot authorize
execution of an old duty or rewrite persisted identity. The old Rendezvous, Initiator,
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
The v1 `ListenNodeCarrier` and its exclusive helpers are absent. Current Node
duties, including the issuer, use `ListenClosedSharedCarrier`. The issuer
passes that listener to Credential's admitted bootstrap server; its direct
`ListenClosedRoleCarrier` branch has no current production Node caller. Shared
byte-lane and TLS/QUIC mechanics remain with the closed consumers. The v1
State/profile readers and reciprocal codec are separate compatibility
questions and are not retired by this listener disposition.
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
a fallback. The Contributor start gate was superseded by complete removal:
ADR-0114 deleted the subcommand, its module, and its runbook because no live
installation remains, so every `contributor` command shape now fails with the
standard usage error before any platform, bundle, installation, root,
supervisor, output, or Network effect. The Source gate is also integrated: a recognized
`native_rendezvous_profile` returns `old Source profile is retired` after
bounded Source-plan recognition and before trust-map, root, key, listener or
Network work. Mixing that selector with the closed profile receives the same
retirement outcome and cannot fall back. The explicit closed profile and its
already pinned authority retain their previous State and Source behavior.

## Module ownership

| Module | Interface responsibility | Excluded responsibility |
|---|---|---|
| internal/network/state | Orchestrate authenticated Source and offline intake, decide current/pending/conflict publication through the durable root, and supply narrow read-only views from a verified Epoch Decision. | Epoch grammar and signature mechanics, Source authority, physical root mechanics, public wire selection, Node lifecycle, Route selection, or private naming control. |
| internal/network/epoch | Verify bounded Epoch/Record/View grammar, authority signatures, commitments, assignments and materializations under explicit State policy; return owned canonical bytes and authenticated candidate facts. | Source selection, State admission/publication, durable roots, clock-confidence policy, or consumer views. |
| internal/network/closedprofile | Verify only canonical signed ARDCPR03 bytes against an explicit caller-supplied State context; prepare/sign the same grammar for the bounded control command and validate its exact RSA-PSS token SPKI. | Epoch candidate/duty joining, State durable acceptance/conflict, currentness, consumer views, or a replacement authority. |
| internal/network/state/durable | Hold the exclusive State-root lease and preserve opaque generations, the distribution journal, and closed-profile bytes with bounded, synced physical transactions. | Epoch authority verification, Source selection, conflict decisions, or runtime View publication. |
| internal/network/source | Obtain one finite selected Direct-Origin source input with its credential, TLS transport, network-scoped request digest and response-bundle framing, material selector, ordering, and exposure identity. | Verifying Epoch authority, accepting State, or selecting a peer protocol. |
| internal/network/duty | Persist the Endpoint-local Role Domain generation, watermark, expiry, and current conflict Duties. Its root schema is version 2. The `SpendTransitGrant` operation was retired with the Route v2 execution closure (ADR-0093), and ADR-0107 then retired the persisted spend ledger itself (F-53): a strictly validated version-1 generation converts in place at load, dropping its spend records while preserving conflict duties, generation continuity, and the watermark; every committed generation is version 2. No current receiving-Node admission path exists. | Network State publication, assignment creation, Route ownership, issuer custody, or Node process lifecycle. |
| internal/resource | Check selected process placement and measure process/cgroup pressure through a process-local Guard. Hosting accounting belongs to `internal/hosting`. | State or Node authority, admission, listener shutdown, forgiving an outstanding reservation on handle close, or a claim for unsupported platforms. |
| internal/entry | Own the protected Endpoint's durable closed Entry sets: select exactly two State-current members per adjacent Role Domain before use, revalidate a selected member, retain the generation floor, and refuse legacy-root substitution. The Invite subsystem — the `entry recipient/import` operator commands and their older Invite root — is retired by ADR-0106; existing Invite roots stay on disk byte-for-byte with no reader, converter, or deleter. | Complete Route selection, receiving Entry admission, carrier choice, User identity, or any read, conversion, or deletion of a legacy Invite root. |
| internal/route | Implement the closed v3 Node Carrier/wire used by `internal/node` through `OpenClosedNodeCarrier`. The former aggregate Interactive User Route v2 runtime and its whole v2 execution closure (Attachment, EndpointTransitBinding, EntryBinding, credential-relay, Introduction slot/outcome, LegBinding, and Transit Grant verifier files) are absent under ADR-0093. What remains is only the retired v2 `Profile` identity used for Node typed refusals; the sealed Introduction v1 grammar and the publication v1 Introduction instruction codecs are retired by ADR-0094, which supersedes ADR-0035. Nothing provides a second supported Route. The [package map](../development/package-map.md) records the current consumer boundary. | Reintroducing the removed User-route composition as a maintained product path or treating its removal as successor-network readiness; candidate ranking, carrier policy/fallback, H3 compatibility, peer runtime, Node profile, or durable State/Duty/credential-journal writing. |
| internal/node | Run one bounded current closed Node duty from authenticated admission through listener readiness, pressure reaction, drain, withdrawal, and a bounded terminal cleanup outcome. All five old native duty engines are absent; their plan stanzas remain only at the command refusal boundary. | State-root authority, assignment creation, an old native duty listener, or a separate probe runtime. |
| internal/node/authority | Borrow current authenticated closed State views and project one exact receiver or shared peer; verify selected-profile role tokens for the Node duties. The caller retains its duty admission and host/spend reservation. | State-root custody, role selection, process pressure, a receiving spend ledger, or an issuer key. |
| internal/node/forwarding | Own the closed forwarding listener, receiving spend root, pool, sessions, child links and joined drain; check exact State-selected next hops and bootstrap adjacency before dial. | State-root custody, process pressure, unselected fallback, Route wire or Carrier TLS implementation. |
| internal/node/hosting | Adapt the Hosting-domain handle and retain late close after child join; supply class-1/2/3 role envelopes and compose Admission with capacity for forwarding and JOIN. | Durable provider-period ledger custody, process pressure decisions or receiving-role lifecycle. |
| internal/node/issuer | Own the closed issuer listener and both durable roots; forward terminal cause to Node supervision and join every accepted child before late root close. | Process admission and pressure, current State custody, or Credential token grammar. |
| internal/node/introduction | Own the closed Introduction listener, registration slots, capsule deliveries, spend ledger and joined drain. | Process admission, State custody, Hosting pressure, or Route capsule grammar. |
| internal/node/join | Own the closed data JOIN listener, pair set, spend ledger and leased Hosting handle through joined shutdown. Node opens the handle and supplies current State; `node/hosting` supplies its class-2 reservation policy. | Process admission, State custody, global Hosting pressure, or Route pair grammar. |
| internal/node/outer | Serve one accepted authenticated outer Carrier, serialize inner-lane writes, interrupt and join inner handlers on cancellation. The receiving Node duty retains admission, durable roots and the accepted connection's final close result. | Node duty selection, token admission, physical Carrier authentication, or durable-root close. |
| internal/node/probe | Own the private role-probe TLS listener, validated credential copy, fixed request/response, bounded nonce replay memory and joined shutdown; expose a handle for Node supervision. | Authenticated duty selection, State-root custody, process pressure or a separate probe runtime. |
| internal/node/resolution | Own the closed resolution listener, accepted children, Descriptor store and spend ledger; retain terminal and cleanup outcomes after joined drain. | State-root custody, process admission, other Node duties, or Route wire grammar. |

Each Module exposes one consumer-relevant Interface while retaining codec,
storage, replay, socket, and cleanup details privately. State readers receive
immutable snapshots only after durable publication. A source, clock, or
resource uncertainty prevents fresh State publication rather than creating a
fallback truth.

### State execution paths

| Entry | State-owned sequence | Adjacent owner |
| --- | --- | --- |
| [`Open`](../../internal/network/state/open.go) | Validate the one configuration and clock, claim the durable root, verify the retained current chain and distribution journal (an active current without that journal requires explicit recovery), recover pending or interrupted publication, then start the optional Source server, automatic refresh, and resource governor. | `epoch` authenticates restored bytes; `durable` leases and reads opaque generations; `source` validates its TLS plan. |
| [`Accept`](../../internal/network/state/offline_accept.go) | Verify one offline genesis or exact successor and its materialization under one clock sample, apply the closed-schema gate and pending-conflict rule, then publish the active decision and any serving Source duty. | `epoch` verifies the candidate; `durable` commits its generation, control floor, and current pointer; `duty` guards the serving role. |
| [`Refresh`](../../internal/network/state/refresh.go) | Admit one finite Source wave, journal attempts and exposure duties, fetch both configured Sources, verify their bundles, and select conflict, pending, active, or bounded failure before a reader can see a new current decision. | `source` owns TLS and private request/bundle framing; `epoch` authenticates the returned decision; `durable` and `duty` retain the resulting floors and exposures. |
| [Source serving](../../internal/network/state/server.go) | Answer one bounded request from the same verified current decision, or refuse while closed or unavailable; joined shutdown releases the server role. | `source` owns listener, TLS and request/response framing; State owns which authenticated bytes may be served. |
| [`AcceptClosedProfile`](../../internal/network/state/closed_profile_accept.go) | Verify and durably accept one signed profile only for the current closed Epoch; a second valid digest records a durable conflict instead of selecting a winner. If a crash left immutable bytes before the state record, only an exact verified retry may finish acceptance; readers cannot use byte-only evidence. | `closedprofile` verifies signed grammar; `durable` stores its accepted bytes and conflict floor. |
| [Current Snapshot](../../internal/network/state/snapshot_access.go) and [Node duty](../../internal/network/state/node_duty.go) | Derive copied State and Node-duty facts from the same authenticated current decision. | Node and Endpoint consume State projections without gaining State-root custody. |
| [ClosedProfile views](../../internal/network/state/closed_profile_view.go) | Expose the accepted profile's exact issuer/key and recipient constraints only while State and clock remain live. | `closedprofile` verifies signed bytes; Node and Endpoint receive copied facts without State-root custody. |
| [`Wait` and `Close`](../../internal/network/state/lifecycle.go) | Report terminal background failure; treat a scheduler tick refused by the closing owner as normal retirement; cancel and join accepted work, close the durable root, release the serving Source duty, and retain one cleanup result for all Close callers. | `resource` supplies pressure observations; State retains supervision and cleanup ownership. |

A closed-profile state record is published by rename followed by directory
sync. If that final sync fails, the new accepted or conflicting record may
already be visible. The live State owner retires, refuses closed-profile and
closed-route readers, and reports the terminal cause through `Wait` and
`Close`; reopening verifies the persisted record against the signed current
Epoch before serving it. If State has verified a second distinct profile digest
for the same Epoch but cannot persist its conflict, it likewise retires even
when the old accepted record remains on disk. A failed pre-rename conflict
cannot be inferred from that old record after restart; recovery uses only
verified persisted evidence.

The immutable ClosedProfile bytes are staged in an owned temporary file,
synced and closed before the final filename is published without replacing an
existing file. Its staging name is removed, then the root directory is synced
before the accepted state record can be written. An exact retry of
already visible matching final bytes syncs that file and its parent directory
before writing the record; byte equality alone is insufficient. A failed
write, file sync, or close leaves no new final file. A partial pre-existing
final file remains unavailable and is not overwritten. Reopen and Current
continue to use only an authenticated accepted state record, never byte-only
evidence.

The durable State root can retain 64 immutable Epoch generations and two
ClosedProfile files per accepted generation. Its root scan therefore admits
five fixed entries plus those 128 profile entries, with a separate finite
allowance of 64 interrupted staging files. The `generations` child admits 64
committed directories and its own allowance of 64 staging directories.
After taking the root lease, recovery checks both directory populations before
cleanup, removes only regular `.current-*` and `.closed-profile-*` files at the
root and `.stage-*` directories in `generations`, and syncs the changed
directories. Unrelated entries remain untouched and count against the stable
budget; an excess or unexpected staging type refuses recovery. This is
physical recovery only: State still authenticates restored generations and
retains its Direct Source collision guard before publishing a current decision.

The ARDS1D4 distribution journal records one finite Source cycle. Its two
`LATEST` attempt slots are 0 and 1; `BY_DIGEST` slots 2 and 3 use the same
Source index plus 2. The persisted attempt byte has these meanings:

| Byte | State | Transition owner |
| --- | --- | --- |
| 0 | Not started | A new cycle clears all four slots; only an unstarted attempt may begin. |
| 1 | In flight | Starting `LATEST` or `BY_DIGEST` commits the attempt before contact. |
| 2 | Completed | A `BY_DIGEST` response completed, or a final valid `LATEST` observation closed the attempt. |
| 3 | Failed or interrupted | A failed observation, interrupted attempt, or expired cycle closes it without replay. |

Within the recorded 15-second deadline, reopening the cycle preserves its
Source order and exposure floor. An in-flight `LATEST` is recorded as
interrupted when reached again and is not repeated; an unstarted `LATEST` may
run. An in-flight `BY_DIGEST` is recorded as interrupted before the resumed
wave. A `BY_DIGEST` response can reach completed before its bundle is
verified; wave closure records the final outcome. After a crash, a
completed attempt without that outcome is recorded as interrupted and its
spent selector is not replayed. Completing the resumed wave retains outcomes
and observed Epoch/digest evidence of consumed slots; only new observations
replace their corresponding slots. A new cycle clears this evidence. At the deadline, every unresolved slot becomes
interrupted and the cycle enters durable backoff. The journal keeps the same purpose/status
byte values and exact encoding across implementation refactors.

Source marks transport unavailability, TLS authentication, and response or
bundle framing failures at the operation that detects them. State gives caller
cancellation and deadlines precedence, preserves the four distinct protocol
statuses, marks response object-identity mismatch, and records the cause
without parsing diagnostic text. An unrecognized local or verification error
is `invalid-state`. The existing
`resource-failed` outcome byte remains readable in older journals; a new wave
uses it only when its operation identifies a concrete resource failure.
Peer close during TLS handshake, EOF before a response, and socket I/O failures
such as a reset are transport unavailability. Peer verification failure is
authentication; an invalid or truncated response frame is framing.

A terminal automatic-refresh or resource-governor failure also makes
State-owned Direct Source responses unavailable, including requests whose
connections were already accepted. A persisted or recovered Network State
conflict likewise makes `LATEST` and matching `BY_DIGEST` requests return the existing
busy status without a digest or payload, while `Current` retains its
conflicting diagnostic Snapshot. The resolver refuses before materialization;
Source still owns transport and framing.

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
can proceed. State retains one verified current Epoch decision and derives
reader Snapshots from it; the Source wave uses that same decision as its base.
If an ordinary pre-pointer failure prevents the first verified conflict record
from committing, the live State owner retires under its lock: `Current`, Direct
Source, and further admission refuse the prior decision, and `Wait` and `Close`
retain the failure cause. The prior authenticated generation and journal remain
the durable recovery evidence; a restart cannot infer the unrecorded conflict.
The serving Source duty stays protected until the retired owner's joined Close.
Every persisted current or pending generation is bound to its verified digest
by its immutable directory name before it is restored. A Source bootstrap with
no active predecessor never stages a future genesis: it records the complete
wave and defers retry until that Epoch becomes current, leaving the root
reopenable without a current generation.

A serving Direct Source owns local `direct-source/live` collision guards for its
current identity/family and every predecessor still needed by an accepted
handler. Offline acceptance and Source-wave activation advance the guards
through their shared active-decision commit. State holds the local role root
while installing the successor guard and publishing the decision, so another
role owner cannot observe an unprotected transition. A failure before the
durable distribution pointer is replaced retains the predecessor guard;
loss of a needed guard retires the serving State owner. If the pointer rename
succeeds but its directory sync fails, the visible floor is uncertain: the
live State owner closes under its lock, cancels work, and rejects later Current
reads and Source resolutions. Every possibly served predecessor and successor
guard stays until joined Close or verified recovery; it is not rolled back to
one possibly stale decision. Reopen verifies the journal floor and exact
generation before starting the Source server, or refuses recovery. Once the
floor commits without error, the successor guard stays with the recoverable
active decision even if the final State pointer needs repair. The precise
release rule is below.
State resolves the configured local role root once at Open, so later duty
replacement and release use the same root if the process working directory
changes.

### Direct Source Duty lifetime and release

This contract is selected by
[ADR-0118](../adr/0118-retain-direct-source-guards-through-dependent-work.md)
from [R-169](../archive/research/records/r-169-source-duty-lifetime.md). Serving Duty
still stores Epoch `ValidUntil`, but `direct-source/live` conflict truth follows
explicit owner release; State holds accepted-handler predecessors through their
joined close. Outbound contact uses an owner-held live guard across the journal
deadline. After both attempts and terminal publication join, State replaces it
with a time-bound exposure through the current or pending Epoch bound. An
interrupted journal retains the live guard through `Close`; verified reopen
may release work-only retention once the old process and contacts are gone.
Neither timestamp alone proves safe release.

For a serving Source, State must install an effective `direct-source/live`
identity and family collision guard before admitting a connection or publishing
a decision that can be served. The guard covers the selected decision and every
accepted connection/response handler that could use its bytes. `ValidUntil`
remains an authenticated Epoch bound; it does not by itself authorize removal.
State first stops further admission and joins the listener and all handlers,
then removes the last serving guard. A normal `Close`, terminal retirement, or
explicit server stop may take this path. A failed removal keeps a collision or
refuses reopening; it never becomes a readable no-conflict result. Choosing
whether an expired retained Epoch should still produce `ok` requires a
separate clock/response-policy decision and does not weaken this guard.

Active publication must install the successor guard before any reader can
select successor bytes. A handler that selected predecessor A may still send A
after B becomes current. If C follows B before that handler joins, A, B, and C
identities and known families remain guarded as long as each has a possible
dependent handler or current readable decision. Implementations may retain all
predecessors conservatively until *all* accepted handlers join, then shrink to
the current decision. State's serving producer may hold overlapping
`direct-source/live` predecessor records for the same identity/family. State's
exposure producer may retain overlapping *distinct* contacted tuples across
waves when they are still dependent; the two Sources in one accepted plan remain
distinct in identity and family. Any collision between different producers
remains a refusal, including when identity alone or family alone matches. A
failed or uncertain publication must retain every possibly served
identity until State can verify the pointer and handler set or retire and join.

For an outbound wave, State guards both precommitted Source tuples before
contact. The recorded 15-second cycle deadline limits new attempts and
recovery replay; it does not release a guard while contact, TLS, verification,
or terminal State publication is in progress. Cancellation and deadline
expiration close attempts and join their work before release. If the wave
accepts current or pending State, its contacted identities/families remain
excluded through that authenticated derived State's terminal bound and any
dependent work, even after the wave joins. A wave with no accepted derived
State may release the new wave's exposure only after its minimum exposure lease
and all attempts and terminal publication have joined, and only if no earlier
retained State still depends on the same Source. Interrupted or uncertain
publication keeps the conservative guard and refuses new dependent work. The
existing two-tuple Source history and installation-wide 64 Direct Source record
cap stay hard; exhaustion refuses a new contact or publication before an
unguarded identity can be exposed.

Reopen takes the exclusive State-root lease, verifies durable current/pending
generations, journal and pointer floors, and restores every guard needed by
readable State before allowing a Source response or new wave. After a crash,
process death plus the exclusive root lease prove that old handlers cannot
resume; there is no persisted handler set to reconstruct. State may reclaim its
own serving predecessor only when the verified active floor no longer names it
and it protected no other retained decision. For outbound exposure, State may
reclaim a work-only guard only if the verified journal and derived State prove
no retained dependency; otherwise it remains protected. An ambiguous pointer,
journal, dependency, owner provenance, or guard write fails closed.
No other producer's record is cleaned up during this recovery. Implementation
must make this lifetime visible to `duty.Conflict` across clock boundaries;
merely storing a future `NotAfter` is insufficient if active work can outlive
it. Tests must include a blocked real handler and a blocked real wave,
A→B→C with identity-only and family-only changes, cap exhaustion, and crash
points before/after pointer and guard commits.

A root with retained outbound Source exposure history refuses a different
configured Source plan: the changed plan cannot establish the historical
guard's identity/family provenance. A terminal journal may reopen without
Sources for offline reading or `Accept`; it leaves the previously committed
Duty producer intact. An active interrupted journal without its Source plan
requires explicit recovery before the root becomes readable.

The closed Route profile pins its Epoch envelope: new closed candidates are
accepted only as AREP v3 (ADR-0111). Offline acceptance and the Source-wave
verifier share one schema gate and refuse a retired envelope with the typed
`ErrLegacyEpochIntake` before any commit, staging, or activation. A retained
current, recovered-active, or pending generation written under AREP v1/v2 is
classified at Open as a `RecoveryRequiredError` with its generations, pointer,
and control floors preserved byte-intact; historical predecessors keep their
existing chain authentication until the retained population is closed.

State also owns the one active Source wave across bootstrap, caller-requested,
and automatic refresh. An automatic tick that arrives while that wave is active
is non-terminal and leaves the existing wave and scheduler live; it neither
publishes a second result nor records a State failure. A completed or rejected
wave still follows the normal availability, clock-confidence, and durable
admission rules, and an actual terminal automatic-refresh failure remains
visible to `Current` and `Wait`. `Wait` also follows the resource governor
when the H3-S runtime profile is the only background State work, and reports
its terminal failure. This tracer retains at most two Direct Source exposure
tuples in its durable history. A changed Source plan that would exceed
that bound is refused before the wave or contact; State neither truncates the
history nor writes an undecodable control generation.

## Retired generation-2 Route grammar

`ardents-interactive-route-v2` identifies the former native Route grammar, not
an accepting C0 Node duty. The selected closed Route uses `ardents-route-v3`;
old Node selections are refused before Network effects under ADR-0089, and
Node compares the exact stale `Profile` only to refuse it without side
effects (ADR-0093). The v2 EntryBinding and the reciprocal LegBinding
grammars are retired with the whole v2 execution closure (ADR-0093);
LegBinding was wire-only and never persisted. The sealed Introduction v1
grammar and its canonical vectors are retired by ADR-0094; ADR-0026 and
ADR-0034 remain the historical provenance of its bytes. The
`IntroductionPublic` data remnant was closed by ADR-0102 with the typed
refusal disposition: Credential v3 and the current v3 Instance request/response
grammars carry no introduction key, and old roots are refused at their marker
without a compatibility decoder under ADR-0117.
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
credential journal, and Service Connection owns replacement decisions through
its own attachment type; the former Entry replay and Invite-journal state is
retired with the Invite subsystem (ADR-0106).

Closed Entry selection and closed Issuer root startup join their primary
refusal with any failed exclusive-root release and return no owner. Issuer
initialization also requires successful root release before returning its public
SPKI receipt; a release failure returns an error and no receipt even when the
immutable material was committed. Retained roots are never reset by cleanup;
an explicit reopen still verifies that same material and authority.

The Entry attachment execution machinery is retired by ADR-0095: `Acquire`,
its guarded carrier, and the cleanup-lease tracking had no production caller
after ADR-0093 removed the Route v2 attachment opener. ADR-0106 then retired
the whole Invite subsystem, superseding that retained data contract: the
attempt/contact journal schema was deleted with the machinery, and no code
path reads a legacy Invite root.

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
`internal/route/carrier/closed_node_carrier_test.go`,
`closed_shared_carrier_test.go`, and `closed_role_carrier_test.go`, including
peer rejection and QUIC handshake
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

`ardents-next network prepare-node-certificate` is the finite offline producer
for one local Node certificate. It takes an exact signed initial ARNR-v2 Record,
its independently reviewed SHA-256 and matching canonical public declaration,
then admits the original protected matching Node key. Network/epoch verifies
the canonical Record and ordinary signature and returns detached key/interval
facts; the command does not parse Record wire bytes or decide membership.
The standard-library self-signed Ed25519 leaf has an empty subject, CA=false,
DigitalSignature and server/client transport EKUs. Its interval equals the
Record exactly. Actual generated X.509 bytes, signature, key and closed profile
are verified before durable public PEM output. No private key is exported or
replaced. The original finite signing reference and native custody remain
required through final acknowledgement. The [operation owner](../operations/qualification-release.md#local-node-certificate)
defines exact inputs, effects and failure; the certificate supplies neither
State trust, assignment, currentness nor process admission. Runtime Node and
Route independently retain their exact member/key, TLS/ALPN and lifetime checks.

Canonical Control exposes the same closed Node certificate profile through
`forwarding prepare-node-certificate`, without calling a Next or old runtime
backend. Its separate `prepare-node-key` creates only a protected local Node key;
`prepare-node-record` constructs the public declaration and unsigned message
from explicit flags, and `sign-node-record` consumes their exact reviewed bytes.
The independent NodeID, effective-family assertion and Network ID are never
derived from that key. New Epoch owns canonical preparation/signature verification;
composition owns exact purpose, current finite reference, original secret/native
custody, standard X.509 construction and joined output. The plan is machine
generated, not a requirement to edit JSON. No State acceptance, role assignment,
Source authority, runtime launch or private key export is supplied.

Offline `ardents-next source prepare-trust` consumes only externally supplied
public chains/roots and independently reviewed file commitments/leaf pins for
two Sources and one client. `network/source.ObservePublicTrust` checks X.509
purpose/name/reference and the same declaration/collision and domain-separated
key-digest mechanism used by runtime New and TLS verification. It produces no
live Plan or trusted clock. Command composition retains original public native
custody and writes existing serve/acquisition formats; private paths are opaque
and never opened by preparation. The
[operation owner](../operations/qualification-release.md#source-public-trust-preparation)
defines its finite input/output and failure boundary. Supplying credentials
does not implement an issuer or attest independent families. Runtime key-pair
admission, State time, process protection and joined termination remain separate.

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
pin. `ardents`, `ardents-node` and the isolated `ardents-next source serve` consumer
read the declared PEM key pairs and roots
while constructing the bounded Source configuration; `internal/network/source`
then owns copies for its one configured TLS client or listener. The isolated
consumer instead uses `internal/successor/network/source` through its new State
owner, without an old runtime bridge. Network State
supplies the Source TLS verification clock from its sole configured time owner;
a nested override is refused before opening the State root. Replacing a PEM
file does not alter a running Source process: there is no hot reload or
Source-side certificate issuer in the maintained surface. A changed certificate
therefore needs a separately checked new configuration and lifecycle action;
the X.509 `NotBefore`/`NotAfter` limits are checked during a new handshake.
The current contract does not promise seamless rotation or that an already
established TLS connection is immediately interrupted when a certificate
expires.

A finite Source fetch binds dial, handshake, response, and terminal TLS close
to the caller's total exchange context. Cancellation closes its original
established socket, joins that exact cancellation callback before fetch returns,
and discards any partially received Object Digest; State cannot
start a BY_DIGEST fallback from a canceled exchange. A live partial response
may retain its transport-observed selector for the one bounded fallback.
Serving uses the same physical cancellation mechanism before releasing its
active handler. A held original native TCP Close tests this join independently
of successful authentication or State acceptance.

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
owner joins the profile's numeric Role Domain to that Epoch assignment
(initiator=1, rendezvous=2, responder=3, introduction=4; ADR-0103) before
providing a usable duty view. An
accepted successor, expiry, conflict, or withdrawal therefore closes the old
admission authority without a polling grace period.

Terminal success is conditional on known cleanup. Role drain, listener or HTTP
shutdown, and owned issuer/Entry root close errors propagate to the lifecycle
result. Node may emit `DRAINING` while cleanup is attempted, but it must move to
`FAILED` and must not publish `WITHDRAWN` if any required cleanup fails or its
result is unavailable.

Lifecycle output remains a bounded diagnostic projection. On Unix, each write
uses one retained nonblocking descriptor and advances its offset only for
positive syscall byte counts. Saturated pipe or journal socket output waits
within the required caller deadline; cancellation and peer write failures
remain errors. Successful output neither grants readiness nor proves cleanup.

The shared Hosting handle closes and the local-role producer record is removed
before `WITHDRAWN` is published. If a role has not joined by its bounded drain
deadline, Node reports `FAILED`, retains the local-role conflict until its
authenticated expiry, and defers shared Hosting close until that role joins.
The lifetime owner retains an eventual Hosting close error for a later `Close`
call; that error cannot revise the already returned bounded `Run` result.
Node resolves the configured local-role root once when starting the process,
so later retention and removal use the same root even if the working directory
changes.
For emergency resource pressure, Node emits resource `DRAIN` before stopping
the duty and resource `EXIT` only after a successful withdrawal. A failed drain
emits lifecycle `FAILED` without claiming that resource exit completed.

Concurrent retained Node, forwarding, and JOIN duties share successful Hosting
observations only while the actual observation timestamp satisfies each caller's
requested age and the fixed one-second cap, and the provider period is current.
A completed shared flight is rechecked for each waiter; a stricter caller
refreshes through the Hosting owner rather than re-aging another caller's
measurement. Period expiry re-reads the owner's current pressure decision, so
a cached pre-expiry observation cannot retain Drain=false. Every reservation attempt, including failure, invalidates cached and in-flight reuse; failed release also invalidates reuse, while successful release keeps conservative
reserved-byte evidence until the next fresh observation. Invalid freshness and
unavailable contexts retain the Hosting owner's refusal boundary.

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

Node's existing behavior-test `ResourceMeasure` seam supplies both profile
pressure and hosting owner-use observations when explicitly set. It does not
replace durable hosting allowance accounting, and a measurement error still
fails closed. Maintained runtime callers leave it nil: hosting owner use is
measured by the live cgroup adapter, including its invalid-sample rule when a
process disappears during measurement. A component fixture with a declared
owner-use model cannot qualify real OS placement or resource use.

### Forwarding development process profile

The new-domain Forwarding runtime selects one mandatory local development
profile, `ardents-forwarding-process-v1`. The Product Owner approved this
bounded envelope on 2026-10-12. It is separate from the predecessor command's
unselected resource-profile input and does not revive a historical profile.
Selection is not evidence of an implemented accepting Node, supported host,
installed invocation, throughput, capacity or privacy qualification.

| Checked native setting | Required value |
|---|---|
| Platform and placement | Linux, cgroup v2, one original process in a separate leaf cgroup |
| `cpu.max` | `100000 100000` |
| `memory.high` | `805306368` bytes (768 MiB) |
| `memory.max` | `1073741824` bytes (1 GiB) |
| `pids.max` | `256` |
| `RLIMIT_NOFILE` soft and hard | `1024` |
| `GOMAXPROCS` | `1` |
| `GOMEMLIMIT` | `536870912` bytes (512 MiB) |

The process-resource mechanism checks the original process/cgroup and actual
kernel limits; supplied declarations alone cannot admit work. Its observations
include actual process and cgroup memory, cumulative CPU, an elapsed CPU
baseline, native pressure and memory events, descriptors, threads and Go-managed
memory. Each observation declares its scope, observation interval, freshness
and availability. An initial CPU counter without an elapsed baseline is not a
measured CPU rate. Missing, stale, ambiguous or changed original placement
refuses admission. Unsupported platforms refuse before runtime effects.

Protection admission is synchronous at the original work owner's checks.
Notifications and periodic sampling do not give an old accepting observation
a polling grace period. Pressure or evidence loss denies dependent effects and
interrupts original users; the Node lifetime still joins Route before returning
reservations, closing purpose roots, removing its participation producer or
releasing remaining State/resource/key owners. A bounded wait failure cannot
publish joined completion or create a replacement invocation.

First-cause diagnostics are a local read of the original retained phase/cause,
elapsed/deadline and available resource facts. Reading them must not wait for a
physical join or perform a foreign observation. In particular, the bounded
`completion-unavailable` report may include an already known original cause
while cleanup is still unknown. An already canceled original caller is retained
when Stop denies effects; later cleanup errors stay separate. Missing first facts
remain absent. Neither this projection nor the bounded report grants admission,
releases resources, changes the terminal result or establishes physical completion.
The original role report remains one JSON object with string-valued fields.
Optional `first_failure` carries compact JSON as a string within that object;
appending another JSON document or changing existing field types is not permitted.
Retirement filtering preserves unchanged joined-error identities. Removing the
exact private retirement notice cannot reconstruct a neighboring owner's retained
operational failure and thereby relabel it as Route physical cleanup. Subsequent
owner-specific filtering removes only that exact operational subtree; independent
physical errors and wrappers remain retained through actual join.

The Node and Source development profiles use the kernel's ten-second PSI trend (`avg10`,
percentage units), not a claim that every durable operation is stall-free.
CPU `some` or I/O `full` at **1.00% or above** retires the original owner;
positive memory `some`, any high/emergency/PID event, and the table's existing
memory/FD/thread bounds still retire immediately. There is no delayed accepting
decision, second observation requirement, revival, or reset of a failed owner.
Every effect still requires an actual original observation completed within one
second; the existing 100 ms supervisor supplies additional checks. Kernel trend
publication and scheduler delay are distinct from that synchronous effect gate:
this is not an instantaneous stall-duration or availability guarantee.

This development refinement separates short measured scheduling/durable stalls
from established trend pressure. Its evidence was obtained before implementation
on Ubuntu 24.04/systemd 255.4, WSL2 kernel 6.6.87.2 and native ext4 storage:
32 serial 4 KiB create/write/file-fsync/rename/directory-fsync/readback/unlink
transactions used 0.32–1.18 s, with 0.016–0.066 s maximum transaction time in the
initial sample and 8.2 ms additional I/O full stall. Two bounded external 50%
CPU workers with the same durable workload produced CPU avg10 0.54%, whereas
the genuine original Issuer with those competitors reached 3.80% and refused
ordinary issuance. Previously captured original durable work produced I/O
avg10 0.18% and CPU avg10 0.54% (retained #523 evidence). These measurements do
not demonstrate a classifier or Issuer defect under the earlier zero criterion.

The selected 1% trend budget is a finite development allowance between those
observed brief and disruptive cases, fixed before final candidate trials;
it is not tuned per role or raised on failure. The kernel trend is an exponentially
weighted average, so it cannot be interpreted as exactly 100 ms in every ten
seconds. Required real sync, floors/readback, native identity/caller custody,
hard limits, Network authority, protocol/Work Safety deadlines and physical
join-before-release are independent requirements. Installed Source/Node, arbitrary host load, long-run capacity and platform support are unqualified.
The [kernel PSI specification](https://docs.kernel.org/accounting/psi.html)
(accessed 2026-10-20) owns trend and cumulative-stall semantics; the selected
issue retains raw finite measurements, failed attempts and source identities.

The Descriptor CPU negative control must establish the selected 1% overload,
not merely positive PSI. Its fixed twelve external 50%-quota helper groups share
one pinned allowed CPU; their aggregate declared caps are 384 MiB and 24 helper
tasks. The six-second detection bound and eight-second helper lifetime do not
extend the original Node or protocol deadlines. A finite original-three versus
twelve-helper comparison measured CPU avg10 peaks 1.62%/1.26% and joined original
retirement in both cases; the twelve-helper case completed in 6.37 s. A retained
earlier three-helper control failed to obtain retirement before the role bound;
that missing first PSI history is not retrospective proof of a Guard defect.
Failure to establish actual overload remains failed/invalid evidence, never a
passing refusal test. Ordinary workloads use no such affinity or competing load.

Route continues to own its finite queues and physical users. Queue/timer
diagnostics must distinguish measured, unavailable and unmeasured observations;
unobserved counts are never emitted as zero or used as capacity evidence.
Whole-process/cgroup limits include resources that cannot be attributed to one
Route queue. Hosting continues to own provider-period bytes and reservations;
this local process profile grants no token, assignment, Carrier choice or extra
Hosting allowance. Independent native observations and both-Carrier causal
tests must verify the actual new command; a component model, a container sharing
its measured cgroup or a Windows gate does not establish that result.

### Source development process profile

The Direct-Origin Source distribution command selects `ardents-source-process-v1`
under the exact finite settings and pressure/freshness rules in the Forwarding
table above: original sole-process Linux cgroup-v2 leaf, cpu.max `100000 100000`,
memory.high `805306368`, memory.max `1073741824`, pids.max `256`, soft/hard
NOFILE `1024`, GOMAXPROCS `1`, GOMEMLIMIT `536870912`. Each actual observation
must complete within one second. CPU some/I/O full avg10 >=1%, positive memory
pressure, event/hard limits and missing or changed evidence retire without revival;
the additional supervisor runs every 100 ms. Source requires its own native
command/resource evidence. Another workload's receipt supplies no Source proof.

`ardents-next source serve <plan>` constructs only existing new State and process
owners over an independently prepared State root. It serves authenticated Epoch
bundles; it is neither a receiving Node duty nor Route Source/Responder. Network
State owns trusted time, serving guards, current decisions, Wait and joined Close;
Source transport owns mTLS/framing and original sockets. Command supplies original
caller/process admission outside State locks, supervises background failure and
orders State Close before process pin release. No acquisition, signing, refresh,
certificate issuer, hot reload or expired-Epoch response-policy change is added.
This development envelope selects no installed, platform or capacity claim.

Command composition retains one original plan, certificate, private key and
client CA lifetime in `source_custody_linux.go`. Direct no-symlink ancestor/file
descriptors, inode and UID/GID/mode, single-link regular files, metadata and exact
bounded bytes are reobserved before and after native reads. The private file
belongs to the invoking user and excludes group/other access: retaining a
publicly accessible private key would not protect this consumer's credential.
This is runtime input custody, not offline signing's root-only/ext4/0700 policy
or an installed account/delivery rule. Public inputs retain their admitted access;
unrelated ancestor contents may evolve without replacing its identity/access.

Loss permanently denies the original lifetime and cancels dependent admission.
A 100-ms observer also detects loss during held work; effect and handoff checks
remain synchronous through State's existing PermitWork outside State locks.
Recovery/exposure I/O and listener creation each have post-I/O checks. Original
caller cancellation starts the same one-time State.Close even while a command
Check is held in native I/O. Native observations, callbacks and joins never hold
the input lifetime's short state mutex. Composition joins State.Close, its
original Wait and command observations before input Close joins its observers,
erases retained file bytes and releases descriptors. The command's loaded key
and process pins remain until that Close actually joins. The same failed
five-second notice covers held input Close as well as State/Wait/command uses.
Partial acquisition remains creator-owned; copied
handles share retained Close outcomes. Independent input/State/process/output
failures survive; fixed reports contain no raw paths or secret bytes. The
five-second unavailable notice is failed work and keeps custody until actual join.
Replacement requires a separately admitted new invocation, with no live reload
or revival after restoring a pathname. Only the clock path declaration is fixed:
State continues to verify live clock-file mtime and owns trusted time and floors.

### Issuer development process profile

The selected Issuer composition requires `ardents-issuer-process-v1`. It uses
the exact native settings and original measurement/freshness/pressure boundary
in the Forwarding table above: a sole original Linux cgroup-v2 leaf,
cpu.max `100000 100000`, memory.high `805306368`, memory.max `1073741824`,
pids.max `256`, NOFILE soft/hard `1024`, GOMAXPROCS `1` and GOMEMLIMIT
`536870912`. It is a distinct workload/profile identity; Forwarding's profile
and protocol allowances remain unchanged. Selection grants no capacity,
installed or platform qualification. Issuer's checked native workload must
independently exercise its actual command and both Carriers.

### Descriptor development process profile

The selected Descriptor composition requires `ardents-descriptor-process-v1`,
a distinct bounded development workload under the same exact native envelope
and original placement/freshness/pressure rules in the Forwarding table above:
sole original Linux cgroup-v2 leaf, cpu.max `100000 100000`, memory.high
`805306368`, memory.max `1073741824`, pids.max `256`, NOFILE soft/hard `1024`,
GOMAXPROCS `1` and GOMEMLIMIT `536870912`. Its native command and resource
workloads require separate source-matched checks; another role's receipts cannot
qualify Descriptor. Selection grants no installed, capacity, platform or privacy
qualification.

### Introduction development process profile

The selected Introduction composition requires `ardents-introduction-process-v1`,
with the same mandatory native envelope and original placement, measurement,
pressure and one-second freshness rules above: sole original Linux cgroup-v2 leaf,
cpu.max `100000 100000`, memory.high `805306368`, memory.max `1073741824`,
pids.max `256`, NOFILE soft/hard `1024`, GOMAXPROCS `1`, GOMEMLIMIT `536870912`.
Its independently pinned native command/resource workload is distinct from the
other roles. Selection grants no installed, capacity, platform or privacy
qualification. Registration remains 1 MiB/600s and submission Control64KiB/30s,
with the unchanged ten-second capsule and original slot bounds.

### JOIN development process profile

The original Rendezvous DataJoin composition requires
`ardents-join-process-v1`, a separate fixed native workload under the same
Forwarding table envelope: sole original Linux cgroup-v2 leaf, cpu.max
`100000 100000`, memory.high `805306368`, memory.max `1073741824`, pids.max
`256`, NOFILE soft/hard `1024`, GOMAXPROCS `1`, GOMEMLIMIT `536870912`.
Actual original process/cgroup/PSI evidence and its one-second freshness bound
are mandatory. A profile string is no admission or qualification evidence.

### Original JOIN composition

`node.ListenJoin` and `ardents-next node join` compose one original
Role2/subrole4 invocation. Command owns genuine State and closes it last.
Node retains its caller, process guard, Network participation producer, exact
credential files and in-memory TLS key, Receiving and Hosting owners. Original
credential checks refuse lost/replaced files or altered bytes; no key hot reload
or successor adoption exists. Native/producer/credential checks fence foreign
State I/O outside local lifetime locks; late returns also check original caller
and synchronous denial. Peer refusal cannot grant a replacement invocation.

Each side reserves the complete shared class-2 allowance plus declared finite
termination coverage, then Receiver reserves actual JOIN frame/child capacity
before Receiving's irreversible spend. Route alone matches secret/context/profile
and opposite sides, completes both RESULT writes before activation and owns
credit, directional EOF, CLOSE, timers and physical joins. Unpaired setup is
bounded by the earlier original request or thirty seconds from reservation
under ADR-0121. No JOIN refill, refund, renewal or retry is added.

Stop seals original admission and interrupts dependents before waiting.
Receiver joins both sides, selected writers, listener and Carrier before
Receiving/Hosting close; producer, native pins, TLS and command-owned State
follow. First operational and independent physical failures remain retained
through concurrent/copied/repeated Close. A bounded unavailable report keeps
resources until actual completion; absent PID or path never proves join.

| JOIN invariant | Owner and real consumer | Reproducing controls |
|---|---|---|
| Genuine original authority and final handoff | Node observes signed State, exact duty/process/producer/credential; canonical command exposes original listener | `TestNativeJoinOriginalAuthorityAndJoinBothCarriers`, signed successor/conflict/expiry, direct caller/Stop, held State return, process/key/participation loss and canceled partial startup |
| Full reserve before spend and no refund | Hosting and Receiving; Receiver owns capacity and delayed physical return | `TestNativeJoinUndercoverageBeforeSpendBothCarriers`, `TestNativeJoinOperationBoundariesBothCarriers`, pre/post-spend Stop/caller/successor/process loss and concurrent copied Close |
| Actual pair bytes, terminal and reopen | Existing Pairing/Receiver through two canonical native JOIN commands | `TestNativeJoinDuplexAndReopenBothCarriers`, independent duplex byte/credit/EOF oracle, typed CLOSE, retained spends and same endpoint/root reopen |
| Refusal and original physical loss | Route owns pair refusal/RESULT barrier/physical failures; Node retains original lifetime | `TestNativeJoinMismatchedPairRefusesBothCarriers`, `TestNativeJoinOriginalCommandLossAfterIOBothCarriers`; existing Pairing/Receiver causal queued/partial RESULT, third arrival and late writer controls |
| Partial acquisitions and unavailable completion | Creator joins rejected work; command retains State through original join | `TestNativeJoinPartialStartupRefusalsBothCarriers`, `TestNativeJoinUnavailableCompletionRetainsOriginalProducerBothCarriers`; fixed process resource controls under separate JOIN profile |

These are required source-matched development controls, not receipts or installed,
capacity, privacy, independent-security or whole-Route qualification.

### Original Forwarding composition

`internal/successor/node` and `ardents-next node forwarding` compose one exact
new-domain Forwarding duty. The original State owner remains command-owned;
the invocation retains native process admission, its private producer,
purpose-specific TLS material, receiving Admission, Hosting and Route users.
Copied handles share that original lifetime and late terminal failure; a zero
handle supplies no admission or completion evidence.
Every receiving grant reserves first and checks Hosting coverage of the complete
Forward class allowance before spend. Refill keeps the original reservation,
deadline and termination dependency. A smaller operator envelope refuses;
neither a small test workload nor successful HELLO can certify full coverage.

Checks cross owners outside the Node state mutex, reobserve process/producer/
State and compare the original profile and duty after I/O. Stop denies locally
and cancels the original users; Receiver.Close interrupts before its actual
join. Only after that join may purpose roots, Hosting, the original producer,
native pins, TLS material and then State close. The one-second observation cap
is rechecked after State I/O; expiry of that process observation retains the
process owner category. No cached positive or notification grants work.
An expired Epoch/Record denies original work but cannot release the live Node's
local identity/family conflict while its physical users are still joining.
Network retains the `node-duty/live` restriction until original producer Close;
its persisted bound is not renewed. Prepared/quarantined facts remain time-held,
and a new live Node cannot be minted with an already expired stored bound.

Route may return this exact invocation's unwrapped local retirement notice from
its authority watcher while cancellation joins. That notice is distinct from
a physical cleanup failure. All foreign causes and physical wrappers remain
failures, including a wrapper around cancellation or the local notice. First
failure and actual cleanup failures remain in the retained result; a bounded
command wait can report completion-unavailable but cannot publish joined success.
An exact bare context error returned by the original process/producer mechanism
while its own caller is already canceled is retained as the first failure and
translated to that private notice for the Route watcher. Joined or wrapped
mechanism errors are never translated. This preserves the distinction between
original admission retirement and an actual physical cleanup error.

The native scenario in `cmd/ardents-next/node_native_linux_test.go` starts
hash-pinned actual commands with genuine signed State, Stock, Hosting and
independent roots. It exercises both Carriers, explicit cancellation, changed
native CPU protection and lost State clock evidence, then reopens Hosting and
participation after manager/process completion. Native process tests separately
exercise real descriptor loss and resident Go memory over the selected limit.
An undersized-envelope negative control leaves the exact real spend journal
unchanged and returns its unborrowed capacity before receiving spend.
The partial-startup matrix independently refuses wrong keys and roles, shared
roots, an unselected process profile, unavailable Hosting/spend/listener roots
and local identity/family conflicts on both Carriers. Initial signed State
conflict and expired membership also refuse before producer/spend effects;
their unavailability survives State reopen. The matrix checks original Source
history preservation, absent pre-admission spend, actual empty late-startup spend
headers and producer removal only after failed startup joins. A restored-input
control starts and joins the same signed member under the selected native profile.
The native parent-refill scenario in
`cmd/ardents-next/node_refill_native_linux_test.go` drives initial ADMIT and
parent refill through the actual command on both Carriers. Real Stock supplies
both tokens. Successful refill adds the independently fixed 988-byte debit
under the original termination reservation; actual competing Hosting work
and bounded real interface traffic refuse the addition before the second spend.
After original manager/process
join, reopening Receiving preserves the successful initial/refill burns and
permits the token refused before spend. Hosting retains measured use and has
no original reservations after joined retirement.
The native original-authority scenario in
`cmd/ardents-next/node_authority_native_linux_test.go` opens genuine State and
Node owners in one selected process, with signed inputs prepared outside its
measured cgroup. A scheduling barrier delays only the return of a completed
actual State observation. Stop does not wait under a Node mutex for that foreign
return; the original completion, producer and purpose root remain retained until
the watcher joins. Signed successor, signed profile conflict, member expiry and
stale process evidence refuse the original duty and survive applicable reopen.
Late startup cancellation occurs after observing the actual bound listener and
before successful handoff; it returns the original caller cause and joins its
partial listener, producer and roots. Copied handles retain the same result.

The actual command's unavailable-completion case in
`cmd/ardents-next/node_completion_native_linux_test.go` occupies the original
producer journal's exclusive lease as a test fault. A one-millisecond command
wait reports `retiring/completion-unavailable`; manager/process termination is
not Node completion. The unresolved producer remains conflicting across reopen,
and a new producer cannot adopt it. This grants no recovery or root-reset right.

These cases do not qualify stalled kernel/filesystem syscalls, arbitrary crash
recovery, installed operation, independent operators or privacy. The selected
issue owns full bounded acceptance and source-matched verification receipts.

| Original Forwarding invariant | Owner and concrete behavior evidence |
|---|---|
| Genuine startup and both-Carrier forwarding | Actual canonical command in `TestNativeNodeForwardingBothCarriers`; signed State, Stock, receiving Admission, Hosting and original native process/producer |
| Key/role/profile/root/conflict refusal and partial cleanup | Actual command in `TestNativeNodePartialStartupRefusalsBothCarriers`; preserved foreign Source/file bytes, absent early spend, empty late journal, root/producer cleanup and restored-input control |
| Native protection, availability and pressure | Actual command's CPU-protection loss; actual process-resource descriptor loss and resident-memory tests; `TestNativeNodeOriginalAuthorityAndJoinBothCarriers` post-State-I/O freshness refusal |
| Original authority, caller and final acquisition handoff | Genuine original State/Node native composition with signed successor/conflict, member expiry, held observation return, Stop/caller cancellation and late startup cancellation |
| Physical join before root/producer return | Held original watcher in native composition; occupied-lease actual-command unavailable completion; live producer expiry/durable foreign-replacement controls in new Network/duty |
| Initial/refill reservation and irreversible spend | Actual command in `TestNativeNodeParentRefillAndReopenBothCarriers`; exact 988-byte additional coverage, real pre-spend capacity loss, retained journals, replay refusal and measured-use floor |
| Original result and foreign cleanup provenance | Native cancellation/startup cases plus Node retirement/copy regressions; only the exact private notice is removed, and physical wrappers/foreign causes remain errors |
| Unsupported platform and bounded public output | `TestCompiledNodeUnsupportedPlatformRefusesBeforePrivateInput`; Node dispatch and inspection tests preserve finite output and distinguish unmeasured queues/timers |

### Original Issuer composition

`internal/successor/node` and `ardents-next node issuer` retain one original
authenticated Role2/subrole6/PurposeIssuer invocation. State stays command-owned;
process admission, private participation producer, exact-member Ed25519 TLS,
Receiving and Hosting are independent owners. Admission's retained issuing
application opens prepared quota, keys and results once, before accepting Route.
Private RSA and quota/result decisions remain exclusively with Admission.
No profile signing, root creation, rotation, recovery or automatic replacement
is performed by this runtime.

The original caller is checked directly before and after foreign observations,
in addition to the derived cancellation context. Base process/producer/State
observations never call back into Admission. Route's additional issuing check
uses that base observer, then reobserves native/State after key/journal I/O;
no recursion or foreign I/O under the Node lifecycle mutex is permitted.
The exact original binding and one-second process freshness cap apply at effects.
Request refusal and original owner failure remain distinct; only definite original
authority/caller/resource/custody loss retires the invocation.

Ordinary Receiving reserves and verifies the full Control64KiB allowance before
durable spend; no Forward refill is composed. Bootstrap independently reserves
its actual128KiB finite lane and termination before incoming work, without
receiving-token spend. Route retains its one Issuer work slot and bounded copies
through output, half-close and physical join. Issuing call completion cannot
return that slot. Quotas remain common across all original channels.

Stop synchronously denies/cancels issuing and Route admission. Route users join
before Admission joins its calls/observers and closes results, keys and quota;
then Receiving, Hosting, the original producer, native pins, TLS and State close.
First failure and cleanup causes remain separate in the original result. A
bounded command wait reports unavailable completion while the original process
and owners remain retained through physical join. The producer's node-duty/live
conflict persists past work expiry until actual producer removal.

The distinct native Issuer workload exercises actual bootstrap, genuine Stock
finalization, fresh ordinary issuance and spending newly issued Forward tokens
on both Carriers. Native component controls retain original key/quota/result
leases and producer conflict while an actual State observation return is held;
they exercise caller/Stop, signed successor/conflict, expiry, process freshness
and canceled late startup. Scheduling controls grant no successful authority.
Full bounded acceptance and source-matched receipts remain in the selected issue;
these mechanisms supply no installed, capacity or privacy qualification.

The native operation control uses genuine independent Stock/Prefix principals
and one native original Issuer. It delays only a real issuing caller's State
return at causal quota/result-journal boundaries; it supplies no successful
authority, signature or receiving Grant. Original caller denial before debit
leaves issuing journals unchanged; post-debit/result denial retains committed
bytes and withholds delivery. A signed successor after result commit denies the
original duty. Quota/key leases and the actual result-file lock, plus physical
Hosting, remain held through delayed original join. A competing operation cannot
add quota or burn another Receiving token. Explicit healthy exact retry and
independent issuing-owner reopen preserve complete response and journal bytes;
the reopened owners still perform full integrity/durability/crypto validation.

Storage uncertainty controls change only the native test child's file-size
limit, leaving one byte beyond its original quota or result journal. The kernel
performs a real partial append and returns EFBIG; no successful sync or commit
is substituted. Quota uncertainty exports no confirmation/result; result
uncertainty retains the complete prior quota debit and partial result bytes.
Both withhold delivery. Restoring the write limit cannot change the original
failed result across copies/repeated Close; fresh independent issuing Open
refuses the partial checksum chain without truncation, repair or floor reset.

### Original Descriptor composition

`internal/successor/node` and `ardents-next node descriptor` compose one original
authenticated Role2/subrole5/PurposeReachability3 invocation. State remains
command-owned. Node retains the original caller, native process guard, exact
member TLS material and private participation producer; Reachability owns its
independent original Store, proof/conflict policy, native custody and durability.
Admission owns Control verification and irreversible spend, Hosting owns full
64KiB work and termination reservations, and Receiver owns physical exchange.
No Descriptor or Identity domain, general aggregate or event-driven revoke is
introduced by this supporting composition.

Base process/producer/State observations run outside Node state locks and check
the original caller directly. Store custody inspection calls no foreign observer;
Node reobserves native process/State after Store I/O. The exact retained duty,
profile and one-second process freshness cap apply at effects and handoff.
An invalid/stale/conflicting/capacity-refused request cannot retire an otherwise
healthy invocation. Definite original authority, caller, protection or Store
failure denies work permanently; restored paths never revive the original.
Caller retirement and real in-flight native Store failure remain separate, so
retirement cannot conceal a storage failure or turn cleanup into success.

Stop synchronously denies original admission and Store borrows, then interrupts
dependents. Actual Receiver join precedes Store/Receiving/Hosting close, producer
removal and process/TLS release; the command closes State last. First operational
failure and actual cleanup results survive copied handles and repeated Close.
Descriptor retains its original operational cause locally and passes its private
denial to dependents; a foreign failure is not a Receiver physical release result.
After Store join it reads the separately retained native failure, so a late
storage failure after caller cancellation still prevents successful withdrawal.
A bounded retirement report retains all original owners until actual completion.
These implemented mechanisms require their own native both-Carrier/causal/race/
crash checks; selection or compilation supplies no installed, operator, platform,
capacity, privacy or independent-security qualification.

The following controls are behavioral oracles for this receiving boundary.
Results apply only to their checked source and selected environment; the testing
owner defines complete profile dispatch and artifact pinning.

| Original Descriptor invariant | Owner, genuine caller and effect boundary | Positive / causal refusal controls |
|---|---|---|
| Strict original startup and partial custody | `node_descriptor_linux.go` opens State and calls `node.ListenDescriptor`; each rejected acquisition joins before a fresh invocation | `TestNativeDescriptorPartialStartupRefusalsBothCarriers`, canceled original startup and unsupported-platform controls; corrected inputs start a separate original command |
| Full Control coverage precedes spend | Node `admit` calls Hosting Reserve/CoversJoint inside genuine Receiving Accept; Receiver retains the transferred reservation | `TestNativeDescriptorPublishLookupAndReopenBothCarriers`, `TestNativeDescriptorUndercoverageBeforeSpendBothCarriers`; unchanged Receiving/Store before coverage, no refund after spend |
| Fresh original authority at effects and ACK | Node `currentBase/current`, Network MatchDuty, original process Check and genuine Receiver/Store operations | `TestNativeDescriptorOriginalAuthorityAndJoinBothCarriers` and `TestNativeDescriptorOperationBoundariesBothCarriers`; signed successor/conflict/expiry, stale post-I/O process evidence, caller/Stop/process loss before spend and before/after commit |
| Signed bytes and retained floors remain owner-verified | Reachability Verify/Store and real Prefix publish/lookup; Store commit is distinct from Route ACK | Independent Descriptor draft/signature and ARDP byte vectors; Store revision/publication conflict, expiry/overlap/capacity controls and native partial-write/process-crash reopen; actual TLS/truncated RESULT/half-close refusal controls |
| Delayed work cannot release custody or revive admission | Node denial interrupts dependents; Receiver and Store join before Receiving/Hosting/producer/TLS release | Native held actual State returns, concurrent same-Source lookup and copied Close; root/record/marker/lock loss and restoration; late Store failure after caller denial remains a failed original result |
| Native protection loss and pressure retire the original | processresource measures original placement/limits/PSI; Node retains first cause and command closes State last | `TestNativeDescriptorOriginalCommandFailureBothCarriers` plus the three mandatory original Descriptor resource controls; bounded helper pressure never changes the role envelope |
| Bounded reporting cannot manufacture completion | Canonical command waits for original Done/Close before State Close and process exit; Network owns producer conflicts | `TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerBothCarriers`; shared NodeProducer identity/family, foreign Source and expired-work controls retain conflicts until actual original removal |

### Original Introduction composition

`node.ListenIntroduction` and `ardents-next node introduction` compose one
original Role4/subrole3 invocation. The same original caller/process/TLS/private
participation lifetime as the other selected Node roles surrounds genuine
Receiving, Hosting and the prepared Route Registry/History. Receiving opens
before Route consumes its opaque one-use fresh-root fact; Registry is ready
before listener publication and is borrowed by exactly one Receiver. Node owns
no filesystem slot policy, arithmetic, decrypt/recipient selection or readiness.

Base observations check original caller, process, producer and genuine State
outside lifecycle locks. Native Registry checks invoke no foreign observer;
fresh base observations follow that I/O. Stop short-denies Registry admission
without waiting behind durable transactions or physical callbacks. Receiver,
registration readers/writers, delivery and withdrawal users join before Registry,
Receiving and Hosting close; producer removal, native/TLS release and command
State closure follow. First operational and true physical release causes remain
separate through copied handles and repeated Close. Peer refusal cannot retire
a healthy invocation; restored native paths cannot revive a failed original.

Each accepted purpose reserves the complete respective class plus termination
before spend; Registration also reserves Route capacity before spend. No refill,
refund, enlarged allowance, automatic restart or new slot for Source submission
is provided. Native command/HPKE holder controls are bounded development evidence,
not qualified private binding or installed Service/Connection acceptance.

| Introduction invariant | Owner and actual consumer/effect | Positive and causal refusal controls |
|---|---|---|
| Original authority and freshness | Node base observes genuine State/process/producer outside locks, then original Registry and fresh base; `node introduction` owns State last | `TestNativeIntroductionOriginalAuthorityAndJoinBothCarriers`: held real State, Stop/direct caller, signed successor/conflict/expiry, native memory/descriptor/custody loss; copied terminal result |
| Admission and durable ACK ordering | Route capacity precedes Receiving spend; Hosting covers the whole respective class; History claim precedes REGISTER RESULT | `TestNativeIntroductionUndercoverageBeforeSpendBothCarriers`; `TestNativeIntroductionOperationBoundariesBothCarriers`: unchanged pre-spend journals, irreversible post-spend/claim, cancellation before ACK, concurrent copied Close, late native failure and physical retention |
| Original slot and opaque delivery | Route Registry/History/Receiver plus real Domain4/Domain1 Prefix; Node sees no recipient plaintext | `TestNativeIntroductionRegisterSubmitWithdrawBothCarriers`, independent stdlib HPKE recipient/ARDISL01 bytes; `TestNativeIntroductionOriginalHistoryLossBothCarriers`; Route's queue/rate/reserve/RESULT/CLOSE and crash/partial-write controls remain with Introduction |
| Startup and terminal custody | Command validates independent roots/member/TLS/profile; Node joins partial acquisitions and retains producer conflicts | `TestNativeIntroductionPartialStartupRefusalsBothCarriers`, retained-empty Receiving refuses renewed birth; `TestNativeIntroductionOriginalCommandFailureBothCarriers`; `TestNativeIntroductionUnavailableCompletionRetainsOriginalProducerBothCarriers` |

These tests identify required mechanisms and oracles; their execution/source
receipts and full issue acceptance belong in GitHub. They do not themselves
attest a qualified recipient. The affected genuine qualified Publisher delivery
remains selected by `execution-route-check` under its own installed worker owner.

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

The former native resource profile identity
`ardents-rendezvous-dedicated-host-v1` was proven production-unreachable —
no runnable Node or State configuration can select it — and was removed
together with its Node clock-observation trigger by ADR-0116; the resource
guard now refuses it like the historical `h4-5-rendezvous-alpha-v1`
identity. The Node command refuses an old reservation before
resource-profile validation and no longer normalizes the historical
identity into a runnable plan. Retained engine tests and `h3-*` guard
profiles are compatibility evidence pending their own deletion slices, not
accepting command routes.

The dedicated-host Contributor command, its module, and its runbook were
removed entirely by ADR-0114 after the Product Owner confirmed that no live
`ardents-rendezvous-contributor` installation remains; the ADR-0089
owned-shutdown obligation has no object. Any residual installation bytes, if
ever surfaced, stay on disk with no reader.

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
- [ADR-0024](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0024-native-interactive-route-foundation.md),
	[ADR-0070](../adr/0070-own-volatile-user-route-orchestration.md),
  [ADR-0025](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0025-state-referenced-entry-invites.md),
  [ADR-0072](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0072-adopt-offline-enrollment-route-v2.md), which
  supersedes their C0 Route/Entry selection,
  [ADR-0048](../adr/0048-maintain-tcp-and-quic-carriers.md), and
  [ADR-0049](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0049-defer-blocked-entry-profile.md) define the selected
  native Route and Carrier facts.
- [R-092](../archive/research/records/r-092-native-node-operating-profile.md) retains the
  measured dedicated-host Rendezvous Functional Alpha result for its original
  recorded candidate. That historical result does not qualify the current C0
  candidate, select another duty, or establish public capacity, availability,
  co-resident, permissionless, or independent-operation claims.

The receiving replay Ledger retains one terminal Close result, joining any
recorded journal failure with lease-release failure. The ClosedTokenIssuer
erases its in-memory private material before release and retains the release
result for every Close caller. Their lifecycle locks join concurrent closure;
neither operation resets durable journals or issuer material on disk.

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
limits both admission and actual dispatch to four per second. The implemented
registration allowance is bounded to 8 MiB and includes delivery OPERATION, RESULT,
CLOSE and reserved withdrawal; another submission token cannot enlarge it.
This differs from the selected 1 MiB class-3 allowance in the
[admission contract](private-admission.md#resource-classes-and-admission);
the implementation description does not accept a quota increase. Publisher
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

## Local process debugging

The command adapter can explicitly enable the owner-private process Interface
with `ARDENTS_DEBUG_SOCKET` under the
[local diagnostic contract](../development/local-diagnostics.md). It supplies
runtime counters and finite profile/trace requests without modifying Route,
State authority, peer selection, Application privileges or worker confinement.
Empty input creates no diagnostic resources; configured admission failures
precede product dispatch and diagnostic cleanup is joined to command outcome.
The diagnostic interface reports process observations; existing owner events
remain the source of readiness and typed product failure.


## Proposed installed infrastructure

**Status: accepted contract under ADR-0123. Source artifact/admission, read-only
preflight and the bounded initial stopped transition have direct component
consumers. Source startup and installed qualification remain separate.**
[ADR-0123](../adr/0123-bind-installed-infrastructure-generations.md) isolates the
consequential artifact, persisted binding and bootstrap decision. ADR-0119 and
ADR-0120 retain their existing Endpoint/recorded-operation scope. This section
owns the infrastructure design; GitHub owns selection, sequence and completion.
It adds no package permission, executable interface or operational instruction.

### Assessment and existing source boundary

This is a brownfield modular Go system with behavior-rich State, Admission,
Hosting and Route owners. Installation/composition is a supporting subdomain
with complex original-lifetime invariants: use light DDD. The consistency unit
is one original installation transition or runtime invocation, not a global
Node/Identity/Control aggregate. The existing domain map remains authoritative.
No repository, event bus or eventually delivered revoke is needed. Semantic
commands are admit/prepare/select/start/deny/join; observations of prepared,
selected or joined state convey only the producer's actual result.

Source code references below are relative to the repository root. They describe
implemented behavior, not receipts. Native development receipts must match their
recorded source; source inspection alone is not execution.

| Composition / exact consumer | Inputs, genuine verification and effect | Resource and terminal owners / current limit |
|---|---|---|
| Source: `cmd/ardents-next/source_linux.go`, `source_input_linux.go`, `source_custody_linux.go` | Closed serve plan; `network` root/ID/authority list/threshold/profile pin/clock file; Source process profile; independent participation root; address, server certificate/key, client CA and 1..3 exact client pins. Original plan/cert/key/CA file/ancestor custody, `networkStateConfig`, `state.Open/Current`, Source mTLS and Guard.Check retain their separate boundaries. An independently prepared State root is required; serving does not acquire/sign/refresh State | State owns authenticated responses, exposure guards, transport/background Wait/Close and its root. Composition denies PermitWork and interrupts before waiting; original State Close/Wait and command observations join before input Close, then loaded command key and process pins release. Five-second notice retains failure and continues waiting through all original uses and input Close. This development custody is not an installed binding |
| Forwarding: `node_forwarding_linux.go` → `node.ListenForwarding` | Common Node inputs below; `work`, `termination`, finite retirement wait. CurrentRuntime → RetainDuty → exact role.Authority; original process and participation producer precede receiver work; full Forward allowance reserve precedes irreversible Receiving spend | Route owns listener/parents/children/refill/physical joins; Node owns original lifetime and purpose roots/Hosting/producer/Guard/key; command owns State last. Its bounded command wait can return completion-unavailable before its retained cleanup goroutine finishes; process exit is not proof of producer release |
| Issuer: `node_issuer_linux.go` → `node.ListenIssuer` | Common inputs plus `issuance` plan with separate Admission quota, private key and result roots/bindings; Control work, bootstrap work and termination. Admission issuer application opens exact prepared material; signed current State binds its issuer reference | Admission owns quotas/key/results and durable debit before response; Route owns Credential listener and physical users; Hosting owns separate Control/Bootstrap coverage. Node retains all until join. No Forward refill, generic signer or key export |
| Descriptor: `node_descriptor_linux.go` → `node.ListenDescriptor` | Common inputs plus independent Descriptor root, Control/termination coverage. Genuine `reachability.OpenStore` verifies proof/history, Receiver owns transport/admission | Store owns proof/conflict/floors and ACK; Node joins Receiver before Store/Receiving/Hosting/producer/process/key; State last. Store ACK is not Publication readiness |
| Introduction: `node_introduction_linux.go` → `node.ListenIntroduction` | Common inputs plus independent Introduction root. `introduction.OpenReceivingRegistry` consumes the genuine Receiving fresh-root fact before listening; Registry checks native/history binding | Route owns slots/non-reclaim floors/capsule delivery, not Admission. Join Receiver/children before Registry/Receiving/Hosting and producer/Guard/key; State last. REGISTER ACK is not installed or Service readiness |
| JOIN: `node_join_linux.go` → `node.ListenJoin` | Common inputs with JOIN profile and full class-2/termination coverage. Exact Rendezvous DataJoin duty; original credential file pins (`node/join_credential_linux.go`); genuine Receiver/Pairing | Route owns opposite-side match, both RESULT barrier, credit/EOF/CLOSE and physical join. Node retains Receiving/Hosting/producer/native pins/TLS until actual join; command keeps State. No refill, retry or renewed setup deadline |

Common Node declarations are exact Network config, NodeID, distinct participation,
spend and Hosting roots, certificate/private-key paths, exact role process profile,
work/termination coverage and retirement wait in 1..5000 ms. Extra role roots are
pairwise independent including ancestors (`node/root_paths.go` and command
validation); changing directory names cannot create new authority or erase history.
Network alone binds signed member/key/family/assignment/endpoint/Carrier/windows.
Node retains its original producer and admission latch; Receiver reobserves
current authority at effects. Process protection is not Hosting byte capacity.
`node/credential_linux.go` loads exact matching Ed25519 material with owner-only
private file access; Forwarding/Issuer/Descriptor/Introduction retain the loaded
pair, whereas JOIN additionally pins and rechecks original files. Do not claim
all five have JOIN's custody. Installation must independently bind inputs for
all roles; neither memory retention nor a self-signed leaf proves installation.

### Producer, proof and external-input inventory

| Input → current producer → verifier / consumer | Actual evidence boundary and remaining mechanism |
|---|---|
| Canonical unsigned initial Record/Epoch/closed profile → `network_preparation.go` and Network epoch/closedprofile public owners → ordinary verification and State acceptance | Public preparation is implemented. No private signer, State acceptance or independence follows from unsigned bytes |
| Exact signed Record and initial Epoch → `network_signing.go`, original protected purpose-specific key/pin/time → canonical verifier and `network accept-epoch` in `network.go` | Genuine finite sign/output chain exists; Record generation one and initial Epoch only, not arbitrary successor issuance or operational bootstrap |
| Exact signed profile → `network_profile_signing.go` and fixed public transcript → closedprofile.Verify and `network accept-profile` | Genuine producer; independent State signer is distinct from in-body Admission authority. Acceptance receipt is not current runtime permission |
| State and 1..64 Node private keys → `network_keys_linux.go` → each separate signer and public receipt verifier | Fresh original 0700/0600 root-controlled outputs; no NodeID/assignment, authority grant, runtime credential placement or renewal service. Offline Node-key file cannot simply be handed to a non-root runtime |
| Source public transport declarations → `source_preparation_linux.go` / `source.ObservePublicTrust` → `source serve`, installed Endpoint acquisition projection in `installed_source_linux.go` | Ten exact public files; private paths may be absent at preparation. Existing external CA/leaf/private-pair issuance and independently reviewed CA/key pins are allowed inputs, not a missing product PKI. Runtime TLS alone checks actual private match, trust, EKU/name/time; prepared receipt grants none |
| Node certificate → `node_certificate_linux.go`, original generated Node key plus reviewed canonical signed Record → epoch.VerifyInitialClosedRecord, standard-library X.509, actual Forwarding/JOIN TLS consumers | Public self-signed leaf, no new private key and no membership. Exact State/profile remain separate signed acceptance. Remaining installed custody/UID/access/renewal constraints are not solved by certificate creation |
| Prepared State root → `network.go` accept-epoch/profile through State; authenticated signed materializations and independently pinned authorities → State.Open/current verification | Maintained producer/consumer exists. Installed account must create/accept its own independent fresh history using that owner; copying a fixture or old root is forbidden. State schema/recovery/floors remain unchanged |
| Current clock evidence → explicit external clock-observation file plus State wall/monotonic observation → State trusted time and persisted floor | Intentional external evidence dependency. `state/clock_observation.go` uses the regular file's mtime as independent evidence; State compares wall/monotonic/independent time and retains its floor. No new clock daemon or unsigned timestamp-as-authority. Installed custody must bind path/writer/access; stale/unavailable evidence denies. Offline signing reference time is not this clock |
| Hosting root and finite envelopes → new Hosting command/owner → Node/Route reservation checks | Real provider-period initialization and retained accounting exist. Installed root placement/protection is missing; a smaller workload never waives full-class coverage |
| Receiving spends, Introduction history and Descriptor Store → actual receiving/Route/Reachability owners at original admitted startup → their own durable reopen rules | No separate provisioning task for every fixture root. Fresh-root creation and retained reopen are different operations; missing old floors cannot be repaired as empty. Installation supplies paths/access, never history bytes or successful ACK |
| Issuer permission authority, RSA key/quota/results → purpose-specific new Allocation/Custody and Admission issuerprofile/issuance consumers (`issuer_profile*`, `issuance*`) → Admission + exact signed State profile | Their existing purpose-specific imported credentials and independent reviewed pins are explicit prerequisites. Installation does not generate/import private RSA, mint quota, accept Permission or turn a Node key into a permission authority |
| Program, resources and initial provenance → Enrollment/Release/Installation | Current new Enrollment authenticates manifest-pinned Node/Custody companions outside Release metadata. `installation/generation_authentication.go` requires exact Endpoint targets and complete protected group. The separate Source infrastructure descriptor, initial verifier and coherent pair now have an isolated consumer; the separate bounded Source initial stopped transition now selects its own binding; guarded first startup has a canonical Node consumer, while installed qualification and recovery continuation remain separate |

These rows account for the genuine provisioning outputs of the public preparation,
Record/Epoch signing, profile signing, key, Source trust and Node certificate
changes. Isolated State/Admission/family fixtures used by their tests are not
additional product authorities. External credentials/trust/time remain explicit
operator inputs. Source infrastructure artifact admission is implemented through
the separate Enrollment/Release/infrastructure consumer. The remaining missing
boundaries are installed Source qualification, Node installation/startup and
infrastructure restart/replacement/recovery continuation, remaining command/tool
cutover and source-matched final evidence; not a speculative PKI. Source's bounded
first-start composition is implemented separately from those acceptance gates.

Reverse consumers matter. `cmd/ardents-next/main.go` directly dispatches all six
new compositions and public preparation. `network.go` directly accepts signed
Epoch/profile on independent roots. `installation.go` and
`installed_endpoint_linux.go` consume Endpoint authorization/startup; their
`installed_source_linux.go` reads an Endpoint's acquisition declaration, **not**
an installed serving Source. Native `scripts/node/run-*-native.sh` run pinned
development processes; systemd-run does not create an immutable installed binding.
`tests/profiles/headless-commands.txt`, `packaging/alpha-bundle/build.sh`,
`scripts/protected-generation-check.go`, `scripts/sign-qualification-release.go`
and the qualification release operation retain their existing command/Endpoint
inventories. They neither package nor sign the proposed infrastructure contract.
Canonical `ardents-node` Source/Installation directly consume the new owners;
their needed Control and artifact/qualification tools have separate explicit
consumers. Unselected Node commands and retained qualification scenarios still
have predecessor consumers; none may delegate to a new binary or share live roots
as a shortcut.

### Proposed exact artifact and installation binding

The following is the accepted contract under ADR-0123, not an extension
of any existing accepting parser. Final commands are `ardents-node` for Source,
the five duties and their Installation composition, `ardents-control` for public
preparation/inspection, `ardents` for Endpoint and `ardents-custody` for its
existing purposes. Infrastructure initial verification and native transition
must consume their opaque provenance/proofs in the same original command
operation. A control-process public receipt cannot transfer those rights.

The `infrastructure-<invocation>.json` is UTF-8 compact JSON plus LF,
maximum 16 KiB, fields in this exact order: `schema`, `platform`,
`release_identity`, `release_version`, `invocation`, `files`.
Schema is `ardents-infrastructure-artifact-v1`; platform `linux-amd64`; version
positive int64; invocation one of the six values in ADR-0123. `files` has exactly
two lexical keys: `ardents-node-linux-amd64` and `ardents-<invocation>.service`.
Values are lowercase SHA-256 of actual executable and closed unit-template bytes.
No paths, keys, plans, mutable roots, metadata/manifest digests or runtime facts
occur in it. Reject duplicate/unknown/noncanonical/partial or extra inventory.
This prevents a manifest/signing cycle. Both target Decisions must agree on
Targets version/digest and all common local, release, policy and reference facts;
the program bytes and every resource are checked before any native mutation.
Equal version/different descriptor refuses, including resource-only substitution.

The independent initial inventory is a separate
`ardents-infrastructure-enrollment-v1` document, not enrollment-v3: exact ordered
`schema`, `platform`, `release_identity`, `release_version`, `invocation`, `files`;
the same scalar rules and canonical 16 KiB bound. `files` is a lexical map of
exactly seven SHA-256 entries: the program, invocation unit, infrastructure
descriptor, `root.json`, `snapshot.json`, `targets.json`, `timestamp.json`.
All are direct names, regular original files bounded by the existing 64 MiB
artifact maximum; unknown/excess/noncanonical inventory refuses. An independent
initial pin binds the entire enrollment document before parsing, and each named
file is then verified. The running `ardents-node` must be the identical enrolled
artifact. Independent distribution must verify its bytes before first execution;
self-verification after launch cannot establish that provenance. A direct
final `ardents-node installation` consumer must perform initial verification and transition
in the same root caller, and a separately admitted non-root invocation runs the
selected Source. The isolated `ardents-next infrastructure` component now prepares
and verifies the Source descriptor and `infrastructure-enrollment.json` inventory
through new Enrollment/Release/infrastructure owners. Its separate initial stopped consumer repeats fresh native admission before account, delivery and selection effects.
Existing v3/headless verification is left untouched. Metadata retain their narrower
1 MiB per-file bound. Initial admission requires empty floors; once any floor
exists it cannot mint initial provenance again. Original loader cancellation and
the Release owner/floors are rechecked at immutable handoff. Later candidates
have no first-pin provenance and require complete retained Release floors.

Local Installation binding is root-controlled canonical bounded data, independently
reconstructed at startup, not a serialized Authorization. Recommended fields:
binding schema/version; invocation; both exact target identities/facts and metadata
commitment; descriptor/program/template digests; full rendered unit and plan digests;
generation and predecessor identity; original request commitment; account name and
numeric UID/GID; exact configured root/credential/clock paths; selected native profile.
Retain original file identities/access and journal-born inode provenance separately.
Never publish private bytes, key hashes, private paths or process captures in a
public receipt. A stored digest, journal phase, completion ACK or permission-like
boolean cannot recreate a fresh Release proof, live startup or domain permit.

For the first Source, propose one fixed `ardents-source` account/group and
`/etc/systemd/system/ardents-source.service`. For the bounded installed Node
extension, propose one fixed `ardents-node` account/group, five exact
`ardents-<invocation>.service` units and at most one active Node invocation on a
host/provider allowance. All five unit observations must exclude a second active
invocation before start. Retain one canonical Hosting root for that allowance
across role/generation changes; never mint a new allowance by selecting another
account, unit or path. Source owns no Hosting allowance. This proposed installed
restriction does not change existing Hosting policy or prove host isolation.
Resolve actual numeric UID/GID once and bind
them; no dynamic user, supplementary groups, reused foreign account or password
login. Root performs only admitted installation mutations and manager control;
it acquires no Network/Source/Admission/Service signing right. Runtime is non-root,
with no ambient/bounding capabilities. Source listens on the explicitly bound
unprivileged literal-IP TCP port; no privileged-port helper or socket activation.

The unit is `Type=exec`, `Restart=no`, `KillMode=control-group`, working directory
`/`, umask 0077, no Exec pre/post/condition/reload/stop hooks, no drop-ins, no shell,
no command override or inherited listener. Root renders exactly one absolute
generation executable and closed argv: `source start-installed` plus its
absolute binding reference; role dispatch comes from the binding, never a first
available duty. Canonical `ardents-node` implements this bounded entrypoint with independent Source Startup, followed by actual State and Guard composition; installed execution and qualification remain separately gated. Bind exact manager
unique owner/version/boot/start identity, fragment, loaded configuration, InvocationID,
MainPID, proc inode/start clock, executable inode/bytes, complete argv, real/effective/
saved IDs/groups, original cgroup inode/path and sole-process membership. Two
fresh complete manager rounds surround independent file/process reobservation.
Root's original control connection retains only the fixed Source unit through
`RefUnit`, which loads its configuration without starting it and prevents
collection while it is stopped. Reference acquisition supplies no loaded-policy,
Release or Start authority. Closing that original connection returns its server
reference. Non-root Startup and read-only preflight retain no such loading right.
An admitted manager observer waiting behind original I/O retains its caller's
cancellation when physical Close denies that connection. This local close
refusal is not an additional manager failure; earlier terminal and actual
physical errors remain retained through the original join.
Manager restart/disconnect, extra child, changed namespace/path/argv/account/unit
or cgroup fails closed; reconnection cannot recreate the original custody.

Use the current Source/role process envelope exactly: CPU quota 100000/100000,
memory.high 805306368, memory.max 1073741824, pids.max 256, soft/hard NOFILE 1024,
GOMAXPROCS=1, GOMEMLIMIT=536870912 and existing PSI/event/one-second freshness
rules. No new capacity profile is selected. Render and check accounting, strict
system protection, no-new-privileges, private tmp, protected kernel/control groups,
zero core/capabilities and exact address families. Installation verifies loaded
policy; processresource independently checks actual kernel observations. Read-only
cgroup/proc access needed by Guard must be proven on the actual non-root stand;
a missing readable metric denies rather than grants an unprotected exception.
Native candidate is Ubuntu 24.04/systemd 255/cgroup-v2/native ext4; this proposed
installed profile needs its own evidence, not the development driver's receipt.

| Input class | Proposed custody / write boundary / reobservation |
|---|---|
| Immutable executable, template, descriptor, rendered plan/unit and binding | Root-controlled direct ancestors, no symlinks, original same-inode bytes/access and closed inventory; runtime read/execute only. No mutable root may contain or parent these files or Release history. Root writer lease stays separate from read-only startup custody |
| State, participation, spend/Hosting/Store/Registry/issuer roots | Exact caller-admitted canonical paths and born provenance, independent within the invocation, 0700 for their dedicated owner. The same provider allowance retains its one Hosting root across Node roles/processes, as required by Hosting; role changes never initialize another period or duplicate capacity. Root may create admitted empty directories after journal birth; each semantic owner initializes/verifies its own contents. Runtime writable paths are exactly the role's inventory. No generic writable ancestor, foreign live lease or old/new shared root; retained native reopens preserve all floors |
| Source/Node runtime private TLS key | Explicit operator-provided purpose-specific key, not the offline signing directory. Source retains runtime-owned 0600 delivery. The bounded Forwarding profile below selects root-owned, exclusive Node-group-readable 0640 delivery under root-controlled non-writable ancestors; its independent Startup verifies the fixed exclusive account/group. The Node physical reader checks this exact system UID/GID file layout, no hard links, root-controlled direct ancestors and original metadata around reads, without granting Startup or duty authority. Ordinary runtime-owned private-file admission remains separate. No symlink/hardlink, public export, argv/env secret, generic signer or root-owned-0600 handoff to non-root. Original file/ancestor/access/bytes and matching public key remain pinned through join. Credential replacement requires explicit fresh invocation after old join |
| Public certificate, CA and pins | Exact immutable original files/plan bytes read-only to runtime, independently approved trust. Source TLS verifies CA/EKU/name/pins with State's clock at handshake; Node TLS matches current member/key/purpose. File commitment alone grants neither right |
| Live clock file | Bind canonical path, explicit writer identity and original parent/access policy, readable to runtime, not runtime-writable. State retains its existing regular-file mtime/time/floor semantics; Installation must not freeze a timestamp or invent content authentication. The authorized writer may update evidence under the bound path; each current file must meet the admitted writer/access/type policy. Replaced ancestor/path declaration or changed trust policy denies; fresh evidence is reverified by State |
| Release floors and installation journals | Root-only writable, exact separate native leases and schemas; runtime only reads admitted selection/binding, not private intent. Release remains sole floor owner. Unknown partial storage is retained, not reformatted or treated as a new install |

For Source runtime inputs the exact first engineering predecessor is limited to
plan/certificate/private-key/client-CA custody. It must not silently freeze the
clock file inode or change Network's live clock-admission semantics.

The bounded Source initial stopped transaction implements fresh exclusive
runtime-credential delivery, actual UID/access and same-inode transfer without
copying offline authority. The five Node accounts' delivery remains separate.
This is an Installation input-custody effect, not a new PKI or Custody authority;
its presence alone qualifies no installed invocation.

### Proposed lifecycle and effect admission

#### Forwarding initial lifetime and explicit same-generation restart

The closed installed Forwarding plan opens genuine retained State without a
Source server or automatic refresh worker. Its composition supervises the
original Node and Startup; Node rechecks State `CurrentRuntime`, signed duty and
time at effects and in its own original supervisor. `State.Wait` observes
configured background work and reports its absence in this configuration; that
absence is not an operational failure of the accepted State. Command composition
keeps State owned until Node/Route and purpose/resource users have joined, then
closes it last. No State authority check, lifetime bound or acquisition fallback
is supplied by omitting a wait for an unconfigured worker.

The original Node separately retains its first failed admission or refill as
`WorkRefusal`: fixed operation/stage, duration and original local error custody.
This detached observation neither denies the invocation nor changes its first
lifetime failure, terminal result, spend or reservations. It supplies no success,
currentness or physical-completion fact. Canonical installed composition emits
at most one bounded `work-refused` diagnostic per invocation, using fixed
categories and numeric observations without private error text or identities.
Diagnostic output follows the same finite original output ownership as other
events; it is never admission, retirement or restart evidence.

Forwarding native process observation has separate root and actual non-root
self openers. Its fixed policy admits only the selected generation's Node
executable, exact `forwarding start-installed BINDING` argv, shared Node
UID/GID with no foreign supplementary group, and the Forwarding unit cgroup.
It retains original proc/start-clock, executable and namespace identities.
Copies share permanent observation refusal and joined descriptor retirement;
the root observer's own pidfd permits at most one original SIGTERM after fresh
checks. Source's process/signal handle is never exposed through this type.
These physical observations supply no Release, manager, Startup, Guard, domain
join or restart permission; Installation must establish those independently.

Its distinct native cgroup observer selects only the Forwarding service and
retains its own original directory and `cgroup.events` object. Root's
post-attempt partial acquisition, non-root actual-self observation and explicit
stop's archived-native-identity check are separate operations. Observation
requires the same sole PID and no child scopes. Original kernel join remains
observable after caller denial; a missing pathname, new empty directory or
process exit cannot supply it. Source shares only the private physical reading
mechanism, with its own fixed opener and live resources. Installation still
owns effect admission, domain completion and release after actual native join.

Forwarding manager observation has distinct native root and non-root owners.
The runtime connection exposes only fixed-unit read operations and checks the
other four Node service/socket slots and Forwarding socket before and after
each complete property observation. Root control independently retains the
fixed Forwarding unit through `RefUnit`; it may attempt one fixed Start and one
cleanup Stop, each mode `fail`. Its own native job-reply grammar grants no right
to a Source or read-only connection. Neither reference, job nor detached manager
identity grants Installation admission, process identity or original join.
Original manager declaration/unique owner is rechecked around observations;
cancellation and Close interrupt and join original physical users without
reconnecting. Two complete running-property rounds and native reobservation
remain the Startup owner's separate responsibility.

A neighboring-slot absence query interrupted solely by its original context
returns that failed context observation, not a synthetic changed-unit identity.
Only exact original cancellation/deadline leaves and joins containing solely
those leaves qualify; physical wrappers, foreign errors and independent failures
remain failures. No interrupted query supplies a successful absence fact, and a
deadline remains distinct from ordinary cancellation.

The distinct Forwarding `Startup` owns independent non-root files, actual
Forwarding executable/argv/principal/InvocationID, original kernel scope,
namespace and visible manager/boot observations. Two complete running manager
rounds surround reobservation of those originals and the fixed unit-search
namespace. Root's completed initial reader, authorization or connection cannot
construct it. It connects through the original root-owned, Node-group-readable
0750 completion directory and the shared fixed 160-byte physical exchange;
matching original invocation, generation and binding plus fresh reobservation
before and after root ACK precede publication of a usable Startup. ACK alone
grants no State, role, process-resource, Admission or Hosting authority.

The finite initial-generation invocation inventory uses `forwarding-startup`
for attempt zero and `forwarding-invocation-%08d` for attempts 1..63. Its separate
root journal names are `<installation_root>.forwarding-first-start-journal`
and `<installation_root>.forwarding-invocation-%08d-journal`. Every original
attempt remains; gaps, duplicates, foreign/noncanonical names or exhaustion
refuse. Directory discovery chooses only an original root-peer exchange and
does not prove Start, archive, previous retirement or replayable completion.
Root independently authenticates its full start/retirement chain before each
effect. Runtime acquires its own read-only directory identity; it never reads
private guard/journal contents or borrows the root writer lease.

The private root start journal uses `ardents-forwarding-start-journal-v1` and
ordered compact JSON/LF fields `schema`, `sequence`, `phase`, `path`, `device`,
`inode`, `sha256`, `mode`, `uid`, `gid`. Sequence begins at zero; the opening
phase is `first-start` for attempt zero and `invocation` otherwise, committing
the exact binding digest. The physical writer permits at most 256 records,
32 KiB per record and 128 KiB total. Its closed phase vocabulary distinguishes
directory/file/socket intent, birth and readiness; `start-attempt`,
`invocation-observed`, `completion-received`, `archived`, `ack-attempt` and
`ack-written`. These bounded physical records are not a complete-chain verdict.
The root producer and independent retained reader must additionally enforce
the actual transition order and native/domain evidence before any authority.

Each journal is an exclusively created root-only 0600 native ext4 file under
original retained ancestors, with its own nonblocking exclusive flock. The
completion directory is born root-only 0700 and promoted on the same inode to
root:Node 0750 after durable birth evidence. Its only private record names are
`start-guard.json`, `start-socket.json`, `start-archived.json` and
`forwarding-start-pending`; the exchange socket is `start-completion.socket`.
File intent precedes exclusive empty creation, synced birth precedes bytes and
the synced exact file precedes readiness. Every append reobserves original
journal bytes, resources and ancestors. A failure, including after actual sync,
permanently denies later append; Close joins all owned files and ancestors before
releasing the journal lease and retains original/physical failures. Partial
prefixes survive Close and refuse fresh creation. This physical storage grants
no Start, ACK, runtime, retirement, recovery or automatic retry permission.

Root `StartInitial` consumes its own independently reopened completed initial
installation and fresh coherent Forwarding Release pair. It checks path
independence, the actual mapped controller program, original root kernel/boot
identity, manager connection and all five unit-search slots. Root's
mount/PID/cgroup/UTS/user namespaces must match the
visible original manager; a manager bus borrowed through a different root
namespace refuses. Non-root PrivateTmp observation remains separately owned.
Two stopped-policy
and native-quiescence rounds precede the sole fixed Start attempt. Every future
invocation journal and completion directory must remain absent before the first
attempt; unrecognized contents of the installation root refuse too. The original
initial journal's exclusive lease survives the whole root operation and all its
native joins. No original creator or non-root observer is borrowed.

The private root intent `ardents-forwarding-start-intent-v1` binds the exact
binding, completed initial-history and freshly accepted targets digests, numeric
UID/GID and attempt. `ardents-forwarding-start-socket-v1` binds guard digest and
actual socket device/inode/group. The exact original peer PID/UID/GID, process,
cgroup and two complete running-policy rounds surround the 160-byte completion
request. `ardents-forwarding-start-archive-v1` binds that same guard/binding,
generation, PID/InvocationID, proc and cgroup identities, manager and original
main-process realtime/monotonic start clocks. The archive is synced before ACK.
The completed transcript has exactly 25 canonical ordered records, including the
pending file's three birth records. Full grammar, native identity and running
readback precede joined root publication; an isolated archive or ACK is insufficient.

A complete ACK transfers startup even if a later append, cancellation or Close
fails. That failure preserves the pending marker and does not authorize rollback,
another Start or replay. Without full ACK, the root owner retains its original
scope through actual joined cleanup, using only an independently observed original
process for cleanup Stop. Unknown original scope/join remains unavailable while
its custody is retained; the command reports completion unavailable and continues
waiting rather than converting a deadline or absent process into completion.
Only successful root/native observer joins and writer closes allow the final
pending unlink. The explicit stop/domain retirement and retained restart reader
remain separate obligations; this mechanism does not establish installed qualification.

`ReadStarted` independently opens a fully published first-start transcript under
its own initial and start-journal leases. The completion directory has the exact
root:Node 0750 policy; its group does not widen the ordinary root-owned ancestor
checks. The reader acquires its own parent/directory/file descriptors and an
`O_PATH` socket inode descriptor without connecting to the stale endpoint. It
matches all 25 records, canonical intent/socket/archive bytes, exact binding and
initial-history commitments, original recorded inodes/access and pending absence.
The intentionally unlinked pending file is historical evidence, never a required
current file or a recreated marker. A held original writer, partial prefix,
substitution, missing archive or uncertain publication refuses; no file is created,
rewritten, repaired or adopted. Copies share denial and original users; Close
joins observations before returning the start lease, then the initial lease last.

This reader matches the original retained Release directory identity through its
own live verifier, without requiring unexpired artifact authorization: expiry must
not prevent safe shutdown. Its archive fields remain historical observations.
Stop still needs fresh actual original process/manager/scope checks and independently
proved domain/native completion; restart additionally needs fresh Release, time,
credentials, State and protection admission. Read-only public verification grants
none of those rights and supplies no qualified installed outcome.

The Forwarding start archive's manager field retains the full broker-instance
identifier and unique manager owner, freshly rechecked by the original root
attempt. The loaded observation's unique owner alone cannot replace that pair.
Stop and same-generation restart compare the independently observed full identity;
a shortened historical value cannot be adopted or repaired into permission.

Request admission checks every finite start and retirement completion socket
pathname against Linux's 107-byte payload limit, before native installation
effects. A first-start socket that fits does not make longer retained restart
or retirement paths usable. Other declared paths retain their existing bounds.

Explicit Forwarding retirement uses a separate finite
`forwarding-retirement-%08d` directory under the installation root, independently
of the exact four-file first-start directory. Its own root-only exclusive journal
is created and synced before directory births; an existing or uncertain prefix
refuses adoption or a fresh retry. The directory is born 0700 and promoted on the
same inode to root:Node 0750. Its only owned leaves are `retirement-pending`,
`retirement-result.json` and `stop-completion.socket`; file intent, durable empty
birth and exact ready bytes remain separate records. The physical writer uses
`ardents-forwarding-retirement-journal-v1`, the same bounded native record fields,
and explicit stop/birth/interrupt/domain-joined/native-joined/native-closed/retired
phases. These physical records grant no interruption, domain/native join or
retirement authority. The root stop producer must independently prove those
transitions before publication and retain the blocker through all physical closes.
The complete retirement grammar has exactly 18 canonical ordered records,
including its directory and pending-file births. The root-private pending bytes
are `ardents-forwarding-retirement-pending-v1` plus LF, the original start-archive
digest and LF. The result schema `ardents-forwarding-retirement-v1` binds attempt,
generation/binding/start archive, PID/InvocationID/manager, fresh challenge and
completion digest, matching original main start/exit clocks and clean native
exit code/status. A complete-looking result or `native-closed` record found in
a shorter or reordered prefix supplies no retirement evidence.

The original non-root Startup may report once only after its actual Close has
joined, with the command's complete joined Node/State/Guard/key/output result.
It independently reopens the new retirement directory under the original retained
installation-root identity, checks actual read-only native mounts and root peer,
and answers a fresh nonzero 32-byte challenge. The finite reply is exactly
`ardents-forwarding-domain-joined-v1` plus LF, the original 160-byte startup frame
and that challenge. Source frames and another invocation cannot substitute.
Only exact cancellation leaves and multi-error joins of those leaves classify as
ordinary cancellation; a physical single-error wrapper, deadline or late failure
is retained. Node/State/Startup close, plan-copy erasure and all output users join
before reporting. A clean process exit is reserved for the exact private explicit
stop marker after that one-use exchange. Root still needs the original peer,
native scope/exit/clocks, all observer closes and durable retirement proof;
the message, process exit or marker alone proves none of them. Ordinary retirement
with no root endpoint creates no restart evidence.

The distinct root `forwarding stop HISTORY BINDING` consumer first reopens
published initial and complete invocation/retirement history, verifies the controller's actual mapped program,
and independently retains the archived original process inode/pidfd, cgroup
inode/events, manager and namespaces. It never calls a Source stop or borrows
the start writer. A fresh challenge and exact original Node peer precede the
joined-domain report; actual original kernel join and two matching completed
manager observations follow. Main process ID and both start clocks must match
the archive, both exit clocks must be at least their starts, and native exit
must be CLD_EXITED with status zero. Inactive state alone is insufficient.
The root consumer closes native observers before recording `native-closed`,
then independently reopens synced result/journal/directory/socket files while
the blocker remains. Every observer and writer closes before its final unlink.
The independent committed-history reader instead requires blocker absence and
its own exclusive retirement-journal lease. Neither reader lends live objects
to a successor. Missing or incomplete predecessors, duplicate InvocationIDs,
unknown owned prefixes and mismatched journal/directory inventories refuse.
These mechanisms do not establish installed stop/restart qualification or
bootstrap acceptance.

The explicit `forwarding restart DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`
consumer requires the latest committed retirement in that same complete chain,
then repeats fresh coherent Release, current stopped/native protection and
independent non-root startup admission for the same immutable generation. It
retains all old native file/journal observations under the initial lease through
new start handoff and physical root Close. The new startup owns no old runtime
handle; State/participation/spend/Hosting use retained existing-only openings.
Attempt zero preserves `ardents-forwarding-start-intent-v1` exactly. Attempts
1..63 use `ardents-forwarding-invocation-intent-v1` with the same ordered fields
plus final `predecessor_sha256`, the exact immediately preceding retirement
result commitment. Missing or substituted links refuse; exhaustion never
recycles paths or resets a semantic root. Root selects the next unused attempt
only from its independently read chain. A late/failed start preserves its own
blocking prefix and original join obligation. Restart is explicit, with no
replacement, crash continuation or automatic boot/retry policy.

Canonical Control's separate `forwarding prepare-hosting` initializes the exact
empty Hosting directory proved by the completed initial birth. It requires root,
the original fixed Node account and actual stopped manager/namespace/cgroup
observations, with no prior invocation. Its exclusive root-only Hosting journal,
intent, completion and pending blocker are separate from the Node-writable budget.
Hosting validates the original provider period and real counters, exclusively
creates its files and returns original native births. Installation matches each
descriptor before Node UID/GID delivery; all original preparation users close
before pending publication. First start and restart independently retain the
complete ordered preparation and original `budget.pin`/`budget.lock` identities,
with no borrowed live budget. A partial prefix, unknown inode, missing commitment
or repeated initialization refuses; deleting/recreating the original directory
or selecting another allowance path is never a retry. This supplies no installed
execution or reservation permission.

Canonical Control's separate `forwarding prepare-state PLAN BUNDLE SIGNED_PROFILE`
is finite offline preparation under the actual `ardents-node` system UID/GID,
with no supplementary groups other than that primary group. It retains original
root-controlled public plan, signed bundle/profile and ancestor descriptors,
and only an existing empty Node-owned 0700 State directory. State alone accepts
the Epoch/materialization and closed signed profile, then supplies a checked
member observation for the declared Node. The original State Close and all
input checks/closes precede output. Existing or partial roots refuse rather
than initialize again, including after a failed profile transition. No private
credential, participation, spend or Hosting root opens; no Source serving or
acquisition configuration, Startup, Guard or runtime authority is supplied.
The bundle uses Source's opaque framing codec only; that grants no Source
installation or serving right. Installed runtime independently reopens and
reverifies State, time, membership and the exact role at its own effects.

Startup Stop denies new checks and cancels its original context before any
join. Close interrupts its own manager, joins admitted observations, then
closes completion, file/key, scope, namespace/kernel and process observations,
retaining first and physical failures through copies. Detached runtime input
copies are handed out only after original checks and must remain private through
their actual users' joined cleanup. Complete domain retirement and the real
root start/archive/ACK producer remain separate obligations of their owners.

The bounded first installed Forwarding contract admits only `forwarding`, not
another Node duty or Source. Its artifact pair uses the exact Node program target
and `ardents/linux-amd64/infrastructure/forwarding`; its descriptor and initial
inventory use `infrastructure-forwarding.json` and `ardents-forwarding.service`.
Source descriptor, initial provenance, Startup and retirement records cannot be
reinterpreted as Forwarding authority. Enrollment may share its finite immutable
inventory mechanism; each Installation consumer still checks its exact invocation.

Canonical Control has distinct Forwarding initial key, public release-plan and
signing producers, documented in the [command reference](../reference/commands.md).
Five fresh offline keys and the reviewed public claims remain under original
root-controlled native custody. The closed version-one purpose checks the actual
static Node program and fixed template, signs only Node/Forwarding targets and
requires two coherent Release decisions before publishing metadata. Shared
physical codecs/output mechanisms confer no Source signing right. Public claims
and diagnostic self-verification supply neither independent build evidence nor
the initial pin or separately accepted installed bootstrap.

The separate controlled build and public-plan consumers in the
[qualification operation](../operations/qualification-release.md#forwarding-infrastructure-artifact-component)
replace hand-authored build claims with two actual Node/Control build records,
their exact compiled dependency/SBOM commitments and separately pinned prestart
evidence. Both executions remain under project control. Their Forwarding-only
initial formats neither consume Source authority nor grant a first pin,
independent builders, installation, startup or bootstrap acceptance. The existing
canonical signer still independently validates the exact resources and obtains
both fresh Release decisions before metadata output.

The separate `ardents-forwarding-installation-v1` request is compact canonical
JSON plus LF, at most 16 KiB, with exact ordered fields `schema`, `invocation`,
`profile`, `installation_root`, `state_root`, `participation_root`, `spend_root`,
`hosting_root`, `credential_directory`, `plan`, `certificate`, `private_key`,
`clock`. Invocation/profile are exactly `forwarding` and
`ardents-forwarding-process-v1`. Paths are canonical absolute, pairwise disjoint
including ancestors. The plan/certificate/private-key are original delivery
inputs, not runtime-writable files. Runtime destinations and clock must remain
visible under PrivateTmp/ProtectHome, excluding `/tmp`, `/var/tmp`, `/home`,
`/root` and `/run/user`. The separate fixed template uses `ardents-node`, exact
`forwarding start-installed` argv and exactly State/participation/spend/Hosting
writable roots; it keeps the unchanged fixed protection/resource envelope.
Canonical Control's flag-based request/template producers prepare bytes only;
they neither open declared inputs nor create these future destinations. Actual
Installation must independently admit births and runtime input matching.
The initial journal/pending and all 64 finite invocation journal siblings are
reserved independently of their existence, along with the 64 separate
`<installation_root>.forwarding-retirement-%08d-journal` siblings.
Request/delivery declarations,
distribution, Release history and live clock cannot occupy or contain them.
Initial preflight and creation require that entire future journal namespace to
be absent; an unknown future-attempt prefix is preserved and refuses initial
effects, even if the installation directory has not yet been born.

The installed plan has its own `ardents-forwarding-runtime-v1` schema; it does
not reinterpret Source serving or the development command's unversioned plan.
Its canonical compact JSON plus LF is at most 16 KiB, ordered `schema`, `network`,
`node_id`, `process_profile`, `participation_root`, `spend_root`, `hosting_root`,
`certificate`, `private_key`, `work`, `termination`, `retirement_wait_ms`.
`network` contains ordered `root`, `network_id`, `authorities`, `threshold`,
`profile_authority`, `clock_observation_file`; IDs/keys use exact 32-byte arrays,
1..16 distinct State authority keys and a valid threshold. Work/termination
retain Hosting's separate `tx`/`rx` declarations; Node still checks complete
Forward allowance before spend. The 1..5000 ms retirement wait bounds a notice,
not original join. Paths must match the original request, with exact delivered
credential names. Canonical Control prepares this declaration from the original
protected request and explicit independently approved public pins/coverage;
neither private keys nor State/participation/spend/Hosting roots are opened.
Signed currentness, actual credential match and protection remain runtime checks.

Root-only `forwarding.ReadInputs` independently retains original request, plan,
certificate, private-key and ancestor descriptors before native effects. Paths
are bounded to 4096 bytes and 128 components. Existing delivery files must be
direct root-owned regular files on native ext4, without hard links or writable
group/other access; request, plan and key use 0600. Original bytes, inode,
ownership, access, timestamps and named ancestor links are reobserved around
reads. The certificate/private-key pair must match Ed25519. Its declared live
clock retains original root-controlled ancestors and a direct root-owned 0644
file, while legitimate updates or inode replacement remain available to State's
time owner. Check denies after original failure or closure; Close denies new
uses synchronously, joins original readers, retains first and physical cleanup
failures, then erases private bytes and releases descriptors. Rejected partial
acquisitions complete that same cleanup before returning no usable handle.
This input lifetime initializes no future root, account, Release verifier or
manager, and grants no independent installed startup authority.

Canonical `ardents-node forwarding inspect-inputs REQUEST` separately composes
that input lifetime and a fresh root-only fixed manager connection. It observes
all five Node service/socket slots as both unloaded and uninstalled, checks the
direct manager search paths for exact unit/drop-in overrides, and refuses an
existing `ardents-node` account or future root/journal/pending resource. Every
destination parent must already exist and retain its root-controlled original
identity. Original input/account-database and manager observations repeat before
the read-only result; private inputs and manager users join before public output.
It opens no Release history and grants no install, Start or runtime permission.
The actual creator must independently admit and reobserve all required effects.
The native manager adapter independently retains its own connection and exact
Version/UnitPath/unique-manager identity. Its separate fixed one-reload/loaded
Forwarding observation has no arbitrary unit, Source lifetime or Start operation;
successful native observations never replace Installation's policy comparison.

`forwarding install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK
REQUEST` has a distinct initial creator. It requires the actual first executable
and Forwarding inventory, private initial Enrollment provenance and two coherent
fresh Release authorizations. The original input and verifier lifetimes are
consumed: all native users, account/helper custody, inputs, journal and verifier
must close successfully before final pending publication. It never starts a unit
or initializes State, participation, spend or Hosting history. Every native
destination parent already exists, is root-controlled and traversable by the
future principal, and has writable native ext4 storage. Ubuntu24.04 and the
original systemd255 manager are required. Unknown resources are not adopted.

The shared fixed `ardents-node` account is created through one fixed bounded
`useradd`, with original host database/helper/configuration preimages, closed
local-files/systemd NSS policy and fresh name/numeric absence checks. Its helper
is PID1 of a separate PID namespace with parent-death SIGKILL; original Run joins
namespace descendants before any created-account result. Foreign database bytes,
auxiliary backups, hooks, supplementary groups and unlocked credentials refuse.
Fresh account rows and NSS are independently reobserved. This is the one Node
principal across the prospective five duties; Source retains its separate account.

The selected `ardents-forwarding-binding-v1` is canonical compact JSON plus LF,
at most 32 KiB, ordered `schema`, `invocation`, `generation`, `program_target`,
`generation_target`, `program_sha256`, `template_sha256`, `inventory_sha256`,
`targets_sha256`, `request_sha256`, `plan_sha256`, `unit_sha256`,
`certificate_sha256`, `account`, `uid`, `gid`, `history`, `request`. It admits only
the initial Forwarding identity, fixed shared Node account and equal system
UID/GID in 100..999. Every digest is lowercase SHA-256. Request, fixed unit and
original history declarations must agree; parsing supplies no original proof.
The twelve-file immutable generation comprises the eight frozen artifact files,
`forwarding-request.json`, `forwarding-plan.json`, `rendered-forwarding.service`
and `binding.json`. Program is root-owned 0555, other generation leaves 0444 and
the sealed generation directory 0555. Selected binding and fixed service are
root-owned 0644. The separate credential directory contains root-owned 0444
certificate and root-owned, Node-group-readable 0640 private key. Four independent
semantic directories are born empty, Node-owned 0700; this grants no capacity or
permission and does not initialize a second Hosting allowance.

The root-only exclusive `<installation_root>.forwarding-initial-journal` uses
closed LF-delimited JSON records, at most 4 MiB total and 32 KiB per record,
ordered `sequence`, `phase`, `path`, `device`, `inode`, `sha256`, `mode`, `uid`,
`gid`. Its first phase is exactly `forwarding-initial-v1`.
The `admitted` record binds the exact original Release history path, native
directory device/inode and root-owned 0700 access to the binding digest. A copied
history or equal floors cannot replace that original history declaration.
Intent precedes each
account/directory/file effect, original birth identity precedes byte/access
promotion, and original native readback precedes seal/selection/reload/stopped
completion. The journal and `<installation_root>.forwarding-initial-pending`
remain root-owned 0600. Pending bytes are
`ardents-forwarding-initial-pending-v1\n<BINDING_SHA256>\n` and cannot grant Start.
Partial or uncertain effects retain their original prefix and require repair;
ordinary initial retry, adoption and reset refuse. Final unlink is the last
effect after successful original joins and fresh Release checks; no required
operation follows it. Lost unlink restores denial. Neither complete-looking
records nor absence of a process substitute for separately admitted startup,
original completion or power-loss qualification. Native transaction and installed
runtime acceptance still require their selected source-matched evidence.

Before returning its writer custody, the creator separately reopens the complete
119-record transcript and every recorded current native resource. The closed
reader consumes all phases in order, with exact canonical bytes and binding/file
commitments, and matches 27 distinct original device/inode identities and final
access. Born files are root-owned 0600 before final promotion; born directories
are root-owned 0700. The journal, pending marker and original Release directory
are distinct resources. Historical host database records are checked as helper
provenance, not retained immutable database ownership. This creator readback runs
under the original still-blocking pending marker and original ancestor/writer
custody; failure after `completed` still preserves denial. It is no standalone
completed-installation opener or Startup. A later start must independently acquire
its own completed journal lease, ancestors, current principal, fresh Release and
native invocation observations without borrowing the creator.

`forwarding.ReadCompleted` separately reopens only a complete initial installation
as a root-only, noncreating native observation. It requires pending absence and
its own nonblocking exclusive journal lease before reading the full transcript.
It checks every original resource identity except the intentionally unlinked
pending file, the twelve sealed files and two credential leaves, current locked
fixed account/NSS, the selected/rendered unit, matching Ed25519 credentials,
original ancestors and live clock writer/access policy. Semantic directory
identity/access is retained without reading or initializing another owner's
history. Unknown, missing, partial or substituted resources refuse; there is no
adoption, repair, recovery or original-writer handoff.

Canonical `forwarding inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT
NETWORK BINDING` consumes this separate observation, checks path independence
before opening the existing Release root, obtains two fresh coherent Forwarding
authorizations and matches their program/descriptor/template to the installed
generation. Fresh trusted metadata may advance Release floors without rewriting
historical sealed initial metadata or granting a replacement generation. All
native observations and Release users join before the public verification result.
This operation neither observes a running manager invocation nor grants Start,
retirement, State or runtime authority; those independent gates remain required.

The independent non-root selected-file reader acquires its own original
descriptors under the exact bound Node UID/GID. It reads the same closed
twelve-file byte grammar and two matching credential files, selected binding,
fixed unit and clock custody; it cannot open the root journal or Release history.
It reobserves root-owned immutable directories on read-only native ext4 mounts
and exactly the four Node-owned writable native directories. Both retained and
freshly named directory descriptors are measured: a new bind mount of the same
inode cannot borrow the old descriptor's read-only flags. It opens no semantic
history contents. Cancellation, lost bytes/descriptors, access or mount mismatch
deny permanently through original physical Close. These file observations grant
no manager/process, root completion, Guard, Startup or domain admission.

Original Check/Close copies share denial and first/physical failures. Close denies
new checks, joins actual reads, closes other descriptors and only then returns
the original journal lease. A late restored file cannot revive the failed handle.

The following finite matrix specifies effects and original completion, not a
qualification result. Each causal family needs an independent denied-effect
oracle and retained first/cleanup failures. The execution ledger binds concrete
cases, exact source/artifacts/inputs and observed results before installed use.

| Boundary / owner | Positive operation and effects | Causal refusal / independently observed invariant | Original completion |
|---|---|---|---|
| Enrollment and Release | Independent first pin and actual executable; two fresh coherent Node/Forwarding target decisions | Wrong pin/program/descriptor/role/target/resource, unknown or partial inventory, mixed metadata or retained floor conflict: no native installation effect; committed Release floors never decrease | Original immutable input reads and Release observations join; copied receipt grants nothing |
| Initial Installation | Exact finite request/template; original fixed account, empty independent semantic directories, immutable generation, selection and stopped unit | Existing unknown account/root/unit, alternate Hosting allowance, partial or uncertain birth/write/seal/reload: retain provenance and repair-required; no adoption, reset or automatic continuation | Original input/native/journal users join before returning; uncertainty retains blocking evidence |
| Root start and independent non-root Startup | Fresh Release pair; exact program/argv/IDs/groups/unit/manager/InvocationID/proc/cgroup/input inventory; all five Node unit slots checked | Substituted inputs/ancestors/credentials/account/manager/invocation or another active slot: no State producer, Receiving spend, Hosting reservation or Route listener | Root barrier and independent startup own their physical observations; no borrowed Source Startup |
| Network, Node and process protection | Genuine signed current duty, separate local producer, original Guard and loaded purpose credential | Wrong member/key/profile, signed conflict/successor/expiry, unavailable time/metrics, changed limits or real pressure: synchronous denial before dependent effects | Held observation/cancellation/late result keeps original producer and native custody until join |
| Admission, Hosting and Route | Real authenticated TCP/TLS initial Forward admission and parent refill; full class coverage reserved before spend | Undercoverage or genuine capacity/authority loss before spend leaves exact spend history unchanged; post-spend failure never refunds; deferred QUIC refuses before effects without fallback | Real held parent/child/receiver work keeps the physical reservation until Route join; Hosting independently proves actual return |
| Explicit stop | Deny synchronously, interrupt Receiver/Route, join original users, then purpose roots/Hosting/producer/Guard/key, with State last | Held borrower/native Close, cancellation and late failure retain resources and every original cleanup cause; unavailable completion notice, PID absence or inactive unit is insufficient | Independently proved original domain result and native scope join precede retirement commitment |
| Explicit restart | Reopen retained State/participation/spend/Hosting/Release histories; fresh independently admitted invocation of the same immutable generation; real contact and final stop | Concurrent restart while predecessor held, unknown join, missing/damaged history or stale Release/time/credential/protection: refuse without initialization or transferred live handles | Preserve account/keys and all histories; new invocation has its own producer/native/startup custody |

Compatibility with the next lifecycle is deliberately bounded. Resource/program
replacement requires a strictly forward generation, fresh coherent Release pair
and proven predecessor retirement; the first-lifetime path does not implement or
accept it. The one-use Hosting preparation, original allowance directory and
pin/lock births remain retained compatibility obligations; a new binding or
role cannot initialize another allowance or discard the initial birth proof.
Any future replacement must explicitly preserve that provenance rather than
treat the first-lifetime binding check as a migration mechanism.
Interruption before complete stopped installation or during uncertain
start/retirement retains its original journal/births and refuses ordinary retry
as a fresh installation or restart. Crash recovery requires a separately admitted
original-prefix/completion contract; first-lifetime operations supply no continuation, replay
of Start/ACK, history repair or capacity reset. Complete same-generation explicit
stop/restart must preserve enough immutable generation and original retirement
evidence for that later contract, without preauthorizing it.

#### Explicit Forwarding generation transitions

The manual transition has separate stopped replacement, explicit start and
original-prefix recovery admissions under ADR-0123. Replacement consumes a
fresh coherent Node/Forwarding Release pair and the complete latest original
retirement before its first native effect. Enrollment remains initial provenance.
One `ardents-node` account, all five fixed Node slots, the original credentials,
State/participation/spend roots, Release root and provider-period Hosting root
remain exact. State, participation, Receiving spending and Hosting each supply
their own noncreating read-only inspection; Installation cannot parse their
private histories or borrow their runtime handles. Unknown original producers,
reservations or domain/native completion block replacement even after process
exit. Inspection supplies no accepting runtime, reservation, refund or root reset.

A resource-only forward generation changes actual finite `work` or
`termination` declarations in the immutable installed plan. It preserves the
program, signed protection template, Network pins/Node identity, credential and
semantic-root declarations. A descriptor/version-only or formatting change is
not a resource update. Changed program bytes still require fresh exact program
authorization. The descriptor release version must strictly increase; neither
case lowers metadata or security floors or creates capacity. Node and Hosting
retain class coverage, pressure, actual accounting and reservation policy.

Canonical Control has distinct initial and forward Release preparation/signing
purposes. Forward signing retains the original protected complete eight-file
predecessor, matches its five Root keys and three-signature roles, and obtains
genuine coherent Node/Forwarding Release decisions before signing the next pair.
It preserves original Root bytes and advances Targets/Snapshot/Timestamp versions
and exact version references by one. Its freshly born diagnostic history is
separate from the installation's retained trust and floors. Public build-plan
preparation supports the separate forward schema without JSON editing; it grants
no signature, initial pin, installed replacement or bootstrap acceptance.

The selected replacement is canonical compact JSON plus LF, at most32KiB:
`schema`, `configuration`, `initial_generation`, `predecessor_generation`, `predecessor_binding_sha256`,
`retirement_sha256`, `predecessor_attempt`. Its schema is
`ardents-forwarding-replacement-binding-v1`; configuration retains the existing
binding field order with distinct schema
`ardents-forwarding-replacement-configuration-v1`. Initial binding remains
`ardents-forwarding-binding-v1` and is never silently converted. The exact
initial generation, exact predecessor generation/binding and final retirement
bind one lineage. The explicit predecessor generation names its sealed directory;
no directory scan or nearest matching generation supplies a predecessor.
Every sealed generation and original start/retirement remains. Attempt numbers
are global, contiguous and bounded by64 across generations; replacement does
not recycle a pathname, deadline, token or allowance. Root history readers
verify each attempt under its own generation and predecessor, not under the
newest configuration. Non-root startup reads only its independently selected
files and receives a separately admitted root-peer exchange for that selection.

The replacement journal is the root-only0600 exclusive sibling
`<installation_root>.forwarding-replacement-<generation>-journal`. Its closed
record fields are the existing `initialRecord` fields. Ordered phases are
`forwarding-replacement-v1`, `replacement-admitted`, `native-context`,
`predecessor-binding`, `predecessor-unit`, `predecessor-retired`; original pending
`file-intent/born/ready`; candidate `directory-intent/born/ready`; the twelve
lexically ordered generation files' `file-intent/born/ready`; `seal-intent`,
`sealed`; `selection-intent`, `selection-written`; `unit-intent`, `unit-written`;
`reload-intent`, `replacement-verified`. Each native record binds the original
device/inode/access; hashes bind exact preimages, candidate bytes and the whole
selection commitment. Selected binding and fixed unit are separate same-inode
recorded publications. The pending file
`<installation_root>/forwarding-replacement-pending` contains
`ardents-forwarding-replacement-pending-v1`, LF, selection SHA256, LF. The
original writer retains all independent predecessors/inspections and rechecks
fresh authority/native policy around effects. Its own fixed manager owner may
reload once and cannot Start, Stop, Enable, select another unit or reconnect.
All required readbacks, resyncs and original physical closes precede final
blocker unlink. Partial or uncertain results remain blocking; no rollback runs.

New replacement writers use `forwarding-replacement-v2`, exactly62 records.
The complete56-record v1 transcript remains readable without conversion. V2
adds two root-only0600 sibling files, `<journal>.native-before` and
`<journal>.native-after`, each at most64KiB of the canonical native-context
bytes. A `file-intent/born/ready` triple for the before file follows the original
pending triple; its hash must equal the original `native-context` commitment.
The after file's triple follows the original `reload-intent`, after the actual
one-use reload, stopped policy rounds and checked generator observation, and
precedes `replacement-verified`. Its bytes may differ from the before snapshot
only in the three generator directories. Both files retain their original
birth identities, Sync/readback/Close ordering and the replacement blocker.
They are outside the sealed twelve-file generation; an empty or partial sibling
blocks ordinary completion and never supplies a retry or context refresh right.

A recovered initial selection followed by replacements retains every original
non-generator ancestor, absence, manager/boot/process/namespace and search path.
Each later before snapshot contains those exact observations; additional
independently retained ancestors are permitted but cannot replace an original.
Its generator identities and absences also match the previous proved current
snapshot: an intervening unproved change cannot be hidden in the next before
file even when that next before/after pair is internally coherent.
Only a complete ordered v2 before/after proof admits that reload's generator
change. Ordinary readers validate the entire selection and native proof lineage
before opening their own fresh current context. A v1 gap or missing snapshot
cannot be replaced by a current-looking directory or caller-supplied waiver.
During the actual replacement, the original recovered observer may change only
its three generator observations after its own successful one-use reload and
checked fresh namespace; old descriptors close before the new observation is
accepted. A failed close leaves the operation blocking.
Controlled v2 cuts may bind this checked current context at the after-file
intent/born/ready boundaries as well as `replacement-verified`. The exact after
path and intent hash must bind that same context after the original reload
intent. This does not permit context changes at any other file group, Start or
retirement, or repeat an attempted original reload.

New initial journals use opening phase `forwarding-initial-v2` and add
`native-context` immediately after it. New start journals likewise add
`native-context` after the opening record; first/invocation intent and archive
bindings remain distinct. The context hash commits canonical
`ardents-forwarding-native-context-v1` observations: full broker/unique manager
identity, original manager proc/start clock and boot commitment, original root
namespaces, fixed unit-search paths and retained input/destination ancestors.
These are native observations, not authority. Recovery must independently match
them. Complete historical119-record initial and25-record start transcripts stay
readable; they grant no recovery of a prefix whose original context cannot be
proved. The retirement18-record and Hosting16-record grammars remain separately
owned. Hosting preparation always commits the initial binding, original allowance
directory and original pin/lock births, even while a later generation is selected.
No new binding can renew preparation or initialize the allowance again.

The replacement context's exact ordered fields are `schema`, `manager`,
`process`, `started`, `boot_sha256`, `namespaces`, `unit_paths`, `ancestors`,
`absent`, encoded as compact JSON plus LF, at most64KiB. Each native object is
ordered `path`, `device`, `inode`, `mode`, `uid`, `gid`. The original root namespace
order is `mnt`, `pid`, `cgroup`, `uts`, `user`; manager-namespace equality and
original kernel descriptors are independently rechecked. Unit search paths keep
the manager's order; ancestors and absent paths are sorted and unique. Hashing
these observations cannot prove original Sync, Close or a genuine cut.
Ordinary root history inspection also bounds the installation parent's name
inventory to4096 entries and rejects every unrecognized replacement sibling.
This includes an empty journal born before its pending file. The replacement
writer admits only its own retained journal and newborn candidate/pending
objects; no caller-supplied waiver enters ordinary completed readers.

Canonical Node uses `forwarding replace DISTRIBUTION HISTORY ENVIRONMENT NETWORK
REQUEST BINDING` for the stopped transition and a separate `forwarding
start-selected DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING` for its first
runtime invocation. `restart` requires the already selected generation's own
completed start and retirement. Neither operation substitutes new configuration
for the predecessor's archived execution clocks, process or domain completion.

Canonical `forwarding supervise-transition --stop-after-record N --timeout-ms MS
-- ACTION ...` owns only an exact original coordinator from birth. ACTION is one
of the closed existing initial/replacement/start/stop root operations, never
`start-installed`, another command, role or arbitrary executable. Zero means
ordinary completion; positive N selects one bounded original record boundary.
The controller and coordinator use their own root-peer packet channel, original
program/process/pidfd, finite caller, fresh nonce and contiguous sequence. After
successful original record Sync/readback and before the next effect, the
coordinator reports exact journal native identity/hash/count and self-stops.
Only the original controller's checked ACK/resume permits another effect.
Attempted ACK is outstanding even after uncertain send. Denial is synchronous
before native interruption/join; no competing deadline callback can begin actor
cleanup while frozen. Unknown packets, foreign peers, received descriptors,
unjoined descendants and physical Close errors refuse. Descriptor-bearing
packets close every received descriptor before refusal. This is an original
native coordinator mechanism, not a Source permission or accepting test bridge.

Positive recovery additionally requires its own genuine completed
`ardents-forwarding-original-cut-v1` result, not a raw process-kill receipt.
Its root-only `<journal>.original-cut-journal`, `.original-cut-result` and
`.original-cut-pending` bind the frozen original boundary, actor from-birth
program/process, manager/context, denied next effect and actual original-scope
join. They have independent custody and a guard retained through every required
publisher Sync/readback/Close. Kernel/process join proves no Node domain join.
Recovery and every ordinary recovered consumer independently validate both
files, original journal identities/bytes, absent cut guard and fresh native
context. Without that proof, identical visible bytes may represent a previously
failed Sync/Close and must remain repair-required, including a complete-looking
original transcript. Source cut results/permissions cannot substitute.

The cut result's exact ordered fields are `schema`, `boundary`, `origin`,
`control_device`, `control_inode`, `acknowledged`, `code`, `status`, `reaped`,
compact JSON plus LF, at most256KiB. Its boundary retains the exact canonical
binding and raw original native-context bytes; the private channel transports
those as JSON byte strings without stripping their committed trailing LF.
The original snapshot remains separately retained as `original_context` after
an owned reload; `native_context` then binds the checked current snapshot.
Only the three generator directory observations may differ, and only at the
original `stopped-verified`/`completed` initial or `replacement-verified`
boundary after the original context commitment and reload intent. Start and
retirement cannot refresh this context. The packet has a256KiB limit and the
original socket pair must supply finite capacity for one entire packet; neither
snapshot nor binding may be truncated to fit an unsuitable environment.
Origin binds actual from-birth PID, parent, proc device/inode/start clock and
the original executable's device/inode/SHA256. `acknowledged` is exactly one
less than the frozen boundary sequence; kernel termination is SIGKILL and the
dedicated original scope must reap exactly one actor. These are physical
historical facts, never a Node domain-joined report.
Its independent control journal uses `initialRecord` fields and exactly13
records: `forwarding-cut-v1`; pending `file-intent/born/ready`;
`coordinator-born-observed`, `boundary-held`, `next-effect-denied`,
`native-joined`, `physical-closed`; result `file-intent/born/ready`;
`original-cut-completed`. Empty births have no content hash; ready records
bind complete bytes and the same native inode. This distinct journal and its
pending file do not replace or remove the interrupted operation's blockers.

New Forwarding start intents also bind whether genuine semantic roots have
ever had an admitted runtime start. Root computes this only from the complete
original chain; missing history cannot select fresh opening. A new local root
ACK is the original160-byte completion frame plus one exact history byte:
zero for proven first runtime opening, one for existing-only retained opening.
The entire161-byte ACK must complete before transfer. Its root journal records
the distinct request/ACK hashes and its intent binds that same history choice.
The new intent schemas are `ardents-forwarding-start-intent-v2` and
`ardents-forwarding-invocation-intent-v2`; their existing ordered fields append
the numeric `history` byte. The latter retains its exact predecessor commitment.
Complete-chain inspection independently checks the new byte against genuine
earlier runtime completions. Non-root composition selects existing-only semantic
opening from the fully verified ACK rather than its attempt pathname or number.
Legacy completed160-byte transcripts retain their old initial-versus-subsequent
meaning; they do not authorize an interrupted new exchange. The non-root Startup
independently checks the original peer/frame/native resources before consuming
the history byte. No plan flag, missing root or cached observation selects it.
This permits an explicit later start after a proved never-started cut without
recycling its attempt namespace or pretending that nonexistent spend/producer
history is retained. It changes no protection mode, token, State floor or budget.

`forwarding recover DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST JOURNAL`
admits exactly one named original initial/replacement/start/retirement prefix.
It requires an existing complete Release trust root and fresh coherent candidate
authorization, exact original request/selection/predecessor commitments, a fresh
exclusive original-journal lease and independent original native resources.
The original journal is immutable. A separate root-only exclusive completion
suffix binds operation, original path/native identity/raw-prefix hash, generation,
selection and its own ordered continuation transcript. It is not an original
writer result. The suffix has a separate pending guard and one finite original
lifetime; an existing partial or uncertain suffix refuses adoption or another
effect attempt. Canonical consumers require both original and recovery blockers
to be absent and independently validate the complete suffix. Recovery's actual
users and closes precede final publication; a copied phase, receipt or expired
wait cannot publish completion.
The canonical `ardents-forwarding-recovery-completion-v1` envelope is compact
JSON plus LF, at most4MiB, ordered `schema`, `operation`, `journal`, `device`,
`inode`, `prefix_sha256`, `prefix_records`, `generation`, `binding_sha256`,
`original_cut_sha256`, `outcome`, `records`, `transcript_sha256`. Operation is
exactly `initial`, `replacement`, `start` or `retirement`; outcome is exactly
`selected-stopped`, `stopped-before-start` or `retired-stopped` as appropriate.
Records are only the bounded canonical continuation records of that operation;
their sequence follows the immutable original prefix, whose complete transcript
hash is retained. Start/retirement continuation cannot add Start, ACK, archive,
domain-joined or native-joined evidence. A proved never-started attempt becomes
a retained terminal chain entry with this completion commitment as predecessor;
a separately admitted later start uses the next global attempt. Any recovery
sidecar forces full recovered validation, even if an older pending marker is
absent. A partial envelope or uncertain recovery Close is never reopened for
append or treated as an invitation to try the same effect again.

The recovery suffix has separate root-only0600 exclusive
`<journal>.recovery-journal`, `<journal>.recovery-completion` and
`<journal>.recovery-pending` files. For an already stopped complete generation,
the control journal has eleven ordered `initialRecord` rows:
`forwarding-recovery-v1`; pending `file-intent/born/ready`;
`original-verified`, `candidate-verified`, `stopped-verified`;
completion `file-intent/born/ready`; `recovery-completed`.
It binds the actual original inode and cut hash, exact sealed candidate and
fixed-unit identities, binding and unit hashes, and complete envelope bytes.
The pending body is `ardents-forwarding-recovery-pending-v1`, LF, original-cut
SHA256, LF. Required independent readback and every original/recovery close
precede original blocker removal; recovery's own guard is removed last.
An existing suffix, including an empty control journal born before its pending
file, refuses another recovery attempt. The prefix journal is never rewritten.

An already selected replacement-v2 original prefix may finish stopped
publication after its complete after-snapshot-ready record (61 records), or
after `replacement-verified` (62 records). The immutable prefix, original cut,
before/after snapshots, exact candidate, predecessor and complete original
retirement must all independently agree. The only logical continuation at61 is
the final `replacement-verified` row in the separate completion envelope; at62
the continuation is empty. Neither case reloads or starts anything.
Fresh coherent authority may have newer metadata versions for the same exact
program/descriptor/template. Recovery retains the already sealed original
metadata bytes and their original hashes; it never replaces them with that
fresh metadata or lowers the retained Release floors to reuse old permission.
Fresh semantic inspections still belong to State, participation, Spending and
Hosting, including the original one-use Hosting preparation and zero returned
reservations. Fresh stopped-policy observations match the original predecessor
execution and all five slots, not merely an inactive flag or absent PID.

The active recovery owns its freshly acquired exclusive original lease and
independently opens the full initial/selection/invocation resource observation.
An operation-private interrupted observation may retain that same recovery
owner's actual original lease; it is never creator custody or a public completed
opener. It verifies that original's exact bytes/inode/cut and admits only its
own concrete suffix births during publication. Ordinary readers require the
complete suffix and absent blockers. Independent readback acquires every lease
anew after original observers and writers close. The original replacement
blocker has its own retained installation-root parent; the suffix guard retains
the original journal's parent. All physical and semantic users close before
removing the original blocker under the suffix guard, then removing the suffix
guard last. No path list, caller boolean or copied receipt grants this view.

Root completed-generation observation owns the independently opened invocation
chain together with its original initial lease. A started-history handle retains
that same operation's chain; it does not acquire a second exclusive lease on
already observed child journals. Child readers own their separate file/socket
and journal descriptors, never the parent observation's Close. The parent joins
all child observations before returning its original initial lease. Current
start/stop writers remain separate actual owners; checking their own new storage
cannot authorize a copied path or waive an unrelated partial child history.

An original retirement prefix may finish publication at17 records (complete
result-ready) or18 records (`retired`). Both require the genuine original cut,
original domain-joined/native-joined/native-closed rows, exact retained result,
start archive, challenge/frame commitments, directory/socket/file identities and
the complete preceding invocation chain. At17 the separate completion contains
only the final `retired` row; at18 it is empty. Earlier result-intent/born hashes
cannot reconstruct the original random challenge or execution result and remain
repair-required. Recovery never repeats Stop, domain reporting or native join.
Fresh coherent candidate and existing-only semantic inspections, original
Hosting preparation and two exact stopped-policy rounds under the original
manager remain mandatory. Independent readback uses new child and parent leases
after all old observations and writers close. The original retirement blocker
retains its retirement-directory parent; the recovery guard retains the original
journal's parent. Both publication ordering and the common recovery control
grammar remain exact. The retained retirement-result digest continues to bind
later start/replacement history; the recovery suffix cannot replace that result.
Independent readback retains a new fixed Forwarding manager reference under its
freshly verified manager identity. It repeats both complete five-slot stopped
policy/quiescence rounds against the independently read original history and
fresh authority, then joins that reference and every fresh reader before either
blocker is removed. Original pre-close observations cannot substitute for those
readback rounds. This supplies no repeated reload, Stop or Start.

New controlled start boundaries use the distinct private schema
`ardents-forwarding-transition-boundary-v2`, appending `start_intent` after
`binding`. It is a byte string retaining the exact canonical intent plus LF,
at most8KiB; all other operations retain boundary-v1 and omit this field.
The actual start producer computes that same intent before its first journal
birth from its retained fresh Release decision, initial-history commitment,
global attempt, predecessor and proven runtime-history choice. The guard writer
uses those same bytes. The from-birth controller retains the intent unchanged
through every boundary and checks any original guard intent/birth/readiness or
start-attempt commitment against it. No file, Start or ACK is replayed by this
native proof. Recovery must still independently match the complete original
chain and obtain fresh authority; the copied intent is not permission.
The private256KiB packet bound and original scope/peer/sequence/join rules remain.
Boundary-v1 cuts remain readable without synthesizing a missing early intent;
an original complete guard can supply its own separately verified preimage.
The original25/26-record start grammars and completed160/161-byte ACKs do not
change. In particular, a later fresh metadata digest cannot be substituted for
the original decision committed by a cut before its guard was written.

A proved never-started cut retains a distinct terminal invocation entry, not a
start archive or retirement. For the new26-record grammar it is bounded by the
first record through the original `start-attempt` record (counts1..18): the
original coordinator is held before its next observation and manager Start,
then denied and actually joined. A raw marker cannot supply this proof. Every
original file/socket/directory intent, born or ready state must match its exact
recorded inode/access and empty-born or complete-ready bytes; unrecorded births
and uncertain I/O remain repair-required. Original resources are not rewritten.
An already born directory retains its original0700root or0750root:Node state.
If no directory was born, recovery alone may create that canonical attempt
directory as root:Node0750, retaining a separate recovery-owned birth proof.
The original pending file is unlinked only when its actual recorded birth exists;
otherwise its absence must remain independently proved through publication.

The separate start completion retains `stopped-before-start` outcome. Its
continuation has exactly one final `stopped-before-start` startEntry, preceded
only when needed by three `terminal-directory-intent/born/ready` startEntry rows.
The final row names that attempt directory and its exact current native identity,
mode and owner, with SHA256 of the original canonical intent. The optional
directory rows record recovery's own empty0700root birth and same-inode
promotion to0750root:Node; they never assert an original directory birth.
Recovery control keeps the existing eleven rows when the directory already
exists. Otherwise it inserts that actual directory-intent/born/ready triple
after `candidate-verified`, before `stopped-verified`, giving fourteen rows.
The complete envelope, native records and independently opened directory must
agree. Both old and recovery guards and all original/fresh closes retain the
same publication ordering. An existing partial suffix is never adopted.

The ordered invocation chain stores either a genuine runtime start or this
terminal proof at every global ordinal. A terminal neither creates an archive,
InvocationID, ACK or domain join nor sets the ever-runtime history choice.
Its exact completion digest becomes the next separate Start's predecessor;
that Start uses the next ordinal. Ordinary readers retain every child proof
under their own initial lease. A fresh first runtime after only terminal entries
may observe its known-born empty spend directory without creating it; actual
prior runtime requires the genuine existing-only spending inspection. State,
participation, original Hosting preparation/counters and zero reservations
remain independently checked through their genuine owners. Unknown joins or
roots never authorize a fresh opening. Stop cannot target a terminal entry.

Recovery's historical artifact reconstruction independently reads all four
original Release metadata files and the original infrastructure Enrollment
inventory. That inventory commits their seven public file hashes and remains
bound by the original configuration's Inventory SHA256. It must not be replaced
by the fresh candidate's inventory when current Release metadata advances for
the same payloads. The complete original generation grammar independently
rebuilds the canonical inventory through Enrollment; missing, substituted or
uncertain historical files refuse without fallback or writes. Fresh opaque
Release authorization separately establishes current admission and floors;
historical metadata/inventory bytes grant no renewed authority.

The pure original-inventory decoder describes72..104-record generation-file
prefixes as well as105/106/107. Account, directory and credential groups must
already be complete. It retains only the complete ordered artifact prefix and,
when recorded, the next original empty root0600 birth; an intent supplies no
file. Original directory-ready50/root0755 remains unchanged, and all later
artifact, seal, selection and unit records remain absent. This byte inventory
alone supplies no concrete native continuation permission.

At105/106/107 the decoder distinguishes timestamp intent absent, intent without birth, or original empty
root0600 timestamp birth106. It retains only the eleven completed generation
files and original directory-ready50/root0755; it never supplies timestamp
readiness, sealing, selection or unit births. This byte classification grants
no native continuation. For an exact original72..107 boundary chosen before
historical reads, candidate reconstruction reads only original file-ready
metadata: Enrollment inventory from90 records, root from99, snapshot from102,
Targets from105 and timestamp from108. Any not-yet-ready metadata remains a
detached byte proposal, including an originally recorded empty file. Enrollment's
complete canonical original inventory and the original binding must commit
every proposed byte exactly. Advanced/substituted proposals, missing original
ready metadata or canceled reads refuse without file creation or read-error
fallback. Outside this exact initial generation-file range, reconstruction
still requires all five original historical files. This byte reconstruction
alone supplies no concrete native continuation permission.
Matching proposal bytes alone grant no continuation. The concrete105..107
continuation additionally requires the original cut, physical custody and the
complete guarded proof below.

An Initial v2 cut at105 records (before timestamp intent),106 records (intent
without timestamp birth),107 records (original empty timestamp birth106),108
records (complete artifacts, before seal intent),109
records (seal intent durable, before access promotion),110 records (before selection intent),111 records (intent
without selection birth),112 records (selection born empty),113 records (complete selection file, before its selected
acknowledgement),114 records (before fixed-unit intent),115 records (intent
without birth),116 records (fixed unit born empty),117 records (fixed unit ready) or118 records (original
reload-intent durable) can prove a not-yet-attempted Reload only through its
genuine from-birth controller held at that exact boundary, next effect denied
and original scope joined. The original native context must still equal its
before context. The105..118 prefix must independently pass the complete original
account, directory, credential and phase-appropriate generation, selection and
fixed-unit inventory; it is not a completed initial transaction or startup proof. A marker
without that stopped source-ordering proof grants no first attempt or retry.
Fresh initial recovery requires the fixed Forwarding service still unloaded,
its installed file state static, the other four Node service/socket slots and
Forwarding socket absent, all four semantic roots empty, fresh coherent Release,
original principal/ancestors and exact native observations. No account operation or already completed effect is replayed by this boundary. At110..115 the unit must still be
absent; installed-file state is required only after actual unit delivery.

At105/106/107, the timestamp proposal must first match the complete original
Enrollment inventory and binding. The original generation directory remains
root0755 with ready50 provenance, eleven complete artifacts and either no
timestamp or its recorded root0600 empty birth106. All exact names, retained
native identities, bytes and access are checked; an unknown entry refuses.
After its own guard,105/106 records a timestamp intent and exclusively creates
a new root0600 empty file, recording its actual birth before byte delivery.
At107, fixedfile's `ContinueBirth` retains the original same inode. Independent
named/retained readback requires root0444 and exact original committed bytes
before sealing, selection, unit creation or Reload. Cancellation and uncertain
write/sync/Close remain blocking without repeated effects or file adoption.

The artifact controls extend the26-row seal grammar immediately after retained
candidate row5. At107, `forwarding-recovery-artifact-v1` inserts
`continued-artifact-retained` and `continued-artifact-written` (28 rows).
At105/106, `forwarding-recovery-artifact-create-v1` inserts intent/born/written
artifact rows (29 rows). Intent commits original timestamp bytes/root0444;
born/retained binds the exact empty root0600 inode; written binds that same
inode, original bytes and root0444. The separate logical v2 continuation adds
only missing timestamp intent105, actual creation106 if needed and ready107,
then seal108/109 and subsequent actual selection/unit births through120 rows.
Fresh independent readers match original/new timestamp provenance, current
file and canonical inventory, the logical transcript and the complete control.
No proposed bytes or synthesized historical FileInfo grants this custody.

At108/109, all twelve original artifact files and credentials are complete;
the original generation-directory-ready50 record still owns the root0755
directory. Selection and unit remain absent. After independent original/native
inventory, fresh authority and stopped admission, recovery creates its own
guarded control and records the candidate as retained0755. Its separate seal
intent precedes one same-inode Chmod0555 and directory Sync; independent named
and retained native checks and complete artifact reads precede the sealed row
and any selection/unit creation. No directory is recreated or adopted.
The26-row control is headed `forwarding-recovery-seal-v1`. In the24-row
selection-creation grammar, row5 becomes `candidate-retained` root0755; insert
`continued-directory-seal-intent` and `continued-directory-sealed` immediately
after it. Both bind the original directory device/inode, Generation SHA256 and
target root0555. The original ready50 record and retained row bind the before
state; current root0555 and the sealed row bind the actual result. The separate
v2 continuation adds seal-intent108 only if missing and sealed109, then actual
selection/unit births and the final stopped rows. Fresh readers verify original
directory provenance, control, logical transcript and current native inventory.
Unknown/torn inventory or uncertain mutation/sync/Close remains blocking; there
is no retry, rollback, fabricated historical FileInfo or accepting0755 waiver.

At110/111, every credential and sealed artifact is already complete, while the
selection must still be absent. The recovery actor retains its parent, repeats
absence and records its own guard/selection intent before exclusive creation.
It records the actual synchronized root0600 empty birth before writing the
original binding bytes. Current root0644 selection is independently read back
before unit creation. No pre-existing leaf is adopted. Its24-row control uses
`forwarding-recovery-selection-create-v1`: insert selection intent/born/written
before the unit-creation triple in the21-row grammar. Intent binds binding
SHA256/root0644, born binds the actual new device/inode and empty root0600,
written binds that same inode and original binding. The v2 continuation adds
intent110 only if missing, actual born111, ready112 and selected113, then the
unit continuation. Independent readers match that new birth in control,
continuation and current physical selection. Partial/uncertain effects block.

At112, the selection's original file-born111 must match the actual root0600,
single-link empty inode under its retained parent. Recovery independently opens
that exact birth through fixedfile ContinueBirth, records retained custody only
after its own recovery guard, then delivers only the original binding bytes and
promotes access on the same inode. Independent read-only byte/native checks
precede the guarded unit creation and single Reload. No missing, replaced or
nonempty selection is created, truncated or adopted. The23-row control is headed
`forwarding-recovery-selection-v1`: insert selection `continued-file-retained`
and `continued-file-written` before the unit-creation triple of the21-row
grammar. Both selection rows bind the original device/inode, empty root0600 then
exact binding SHA256/root0644. The separate v2 continuation adds file-ready112
on that original birth, selected113, then the unit continuation. Fresh readers
must match original selection birth, current selection, complete control and
logical transcript; writer/reader closes precede either blocker publication.

At113, the original selection file-ready row must independently match the
complete current selection on its recorded inode. Recovery never rewrites that
file: its separate logical continuation first records selected113, then follows
the exact unit-creation path below. The original prefix remains immutable; a
missing or changed selection cannot supply this acknowledgement.

At112/113/114/115, recovery retains the original unit parent and repeatedly proves the
canonical leaf absent. After its separate guard and `continued-file-intent`,
fixedfile creates that one leaf exclusively, synchronizes it and its parent,
and records its actual root0600 empty birth before delivering bytes. No existing
leaf is adopted. Independent same-inode root0644 byte readback precedes static
policy and the one Reload. Its21-row control is headed
`forwarding-recovery-unit-create-v1`: insert `continued-file-intent`,
`continued-file-born` and `continued-file-written` after candidate-verified in
the18-row grammar. Intent binds Unit SHA256 and final access; birth binds actual
new device/inode and empty SHA256; written binds that same inode and Unit SHA256.
The v2 continuation supplies selected113 and file-intent114 only if missing, actual file-born115,
file-ready116 and the final reload/stopped/completed rows. The separate control,
continuation and independently reopened physical unit must agree on the new
birth; it is never represented as an original recorded birth. Partial or
uncertain creation remains blocking with both histories preserved.

At116, the exact original fixed-unit inode must still be root0600, single-link
and empty. Only its original bytes may be delivered and access promoted to
root0644 on that same inode. No missing/replaced/nonempty leaf is created,
truncated, adopted or repaired. Before delivery, the separate native observation
requires Forwarding unloaded and every other Node service/socket slot absent;
it grants no installed-file-state or Release right. After actual same-inode
delivery and independent byte/read-access checks, the existing static/unloaded
pre-Reload checks apply. Any uncertain write/close remains blocking.
The116 control uses20 rows headed `forwarding-recovery-unit-v1`: insert
`continued-file-retained` and `continued-file-written` after candidate-verified,
before the native-before triple of the18-row grammar below. Retained binds the
original file-born device/inode, root0600 and SHA256 of empty bytes; written
binds that same inode, root0644 and the exact Unit SHA256. The separate v2
continuation adds only file-ready116, reload-intent117, stopped-verified118 and
completed119. The original birth stays in the immutable prefix. An independent
reader verifies that original birth, both continuation records, complete current
unit and the full native/reload proof before publication.

Such recovery uses the distinct completion schema
`ardents-forwarding-recovery-completion-v2`, appending
`native_context_sha256` after `transcript_sha256`. This is the SHA256 of its
actual complete after-Reload native context. V1 omits that field and remains
byte-compatible. V2 is admitted only for this proved Initial pre-Reload cut or
the Replacement57/58 boundary below; it cannot reinterpret an uncertain
original Reload or any Start/retirement.
Its separate control grammar has18 records: `forwarding-recovery-reload-v1`,
the pending file intent/born/ready triple, `original-verified`,
`candidate-verified`, the `recovery-native-before` file intent/born/ready triple,
`recovery-reload-intent`, the `recovery-native-after` file intent/born/ready
triple, `stopped-verified`, the completion file intent/born/ready triple and
`recovery-completed`. Both native files are canonical root0600 bounded64KiB
siblings `<original-journal>.recovery-native-before/after`; before equals the
original cut context, and after may change only the three owned generator
observations. Every native birth and complete hash remains independently
verified. The original journal is immutable. The continuation contains only its
missing reload-intent (at117), stopped-verified and completed rows, produced
after the one actual successful recovery Reload and stopped policy rounds.
Uncertain Reload or any partial recovery suffix remains blocking, with no
retry/adoption. Independent readback verifies the new completion commitment,
both original native snapshot births and complete stopped current context under
new leases before either guard is removed. Guard ordering and every required
physical join remain unchanged.

A Replacement v2 cut at57 records (unit-written) or58 records (reload-intent)
requires the same actual held/denied/joined before-effect proof, unchanged
original native context and complete original candidate/selection/unit
inventory. Its exact original `.native-before` must remain present and match
the cut. Original `.native-after` is independently proved absent. An active
operation-private observation may retain only that original prefix plus the
complete original predecessor/retirement/Hosting and semantic histories. It
supplies no current stopped-generation verification, completed replacement or
Start permission; ordinary readers still require a complete separate suffix.
The original loaded predecessor policy, execution and manager must remain
independently matched before the one admitted recovery Reload. Unknown or lost
native provenance cannot be replaced by an inactive flag or copied retirement.

Replacement uses the same v2 completion and18-record control, with its own
`recovery-native-before` birth and the after birth at the exclusively created
canonical `<original-journal>.native-after` path. That file is recovery-owned:
the immutable original57/58 prefix has no after birth. Both the recovery control
and separate continuation record its actual new inode; no original birth is
invented or an existing file adopted. The continuation contains only a missing
reload-intent at57, that actual after intent/born/ready triple and the final
replacement-verified row. The unused `.recovery-native-after` alternative must
remain absent. Only the actual operation's exclusive newborn file may appear
in its private partial view. Independent complete readers bind the native
before/after pair, v2 completion digest and original/recovery birth provenance
before admitting current native context. All prior generations, ancestors,
generator proofs, domain joins, allowance and close/publication ordering remain
mandatory; no current history, account, selection or unit is rewritten here.

Recovered native contexts are checked in actual generation/invocation order.
After each generation's own proved reload, its terminal and recovered retirement contexts
may add retained ancestors but cannot change any earlier object or absence,
including generator observations. A later replacement may then change only its
own three generators through its complete before/after proof. Only after this
entire graph is verified may a reader open fresh current context. Processing
old retirement context after a later reload would not prove that lineage.

| Original boundary and owner | Safe stopped continuation and retained proof | Blocking/refusal and join obligation |
|---|---|---|
| Initial journal/account / Installation and fixed native account helper | Complete original account birth and exact current principal precede any continued generation write; all original account/auxiliary provenance remains | Empty/torn/foreign journal, ambiguous helper/account birth, unrecorded inode or incomplete directory permission birth: repair-required; no useradd replay, account adoption or directory recreation |
| Initial directories and semantic roots / Installation | Every originally recorded complete directory keeps its inode/access; initial semantic roots remain empty; an original-cut proof at a file-born boundary requires the exact empty0600 inode, and file-ready requires exact complete candidate bytes/access | Missing/unrecorded birth, foreign or partial bytes, changed ancestor/credential/root, nonempty initial semantic root or unknown preparation: preserve blockers; no truncating foreign bytes, root initialization or history repair |
| Candidate files/seal / Installation and original file mechanisms | Intent precedes exclusive empty birth; recorded birth precedes bytes; the exact empty recorded inode may receive its admitted complete candidate, while ready bytes are only independently checked/resynced; complete twelve-file inventory precedes same-inode seal | Once the controller attempts ACK, the next effect is outstanding until a new proven successful boundary. Cuts during writes, between actual birth and durable birth record, uncertain Sync/Close or noncanonical inventory refuse, even when visible bytes match. Hold actual readers/writers/Close and retain original descriptors until join; timeouts do not release custody |
| Selection/fixed unit / Installation and fixedfile | Independently match the exact original inode and complete preimage at an intent boundary, or exact complete candidate at a written boundary; preserve both sealed generations and complete predecessor chain | Mixed/partial bytes, foreign inode, changed root or overlapping writer refuses without rollback/adoption. A physical write cut after ACK has no safe boundary proof; lower file mechanisms grant no authority |
| Reload / Installation and fixed Forwarding manager | At an exactly proved pre-Reload cut, separately record and attempt the one admitted reload; Initial117/118 requires the additional v2 proof above. After an original attempt, finish only an independently proved exact stopped result under the original manager/context, with fresh namespace/policy rounds | An unknown original reload result is not permission to replay the effect. Changed manager/context, competing slot, live/unknown process or native observation/Close uncertainty gives repair-required |
| Start / original root Startup producer | Independently classify the original prefix and any complete subsequent genuine retirement; a proved never-started prefix may only retire into stopped state, retaining its attempt and explicit future history-opening choice | Never repeat Start, archive, ACK or domain completion. A start-attempt marker, archive, ACK-looking bytes, inactive unit or PID absence cannot prove original domain join; unknown join stays unavailable |
| Retirement / original domain and native owners | Only exact original domain-joined/native-joined/native-closed provenance plus canonical result and independent original history/native checks may finish stopped publication | Never manufacture or repeat a joined-domain report. Missing result/join, substituted invocation/challenge/clocks, late close failure or uncertain publication preserves the original blocker |
| Hosting preparation and all retained domain histories / their genuine owners | Match initial allowance directory/pin/lock provenance and existing semantic floors through independent read-only inspections; reservations must have actually returned after their physical users joined | A new generation/path/unit cannot reset counters, refund burn, reinitialize preparation or clear an unknown producer/reservation. Partial Hosting preparation remains blocking, not another initializer |

Each repeated directory/file group includes before-intent, intent, actual birth,
durable birth, partial bytes, sync/readback, readiness and original-close cuts.
The same matrix applies to every reachable group; a claimed unsupported prefix
needs its exact missing proof and original source boundary, not a test waiver.
Finite source-backed byte/native controls precede the separately accepted exact
installed campaign. Only actual first contact/refill, joined stop, resource-only
replacement, fresh start, contact/refill, joined stop and same-generation restart
can qualify that installed journey. This contract selects no other Node role,
automatic restart/update/rollback, arbitrary repair, public/platform/availability
or independent-security/privacy qualification.

For Initial72..104, the finite continuation contract is the ordered unfinished
suffix of the twelve generation-file groups. Already-ready files are only read;
at most one original empty root0600 birth can be continued on its exact inode.
Every later file must be absent, then exclusively born after the recovery guard
and its own intent. Record the actual empty inode before delivering bytes,
promote the program to root0555 and other artifacts to root0444, and independently
read back each before proceeding. Full exact inventory precedes the existing
same-inode directory seal, selection, unit and one separately admitted Reload.
Every artifact writer must close before independent stopped readback acquires
fresh leases. Partial bytes, unknown entries or uncertain writes/joins block.

Its separate `forwarding-recovery-generation-v1` control inserts each unfinished
artifact's ordered intent/born/written triple immediately after candidate row5,
or retained/written for the one original recorded empty birth. The total is
26 plus three rows per unfinished artifact, minus one for an original retained
birth. The independent reader rederives the closed names, original positions,
native identities and ready bytes; the logical v2 completion fills only missing
original rows through the complete120-record transcript. Existing105..118
controls remain unchanged. These finite rules require source-matched native
and interruption evidence; this format alone supplies no successful recovery,
Release permission or installed qualification.

#### Initial stopped Source persistence

`ardents-next infrastructure install-initial-stopped` consumes the original
initial Enrollment provenance, the two fresh coherent Release authorizations
and original Source request/input custody in one root operation. It has no
candidate/recovery/Start form. Its four destination parents must already exist
and satisfy preflight custody; implicit parent creation is refused. The earlier
read-only request grammar and absence checks remain unchanged.

The exclusive root-only file `<installation_root>.source-initial-journal` is
both this operation's original native lease and its append-only initial journal.
An existing journal refuses, including when its installation root is absent.
It is never reopened as a new initial operation. Each record is compact JSON
plus LF, in the ordered fields `sequence`, `phase`, `path`, `device`, `inode`,
`sha256`, `mode`, `uid`, `gid`. Sequence starts at zero. Records distinguish
`initial` (the retained legacy format), `initial-recoverable`, `admitted`,
`account-preimage`, `account-auxiliary-preimage`,
`account-intent`, `account-born`, `account-auxiliary-result`,
`account-auxiliary-cleared`, `account-ready`,
`directory-intent`, `directory-born`, `directory-ready`, `file-intent`,
`file-born`, `file-ready`, `seal-intent`, `sealed`, `selected`, `reload-intent`
and `stopped-verified`. Intent is synced before the exclusive native effect;
born identity precedes byte delivery/access promotion. File records bind exact
intended public hashes or private local commitments, not exported credentials.
The journal and private observations are never public receipts.

The initial `admitted` record's private commitment binds the exact binding
SHA-256 to the original canonical Release history path, native directory
device/inode, mode and numeric owner/group. Its transcript is compact JSON plus
LF, ordered `schema`, `binding_sha256`, `history`, `device`, `inode`, `mode`,
`uid`, `gid`, with schema `ardents-source-initial-history-v1`; the record stores
that transcript's SHA-256, not its private paths. Installation retains independent
read-only directory/ancestor custody and checks the declared path against the
actual original Release verifier around floor observations. Release alone keeps
its lease, original directory/lock-file identities retained since opening, trust
and floor policy. Substitution before Installation acquires its independent pins
also refuses. Earlier stopped-only journals remain evidence;
their different commitment cannot admit first startup or select another history.

The legacy complete initial transcript contains exactly 112 canonical records.
New initial producers use the distinct `initial-recoverable` first phase and
exactly 115 records. Immediately after the original journal record, a
`file-intent`/`file-born`/`file-ready` triple binds the root-owned 0600 sibling
`<installation_root>.source-initial-pending`; `admitted` and the remaining
original effects follow. The marker bytes are exactly
`ardents-source-initial-pending-v1\n<BINDING_SHA256>\n`. Its original born inode
and ready identity must agree. This transient marker is not a thirteenth
generation resource or a runtime permission.

Both initial formats have separate complete-only validation. After checking
the recoverable header and its exact marker triple, a detached normalized
112-record view may reuse the unchanged legacy semantic validator; neither
the original journal nor its past result is rewritten. Ordinary readers require
marker absence and retain an independent shared journal flock. The actual
current creator may read back its own complete blocked transcript only through
its original tracked journal/marker descriptors. No ordinary caller or copied
marker obtains that exception. Partial legacy initial journals acquire no
new birth provenance by creating a marker afterwards.

Independent root-only readback consumes every phase in order,
checks intended paths/bytes/access and matching born/ready identities, binds the
original history commitment, and reobserves its own Source resource descriptors.
The stopped creator performs this readback after `stopped-verified` and retains
its writer lease until all native observers/helpers join. It consumes and joins
its original inputs before final marker removal, rechecks its still-live Release
owner and original parent/marker native identity/access, and leaves the marker
on any prior or cleanup failure. No required Installation sync or manager
effect follows removal; lost removal restores refusal. The reader neither
borrows that lease nor grants creation, current account, Release or Start rights.
Account database records retain historical helper evidence; they do not freeze
foreign host databases across operations. A later start separately admits the
current exact principal. State/participation contents remain semantic-owner data.

Each fresh root principal observation reads the current protected account/NSS
files and resolves both the fixed name and numeric identity in each of the
passwd and group databases. Each database uses one original `getent` helper
with both keys; both ordered output rows must exactly match the validated
current row. A missing key, partial output, foreign row, failed exit or deadline
refuses. The helper retains its original PID namespace and joined completion;
the actual `getent` file and every account/NSS file are reobserved before a
successful observation returns.
Combining keys does not cache principal authority or remove either lookup.

Canonical initial/replacement commands own a dedicated original coordinator
process, executing the same retained running artifact through a private inherited
Unix packet channel. They preserve the actual manager PID/mount/cgroup
namespaces. The controller retains its launching thread, original from-birth
pidfd and unreaped process-group leader, and acts as a dedicated subreaper with
no unrelated children. Denial precedes group interruption; actual waits join
the coordinator and every adopted descendant before native scope release.
Ordinary success additionally refuses descendants that the controller had to
reap: killing a helper cannot supply its missing domain completion.

After successful journal Sync/readback, the coordinator may report one exact
native journal identity, prefix commitment/count, generation and binding. It
self-stops before another effect. The controller verifies root peer credentials,
original program, nonce/sequence and the actual kernel stop before sending the
next-effect ACK and resuming the original pidfd. An attempted ACK makes the next
effect outstanding even when its send is uncertain. Original cancellation and
the one finite deadline belong to the controller; no competing coordinator
deadline callback may begin cleanup while a boundary is frozen. Unknown packets,
channel loss, native errors and uncertain Close preserve failure. Received
file descriptors are never channel inputs and must be physically closed before
refusing their packet, including truncation and missing credentials.

These private process/channel facts alone grant no recovery permission. A
separate completed original-cut proof, guarded through all required controller
Close and independently bound to that exact prefix, must be mandatory both at
recovery admission and ordinary recovered-transcript consumption. An
uncontrolled old prefix, ambiguous Sync/Close or unproven domain join remains
repair-required. Physical process release does not assert application Close.

The separate root-only original-cut journal/result retain the exact frozen
boundary, original from-birth process, program commitment, native manager
namespace/boot facts and the original connection's broker-instance/unique-owner
identity. A pending marker survives every required publisher Sync/readback/Close;
its final removal follows actual process and observer join. Complete-looking
result bytes under that marker remain unusable. This records a controlled
process cut, not disk/power-loss durability or a clean application shutdown.
Fresh recovery and ordinary recovered-initial readers independently open and
retain the original prefix and both result files, validate their exact closed
grammar/native lineage and require guard absence. They separately compare fresh
broker/native manager observations; detached historical facts cannot supply
current stopped policy, Release/State authority or an accepting manager reply.

Explicit recovery acquires a fresh exclusive original journal lease and writes
only a separate `ardents-source-recovery-completion-v1` envelope beside it. The
envelope binds original journal path/device/inode, exact prefix hash/count,
operation, generation, binding hash, subsequent canonical records and complete
transcript hash. It neither rewrites original records nor erases the interrupted
caller's failure. Partial envelopes are retained and cannot be reopened for
append by another attempt. Ordinary readers independently retain both files,
the genuine original-cut result and their shared leases, then validate the
unchanged complete operation grammar and native resources. Any initial cut
journal/result/pending or completion sidecar forces this recovered
validation even if the original itself contains all115 records. Such evidence
cannot be ignored because the initial pending marker is absent. An empty suffix is
valid only when the original full115/full55 transcript was already recorded;
the original pending guard still excludes ordinary consumption until all fresh
recovery users have joined and fresh Release/native rechecks permit its removal.

Initial recovery requires complete original account and directory/credential
provenance, empty State/participation roots and no invocation history. Only
exact independently reopened empty0600 original born files may finish delivery;
partial bytes or promoted access without ready provenance refuse. Complete
generation/seal/selection may continue under the same fresh checks. After a
complete fixed-unit birth, all recorded native resources are matched before a
fresh broker/unique-owner-matched manager reference can load the unit. Exact
never-started properties remain mandatory. A previously recorded successful
reload is not repeated. Intent-only or incomplete fixed-unit birth remains
repair-required; no preflight absence exception adopts it.

Replacement recovery independently retains the full predecessor lineage and
committed retirement, fresh State inspection, original fixed-file preimages and
the exact old/new current bytes allowed by that prefix. Fixed selection and unit
effects remain separate; unknown/torn/mixed publication refuses. Neither a
fresh manager reference nor its copied expectation grants retirement or State
authority. Startup/archive/ACK and retirement journals are never replayed. A
read-only recovery classification may report stopped only for the requested
latest complete and committed attempt under the currently selected generation,
after fresh input/Release/native/stopped and quiescent checks. Missing original
domain/native/Close completion remains repair-required after process exit.

One fixed native `useradd` creates `ardents-source` with an unused, explicitly
bound numeric UID/GID in 100..999, locked password, `/nonexistent`, nologin,
no supplementary groups, home, subordinate IDs, mail spool or log initialization.
The original helper and its fixed configuration are pinned; local hooks and
nonempty supplementary-group defaults refuse. A successful original helper,
exact unchanged foreign database bytes, one exact added locked account/group
and retained post-effect native observations establish this operation's birth.
Public passwd rows cannot adopt an existing account. Helper failure or ambiguous
account completion preserves the original journal as repair-required.
Hook absence or original directory inode/access/emptiness remains pinned and
reobserved before helper admission and through completion. Installed nscd or
SSSD cache helpers refuse; their effects are outside this local-files contract.
The helper is PID1 of its own new PID namespace, with original-caller cancellation
and parent-death SIGKILL; kernel namespace teardown joins descendants before init
reap. Database backups, the password lock and fixed PID1 temporary lock names are
separately recorded and checked. This is not a claim that useradd never forks.

Installation creates only empty runtime-owned 0700 State and participation
directories. Their semantic owners initialize/accept their contents in a later
separately admitted operation. A root-owned 0755 credential directory receives
the exact original public certificate/CA as 0444 root files and the explicit
runtime TLS private key as a fresh runtime-owned 0600 file. Source's existing
certificate/private-pair agreement is checked before birth; this is no offline
authority import, PKI, State acceptance or handshake-time trust permission.

The selected generation identity is the Source descriptor SHA-256. Its closed
twelve-file inventory is the original eight-file infrastructure distribution,
`source-plan.json`, `source-request.json`, `binding.json` and
`rendered-source.service`. Files are root-owned 0444, except the actual program
0555; the completed generation directory is sealed 0555. No Endpoint fifteen-file
inventory or journal is extended. Each original inode, bytes, access and complete
inventory remains reobserved across subsequent effects.

`ardents-source-binding-v1` is compact JSON plus LF, at most 32 KiB. Ordered
fields are `schema`, `invocation`, `generation`, `predecessor`, `request_sha256`,
`program_target`, `generation_target`, `program_sha256`, `template_sha256`,
`inventory_sha256`, `targets_sha256`, `plan_sha256`, `unit_sha256`, `account`,
`uid`, `gid`, `request`. Initial predecessor is empty; request is the complete
canonical Source request. This is immutable selected configuration, never a
serialized Authorization. The root's 0644 `binding.json` selects its exact
sealed generation binding; the fixed 0644 Source unit is published afterwards.
No symlink selector or automatic rollback is used.

The original manager connection survives publication, one explicit daemon
reload, and two complete loaded Unit/Service observations around independent
resource reobservation. Compare original unique owner, Version, UnitPath,
boot/start and namespaces, exact fragment/argv/account/protection/resource
envelope, no drop-ins/hooks/queued Job, inactive/dead and zero process/execution
facts. No Start/enable/restart is called. Reload uncertainty, substitution,
cancellation, sync/readback/access/close failure retains the partial prefix and
first failure; files and manager are not one atomic transaction. This bounded
consumer neither implements continuation nor claims a bootstrap or runtime
acceptance. Exact acceptance and evidence remain in the selected issue.
The never-started `InvocationID` is the exact empty `ay` returned by systemd255,
not a synthetic sixteen-zero invocation. Original input/destination ancestors
stay pinned throughout. Unit-search namespace observations are separate: after
the owned successful reload, only `/run/systemd/generator`, `generator.early`
and `generator.late` receive fresh phase custody because systemd recreates its
generator output. Other namespace pins remain original. Fresh namespace absence,
loaded fragment/drop-in/argv/protection checks and subsequent same-phase inode
checks remain mandatory; the read-only preflight's original pins never rebind.

| Boundary / effect | Proof, linearization, failure and completion owner |
|---|---|
| Prepare / pre-effect admission | Command retains original caller/request and all immutable inputs; Enrollment provides initial provenance only for initial install; Release independently provides fresh coherent proofs. Installation validates exact role/roots/access/protection/stopped state before account/root/file/manager effects. No runtime owner is opened as a side effect of read-only inspection |
| Account, directory and generation birth | Installation acquires its exclusive original lease, durably records exact intended births before exclusive creation, then writes/syncs/readbacks same-inode resources and records seal. Partial creation retains original evidence and first failure. No adoption of existing directories/accounts or writer-prefix cleanup by a different invocation |
| Publish selection / reload | Retain a known stopped or fully joined predecessor and original manager reference. Verify forward candidate plus every input again, record preimages, publish/sync exact selection/fixed unit, then explicit manager reload and complete loaded-property check. Filesystem and manager are not one atomic commit. Failure keeps exact partial state, no start or automatic rollback |
| Explicit start / startup barrier | Fresh start admission revalidates selected resources and current Release policy under surviving floors. Root starts only the fixed exact unit. Non-root startup independently retains files/proc/cgroup/manager, checks exact generation and completes an original root-peer barrier after durable start archival. Only then can its opaque original startup Check admit Source/Node construction. ACK is transaction completion only, not State authority or runtime readiness |
| First and continuing work | Source checks its original inputs/process/startup before State.Open can create exposure/listen, and again after I/O/handoff. State owns current signed responses/time/exposure. Node adds exact duty/producer, receiving/Hosting and selected Store/Registry/issuer owners before Receiver.Listen. Checks run outside foreign state locks; local commit latch only compares original identity/denial, never performs foreign I/O or callbacks |
| Deny / interruption / original join | Startup or input/protection loss synchronously seals dependent admission and cancels original caller. Source State.Close joins handlers/transport/refresh and exposure cleanup while retaining its root; command joins Wait, then input/key/process custody. Node Stop interrupts Route, joins physical users, closes role roots/reservations/producer/process/key, and command closes State last. Installation separately joins original manager/process/cgroup/completion users before releasing transition custody |
| Result / late failure | Preserve first domain failure plus independent physical close/sync/release errors. A bounded unavailable notice, output success, ACK, inactive unit, absent PID or empty path does not prove domain join. No copied handle or late callback can admit a new generation. A failed wait never becomes success because physical work finishes later |
| Explicit restart | No manager automatic restart/enable-on-boot is authorized. A new explicit invocation rechecks actual artifacts/unit/account/process absence, retained histories, fresh Release safety, current State/time and exact credentials. It creates no authority by repeating an old ACK. Expired/conflicting/missing State, unknown old producer completion or unavailable floor refuses; restoring files does not revive an already denied lifetime |
| Explicit replacement | Requires strictly forward exact generation proofs even if only resources changed. Deny and join predecessor first; retain its complete bytes and all semantic histories. Fresh invocation performs independent startup. No live key reload, shared roots with a concurrent generation, silent continuity or automatic rollback |
| Crash / explicit recovery | Independently reopen original journal-born inodes and retained selection/preimages, authenticate selected/candidate bytes and fresh proofs, classify one exact prefix. Pre-effect prefix may remain stopped; admitted born writing prefix can continue only recorded original files; sealed selection/reload prefix must reobserve exact manager and stopped/joined predecessor. Ambiguous account birth, mixed files, missing archives, torn/unrecognized record, unproven join or uncertain sync gives repair-required with no start/adoption/reset. No replay of Start/ACK from phase presence. Infrastructure recovery needs its own implementation/oracles; Endpoint recovery grammar does not authorize it |

Source exposure removal and Network Node-producer release remain their owners'
decisions. A killed process cannot issue their missing clean join. In particular,
unresolved live Node producers remain conflicting after Epoch expiry; installed
recovery must refuse a new Node rather than erase that fact. Bounded recovery
can legitimately remain unavailable. Power-loss and arbitrary host repair are
not inferred from process-kill controls.

#### Explicit forward Source transition grammar

The bounded manual replacement has three distinct operations, with no implicit
stop or Start: prepare/sign a forward candidate, `stop-source` the original
invocation, then `replace-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK
BINDING REQUEST`. A separate explicit `restart-source` starts the newly selected
generation. These are the implementation contract for the selected replacement
slice; the existing initial and restart consumers alone do not implement it.

Preparation retains the original purpose-specific private inputs and public
predecessor metadata. `sign-forward-release` preserves the exact predecessor
Root and key set, increments timestamp/snapshot/Targets versions and requires a
strictly greater descriptor release version. It first authenticates the complete
predecessor pair, then the candidate pair under that same diagnostic verifier;
the diagnostic history is separate from the installation's retained history.
Only Release decides either pair. A fresh diagnostic root or output receipt
never substitutes for the installed verifier's original trust and floors.

Replacement requires complete original retirement and the original first-start
journal's exclusive lease before any candidate birth. It retains the initial
birth transcript, every prior generation's complete immutable inventory, every
start/retirement transcript, exact account/UID/GID, original credentials, State,
participation and Release-history identities. Replacement provenance is distinct
from the initial binding grammar; an arbitrary predecessor field cannot enter
the initial/first-start reader. The replacement commitment names the exact old
binding, generation and final retirement, new descriptor/plan/request and
retained native resources. It grants no runtime or Release authority.

Before the first replacement effect and around subsequent native observations,
Installation consumes Network State's opaque `SourceInspection` against the
exact original public serving declaration and numeric UID/GID. State retains
independent read-only descriptors and the existing exclusive State and local
participation leases. It authenticates the retained signed chain, current
security floor, pending evidence and trusted time, and refuses missing,
conflicting, uncertain or expired history and unresolved participation producers.
It initializes, repairs and prunes nothing, opens no runtime or listener and
returns no `CurrentRuntime` authority. Its original caller and copied handles
share denial and joined Close; all physical inspection users and descriptors
close before Installation removes its final replacement blocker. The old
invocation's genuine retirement proof remains independently mandatory.

The replacement writer records intent before each exclusive birth, seal,
selection and fixed-unit publication, explicit manager reload and independent
readback. Selected binding and fixed unit are separate durable effects, not one
atomic transaction. An exclusive pending blocker survives any partial/uncertain
write, readback, reload or Close; complete-looking records under that blocker
refuse Start and replacement. No operation adopts, repairs, rolls back, removes
or resumes a failed prefix. All original physical users join before custody
returns; cancellation and late success preserve the original first failure and
independent cleanup causes. A new explicit Start obtains its own fresh coherent
Release/native/State/time/credential/protection observations and root-peer
archive/ACK; no old ACK crosses a generation boundary.

A resource-only generation may change the Source's bounded local serving
declaration (for example its literal unprivileged listen address), preserving
the exact program, signed fixed unit template, credentials and all semantic
roots. The request and plan must agree and receive immutable replacement
binding; a strictly forward signed descriptor/Release version is still required.
This is an actual configuration change, not a comment, padding or protection
template change. It supplies no new trust or capacity. Both old and new native
listeners must be observed in their respective original invocations, with old
listener/domain/native join preceding replacement and the new listener opening.

The replacement selection is compact JSON plus LF, at most32KiB, ordered fields
`schema`, `configuration`, `initial_generation`, `predecessor_binding_sha256`,
`retirement_sha256`, `predecessor_attempt`. Schema is
`ardents-source-replacement-binding-v1`. Its nested configuration has the
existing binding field order but distinct schema
`ardents-source-replacement-configuration-v1` and a nonempty exact predecessor
generation. The original initial binding decoder remains initial-only. The
initial generation, exact previous binding and complete original retirement
commitment bind one retained lineage. The attempt sequence remains global,
contiguous and bounded by64 across generations. An unchanged program requires
an actually changed canonical listen address; formatting or descriptor-version
changes alone supply no configuration delta. State/trust, credentials and the
fixed protection policy remain exact.

`<installation_root>.source-replacement-<generation>-journal` is root-only0600
and exclusively born. Its55 ordered records use the existing closed record
fields, with phases: `initial`, `replacement-admitted`, `predecessor-binding`,
`predecessor-unit`, `predecessor-retired`; the original pending file's
`file-intent/born/ready`; candidate `directory-intent/born/ready`; all twelve
lexically ordered generation files' `file-intent/born/ready`; `seal-intent`,
`sealed`; `selection-intent`, `selection-written`, `unit-intent`, `unit-written`,
`reload-intent`, `replacement-verified`. The admitted commitment has distinct
schema `ardents-source-replacement-history-v1`, binding the complete new selection
to the same original Release-history native identity. Initial history commitment
bytes are unchanged. A namespace bound of4096 sibling names prevents an unbounded
ledger scan; exhaustion refuses without pruning.

`<installation_root>/source-replacement-pending` has exact bytes
`ardents-source-replacement-pending-v1`, LF, complete selection SHA256, LF.
Unknown replacement journals refuse even before pending-file birth. Complete
records under that blocker never admit a later start. Final original unlink
follows every required sync/readback and original physical Close, with no required
sync after unlink; losing that final unlink can only restore refusal. This is
safe prefix retention, not recovery continuation.

The fixed selection and unit are rewritten through independently retained
physical mutation handles on their original inodes. Their complete predecessor
bytes remain in the sealed generation; original preimage/native records and
same-inode publication records account for the changed active bytes. Independent
lineage readback checks those exact two historical observations against the
current fixed-file inodes and every other initial/native birth against its
unchanged actual resource. It consumes every intervening start/retirement record
under that attempt's own generation, never the newest binding. A replacement
writer's synchronous readback checks its own original blocked journal/identity;
subsequent readers must acquire their own lock after that writer has joined.
No public receipt or serialized phase recreates those original rights.

Only the successful owned reload admits fresh observations of the three
systemd generator output directories. All other original namespace pins and
absence checks survive, and the original replaced observers physically close.
The replacement manager handle exposes fixed observations and one reload;
it exposes no Start, Stop, Enable, arbitrary selector or reconnect operation.

#### Explicit Source retirement and subsequent invocation

The supported root consumers are `ardents-node installation stop-source HISTORY
BINDING` and `restart-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING`.
Stop verifies retained history identity and the exact selected running program;
it does not require unexpired Release authorization to interrupt original work.
Restart independently obtains two fresh coherent Release authorizations under
the surviving floors. Neither operation adopts, replaces or updates a generation.

The first-start journal and its original `source-startup` directory remain
unchanged histories. An independent root descriptor takes an exclusive native
lock on the original first-start journal for later stop/restart operations;
this excludes its original writer and concurrent successors. First start still
requires exclusive creation and never reopens that journal as a new attempt.
Subsequent invocations exclusively create `source-invocation-NNNNNNNN` and the
sibling `<installation_root>.source-invocation-NNNNNNNN-journal`, starting at
00000001. Discovery requires a contiguous canonical prefix. The bounded history
holds at most 64 attempts including first start; exhaustion refuses without
pruning or reusing any name. No mutable selector, directory deletion or empty
reinitialization supplies another attempt.

All 22 start records, in order through `ack-written`, are independently checked
with their canonical bytes and matching born/ready inode identities. Subsequent
start records use schema `ardents-source-invocation-record-v1` and first phase
`invocation`; the existing first-start grammar retains its original schema and
phase. The subsequent guard schema is `ardents-source-invocation-intent-v1`,
with the existing ordered intent fields followed by nonzero `attempt` and
`predecessor_sha256`, the exact previous retirement file commitment. Socket and
archive schemas are `ardents-source-invocation-socket-v1` and
`ardents-source-invocation-archive-v1`, with the existing ordered fields. First
start omits those two successor intent fields. Each new invocation has its own
root peer, socket, guard, archive and ACK; an older ACK is never replayed.

Each start directory also receives the original root-only
`source-start-pending` blocker before guard/socket use. Its fixed bytes are
`ardents-source-start-pending-v1`, LF, the binding SHA256 and LF. The first or
subsequent start writer removes only that original blocker after complete
archive/ACK journaling, every proof sync and original observer/writer Close
succeed. No required sync follows the final unlink. Independent stop/history
readers refuse its presence even if all 22 records appear complete. This keeps
archive-before-ACK, uncertain final append, failed Close and pre-commit process
cuts unavailable; the full ACK's live handoff is not durable restart permission.

Stop independently matches the original archive to actual manager execution
clocks and InvocationID, current principal/selected files, original proc inode,
argv/executable/namespaces, retained pidfd and original cgroup directory/events.
The archived process clocks are the typed `ExecMainStartTimestamp` and
`ExecMainStartTimestampMonotonic` fields. Their original values remain fixed
through process exit. `ExecStartEx` initially reports the command's earlier fork
clocks and may replace them with the main-process clocks on reap; those earlier
values cannot identify the retained process start. Both observations must still
agree on PID, exit clocks and wait result, and completed command clocks must
match the original main-process clocks exactly.
Its short admission excludes successors before physical interruption. The
process adapter sends one SIGTERM through the original pidfd, without numeric
PID killing or StopUnit on a possibly replaced invocation. Existing first-start
`SourceControl.Stop` remains cleanup of that control's own attempted Start.

Within the original attempt directory, root first exclusively creates
`source-retirement-pending`, containing the exact ASCII line
`ardents-source-retirement-pending-v1`, LF, the 64-character start-archive SHA256
and LF. It synchronizes this blocker and its directory before continuing.
Presence of any blocker, including an empty/torn one, refuses retirement use.
The separately exclusive 0600 `source-retirement-journal` uses ordered
fields `schema`, `sequence`, `phase`, `path`, `device`, `inode`, `sha256`, `mode`,
`uid`, `gid`, with schema `ardents-source-retirement-record-v1`.
Its exactly 12 phases are `stop`, `socket-intent`, `socket-born`, `socket-ready`,
`interrupt-attempt`, `domain-joined`, `native-joined`, `native-closed`,
`file-intent`, `file-born`, `file-ready`, `retired`. Header binds the original
start archive and journal inode. Socket/file births precede use; every append
is synchronized and read back. Challenge, domain frame and result commitments
have distinct records; a phase search or detached cleanup struct is insufficient.

Root publishes one original `stop-completion.socket`, 0660 root:Source-group
under the existing 0750 root:Source-group attempt directory. It verifies the
actual peer PID/UID/GID against its retained original process and sends one fresh
32-byte challenge. The original non-root command may reply only after State
Close/Wait, transport/handlers/exposure, original observations, Startup input
Close, loaded-key erasure, Guard Close and bounded final output all join. The
complete result must contain exclusively ordinary cancellation or no error;
an earlier completion-unavailable notice or physical failure prevents reporting.
An interrupted original manager-clock read retains its actual cancellation or
physical error. It is not classified as a changed binding merely because the
read was canceled; a successfully read foreign clock still refuses.
The one-use opaque Startup also requires its original Close completion. It
reopens only its original completion directory, verifies the actual root peer,
and replies with ASCII `ardents-source-domain-joined-v1` plus LF, its original
160-byte startup frame and the exact challenge. It closes this exchange before
the command can produce its private completed-stop result. Main preserves the
original cancellation in that result and exits zero only for this exact clean
stop; ordinary signals and failed reporting retain their failed exit.

The root consumer then independently joins its original kernel events object
and requires the original manager's matching start/exit clocks, PID and clean
native wait code/status 1/0 in two complete observations. It closes the original
manager, proc/pidfd and cgroup before recording `native-closed`. Exit zero,
inactive unit or an empty cgroup alone cannot substitute for the domain frame.
Every failure retains the original result and the blocker. A canceled or failed
observer after attempted interruption still joins original physical work before
returning custody; a timeout never proves completion.

The root-only 0600 `source-retirement.json` is compact JSON plus LF, at most
64 KiB, with ordered fields `schema`, `attempt`, `generation`, `binding_sha256`,
`start_sha256`, `invocation_id`, `pid`, `manager`, `challenge`,
`completion_sha256`, `started_realtime`, `started_monotonic`, `exited_realtime`,
`exited_monotonic`, `code`, `status`. Schema is `ardents-source-retirement-v1`.
The reader recomputes the domain frame, consumes the complete start and stop
transcripts and checks original named resource identities. This private evidence
is not public receipt material and cannot reconstruct a live Startup right.

Root removes only its exact original blocker as the last commit effect, after
all proof sync/readback and original observer/input/writer Close succeed. No
required sync follows that unlink: losing the unlink can restore refusal, while
an uncertain earlier sync or Close leaves the blocker present. A process cut
before this point remains refusal even if complete-looking result bytes exist.
A cut after the point does not undo already proved domain/native completion;
later command cancellation/reporting failure is retained, not an automatic
restart. Power-loss, arbitrary disk repair and root compromise remain unqualified.

Restart rechecks blocker absence around complete proof readback and at effects,
requires the same generation/credentials/roots/account/manager and repeats fresh
Release, native stopped/empty-scope and independent startup admission. A collected
unit may have no retained execution fields; it still requires the complete
durable original retirement proof. State's `OpenRetained` refuses missing
ownership/control history and uses existing participation with `Create: false`;
State alone verifies signed generations, conflicts, exposure and time floors.
This changes no expired-Epoch serving or CurrentRuntime authority rule. No old
State object, root lease, guard, key borrow or accepting lifetime crosses to
the new invocation. An incomplete start, unknown process exit or retained failed
stop remains repair-required; these consumers implement no recovery continuation.

### Component order and the command/artifact boundary

Source is the smallest real infrastructure consumer: State + Source + Guard.
Its implemented development predecessor is original Source plan/certificate/private-key/CA
custody through actual serving and joined retirement. This uses existing
authority, changes no persisted/wire grammar and can be verified on the genuine
development command now. It is not an installed tracer, installer or bootstrap
acceptance. Its single implementation brief belongs in the execution ledger.

After accepted infrastructure authority, the next distinct boundary is exact
Source read-only native preflight, then initial stopped
generation and guarded startup with its finite interruption/recovery controls.
Existing Endpoint native mechanisms may be reused through explicitly registered
narrow seams, never by feeding Source into `endpoint` Request/Authorization or
generalizing its fixed unit behind a boolean. Future code/package allowances
require the normal complete architecture change, not this document.

The separate Source artifact authentication component is implemented with a real
compiled self-artifact consumer and public finite preparation. Its same-caller
`infrastructure preflight-initial` / `preflight-candidate` consumer additionally
checks the closed Source request/template and read-only original native facts.
The public result is a bounded observation, never installed permission. Other
invocations remain refused.

The canonical `ardents-source-preflight-v1` request binds exactly `schema`,
`invocation`, `profile`, `installation_root`, `state_root`, `participation_root`,
`credential_directory`, `plan`, `certificate`, `private_key`, `client_ca`, `clock`,
`address`, in that order, compact JSON plus LF and at most 16 KiB. Invocation is
`source`, profile `ardents-source-process-v1`; address is a canonical literal-IP
TCP endpoint on an unprivileged port. All paths are canonical absolute and
independent. Plan/certificate/key/CA are existing original inputs; the four
root/directory fields are separate absent future destinations. The plan uses
the future credential directory's exact `certificate.pem`, `private-key.pem`
and `client-ca.pem` names. Preflight does not deliver them. Source owns the shared pure closed serving-declaration codec; Installation
checks exact future-input matching and bounded preflight declarations without
opening State or its roots. Runtime authority/time decisions remain separate.
Root-controlled original files, access and ancestors remain retained through
physical Close; private input bytes never enter a public receipt.
Runtime installation, State, participation, credential and clock paths must also
remain visible under the fixed unit's namespace. Equal or descendant paths under
`/tmp`, `/var/tmp`, `/home`, `/root` and `/run/user` refuse during request decoding,
before installation effects: selected `PrivateTmp=yes` supplies different temporary
directories, and `ProtectHome=yes` makes the home locations inaccessible. Root-only
original plan/certificate/key/CA delivery inputs are separate; the runtime consumes
their sealed/delivered copies instead. This follows the selected
[systemd255 namespace semantics](https://www.freedesktop.org/software/systemd/man/255/systemd.exec.html)
(accessed 2026-10-22), without disabling protection or adding path exceptions.
Earlier stopped-only evidence grants no startup admission for a hidden path.

The exact closed Source release template lives in the infrastructure policy,
with an independent complete byte vector. Rendering binds only future generation
program, binding reference and State/participation writable paths. It does not
publish a unit or claim loaded protection. Native preflight requires actual
Ubuntu24.04/systemd255/cgroup-v2, readable required controllers and writable ext4
future ancestors traversable by the future non-root account. Local files/systemd
NSS and both fixed account/group queries exclude foreign existing identities.
The manager's complete unit search path and fixed namespace must be fresh;
unknown existing roots, unit fragments or drop-ins refuse. Original manager
unique owner/Version/UnitPath and caller/manager mount/PID/cgroup namespaces,
boot/start identity, files/access/ancestors are reobserved after I/O. The original
connection never reconnects. The live clock declares root as its writer and
0644 read-only runtime access; its contents, inode and mtime are not frozen.
State retains actual live time acceptance.

Release alone may authenticate and advance its separate floors. Paths are checked
before opening history, so these effects cannot create a requested runtime root.
Initial provenance cannot be recreated once floors exist; candidate preflight
supplies no first-install provenance. Cancellation denies the original caller,
interrupts and joins manager/helpers, joins input borrowers and physical Close,
and retains first failure plus independent cleanup errors. Every future mutation
must perform fresh native admission; no observation is serialized permission.

There is no stages 2–4 cycle. Committed State/source/processresource and original
input custody support component checks now. Exact infrastructure verifier,
artifact producer, fixed-unit policy and source startup are named components,
not a requirement to finish every operator action. A temporary `ardents-next`
consumer can check development mechanisms with independent roots; its pinned
binary and systemd-run are not final installed qualification. Before the first
final installed positive, direct new-owner `ardents-node` Source/Installation
entrypoints and the needed `ardents-control` preparation/inspection, canonical build,
inventory/signing/verification and qualification consumers must be committed for
that exact candidate. This is a partial command/tool cutover prerequisite; other
unrelated commands need not be finished. Old selectors must have explicit
replacement or pre-effect retirement; no old/new runtime import, wrapper,
callback/process bridge or shared root is allowed, including in tests.
After complete command/tool cutover, remove temporary accepting `ardents-next`
entrypoints only after checking their last production/test/build/recovery caller.
Final four-command qualification reruns on the resulting exact artifact; a
component receipt never upgrades itself when a filename changes.

Node work is a dependency graph: common infrastructure binding and Source
coverage first; Forwarding and JOIN then need exact State/Node credentials,
Receiving/Hosting and their distinct producer/join controls. Forwarding supplies
actual protected legs for Descriptor and Introduction client scenarios. Descriptor
adds Store/history/ACK and Introduction adds Receiving birth/Registry/slot floors;
neither requires pretending the other's complete issue is accepted. Issuer adds
prepared quota/key/results, signed profile relation and separate bootstrap
coverage; its integrated issuing/refill consumers need the already accepted
Forwarding path. Tokens for bounded component tests can use genuine existing
Admission producers without claiming installed Issuer. No arbitrary role queue
or Source-as-sixth-Node dispatch is introduced.

### Proposed acceptance matrix and evidence limits

All rows require exact source/tree, command/test binary digests, actual input
inventory, native environment/account/unit/manager/process/cgroup identities,
positive result and causal refusal observations with original failures retained.
Use Ubuntu 24.04/systemd 255/cgroup-v2/ext4 for native installed cases; driver
outside measured leaves. Missing prerequisite is invalid, not a skip. Ordinary
quick/full/hook gates remain necessary; Windows checks cannot prove Linux admission.
Existing native profile names below identify development oracles, not new
registered installed profiles. Future installed tests require explicit checked
profile ownership before execution.

| Invariant / owner | Real positive and causal refusal / independent oracle | Existing source and future required evidence |
|---|---|---|
| Exact program/generation, initial provenance and forward Release floors | Independently encode descriptor/enrollment bytes; verify identical target set and initial running artifact; alter every resource/target/version/pin/metadata input, partial and extra inventory, same-version different resources; prove refusal before native mutation and no floor decrease | `generation_authentication.go`, new Enrollment/Release and Endpoint pair tests are comparison mechanisms only. Source verifier/producer now have independent canonical/signature and compiled-consumer controls; native installed inventory evidence remains required |
| Source original inputs / command | Genuine prepared trust and signed State serve LATEST/BY_DIGEST to actual clients. Replace/mutate/unlink plan/cert/key/CA/ancestor/access before listener and while I/O is held; deny before next effect, retain original cause and held Close, independent original root reopen only after join | Implemented development custody uses `source-process-linux` / `make source-process-check`, `source_*linux_test.go`, Source transport and State race suites. It supplies no installed input/startup binding; fixtures cannot supply those remaining mechanisms |
| Exact installed native binding / Installation | Actual non-root fixed unit reaches independent startup and genuine Source response; substitute executable/argv/UID/GID/groups/unit/drop-in/plan/generation/manager/InvocationID/proc/cgroup and prove no Source/Node effect; root impostor/completion replay refused | Existing Endpoint native process/manager/completion mechanisms are reusable evidence of their own scope only. New Source/unit/account and two complete manager rounds must be observed on final artifacts |
| State, time, TLS / Network and Source | Genuine prepare/sign/accept chain plus explicit external credential trust; actual mTLS verifies CA/name/EKU/key pins/live State clock and signed Epoch materializations. Wrong CA/key/pin/digest/index, stale clock, signed conflict/successor/expiry deny; original guards retained during blocked handlers | `network-signing-linux`, `source-process-linux`, native State/duty race, `source_preparation_process_linux_test.go`, `source_state_linux_test.go`. Independent test authorities/families are not operational bootstrap |
| Process protection / processresource | Actual selected Source or role profile, measured resource use/freshness; changed placement/limits, lost descriptors, real memory/CPU/I/O pressure and event controls deny without revival; missing overload evidence fails test | Source profile separately; `node-process-linux`, `node-issuer-linux`, `node-descriptor-linux`, `node-introduction-linux`, `node-join-linux` / their exact Make targets and `run-*-native.sh` discovery. Each final non-root installed workload needs its own coverage, not another role's PASS |
| Reserve/spend/owned data / Node and neighbors | Actual Forwarding traffic/refill, Issuer issuing/bootstrap, Descriptor publication/lookup, Introduction registration/submission/withdrawal, paired JOIN duplex/terminal. Causal undercoverage/State loss pre-effect leaves spend unchanged; post-spend loss never refunds; retained histories reopen after actual joins | Role `node_*native_linux_test.go` authority/startup/operation/completion cases, Receiving/Hosting/Store/Registry suites. Development proof exists per source receipts; installed role/command/artifact continuity remains required |
| Denial, late output and original joins / every original owner | Hold actual reader/writer/Close/producer/native observation; cancel/Stop/lose authority before and after handoff; retain needed resources/conflicts until actual join and preserve independent late failures through repeated/copied Close. Unavailable notice cannot become joined success | Existing Source lifetime/Node authority+completion controls; new startup/input/manager interlock and installed stop tests must exercise both neighbors. Process Wait alone is insufficient, especially Forwarding's bounded command exit |
| Restart/replacement/interruption/recovery / Installation | Same exact admitted generation explicitly restarts only after known completion; expired credential/State/Release or unresolved producer refuses. Resource-only forward replacement, failures before/after seal, selection, reload, Start, durable archival and ACK; kill original process then Wait, independently reopen exact prefix with fresh proofs, no repeat effects/adoption | Endpoint process-cut receipts are inspected precedent, not infrastructure PASS. Each new phase needs causal cut and uncertainty cases. Crash with unknown domain join may correctly remain repair-required; no automatic restart/reset |
| Final supported operation and claims / Qualification | Exact four-command artifacts, real operator preparation/first-pin/bootstrap receipt and actual independent-family eligibility; two installed Endpoint selected TCP/TLS journey, stop/restart and mandatory protection/interruption gates | Command/build/signing/qualification cutover and final candidate evidence remain required. Explicit QUIC pre-effect refusal/no fallback. Same-controller family labels, signed initial State, prepared pins and local fixture PASS never prove operational independence, bootstrap acceptance or privacy |

The proposed integrity claim is limited: protected bytes/configuration and
original process admission against substituted artifacts or unprivileged local
configuration edits, conditional on intact Release/initial trust and root/kernel
custody; measured by the causal native substitutions and effect/cleanup oracles
above. Compromised root/kernel, dishonest trust delivery, common effective
control, whole-host privacy, anonymity, capacity and power-loss guarantees remain
outside that bounded evidence. No new claim is qualified by this design.

## Hosting domain boundary

`internal/hosting` owns provider-period policy, interface-counter charging, durable
work/termination reservations and shared observation freshness. `internal/node/hosting`
only adapts the domain handle to receiving duties, supplies role envelopes and composes
Admission with capacity reservation. It retains handles until child work joins.
Reservation and shared-handle copies retain one private lifecycle state. Returned
observations own their slice data; closing a shared handle refuses further observations
even while another handle retains a cached observation for the same installed root.
The existing period storage schema and conservative release-cache policy are unchanged.
