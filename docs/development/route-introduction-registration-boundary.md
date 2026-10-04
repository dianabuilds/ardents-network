# Route Introduction registration boundary

Source-backed design for the isolated Route domain. This is an owning design,
not a second execution ledger, accepted implementation or Service readiness
claim. The Route migration contract and complete domain map remain mandatory.

## Accepted responsibility and scope

Route owns the receiving Introduction duty, one exact admitted registration
channel, REGISTER/WITHDRAW transport, slot occupancy and durable non-reclaim/time
floors. Registration proves possession of this bounded transport, not Service
Authority. Publication owns the Instance, registration pair/revision readiness,
recipient keys, refresh and withdrawal decision. Execution owns the local Job.

Registration is a rich state/lifetime responsibility within the Route domain,
not a separate domain or a new Admission aggregate. Canonical byte codecs and
exclusive durable storage are mechanisms under that semantic owner.

The next bounded behavior is actual registration and owning-channel withdrawal
through a retained Introduction prefix using new signed Network observations,
genuine class-3 Stock/presentation/Receiving, Hosting and real TCP/TLS plus QUIC.
It must not manufacture successful capsule acceptance, Publication readiness,
Descriptor authority, Instance authentication or logical Connection recovery.
The complete Route goal also retains submission/delivery and JOIN; completion
of this behavior does not complete that goal.

## State, authority and resource owners

| Owner | State and invariant | Observable boundary |
|---|---|---|
| Receiving Introduction duty | Exact Network/profile/Node/duty identity; finite channel/slot metadata; admission-open and retained terminal result | Refuse stale duty, stop all producers before joining any; close roots only after physical borrowers join |
| Registration channel | Original accepted allowance/deadline, exact TLS channel, request identity, slot/revision/expiry, serialized writer and termination state | One REGISTER; withdrawal only on the same channel with exact slot/revision; no reconnect transfer |
| Slot occupancy | Vacant -> reserved -> acknowledged -> retired; original expiry remains fixed | Retire removes live delivery authority but does not erase the non-reclaim record |
| Route durable slot history | Slot hashes, original expiry, monotonic time floor and exact public duty binding; exclusive root lease | Durable claim before registration ACK; damaged, ambiguous or rebound history refuses |
| Admission | Token verification, class policy, irreversible spend and genuine bounded Grant | Capacity before spend, spend before acceptance; no refund after a failed REGISTER/ACK/withdraw |
| Hosting | Provider-period budget and finite work/termination reservation | Route retains its actual return through joined physical completion; no second budget |
| Network | Authenticated current observation, time/conflicts and retained duty | Reobserve the same identity at effect and post-I/O boundaries; no cached approval or successor rebinding |
| Publication | Instance authority, readiness, refresh and current/predecessor pair | A receiving slot ACK cannot declare a Service accepting work |

Model the in-memory registry and durable history as cohesive consistency owners;
use synchronous ordering for claims/retirement. A durable transaction may be
performed by an adapter without exposing its lease or journal to Admission.
Do not wait for physical I/O while holding an owner lock that a completion needs.
No event bus, generic repository or speculative interface is required.

## Canonical transport and immutable bounds

HELLO Purpose 4 requires the selected Introduction delivery duty and genuine
Registration/class-3 admission. Submission Purpose 5 and class-1 are separate
operations; forwarding Purpose 7 is not a terminal-purpose shortcut.

REGISTER operation 3: operation byte, fresh channel-local request nonce[32],
slot[32], revision u64, expiry u64 and canonical zero padding to 4096 bytes.
WITHDRAW operation 7: operation byte, fresh channel-local nonce[32], slot[32],
revision u64 and canonical zero padding to 4096 bytes. RESULT is exactly 16384
bytes, with matching request nonce, outcome and empty accepted payload. Verify
both independently encoded expected bytes and malformed/nonzero padding; a
roundtrip through the same codec is insufficient.

Admission owns the selected Registration maximum of 1048576 bytes and 600
seconds. Route consumes its Grant and counts actual ingress+egress, fixed bodies,
frame headers and termination. Earlier caller/State/duty/parent deadlines still
win. Registration does not refill, renew expiry or borrow a new token to extend
its original allowance. Control 64KiB/30s and Forward 32MiB/1800s stay unchanged.

Reserve REGISTER/result and owning withdrawal/termination capacity before
allocating the slot or claiming its durable history. Include withdrawal in the
pre-effect exhaustion check; adding a withdrawal reserve after checking only
registration bytes cannot authorize over-budget work. Capacity is not a second
Admission quota. Keep the selected 1024-duty ceiling and 64MiB aggregate queued
ciphertext; do not preallocate 1MiB per slot.

A fresh random slot belongs to one terminal channel and original expiry. Neither
loss, failed ACK, withdrawal, reopen, fresh token nor another channel reclaims it
before original expiry. Revision checking is exact channel ownership, not a
Publication revision-authority implementation. A semantic refusal does not
replace another live slot or hide a physical terminal failure.

## Durable initialization and adoption boundary

Route slot history and new Admission spend history must have separate roots and
live leases. Open/initialize slot history successfully before the composition
can expose any class-3 receiving operation. Hold the Route root and actual
Admission root through joined receiving termination.

Ordinary receiver restart must strictly open retained slot history; it must
never initialize missing history opportunistically. The predecessor behavior
refuses a missing slot floor after previous spends. Preserve this safety rule
without reading/copying Admission's private journal format or borrowing its
mutex/lease. Initialization versus retained reopen must be explicit.

The initialization mechanism must prove fresh receiving-duty creation before
any class-3 spend can exist, refuse a retained Admission root paired with newly
created slot history, and retain damage after interrupted creation. Changing a
configured path is not authority to reset floors. Independent development roots
are not installed adoption or a compatibility migration.

Selected engineering arrangement after bounded read-only review: Admission
reports an opaque one-use durable fresh-root initialization fact for its exact
public duty binding. It originates only from the genuinely successful new-journal
creation branch and is obtainable before any admission/spend attempt; it is not
a caller boolean, current empty spend count, retained-root reopen result or
receipt granting Network/Service authority. Route consumes that fact before
exclusive slot-root initialization. Admission exposes no journal bytes, spends,
mutex or live lease and acquires no slot policy. Use the smallest real caller-backed
contract in the owning implementation; names below are semantic, not pre-created
interfaces/packages.

Composition opens genuine Receiving, then strictly opens retained Route slot
history or initializes an absent slot root using that exact one-use fact. Route
history keeps its own retained lease, exact duty binding and snapshot; missing
members of an existing root, foreign binding, corruption or uncertain durability
refuse without a replacement floor. Publish no listener/Accept callback until
both roots are durable and ready. A fact already taken/consumed, an admission
attempt, owner close or binding mismatch cannot authorize another initialization.
Check any delayed completion against the exact still-live original owners.

Reopen never issues a new initialization fact, including a header-only journal
or history pruned to zero current spend entries. Therefore removing the entire
slot root after prior admission cannot create a new floor. A crash after durable
Admission root creation but before completed Route initialization can leave an
unusable pair even without a spend; this is a retained fail-closed result, not
permission to reset. Completed retained bytes may be opened only after their
actual durability/binding checks. Automated incomplete-pair recovery requires
its own proved contract. Losing both roots or deliberately substituting an entirely
new installation pair is outside this local mechanism's detection; independent
development roots do not authorize installed history replacement.

This freshness fact belongs to Admission's local storage lifecycle, not token
rights or transport readiness. Do not infer it by reading filesystem names or
private journal format in Route. No successful readiness callback, reconstructed
receipt or generic shared-root manager qualifies the boundary.

Preserve the accepted public duty binding and slot snapshot identity/bytes where
reused (predecessor ARDISL01); do not imply conversion or adoption of old live
roots. Any retained-root compatibility change needs its own accepted contract.

## Effect and termination ordering

1. Verify current signed Network observation and exact retained Introduction
   duty; validate separate root paths and original deadlines.
2. Strictly acquire ready Route slot history and genuine new Receiving/Hosting
   owners before listening or admitting class-3 work.
3. Open the actual holder prefix/terminal TLS; holder Stock durably records
   presentation before token bytes are sent. Authenticate exact role peer and
   exporter; no supplied boolean or diagnostic Snapshot is authority.
4. Validate HELLO/purpose/class; retain physical channel ownership and finite
   Hosting/role capacity before Admission's irreversible spend. Recheck after
   durable I/O; spent rights remain spent even if subsequent steps fail.
5. Read one exact REGISTER; validate fixed body, original bounds and withdrawal
   reserve; serialize vacant slot reservation and durable non-reclaim claim.
6. Reobserve duty/currentness/expiry after durable claim and before complete ACK.
   A failed or uncertain ACK retains the claim. Do not let a late ACK create
   replacement authority or an accepting Publication.
7. Accept WITHDRAW only on its original channel with exact slot/revision. Stop
   delivery admission, join the serialized writer and physical terminal work,
   return the retained outcome, then release actual reservations.
8. On State loss, expiry, cancellation or physical failure, stop acquisitions,
   interrupt all borrowers, join readers/writers/children, preserve late physical
   errors and release roots/reservations once. Timeout waiting is not completion.

The registration transport handle supports future Publication composition;
callbacks must not execute old policy or grant success from an absent neighbor.
A product operation may expose a real registration/withdrawal lifecycle. It
must not be a test-only command, plan printer or wrapper-only package consumer.

## Collect, split and leave

| Maintained predecessor source | Route material to adapt | Excluded owner |
|---|---|---|
| internal/node/introduction/registration.go | Receiving purpose/class checks, REGISTER lifecycle, slot claim before ACK, actual Carrier/TLS | Old Node composition, authority wrappers, shared old spend lease |
| internal/node/introduction/delivery.go | Later serialized even-lane writer, pending/rate/allowance and withdrawal coordination | Successful Publisher validation/ACK cannot be substituted |
| internal/admission/spending/introduction_slots.go | Slot hash/expiry/time-floor snapshot and failure containment | Admission does not retain new Introduction slots; new spend formats stay private |
| internal/route/terminal/registration*.go | Exact REGISTER/WITHDRAW codecs and independent byte oracles | No old package/test import or broad dependency exception |
| internal/endpoint/introduction_registration.go | Identify exact registration handle and caller lifetime | Instance authority, pair/revision, refresh and Descriptor readiness stay Publication |
| internal/endpoint/introduction_dispatch.go and introduction_receive.go | Identify delivery handle/retirement and final local effect checks | Job, publication/replay validation and Connection recovery stay their genuine owners |
| New Route transport receiver/OPEN | Real terminal opening and receiving lifecycle under exact purpose | Existing Purpose7-only forwarding is not claimed terminal reachability |

New implementation must register cohesive packages/imports, doc.go, behavior
tests and genuine non-test callers in the owning change. Keep terminal receiving
and forwarding lifetimes explicit rather than turning Receiver into a generic
success callback dispatcher. No old/new import, reverse caller, process bridge,
shared live root or copied neighboring authority is allowed, including tests.

## Invariant-to-evidence map

| Invariant | Positive observation | Causal refusal/control |
|---|---|---|
| Genuine registration | Compiled holder and receiving commands perform real REGISTER/withdraw through signed State, genuine class3 tokens, new spend/Hosting roots and both Carriers | Wrong duty/key/family/purpose/class; authority/expiry loss before presentation or spend; no unauthorized effects |
| Independent canonical bytes | Manually constructed 4096-byte REGISTER/WITHDRAW and 16384-byte RESULT match accepted encoding | Wrong lane/op/nonce/length, nonzero padding, concatenation, unrelated/second registration |
| Exactly owned slot | Original channel receives complete ACK; exact owning withdraw retires transport | Duplicate slot on another channel, wrong revision, reconnect, new token, lost ACK cannot replace/renew |
| Durable non-reclaim | Reopen refuses original slot until original expiry; expired pruning retains monotonic floor | Deleted/missing root members, corrupt/rebound snapshot, uncertain replace/sync/close; repeated open does not reset |
| Fresh initialization | Route history ready before first actual class3 spend and receiver admission | Crash at each creation boundary, retained spend root plus absent/new floor, initialized root damage, swapped path; no silent reset |
| Capacity/spend/claim ordering | Finite reservation before burn; durable claim before ACK; original bound unchanged | Exhaust capacity before spend, authority loss during durable I/O, failure after burn/claim, retained no refund and no slot reclaim |
| Joined lifetime | Roots/Hosting held until actual Carrier/TLS/writers/children join; immutable repeated close | Cancel/expiry/withdraw race, blocked write, late physical error, slow join and replacement generation |
| Neighbor integrity | Existing new Network/Admission/Hosting/prefix regressions still pass | No duplicate quota, changed bounds, altered selection/refill or successful Publication substitute |

Run the selected genuine Linux Carrier/compiled-command profile with race and
all affected new owners, applicable Windows checks, architecture isolation,
make quick-check and make check on final source. Preserve causal earlier failures
and invalid environments. Unit fault injection is mechanical evidence; a real
Carrier pass is not an injected failure through the whole admitted path unless
that exact scenario was exercised. Inspect receipts honestly and bind them to
working bytes, committed source and dev integration. Installed adoption,
Publication readiness and complete Service qualification remain separate.

## Whole Route dependencies still retained

Receiving submission/delivery additionally requires the fixed opaque 474-byte
capsule, separate channel/request/delivery nonce namespaces, unchanged ciphertext,
fresh strictly increasing even lanes allocated/emitted by one writer, pending<=16
including writer waiters, four starts/s and original class3 allowance. Positive
successful delivery to a Publisher requires genuine recipient validation under
Publication and local lifetime owners. A registration transport ACK cannot
stand in for that success. Missing/invalid/no ACK and joined refusal remain real
Route scenarios, not complete Publication delivery evidence.

JOIN requires current eligible retained Rendezvous selection under accepted
uniform/preselected-alternative policy; class2 and one odd lane1; exact matching
secret/context/profile and opposite sides; no third/duplicate replacement;
unpaired <=10s/original deadline; both accepted RESULT transitions before data;
original allowance/lifetime/credit/queue bounds; joined physical termination.
Route delivers protected transport; Service Connection authenticates Instance
and decides logical recovery before Application data is exposed. These remain
separate owners even when a source file presently mixes them.

This registration design does not drop those obligations or close the complete
Route goal. Resolve genuine consumer dependencies from accepted owners, without
absorbing Publication/Connection/Execution into Route or hiding partial readiness.

## Inspection limits

The orchestrator inspected accepted protocol terminal/JOIN sections, new
Receiving/spending root and forwarding receiver/command composition, predecessor
registration/floor and Endpoint delivery-validation boundaries. This is static
source evidence, not rerun transport/persistence tests. A bounded read-only
review checked the separate-root initialization design at `5f7467b30`;
causal implementation controls remain required in its owning change. No implementation or package is
created by this document.


