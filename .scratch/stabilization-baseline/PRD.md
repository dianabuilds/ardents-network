# Stabilization baseline

Status: in-progress

## Outcome

Create one exact, reproducible research baseline for Ardents so that subsequent
feature research, implementation work, and release qualification all refer to
the same source state and evidence contract.

## Current state

The repository is a stabilization candidate, not a production release.
Preparation has established:

- a current capability/evidence register;
- a current remediation ledger separated from the retained historical audit;
- an explicit LF checkout policy for Go source;
- a tagged integration/E2E scenario catalogue that rejects an empty result;
- a local Markdown issue tracker;
- a reduced historical audit containing only coverage and findings evidence.

These changes are locally verified but are not yet bound to one clean commit.
Docker, native Linux, multi-host, security, and independent release evidence
remain part of the later R3 qualification program.

## Scope

- Freeze the preparation work as one reviewable baseline.
- Prove the Go LF contract in a fresh Windows checkout.
- Prove the tagged scenario catalogue from the baseline source.
- Retain a clean, commit-bound R0 evidence snapshot and reconcile the current
  ledger without overstating release readiness.

## Out of scope

- Docker integration and E2E execution.
- Native systemd installation qualification.
- Multi-host and adversarial network qualification.
- Vulnerability evidence and independent release builds.
- New Application, messaging, hosting, or deployment features.
- Promotion of any capability to production-ready based only on R0 evidence.

## Issue order

```text
R0-001 Freeze preparation baseline
   |
   +--> R0-002 Prove Windows LF checkout contract --+
   |                                                |
   +--> R0-003 Prove tagged scenario catalogue -----+--> R0-004 Retain snapshot
```

R0-002 and R0-003 may run independently after R0-001.

## Completion

This work program is complete when all four issues are accepted, their evidence
identifies one exact source commit, and active documentation still describes
the project as a stabilization candidate pending R3 qualification.
