# Remaining Route: opaque Introduction delivery and Rendezvous JOIN

Maintained source-backed design for the remaining Route responsibilities. This
design changes no implementation, package registration, execution
selection or acceptance. The Route migration contract and accepted technical
owners remain authoritative. It preserves the complete Route goal, rather than
treating the accepted prefix or registration as the whole domain.

## Assessment and domain question

This is brownfield Go work in one root module with existing rich state machines
and scattered decisions in Node, Route and Endpoint. Route is a core domain:
authenticated path selection, local identifier namespaces, irreversible finite
allowances, concurrent pairing and joined termination justify explicit domain
modeling. Codecs, sockets and durable-file mechanisms need cohesive adapters,
not artificial aggregates, a service split, an event bus or generic repositories.

The question remains: which protected leg and role channel may this operation
use, and who retains its finite transport state until physical completion?
Application composition calls real owners in the required synchronous order.
Eventual events cannot replace revocation, post-I/O authority checks or joined
release. No shared old Context mutex or live root is a permitted shortcut.

## Exclusive owners

| Owner | Rules, state and terminal responsibility | Exclusions |
|---|---|---|
| Route selection | Current eligible public Rendezvous choice, known Node/key/family exclusions, retained choice and preselected bounded alternative | Target authorization, Connection recovery, arbitrary candidate endpoint |
| Route receiving Introduction | Exact live registration; opaque submission/delivery; channel-local lanes/nonces; byte/rate/queue bounds; durable non-reclaim/time floors; joined physical retirement | Recipient private keys, decryption authority, Descriptor ACK/readiness, accepted capsule replay policy |
| Route JOIN | Secret/context/profile pairing, two opposite sides, confirmation barrier, framed data/credit/queues, original bounds and joined termination | Instance TLS authentication, ordered logical Connection bytes/recovery, Job mutation |
| Network | Authenticated current profile/member/duty/time and retained same-observation facts | Route choice or finite token rights |
| Admission | Genuine stock/presentation, receiving verification, finite allowance, irreversible spend | Slot history, pairing, transport byte scheduling, Publication authority |
| Hosting | Actual independent physical work/termination reservations and return | Token authority, transport retirement decision |
| Publication and purpose-scoped Instance | Exclusive accepting Publisher; consumed/generation floors; private recipient, revision/erasure; registration/Descriptor ACK readiness; current/predecessor pair; recipient opening-rate and accepted replay state | Receiving relay slots, generic signing, worker cleanup, logical Connection recovery |
| Reachability | Signed exact-Target Descriptor proof and receiving durable conflict/Store ACK | Local Publisher readiness or Instance authentication |
| Local Application Execution | Exact live qualified Job/operation, synchronous revoke, its descendants and joined local cleanup | Target, token allowance, capsule key/replay or Connection policy |
| Service Connection | Immutable initial binding, permitted recovery generation/deadline, logical stream and Instance authentication | Path choice, registration refresh, Job mutation |
| New command composition | Orders real owners and transfers exact handles/results | A new regression domain, generic successful callback, duplicated policy |

These are semantic owners, not permission for Go imports. Exact non-test callers
and cohesive package/import registration are part of each implemented change.
All identified future domains remain in domain-map.md.

## Tactical consistency boundaries

The receiving registration is a live Route lifetime bound to one admitted TLS
channel and its original duty/revision/expiry. Its delivery entities are local
to that registration: pending count, writer waiters, next even lane, local nonce,
acknowledgement state and finite byte reservation must transition coherently.
There is no independently persisted Delivery repository or durable restored
channel. Slot hash/expiry/time history is the separate durable Route owner
implemented by the registration owner and designed in route-introduction-registration-boundary.md.

JOIN has a bounded pairing root at the Rendezvous. Each live pair retains at
most two opposite side entities with immutable secret/context/profile facts,
their local channel/request identities, setup expiry and confirmation state.
Each side retains its original independently admitted transport limits. Pairing
does not mutate Admission or Hosting: their actual grants/reservations remain
independent owners transferred through application/physical composition.
The pair owns stop/join before return of all its physical borrowers.

Values include canonical slot/revision/expiry, sealed capsule, channel-local
request nonce, local lane, JOIN secret/context/side, public retained recipient
and original bounded allowance. These labels are design language, not a request
to create exported structs or packages for each value.

An initiating acquisition retains the exact original Source handle. A Publisher
JOIN acquisition retains both its exact live Responder handle and that handle's
exact Source issuer, including when existing stock is used. Both identities
remain bound through stock preparation, recipient selection, presentation and
final handoff. Retirement/replacement of either invalidates new effects
synchronously; a late stream joins and releases only that original acquisition.
Success transfers stream plus acquisition until joined transport close.
Replacement cannot receive the old completion or acquire its authority.

## Opaque Introduction operation

The submission channel is purpose 5/class 1, independently authenticated and
admitted for the same Introduction duty. An active purpose 4/class 3 registration
retains the delivery channel. A new submission token cannot enlarge its original
Registration 1 MiB/600-second allowance or reclaim an old slot.

The fixed submission OPERATION is 4096 bytes: operation 4, 32-byte local request
nonce, 474-byte sealed capsule and 3589 zero padding bytes. The capsule contains
the 80-byte slot/revision/expiry/delivery-nonce header, 32-byte encapsulation,
two-byte length and exactly 360 ciphertext bytes. Recipient plaintext is 344
bytes. HPKE info and associated data follow the exact protected-route-protocol
grammar; use the selected maintained cryptographic mechanism, never a new one.

Route forwards the sealed bytes unchanged. It does not inspect Target, join
secret, Rendezvous or Connection facts. Delivery uses a newly generated nonce
local to the registration, distinct from both source request and capsule delivery
nonces. The source receives its own nonce in its RESULT.

Delivery reservation includes OPERATION, RESULT, CLOSE and every frame header
before dispatch. At most sixteen deliveries, including writer waiters, are
pending. Both admission and actual dispatch are bounded to four per second by
the receiving Route owner. Allocate the next strictly increasing even lane only
while holding the frame writer; no OPEN or lane/nonce copying between channels.
Retain withdrawal capacity separately. No byte or token refund follows a failed
dispatch, invalid ACK, cancellation or joined close.

Exactly one fixed 16384-byte RESULT must match that delivery's nonce and lane,
have empty result payload and satisfy original expiry/currentness. Introduction
sends CLOSE with the same outcome and joins the child. Missing/invalid/late ACK
means unavailable. A failed/expired write that cannot finish the protocol retires
the registration without reclaim. Physical failures remain in its terminal
result; a transport EOF is not a successful capsule acknowledgement.

## Genuine successful recipient, and what follows later

A REGISTER ACK establishes Route slot transport. A Descriptor Store ACK is a
different durable receiving result. Local Publication commits accepting state
only after the latter ACK and a final still-live authority/recipient check.
Neither ACK independently grants Service readiness.

Before returning successful capsule delivery, real recipient composition must:

1. Retain an exact live qualified Publisher Execution operation and bounded
   exchange; reject drain/withdrawal/currentness loss.
2. Select the actual accepting current/predecessor registration and retain the
   exact live Publication/Instance generation with acknowledged Descriptor.
3. Independently verify Target/Network/profile/Publication digest/Credential,
   slot/revision/recipient public key and original expiry.
4. Reserve actual recipient opening/replay capacity, decrypt with the live
   non-exporting Instance recipient and verify the complete plaintext facts,
   including current Rendezvous duty and initial/recovery bounds.
5. Create/check the immutable Connection binding; recovery cannot change its
   Target/Instance/profile/safety tuple or extend the original deadline.
6. Recheck exact Job/Publication/recipient/bounds after I/O and retain the accepted
   delivery nonce until original registration expiry plus sixty seconds.
7. Prepare the genuine separately owned Responder forwarding prefix for the
   exact accepted Rendezvous, retaining its exact Source issuer as well and
   rechecking the live binding and both handles before handoff.
8. Retain initial logical recovery state where required, then emit the actual
   matching fixed RESULT and join matching terminal CLOSE.

Accepted replay history is not rolled back when subsequent Responder or RESULT
work fails. Worker loss cannot clear still-retained recipient authority history.
Publication retirement stops new acquisitions before joining and erasing its
private recipient; predecessor overlap can shorten, never renew its cutoff.

Paired JOIN and Instance-authenticated Service TLS occur after this delivery
RESULT. They need not be forged to test this narrower genuine recipient result,
but are required before claiming a Service Connection or exposing Application
bytes. Until the real new neighbors exist, mechanical transport completion is
explicitly component evidence. A successful substitute, boolean, arbitrary ACK
callback, raw public key or old consumer cannot qualify the missing integration.

## Rendezvous choice and JOIN

Choose uniformly among current eligible data-join duties after all known
Node/key/family conflicts. Retain the public choice only in the same local
Isolation Context for at most thirty minutes/earlier duty expiry. Select at
most one eligible alternative at the same time. Failures cannot redraw, expand
the set or let a capsule supply an arbitrary literal endpoint. Each fresh
Connection uses a fresh secret and terminal channel; completed pair state is
not reused. Interior selection independence remains an unresolved separate
interpretation: this design accepts no new randomization policy for it.

Each opposite side reaches the exact State-selected Rendezvous through its
Source or Responder prefix, with independently authenticated fresh role TLS,
HELLO and genuine class-2 ADMIT/ACCEPT on lane zero. JOIN operation 5 is exactly
4096 bytes, on local odd lane 1, once, without OPEN. It carries secret[32],
side byte (User=1/Publisher=2), context[32] and deadline u64, with exact padding.
Unknown side, second JOIN/other child lane, malformed bytes, wrong profile or
context, duplicate/third side refuse without replacing a retained side.

Unpaired expiry is the minimum of original setup deadline and ten seconds from
reservation. Only a real opposite-side match permits the fixed 16384-byte empty
accepted RESULT on each local lane with its own request nonce. Both successful
RESULT emissions must establish the pairing barrier before forwarding data.
There is no mapping of one channel's request nonce into the other channel.

The accepted framed stream starts with the original 64 KiB receive credit and
uses BYTES <=16384, CREDIT, EOF and CLOSE within its original admitted reserve.
Credit returns only after bounded consumption. Count setup/control/data/headers
and terminal traffic. Setup expiry does not shorten permitted paired data to
ten seconds; original HELLO/class-2/parent/State/duty/Work Safety bounds remain.
EOF closes one direction; CLOSE stops effects and joins both directions.
Received bytes survive a following raw transport EOF within original bounds,
then end with unexpected closure unless an inner verified CLOSE was received.

Later ADMIT is forbidden on JOIN, including lane zero. Check channel eligibility
before requesting a Hosting refill or spending a token. Receiving.Refill remains
available only for the accepted forwarding-parent contract under ADR-0085.
The old positive JOIN-replenishment test is not an invariant for the new owner.

Source JOIN and capsule submission run concurrently after their independent
stocks/presentations are prepared and durably marked. This orchestration belongs
to the real initiating operation composition, not to Rendezvous or a shared
Route/Connection root. Route exposes only a protected framed stream; exact
Instance Service TLS and logical continuity belong to Service Connection.

## Physical retirement

All owners first deny new effects/joins and invalidate exact acquisitions, then
cancel/interrupt all dependent work before waiting. Join readers, writers,
expiry callbacks, children and Carrier borrowers; retain pending bytes until
consumption/expiry/join; return actual reservations and close independent roots
after their last borrower. A Done notification or timed-out wait is not physical
completion. Keep the retained terminal result, including late physical failures,
and distinguish ordinary peer refusal from cleanup failure by actual provenance.
No actor closes or mutates a replacement owner to finish an obsolete operation.

## Source inventory: collect, split and leave

| Current source | Route behavior to recover | Behavior left with another owner |
|---|---|---|
| internal/node/introduction/delivery.go | Exact opaque relay, byte/rate/pending/writer bounds, local even lanes, RESULT/CLOSE and withdraw | Server composition/old Admission root must not be imported |
| internal/route/capsule/{submission.go,capsule.go,crypto_linux.go} | Inspected canonical envelope and selected crypto mechanism | Instance private recipient and authority remain Publication; parsed plaintext is not authority |
| internal/route/closed_join_pairing.go; closed_join_stream.go; client/closed_join_client_stream.go | Pairing/confirmations, frame budgets, queues, retained bytes and joined termination | ClosedOuterBridgeLane and old receiving callbacks cannot transfer |
| internal/route/closed_join_replenishment.go; internal/node/join/listener.go | Evidence of the forbidden old JOIN-refill path to reject | No new JOIN refill mechanism or successful old refill oracle |
| internal/route/client/closed_terminal_recipient.go | Current public duty verification and known conflicts | Old second-candidate refusal is not uniform Rendezvous choice |
| internal/endpoint/source/lifecycle.go; responder_prefix.go; join_service.go | Exact Route acquisition, peer/presentation/final stream identity and cleanup | Job admission, Connection orchestration and old Context mutex |
| internal/endpoint/introduction_accept.go; introduction_exchange.go; introduction_dispatch.go | Genuine consumer ordering to preserve through new composition | Publication/Instance authority, recipient replay/rate, exact Job and Connection/recovery decisions |
| internal/endpoint/introduction/{registration.go,pair_lifecycle.go,admission.go} | Boundary evidence for physical registration handle only | Local accepting state, refresh/overlap/withdraw barrier, recipient history |
| internal/service/instance; publication; reachability; connection | Named provenance of real prerequisite contracts | Whole implementations stay with their proper new domains |

The old shared admission-root slot lease is superseded for new Route by its
independent durable floor design. The Admission source inventory distinguishes the old contradictory JOIN-refill
path from accepted forwarding-parent refill under ADR-0085. New Receiving has
refill component behavior; the current new Route session still rejects lane-zero
ADMIT and has no refill caller. That missing forwarding-parent wire behavior
remains a full Route obligation. JOIN must refuse it before reserve or spend;
component Admission passes do not prove accepting Route refill integration.

## Independent acceptance and regression oracles

Opaque delivery: independently assembled exact bytes, unchanged sealed envelope;
three distinct nonce namespaces; out-of-order submissions cannot reorder even
wire lanes; writer waiters count toward sixteen; blocked writer then release
cannot burst beyond the actual-dispatch rate; withdrawal budget is retained;
invalid/duplicate/late ACK or unknown lane cannot succeed; no reclaim/refund on
any failing phase; State/expiry/withdraw/cancel during I/O prevents late success.

JOIN: independent nonce/side/context/padding oracles; true opposite-side pair;
both RESULT transitions before data; third/duplicate does not replace; expiry
before pairing; paired data after setup but before original bound; credit/queue
and aggregate exhaustion; denied JOIN refill with no extra reserve/spend; EOF
retains prior bytes then unexpected closure; cancellation with opposite writer
blocked; exact acquisition lost during durable presentation/final handoff;
late physical/release failure retained until actual return.

Cross-domain controls exercise signed current Network, genuine tokens and
independent durable roots, real Admission/Hosting ordering/no refund, reopening
and uncertain writes, races, both TCP/TLS and QUIC and compiled real consumers.
Component injected failures are separate from genuine success; test-only
reachability is never package/product acceptance. Required quick/full gates
must match final source. Installed confinement/privacy remains separate proof.

## Verification limits and dependency graph

The registration implementation baseline is `ca0767ebaa02513166d6bcb14cd55836a45ae661`; the domain map records its source and evidence separately. This design does not accept unimplemented behavior.

Root independently read the selected protocol sections, accepted ADR-0083 and
ADR-0085, Node/Endpoint technical owners, exact old delivery, recipient acceptance
and Responder sources, JOIN stream/refill and terminal selection. Bounded agents
inventoried the remaining named paths read-only; their reports are not root
reproduction. No runtime tests or acceptance follow from this design. A separate
source-hash receipt identifies the read inputs; HEAD alone is not dirty-source
identity.

Protocol dependencies are acyclic when authorities are kept distinct: genuine
Network/Admission/Hosting enable Route transport; receiving registration enables
Publication's registration step; Reachability's durable Descriptor ACK plus
Instance/Execution allow Publication readiness; those plus Connection binding
and real Responder permit genuine successful recipient delivery; paired Route
JOIN plus Instance authentication permit Service Connection and Application I/O.

Route JOIN can be implemented and genuinely exercised at its protected transport
boundary without pretending to be Publication or Service Connection. This does
not discharge complete capsule-recipient proof. Preserve every broader Route
obligation and missing genuine neighbor explicitly; prepare the needed real
owner designs before assigning dependencies. This design starts no additional
implementation and requests no repeated Product Owner selection or permission.

