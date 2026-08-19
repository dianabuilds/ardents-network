---
id: R-045
title: Which R-010-compatible bounded Anonymous Cost and local admission profile freezes Stage 6 claim, renewal, resolution, policy, and recovery surfaces?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-19
---

# R-045 — Stage 6 Anonymous Cost and local admission

## Decision this unlocks

Freeze the cost mechanism set, the per-surface difficulty and budget
calibrated to the R-023 reference host, the stateless → state admission
order, and the fail-closed behavior so S6.5 (concurrency/fork/abuse)
can implement bounded admission and the verifier can test exhaustion
without ambiguity. Without this freeze, S6.5 would either embed an
unmeasured cost or fall back to identity-coupled admission, which the
R-010 product boundary forbids.

## Current contract

R-010 (decided product boundary) fixes:

- resource-specific staged admission uses cheap stateless validation
  before expensive bounded state;
- may add scoped short-lived capabilities or bounded puzzles under
  load;
- no money, account, IP reputation, stable identity, personhood, or
  fairness claim;
- exact mechanisms remain experiments.

`horizon-3-stage-6-brief.md` S6.5 fixes the surfaces that the cost
must cover and the no-identity / no-fairness rule. R-002 fixes the
Connection Result taxonomy (`admission-denied`). R-005 fixes Time
Confidence; R-029 fixes the Network Epoch; R-041 fixes the
`schema_version`; R-042 fixes the order key; R-046 fixes the role
matrix. R-023 fixes the `2 vCPU` / `2 GiB` Ubuntu reference host used
to calibrate the per-surface difficulty.

What remains open before S6.5 can start is the concrete mechanism,
the per-surface bit-difficulty and budget, the admission order, the
capability shape, and the fail-closed contract.

## Hypotheses

- **H1:** Hashcash-style SHA-256 PoW (k leading zero bits) combined
  with a per-endpoint rate limit and a scoped short-lived capability
  is sufficient to bound the five naming surfaces on the R-023
  reference host, with no identity coupling.
- **H2:** a memory-hard PoW (Argon2id) is required to resist
  commodity-hardware adversaries.
- **H0:** no bounded mechanism satisfies the no-identity rule under
  the R-023 reference profile.

## Evaluation criteria

1. **No identity coupling:** the cost is CPU time plus a counter; no
   account, no IP reputation, no KYC, no personhood, no fairness
   claim, no stable cross-context identifier.
2. **Measurable exhaustion:** the verifier can independently reproduce
   `admission-denied` from the counter and the manifest.
3. **Surface coverage:** every naming surface (claim, renewal,
   resolution, policy, recovery) has a stated bit-difficulty and
   per-epoch hard cap.
4. **Calibrated to R-023:** the per-surface difficulty fits the
   `2 vCPU` / `2 GiB` reference host without starving legitimate
   workloads.
5. **Stateless before state:** the admission order is
   `epoch → role → schema → counter → PoW (when required)`, so the
   cheap path dominates ordinary traffic.
6. **Capability is bounded:** TTL ≤ 60 s, scoped to
   `(endpoint_pubkey, surface, target_epoch)`, and not transferable.
7. **Fail-closed on exhaustion:** `admission-denied` is a terminal
   Connection Result; no retry-with-bigger-cost shortcut, no override
   path.
8. **Reproducible cost on non-reference hosts:** stronger or weaker
   hardware takes proportionally more or less wall time, with no
   differential admission behavior.

## Evidence plan

Primary sources, accessed 2026-08-19:

- R-010 — operational product closure § security (decided).
- R-023 — interactive route performance budget (reference host).
- `horizon-3-stage-6-brief.md` S6.5.
- `stage-6-readiness-checklist.md` §B.5.
- R-002 — Service Connection Connection Result taxonomy.
- R-005 — hostile bootstrap and Time Confidence.
- R-029 — authenticated Node lifecycle (Network Epoch).
- R-041 — canonical name limits and `schema_version`.
- R-042 — claim ordering.
- R-046 — role matrix.
- Adam Back, *Hashcash - A Denial of Service Counter-Measure* (2002),
  for the SHA-256 PoW pattern.
- IETF RFC 9485 / RFC 9458 (OHTTP) — referenced indirectly via R-026
  for capability shape, not adopted here.

The admission path and coverage are implemented in S6.5 against this
contract; no new experiment is required for R-045.

## Failure scenarios

- An endpoint exceeds the stateless limit without PoW and admission
  accepts the request.
- A PoW with `k = 20` bits is accepted for a surface that requires
  `k = 22`.
- A capability with TTL 60 s is used on second 61.
- A capability scoped to `(endpoint_A, claim)` is accepted for
  `(endpoint_A, renewal)`.
- The per-epoch counter is reset between epochs (epoch monotonicity
  broken).
- Exhaustion returns success instead of `admission-denied`.
- The reference host executes a `k = 20` surface faster than the
  calibration expects, indicating an underestimated cost.
- An identity attribute (account, IP, personhood, fairness) is
  consumed as a cost input.

## Options and recommendation

1. **Option A — Hashcash-style PoW + rate limit + capability
   (recommended).** SHA-256 with `k` required leading zero bits;
   per-endpoint per-epoch rate limit; scoped short-lived capability
   (TTL ≤ 60 s, scoped to `(endpoint_pubkey, surface, target_epoch)`)
   for batched operations. Per-surface bit-difficulty and hard cap
   calibrated to the R-023 reference host:

   | Surface | Stateless / epoch | PoW (bits) | Hard cap / epoch | Notes |
   |---|---|---|---|---|
   | Claim (new Lease) | 1 | 20 | 100 | most expensive |
   | Renewal (active Lease) | 10 | 16 | 1000 | medium |
   | Resolution (Name → Target) | 100 | 8 | 10000 | cheapest, cache-friendly |
   | Policy (Recovery Policy add/replace/disable) | 1 | 18 | 10 | visible delay preserved |
   | Recovery (post-pending successor record) | 0 | 22 | 1 per generation | bounded, one-time |

   Admission order:
   `epoch (R-005/R-029) → role (R-046) → schema (R-041) → counter →
   PoW (when required)`. Capability flow: endpoint requests a
   capability by paying the next-tier PoW; receives a token bound to
   `(endpoint_pubkey, surface, target_epoch)`; uses it within TTL.

2. **Option B — Memory-hard PoW (Argon2id).** ASIC-resistant and
   friendlier to commodity hardware. Rejected: requires substantial
   memory under load and is not compatible with the H3 reference
   profile `2 GiB`; verifier cost is asymmetric.

3. **Option C — Pure rate limit, no PoW.** Simplest and least
   expensive. Rejected: distributed flooding remains possible within
   the rate limit; the brief explicitly mentions bounded puzzles
   under load.

4. **Option D — Capability only, no PoW.** Bearer-friendly. Rejected:
   a transferable bearer becomes a stable cross-context identifier
   and violates the R-010 no-stable-identity rule.

Recommendation: **Option A**, accepted by the Product Owner on
2026-08-19.

## Disposition

- R-045 becomes `decided`. The open row in `docs/research/questions.md`
  is updated to point at this record and the frozen contract.
- §B.5 of `stage-6-readiness-checklist.md` is checked.
- S6.5 (concurrency/fork/abuse) may implement the admission path, the
  per-surface cost table, the capability flow, and the fail-closed
  contract. The verifier may independently recompute `admission-denied`
  from the counter and the manifest.
- This freeze does not authorize code; the Stage 6 coding gate remains
  closed until §B.3 and §B.4 of the readiness checklist are also
  checked and the corrected brief, plan, and evidence contract are
  accepted.
- No ADR is required: this is a configuration freeze that uses a
  well-known PoW pattern under an already-decided product boundary
  (R-010).
