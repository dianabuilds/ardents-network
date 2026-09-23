---
id: R-157
title: What receive credit is backed by one forwarding parent's shared capacity?
status: open
owner: Product Owner and design assistant
started: 2026-09-23
reviewed: 2026-09-23
---

# R-157 — Backed receive credit for one forwarding parent

## Decision this unlocks

Issue [#78](https://github.com/dianabuilds/ardents-network/issues/78) must select one receive-credit, DATA queue and control-reservation contract before the dependent runtime issue #79. This record is open evidence and a candidate comparison. It does not amend the wire grammar or authorize #79.

## Current contract

The [C0 product scope](../../product/scope.md) keeps the bounded closed-alpha Route and both selected Carriers. The [protected text workload](../../product/protected-service-workload.md) includes a 512-byte request, 64 KiB response and the named exchange latency gate. The [threat model](../../security/threat-model.md) requires finite local resource use even with malicious or Sybil peers. The [protected forwarding protocol](../../technical/protected-route-protocol.md#flow-control-and-scheduling) currently states 64 KiB initial receive credit per live lane, 1–16,384 payload bytes per BYTES frame, at most 256 work lanes plus two reserved control lanes, a 4 MiB prefix queue, a separate 16 KiB channel control queue, and a 64 MiB receiving-duty queue ceiling. [ADR-0085](../../adr/0085-bound-forwarding-replenishment.md) replenishes the parent's byte allowance, not a child's receive queue or credit.

The present ARDP generation is 3. Child OPEN has no credit field. ACCEPT carries a u32 credit but is restricted to lane zero and acknowledges parent admission, not a child OPEN; child CREDIT carries a u32 increment. The current source and forwarding implementations initialize both child directions at an implicit 64 KiB. No public or long-term protocol commitment follows from this closed-alpha grammar.

## Hypotheses and falsification

- **H1, current promise is backed:** every admitted lane may use all credit already granted without refusal solely because an ancestor DATA queue is full. A legal simultaneous burst that exceeds an ancestor cap falsifies H1. The arithmetic and existing queue test below already falsify it.
- **H2, internal-only reservation is sufficient with all selected maxima unchanged:** keep 64 KiB initially granted in each direction, admit the selected lane count, and keep the 4 MiB prefix cap. A summed promise above the cap falsifies H2. It is falsified even before frame overhead.
- **H3, explicit paired credit change can back the promise:** account complete data frames against the receiving prefix and duty escrow before advertising credit, with a separately reserved control path; establish a bounded grant for each child direction before ordinary BYTES and return capacity only after actual consumption or joined retirement. At a nested seam, either bound and reserve outer expansion or prove that its separately backed lane credit and backpressure can defer a legal inner frame without refusing it or blocking sibling control. This remains a candidate until that proof, compatibility and workload latency are checked.
- **H0:** no candidate preserves the selected workload and resource ceilings; then change the product/technical contract explicitly before implementation.

## Evaluation criteria

A passing contract must make every admitted legal BYTES frame payable at every applicable parent and duty boundary, in both directions, without silent loss or an unexpected full-queue refusal. DATA saturation leaves the selected control reserve and termination progress. Delivery, delayed CREDIT, EOF, CLOSE, cancellation and failed physical output cannot double-grant or prematurely refund capacity. One child cannot multiply the absolute parent byte/time allowance. The 512-byte/64 KiB text exchange and its selected latency gate remain applicable; no lower throughput or lane count is hidden inside an implementation detail. No new destination, authority, identity or privacy claim is selected here.

## Evidence plan

### Primary sources

All local sources were read from the maintained tree at dev@82a5b1b44700a610aac6b7a1400b3a222c2a4a46 on 2026-09-23; the next dev change only selected this question.

- `internal/route/closed_source_channels.go`, `closed_source_queue.go`, `closed_source_lane.go`, `closed_source_capacity.go`, `closed_join_client_stream.go`: Source, nested JOIN, queue and credit owners.
- `internal/route/closed_forwarding_channel.go`, `closed_forwarding_reverse.go`, `closed_forwarding_budget.go`, `closed_duty_limits.go`: receiving Node and duty owners.
- `internal/node/closed_forwarding_link.go`: actual reverse-consumer/credit caller.
- `internal/route/closed_forwarding_channel_test.go`: current prefix-full oracle.
- The product, threat, protocol and accepted ADR owners linked above.

### Experiment

First complete an arithmetic/state-transition table for the existing owners, including frame headers and nested TLS expansion. No network experiment can repair a failed capacity inequality. After choosing a contract, #79 must exercise a simultaneous legal burst, slow consumers, delayed CREDIT, parent cancel/EOF/close and nested JOIN through actual callers on TCP/TLS and QUIC. Capture exact source/build, command, exit, queue high-water and terminal cause in the selected execution profile; missing prerequisites are invalid evidence.

### Failure scenarios

An admitted peer may send minimum-size BYTES frames, spend its entire advertised credit, stop reading reverse DATA, delay CREDIT, send control during DATA saturation, or race CLOSE/cancel with in-flight frames. Multiple admitted lanes and nested parents may do so concurrently. Replenishing the class-2 parent byte allowance does not replenish receive memory.

## Findings

**Sourced fact:** `closed_source_channels.go` initializes `credit` and `receiveCredit` to 65,536 on child open. `closed_forwarding_channel.go` initializes `credit` and `reverseCredit` to 65,536. Source DATA admission limits its shared queue to 4 MiB minus 16 KiB; the forwarding channel caps its shared DATA queue at 4 MiB and the receiving duty separately reserves 16 KiB/channel for control inside 64 MiB. `closed_join_client_stream.go` debits actual nested queued bytes to its Source parent but does not reserve future credit promises there.

**Measurement (exact integer arithmetic, no runtime run):** Let `B = 4,194,304 - 16,384 = 4,177,920` bytes be the tighter Source prefix DATA ceiling after its control allowance. The current one-direction promise at 64 lanes is `64 × 65,536 = 4,194,304 > B`. At 256 lanes it is 16,777,216 bytes in one direction and 33,554,432 bytes across two directions. If the two separately reserved control lanes are also admitted and credited, 258 lanes promise 33,816,576 bytes across two directions. These figures omit frame headers and nested encryption overhead.

**Measurement (ceilings if current initial credit is retained):** The tighter prefix DATA bound can back at most `floor(4,177,920 / (2 × 65,536)) = 31` simultaneous full-credit bidirectional lanes, or 63 in one direction, before frame overhead. Covering 256 such work lanes would need at least nine otherwise independent parents if their capacity, route and admission contracts actually permit that composition; no such composition is assumed. At the receiving-duty maxima, 1,024 children with two 64 KiB promises total 128 MiB. If 1,024 channels are also live, their 16 KiB control reservations total 16 MiB, for at least 144 MiB before frames against the current 64 MiB duty ceiling. Enlarging only the prefix queue therefore does not close the global promise.

**Candidate arithmetic, not a selected grant:** If every one of 256 work and two reserved control lanes must hold equal simultaneous credit in *both* directions, a 4 MiB prefix with 16 KiB withheld for control permits at most `floor(4,177,920 / (258 × 2)) = 8,096` complete-frame bytes per direction; `258 × 2 × 8,096 = 4,177,536`, leaving 384 bytes. A one-frame 512-byte payload would consume 528 such units. A 64 KiB payload would need at least nine BYTES frames at that initial per-lane grant, with later grants possibly pipelined; the selected latency gate remains unproved. Across the duty's maximum 1,024 children, two such initial grants per child total 16,580,608 bytes; even 1,024 channel-control reservations total 16,777,216 bytes, below the 64 MiB duty cap before other retained work. This arithmetic does not prove nested TLS, frame scheduling, control progress or resource behavior. The selected pending-OPEN rule separately permits up to 4,096 early TLS-handshake payload bytes per child; for 258 simultaneous pending children that is up to 1,056,768 bytes in the sending direction, which must be included within a backed grant or another expressly bounded reservation. It cannot be adopted without an explicit incompatible wire disposition and full workload checks.

**Candidate-unit counterexample (arithmetic, not a selected grant):** Calling 8,096 a *payload-byte* grant would not fit the tighter Source prefix. If 258 lanes each hold one lawful 8,096-byte BYTES payload in both directions, the payload promises alone consume 4,177,536 bytes, leaving only 384. `closedSourceLane.enqueueLocked` charges the 16-byte frame header on each queued outgoing BYTES frame while the receive path charges payload, so even one frame in each direction per lane needs at least another `258 × 16 = 4,128` queued bytes: 4,181,664 total, or 3,744 above the 4,177,920-byte DATA ceiling. A contract using that numeric grant must debit *complete encoded frames* before granting and reconcile both owners' queue units; splitting payload into smaller frames only increases header demand. This example says nothing yet about nested TLS expansion or the 64 KiB response latency.

**Sourced fact:** `TestClosedForwardingChannelBoundsOnePrefixQueueBeforeDutyQueue` opens 64 lanes, queues 64 KiB on each, opens lane 65, and expects the latter lane's 64 KiB BYTES to be refused by the full prefix queue. The frame is within lane 65's already initialized credit. This test proves the storage cap but cannot prove the credit promise.

**Inference:** With both the current lane count and prefix cap held, no accounting change confined to local counters can back all initially advertised 64 KiB promises. Either initial grants, admitted concurrency or the cap must change explicitly. A paired wire change is needed if the 256-work-lane and 4 MiB choices remain binding: OPEN currently has no child grant, lane-zero ACCEPT cannot grant one, and both child directions currently assume 64 KiB.

**Sourced fact:** `closed_lane.go` encodes one ARDP generation-3 grammar for all current role purposes. `closed_role_child_stream.go` uses implicit 64 KiB credit in both directions for its single bootstrap or Source Entry-to-Interior child; `closed_join_client_stream.go` does likewise for the joined stream. A changed *global* child-credit rule would affect callers beyond #79. A rule limited to authenticated multi-lane parents might retain the single-child/JOIN 64 KiB behavior, but the current technical owner says every live lane starts at 64 KiB, and forwarding purpose also appears on one-child setup paths. Such a distinction needs an explicit accepted contract, peer-agreed profile/parent classification and an exact caller audit; this record does not assume a global cutover or that incrementing the generation number alone is sufficient.

**Sourced fact:** `ClosedForwardingChannel.NextAvailable` retains a delivered DATA charge until `Credit` after downstream consumption. `QueueReverse` charges complete reverse frames, including their 16-byte header, while forward `bytes` charges the payload to the shared queue. `ClosedDutyLimits` accounts current queued data, not promises. These asymmetric counters must be reconciled before selecting units for a new grant.

**Sourced event-order map:** `closed_source_channels.open` initializes both 64 KiB counters before queueing child OPEN; `ClosedForwardingChannel.open` initializes both 64 KiB counters before yielding its OPEN event, with no child acceptance acknowledgement. Source receive and forwarding input charge BYTES payload, while `QueueReverse` charges each complete reverse frame. `closedSourceLane.Read` releases consumed payload and `returnCredit` increments local receive credit before its CREDIT write completes. `ClosedForwardingChannel.Credit` releases delivered queue charge and increments local credit before returning a CREDIT frame; `closedForwardingLink.copyReverse` writes that frame later and aborts the link on write failure. A new promise escrow therefore has to specify the interval between local consumption, reserved grant, physical CREDIT emission and terminal failure on both sides; merely changing a credit constant or queue cap cannot define this interval.

**Sourced call order:** `newClosedJoinedStream` gives its inner owner `queueParent = outerLane.owner`. Inner `enqueueLocked` charges the inner queue and that outer parent; the inner writer then emits over its secured TLS connection backed by the outer lane, whose enqueue can also charge that parent before the inner write completes. On input, the outer lane's `Read` releases its queued ciphertext before the inner frame is admitted and charged to the same parent. `reserveQueuedLocked` currently returns failure when the parent's actual queue is full. The existing outer-credit backpressure test checks a waiting writer and a sibling lane, not this nested transfer at shared-capacity saturation.

**Sourced toolchain fact (Go 1.26.8, accessed 2026-09-23):** The current [nested role TLS client](../../../../internal/route/closed_role_tls_client_linux.go) and [server](../../../../internal/route/closed_role_tls.go) force TLS 1.3 but leave `DynamicRecordSizingDisabled` unset. In the pinned [Go 1.26.8 `crypto/tls` writer](https://github.com/golang/go/blob/go1.26.8/src/crypto/tls/conn.go#L885-L1020), one application `Conn.Write` can be split into several TLS records; the initial record payload target is smaller and grows with packets sent, while [TLS 1.3 record limits](https://github.com/golang/go/blob/go1.26.8/src/crypto/tls/common.go#L64-L69) are only maxima. Here each encrypted record is written through the outer `closedSourceLane.Write`, which may split it again according to available lane credit and adds a 16-byte ARDP header per resulting BYTES frame. Thus `22 × ceil(inner bytes / 16,384)` plus one outer frame is **not** a safe expansion bound for the current caller. Any reserved bound must cover record-size state, possible outer fragmentation and the time both inner and outer queue charges overlap, or prove safe backpressure independent of that bound. This observation is about the selected toolchain, not a new TLS or wire contract.

**Inference:** A candidate needs an explicit transfer or bounded staging rule for overlapping inner/outer queued bytes, and a proof that a legal nested frame can wait without blocking sibling control. Counting the same logical work twice can create premature refusal; releasing both charges early can overcommit. Process-resident physical copies still belong to the applicable RSS gate even if the logical queue charge transfers once. This is a design obligation, not evidence that the present code has failed a measured nested case.

**Open evidence:** The bounded outer-frame/TLS expansion or an alternative safe nested backpressure proof, and the minimum per-lane grant compatible with the selected text workload, have not been derived or measured. Source allows up to 258 lanes including two reserved controls, while the current forwarding `open` map rejects at 256; the #78 capacity calculation must use the accepted count and must not silently treat this implementation discrepancy as a new product decision.

## State-transition table to close before decision

| State | Required capacity invariant | Current evidence |
|---|---|---|
| Initial admission | Sum of unspent grants in both directions plus retained DATA must fit every applicable prefix and duty pool; control remains reserved. | Violated by the current fixed 64 KiB grants under a 4 MiB prefix. |
| Simultaneous lawful burst | Receiving a complete frame moves its charged size from promise to retained work without increasing total escrow; no full-queue refusal within a grant. | Current 65th-lane test expects such a refusal. |
| Delivered, CREDIT pending | Keep the charge until real consumer removal; reserve any returned grant before it can be emitted. | Current forwarding owner retains its charge until `Credit`; cross-owner promise accounting is absent. |
| EOF, CLOSE, cancel | Unused promise is released only after no accepted in-flight frame can arrive; queued/delivered charges and failed writes have one terminal owner. | Exact release/Join ordering needs owner-level proof. |
| Nested parent | Each ancestor backs its own outer credit. Inner work either reserves a proven expansion at the ancestor or waits behind safe backpressure without a legal-frame refusal or sibling-control stall; no child grows the finite parent byte/time allowance. | Current nested queue charges actual bytes to its Source parent; neither alternative is proved. |

## Options

1. **Larger prefix/duty queues while retaining 64 KiB initial grants.** This preserves current wire fields but needs at least 32 MiB for 256 lanes in two directions before headers/control/nesting; the 64 MiB duty and memory/qualification budget may also need revision. It cannot be silently called the current 4 MiB contract.
2. **Fewer simultaneously admitted lanes per parent.** This can preserve current grant size and cap only by changing the effective concurrency contract and demonstrating ordinary multi-parent composition and workload. It is not an internal fix while the present lane count is required.
3. **Paired child-credit/profile change with receiving-pool escrow.** Keep the queue and lane ceilings, select complete-frame credit units and bounded initial grants in both directions, then reserve before child OPEN/first CREDIT and each later CREDIT. Parent ACCEPT remains lane-zero admission. A new, peer-agreed generation/profile could assign one fixed implicit encoded-byte initial grant to each child direction without adding a field to every OPEN; a variable per-child grant instead requires an explicit OPEN/initial-CREDIT exchange. Either choice must establish both directions' grant before ordinary BYTES and cover the pending 4,096-byte TLS allowance. Prove a bounded nested expansion reservation or safe backpressure at each nested seam. This needs an exact generation/profile compatibility disposition, an affected-caller cohort and latency evidence; a proposed numeric split alone is insufficient.

## Recommendation

Provisional direction: investigate option 3 if the Product Owner retains both 256 work lanes and the 4 MiB prefix ceiling. It is the only listed direction that preserves both constraints. Confidence is high in the contradiction and low in a specific new initial grant until nested expansion and workload checks are complete. The strongest objection is added round trips or implementation state that could fail the 64 KiB response latency and cleanup invariants. No option is accepted yet.

## Disposition

Open. #78 remains the sole selected C0 research question; #79 is not ready. The exact formula, numbers, wire generation and implementation scope must be promoted to the protected-route protocol owner and, if consequential, an accepted ADR. This record is provenance, not a runtime contract. No experiment code was created.
