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

The present ARDP generation is 3. OPEN has no credit field; ACCEPT carries a u32 credit and CREDIT carries a u32 increment. The current source and forwarding implementations initialize both lane directions at 64 KiB. No public or long-term protocol commitment follows from this closed-alpha grammar.

## Hypotheses and falsification

- **H1, current promise is backed:** every admitted lane may use all credit already granted without refusal solely because an ancestor DATA queue is full. A legal simultaneous burst that exceeds an ancestor cap falsifies H1. The arithmetic and existing queue test below already falsify it.
- **H2, internal-only reservation is sufficient with all selected maxima unchanged:** keep 64 KiB initially granted in each direction, admit the selected lane count, and keep the 4 MiB prefix cap. A summed promise above the cap falsifies H2. It is falsified even before frame overhead.
- **H3, explicit paired credit change can back the promise:** account complete data frames against a shared prefix and duty escrow before advertising credit, with a separately reserved control path; require a bounded grant for each direction at opening and return capacity only after actual consumption or joined retirement. This remains a candidate until nested expansion, compatibility and workload latency are checked.
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

**Sourced fact:** `TestClosedForwardingChannelBoundsOnePrefixQueueBeforeDutyQueue` opens 64 lanes, queues 64 KiB on each, opens lane 65, and expects the latter lane's 64 KiB BYTES to be refused by the full prefix queue. The frame is within lane 65's already initialized credit. This test proves the storage cap but cannot prove the credit promise.

**Inference:** With both the current lane count and prefix cap held, no accounting change confined to local counters can back all initially advertised 64 KiB promises. Either initial grants, admitted concurrency or the cap must change explicitly. A paired wire change is needed if the 256-work-lane and 4 MiB choices remain binding: OPEN currently has no grant for the reverse direction, and both directions currently assume 64 KiB.

**Sourced fact:** `ClosedForwardingChannel.NextAvailable` retains a delivered DATA charge until `Credit` after downstream consumption. `QueueReverse` charges complete reverse frames, including their 16-byte header, while forward `bytes` charges the payload to the shared queue. `ClosedDutyLimits` accounts current queued data, not promises. These asymmetric counters must be reconciled before selecting units for a new grant.

**Open evidence:** The maximum complete outer-frame/TLS expansion of one nested inner grant and the minimum per-lane grant compatible with the selected text workload have not been derived or measured. Source allows up to 258 lanes including two reserved controls, while the current forwarding `open` map rejects at 256; the #78 capacity calculation must use the accepted count and must not silently treat this implementation discrepancy as a new product decision.

## State-transition table to close before decision

| State | Required capacity invariant | Current evidence |
|---|---|---|
| Initial admission | Sum of unspent grants in both directions plus retained DATA must fit every applicable prefix and duty pool; control remains reserved. | Violated by the current fixed 64 KiB grants under a 4 MiB prefix. |
| Simultaneous lawful burst | Receiving a complete frame moves its charged size from promise to retained work without increasing total escrow; no full-queue refusal within a grant. | Current 65th-lane test expects such a refusal. |
| Delivered, CREDIT pending | Keep the charge until real consumer removal; reserve any returned grant before it can be emitted. | Current forwarding owner retains its charge until `Credit`; cross-owner promise accounting is absent. |
| EOF, CLOSE, cancel | Unused promise is released only after no accepted in-flight frame can arrive; queued/delivered charges and failed writes have one terminal owner. | Exact release/Join ordering needs owner-level proof. |
| Nested parent | Inner promise includes a conservative complete outer/TLS expansion against each ancestor; no child grows a parent's finite byte/time allowance. | Current nested queue charges only actual queued bytes; expansion bound remains open. |

## Options

1. **Larger prefix/duty queues while retaining 64 KiB initial grants.** This preserves current wire fields but needs at least 32 MiB for 256 lanes in two directions before headers/control/nesting; the 64 MiB duty and memory/qualification budget may also need revision. It cannot be silently called the current 4 MiB contract.
2. **Fewer simultaneously admitted lanes per parent.** This can preserve current grant size and cap only by changing the effective concurrency contract and demonstrating ordinary multi-parent composition and workload. It is not an internal fix while the present lane count is required.
3. **Paired generation/credit change with hierarchical escrow.** Keep the queue and lane ceilings, select complete-frame credit units and bounded initial grants in both directions, then reserve before OPEN/ACCEPT/CREDIT. This needs exact compatibility disposition, nested expansion and latency evidence; a proposed numeric split alone is insufficient.

## Recommendation

Provisional direction: investigate option 3 if the Product Owner retains both 256 work lanes and the 4 MiB prefix ceiling. It is the only listed direction that preserves both constraints. Confidence is high in the contradiction and low in a specific new initial grant until nested expansion and workload checks are complete. The strongest objection is added round trips or implementation state that could fail the 64 KiB response latency and cleanup invariants. No option is accepted yet.

## Disposition

Open. #78 remains the sole selected C0 research question; #79 is not ready. The exact formula, numbers, wire generation and implementation scope must be promoted to the protected-route protocol owner and, if consequential, an accepted ADR. This record is provenance, not a runtime contract. No experiment code was created.
