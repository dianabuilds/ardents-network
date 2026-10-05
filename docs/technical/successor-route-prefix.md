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
header kind/lane/length before body allocation or reads. `carrier` owns exact-key
TCP/TLS and one-stream QUIC, direct/Node classification and finite handshakes.
`transport` owns authenticated role channels, parent/child framing, finite
credit and queues, directed Node Carrier reuse and joined physical termination.
Exact package/import directions are in the [package map](../development/package-map.md).

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

Each channel reserves separate 16 KiB control capacity within its principal's
aggregate ceiling. Data queues fit 4 MiB per prefix/session and 64 MiB per
receiving Node, with 64 KiB lane receive windows and at most 16 KiB frames.
One frame per ready lane is selected round-robin; control receives first
service and then alternates with queued data. Waiting for credit holds no
physical writer. Queue cancellation consumes no emitted bytes or credit and
cannot alter a sibling's active deadline. A failed started physical frame
poisons its shared framing boundary and retains its failure.

The local directed-pair pool contains at most 32 ready/opening/retiring entries,
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

Every returned Grant and capacity addition remains owned until the parent and
physical children join. Additions return in reverse order before the original
reservation, preserving its termination dependency. Lost acknowledgement after
emission retires uncertainty without refund. These operator envelopes do not
certify Carrier overhead or installed billing.

Once the parent retires its physical connection, child close joins the original
writer without changing the closed socket's deadline. The joined result still
retains the original authority cause and any actual late write/close failure.
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

The prefix boundary itself establishes no Service publication, Descriptor ACK,
Introduction delivery, Instance authentication, Local Grant or Connection recovery.
Those responsibilities stay with their named owners in the
[domain map](../development/domain-map.md); no old runtime consumer, callback,
process bridge, shared live root or migration of persisted authority is used.
