---
id: R-091
title: Which Carrier Lab and Gate C execution paths remain after their source-bound results close?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-091 — Closed laboratory retirement

## Decision this unlocks

Complete DA-11's M14 disposition for the Carrier Lab and Named Unlisted Site
execution corpus, including their shared laboratory Modules, commands, manual
workflows, supply scripts, and active reproduction profile.

## Current contract

R-013 records Carrier Lab run `31404126248` at commit
`54eee1232461106af15da3a1665a9f4f8166675a`; R-017 records Gate C run
`31464163490`. Their source, image, manifest, and result hashes remain in the
records. R-075/R-076 reject promotion of the evaluated H3 C-5/C2, Tor,
WebTunnel, and lab framing into the maintained Route, and require fresh native
v1 evidence for a future claim. The two manual GitHub workflows rebuild the
current tree and retain uploaded artifacts for only 30 days, so they cannot
reproduce either accepted result.

## Hypotheses

- **H1:** retain R-013/R-017 and their source-bound receipts as C4 provenance;
  delete the closed lab execution corpus as C0.
- **H2:** retain the commands and workflows as historical reproducers.
- **H0:** delete the records together with their execution corpus.

## Findings

- The only code callers of the shared `internal/lab/*` Modules are the two
  lab commands, their tests, and their manual workflows.
- Both workflows build current source, not the recorded commits, and their
  uploaded evidence has `retention-days: 30`; a new run would be a new
  experiment rather than access to the accepted artifact.
- R-013 says the completed comparison is not rerun or expanded by default;
  R-017 says Gate C closes the controlled tracer, not a current runtime or
  Qualification claim.
- The current native Route profile expressly rejects treating the H3 lab bytes
  as a foundation. No accepted current claim, product caller, or immutable
  repository bundle names any retained executable duty.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
Delete `cmd/carrier-lab`, `cmd/named-site-lab`, all remaining
`internal/lab/*` Modules, their commands' architecture tests, `lab/` sources,
manual Carrier/Gate C workflows, and their dedicated supply scripts and active
historical-reproduction profile as C0. Retain R-013, R-017, R-025, R-075,
R-076, and the historical documentation as C4 provenance. A future claim must
introduce a newly accepted, source-bound evidence suite; it cannot reactivate
these H3 runners.
