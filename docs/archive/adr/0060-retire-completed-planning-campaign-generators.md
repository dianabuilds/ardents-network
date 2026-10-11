---
status: accepted
date: 2026-08-30
supersedes: ADR-0055, ADR-0056, ADR-0057, and ADR-0058 (maintained generator and command consequences only)
---

# ADR-0060 — Retire completed planning-campaign generators

## Context

The Product Owner has explicitly decided that delivery horizons and epic labels
are planning provenance only. The H4-4B, H4-4C, H4-6C, and H4-6D campaigns are
complete evidence-producing exercises, but their local receipt generators were
still maintained as product packages and `ardents-control` routes. Keeping
those generators would turn completed planning machinery into a permanent
runtime surface and obscure the domain owners that enforce the useful
behavior.

## Decision

Retire `internal/namespacelifecyclesimulation`,
`internal/rootclaimsimulation`, and `internal/publiccontrolsimulation`, plus
their four `ardents-control simulate-*` routes. Historical receipts, schemas,
commands, and candidate identifiers remain unchanged in retained research,
ADRs, external evidence, and immutable Git history. They are provenance, not a
compatibility contract or a command that current product code must reproduce.

The domain behavior exercised by the campaigns remains at its actual owners:

- At the time of this decision, `internal/publiccontrol` tests retained the
  diagnostic malformed, forged, stale, replayed, revoked, conflicting,
  unavailable, boundary-collision, and external-evidence outcomes. The later
  command-surface contraction left that future-public reader without a
  production caller, so its exact final source and tests are now historical Git
  evidence at `0e580c153114dd32f4b4c1fff86842b882f71937:internal/publiccontrol`
  rather than a maintained placeholder Module. Release tests continue to own
  consecutive-root, floor, expiry, emergency, build-attestation, and
  no-fallback decisions. Synthetic custodian, builder, and auditor roles had no
  product Interface and are not migrated.
- Namespace Record, Epoch, and Authority tests own publish/update, Grace,
  Released refusal, next-generation reclaim, threshold current state, restart,
  stale state, conflict, and pending-successor continuity. The retirement adds
  the previously unique assertion that two signed successors of one
  predecessor cannot enter one Epoch.
- Claim and Epoch tests own admission, authenticated input order, threshold
  close, winner materialization, lease/Grace derivation, withholding,
  incomplete rejection evidence, rule fork, and authenticated equivocation.

Recreating one of the old campaigns requires a new decision-relevant research
question and a disposable experiment or newly selected product Interface; it
does not justify restoring these generators.

## Consequences

- Maintained command and package names no longer encode completed epic work.
- Formal audit and qualification evaluate one frozen current product candidate,
  not the ability to regenerate historical planning receipts.
- Historical JSON schemas and receipts are not renamed, rewritten, or silently
  promoted into a product protocol.
- ADR-0055 through ADR-0058 remain authoritative for what their completed
  evidence meant. Their requirement to keep local generators and commands is
  superseded.

## Compliance

- [R-124](../research/records/r-124-public-control-candidate-evidence.md)
- [R-125](../research/records/r-125-controlled-project-control-transitions.md)
- [R-126](../research/records/r-126-project-control-canonical-name-lifecycle.md)
- [R-127](../research/records/r-127-project-control-root-claims.md)
- [Package map](../development/package-map.md)
- [Testing policy](../development/testing.md)
