---
id: R-090
title: Does the Stage 5 blocked-entry verifier retain a current reproduction or Qualification duty?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-090 — Stage 5 verifier retirement

## Decision this unlocks

Complete DA-11's M14 disposition for the remaining separately built Stage 5
evidence reader without treating an unrun H3 campaign contract as a current
native Route Qualification obligation.

## Current contract

R-080 intentionally retained `blocked-entry-verify-lab` and `blockedverify`
only until M14 audited the whole Stage 5 record. R-076/ADR-0024 have since
made the H3 Bridge/WebTunnel profile C0 and require any future claim to obtain
fresh native-v1 Qualification evidence. The repository contains no immutable
`h3-s5-b1-v1` campaign bundle or final verifier result: its tracked
`tests/live/stage5-final` files are non-secret preparation inputs whose
`pending-qualifying-stand` identities deliberately make final preparation fail
closed.

## Hypotheses

- **H1:** retain the Stage 5 documents and frozen inputs as C4 provenance, but
  delete the unbound verifier command, Module, and profile entries as C0.
- **H2:** retain the verifier for a future S9.6 campaign.
- **H0:** delete the record and inputs with the executable corpus.

## Evaluation criteria

The result must preserve recorded Stage 5 development provenance, neither
restore H3/WebTunnel runtime nor claim native-v1 Qualification, and retain an
executable laboratory package only when a concrete immutable source/bundle or
accepted current claim names its duty.

## Evidence plan

### Primary sources

- R-076, ADR-0024, R-080, DA-11, and the Stage 8 target architecture,
  inspected 2026-08-23.
- `docs/development/stage-5-blocked-entry-evidence.md` and
  `tests/live/stage5-final/README.md`, inspected 2026-08-23.
- The package map, historical-reproduction profile, and source caller graph,
  inspected 2026-08-23.

### Experiment

The retained reader would require a source-bound immutable campaign bundle and
a current claim or reproduction consumer. Inspect the tracked Stage 5 inputs,
the command's callers, and the accepted Route profile before deletion.

### Failure scenarios

A retained input is misdescribed as a final campaign result; a future native
claim silently uses H3 evidence; the only reader for a stored immutable bundle
is deleted; or the laboratory reader becomes a product dependency.

## Findings

- **Inspection:** `tests/live/stage5-final` contains configuration, supply
  lock, and builder inputs only. Its README expressly says it is not evidence
  and cannot satisfy S9.6 by itself.
- **Inspection:** `stage-5-blocked-entry-evidence.md` records no completed
  qualifying run. It requires a separately created external bundle and keeps
  stand identities pending; it is a historical development contract.
- **Inspection:** the only Go caller of `blockedverify` is its thin command;
  the only retained execution-profile references are that command and Module.
- **Inference:** H2 would preserve a generator-less H3 verifier for a claim
  that current Route authority rejects. It has neither an immutable artifact
  to reproduce nor a valid current Qualification duty.

## Options

| Option | Disposition |
|---|---|
| H1: retain provenance, delete the unbound executable reader | Choose. It preserves what is actually recorded and removes dead H3 execution surface. |
| H2: keep the reader for S9.6 | Reject. R-076 requires fresh v1 evidence; no H3 campaign artifact or accepted claim names this reader. |
| H0: delete records and inputs | Reject. The development and retirement reasoning remain durable provenance. |

## Recommendation

Choose H1 with high confidence. A private external campaign cannot be assumed
as a maintained obligation. If a future claim needs a reproducer, it must add
one with its own source identity, immutable evidence contract, accepted claim,
and M14-style retirement condition.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
Delete `cmd/blocked-entry-verify-lab`, `internal/lab/blockedverify`, and their
package-map and historical-profile entries as C0. Retain the Stage 5 records,
configuration inputs, and R-080 as C4 provenance. R-090 completes the M14
whole-record audit that R-080 expressly deferred; it does not supersede the
R-080 retirement of the Stage 5 generator or alter the native-v1 Route profile.
