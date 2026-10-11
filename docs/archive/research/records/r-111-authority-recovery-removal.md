---
id: R-111
title: Participant Authority recovery and removal
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-111 — Participant Authority recovery and removal

## Decision this unlocks

Select the H4-1C participant custody journey and the exact point at which an
encrypted Recovery Bundle may become an active Authority again or be removed.

## Current contract

`internal/custody` already encrypts records, exports a distinctly passworded
Bundle with isolated test restore, imports it only to an `authority-locked`
quarantine record, retains Authority floors, and activates a Name record only
from a fresh strictly newer Namespace witness. H4-1C must present those facts
without placing passwords in argv/environment or converting Bundle possession
into signing power.

## Hypotheses

- **H1:** A separate terminal-only custody command can safely offer Bundle
  export, locked restore, and explicitly confirmed purge using public
  commitment binding, while leaving activation unavailable until its independent
  Namespace proof is present.
- **H2:** One generic repair command can infer activation or deletion from a
  local Bundle and current record.
- **H0:** Neither command shape is safe enough for an alpha participant.

## Evaluation criteria

- Passwords and root material never appear in command arguments, environment,
  output, or shared Endpoint IPC.
- Export uses a password distinct from the Vault password and isolatedly tests
  the published Bundle.
- Restore accepts only an empty Vault and leaves the result export-only and
  `authority-locked`.
- Activation requires the existing fresh Namespace witness; purge needs a
  separately explicit confirmation and preserves authority watermarks.
- Program replacement, uninstall, and cache cleanup cannot invoke a custody
  operation.

## Evidence plan

Run the real terminal adapter with a fixture SecretInput: create an encrypted
record, export it, restore into a distinct root, verify that output is public
only, reject an unconfirmed purge, complete a confirmed purge while retaining
the floor, and attempt prohibited activation. A future participant-terminal
walkthrough with a real Namespace witness belongs to the deferred activation
route, not to this bounded custody decision.

## Findings

- **Source fact:** `cmd/ardents-custody` previously exposed only inspection and
  verification despite maintained export/restore custody transitions.
- **Implementation evidence:** its H4-1C first slice now exposes explicit
  `export-recovery-bundle` and `restore-recovery-bundle` commands. They require
  the full public Authority binding and terminal SecretInput; a behavior test
  proves export output contains no root/password and restore yields
  `authority-locked`. `purge-record` separately verifies the encrypted record
  and requires explicit terminal confirmation before deleting it; a behavior
  test proves refusal retains the record and successful purge retains its
  Authority floor.
- **Inference:** exporting and locked restoration are meaningful user actions
  without selecting unauthenticated repair or destructive deletion semantics.

## Options

1. Separate custody command for export/locked restore; defer activation and
   purge until their respective proofs exist.
2. Add recovery to the Endpoint or installer lifecycle.
3. Expose a generic repair/purge command now.

## Recommendation and disposition

Choose option 1 for the H4-1C alpha slice: explicit export, locked restore, and
confirmed purge. It preserves custody's existing authority boundary and is
usable without claiming recovery activation. Reject options 2 and 3: they would
couple Authority to program lifecycle or make a local inference into a
destructive decision. Namespace-witness activation and any fuller repair
workflow remain separate future work; the implemented purge already retains
the Authority watermark.
