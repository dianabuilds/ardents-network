---
id: R-054
title: Which canonical evidence profile independently proves Horizon 3 Stage 7?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-054 — Stage 7 evidence and verifier profile

## Decision this unlocks

Freeze the canonical manifest, private-fixture, evidence, cleanup, and verdict
serialization; campaign identity; exact complete A–H schedule; episode counts; clocks;
resources; observers; mutation corpus; and independent `pass|fail|invalid`
predicates before any S7.1 result can be cited.

## Current contract

R-048, the [Stage 7 evidence contract](../../development/stage-7-platform-evidence.md),
R-023, and the H3 technical design require disjoint artifact authorities,
immutable precommitment, native-resolution security/transition observations,
controlled platform observers, complete cleanup, deterministic independent
verdicts, and no secret-bearing repository/evidence. H3 development results are
not Route Qualification or H4 independent-supply evidence.

## Hypotheses

- **H1:** one bounded canonical profile with platform-specific observation
  records and shared behavior predicates can verify both hosts without importing
  candidate decision logic.
- **H2:** separate Ubuntu/Windows evidence schemas are necessary, joined only by
  a higher-level report.
- **H0:** required platform facts cannot be observed reliably enough to judge
  principal/isolation/update behavior; affected candidates remain `invalid`.

## Evaluation criteria

- deterministic canonical bytes with explicit schema/profile/campaign versions;
- manifest frozen before candidate execution and private canary release;
- content-addressed immutable paths and no manifest/evidence/verdict overlap;
- exact host/source/supply/package/root/metadata/artifact/config/tool/observer/
  verifier identity and project-control declaration;
- monotonic per-host timings, wall-clock correlation only, no result-selected
  replacement or seed reroll;
- complete raw transition, process-tree, filesystem/registry/package/service,
  ACL/mode, handle/FD, listener/packet/DNS, resource, and cleanup observations;
- a deterministic expected runtime class and predicate set per cell;
- mutation vectors for schema, ordering, identity, hash, path, cross-cell,
  authority overlap, secret leakage, candidate verdict, and cleanup;
- finite bytes/files/events/attempts/resources/runtime/retention; and
- one-to-one reproducibility without pretending the verifier is an independent
  security reviewer.

## Evidence plan

### Primary sources

Repository sources accessed 2026-08-20:

- Stage 7 lifecycle and evidence specifications;
- H3 technical design evidence/decision model;
- R-023 immutable Qualification Evidence Bundle and measurement semantics;
- R-028 resource/evidence split;
- R-031 Application Interface evidence limitations;
- R-037 Stage 5 evidence-integrity campaign design; and
- Stage 6 private naming evidence contract for the same `pass|fail|invalid`
  authority split.

External platform observer documentation is selected with R-050–R-052. It is
part of the frozen source set and cannot be substituted after results.

### Experiment

Create `experiments/r-054-stage-7-evidence-profile/`. Define the schema and
calculator independently of candidate command code. Generate one synthetic
reference set and one corpus mutation per required failure class. Prove byte-
stable round trip, strict unknown/missing/duplicate handling, ordering and clock
bounds, path confinement, secret detection, candidate-verdict rejection,
cross-platform observation mapping, cleanup semantics, and deterministic
recomputation on Ubuntu and Windows.

Run a small observer-control fixture outside the candidate to prove each packet,
DNS, listener, process, file/object, and resource collector detects a known
positive and distinguishes absence from failure to observe.

### Failure scenarios

Candidate writes expected result; runner omits failures; observer starts late;
wall-clock jump changes deadline; platform schema maps missing fact to false;
mutated image/tool after manifest; symlink/reparse path escape; partial cleanup
called pass; private canary/Authority leaked; duplicate event selects favorable
order; a report averages required cells; invalid run silently replaced.

## Falsification criteria

R-054 has two separated phases. Schema prototyping may use synthetic artifacts
only. The canonical schema, campaign identity, cell/episode counts, clocks,
resource limits, observer set, and predicates MUST then be decided and committed
before any candidate Stage 7 execution; prototype results cannot count as a
Stage 7 pass.

O1/O2 is falsified if either host differs across `100` byte-stable canonical
round trips; any precommitted mutation is accepted once; a candidate-authored
verdict affects recomputation; a missing native fact maps to success; any path
escapes an owned root; or any observer misses one of `10` positive controls or
reports a false positive in `10` paired negative controls. The synthetic maximum
bundle is `1 GiB`; verification must stream within `256 MiB` peak RSS and `60 s`
monotonic time on the frozen weakest verifier host, rejecting excess before
unbounded allocation. Any unavailable required observation yields `invalid`.
No threshold, retry, episode, or cell may be revised after candidate results;
if the frozen profile cannot judge both hosts, select O0.

## Findings

- **Inference:** shared behavioral predicates with typed platform observation
  variants preserve comparable outcomes better than two independent schemas.
- **Inference:** observer positive controls are mandatory; “no packet seen” is
  not evidence when the collector itself was not proven active.
- **Assumption:** existing repository canonical evidence patterns can be reused
  conceptually without importing a Stage 5/6 schema or candidate verdict code.

## Options

- **O1:** one canonical profile with shared manifest/verdict and explicit Ubuntu/
  Windows observation variants.
- **O2:** separate platform schemas with a manifest-bound cross-platform join
  verifier.
- **O0:** no valid verdict for facts that cannot be independently observed.

## Recommendation

Prototype O1 first. Select O2 only if a platform fact cannot be represented
without weakening its native meaning. Every shared predicate remains conjunctive;
no averaging or “not applicable” may hide a required platform failure.
Confidence: medium.

## Disposition

- State: `open`; no schema, profile/campaign identity, episode count, clock,
  observer, resource threshold, or verifier package selected.
- Required before S7.1 evidence and before Product Owner `start S7.1`.
- Generated bundles, secrets, captures, databases, logs, and evidence remain
  outside Git.
