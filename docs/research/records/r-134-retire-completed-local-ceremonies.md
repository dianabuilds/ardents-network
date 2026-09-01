---
id: R-134
title: Retirement of completed local alpha ceremony interfaces
status: completed; retirement accepted
owner: Product Owner and Codex
started: 2026-09-01
reviewed: 2026-09-01
---

# R-134 — Must the completed release-seed and functional-alpha genesis ceremonies remain maintained interfaces?

## Decision this unlocks

Decide whether `ardents-release-custody`, `ardents-state-custody`,
`internal/release/custody`, and the fixed `InitializeAlphaGenesis` State
interface still provide a current product or operational outcome, or whether
their completed ceremonies should remain only as immutable provenance.

## Current contract

ADR-0050 and ADR-0051 selected a separately encrypted fixed-role release seed
and a public receipt inspection operation. ADR-0052 historically allowed that
record to create only the fixed RC1/RC2 inputs; ADR-0059 retired the assembly
operations after those artifacts were completed and states that the retained
record is not accepted automatically for a future release.

ADR-0053 selected one fixed 30-day, empty-topology, 1-of-1 functional-alpha
State genesis. R-121 records that the Product Owner created the encrypted
record and that its public State fragment was consumed by RC1. A non-empty
State, successor, rotation, recovery, or public governance profile remains a
separate decision.

The active team remains one Product Owner and Codex. Maintained code must serve
the current product, an active operation, or a compatibility obligation rather
than preserve the ability to repeat completed planning ceremonies.

## Hypotheses

- **H1:** Both ceremony surfaces can be retired without removing a current
  caller, participant journey, successor capability, or verifier behavior.
- **H2:** The release-seed inspector or fixed genesis initializer remains
  necessary to preserve an active custody or bootstrap lifecycle.
- **H3:** A read-only migration utility must remain for the external encrypted
  records even though no future operation accepts their authority.
- **H0:** Retirement would strand a selected current product operation, so one
  or both surfaces must remain.

## Evaluation criteria

- Every retained command has a current product or operator outcome and at least
  one non-test caller beyond its own terminal adapter.
- Removing a completed ceremony must not move root material into Endpoint,
  Node, Release verification, State acceptance, or ordinary Authority Custody.
- Existing release verification and Network State admission must keep their
  current interfaces and behavior evidence.
- Historical receipts, schemas, research, ADRs, and Git evidence remain
  immutable; retirement must explicitly disposition external encrypted records
  rather than silently claim compatibility.
- Test, artifact, ownership, dependency, package-map, reference, and current
  product owners must describe only maintained surfaces.
- The normal repository gates must pass without a replacement ceremony route.

## Evidence plan

### Primary sources

- ADR-0050, ADR-0051, ADR-0053, and ADR-0059, inspected 2026-09-01.
- R-119, R-120, and R-121 dispositions, inspected 2026-09-01.
- `cmd/ardents-release-custody`, `internal/release/custody`,
  `cmd/ardents-state-custody`, and `internal/network/state/alpha_genesis*`,
  inspected 2026-09-01.
- Current product scope, command-surface inventory, test profiles, package map,
  dependency register, and ownership registry, inspected 2026-09-01.

### Experiment

No disposable experiment is required. Repository-wide caller search, package
dependency inspection, and removal followed by the deterministic, process,
race, architecture, build, static, and vulnerability gates are sufficient to
falsify H1 if a maintained consumer or behavior remains.

### Failure scenarios

- A participant or operator still needs either binary for a current journey.
- Release verification accidentally depended on the retired seed writer or
  inspector.
- Network State acceptance depended on the fixed genesis encoder rather than
  its canonical verifier.
- Removing the ceremony profile hides an orphan command or e2e package from all
  positive inventories.
- An external historical seed is presented as current authority after its
  maintained decoder or writer is removed.
- Retirement deletes, rewrites, or claims secure destruction of Product
  Owner-held external secret material.

## Findings

- **Sourced fact:** R-119 records that the release-seed ceremony created the
  fixed RC1 inputs and that the immutable prerelease was published.
- **Sourced fact:** ADR-0059 retired the only assembly/signing consumers and
  says the retained seed record is not accepted for a future release.
- **Measurement:** `internal/release/custody` has no non-test caller other than
  `cmd/ardents-release-custody`; that command now exposes only `initialize` and
  `inspect`.
- **Sourced fact:** R-121 records that the Product Owner created the fixed
  functional-alpha genesis and RC1 consumed its public State fragment.
- **Measurement:** `InitializeAlphaGenesis` has no non-test caller other than
  `cmd/ardents-state-custody`. The implementation seals a seed but exposes no
  maintained open, successor-signing, rotation, recovery, or non-empty-State
  operation.
- **Measurement:** the two surfaces account for 1,481 production lines and 707
  test lines before documentation, build, profile, and ownership maintenance.
- **Inference:** retaining an initializer does not preserve successor
  continuity. It can only create a new fixed genesis and Network identifier;
  the successor decision it was intended to leave open has no current
  interface.
- **Inference:** authenticated historical receipts and Git evidence preserve
  what occurred. Maintaining executable writers for inputs that no current
  owner accepts adds no product leverage or operational locality.

## Options

### 1. Retire both completed ceremony surfaces

Remove both commands, their command-only Modules or implementation slices, the
local-ceremony artifact/e2e profile, and current documentation claims. Retain
historical research, ADRs, receipts, release identifiers, and Git history.
Explicitly classify the external encrypted records and public ceremony formats
as unsupported historical evidence; do not delete or migrate Product
Owner-held files in this repository change.

### 2. Keep both indefinitely

Preserve initialization and inspection because the encrypted records exist.
This maintains exact decoders and build evidence but supplies no current
release signer, State successor, participant journey, or accepted future
authority.

### 3. Keep read-only migration commands

Remove initialization but retain decoders that print public receipts. No
selected migration destination consumes either receipt, so this would preserve
a hypothetical seam and defer the same decision.

### 4. Keep only State genesis until a successor is selected

This appears conservative but the retained interface cannot use the saved seed
or sign that successor. It preserves a second genesis generator rather than
the missing lifecycle.

## Recommendation

Choose option 1. Confidence is high: both ceremonies are complete, neither has
a current consumer, and removing them leaves the actual Release verifier and
Network State verifier at their existing owners. A future release authority or
Network State bootstrap/successor must start from its current product question
and cannot inherit these completed alpha records automatically.

The strongest argument against retirement is loss of the only maintained
release-seed inspector. That operation can authenticate an external record and
reproduce its public receipt, but no current operation accepts the record or
receipt. If the Product Owner later needs forensic inspection, Git history
preserves the exact source; that is not a supported current custody lifecycle.

## Disposition

On 2026-09-01 the Product Owner explicitly selected option 1 and requested the
ADR, implementation retirement, separate branch, verification, and integration
into `main`. ADR-0067 records the decision. No external encrypted record is
read, changed, migrated, or deleted by this work. Historical records remain
provenance; maintained compatibility for their private envelope, seed, public
fragment, and command-receipt formats ends explicitly with ADR-0067.
