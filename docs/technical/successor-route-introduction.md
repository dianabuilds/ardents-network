# Isolated Route Introduction registration

The [protected Route protocol](protected-route-protocol.md) owns accepted bytes,
role purposes and bounds. The [registration boundary design](../development/route-introduction-registration-boundary.md)
and [migration contract](../development/route-migration-contract.md) preserve
separate authority and lifetime owners. This operation establishes bounded
registration transport, not Publication readiness or complete Route acceptance.

## Owners and transport

`internal/successor/route/introduction` owns the slot registry, original-channel
registration state, canonical requests/results and independent durable slot
history. `introduction.HolderRegistration` separately owns the portable holder
REGISTER/WITHDRAW implementation and its joined lifetime over the actual
`prefix.IntroductionChannel`. The holder opens no durable root and shares no
receiving Registry/History state. Prefix owns OPEN, inner TLS and role admission;
Registration owns its operation bytes, ACK checks and terminal result.
Its immutable `Receipt` retains the original channel and copied Network/profile,
Introduction Node, slot, revision, creation time and original expiry.
Its acknowledgement is SHA-256 of the complete verified REGISTER RESULT body,
preserving the current public Publication's transport-commitment provenance.
`CheckReceipt` reobserves the actual Prefix/duty/caller outside its lifecycle
lock, then checks retirement, withdrawal and expiry synchronously.
Recipient reobservation retains its original unavailable, cancellation or
physical error before checking the observed member's role, exclusions and bounds;
failed observation does not manufacture a second duty-mismatch cause. Detached
`RegistrationFacts` and a retained receipt alone grant no current authority.
Creation precedes REGISTER output; ACK delay cannot move the refresh origin.
This transport receipt supplies no Descriptor Store ACK or Service readiness.
Receiving composition retains its native Admission/root dependency separately.
The receiving REGISTER/RESULT/owning WITHDRAW exchange now belongs directly to
`introduction.Registry.ServeRegistration`. It consumes the original HELLO,
exact role authority, already reserved position and original accepted byte/time
bounds. It derives binding from the actual negotiated exporter, reobserves at
effects, and keeps the durable claim after lost acknowledgement. It does not
own the listener, Admission Grant or physical return: receiving composition
interrupts and joins its connection before releasing those resources. This
exchange is portable; genuine successful claims still require the independently
leased durable History and genuine Admission/Hosting command composition.

The receiving Registry also dispatches one opaque purpose-5/class-1 submission
to the exact live purpose-4 registration. Its original registration stream has
one RESULT reader and a shared frame writer. It reserves the full 20529-byte
delivery exchange before output, separately from the 41024-byte registration and
owning-withdrawal reserve. At most 16 users, including writer waiters, remain
pending; admission and dispatch each permit at most four starts per rolling
second. Even lane allocation and OPERATION emission share the writer. Fresh
registration-local request nonces never copy source or sealed delivery nonces
and remain recorded after user join. Exact fixed RESULT precedes matching CLOSE;
invalid, expired or incomplete protocol retires the original channel without
slot reclaim or debit refund. Physical users and late failures remain retained
through receiving join. Owning withdrawal with pending delivery refuses and
retires that channel rather than acknowledging an unfinished child.
Registration cancellation and protocol/read failures remain in that original
operation's joined result. Receiving composition separately records actual
output and interruption failures. Interruption uses the original lower lane:
it denies new read/write progress and interrupts its selected output, while
an already retired parent performs no redundant physical deadline effect.
Every actual interruption or late writer failure remains retained through join;
this distinction does not classify error values as successful cleanup.
The actual Receiver routes admitted purpose-5 work to this owner; its Control
reservation consumes no new registration position. Portable pipe/accounting
controls establish these mechanics, not genuine successful capsule delivery.
Source submission retains its original Domain 1 Prefix and resolves the claimed
Introduction duty from the same authenticated observation, with identity, key
and known-family exclusions for its retained legs, issuer, resolution and caller
inventory. It opens fresh purpose-5/class-1 TLS and forwards only the sealed
capsule. Introduction accepts an authenticated Domain 1 Interior only for that
submission purpose; Domain 4 Interiors retain registration purpose. These child
checks precede role TLS and Admission. A matching Source-local RESULT, actual TLS
and lower termination, physical cleanup and final original caller/duty checks
complete Submit. A typed refusal never grants retry or Connection authority.
Prefix's actual frame reader retains Transport's original TLS peer-EOF evidence
before local Close; it leaves kind/lane/nonce/status interpretation with Submit.
Raw or local EOF cannot acquire that evidence or become successful completion.
Both-Carrier qualified recipient acceptance remains a separate prerequisite
for successful private delivery evidence.

Holder and receiving principals have independent state and lifetimes in this
package; actual `receiver` composition dispatches the admitted channel. The
intermediate operation owner is removed.
Prefix owns retained leg/recipient checks and physical OPEN/TLS/role admission;
`prefix.IntroductionChannel` retains that setup, its lower lane/control hold and
original callback completion. Registration consumes its opened stream and
interprets its own operation ACK.
It does not read the Prefix's configuration, context or framing parent. A failed
observation completes the child setup claim without retiring its live parent.
Network authenticates current retained duties; Admission owns presentation,
class-3 verification and irreversible spend; Hosting owns actual reservations.
No owner shares Admission's private journal, lease or mutex with Route.

A retained Domain 4 Entry/Interior prefix opens fresh exact-key TLS to the current
Introduction delivery duty. Purpose 4 and class 3 are mandatory. Selection excludes
both retained Entry/Interior pairs and locally known Node/key/family conflicts.
The closed operation requires one eligible delivery duty; it performs no retry,
redraw or general Rendezvous selection.

REGISTER is exactly 4096 bytes with operation 3, fresh request nonce, random slot,
revision, original expiry and zero padding. The complete matching RESULT is
exactly 16384 bytes with empty payload. Owning WITHDRAW uses operation 7, a different
request nonce and the original channel/slot/revision. Registration consumes the
original 1 MiB/600-second Grant, with every earlier caller/profile/duty/parent bound
still applying. It does not refill or renew a slot.

The registry reserves finite capacity within the 1024-duty ceiling before spend.
It retains pending reservations until physical retirement, and reserves REGISTER,
RESULT and owning withdrawal bytes before claiming a slot. A durable claim
precedes registration acknowledgement; failed or lost acknowledgement retains
the claim. Withdrawal retires live slot authority and keeps its original floor.

## Independent durable history

Slot history persists hashes and original expiries plus its monotonic time floor
and exact Network/profile/Node/duty binding. The `ARDISL01` identity and canonical
bytes remain unchanged. Strict retained open checks both private members, its
exclusive lease and exact snapshot. Missing, corrupt, foreign or uncertain
history refuses rather than resetting its floor.

Initialization requires a vacant private directory and a genuine opaque one-use
`Receiving.TakeFreshRoot` fact. That fact originates only after durable exclusive
creation of the independently owned Admission spend journal. Retained or pruned
empty journals do not recreate it. Admission attempts, Close, binding mismatch,
double use and late completion invalidate initialization. Both roots must be
ready before listener publication. Interrupted initialization retains damage;
automated adoption/recovery and loss of both independent roots are outside this
mechanism's detection and authorization.

## Physical retirement and outcomes

The prefix seals acquisition, cancels every pending original terminal setup and
stops active registration handles. It joins those borrowers before retiring
their framing parents and returning reservations. Registration Close joins its
reader, watcher, physical interrupt and entire owning withdrawal attempt and returns the
same retained outcome. Done signals retirement; it does not replace Close.
Its original child-cancellation callback seals the Registration stop barrier
before physically interrupting the idle reader, matching explicit Close.
That barrier retains cancellation of the original REGISTER caller even when
explicit Close beats the asynchronous callback. Cancellation of the private
terminal after a successful owning stop does not cancel that original caller
or replace the already retained outcome.
Unexpected cancellation of that original child remains a failed outcome even
while the REGISTER caller is live. An earlier successful owning stop keeps its
already retained result; the cancellation callback cannot relabel it.
The owning `Stop` operation seals acquisition and interrupts this original
Registration without joining. Publication uses it before canceling its own
flight contexts; `Close` remains mandatory before returning physical resources.
Stop still records an already canceled original REGISTER caller and preserves
the first observed failure. It grants no receipt, accepting pair or replacement.
Previously observed reader failures, physical Close errors and whole WITHDRAW
results remain separately retained; this ordering supplies no clean-write
classification or waiver of an interrupted physical output.
`DeliveryResultFailure` identifies a failed Reply after this original child
emitted and checked status0 RESULT. It retains every original cause in the
Registration's joined outcome; it proves no peer receipt or matching CLOSE.
The original expired-terminal, interrupted-delivery and unavailable-withdrawal
error values retain their existing messages and distinct failed outcomes.
None supplies an ACK, accepting authority, refund or physical join receipt.
The original Prefix terminal keeps its lower lane, control-capacity return and
original cancellation callback. Registration retains that terminal rather than
copying those resources into separate fields. It joins the callback before its
reader/writer completion and requests lower return only after physical join and
the whole withdrawal attempt, preserving the same release ordering.
The original caller context remains linked after REGISTER hands out its handle.
Close stops or joins that exact cancellation callback before releasing resources;
successful setup does not detach the registration from its caller's lifetime.
Effect checks and final handle handoff also inspect the original caller's
cancellation synchronously; an unscheduled callback cannot authorize success.
The presentation channel rechecks both original caller and physical child after
durable Stock presentation and before ADMIT emission. Refusal retains the burnt
presentation without emitting token bytes or authorizing Receiving spend.

WITHDRAW also retains its own original operation caller, distinct from the
registration lifetime. It checks that caller after currentness observation,
before and after physical request output, and after stopping or joining its
cancellation callback at completion. A genuine successful ACK cannot discharge
an already canceled caller. The whole attempt remains held through ACK and
that final check; concurrent Close cannot return its Prefix borrow or publish a
terminal result before the withdrawal's retained error is recorded. These
checks change no wire bytes, slot floor, deadline, retry or spent right.
The sole holder reader completes TLS and lower peer termination after the
matching owning ACK, before publishing its reply and canceling the terminal.
This keeps original receiving withdrawal alive until it has consumed holder
EOF; ACK receipt alone cannot return its physical channel or parents.

After emitting the owning WITHDRAW result, the receiving channel remains held
until the holder closes it or its original deadline/cancellation interrupts it.
A nested write returning success alone does not prove the holder received its
RESULT. This drain supplies no new slot authority, allowance, deadline or ACK.

Incoming CLOSE interrupts only its lane's active physical writer. Started failed
frames retain their actual cause and charged bytes; queued cancellation neither
emits nor changes a sibling's physical deadline. Exact lower framing witnesses
span the whole encoded TLS write. Only authenticated CLOSE(0), unchanged required
physical attempt counts and no failed/active lower writer can discharge a wholly
unemitted CREDIT/CLOSE. Raw EOF, refusal, local closure, partial output and earlier
failed CREDIT cannot provide this evidence.

Physical failure remains a failure. Command stage `peer-retired-write` requires
every retained error branch to have proven started-write provenance and an exact
typed peer-retirement cause; unknown or mixed leaves remain failures at another
stage. It never turns failed cleanup into graceful completion or token refund.

## Actual command composition and evidence

With genuine Network and Publisher Stock configured, `admission holder` exposes
`registration-open` with positive `revision`, `registration-withdraw` and
`registration-close` after `prefix-open`. The open result contains the opaque
slot and detached public registration facts from the checked original receipt.
Explicit holder close joins Route before Stock and Network roots.
The Source holder's `capsule-submit` command takes the exact Introduction
Node/duty generation and either sealed capsule bytes in `payload` or a local
`capsule` preparation input, never both. Preparation takes the exact slot,
delivery nonce, revision, whole-second expiry, public recipient key and canonical
344-byte untrusted request. Standard HPKE produces the same 474-byte envelope
before the actual Submit operation. Request bytes are not returned in the result
and the command clears its decoded request buffer after use. This mechanism
supplies no capsule authentication, Work Safety, Connection authority or
Publisher readiness; the recipient independently checks every candidate fact.
`route receive` requires independent `introduction_root` for the exact Domain 4
delivery duty; its directory must be private. Other duties cannot borrow that
root or its receiving initialization fact.

Independent byte/floor oracles, capacity/spend ordering, durable loss/reopen and
initialization interleavings are separate from actual Carrier integration.
Genuine signed-Network/Publisher-token/Receiving/Hosting tests and compiled
holder/receiver processes exercise both TCP/TLS and QUIC. Mechanical nested TLS,
queued/started CREDIT and incoming-CLOSE tests isolate physical ordering without
substituting successful authority or ACK. The full new-domain `make route-check`,
repository quick/full gates and final source/commit receipts remain required.
Execution status and acceptance belong to GitHub, not this technical owner.

Opaque submission/delivery, JOIN, Publication, recipient decryption/acceptance,
Descriptor authority and authenticated Service Connection remain separate
responsibilities. No old runtime consumer, process bridge, persisted-root
adoption, installed qualification or privacy claim follows from registration.
