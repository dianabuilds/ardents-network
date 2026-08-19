---
id: R-045
title: Which measured Anonymous Cost and local admission profile protects Stage 6 naming surfaces?
status: open
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-045 — Stage 6 Anonymous Cost and admission

## Decision this unlocks

Select finite, measurable admission for claim, renewal, resolution, policy, and
recovery without money, accounts, IP reputation, stable identity, wallet,
token, or personhood claims. S6.5 and its resource verdict remain blocked until
candidate limits are measured on the accepted reference host.

## Current contract

R-003, R-010, R-023, R-039, and the threat model require bounded anonymous work
and explicitly state that a mechanism must be selected and measured. Anonymous
Cost raises mass-abuse cost but proves no personhood, fairness, legitimate use,
or rightful control. Per-endpoint state must not become a stable cross-context
identity.

The previously printed Hashcash difficulty and rate tables were hypotheses, not
measurements. They are no longer frozen and must not appear as accepted limits.

## Hypotheses

- **H1:** bounded SHA-256 work plus local per-epoch counters and short-lived
  scoped capabilities can protect all five surfaces within R-023 budgets.
- **H2:** resource-only local admission without client work is sufficient.
- **H0:** no identity-free mechanism keeps both legitimate accessibility and
  adversarial work bounded; the Stage 6 surface must be reduced.

## Evaluation criteria

1. Predeclare per-surface legitimate throughput, attacker budget, CPU, memory,
   storage, bandwidth, queue, and latency limits.
2. Measure modest and stronger clients; do not calibrate only on the verifier
   host.
3. Bind every token/counter to surface, epoch, operation, and Isolation Context
   without a reusable cross-context identifier.
4. Exhaustion fails `admission-denied` before expensive work and never mutates a
   Lease, policy, recovery, or durable counter incorrectly.
5. Rollback, replay, parallelism, restart, and capability theft fail closed.
6. The mechanism has no payment, global account, IP reputation, operator
   override, or hidden allow-list.

## Evidence plan

### Primary sources

- R-003 and R-010, accessed 2026-08-20 — Anonymous Cost product boundary.
- R-023, accessed 2026-08-20 — reference-host resource methodology.
- R-039, accessed 2026-08-20 — Stage 6 lifecycle and privacy boundary.
- Threat model § naming abuse, accessed 2026-08-20 — selection and measurement
  requirement.
- Candidate proof-of-work and admission specifications must be cited before an
  implementation is selected.

### Experiment

Create `experiments/r-045-anonymous-cost/`. Before running it, freeze a matrix of
the five surfaces, legitimate rates, attacker concurrency, candidate work
factors, capability TTLs, restart/rollback cases, and pass/fail resource limits.
Measure latency distributions, CPU, RSS, queue depth, accepted/rejected work,
energy proxy, and retained state on R-023 reference hardware and a weaker client.

### Failure scenarios

- One cheap request causes materially more verifier work than claimant work.
- A capability crosses surface, epoch, endpoint, or Isolation Context.
- Restart or rollback restores spent capacity.
- Parallel requests bypass a local limit.
- Resolution becomes inaccessible on modest hardware.
- Admission creates a stable identity or operator exception path.

## Findings

- **Sourced fact:** accepted documents require both mechanism selection and
  measurement.
- **Measurement:** none has yet been recorded for the proposed Stage 6 table.
- **Inference:** the former exact values cannot be cited as calibrated limits.
- **Assumption:** SHA-256 work may be implementable with existing supply, but its
  accessibility and amplification behavior are unknown.

## Options

1. **Client work + local counters + scoped capability.** Candidate; potentially
   bounds amplification but needs calibration and privacy analysis.
2. **Local resource admission only.** Candidate; simpler client journey but may
   make distributed Sybil flooding too cheap.
3. **Reduce or delay expensive naming surfaces.** Fallback when no candidate
   fits accessibility and abuse budgets.

## Recommendation

Run the named experiment and choose none until its thresholds are frozen in
advance. Confidence is high that the former table is unsupported; confidence in
any candidate is low without measurements. The strongest argument against proof
of work is that it can exclude weak devices while remaining cheap for specialized
attackers.

## Disposition

- State: `open`; former per-surface rates, difficulties, capacities, and TTL are
  hypotheses only.
- S6.5 admission implementation and evidence predicates remain blocked.
- The experiment must end in a decided record or an explicit reduction of the
  Stage 6 surface.
