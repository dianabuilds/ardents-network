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
| Source/Node runtime private TLS key | Explicit operator-provided purpose-specific key, not the offline signing directory. Fresh protected delivery to runtime-owned 0600 file under a root-controlled, runtime-readable but non-writable directory; no symlink/hardlink, public export, argv/env secret, generic signer or root-owned-0600 handoff to non-root. Original file/ancestor/access/bytes and matching public key remain pinned through join. Credential replacement requires explicit fresh invocation after old join |
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
`initial`, `admitted`, `account-preimage`, `account-auxiliary-preimage`,
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

The complete initial transcript contains exactly 112 canonical records for this
closed inventory. Independent root-only readback consumes every phase in order,
checks intended paths/bytes/access and matching born/ready identities, binds the
original history commitment, and reobserves its own Source resource descriptors.
The stopped creator performs this readback after `stopped-verified` and retains
its writer lease until all native observers/helpers join. The reader neither
borrows that lease nor grants creation, current account, Release or Start rights.
Account database records retain historical helper evidence; they do not freeze
foreign host databases across operations. A later start separately admits the
current exact principal. State/participation contents remain semantic-owner data.

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
