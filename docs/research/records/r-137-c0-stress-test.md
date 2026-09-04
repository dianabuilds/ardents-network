---
id: R-137
title: Did the pre-C0 Linux stress spike reveal a maintained-candidate defect?
status: deferred
owner: Product Owner and Codex
started: 2026-09-03
reviewed: 2026-09-04
---

# R-137 — Did the pre-C0 Linux stress spike reveal a maintained-candidate defect?

## Decision this unlocks

Decide whether a Linux stress/soak runner is required before C0 stabilization,
and whether its observed Endpoint failure is a product defect, a test defect,
or insufficient evidence.

## Current contract

The C0 candidate has no supported hostile-load, soak, capacity, or VPS-host
claim. `make check` is the maintained local quality gate; the deep-audit method
defines a later exact-candidate audit. A missing selected test prerequisite is
an invalid environment, not a passing skip.

## Hypotheses

- **H1:** The candidate's selected checks can remain green under a bounded
  Linux stress run without data races or resource-growth evidence.
- **H2:** A failure is attributable to a maintained production lifecycle.
- **H0:** The spike cannot distinguish a product defect from its test harness
  or is not reproducible enough to select a C0 gate.

## Evaluation criteria

The runner must use a declared Linux image, finite CPU/memory limits, one
exact candidate digest, external evidence storage, and a result that can be
repeated. A failure must identify the affected maintained behavior and have a
minimal, deterministic reproducer before it changes the product contract.

## Evidence plan

### Primary sources

- [C0 product scope](../../product/scope.md), accessed 2026-09-04.
- [Testing policy](../../development/testing.md), accessed 2026-09-04.
- [Deep audit method](../../development/deep-audit.md), accessed 2026-09-04.

### Experiment

The disposable runner built four headless commands in a Linux container and
planned `go test -race`, `make quick-check`, process evidence, and a repeated
deterministic-profile loop. Generated logs, binaries, and resource samples
were required to live outside the repository.

### Failure scenarios

The spike was intended to detect a race report, test/process failure, bounded
resource breach, panic, failed cleanup, or a non-reproducible result that made
the runner unsuitable as evidence.

## Findings

- **Measurement (recorded 2026-09-03):** the focused
  `TestSlowConsumersApplyBackpressureUntilLocalCancellation` passed 50/50
  under Linux `-race`; one full parallel stress loop observed an intermittent
  `abrupt connection loss` result with zero transferred bytes.
- **Inference:** the failure report showed that an immediate `t.Fatalf` could
  run deferred pipe closes while the second endpoint goroutine was still
  processing cancellation. That makes the observed remote-close class a
  test-lifecycle artefact, not evidence of a production Route or Service
  regression.
- **Measurement (2026-09-04):** the repaired test consumes both outcomes
  before failing and passed `go test ./internal/endpoint -run
  '^TestSlowConsumersApplyBackpressureUntilLocalCancellation$' -count=50
  -shuffle=on` on the current workspace.
- **Limitation:** no clean, exact-candidate Linux stress rerun was completed
  after the repair. This record establishes neither load behavior nor C0
  qualification.

## Options

1. Make the disposable stress runner a C0 requirement. Rejected for now: its
   clean rerun, resource budget, and release-candidate binding are not selected.
2. Treat the observation as a production defect. Rejected: the minimized
   behavior is test-only and the corrected test remains red-capable.
3. Retain the finding and defer the runner. Chosen.

## Recommendation

Keep the outcome-consumption correction and defer the stress runner until a
C0 contract selects a Linux load budget and exact artifact. Confidence is
medium: the focused loop supports the test diagnosis, but not a system-load
claim. The strongest contrary argument is that an unrepeated parallel failure
could conceal a second timing defect.

## Disposition

**Deferred.** The disposable runner and its in-repository evidence directory
are removed so they cannot enter a maintained candidate. This record retains
the useful observation; it does not create a release or audit gate. Reopen only
with a new bounded research question and external evidence root.
