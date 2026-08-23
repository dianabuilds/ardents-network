---
id: R-045
title: Which measured Anonymous Cost and local admission profile protects Stage 6 naming surfaces?
status: decided
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
- [Hashcash papers](https://www.hashcash.org/papers/), accessed 2026-08-20 —
  non-interactive publicly auditable hash work, expiry, and service-string
  binding; this supports a candidate cost function, not an anti-Sybil or
  fairness claim.

### Experiment

The now-retired disposable experiment froze a matrix of the five surfaces,
legitimate rates, attacker concurrency, candidate work factors, capability
TTLs, restart/rollback cases, and pass/fail resource limits. It measured
latency distributions, CPU, RSS, queue depth, accepted/rejected work, energy
proxy, and retained state on R-023 reference hardware and a weaker client.

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
- **Measurement:** O1 failed every predeclared honest solve-latency gate on both
  the Windows development endpoint and the weaker Linux profile.
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

## Predeclared candidate O1 — scoped challenge work

O1 was not accepted and is retained as the predeclared failed profile. The
Product Owner accepted the separately predeclared O1b profile on 2026-08-20.

Each admitting Node issues a stateless HMAC-SHA-256 challenge bound to its
random boot secret, Node identity, network/epoch, surface, operation digest,
Isolation Context, expiry, and a fresh `16-byte` nonce. The claimant searches a
`uint64` nonce until SHA-256 of the canonical challenge/proof transcript has the
required number of leading zero bits. Verification performs one HMAC and one
SHA-256 evaluation before any expensive naming work.

Successful proofs are single-use. Each surface owns a finite spent-digest set;
when it is full, new work fails `admission-denied` rather than evicting a live
entry or growing memory. Restart creates a new boot secret, so every pre-restart
challenge fails closed without restoring spent capacity. The proof is local to
one Node and one Isolation Context and is carried inside the accepted private
naming path; it is neither a global identity nor a transferable capability.

The experiment starts with these hypotheses, not accepted limits:

| Surface | Initial work bits | Maximum outstanding/spent entries | Target honest p95 solve time |
|---|---:|---:|---:|
| exact-name resolution | 18 | 4,096 | `<= 100 ms` weaker client |
| renewal/update | 19 | 2,048 | `<= 200 ms` weaker client |
| policy/recovery | 20 | 1,024 | `<= 350 ms` weaker client |
| root claim commit/reveal | 22 | 1,024 | `<= 1 s` weaker client |

The experiment rejects O1 if verifier work exceeds `100 us` p95, proof state
exceeds `1 MiB` per surface at its cap, any cross-surface/epoch/Node/context
replay succeeds, parallelism bypasses the cap, restart accepts an old proof, or
the weaker-client latency target fails. Passing does not establish fairness,
personhood, energy efficiency, or resistance to specialized hardware.

### O1 measurement — rejected

The harness used `20` deterministic challenges per surface and `100,000`
successful verification iterations per surface. The Windows endpoint reported
Go `1.26.6`, `windows/amd64`, an `AuthenticAMD` family 26/model 68 processor,
and 12 logical processors. The weaker run used the pinned
`ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`
image with `--network none`, `1 vCPU`, `512 MiB`, 64 PIDs, a read-only root,
and all capabilities dropped. Its binary SHA-256 was
`e9b8b8a48a8770fa6f741227d531f6acc22e74586f30c173992b44f06c99125a`.

| Surface | O1 bits | Windows solve p95 | Weaker Linux solve p95 | Linux verify p95 | Heap at cap |
|---|---:|---:|---:|---:|---:|
| exact-name resolution | 18 | 316.63 ms | 340.90 ms | 1.47 us | 394,584 B |
| renewal/update | 19 | 443.02 ms | 835.99 ms | 2.38 us | 197,784 B |
| policy/recovery | 20 | 1,066.95 ms | 1,119.40 ms | 1.59 us | 99,400 B |
| root claim | 22 | 8,984.37 ms | 5,098.40 ms | 1.39 us | 99,496 B |

O1 is rejected: all four weaker-client solve p95 values exceed their frozen
limits. Verification, retained heap, hostile scope/replay/restart cases, and
the one-winner parallel test passed, but they cannot rescue the accessibility
failure. Windows sub-millisecond verification quantiles are omitted because
that clock reported zero-duration samples.

### Predeclared candidate O1b — accessible work and bounded admission

O1b changes only the failed work factors and adds an explicit no-wait
verification concurrency bound; it does not reinterpret O1. The spent caps,
TTL maximum, transcript, hostile corpus, and all earlier pass/fail gates remain
unchanged.

| Surface | O1b work bits | Maximum in-flight verifications | Solve p95 gate |
|---|---:|---:|---:|
| exact-name resolution | 16 | 64 | `<= 100 ms` |
| renewal/update | 16 | 32 | `<= 200 ms` |
| policy/recovery | 17 | 16 | `<= 350 ms` |
| root claim | 18 | 8 | `<= 1 s` |

The implementation must reject immediately when a surface's in-flight slots
are full; it must not create an unbounded application queue. O1b will use the
same `20` solve samples and `100,000` verifier iterations per surface on both
hosts. Passing O1b would show only bounded local amplification and acceptable
latency under these profiles. Sixteen to eighteen bits remain cheap for
specialized hardware, so this is not a global anti-Sybil or anti-squatting
mechanism.

### O1b measurement — passed

The unchanged harness and hostile corpus used the predeclared `20` solve
samples and `100,000` verifier iterations per surface. The Linux binary
SHA-256 was
`d950980fa3adc05fab9c02b92b46ec3022e754c43e901ebf6fa21f98be26e0cf`;
the host, pinned image, and container restrictions were otherwise identical to
the O1 run.

| Surface | O1b bits | Windows solve p95 | Weaker Linux solve p95 | Linux verify p95 | Heap at cap |
|---|---:|---:|---:|---:|---:|
| exact-name resolution | 16 | 60.66 ms | 72.73 ms | 2.19 us | 395,032 B |
| renewal/update | 16 | 75.43 ms | 103.70 ms | 1.81 us | 198,168 B |
| policy/recovery | 17 | 131.89 ms | 235.51 ms | 4.19 us | 99,784 B |
| root claim | 18 | 511.78 ms | 524.81 ms | 2.84 us | 99,768 B |

All predeclared solve, verifier, and retained-state gates passed. Ten hostile
matrix repetitions passed for scope binding, expiry, replay, boot-secret
restart, per-surface saturation, no-wait busy rejection, exactly one accepted
parallel duplicate, and no spent-state mutation after invalid work. Logical
proof size was 262–267 bytes.

**Inference:** O1b is suitable as the Stage 6 local amplification guard under
the measured profiles. **Honest limitation:** it does not stop a specialized
solver from exhausting a Node's finite per-surface window, distribute trust,
or establish personhood, fairness, or anti-squatting.

## Recommendation

Select O1b for Stage 6 local admission with its explicit limitations.
Confidence is high in the measured local bounds and
moderate in their portability beyond the two profiles. O1b must never be
described as global Sybil resistance.

## Disposition

- State: `decided`; the Product Owner accepted O1b and ADR-0019 on 2026-08-20.
  O1 remains rejected.
- O1b's bits, caps, in-flight limits, 30-second maximum TTL, reason classes, and
  honest limitations are normative Stage 6 inputs.
- S6.5 implementation is authorized through `Admission.Verify`.
