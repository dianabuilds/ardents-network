---
id: R-127
title: Can the Product Owner and Codex reproduce deterministic root claims through threshold-current Namespace state?
status: decided
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-127 — Project-control root claims

## Decision this unlocks

Close H4-4C as a project-controlled root-claim mechanics simulation without
claiming a public Namespace or requiring unavailable third parties.

## Current contract

The [H4-4 journey](../../product/horizon-4/04-namespace-private-resolution.md),
[threat model](../../security/threat-model.md), and [glossary](../../../CONTEXT.md)
apply. ADR-0017 fixes commit in `E`, reveal in `E+1`, lowest eligible input
ordinal, and fail-closed close evidence. ADR-0019 fixes the bounded local
root-claim admission guard. ADR-0020 fixes threshold-authenticated current
Namespace state. ADR-0058 selects only the bounded project-control evidence.

## Hypotheses

- **H1:** two locally admitted committed reveals, a `2-of-3` authenticated
  close, and `EpochInstallation.MaterializeClaim` produce only the lowest
  ordinal root as threshold-current state.
- **H2:** an incomplete, withheld, rule-conflicting, or equivocal close cannot
  produce equivalent state from a local corpus or signer.
- **H0:** an invalid close is accepted or the exact current state cannot be
  reproduced.

## Evaluation criteria

One versioned non-secret receipt must identify its revision/digest and report
`simulation: true`, `qualified: false`, the winner, authenticated close,
threshold-current materialization, one finite Active-to-Grace lease, and
rejection of withheld reveal, incomplete evidence, rule fork, and control
fork. The adversary may withhold, tamper with, replay, or equivocate local
control evidence. There is no corpus fallback. Missing output, timeout,
resource error, unbounded proof, or acceptance of any named fault fails.

Governance is limited to the synthetic fixed `2-of-3` close and `2-of-3`
materialization keys in this run. It establishes no legitimate public control
authority. The ADR-0019 work proof is only a finite local amplification guard;
it is not Sybil resistance, fairness, personhood, payment, or anti-squatting.
The selected Record has a one-hour Active lease plus one-hour Grace; expiry,
renewal, reclaim, and operation after Grace remain H4-4B lifecycle concerns.

## Evidence plan

### Primary sources

- ADR-0017, ADR-0019, ADR-0020, ADR-0023, ADR-0057, and ADR-0058, accessed
  2026-08-29.
- Maintained `claim`, `epoch`, `record`, and `admission` source/tests, accessed
  2026-08-29.

### Experiment

Historical reproduction only: use an isolated checkout of the accepted
implementation revision `baeed253cf55c3689ef8d4592dd09d8839ccf29c` and run:

```powershell
go run ./cmd/ardents-control simulate-root-claims --source-revision baeed253cf55c3689ef8d4592dd09d8839ccf29c
```

ADR-0060 retires this route from the current command surface; do not substitute
the current `HEAD`. Retain its historical JSON receipt outside Git. It creates two
locally admitted commitments in Epoch 8, reveals them in Epoch 9 only after the
commitment is fixed, assigns authenticated input ordinals, and uses no alpha
corpus or network input.

### Failure scenarios

Withheld reveal, missing rejection material, incompatible close rule, and two
different authenticated close statements must fail before current state is
opened. A signer-substituted Record and invalid threshold attestation remain
covered by maintained `epoch` behavior tests; restart/corpus fallback is
H4-4B evidence, not an H4-4C receipt claim.

## Findings

- **Measurement:** the simulator admits two committed reveals, chooses ordinal
  zero, and makes only that authority a signed record through a threshold
  `EpochInstallation.Commit`.
- **Measurement:** withheld, incomplete, rule-fork, and control-fork inputs
  fail before Namespace current-state mutation.
- **Inference:** the H4-4C mechanics are reproducible by the actual team, but
  this does not establish permissionless public operation or governance.

## Options

1. **Treat a local corpus as current** — rejected: ADR-0020 explicitly
   requires threshold-attested current state.
2. **Claim a public permissionless root now** — rejected: no public Epoch
   operation, governance legitimacy, availability evidence, or independent
   validation has been selected.
3. **Run the maintained threshold-current mechanics locally** — selected:
   verifies the actual commit/reveal/close/materialization boundaries without
   inventing staff, payment, a registrar, or a public authority.

## Recommendation

Choose option 3. Confidence is high for the bounded mechanics. The strict
limit is that a future public root-claim programme needs a separately selected
authority, governance, operations, and evidence decision.

## Disposition

**Decided for H4-4C project-control scope.** ADR-0058 and this record retain the
completed evidence. ADR-0060 later retired the campaign generator and command
after moving its unique lease/Grace and incomplete-rejection assertions into
Claim tests and cross-checking the remaining outcomes against admission,
ordering, and Epoch tests. The historical command, schema, and receipt are
unchanged. No VPS, deployment, public authority, or user action occurs.
