---
status: accepted
date: 2026-08-29
supersedes: none
---

# ADR-0058 — Simulate H4-4C deterministic root claims

## Context

ADR-0017 selects authenticated commit/reveal ordering and ADR-0020 selects
threshold-current Namespace materialization, but H4-4C needs one maintained
end-to-end evidence path. The actual project team is the Product Owner and
Codex, not an assumed public operator organisation.

## Decision

H4-4C is completed as the bounded local command `ardents-control
simulate-root-claims --source-revision LOWERCASE_40_HEX_COMMIT`. It admits two independent
commitments in Epoch `E` under ADR-0019's root-claim guard, reveals them in
`E+1`, fixes authenticated input ordinals, accepts only the lowest ordinal under a
`2-of-3` Epoch close, and materializes that winner only through
`EpochInstallation.MaterializeClaim` and a distinct `2-of-3` current-state
attestation. The selected root has a one-hour Active lease and a one-hour
Grace boundary.

Withheld reveal, incomplete rejection evidence, incompatible rule, and two
different authenticated close statements fail closed and mutate no Namespace
state. The receipt is non-secret and explicitly `simulation: true` and
`qualified: false`. Temporary keys and Store state are removed after each run.

## Consequences

- Current Namespace state remains a threshold-attested Epoch result, never a
  caller-built corpus, claim signer result, or alpha corpus.
- The accepted anonymous work profile remains a bounded local amplification
  guard. It does not provide Sybil resistance, fair allocation, personhood,
  payment, anti-squatting, or a governance system.
- A captured threshold remains able to censor or equivocate; evidence makes
  that visible and resolution fails closed. This simulation supplies no public
  authority legitimacy, independent operation, availability, or Public Beta.

## Compliance

- ADR-0004, ADR-0017, ADR-0019, ADR-0020, ADR-0023, and ADR-0057
- [R-127](../research/records/r-127-project-control-root-claims.md)
