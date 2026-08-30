---
id: R-125
title: Can project-controlled transitions accept only continuous overlap and otherwise stop or become unavailable?
status: decided
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-125 — Controlled project-control transitions

## Decision this unlocks

Close H4-6D as a reproducible Product Owner-and-Codex transition simulation,
without treating public participants or independent operators as a prerequisite.

## Current contract

The [H4-6 journey](../../product/horizon-4/06-transparent-control-transition.md),
[threat model](../../security/threat-model.md), and [glossary](../../../CONTEXT.md)
apply. ADR-0004 separates control roots, ADR-0054 separates alpha transition
domains, and ADR-0055 fixes H4-6C as project-controlled mechanics. ADR-0056
selects only this H4-6D evaluator; public operation remains unselected.

## Hypotheses

- **H1:** One bounded local evaluator can accept continuous overlap and expose
  every selected unsafe transition as an exact stop or unavailable result.
- **H2:** An unsafe transition may be repaired by choosing another source or
  older generation without reducing the stated safety boundary.
- **H0:** Any required stop is accepted, ambiguous, or not reproducible.

## Evaluation criteria

The receipt must contain its schema, contract, caller-declared source revision,
digest, `simulation: true`, `qualified: false`, and `simulation_result: passed`.
It must pass exactly: continuous overlap, expiry stop, revocation stop,
incompatible-generation stop, rollback stop, distribution-outage unavailable,
and live disable-only emergency stop. It must reject missing continuity,
emergency escalation, and expired emergency.

The protected information is only ephemeral test inputs; the adversary is a
malicious or failed control/distribution input attempting downgrade, replay,
revocation bypass, incompatible activation, or emergency escalation. There is
no network, participant, availability, latency, bandwidth, storage, operator,
or governance dependency beyond the Product Owner decision. A test timeout,
resource error, missing result, or fallback is failure. Standard-library Go is
the sole implementation surface; no dependency, license, distribution channel,
or accessibility promise is added. Developer experience is one documented
command and non-secret JSON receipt.

## Evidence plan

### Primary sources

- [The Update Framework specification](https://theupdateframework.github.io/specification/v1.0.28/), accessed 2026-08-29.
- ADR-0004, ADR-0054, ADR-0055, and ADR-0056, accessed 2026-08-29.

### Experiment

Historical reproduction only: use an isolated checkout of the accepted
implementation revision `6d2280213496c37eef44c9ce4003a8638e6c8625` and run:

```powershell
go run ./cmd/ardents-control simulate-public-control-transitions --source-revision 6d2280213496c37eef44c9ce4003a8638e6c8625
```

ADR-0060 retires this route from the current command surface; do not substitute
the current `HEAD`. Retain its historical JSON
receipt outside Git. It must identify the revision, versioned outcome matrix,
and `simulation: true`/`qualified: false`; any other result falsifies the run.
The simulation has no network or persistent authority.

### Failure scenarios

Expiry, revocation, incompatible generation, rollback, distributor outage, and
disable-only emergency must have their exact result. A missing predecessor
continuity, escalated emergency, and expired emergency must reject. A withheld
input is unavailable, not proof of malice; there is no recovery by fallback.

## Findings

- **Sourced fact:** trusted-update transition systems retain a floor and reject
  expired or unauthorized metadata rather than silently downgrading.
- **Measurement:** the evaluator records seven required bounded outcomes and
  three invalid-transition rejections in one local receipt.
- **Assumption:** the Product Owner-and-Codex team is the only project team
  relevant to this selected simulation.
- **Inference:** this is sufficient to close H4-6D's simulation criterion and
  insufficient for independent or public-operation claims.

## Options

1. **Require external actors** — rejected: it creates an unowned staffing and
   governance dependency and does not add simulator mechanics.
2. **Allow fallback/repair after a failed transition** — rejected: it weakens
   rollback and outage handling and hides the exact unsafe state.
3. **Accept the bounded local matrix** — selected: it fits the actual team,
   fails closed, requires no new dependency or operation, and preserves the
   public-claim boundary.

## Recommendation

Choose option 3. Confidence is high because every criterion is directly
executable. The strongest limitation is that a local simulation cannot prove
independent operation, public availability, or Public Beta readiness.

## Disposition

**Decided for H4-6D.** ADR-0056 and this record retain the completed evidence.
ADR-0060 later retired the campaign generator and command after cross-checking
its floor, expiry, revocation, compatibility, emergency, and no-fallback
assertions against their domain owners. The historical command, schema, and
receipt are unchanged. Security and operations documents need no new procedure
because the campaign had no deployment, authority, network, or VPS action. A
public claim needs a new Product Owner decision; it is not residual H4-6D work.
