# Remaining Route: terminal transport, opaque delivery and Rendezvous JOIN

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

### Installation selection and exact Source lifetime

The protocol selects Entry sets once per installation and adjacent Role Domain;
Interior sets belong to an Isolation Context or Service role. At the registration baseline, the prefix-only
composition coupled those lifetimes: `selection.Open` opens the
exclusive Entry root and `Owner.Close` closes it; `startRoutePrefix` closes
selection and Hosting with the physical prefix. Two independent calls with the
same Entry root cannot compose one Publisher's simultaneous Source/Responder.
A second installation root would instead change the retained-selection contract.

The [implemented JOIN composition](../technical/successor-route-join.md) now
separates these four owners without adding another domain:

| Owner | Lifetime and narrow responsibility |
|---|---|
| Installation Entry selection | Open one exclusive Entry root; serialize retained public sets and floors across activated adjacent domains; lend domain-bound selection access. Borrowers cannot close the root or redraw retained sets |
| Context/role selection | Own its Interior root, exclusions and retained pair; borrow installation Entry access. Closing it returns that borrow after its physical users join, without closing sibling selections |
| Source/Responder lifecycle | Own exact opening reservation, published handle, synchronous seal and outstanding borrowers. A Responder JOIN acquisition retains both its exact Responder and original Source handle. No acquisition can look up a replacement implicitly |
| Physical prefix | Own actual admitted channels, readers/writers/children and physical stop/join. Its close returns its reservations and borrows; it does not close an installation owner still used by another role |

The installation owner must obtain current Network facts and installation-wide
exclusions independently of the first borrowing context's lifetime. Context
exclusions are additionally checked before effects; they cannot rewrite shared
history. No common private context history or cross-role trace follows from
sharing this public Entry owner within Route. This does not permit sharing a
live root with old code or another domain. Preserve retained bytes, expiry,
conflict and uncertain-write behavior; no new selection or resampling policy.

A Source lifecycle may exist before opening a physical prefix, but is then
unready. In inspected `internal/endpoint/source/lifecycle.go`,
`AcquireJoinLocked` requires a live handle with non-nil prefix, and
`CurrentLocked` checks that exact published handle. The contract likewise
binds consumers to the exact published Source opening. Do not turn a newly
constructed lifetime, supplied identity or unused `Issuer *Prefix` field into
successful Source authority. Demand-driven physical opening is valid; an idle
Publisher does not open Source merely to inspect an incoming capsule. Before
the accepted JOIN handoff, retain the genuine original Source/Responder
acquisitions, including when stocked tokens make fresh issuance unnecessary.
Admission stock and Execution/Publication authority remain their own owners.

Opening completion publishes only into its still-current reservation after
caller/currentness checks; otherwise it joins its own result. Seal synchronously
denies new effects/acquisitions, interrupts dependents, joins openings and
borrowers, then returns resources. Copy borrower identities under the owner
lock; do not hold it during physical I/O or join. Repeated close retains one
terminal result; cancellation of a waiter cannot release another operation.
Close installation selection only after all role/context borrowers finish.

Hosting remains one real provider-period budget owner at its existing scope.
Composing another prefix cannot reopen the same lease independently, create a
second budget to multiply capacity, or close the borrowed budget. Each physical
prefix receives its own finite reservation from the real shared budget; only
the composing budget owner closes that root after every borrower joins.

Acceptance needs simultaneous Source/Responder in one installation with one
Entry lease and separate Interior owners; closing one cannot invalidate its
sibling. Verify unchanged retained sets across close/reopen, late opening after
replacement, stocked-token Source loss, original-caller cancellation before
handoff, close during opening/I/O, exactly-once return after join and retained
cleanup errors. A second installation, identity-only Source or additional
physical prefix created solely to satisfy a field is not the positive oracle.
The implementer chooses cohesive concrete APIs inside existing registered
owners; these lifetime requirements do not prescribe speculative interfaces.

For the bounded holder console consumer, one console lifetime may compose one
explicit local Route context with fixed role/configuration and an independently
owned Admission holder. It is not a qualified Execution session, Local Grant
or Service Isolation Context merely because the console exists. The context
retains its Rendezvous choice/alternative and original validity across physical
prefix close, idle retirement, failed open and reopen. Selection belongs to
the context owner in Route, not a fresh command helper call for each prefix.
Admission state must not acquire Route's private selection policy.

Prefix closure stops that physical generation; context closure seals further
openings, joins pending/active prefixes and acquisitions and then retires local
selection. A failed cleanup remains a terminal admission failure; prefix-close
cannot erase it and reopen. Neither physical reopen nor a changed Source pointer
resets choice, time floors or the original retention deadline. Validate the
new physical leg and all known exclusions against the retained choice before
effects; a conflict refuses instead of redrawing. Expired selection refuses
under this bounded consumer rather than silently manufacturing a fresh context.
Console EOF/close/cancellation/lifetime expiry retire the context. A later
genuinely new console does not reset installation Entry floors or durable
Admission history. Publisher consumes its genuine incoming choice; this seam
does not authorize it to choose an unrelated Rendezvous.

## Issuer and Descriptor Control transport

These are additional full-domain transport obligations from the current
[Admission contract](../technical/private-admission.md#bootstrap-without-a-token-cycle),
[terminal grammar](../technical/protected-route-protocol.md#terminal-payloads-and-private-reachability)
and [Reachability contract](../technical/private-reachability.md).
Recording them selects no additional implementation slice and does not enlarge
the admitted JOIN slice. They cannot be lost when accepting delivery/JOIN/refill.

**Issuer bootstrap and ordinary issuance:** Route owns the confidential path
to the exact current issuer, purpose 1/operation 1 grammar, immutable
issuer-bootstrap child restriction, bounded transport and physical completion.
Admission owns permission verification, the two-batch bootstrap limit, exact
pending batch/retry, whole-class reservation, quota/debit, signing and retained
result. A successful issuer response cannot relabel a restricted child.
Retire bootstrap channels; private work uses fresh children under genuinely
admitted parents. Ordinary issuance uses a fresh terminal TLS channel and real
class-1 Control presentation/spend. It retains the exact admitted prefix and
marks presentation durably before token bytes can leave the holder.

Apply the existing per-hop bootstrap ceilings: 128 KiB/10 seconds per lane,
four live lanes per adjacency, sixteen per duty, 256 KiB queued work and duty
output 1 MiB/minute with a 128 KiB burst. New peer identities cannot multiply
the aggregate bound. Preserve the 50-byte Node OPEN restriction propagation and
49-byte Endpoint OPEN distinction. Admission request encoding and exact retry
identity remain Admission decisions; Route carries the selected padded bytes.
The local command `admission_current_issuer.go` calls genuine `IssueCurrent`
with supplied batch bytes, but does not establish a confidential network issuer
exchange. Full acceptance needs real new holder and issuer consumers through
both selected Carriers, including ordinary issuance after bootstrap retirement.
No Publication or Service authority substitute is needed for this boundary.

### Issuer exchange consistency and implementation seams

The confidential exchange has distinct cooperating owners, not one shared issuer
aggregate. Route retains the exact Source selection, physical generation,
bootstrap restriction and terminal channel. Admission Stock retains the exact
pending blind batch, permission, original issuance kind and delivery binding.
Receiving Admission owns ordinary class-1 spend. Issuing Admission owns the
separate quota, private key and retained-result transactions. Hosting retains
the actual reservations until Route joins every borrower. None of these owners
may substitute its identity for another owner's authority.

Existing `stock.Owner.Begin`, `Attempt.Request`, `Complete` and `Discard` already
separate the pending batch from transport. The exchange runs outside Stock's
mutex. Retain its exact operation and binding through I/O and check them before
finalization; no later Source generation can receive an obsolete result. A
same-process exact retry keeps request bytes, request ID, issuance kind,
blinding state, delivery binding and original deadline. It is not an automatic
retry or permission to recreate lost blinding state after restart. A failed
transport does not return reserved permission quota or presented tokens.

`issuer.IssueCurrent` already checks genuine current Admission facts before
debit, result generation and final export. It opens independent quota, key and
result roots, retains cleanup outcomes and suppresses export on cleanup failure.
The network receiver uses that owner, including its exact retry/conflict rules;
Route neither copies its quota/signing logic nor obtains private keys. Concurrent
requests require bounded operation admission and the actual exclusive root
ownership. No unbounded goroutine/queue or successful callback bypasses a busy,
uncertain or unavailable issuer. Transport kind comes from the authenticated
ordinary/bootstrap channel state, never an untrusted request flag.

The existing holder prefix requires forwarding tokens at Entry and Interior.
It cannot establish bootstrap before those tokens exist. Bootstrap therefore
has its own finite transport lifetime using the same retained selection and
verified Network owners. Entry derives restriction 1 from its real bootstrap
reservation; each Node child retains it before inner TLS allocation and carries
it onward. Retiring this lifetime joins children, readers, writers and capacity
before opening fresh genuinely admitted parents. Obtaining tokens does not
change any old child's restriction or permit a private OPEN through it.

Ordinary issuance retains the exact admitted Source prefix, opens a fresh
purpose-1 terminal and presents a genuine class-1 token. Its original allowance
is 64 KiB/30 seconds including admission and terminal frames, shortened by all
parent/authority bounds. Bootstrap remains 128 KiB/10 seconds at each hop.
Neither path replenishes its dedicated issuer terminal or extends its original
deadline. Public-evidence bootstrap, Name and Descriptor operations do not
become accepting paths merely because issuer bootstrap is implemented.

The real consumer must obtain verified stock through the confidential exchange,
retire bootstrap, then use that stock for an admitted prefix and ordinary
issuance on TCP/TLS and QUIC. Console-supplied batch/result bytes remain an
offline operation, not evidence of this path. Issued/exhausted/withdrawn/
unavailable responses retain the exact Admission result encoding and fixed
16,384-byte terminal RESULT; transport failure is never a fabricated issued
result. Correlate only the terminal's own request nonce; do not export permission,
request, Target or cross-hop identities into diagnostics.

Independent controls include exact 49/50-byte OPEN grammars; missing/unknown or
Endpoint-injected restriction; a valid private token on a restricted child;
mixed restricted/ordinary children sharing one Node Carrier; all per-adjacency,
per-duty, queue, output-rate and burst limits; both permitted bootstrap batches
and refusal of an additional batch without a fresh debit; same-kind exact retry
and changed-digest/kind refusal; lost result and durable reopen; cancellation or
State loss across reservation, debit, signing, output and finalization. Preserve
fractional rate credit on refused sends. Observe the actual public consumer and
original caller, not only a lower-level helper. Physical failure controls retain
late writer errors and every reservation until joined termination.

### Descriptor Control

Route selects the current resolution duty from coherent
Network facts, authenticates a fresh purpose-3 terminal TLS channel, composes
real class-1 admission and carries one lane-zero lookup (operation 2) or publish
(operation 6). Lookup body is 4096 bytes, publish body is 16384 bytes and RESULT
is 16384 bytes under the exact canonical grammar; a complete Descriptor is at
most 15000 bytes before padding. A decoder or transport success grants no proof.
Reachability verifies exact Target/Network/profile and owns lookup/history and
the receiving Store's durable conflicts and ACK. Genuine signed publication
inputs and accepting readiness remain with Publication/Instance. Missing new
neighbors must remain explicit; no unchecked storage callback or fabricated
ACK qualifies this boundary. Purpose 2 Name remains reserved and refused.

Independent acceptance controls cover canonical padding/body bounds, wrong
purpose/duty before effects, immutable restricted children even with a valid
token, exact batch retry without another debit, presentation/reserve/spend
ordering without refund, source replacement/cancellation across I/O, bootstrap
aggregate ceilings, Descriptor conflict/stale/oversize refusal, Store commit
before ACK and joined physical release. Evidence must use genuine new owners,
separate roots and both TCP/TLS and QUIC. A transport-only check does not prove
Store authority, Publication readiness or complete Service integration.

## Forwarding-parent replenishment

This boundary implements the existing [ADR-0085](../adr/0085-bound-forwarding-replenishment.md)
and [byte-accounting contract](../technical/protected-route-protocol.md#flow-control-and-scheduling).
It adds no quota, retry, threshold, idle maintenance or deadline policy.
The admitted forwarding parent is the consistency owner: its immutable HELLO,
TLS exporter, peer/purpose, original deadline, children and cumulative accounting
survive a refill. A new Grant is another finite reservation lifetime within that
same parent, not a replacement channel or permission to redraw a prefix.

The [implemented prefix owner](../technical/successor-route-prefix.md#requested-forwarding-parent-replenishment)
now describes the genuine receiving refill and holder `Prefix.Replenish` paths.
Receiving Admission keeps original-Grant/deadline checks and irreversible spend;
the command connects its refill reservation to the exact original Hosting owner.
This design states their required boundary, not a replacement for source-matched
regression or evidence of complete Route delivery.

Route checks the exact admitted forwarding state before any refill callback,
Hosting reservation or token spend. Only a complete class-2 ADMIT on lane zero
is eligible. Dedicated JOIN, Registration, other terminal purposes, outer Node
Carriers, restricted bootstrap channels and child lanes gain no such right.
The caller retains its original live prefix and current selected recipient;
fresh stock is durably marked before token bytes leave. Only requested work
drives replenishment. No timer, idle pool or background stock task may trigger it.

Debit all 371 bytes of the ADMIT header/body from the old parent allowance
before its effect, with a positive remaining reserve as required by Admission.
Verify the genuine token and reserve actual additional Hosting work capacity
and any termination capacity not already held before durable spend. After
successful spend and currentness checks, replace remaining reserve with exactly
32 MiB. Keep cumulative usage and all concurrent input/output debits accounted
at one explicit transition; do not implement this by adding 32 MiB or resetting
usage history. The matching lane-zero ACCEPT and all following traffic are
also charged. Refill does not alter child lanes, credit windows, peer, purpose,
selection, original deadline or another hop's allowance.

Route serializes each parent's control exchange and admission transition using
bounded existing control accounting; the reader and writer must preserve child
progress, termination priority and channel limits. No unbounded control workers,
second reader or I/O under a domain-state mutex is introduced. Recheck original
caller, State/duty and parent retirement after presentation, reservation/spend
and ACK I/O. A lost ACK cannot cause automatic token replay, another spend or a
successful substitute result. Keep the uncertain operation's refusal and cleanup
explicit; no deadline extension, refund or resurrection follows.

Each returned Admission Grant and Hosting reservation has an explicit owner
until that parent's physical borrowers join. Preserve the original live Grant
required by Refill. Successful refill must not drop the new Grant: the current
retirement adapter returns only callbacks whose release was requested. Refused
post-reservation work must follow the same joined return path. Do not release
capacity early, refund a token, reopen a budget root, multiply a provider period
or indefinitely accumulate completed control attempts. Bound retained active
reservation state through actual capacity and original lifetime; compact completed
state while retaining terminal failures.

The receiving composition uses the same genuine receiving Admission and Hosting
owners. The holder consumer replenishes its actual retained forwarding channel
and demonstrates subsequent real child traffic within the original lifetime.
It must expose no peer-supplied arbitrary destination, fake Service operation or
test-only product command. Existing finite operator Hosting inputs do not prove
carrier overhead or installed provider billing; additional reservation must cover
the requested envelope under those same inputs, not silently reuse already held
capacity as a second allowance.

Independent tests account for ADMIT and ACCEPT headers/bodies, distinguish
replacement from addition, preserve concurrent debits and original child credit,
and prove the same channel can carry real child traffic after refill. Exercise
wrong class/purpose/lane and exhausted parent before reservation/spend; replay,
State change or expiry during I/O; actual capacity refusal; loss of ACK after
spend and reopen without refund; cancellation and late physical failure; repeated
Close and exactly-once release after all children join. Forbidden JOIN refill
remains a negative integration control. Use signed Network, genuine tokens and
separate durable roots on both Carriers and compiled real consumers, followed by
full regression of all new owners and the required repository gates.

This boundary needs no fabricated Publication, Reachability, Execution or
Connection success. Those domains remain prerequisites for their own broader
journeys. Full Route still includes opaque delivery and issuer/Descriptor Control.

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
path from accepted forwarding-parent refill under ADR-0085. New Route now has
an explicit holder refill caller and a genuine receiving path described by the
prefix owner. JOIN must still refuse refill before reserve or spend; component
Admission passes alone do not prove that Route integration or its caller lifetime.

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

The registration baseline is `ca0767ebaa02513166d6bcb14cd55836a45ae661`; paired JOIN and its installation/context lifetimes are implemented at `df94a181810548f977cd5aa03a24cd96447c3042`. The domain map records both results and their evidence limits. The remaining design does not accept unimplemented behavior.

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

