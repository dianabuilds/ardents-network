---
id: R-034
title: Which stage owns Bridge-specific capacity evidence?
status: accepted
owner: product research
started: 2026-08-15
reviewed: 2026-08-15
---

# R-034 — Stage 4 and Bridge capacity sequencing

## Decision this unlocks

Remove the cycle that required Stage 5 Bridge implementation before Stage 4
capacity work could finish, while keeping Stage 4 from selecting a camouflage
transport or making a blocked-entry claim.

## Current contract

[R-023](r-023-interactive-route-performance-budget.md) requires independently
measured infrastructure-role capacity. [R-033](r-033-h3-stage-5-research-map.md)
shows that the maintained Bridge, Invite state, and Camouflage Adapter belong to
Stage 5. A synthetic Bridge workload cannot qualify that later implementation.

The Product Owner directed completion of all Stage 4 development on 2026-08-15.

## Hypotheses

- **H1:** S4.4 can close the roles implemented through Stage 4 while Stage 5
  owns Bridge-specific useful-work and capacity acceptance.
- **H2:** a disposable Bridge-shaped workload in Stage 4 provides useful
  qualification evidence for the maintained Stage 5 Adapter.
- **H0:** Stage 4 must remain blocked until Stage 5 is implemented.

## Evaluation criteria

The decision must remove the ordering cycle, avoid qualifying synthetic work,
preserve the accepted reference-host and post-exclusion rules, and add no Stage
5 transport, state, dependency, or claim to Stage 4.

## Evidence plan

### Primary sources

Accessed 2026-08-15:

- R-023 for the role-capacity and reference-host contract;
- R-032 and the Stage 4 brief for the recovery/capacity boundary; and
- R-033 for the documented sequencing cycle and the two bounded options.

No external source or new experiment is needed: this is ownership of already
defined work, not a technology selection.

### Failure scenarios

- synthetic forwarding is reported as Bridge or camouflage capacity;
- Stage 4 silently adds an Invite, Adapter, or blocked-entry claim;
- Bridge capacity is omitted from Stage 5 qualification; or
- moving the work changes role trust, selection weight, or exclusion rules.

## Findings

**Sourced fact:** Stage 4 implements no maintained Bridge or Camouflage Adapter,
while Stage 5 explicitly owns both.

**Inference:** measuring a disposable Bridge-shaped forwarder in Stage 4 would
produce a number that cannot qualify the later selected implementation and
would obscure rather than resolve the dependency.

## Options

1. Use a disposable Bridge workload in Stage 4. Rejected because the result is
   neither product capacity nor reusable Stage 5 evidence.
2. Move Bridge-specific useful-work and capacity acceptance to Stage 5, and
   close S4.4 over roles implemented through Stage 4. Accepted.
3. Block Stage 4 on Stage 5. Rejected because it reverses the accepted horizon
   order without improving evidence.

## Recommendation

Choose option 2 with high confidence. S4.4 must still measure each maintained
Stage 4 infrastructure role separately under the accepted reference profile,
maximum applicable exclusions, pressure, lifecycle, and scale-up gates. Stage 5
must define and qualify Bridge capacity for the selected Adapter; no Stage 4
measurement transfers credit to it.

The strongest counterargument is that Stage 4 no longer reports one number for
every future infrastructure role. That is honest: a role-specific floor cannot
precede the role whose cost it claims to measure.

## Disposition

- State: accepted by the Product Owner's 2026-08-15 instruction to complete all
  Stage 4 development.
- S4.4 is authorized for roles implemented through Stage 4.
- Bridge-specific capacity acceptance is a Stage 5 prerequisite.
- No experiment code, dependency, package, binary, or public claim is created
  by this decision.
