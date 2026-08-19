---
id: R-046
title: What exact field-level role matrix enforces Stage 6 naming knowledge separation?
status: open
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-046 — Stage 6 field-level role matrix

## Decision this unlocks

Freeze exact per-operation inputs, outputs, observations, stable identifiers,
Role Domains, known-family exclusions, and Isolation Context boundaries. S6.2
cannot claim compiler-enforced knowledge separation until the matrix names every
field rather than only naming roles.

## Current contract

R-039 requires that one ordinary Node never receive both User/Endpoint location
and the exact Service Name or lookup value under the stated conditions. Names
remain guessable; naming-side metadata, collusion, Correlated Control, and a
Broad Traffic Observer remain limitations. ADR-0005 supplies Role Domains, and
CONTEXT.md supplies Isolation Context and Network-Isolated Application Boundary.

The five candidate roles (`endpoint-adjacent`, `naming-rendezvous`,
`local-resolver`, `authority-operation`, `observer`) are vocabulary, not a
field-level decision. The former record did not list operation-specific request,
response, logging, error, retry, cache, or evidence fields and is reopened.

## Hypotheses

- **H1:** five operation-specific role families with concrete request and
  observation types can enforce the forbidden combined view.
- **H2:** fewer role families are sufficient if resolution and control
  operations use distinct types.
- **H0:** the current route/naming composition necessarily exposes the forbidden
  combined view and must be redesigned before Stage 6.

## Evaluation criteria

1. For claim, renew, resolve, record, delegate, policy, recovery, and observe,
   list every input, output, cache key, retry token, error field, log field, and
   retained evidence field.
2. Mark every field `required`, `optional`, or `forbidden` for every role; no
   superset object or convention-based hiding.
3. Bind identifiers to one role, operation, and Isolation Context; define
   lifetime, unlinkability boundary, and permitted evidence commitment.
4. Assign Role Domain and known-family exclusions to every network operation.
5. Missing/invalid role proof fails closed before name disclosure or Lease
   mutation and maps to an R-002 result.
6. State the protected information, adversary, conditions, measurement, and
   honest limitation for each claimed separation.

## Evidence plan

### Primary sources

- R-039, accessed 2026-08-20 — exact privacy and role-separation contract.
- ADR-0005, accessed 2026-08-20 — Role Domain and adjacency rules.
- R-005 and R-035, accessed 2026-08-20 — hostile bootstrap and Bridge state.
- R-002, accessed 2026-08-20 — Connection Result taxonomy.
- CONTEXT.md, accessed 2026-08-20 — Isolation Context, Application Principal,
  Role Domain, and Destination Resolution Role.

### Experiment

First produce the matrix as a checked-in table. Then build a bounded tracer that
serializes each role's actual request, response, error, log, and evidence shape.
Mutation tests add every forbidden field one at a time; the declared interface
and independent verifier must reject it. Run cross-context and known-family
reuse cases as separate cells.

### Failure scenarios

- Endpoint-adjacent input, logs, or errors contain exact name/lookup bytes.
- Naming-side input or observation contains User/Endpoint location.
- Retry, cache, or evidence identifiers link Isolation Contexts.
- One identity or known family occupies conflicting Role Domains.
- Missing role proof reaches a resolver or mutates a Lease.
- Observer or authority-operation input contains Application data not required
  for its operation.

## Findings

- **Sourced fact:** R-039 fixes the forbidden combined view and explicit
  limitations, not a concrete field layout.
- **Sourced fact:** ADR-0005 prevents conflicting Role Domain knowledge from
  being hidden behind one identity family.
- **Inference:** role names without field lists cannot be verified mechanically
  and permit leaks through errors, logs, retries, or caches.
- **Assumption:** concrete operation-specific Go types can encode the matrix once
  its fields are decided.

## Options

1. **Five role families with operation-specific concrete types.** Candidate;
   preserves current vocabulary but requires the full matrix.
2. **Fewer role families with separate resolution/control types.** Candidate if
   it produces the same field exclusions with a smaller interface.
3. **Superset object plus runtime visibility checks.** Rejected: hidden fields by
   convention do not enforce the accepted privacy boundary.

## Recommendation

Complete the matrix and mutation tracer before choosing between Options 1 and 2.
Confidence is high that the former record is incomplete. The strongest
counterargument is documentation size; that cost is necessary because omitted
error/log/cache fields are part of the privacy boundary.

## Disposition

- State: `open`; five role names remain candidate vocabulary, not a frozen
  field-level contract.
- S6.2 and dependent evidence schema work remain blocked.
- The next revision must contain the complete matrix and a coverage mapping from
  every Stage 6 operation to its concrete role inputs and observations.
