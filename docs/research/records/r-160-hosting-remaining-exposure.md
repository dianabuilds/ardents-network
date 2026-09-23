---
id: R-160
title: What remaining-exposure bound pays initial work, refill and termination once?
status: open
owner: network-core
started: 2026-09-23
reviewed: 2026-09-23
---

# R-160 — Remaining hosting exposure for one forwarding parent

## Decision this unlocks

Issue [#84](https://github.com/dianabuilds/ardents-network/issues/84) needs one accounting formula and numeric oracle before [#85](https://github.com/dianabuilds/ardents-network/issues/85) changes the real Resource/Node/Route owners. The formula must pay for a parent already admitted, its exact additional promise after a class-2 refill, and the termination work still owed. It does not select a provider tariff, operator allowance, public resource policy, reboot reconciliation or implementation API.

## Current contract

The [closed protected Route](../../technical/protected-route-protocol.md) and [ADR-0085](../../adr/0085-bound-forwarding-replenishment.md) fix a 32 MiB parent remaining allowance after a valid later ADMIT: debit the whole ADMIT under the old allowance first, then set remaining to exactly 32 MiB, never add 32 MiB. Before token spend, the installed host reserves the **additional** allowance and termination capacity not already held. The original deadline, authenticated receiver, HELLO, peer and purpose do not change. The [qualification owner](../../development/privacy-qualification.md#counting-useful-work-and-hosting-cost) requires locally configured actual provider period, units, counted directions, interfaces and allowance; it separates measured tx/rx, reserve, low-watermark refusal and bounded drain. The [Network/Node owner](../../technical/network-route-node.md#node-and-resource-lifecycle) gives Resource pressure measurement but Node/Route the admission and shutdown reaction. The [threat model](../../security/threat-model.md) forbids a new child/context from multiplying an ancestor budget.

At dev 7e381928 (2026-09-23), internal/resource/hosting_allowance.go stores provider-counted Used, aggregate Reserved, original counter reading and policy in one durable period. Observe adds the complete interface-counter delta, including other processes, without attributing it to a reservation. hosting_ledger.go serializes Reserve/Release under the same period lease. Reserve adds cost(work)+cost(termination); Release subtracts the handle's full reservation only after the consumer joins. A lost handle leaves the durable debit. In internal/node/closed_forwarding_admission.go both initial admission and every refill call the same full Reserve(local.AdmissionTraffic, local.TerminationTraffic). In internal/route/closed_forwarding_channel.go the refill callback receives token/HELLO/deadline but **not** the already debited parent remaining amount; Route then resets its remaining allowance to 32 MiB. Thus current code cannot calculate exact incremental parent allowance or know whether termination was already reserved from that callback alone.

## Hypotheses and falsification

- H1: a conservative gross-exposure bound U+R, with each new *incremental* work promise and only the missing termination promise added once, protects the provider period without attributing host counters to jobs.
- H2: subtract observed host delta from live reservations to free capacity earlier. A delta caused entirely by another process falsifies its safety.
- H0: if qualification demands an exact non-overlapping physical remaining-byte value while work runs, aggregate interface counters and one aggregate Reserved field cannot supply it; per-job attribution and a separate accepted owner are required.

H1 is falsified if an admitted parent can legally spend more than its declared remaining directional envelope plus held termination, if two concurrent refills both pass using the same free bytes, if a failed counter observation releases capacity, if a refill reserves termination twice, or if a release before child/transport Join refunds uncompleted work.

## Evaluation criteria and evidence plan

Use only provider-counted bytes for the actual declared direction: C(Tx,Rx)=Tx, Rx, or checked Tx+Rx. Perform checked integer arithmetic before accepting. Require a current period, continuous counters, original deadline within the period, and at least the configured low-watermark after each new reservation. The observer's inability to attribute aggregate traffic is a limitation, never evidence that a live reservation has been spent. Keep termination protected until the parent and every child/transport operation joins. An unknown counter, boot/interface discontinuity, failed durable commit or lost handle cannot become free capacity.

Primary sources reviewed 2026-09-23: accepted ADR-0085 and current owners above; internal/resource/hosting_allowance.go, hosting_ledger.go, hosting_sample.go and hosting_ledger_linux_test.go; internal/node/closed_hosting.go, closed_forwarding_admission.go, closed_forwarding_host.go; internal/route/closed_forwarding_channel.go and its tests. The numeric ledger below is an arithmetic oracle, not a measured provider bill or complete network cost. No runtime experiment is necessary to compare these formulas; #85 must test the chosen semantics at its actual callers.

## Findings

**Sourced fact:** Current durable RemainingBytes is max(0, B-U-R) when the period and counters are valid, where B is the provider allowance, U the measured provider-counted used floor and R the aggregate committed reservation. Low-watermark protection starts at RemainingBytes <= L. A Reserve transaction measures first, then adds the full requested work plus termination and refuses if the new remaining would be below L. A Release measures first and subtracts that handle's reservation. There is no per-job attribution in U.

**Inference:** Because a new host delta may come from the admitted parent, an unrelated process, retransmission or control traffic, subtracting it from R without independent attribution can underpay a still-live promise. Retaining R gross while U rises is a deliberately conservative upper bound on exposure. It may count the same physical bytes in U and in a still-held *possible* work envelope until Join; it must not be described as exact physical remaining capacity or charged twice to the provider. The accepted safety requirement is that each reservation increment occurs once, each measured delta enters U once, and uncertainty never frees a reservation. If this conservative availability cost is unacceptable, a new per-owner metering/representation decision is needed; #84 does not silently invent it.

**Sourced fact:** Route's parent byteLimit-usedBytes is the remaining inbound ARDP promise after the later ADMIT frame. On successful refill, the new remaining value is M=32 MiB. The incremental parent promise is therefore D=M-oldRemaining, with 0<=oldRemaining<=M. For example, oldRemaining=8 MiB means D=24 MiB, not 32 MiB. Child allocations and the original deadline remain unchanged. Node's current refill callback has no D, and the current fixed AdmissionTraffic is a declared directional upper envelope, not a mathematical conversion function from D to provider bytes.

**Inference:** To reserve the exact added obligation, #85 needs a trusted D (or equivalent before/after allowance) from Route and a declared directional upper-bound function W(D, context) for the additional interface traffic, including control, TLS/Carrier overhead and bounded retransmission allowed by the original deadline. Proportional scaling of today's AdmissionTraffic is unsafe unless the owner proves linearity and covers fixed per-refill costs. The actual provider's C then prices W. Reserve once for C(W(D,context)) and only C(T_missing), where T_missing is a proven additional termination envelope beyond what this parent still holds. A fresh token alone does not grant a second termination reserve. If W or T cannot be bounded, refuse the refill before spending its token. Do not substitute the 32 MiB inner frame limit for a provider-interface upper bound.

**Inference:** The current aggregate durable (U,R) representation can hold incremental reserve increases without a new persisted per-job schema if the live parent owns one cumulative reservation and lost/ambiguous handles remain debited as today. That is conditional on #85 proving atomic increase and conservative failure handling; it does not promise recovery or refund after process death. An exact post-crash per-parent refund or per-job spend attribution would require a separate persisted representation decision outside #84.

## Formula for decision

At a valid observation, define F=B-U-R with checked nonnegative arithmetic; otherwise F=0 and admission unavailable. For a new parent reserve R_parent=C(W_initial)+C(T_initial). For a refill after the ADMIT debit, D=M-oldRemaining and add R_refill=C(W(D,context))+C(T_missing) once. T_missing is zero when the existing parent termination reserve still covers the selected finish. Admit only when F> L, R_refill <= F and F-R_refill >= L, with all authority/deadline/counter/storage checks satisfied. Treat the existing R as live until joined; never credit observed aggregate host delta against it. Release exactly the parent's cumulative reservation only after its parent, children and carrier cleanup join, while U retains every measured delta. If an attempted reserve or spend fails, preserve the already measured delta and do not grant the new parent allowance.

This formula fixes accounting relationships, not a numerical universal W or T. The operator's declared envelope must be validated against the selected Route/Carrier work and termination path during #85 and qualification; a positive local configuration value alone is not proof that its bound is payable.

### Numeric oracle, provider counts tx+rx

Let B=1000 bytes, low watermark L=100, initial used U=100, and no other reservations. The selected initial directional work bound costs 300 and one termination bound costs 100. Initial reserve makes R=400 and F=500. A later aggregate observation of 70 bytes makes U=170, R=400, F=430; do not guess how much of the 70 was this parent.

After a valid ADMIT is debited, suppose the selected directional W(D,context) costs 180 and existing termination still covers finish. One refill adds 180, so R=580 and F=250. A concurrent different parent's attempted increment of 160 must refuse because it would leave F=90<L; a 140-byte increment may commit first and leave F=110, after which the 160-byte contender still refuses. Each decision runs under the shared period lease and observes the latest committed R. If another measured 80 bytes arrives, U=250, R=580, F=170. After the parent and all children join, Release makes R=0 while U remains 250, so F=750. If measurement or storage becomes uncertain before that Release, the debit remains and no inferred refund occurs. Reserving the 100-byte termination envelope again at refill would change R to 680 and falsely deny an otherwise payable competing increment; it does not buy a second finish.

## Options and recommendation

1. **Gross shared-period exposure with incremental parent reserve and one retained termination bound (recommended).** Fits existing provider counters and ADR-0085; fail closed on missing W/T proof. Availability is conservative during live work because U has no job attribution. #85 must give Route→Node the D and retained termination state, and verify cumulative release at Join.
2. **Subtract aggregate observed deltas from each live reservation.** Appears to free capacity sooner but can spend the same host allowance twice when another process generated the delta. Reject.
3. **Re-reserve a full parent and termination on every refill.** Safe only if those full envelopes are real upper bounds, but duplicates already-held allowance and can refuse a payable refill near the watermark. It does not satisfy the selected "additional" ADR-0085 rule. Reject as the #84 target.
4. **Exact per-job provider attribution and crash recovery.** Would need independent measurement and a new durable owner/representation. Do not fold into #84/#85 without a separate contract.

Recommend option 1 with conditional confidence. The ledger arithmetic and failure direction are source-backed; the strongest objection is that W(D,context) and a selected, payable termination upper bound are not yet specified by the current local profile. This is a real contract gap for accepting #84 and starting #85, not an excuse to use a guessed multiplier, extend deadlines or weaken the low watermark. Keep #84 open until the affected owners accept those concrete bounds and the numeric oracle. No runtime, storage, tariff, dependency or ADR change is made by this record.
