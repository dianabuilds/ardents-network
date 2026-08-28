---
id: R-122
title: A11 harness and RC2 candidate qualification continuity
status: decided
owner: Product Owner and Codex
started: 2026-08-28
reviewed: 2026-08-28
---

# R-122 — Can the corrected A11 harness qualify immutable RC2 without a new candidate?

## Decision this unlocks

One official A11 campaign using immutable `h4-alpha-1-rc-2` release inputs and
a separately committed corrected qualification harness. This record does not
accept the campaign result; it only fixes the evidence identity rule that the
runner enforces.

## Current contract

RC2 is immutably tagged at `2c18bdf92f11f84075915576f595202f48eb05bc`.
The normal-path A11 failure was in the qualification fixture, not in the
Endpoint candidate: Publisher Application applied its five-second per-cycle
limit before the local Browser/Proxy origin was ready. Commit
`9ef9ebc4946915b161e5df792d885a376ac10633` gives the first request a bounded
15-second assembly window while preserving the five-second limit for every
observed cycle. Its 10-cycle Windows-to-Ubuntu canary completed 10/10 cycles,
used one proxy dial with no redial, and retained complete User and remote
evidence.

`tests/qualification/h4-8-a11/run-windows.ps1` independently requires that
the candidate worktree HEAD, the immutable release tag, and
`H4_8_A11_SOURCE_REVISION` name RC2. It independently requires a clean,
committed harness and records its revision as `harness_revision`. It does not
compare the harness revision to the candidate revision. Each attempt records
both identities. Therefore the corrected harness does not alter RC2 release
bytes or break the runner's candidate binding.

## Hypotheses

- **H1:** the runner requires candidate and harness revisions to be equal, so
  a new immutable successor candidate is required.
- **H2:** the runner binds immutable candidate inputs and a separately
  versioned clean harness, retaining both identities in evidence.

## Evaluation criteria

- The release tag resolves exactly to the declared RC2 source revision.
- The selected RC2 worktree is clean and at that source revision.
- The harness worktree is clean and committed, and its revision is retained
  with the campaign inputs and every attempt.
- The retained evidence identifies both revisions, the fixed archive and
  program digests, topology, duration, resource limits, measurements, and all
  six A11 cells.
- No existing tag, artifact, metadata, or evidence is overwritten.

## Evidence plan

### Primary sources

- `tests/qualification/h4-8-a11/run-windows.ps1`, inspected 2026-08-28:
  its input validation, input receipt, and per-attempt validation record the
  candidate and harness identities independently.
- Retained short-canary evidence at
  `C:\Users\vitek\AppData\Local\Temp\ardents-a11-after-initial-grace`,
  observed 2026-08-28. It is diagnostic evidence only.

### Experiment

Preflight the clean RC2 candidate/tag/archive/control inputs and the clean
committed harness. Run the official 6/6 A11 campaign once. Preserve every
attempt, including a failed one, under a new external evidence root.

### Failure scenarios

- Candidate/tag/source, archive, or program identities disagree.
- The selected candidate or harness worktree is dirty or has no committed
  revision.
- A harness revision is missing from the receipt or an attempt record.
- A failure is retried, erased, or represented as a pass.
- A changed release byte is attributed to RC2.

## Findings

- **Current-contract fact:** the runner's candidate validation compares
  candidate HEAD and release-tag commit to `SourceRevision`; its harness
  validation only requires a clean committed harness. The emitted input
  receipt includes both `source_revision` and `harness_revision`.
- **Current-contract fact:** per-attempt validation applies the same separate
  checks and records `working_tree_revision` (harness) alongside
  `candidate_source_revision`.
- **Measurement:** the corrected short canary completed 10/10 cycles in
  10.13 seconds and retained clean terminal evidence. It is not an A11 pass.
- **Inference:** H2 is confirmed. H1 came from an incorrect reading of the
  runner and is rejected. A successor candidate would add custody and repeat
  gates without changing the RC2 release bytes.

## Options

1. **Use RC2 with the separately versioned committed harness.** Accepted. It
   is the runner's declared identity model and preserves both audit anchors.
2. **Create RC3 solely to match the harness revision.** Reject. The candidate
   bytes and release identity have not changed; this is unnecessary custody
   work and would broaden the qualification surface.
3. **Retag or overwrite RC2.** Reject. It violates immutability.
4. **Do not run A11.** Keep H4-3/H4-8 open. This remains the honest outcome
   if the preflight or campaign fails.

## Recommendation

Run the official A11 campaign once with the immutable RC2 candidate and the
corrected committed harness. The final receipt must name both revisions. No
RC3, custody operation, or dependent candidate requalification is required
unless a release input byte changes.

## Disposition

Concluded on 2026-08-29. The planned RC3 successor remains withdrawn. Official
RC2 A11 attempt 14 accepted all six cells and ten invocations in 2,462,217 ms:
`C:\Users\vitek\Ardents-Release\evidence\h4-alpha-1-rc-2-h4-8-a11-attempt-14\campaign-receipt.json`.
The receipt binds RC2 source/tag/archive and separately records harness revision
`a7147b04`; no release bytes changed and no dependent candidate requalification
was required. A12 may therefore close the selected H4-3/H4-8 functional-alpha
profile while retaining all broader limitations.
