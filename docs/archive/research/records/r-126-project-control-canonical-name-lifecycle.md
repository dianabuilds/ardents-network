---
id: R-126
title: Can the Product Owner and Codex reproduce the complete canonical Name lifecycle through threshold-authenticated current state?
status: decided
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-126 — Project-control canonical Name lifecycle

## Decision this unlocks

Close H4-4B as a project-controlled lifecycle simulation, without implying a
public Namespace or waiting for unavailable external operators.

## Current contract

The [naming contract](../../technical/naming.md),
[threat model](../../security/threat-model.md), and [glossary](../../../CONTEXT.md)
apply. ADR-0020 requires threshold-attested current materialization; ADR-0023
requires durable pending successors; ADR-0043 selects derived Grace. ADR-0057
selects the bounded H4-4B simulation only.

## Hypotheses

- **H1:** durable pending successors plus threshold-attested Epoch installation
  reproduce publication, update, Grace, Released, reclaim, and restart.
- **H2:** a caller-built corpus or old generation establishes equivalent current
  state without that durable threshold path.
- **H0:** a required lifecycle result is accepted incorrectly or unreproducible.

## Evaluation criteria

One versioned non-secret receipt must report its revision/digest,
`simulation: true`, `qualified: false`, publication/update threshold-current,
Grace warning, Released unavailable, generation-two reclaim, restart, and
rejection of stale replay, forked successor, conflicting current state, and
old-generation reclaim. The
adversary can replay, fork, downgrade, or withhold local control input; no
network, participant, external operator, availability, or governance claim is
made. Missing result, fallback, corpus current state, timeout, or resource
error fails. Standard-library Go and maintained Namespace modules are the only
dependency/distribution profile.

## Evidence plan

### Primary sources

- ADR-0020, ADR-0022, ADR-0023, ADR-0043, and ADR-0057, accessed 2026-08-29.
- [The Update Framework specification](https://theupdateframework.github.io/specification/v1.0.28/), accessed 2026-08-29.

### Experiment

Historical reproduction only: use an isolated checkout of the accepted
implementation revision `b908363c3ded0d4d921fa6ffbb4836b31434372e` and run:

```powershell
go run ./cmd/ardents-control simulate-namespace-lifecycle --source-revision b908363c3ded0d4d921fa6ffbb4836b31434372e
```

ADR-0060 retires this route from the current command surface; do not substitute
the current `HEAD`. Retain its historical JSON receipt
outside Git. It uses a temporary local Store and keys, not an alpha corpus or
network input; any result outside the stated versioned contract falsifies it.

### Failure scenarios

Expired/released state, old-generation reclaim, stale Epoch commit, durable
fork, conflicting current state, and restart must fail closed or retain only
exact current state.

## Findings

- **Measurement:** the simulator records six lifecycle outcomes and four
  rejections through durable Store and `2-of-3` Epoch attestation.
- **Measurement:** published Target validity extends through its signed Grace
  boundary and then fails at Grace expiry in the lifecycle simulation.
- **Inference:** this closes H4-4B simulation evidence, not public Namespace
  governance, availability, or independence.

## Options

1. **Use an alpha corpus as current state** — rejected: it violates ADR-0020,
   weakens the security boundary, depends on a distributor as control, and
   provides neither durable lifecycle evidence nor maintainable governance.
2. **Require a public operator programme** — rejected for this scope: it
   changes the product claim, adds unavailable staffing/governance and support
   costs, and has larger operational/distribution risk without adding mechanics.
3. **Use threshold-attested local simulation** — selected: it directly
   exercises the maintained lifecycle boundary, has no new dependency or
   license, and is operable by the actual team without a public claim.

## Recommendation

Choose option 3. Confidence is high for this simulation. Its strongest limit is
that it cannot establish public operation or governance.

## Disposition

**Decided for H4-4B.** ADR-0057 and this record retain the completed evidence.
ADR-0060 later retired the campaign generator and command after moving the one
unique pending-successor fork assertion into the Namespace Authority tests and
cross-checking the remaining lifecycle outcomes against Record and Epoch tests.
The historical command, schema, and receipt are unchanged. No operations or
security procedure changes are required because the campaign had no
deployment, authority, network, or VPS action.
