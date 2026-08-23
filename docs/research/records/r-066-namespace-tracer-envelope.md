---
id: R-066
title: What bounded Namespace resource envelope may Stage 8 retain without claiming product scale?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-066 — Namespace tracer envelope

## Decision this unlocks

DA-04 requires a declared Namespace cardinality and resource envelope before M5
turns the current arbitrary 4,096-record implementation cap into a target
architecture fact. This question decides only the narrow technical-tracer
envelope that Stage 8 may preserve; it does not claim a product Namespace
capacity, registrar availability, or independent operator performance.

## Current contract

R-041 permits a legal Service Name depth of 127 labels. R-057/ADR-0020 retain
one threshold-authenticated current Namespace statement and a compact proof in
the fixed 4,096-byte private-resolution envelope; its retained development
fixture has 127 current Records and a 1,667-byte maximum-depth proof. R-043
requires atomic durable publication, tamper failure, and restart derivation.
R-060 makes Namespace persistence/commitments domain-owned. The unmeasured
`maximumRecords = 4096` implementation constant has no authority.

## Hypotheses

- **H1:** one 127-record, one-writer technical-tracer corpus can be retained
  with an explicit local lookup/reopen envelope and no product-scale claim.
- **H2:** the existing 4,096-record cap can be treated as a supported scale
  because it is finite.
- **H0:** even the retained 127-record corpus cannot meet the fixed proof,
  restart, or bounded local-resource conditions and M5 must stop at a smaller
  conformance-only slice.

## Evaluation criteria

The experiment must predeclare and measure: 127 signed Records, one serial
materialization update, eight concurrent local exact-name lookups, compact
proof size, restart-open plus lookup latency, and process heap allocation. It
must reject H1 if a proof exceeds 4,096 bytes, any lookup/reopen fails, a
concurrent lookup returns another Name, or the test produces a durable root
outside its temporary directory. The result is a local development envelope,
not a throughput, availability, anonymity, Sybil-resistance, or production
capacity claim.

## Evidence plan

### Primary sources

- R-041, R-043, R-057, R-060, and ADR-0020, accessed 2026-08-23.
- `internal/namestore/{persistence,materialization,proof,store}.go` and the
  Stage 8 G2 F032 review, inspected 2026-08-23.

### Experiment

Run `go run experiments/r-066-namespace-envelope/main.go` on the current
development host. It creates a temporary Namespace root, deterministically signs the 127
record hierarchy, commits it once, measures exact-name lookup and reopen-plus-
lookup samples, runs eight simultaneous independent lookups, and removes the
temporary root on exit. Its JSON output is the measured artifact.

### Failure scenarios

- proof expansion beyond the fixed response envelope;
- a lookup or restart returns stale, wrong, or absent current state;
- concurrent reads corrupt, mix, or expose another Record;
- a materialization update or reopen requires a second writer; and
- retained heap grows outside the declared bounded corpus.

## Findings

- **Measurement:** on Go `go1.26.6` / Windows `amd64`, the deterministic
  127-record corpus produced a 1,667-byte proof. One hundred local exact-name
  lookups measured `16,649 us` p50 and `17,857 us` p95; one hundred independent
  reopen-plus-lookup samples measured `24,244 us` p50 and `26,367 us` p95.
  Eight simultaneous lookups all returned a non-empty proof for the requested
  Name. The post-run process heap observation was `1,180,944` bytes.
- **Verification (2026-08-23):** after the implementation cap was reduced to
  the accepted 127 records, the same disposable scenario completed with 127
  records, an 1,675-byte proof, eight successful concurrent lookups, and no
  retained root. Its p50/p95 lookup sample was `17,100/19,826 us`; reopen plus
  lookup was `24,594/27,548 us`; heap observation was `2,793,192` bytes.
  These host/load-sensitive samples remain local tracer evidence, not a
  performance or capacity claim.
- **Measurement:** `go run -race ./experiments/r-066-namespace-envelope` is an
  invalid local environment: the Windows linker inherits the repository's
  native dependency closure and cannot resolve `-ldl`. This is recorded as an
  unavailable detector, not a passing race result; the accepted envelope does
  not claim race qualification.
- **Sourced fact:** R-057's independently recomputed 127-record fixture has
  the same 1,667-byte proof result, so this run reproduces the retained proof
  measurement rather than establishing a new global capacity.
- **Inference:** H1 is sufficient for a local, one-writer technical tracer.
  The values do not support H2: 4,096 remains an unmeasured implementation
  limit and whole-corpus lookup remains unsuitable as a scale selection.

## Options

| Option | Fit | Disposition |
|---|---|---|
| H1: retain the measured 127-record tracer envelope | Preserves the accepted legal-depth and proof characterization without inventing a larger scale claim. | Accepted. |
| H2: retain 4,096 records by implementation constant | Has no cardinality, latency, memory, concurrency, proof, or restart evidence. | Rejected unless new evidence reverses this result. |
| H0: reduce further | Required if the retained conformance corpus fails the predeclared checks. | Fallback. |

## Recommendation

Accept H1: M5 may retain exactly a 127-record, one-writer, local technical
tracer envelope with bounded 4,096-byte proof, serial materialization update,
and eight local concurrent exact-name readers. Its target must reject larger
corpora rather than carry forward the unaccepted 4,096 constant. No index,
cache, distributed concurrency, availability, or product capacity is selected.
Any widening needs a new question with representative platform, adversarial,
and restart evidence.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** DA-04 is closed only for this narrow tracer envelope. The
experiment remains disposable research evidence; it adds no runtime package,
dependency, or persistent data format.
