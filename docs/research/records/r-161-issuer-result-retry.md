---
id: R-161
title: Finite recovery of one committed issuer RESULT after loss
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-161 — How is one committed issuer RESULT recovered after loss, including the last Control token?

## Decision this unlocks

Issue [#93](https://github.com/dianabuilds/ardents-network/issues/93) must select a finite, already-paid access budget before conditional implementation [#94](https://github.com/dianabuilds/ardents-network/issues/94) can make a real stock/refill caller recover an exact committed RESULT. This record proposes one loss tolerance; it is **not an accepted admission contract**. Job-cancel retention belongs to #95.

## Current contract

The [C0 scope](../../product/scope.md) requires finite budgets, explicit unavailability and durable authority floors. The [threat model](../../security/threat-model.md) includes malicious peers and availability pressure. The [private-admission owner](../../technical/private-admission.md#issuance-ambiguity-and-spending) fixes one signed request ID/digest, one whole-batch durable issuer reservation, an exact same-kind retry, volatile pending blind state, a 16 KiB OPERATION/RESULT pair, and class-1 admission before ordinary issuer work. The [Endpoint runtime owner](../../technical/endpoint-service-runtime.md) retains one operation owner and an exact Source acquisition. A retry may not renew permission, State, Source or bootstrap allowance.

The issuer ledger currently returns an existing request ID/digest reservation without a second quota debit. This covers result recomputation, not access to the issuer. The receiving Spend ledger rejects duplicate token presentation. An ordinary new issuer child spends a fresh class-1 Control token before OPERATION. This fails when the first child's RESULT is lost after commit and its token was the last usable stock item.

## Hypotheses and predeclared falsification

- **H1:** One exact replay of the *original* spent Control token, retained only with the same in-process pending batch, can be admitted as one restricted recovery child after the issuer has durably committed that batch. The receiver binds the original Spend to request ID, digest, kind and original admission; permits one further presentation only after verifying the exact signed batch against that reservation; and durably exhausts the recovery allowance before releasing RESULT. A changed digest, kind, permission, token, profile or owner cannot claim it.
- **H2:** One fresh Control token per retry suffices without a special recovery admission.
- **H0:** No finite access mechanism can meet the last-token case under the current admission semantics.

Falsify H1 if original token bytes cannot be retained only inside the same pending owner; if a duplicate can reach general work, reserve another batch, bypass first Spend, or be admitted without a committed exact reservation; if a crash/ambiguous durable write grants another recovery; or if one loss still cannot reach the old result with no fresh stock. Falsify H2 with a one-token stock after the original admission and a lost committed RESULT. The existing code already supplies that counterexample.

## Evaluation criteria

The success outcome is the original issuer's exact committed signatures, finalized only against the original volatile blinding state, after at most one lost RESULT. Refusal after the recovery opportunity is exhausted is finite and explicit. No user request, Job, permission, issuer or bootstrap switch occurs. Privacy remains the existing terminal TLS confidentiality boundary; a retry makes another observable connection and must not be called anonymity. A malicious holder with a spent token must not obtain general issuer service or unbounded channel work. The original host, duty, parent, queue, time and permission ceilings still apply.

The implementation must have one bounded admission path with an existing non-test caller on both selected Carriers; durable Spend/issuer facts must survive restart without creating a token refund. No new dependency or cryptographic primitive is justified. Operationally, a lost RESULT may consume availability; it must not create an unbounded retry queue or a new operator.

## Evidence plan and findings

Primary local sources, inspected 2026-09-23 at dev@`7e381928dd4bd906a1d623ca78a2dedf3738829c`:

- `internal/endpoint/text_issuer_stock.go`: stock preparation refills only for requested work; `presentTextIssuerToken` takes a fresh stock item on every ordinary attempt.
- `internal/endpoint/text_token_transfer.go`: `takeTextTokenLocked` removes the stock item before a durable attempt mark and returns only its bytes to Route.
- `internal/endpoint/text_issuance.go` and `text_issuance_operation.go`: the pending batch pins request bytes and opaque blinding state; transport error retains that batch for explicit same-process retry, subject to original permission deadline.
- `internal/route/credential/closed_token_issuer_ledger.go` and `closed_token_issuer.go`: exact request ID/digest and kind hit the prior durable reservation before quota check; the result is recomputed. A changed digest is unavailable.
- `internal/route/closed_issuance_operation.go`, `closed_admission_channel.go` and the private-admission owner: one OPERATION and RESULT are each 16,384 bytes; class-1 child has 64 KiB per-direction credit and at most 30 seconds. The forward parent has its own 32 MiB reserve and original deadline.

**Sourced fact:** The code and owner establish distinct issuer debit and receiver admission/spend. **Inference:** A second ordinary transport attempt currently needs another Control token even though the batch debit is already committed. **Inference:** A zero-stock last-token state makes H2 false. **No measurement:** Neither loss probability nor host capacity for a recovery child was measured here.

**Sourced fact, additional boundary audit:** `internal/route/closed_spend_ledger.go` persists only token digest and receiving-local expiry, rejects a duplicate before any operation, and has no request binding or recovery counter. `internal/route/credential/closed_token_admitted.go` calls `ClosedAdmissionChannel.Accept(ADMIT)` before reading OPERATION, then passes only the operation body and admission kind to `issueTerminalOperation`; `closed_token_issuer_ledger.go` retains request ID/digest/kind/count but no original token digest or recovery use. Thus H1 needs a newly specified durable association and one-use recovery state, with migration/refusal of older journal entries, as well as a narrowly provisional admission path. The current persisted formats and caller interface do not already pay or prove that right. This is a consequential new persisted/admission boundary under #93's split rule; [conditional card #264](https://github.com/dianabuilds/ardents-network/issues/264) owns its acceptance before #94 implementation if H1 is chosen.

Reproduce the counterexample in #94's real path: start with exactly one class-1 issuer token and a prepared batch, allow the first OPERATION to append its issuer reservation, drop its RESULT, then explicitly retry while its pending blind state and permission are live. Observe one issuer debit, first token removed and journaled, and current second presentation failing for lack of stock. Do not create a test-only owner or treat a retry on a green path as this loss experiment.

## Candidate budget and numerical oracle for #94

This is a **proposed** whole-operation budget, not an accepted extension of present admission semantics:

| Quantity | Proposed ceiling for one batch |
|---|---:|
| Committed issuer reservation/debit | 1 batch of 1–32 blind requests |
| Distinct Control tokens consumed | 1 original token; 0 new tokens for recovery |
| Receiver admissions and protected child channels | 2 total: original plus one restricted recovery |
| Lost RESULTs tolerated | 1; another loss yields terminal unavailable |
| Child traffic ceiling | Existing 64 KiB per direction per child; at most 128 KiB per direction across two children, charged to the existing parent/host |
| OPERATION/RESULT plaintext | 16,384 bytes each per attempted exchange |
| Recovery deadline | One original 30-second total window: cap the first child at 15 seconds and the recovery child at the remaining 15 seconds; both also obey the earlier permission, current profile/duty, caller and parent terminal bounds. No fresh 30-second lease |
| Additional bootstrap batches or issuer choices | 0 |

The first admission must reserve or otherwise prove payable the additional restricted child and a full 30-second window before promising recovery. If the original authority or parent has less time, or host/duty/parent capacity cannot pay both children, refuse before issuer debit rather than leave a committed batch with an illusory recovery guarantee. The 15+15 split guarantees a scheduled retry opportunity after a first RESULT loss, conditional on the network and issuer remaining reachable; it does not guarantee delivery against an adversary or outage. Whether one recovery child can be pre-reserved under the current host accounting is an implementation and capacity proof obligation; #84/R-160 is not accepted evidence for it. A provisional duplicate presentation may parse only the fixed request needed to prove the committed exact reservation, under the pre-admission rate/concurrency/byte cap. It may not expose a general tokenless issuer API. The original token bytes stay volatile with the pending batch and are erased on success, terminal exhaustion, revocation or process crash. A crash loses the ability to unblind, as the current contract already states.

A numeric falsification oracle: one-token stock, 32-token pending batch, first admission and committed ledger record, first RESULT lost, second restricted admission using the same token material, then the exact 32 old signatures with **one** issuer reservation, **one** receiver Spend, **two** counted child admissions, no extra permission allocation, and no more than the original deadline. Repeat with a second RESULT lost: no third admission. Change one batch byte or request kind: no old RESULT and no new issuer debit through recovery. Repeat over TCP/TLS and QUIC, and with cancellation/revocation at every durable boundary.

## Options

**H1, one-use exact spent-token recovery (recommended for Product Owner decision).** Meets the last-token case without a new issuer or token. It changes receiver admission semantics and needs a new persisted original Spend-to-reservation binding plus an exhausted recovery marker. The old issuer and Spend journals have no field that can be silently reinterpreted as this entitlement. The strongest objection is that a provisional duplicate admission becomes an availability attack surface; its restricted parser, resource reservation and crash semantics must be proven before implementation. If that binding requires a new wire format, authority surface or independent resource owner, split the new acceptance result into a linked card before #94 implementation.

**H2, stock-funded retry.** Keeps present receiver rules and is simpler, but cannot satisfy the explicit last-token acceptance case; reject for #93.

**H0, honest bounded unavailability.** Keeps current admission rules and can declare the last-token RESULT irrecoverable. This changes #93/#94 product outcome and requires an explicit Product Owner decision if H1 cannot be made safe.

## Recommendation and disposition

Ask the Product Owner to choose H1 as a *design direction* for exactly one lost committed RESULT, or explicitly accept H0 and revise #93/#94. Confidence is high in the current-path contradiction and moderate in H1 feasibility until the durable binding, rate bound, and host reservation are validated. No maintained owner, ADR, runtime or wire grammar changes are accepted by this research draft. R-161 remains selected until the consequential admission decision is made; #94 is not Ready. The research record is provenance; any accepted contract must be promoted to `docs/technical/private-admission.md` and the affected Endpoint/Route owner before implementation.
