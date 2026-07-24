# Current remediation ledger

## Purpose

This ledger reconciles the historical audit at
`main@52af3b2480b62da60ae82c7f1d43f45cd5778230` with the implementation at
the frozen stabilization baseline
`main@75471a6c08bf0c8a130db65d64c7f37dc33f03b5`.

The retained historical sources
`docs/audit/2026-07-23/03-audit-coverage.md` and
`docs/audit/2026-07-23/04-findings-register.md` remain immutable evidence of
what was checked and found at the audit point. This document owns the current
reconciliation and must not retroactively change the historical finding text.

## Status vocabulary

- `remediated_candidate`: a targeted implementation and deterministic evidence
  exist in the repository.
- `locally_verified`: the applicable non-environmental evidence passes on the
  current research worktree.
- `qualified`: every required clean Linux/Docker/native/deployment gate passes
  for one exact commit and retained evidence identifies that commit.
- `reopened`: current evidence reproduces the finding or contradicts its
  remediation contract.

No row is `qualified` until the complete required gate matrix runs against one
clean commit. A passing unit test alone is not release qualification.

## Reconciliation summary

| Measure | Count |
|---|---:|
| Historical findings | 24 |
| Historical P1 findings | 16 |
| Historical P2 findings | 8 |
| Findings in machine traceability | 21 |
| Additional architecture/documentation/duplication findings | 3 |
| Findings with a targeted remediation commit | 24 |
| Qualified on one current clean commit | 0 pending Wave 0 gate snapshot |

The canonical machine mapping for the first 21 rows is
`tests/ci/audit-test-traceability.json`. This ledger adds commit reconciliation
and the three non-critical-lifecycle findings `CLI-001`, `ARCH-001`, and
`DOC-001`.

## Finding ledger

| Finding | Priority | Remediation commit(s) | Deterministic evidence owner | Current status |
|---|---|---|---|---|
| CI-001 / ARD-001 | P1 | `ffee5b5` + Wave 1 tagged-catalog fix | tagged testcatalog, non-empty guard and static workflow contract | locally_verified |
| CI-002 / ARD-002 | P1 | `ffee5b5` | native-install evidence/smoke contract | remediated_candidate |
| OPS-001 / ARD-003 | P1 | `975c493` | deployment upgrade-backup gate | remediated_candidate |
| SUP-002 / ARD-004 | P2 | `db3017c` | release source identity negative matrix | locally_verified |
| SEC-003 / ARD-005 | P1 | `d58c07a` | Waku Store retention/quota restart tests | locally_verified |
| REL-001 / ARD-006 | P1 | `c542b1e` | ingress reset/listener lifecycle test | locally_verified |
| REL-002 / ARD-007 | P1 | `c542b1e` | ingress source/port fairness tests | locally_verified |
| SEC-002 / ARD-008 | P1 | `1f9961b`, `57b5192` | private envelope authorization-before-replay tests | locally_verified |
| SEC-001 / ARD-009 | P1 | `5954246`, `9febbcf` | owner-qualified identity and schema migration tests | locally_verified |
| IAM-001 / ARD-010 | P1 | `bf4c5f2` | recovery Credential state matrix | locally_verified |
| IAM-002 / ARD-011 | P1 | `9daa427` | Bootstrap/Application ticket failure and retry tests | locally_verified |
| REL-003 / ARD-012 | P1 | `764061d` | active-stream process shutdown tests | locally_verified |
| REL-004 / ARD-013 | P1 | `764061d` | hung Docker and bounded runtime tests | locally_verified |
| OPS-002 / ARD-014 | P1 | `afcaeea`, `81ad5a6` | rollout mutation/compensation/crash matrix | remediated_candidate |
| OPS-003 / ARD-015 | P1 | `b16202c`, `6cfa3da` | composite readiness unit/deployment matrix | remediated_candidate |
| OPS-004 / ARD-016 | P2 | `0c67434`, `bc24644`, `e58bc22` | native configured-target/decoy smoke | remediated_candidate |
| SEC-004 / ARD-017 | P1 | `6d1689b` | multi-provider honest-success and exhaustion tests | locally_verified |
| SEC-005 / ARD-018 | P2 | `bfc066b` | physical replay-store identity matrix | locally_verified |
| SUP-001 / ARD-019 | P1 | `ea8430a` | release materials policy gate | locally_verified |
| QLT-001 / ARD-020 | P2 | remediation series + `1be0842` | critical-file audit traceability gate | locally_verified |
| TST-001 / ARD-021 | P2 | `57debcb` | background-writer drain lifecycle test | locally_verified |
| CLI-001 / ARD-022 | P2 | `91ef834` | shared sessionclient and CLI/SDK parity tests | locally_verified |
| ARCH-001 / ARD-023 | P2 | `defd677`, `ffd9a23`, `deeb4bb`, `7c0965c` | machine-readable architecture acceptance | locally_verified |
| DOC-001 / ARD-024 | P2 | `f1a3033`, `f53b325` | active documentation contract tests | locally_verified |

## Required evidence by gate

| Gate | Findings | Wave 0 disposition |
|---|---|---|
| Static | CI-001, SUP-002, SUP-001, QLT-001, ARCH-001, DOC-001, CLI-001 | run locally where independent of clean checkout |
| Critical lifecycle with `-race` | SEC-001/002/003/004/005, REL-001/002/003/004, IAM-001/002, OPS-003, TST-001 | run on supported Linux runner; local Windows result is diagnostic only |
| Deployment | OPS-001/002/003 | requires Docker/Linux and retained deployment evidence |
| Native install | CI-002, OPS-004 | requires native-systemd acceptance container/runner |
| Integration/E2E | cross-domain regression and user journeys | requires canonical Docker/Linux runner |
| Release | SUP-001/002 plus all dependencies | requires clean exact commit and independent builds |

## Wave 0 observations

### W0-001 — Windows formatting gate is deterministic in a fresh checkout

The repository has no historical `.gitattributes` contract while the current
Windows checkout uses `core.autocrlf=true`. Git therefore considers the worktree
clean, but `gofmt -l` reports CRLF Go files as unformatted. Running `gofmt -w`
over the tree would create a large mechanical worktree rewrite and interfere
with concurrent work.

The remediation adds `*.go text eol=lf` to `.gitattributes` so new checkouts
have a deterministic Go source representation. R0-002 validated the policy in
a disposable Windows checkout of
`75471a6c08bf0c8a130db65d64c7f37dc33f03b5` with `core.autocrlf=true`.
The checkout contained zero CRLF Go files, passed the canonical formatting
entrypoint, and remained clean. As a negative control, parent commit
`7c0965c4b4aeaccd1aefe8c1c0c267159eb01e87` materialized two CRLF Go files
and failed the gate. The supported Linux static job remains part of R3
qualification.

### W1-001 — Static scenario catalog omitted tagged suites

The static workflow invoked `testcatalog` without `integration,e2e` Go build
tags. Its command exited successfully but produced `[]`, so it did not validate
the tagged suite metadata that it claimed to cover.

The remediation candidate passes both build tags, rejects an empty result, and
extends the entrypoint negative matrix so either omission fails the static job.
R0-003 generated 142 valid entries from the frozen baseline and the negative
matrix passed without retry.

## Promotion rule

A row may be promoted from `remediated_candidate` to `locally_verified` only
with a recorded command and outcome from the current worktree. It may be
promoted to `qualified` only when:

1. the worktree is clean and identifies one exact commit;
2. all gates named in `tests/README.md` for the finding pass;
3. required JSON/JUnit/security/deployment evidence is retained;
4. no retry hides an earlier failure;
5. the evidence commit contains the remediation and its regression tests.

Until then the historical finding is not open by default, but its closure is
also not a release claim.
