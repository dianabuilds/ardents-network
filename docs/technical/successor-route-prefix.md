# Isolated retained Route prefix

The new Route operation is a retained Entry/Interior protected prefix, composed
only with new Network, Admission and Hosting under the
[migration contract](../development/route-migration-contract.md). The
[protected Route protocol](protected-route-protocol.md) owns accepted wire,
Carrier, role, lifetime, credit and scheduling rules. This owner adds no quota,
selection, retry or privacy-policy amendment.

## Responsibilities and authority

`internal/successor/route` owns role-purpose eligibility and Node/key/family
conflicts. `selection` owns exclusive independent Entry and Interior roots,
committed ordered alternatives, original horizons and monotonic floors. It
retains exact duties from one coherent Network observation and rechecks with
fresh `CurrentRuntime`/`MatchDuty`; transport failure does not redraw a pair.
Known locally controlled identities and families enter selection as exclusions.

`ardp` owns canonical generation-three frames and HELLO. It rejects invalid
header kind/lane/length before body allocation or reads. The lower `transport`
owns the portable stream/exporter contract, exact accepted profile identities,
literal endpoint validation and shared authentication/error classification.
`transport/tls` and `transport/quic` own the actual exact-key TCP/TLS and
one-stream QUIC implementations, direct/Node classification and finite
handshakes. Prefix and receiving composition select the actual adapter directly
from the already selected profile, without fallback. The former `carrier`
package is removed; shared actual-adapter behavior tests belong to the actual opening/composition
owner and exercise both implementations of the lower contract. Lower transport
imports neither adapter, including in tests.
`channel` owns portable parent/child framing, finite credit/queues, serialized
physical output and joined retirement. `prefix` owns its exact generations and
terminal openings; `join` and `introduction` own their holder operations.
`receiver` owns listener/dispatch, directed Node Carrier reuse and receiving
Grant retirement. The intermediate `operation` package is removed.
`role` owns the shared portable original-duty/profile reobservation and exact
HELLO purpose/recipient/deadline checks, including fresh nonce construction.
It also owns the shared holder handshake and negotiated exporter binding;
the supplying Stock owner durably presents bytes, and original-caller checks
surround HELLO, presentation I/O, ADMIT and ACCEPT. The receiving Grant and
its physical retirement remain with the receiving operation.
Prefix, JOIN and receiving call this owner directly at their effect boundaries;
it grants no transport, token, reservation or replacement authority.
Prefix also owns pair publication with the original Source-before-Responder
lock order. Context opening, final JOIN Context handoff and acquisition stream
publication use the same local transition. JOIN Context owns its context state;
it does not acquire another owner's generation mutex. The handoff performs no
Network observation, physical I/O or join while either Prefix lock is held.
The actual `prefix` package owns these physical generations and exposes opaque
Borrow, JoinBorrow and terminal channel lifetimes. It imports neither
Introduction nor JOIN consumers. `introduction.HolderRegistration` and holder JOIN keep operation bytes,
readers/writers and their retained terminal result; they cannot access Prefix
configuration, generation locks or lower lane/control-return callbacks. Native
JOIN Context binds its concrete original Prefix objects once; its private
lifetime seam preserves absent/partial-opening cleanup without exposing a
successful test factory. Pair identity/lock-order tests stay in Prefix;
acquisition and nested-stream tests exercise consumer retirement separately.

Exact package/import directions are in the [package map](../development/package-map.md).

One independently leased Introduction History transfers to one receiving
Registry. Its pending positions and durable non-reclaim claims share that
exclusive owner; constructing another Registry over the same live History
refuses before capacity or spend. A retained reopen obtains a new exclusive
History after the original owner has joined and released it, preserving floors.

Network owns signed State/profile acceptance, conflict/time history and current
membership. Admission Stock durably marks presentation before returning token
bytes. Receiving Admission owns verification, capacity-before-spend ordering,
irreversible debit and finite Grant transfer. Hosting owns the independent
provider budget and work/termination reservations. Route receives these narrow
operations from command composition and never implements their decisions.

Composition wraps the Hosting return in `Channel.HoldReservation`. An Admission
rollback after capacity or spend requests release; the channel retains it until
physical retirement joins. Failed setup therefore follows the same lifetime
rule as a successful Grant, without changing Admission's irreversible debit.

An outer Node-authenticated Carrier grants no Endpoint admission. Every forwarding child
opens fresh exact-recipient role TLS and validates its HELLO, Purpose and local
TLS exporter before genuine class-2 receiving admission. Inner HELLO must match
the exact OPEN before spending. Reobservation follows durable presentation and
receiving I/O and precedes acceptance, output and publication of the prefix.
Neither a supplied profile nor a diagnostic snapshot authorizes this operation.

## Physical ownership

Each original Prefix generation retains setup claims and published physical
borrows. A setup claim keeps cancellation and completion through refusal or
handoff. A published borrow binds interruption, joined result and activity to
that exact generation; no callback is rebound to a replacement. Registration
and JOIN keep their own operation rules, readers/writers and terminal result.
Prefix seals every setup/borrow, joins them without holding its lifetime lock,
and only then retires parent framing and returns generation resources.
After a local seal, the watcher leaves ordered parent retirement to Close:
joining the Interior reader cannot cancel Entry before its clean retirement.
Unexpected parent failure before a local seal still cancels dependent work;
the joined result retains the original physical failure. Portable pipe tests
force both orders and verify capacity returns only after both readers join.
Terminal setup checks the original Prefix caller before acquiring a claim or
observing Network, independently of the terminal operation's caller. Original
physical observations check that Prefix caller before and after the actual read,
including failed reads, and retain cancellation together with the original read
failure. Registration/JOIN role admission and recipient checks use this same
observation seam. Delayed derived cancellation cannot permit new presentation;
a local seal still permits bounded terminal cleanup under the original physical
caller and deadline. Portable real-pipe refusal controls isolate delayed callback
propagation without supplying successful Network, Stock or ACK.
Original-generation checks and Source/Responder pair commit sit alongside
their borrowers in `prefix/borrowing.go`; setup deadline bounds sit with setup
claims in `prefix/opening.go`. These small methods have no separate file owner.
Prefix also owns atomic Source/Responder borrow admission and the role locks
at JOIN publication. The stream owner locks its own state inside that commit;
fresh observations and physical I/O run outside all generation locks. A foreign
Source cannot borrow a Responder bound to another original generation.
Pending terminal setup belongs to the same Prefix generation. Its publication
checks synchronous seal, original caller/child and the still-held claim under
the owner lock, then transfers to one exact borrow. Repeated setup completion
does not decrement another claim or return a published physical borrower.
Prefix's runtime, setup, borrows, readiness and refill are portable; native
selection and Admission roots remain with those owners. Identical mechanical
tests execute on Windows and Linux without qualifying Windows durable owners
or an installed journey.

Activity and physical retention differ. A stopped Registration may cease to
keep a Prefix active for idle policy while its original physical work is still
retained for join. Its late failure remains part of Prefix retirement. Source
and Responder acquisition/handoff retain their original lock order and observe
the real caller after waiting for those locks; changing the retained record
does not permit a sealed generation or a delayed caller to publish new work.

Each channel reserves separate 16 KiB control capacity within its principal's
aggregate ceiling. Data queues fit 4 MiB per prefix/session and 64 MiB per
receiving Node, with 64 KiB lane receive windows and at most 16 KiB frames.
One frame per ready lane is selected round-robin; control receives first
service and then alternates with queued data. Waiting for credit holds no
physical writer. Queue cancellation consumes no emitted bytes or credit and
cannot alter a sibling's active deadline. A failed started physical frame
poisons its shared framing boundary and retains its failure.

The portable `receiver/carrier_pool.go` and its physical retirement tests execute
on Windows and Linux; listener and receiving Grant composition retain their
native root dependencies separately. The local directed-pair pool contains at
most 32 ready/opening/retiring entries,
with at most one Carrier per peer. It validates exact profile, retained duties
and selected Carrier outside the pool lock. Same-pair waiters use their own
cancellation; dial and retirement remain counted operations. This operation
retires the Carrier after its last joined borrower, without idle retention or
speculative dialing. Every child retains its own original bounds and admission.
QUIC physical deadlines attach the current local monotonic clock to the exact
supplied absolute instant. A wall-clock adjustment since library startup must
not shift physical expiry; this conversion changes no wire or authority bound.
OPEN allocation and complete emission are serialized together, with one
immutable setup deadline no later than ten seconds or the original child/parent
bound. Waiting for that operation is cancellable and grants no child authority.

EOF preserves the reverse direction. CLOSE retires queued work and interrupts
the child's physical writer; retained input leaves its queue through actual
consumption or joined owner cleanup. Withdrawal interrupts listeners, pending
setup, readers, writers and children; owners join them before returning
reservations or closing roots. Repeated Close returns one retained result,
including late physical and release failures.
Receiver cancellation first denies new accepting work and interrupts its
accepted connections, then closes the shared listener and transport. QUIC's
shared UDP transport must remain available while connection-close packets are
sent; destroying it first leaves remote retirement to idle timeout. Closing
the listener still interrupts unaccepted handshakes, and physical workers join
before reservations and roots return.
An already expired child starts no new terminal frame. Its original operation
remains refused, while a completed local child join alone does not retire its
still-live framing parent or siblings. Output already in progress still joins
and retains every physical failure; expiry grants no additional write time.
After that join, CLOSE rechecks peer and parent retirement under the session
lock. An earlier live snapshot cannot authorize another frame on a parent that
retired while its writer was completing; that parent's retained failure remains
its own joined result.
After writers join, this check includes the original opening caller's elapsed
deadline and the parent session's elapsed deadline. A later wall reading cannot
restore output authority after either original timer has fired.
If the original child bound expires while CLOSE waits for its writer turn,
the explicit pre-output deadline refusal completes without a new frame and
does not retire a healthy sibling. A shorter cleanup deadline still fails.
An error after entering TLS.Write remains that TLS generation's failure even
when the lower lane emitted no payload; zero lower progress cannot establish
that the TLS writer is reusable.
Physical session write/close failures survive Receiver and outgoing pool
retirement; ordinary peer refusal and EOF remain local session outcomes.
Command reporting identifies peer retirement only from the complete retained
error tree. An owned physical Close error may accompany a started write failure
only when both belong to the same session and every cause is a recognized peer
retirement. Close alone, another session's write, a deadline error or an unknown
cleanup cause remains a terminal failure. Classification retains the error and
the command's unsuccessful outcome; it does not establish clean termination.
A nested channel's Close can emit its parent lane's terminal frame. If that
physical write fails, the nested result retains the parent's same write witness
and cause; closing a borrowed lane does not mint a socket-close witness for the
nested session. An unrelated physical close or release failure still prevents
peer-retirement classification.
Healthy explicit or idle retirement joins terminal borrowers, completes the
original Interior TLS direction and sends lower EOF after its last bytes. The
lower framing reader stays alive until the exact peer CLOSE(0), including join
of any started CREDIT, before the Entry Carrier is interrupted. Reverse EOF
alone, refusal or a late physical failure cannot discharge that retirement.
This uses the earlier of the original bound and the existing one-second cleanup
horizon. Original cancellation, authority loss or expiry keeps immediate
physical interruption; no terminal write creates a renewed payload allowance.

`Prefix.Done` signals retired readiness; its caller must still Close to join
and release. `Receiver.Done`
signals joined listener retirement, and Close retrieves its terminal result.
A prefix without active terminal work expires after 120 seconds of idle
readiness or its earlier immutable deadline. Introduction registration work
remains bounded by its own original lifetime; its retirement resets idle
readiness without renewing any admission or parent deadline.
Autonomous idle expiration uses the same joined retirement path before its
notification. Close still retrieves its retained terminal result. A Source
joins original bound Responder openings and prefixes before retiring its own
framing parents, keeping bounded terminal authority alive during that join.

## Real command composition

### Requested forwarding-parent replenishment

`Prefix.Replenish` carries a fresh class-2 presentation on lane zero of the
retained Entry and Interior forwarding parents. It keeps their original HELLO,
exporter, peer, purpose, child credit and absolute deadline. Dedicated JOIN and
terminal channels refuse replenishment. There is one active parent control
exchange and one reader; child control continues during durable reservation.
No timer, automatic replay, new channel or selection accompanies this request.

The complete ADMIT costs 371 old bytes and must leave positive old remaining
capacity. Successful receiving verification/reservation/spend replaces remaining
capacity with the Admission class's 32 MiB; the matching ACCEPT costs another
21 bytes from that replacement. A cumulative usage witness preserves intervening
child debits instead of resetting history or adding another allowance. A refused,
invalid or missing acknowledgement cannot authorize replacement or token replay.

Command composition reserves the positive additional shared ingress/egress
envelope through the same Hosting owner and exact original reservation before
spend. Holder reservation occurs before emission; if child traffic changes the
required delta while storage runs, the writer returns without output and asks
only for the missing capacity. This does not hold the physical writer during
storage. Original operation cancellation is checked again after currentness
observation, before reservation/debit/output, even if derived cancellation is
delayed. Started physical failures keep their existing framing provenance.
The refill request's original caller is distinct from the caller that created
the prefix. Both lifetimes remain checked: request cancellation is observed
synchronously after currentness and at presentation, capacity, selected output,
ACK and final handoff, while prefix lifetime and Network checks remain in force.
The derived request context supplies seal interruption without concealing the
original request's already terminal state from durable owner callbacks.

Every returned Grant and capacity addition remains owned until the parent and
physical children join. Additions return in reverse order before the original
reservation, preserving its termination dependency. Lost acknowledgement after
emission retires uncertainty without refund. These operator envelopes do not
certify Carrier overhead or installed billing.

Once the parent retires its physical connection, child close joins the original
writer without changing the closed socket's deadline. The joined result still
retains the original authority cause and any actual late write/close failure.
Original parent context cancellation also denies new child deadline effects
synchronously, before its reader's retirement callback marks the framing owner
stopped. Callback scheduling cannot reopen that effect window.
Retained Rendezvous checks use the original Prefix leg, observer and immutable
deadline through Prefix-owned operations. JOIN Context retains the selected
choices and exclusions and commits its result against the same original pair;
it does not read Prefix configuration or generation locks. Network observation
still runs outside those locks and an exact incoming recipient is never replaced.
Prefix also owns the original terminal JOIN channel's control reservation,
OPEN, exact role TLS and holder presentation. JOIN owns its operation and RESULT;
only after verifying RESULT does it request framing preparation under the same
parent budget and original lifetime. Failed setup returns its partial physical
owner for joined retirement, whose result is retained before the acquisition
opening completes. No spent right is refunded and no replacement parent is used.
Stopping and joining its setup interruption is one retained transition. A false
second AfterFunc stop result cannot be interpreted as a running callback after
the first stop already prevented it; a genuinely running callback still joins
before physical cleanup or return of control capacity.
Physical Prefix opening, terminal Registration and terminal JOIN use this same
Prefix-owned stop/join mechanism; their requests, acknowledgements and published
operation lifetimes remain with their respective owners.

An unemitted upper frame's lower deadline refusal remains a failed operation;
it cannot mint an upper physical failure. Actual lower deadline I/O failures
are retained by the lower framing owner until joined return.

`ardents-next admission holder --config PATH` accepts optional `route` with
`entry_root`, `interior_root`, `hosting_root`, `domain`, whole-second absolute
`deadline`, `work`, `termination` and optional `exclusions`. With genuine
`network` configured, console operations `prefix-open`, `prefix-replenish` and
`prefix-close` open, replenish and join the retained protected prefix. Holder,
selection, Hosting and Network roots must be distinct and unnested. Local
Hosting capacity transfers exactly once, including failed setup; uncertain
release is retained rather than automatically retried.

`ardents-next route receive --config PATH` takes `network`, `node_id`,
`spend_root`, `hosting_root`, `certificate`, `private_key`, `work` and
`termination`. It opens genuine new owners and the exact signed forwarding
duty, reserves physical capacity before receiving spend, and listens on the
State-selected Carrier/address. Configuration is bounded by the existing
strict command decoder. Private key input is bounded and owner-private.
Cancellation or authority withdrawal joins Route before Admission, Hosting and
Network roots close. Output exposes fixed operation/phase/outcome only.

An exact Domain-2 issuance duty additionally requires the `issuer` plan, bound
to that same Node and independent quota, key and result roots. Its purpose-1
terminal carries one fixed 16,384-byte OPERATION and matching RESULT, using
Admission's canonical enclosed batch and padded result. Restricted children
derive the bootstrap quota kind from their original incoming claim; ordinary
children require genuine class-1 receiving Admission. The command composition
calls the existing `issuer.IssueCurrent`, without implementing quota or signing
inside Route. Issuer transport retains one operation slot and finite buffers
until joined retirement.
Portable `route/issuer` owns the single OPERATION/RESULT exchange, canonical
nonce/outcome agreement and termination of both role TLS directions. Prefix
calls its holder exchange and joins the lower channel before Stock completion;
Receiver dispatches its receiving exchange only after exact role authentication
and genuine ordinary admission or an immutable bootstrap claim. Receiver derives
the issuance kind and retains exclusive work, queues and Hosting until join.
The exchange opens no roots, signs nothing and releases no physical capacity.

The holder uses a separate typed `BootstrapPrefix`, exposing only issuance and
joined retirement. It retains the original Domain-1 selection, ten-second bound,
128 KiB lane allowance and shared 256 KiB queue. It shares actual channel and
Carrier mechanisms with admitted Prefix, without a successful presentation
substitute or an upgrade to ordinary work. Selection chooses only the exact
current profile issuer and rejects every known Node/key/family conflict.

`bootstrap-open`, `issuer-issue` and `bootstrap-close` drive genuine Stock and
issuer owners through the maintained holder console. The holder joins bootstrap
before `prefix-open` creates fresh admitted channels; ordinary `issuer-issue`
uses a fresh Control token. Request preparation, exchange, physical join and
`Stock.Attempt.CompleteBound` retain one original opening. Canonical result
padding, matching nonce and agreement between envelope and Admission outcome
precede cryptographic finalization. Stock reobserves its authority after signature
verification and applies the original local lifetime guard before token deposit.
Failure cannot refund the allocation or transfer a response to a replacement.
Failed issuer I/O also retains the original caller/generation cause before
Stock completion. A cancellation that interrupts reading as EOF remains
`context.Canceled`; it cannot deposit a delayed valid result.
Receiving issuer retirement likewise retains the original interruption cause
alongside a failed physical read/write. A real blocked-pipe refusal control
checks that cancellation remains observable and exclusive work stays held until
the writer joins. This mechanical control issues only Unavailable; it does not
establish successful blocked RESULT output through a complete admitted Carrier.

Receiving issuer work is exclusive before private Admission and is retained
through physical retirement and reservation return. A busy peer cannot spend
another Control token at the receiver or start another signing operation.
Its holder presentation and pending allocation are nevertheless irreversible;
an explicit same-prefix retry retains the original batch and delivery binding.
Local Hosting return failure belongs to the receiving owner's retained terminal
result. It occurs after physical joining and cannot be sent through an already
closed peer channel or revoke an earlier verified token signature.

The actual holder and compiled-process scenarios exercise both bootstrap batches,
refusal of a third, fresh admitted channels, ordinary issuance and joined Hosting
return on both Carriers. Two independent genuine holders also exercise busy
refusal before receiving spend, sibling progress after joining, cancellation
after actual issuer signing with Hosting still held, and retained late return
failure. A lost signed result is explicitly retried on the same original Prefix:
actual `IssueCurrent` reopens its durable owners and returns identical committed
bytes, while Stock retains its pending request and reserves allocation only once.
Signed successor State intake and cancellation of the actual receiving listener
are separately exercised before receiving Admission, after reservation but
before spend, after actual spend, before and after durable issuer debit, and
after a genuinely signed committed result. The holder caller remains live in
these cases; receiving cancellation is not replaced by caller cancellation.
The real quota and result journals
independently distinguish the debit and signing effects; obsolete completion
cannot append another result or erase an existing debit.
Each case refuses obsolete completion, retains the original allocation,
preserves repeated terminal Close results, and joins actual Hosting borrowers.
While the gated physical borrower has not joined, the fixture checks the exact
sum of its configured Hosting work and termination reservations in both
directions. After joined retirement, each actual return occurs once and the
reserved total is zero. This checks reservation ownership, not physical Carrier
overhead or provider billing.
Public Prefix/Stock composition also cancels the original issuer caller after
actual signature verification and before deposit on both Carriers. Finalization
has already erased blinding state; this refusal preserves spent allocation,
leaves no pending batch to reconstruct, and makes the verified tokens unavailable
to a fresh Stock presentation. Ignoring that guard's refusal causes the genuine
Stock probe to detect available tokens on both Carriers.
The lost-result scenario also joins the original Prefix and reopens the same
holder presentation root. The issuer retains identical signed bytes and refuses
a changed kind; the reopened holder has neither accepted permission nor pending
blinders. A fresh public permission request uses a new holder key and cannot
adopt the old permission or issuer response. Public bytes do not reconstruct
the lost volatile state.
Separate genuine Network/allocation/issuer composition verifies two valid
holder-signed requests with one Request ID and different blinded payloads.
The changed digest refuses at quota/debit, before signing, and leaves both
journals and the original retry result unchanged. This owner-bound conflict
control does not claim another full Carrier or holder-Stock exchange.
These controls do not establish the complete State-loss/cancellation matrix,
blocked physical output, every retry/refusal obligation or complete Route acceptance.

Physical envelopes are explicit bounded operator inputs; Hosting measures whole
named interfaces. These component operations do not qualify carrier overhead,
provider invoice attribution, anonymity or installed host protection.
Unsupported platforms refuse without opening Route roots.

The separately owned [Introduction registration](successor-route-introduction.md)
uses purpose 4/class 3 through a retained Domain 4 prefix, independent durable
slot history and actual holder registration-open/withdraw/close commands.
The separately owned [Rendezvous JOIN transport](successor-route-join.md)
composes one installation with original Source/Responder openings, retained
context selection, genuine paired admission and protected framed streams.

## Evidence boundary

`make route-check` is Linux new-domain regression and race execution over
Network, Admission, Hosting, Route and `ardents-next`, with structural isolation
and profile checks. The [test registry](../../tests/profiles/profiles.json) names
its prerequisites. Required repository quick/full gates remain separate.

Command tests exercise signed State, real allocation/blind issuance, durable
Stock and receiving spend, actual Hosting and both Carriers. Retained selection
reopen, pre-admission clock loss, post-admission cancellation/clock loss/
successor/expiry, live-receiver burnt-stock refusal and compiled processes are
distinct scenarios. Transport-local tests isolate bounded scheduling, partial
physical failure, independent pool waiter cancellation, late dial/write results
and release-after-join; they supply no successful neighboring authority.
Exact candidate receipts and failures belong to the selected execution issue.

`TestRouteReceivingRefillFailureAndReopenBothCarriers` exercises a genuine
receiving forwarding parent with signed Network, durably presented Stock and
independent spend/Hosting roots. Actual competing capacity and authority loss
after reservation refuse before spend; loss of acknowledgement or authority
after spend cannot refund the token. The blocked post-spend worker keeps both
reservations until joined retirement. Reopening the same receiving and Hosting
roots distinguishes unspent refusal from burned rights and checks complete
return without lowering measured-use floors. This receiving fault harness is
separate from the compiled prefix/refill/subsequent-child consumer scenario;
its injected ACK loss supplies no successful neighboring authority.

`TestRouteRefillPublicOriginalCallerBothCarriers` calls actual public
`Prefix.Replenish` with independent prefix and request callers and delayed
cancellation propagation. Genuine signed Network, Stock and physical admission
exercise observation, presentation, additional capacity and receiving spend
boundaries. Before emission, cancellation starts no receiving refill; after
durable presentation/spend, rights remain consumed and reservations return only
after joined retirement. These controls supplement the successful compiled
consumer and the transport-local scheduling/failure controls.

The prefix boundary itself establishes no Service publication, Descriptor ACK,
Introduction delivery, Instance authentication, Local Grant or Connection recovery.
Those responsibilities stay with their named owners in the
[domain map](../development/domain-map.md); no old runtime consumer, callback,
process bridge, shared live root or migration of persisted authority is used.
