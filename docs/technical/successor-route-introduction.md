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
slot only. Explicit holder close joins Route before Stock and Network roots.
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
