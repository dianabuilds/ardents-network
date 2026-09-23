---
id: R-164
title: One Publication parent-expiry transition
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-164 — What happens when one Introduction parent expires?

## Decision this unlocks

Issue [#80](https://github.com/dianabuilds/ardents-network/issues/80) needs one finite current/previous Registration outcome at the original Introduction parent expiry. This record proposes an explicit unavailable boundary for the present C0 Publisher. It does not authorize a successor parent, runtime work under [#81](https://github.com/dianabuilds/ardents-network/issues/81), or a new availability claim.

## Current contract and hypotheses

The [product scope](../../product/scope.md) requires finite lifetimes and explicit unavailability when authority is absent. The [Endpoint owner](../../technical/endpoint-service-runtime.md) already selects a verified Descriptor ACK before a replacement Registration accepts, and predecessor visibility only until the earlier of 60 seconds after first switch and signed Registration expiry. Withdrawal stops new admission before network I/O and joins refresh. [Private admission](../../technical/private-admission.md) bounds one class-3 Registration to 600 seconds, one slot, 1 MiB, 16 pending capsules and four deliveries per second. A Route parent never grants a child a later deadline.

- **H1, seamless parent successor:** the current owners can admit a second Introduction parent, ACK a new Registration and preserve old-reader completion across the old expiry within already selected budgets.
- **H2, terminal unavailability:** the current Publisher admits no replacement parent in this run. The old Registration ends at its original bound; any separately authorized future start must establish fresh authority and cannot revive it.
- **H0, no choice yet:** neither outcome can be made exact without another authority, storage or admission decision.

Falsify H1 by a single-owner constraint that forbids a second live parent or by an old child that cannot finish after its parent's original expiry. Falsify H2 by a current non-test caller that opens and publishes a next parent in the same run after the old one ends.

## Method and source evidence

Inspected current product, threat, Endpoint and private-admission owners, then the listed Go owners at dev@7e381928dd4bd906a1d623ca78a2dedf3738829c on 2026-09-23. Reproduce with searches for openTextIntroductionPrefix, rotateTextPublication, openingAvailableLocked, RegisterIntroduction, commitAcknowledgedLocked and inspectTextIntroductionDelivery in internal/endpoint and internal/route. This is a source trace, not a timed network experiment or a qualified availability measurement.

**Sourced facts:**

1. Publisher startup opens exactly one Introduction prefix, then makes Registration revision 1 with expiry no later than the prefix's recipient duty and 600 seconds. The private Introduction lifecycle holds one live handle and one opening; openingAvailableLocked refuses a second live prefix. No ordinary later Publisher caller reopens that prefix.
2. Route RegisterIntroduction rejects a request whose expiry is after the selected peer duty or prefix plan deadline. It opens a child lane with the request expiry. IntroductionRecipient returns the earlier duty/plan end. Thus a new Registration under the old parent cannot acquire a later signed deadline, and an old reader cannot be promised survival past that parent's bound.
3. Refresh is one scheduler, first due 300 seconds after the registered creation time. It rotates Registration under the **same** Introduction prefix; it does not create the next parent. If the current Registration expires first or rotation fails except the specific Source conflict-read retry, refresh detaches and closes current/pending/previous Registrations. It has no second attempt after terminal failure. A parent whose remaining time is below 300 seconds can cause expiry before the first refresh.
4. Pair lifecycle has current, pending and previous fields. Opening requires no pending; refresh also requires no retained previous. Before ACK, current remains selected and pending is not accepting. At the first verified ACK, the pair switches locally and the old Registration may be selected only until min(first switch + 60 seconds, its signed expiry). A duplicate ACK retains this cutoff. Admission checks exact slot/revision, publication readiness, live channel, capsule expiry and local authority.
5. Withdrawal marks the pair draining before joining refresh and network withdrawal. A late ACK cannot commit after the drain. Previously admitted work retains its own original deadlines and the withdrawal's additional five-second drain ceiling; neither is renewed by a new Descriptor or retry.
6. One prefix and at most two simultaneously held Registration channels are reachable in the ordinary rotation path: current + pending before ACK, or current + previous after ACK. An in-flight opening replaces the pending position, not a third accepted Registration. Each Registration has its own class-3 ceiling. These are structural maxima of the inspected path, not measured host-wide resource maxima.

**Inference:** H1 is false for the current owner graph. A seamless successor needs a second parent owner and independently admitted class-2 stock while the old parent remains live, or a different serial restart contract. Those are new admission/cleanup outcomes; this issue cannot silently introduce them. H2 matches the current terminal behavior, subject to #81 verifying it through the actual Publisher path on both Carriers.

## Proposed H2 boundary matrix

Let T be the old parent's original terminal deadline, R its Registration's signed expiry (R <= T), and A the first verified replacement Descriptor ACK if rotation under that same parent succeeds. No current Registration or child gets a deadline later than the earliest applicable bound.

| Case | Exact proposed outcome |
|---|---|
| Old reader admitted before T | May finish only within its already admitted child/Registration/parent deadlines. Expiry or transport closure ends unfinished work; no migration or new child on the old parent. An already established Service Connection has its separate original lifetime and must not be represented as a surviving Registration reader. |
| New reader before A | Uses only the old acknowledged pair while its channel and authority remain live; pending is refused. |
| New reader after A but before R/T | Uses new acknowledged pair. Old slot/revision is retained only until min(A + 60 seconds, R), and only while its channel/authority remain live. |
| At or after old parent T, or R if earlier | Old channel and child admission terminate. If no independently accepted new parent exists, new Introduction/Service setup is explicitly unavailable; no implicit fallback or signed-expiry extension. |
| Pending Registration or ACK crossing R/T | If old parent/channel dies before verified commit, pending never becomes accepting. An ACK after expiry/drain/context loss is rejected, and cleanup joins the pending channel. A late network ACK alone cannot create local readiness. |
| Failed next-parent attempt | There is no next-parent attempt in this selected run. A later explicit start, if authorized by its own owner, must fail closed on State, permission, duty, token or Route failure and cannot restore the old pair or its previous overlap. No recovery guarantee is selected here. |
| Withdrawal on either side of expiry | Drain barrier closes admission first; cancel and join scheduler, opening and owned channels in existing order. Repeated withdrawal and late ACK cannot move the original deadline or revive readiness. A withdrawal after terminal expiry may report unavailable. |

The maximum extra parent resources selected by H2 are **zero**. Ordinary refresh retains at most one parent and two Registration channels, never two independently accepting parent generations. The per-Registration class-3 limits remain unchanged. If the Product Owner instead selects seamless H1, first define the second-parent admission, overlap and cleanup budget in a separate linked decision before changing #81; its two-parent resource maximum cannot be inferred from the current class-3 bound.

## Verification and recommendation

Recommend H2 for this bounded C0 transition, with moderate confidence in exact runtime reachability and high confidence that the current code does not contain a second-parent transition. Strongest objection: it exposes an avoidable outage when the parent expires. That outage is honest under the present authority and resource model; a seamless route is a separate product/admission choice.

After acceptance, promote the matrix to the [Endpoint owner](../../technical/endpoint-service-runtime.md) and the explicit parent/child limit to [private admission](../../technical/private-admission.md). #81 should first test actual Publisher/Endpoint paths over TCP/TLS and QUIC: R<T, R=T, ACK just before/after R, old reader, new reader, failed open, withdrawal and late ACK, with original deadlines and joined cleanup. If those oracles already pass without a runtime delta, close or rescope #81 on evidence rather than manufacturing a code change. The current accelerated scheduler tests do not qualify a full parent-expiry run.

This draft record and queue entry are decision preparation only. The Product Owner has not accepted H2; #81 remains conditional.