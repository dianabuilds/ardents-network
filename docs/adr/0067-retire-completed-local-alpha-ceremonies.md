---
status: accepted
date: 2026-09-01
supersedes: ADR-0050 and ADR-0051 (maintained release-seed interfaces only); ADR-0053 (maintained genesis writer and terminal Adapter only); ADR-0059 (retained initialize/inspect consequence only)
---

# ADR-0067 — Retire completed local alpha ceremonies

## Context

The Product Owner already used the fixed release-seed and functional-alpha
State genesis ceremonies to create the retained RC1/RC2 evidence. ADR-0059
retired the only release assembly consumers, and no future release accepts the
old seed automatically. The fixed State initializer has no open, successor,
rotation, recovery, or non-empty-topology operation; its public fragment was
already consumed by RC1.

Keeping both terminal Adapters, their implementation, and an independent build
and process-test lane preserves the ability to repeat completed planning work,
not a current product or operator lifecycle.

## Decision

Retire `cmd/ardents-release-custody`, `internal/release/custody`,
`cmd/ardents-state-custody`, and the fixed `InitializeAlphaGenesis`
implementation from `internal/network/state`. Remove their local-ceremony
artifact, process-test, package, ownership, reference, and current product
registrations. Release verification, Network State admission, and ordinary
Authority Custody remain unchanged at their existing Modules.

The historical `ardents-release-seed-envelope-v1`,
`ardents-release-seed-record-v1`, `ardents-release-custody-receipt-v1`,
`ardents-state-authority-envelope-v1`,
`ardents-functional-alpha-state-seed-v1`,
`ardents-functional-alpha-state-v1`, and
`ardents-functional-alpha-state-receipt-v1` identities remain immutable
provenance, not maintained compatibility contracts or accepted current inputs.
This decision explicitly retires their maintained reader/writer obligation.
It does not read, migrate, destroy, or claim secure deletion of any external
Product Owner-held record.

A future release authority, first-network bootstrap, State successor, rotation,
or recovery operation requires a new decision from the current product
contract. It cannot silently reuse these keys, formats, or delivery-horizon
identities.

## Consequences

- The maintained command surface contracts from eight binaries to six and the
  local-ceremony execution profile disappears.
- Completed alpha ceremony evidence remains available in R-119 through R-121,
  the superseded ADRs, immutable receipts/releases, and Git history.
- The current Release and State verifiers retain their accepted inputs and
  behavior; only orphan writers and terminal routes leave.
- Inspecting an old release seed is no longer a supported operation. Any future
  forensic or migration need must select an explicit destination and bounded
  reader rather than restore the former writer Module.

## Compliance

- [R-134](../research/records/r-134-retire-completed-local-ceremonies.md)
- [ADR-0059](0059-retire-fixed-alpha-candidate-assembly.md)
- [Current command surface](../development/command-surface.md)
