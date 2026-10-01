# Network State, Entry, Route, and Node

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
| internal/resource | Check selected process placement and measure process/cgroup pressure through a process-local Guard. Separately own the initialized durable shared Hosting period, interface-counter charging and work/termination reservations. | State or Node authority, admission, listener shutdown, forgiving an outstanding reservation on handle close, or a claim for unsupported platforms. |
| internal/entry | Own the protected Endpoint's durable closed Entry sets: select exactly two State-current members per adjacent Role Domain before use, revalidate a selected member, retain the generation floor, and refuse legacy-root substitution. The Invite subsystem — the `entry recipient/import` operator commands and their older Invite root — is retired by ADR-0106; existing Invite roots stay on disk byte-for-byte with no reader, converter, or deleter. | Complete Route selection, receiving Entry admission, carrier choice, User identity, or any read, conversion, or deletion of a legacy Invite root. |
| internal/route | Implement the closed v3 Node Carrier/wire used by `internal/node` through `OpenClosedNodeCarrier`. The former aggregate Interactive User Route v2 runtime and its whole v2 execution closure (Attachment, EndpointTransitBinding, EntryBinding, credential-relay, Introduction slot/outcome, LegBinding, and Transit Grant verifier files) are absent under ADR-0093. What remains is only the retired v2 `Profile` identity used for Node typed refusals; the sealed Introduction v1 grammar and the publication v1 Introduction instruction codecs are retired by ADR-0094, which supersedes ADR-0035. Nothing provides a second supported Route. The [package map](../development/package-map.md) records the current consumer boundary. | Reintroducing the removed User-route composition as a maintained product path or treating its removal as successor-network readiness; candidate ranking, carrier policy/fallback, H3 compatibility, peer runtime, Node profile, or durable State/Duty/credential-journal writing. |
| internal/node | Run one bounded current closed Node duty from authenticated admission through listener readiness, pressure reaction, drain, withdrawal, and a bounded terminal cleanup outcome. All five old native duty engines are absent; their plan stanzas remain only at the command refusal boundary. | State-root authority, assignment creation, an old native duty listener, or a separate probe runtime. |
| internal/node/authority | Borrow current authenticated closed State views and project one exact receiver or shared peer; verify selected-profile role tokens for the Node duties. The caller retains its duty admission and host/spend reservation. | State-root custody, role selection, process pressure, a receiving spend ledger, or an issuer key. |
| internal/node/forwarding | Own the closed forwarding listener, receiving spend root, pool, sessions, child links and joined drain; check exact State-selected next hops and bootstrap adjacency before dial. | State-root custody, process pressure, unselected fallback, Route wire or Carrier TLS implementation. |
| internal/node/hosting | Own the opened shared Hosting handle, sampler and late close; bound class-1/3 control envelopes and verify class-2 reserve-before-spend for forwarding and JOIN. | Durable provider-period ledger custody, process pressure decisions or receiving-role lifecycle. |
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
| [`Wait` and `Close`](../../internal/network/state/lifecycle.go) | Report terminal background failure; cancel and join accepted work, close the durable root, release the serving Source duty, and retain one cleanup result for all Close callers. | `resource` supplies pressure observations; State retains supervision and cleanup ownership. |

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
spent selector is not replayed. At the deadline, every unresolved slot becomes
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
from [R-169](../research/records/r-169-source-duty-lifetime.md). Serving Duty
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
then owns copies for its one configured TLS client or listener. Network State
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
to the caller's total exchange context. Cancellation closes an established
connection and discards any partially received Object Digest; State cannot
start a BY_DIGEST fallback from a canceled exchange. A live partial response
may retain its transport-observed selector for the one bounded fallback.

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
