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

An outer Node-authenticated Carrier grants no Endpoint admission. Every child
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
OPEN allocation and complete emission are serialized together, with one
immutable setup deadline no later than ten seconds or the original child/parent
bound. Waiting for that operation is cancellable and grants no child authority.

EOF preserves the reverse direction. CLOSE retires queued work and interrupts
the child's physical writer; retained input leaves its queue through actual
consumption or joined owner cleanup. Withdrawal interrupts listeners, pending
setup, readers, writers and children; owners join them before returning
reservations or closing roots. Repeated Close returns one retained result,
including late physical and release failures.
Physical session write/close failures survive Receiver and outgoing pool
retirement; ordinary peer refusal and EOF remain local session outcomes.
`Prefix.Done` signals retired readiness; its caller must still Close to join
and release. `Receiver.Done`
signals joined listener retirement, and Close retrieves its terminal result.
This bounded operation carries no further useful work after opening, so its
idle readiness expires after 120 seconds or its earlier immutable deadline.

## Real command composition

`ardents-next admission holder --config PATH` accepts optional `route` with
`entry_root`, `interior_root`, `hosting_root`, `domain`, whole-second absolute
`deadline`, `work`, `termination` and optional `exclusions`. With genuine
`network` configured, console operations `prefix-open` and `prefix-close`
open the retained protected prefix and retrieve its joined result. Holder,
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

This slice has no Service publication, Descriptor ACK, Introduction delivery,
JOIN workload, Instance authentication, Local Grant or Connection recovery.
Those responsibilities stay with their named owners in the
[domain map](../development/domain-map.md); no old runtime consumer, callback,
process bridge, shared live root or migration of persisted authority is used.
