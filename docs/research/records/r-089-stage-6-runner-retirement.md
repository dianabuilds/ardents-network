---
id: R-089
title: Which Stage 6 reproduction obligation remains after the S6E1 runner no longer reproduces?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-089 — Stage 6 runner retirement

## Decision this unlocks

Complete DA-11's disposition for the S6E1 evidence generator, verifier, and
their two lab commands without promoting a historical campaign into a runtime
or Qualification obligation.

## Current contract

R-055 records the Stage 6 development-evidence format and its historical
completion boundary. Stage 8 retains product Modules under their current
owners; its target architecture permits a laboratory runner only for a named
reproduction or Qualification duty. The repository has no captured immutable
S6E1 campaign artifact. The only callers of `stage6evidence` and `stage6verify`
are their commands, each other's tests, and the historical-reproduction
profile. The command campaign currently fails before a verdict with
`publication not acknowledged`.

## Hypotheses

- **H1:** preserve R-055 and Stage 6 documents as C4 provenance, but delete
  the non-reproducing runner, verifier, commands, and profile entries as C0.
- **H2:** repair and retain the S6E1 executable corpus indefinitely.
- **H0:** delete both records and code.

## Evaluation criteria

The decision must not discard an immutable accepted evidence artifact, create
a current Qualification claim, retain a product dependency on laboratory code,
or represent a failed current runner as a reproducible historical result.

## Evidence plan

### Primary sources

- R-055 and Stage 6 development/evidence documents, accessed 2026-08-23.
- M14 target architecture and DA-11 register, accessed 2026-08-23.
- Source and profile caller inventory, inspected 2026-08-23.

### Experiment

Run the command corpus and inspect its callers. A retained reproduction duty
would need a recorded immutable source/campaign identity and a successful
independent verifier run; neither exists in the current tree.

### Failure scenarios

A deleted runner was the sole reader of an immutable artifact; a historical
result becomes a current Qualification claim; a repaired runner silently
depends on current product behavior; or an unrelated lab family is removed.

## Findings

- **Inspection:** no product, e2e, command, or external observer imports the
  Stage 6 lab packages; all code callers are self-referential.
- **Measurement:** `go test ./...` reaches `publication not acknowledged` in
  both Stage 6 command suites before publishing a reproducible verdict.
- **Inspection:** no S6E1 campaign bundle is retained under version control;
  R-055 and its development documents are the remaining provenance.
- **Inference:** repair would create a new current campaign without a claim or
  observer, while keeping a failing runner is neither C4 evidence nor product
  behavior.

## Options

| Option | Disposition |
|---|---|
| H1: retain documents, delete S6E1 executable corpus | Choose. It keeps the accepted historical record without a stale runner. |
| H2: repair and retain runner/verifier | Reject. No current reproduction or Qualification obligation names it. |
| H0: delete documents too | Reject. R-055 remains durable decision provenance. |

## Recommendation

Choose H1 with high confidence. The strongest counterargument is a private
campaign outside the repository, but the Stage 8 observer audit cannot turn an
unknown artifact into a maintained support obligation. A future claim may add
a new source-bound reproduction package under a new decision.

## Disposition

Accepted under the Product Owner's standing Stage 8 delegation. Delete only
`cmd/stage6-evidence-lab`, `cmd/stage6-verify-lab`,
`internal/lab/stage6evidence`, `internal/lab/stage6verify`, and their profile
and package-map entries. Retain R-055 and Stage 6 documents as C4 provenance.
