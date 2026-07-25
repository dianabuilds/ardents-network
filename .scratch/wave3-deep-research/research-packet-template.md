# <DR-ID>: <Title>

## Metadata

- Status: draft
- Research class: R2 deep research
- Decision owner:
- Research owner:
- Date:
- Frozen baseline commit:
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research:
- Downstream consumers:

## Answer first

State the recommendation, the user outcome it enables, and the main tradeoff in
no more than three paragraphs.

## User outcome

Describe one externally observable result. Do not describe an internal
subsystem as the outcome.

## Scope

### In scope

- _To be completed._

### Out of scope

- _To be completed._

## Current product truth

### Supported interfaces

List only interfaces supported by active contracts. Separate Operator,
Application, internal, deployment, and true external interfaces.

### Reachable journey

Describe the actual caller-to-domain path. Identify every point at which the
journey becomes internal-only or unavailable.

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
|  |  |  | implemented / reachable / operable / qualified |

Historical evidence must be labeled historical and must not substitute for
current commit-bound evidence.

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
|  |  |  |  |  |

Distinguish Ardents Principals, Waku Peer IDs, transport identities, Credentials,
Access Grants, Delegations, and Channel Grants.

## Invariants

- _To be completed._

Include authorization ordering, bounded resource use, privacy, restart truth,
and fail-closed behavior where applicable.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
|  | in-process / local-substitutable / remote-owned / true-external |  |  |  |

## Alternative designs

Provide at least two materially different designs. Renaming the same interface
does not count as an alternative.

### Alternative A

- External interface:
- Internal seam:
- State ownership:
- Authority model:
- Failure and recovery:
- Compatibility and migration:
- Operational cost:

### Alternative B

- External interface:
- Internal seam:
- State ownership:
- Authority model:
- Failure and recovery:
- Compatibility and migration:
- Operational cost:

### Decision matrix

| Criterion | Weight | Alternative A | Alternative B | Evidence or reasoning |
|---|---:|---:|---:|---|
| Module depth |  |  |  |  |
| Caller leverage |  |  |  |  |
| Change locality |  |  |  |  |
| Trust-model fit |  |  |  |  |
| Failure clarity |  |  |  |  |
| Migration cost |  |  |  |  |
| Operability |  |  |  |  |

## Selected design

### External interface sketch

Describe the smallest caller-facing interface. Do not expose Waku selectors,
encryption, replay stores, workload orchestration, deployment internals, or
certificate plumbing merely because they exist underneath.

### Internal seam and state machine

Include a compact state machine when lifecycle or recovery behavior matters.

### Authority and audit semantics

Specify Actor Principal, Effective Principal, exact actions/resources,
Delegation behavior, revocation, and attributable audit results.

## Delivery and data semantics

Cover ordering, acknowledgement, deduplication, expiry, limits, backpressure,
large payload references, and terminal outcomes where relevant. Mark
non-applicable rows explicitly.

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
|  |  |  |  |  |

## Security, privacy, and abuse analysis

Cover malicious authorized participants as well as unauthenticated attackers.
Specify quotas, cardinality bounds, redaction, enumeration resistance, replay,
key rotation, and recovery.

## Observability

Define bounded metrics, health/readiness effects, events, diagnostics, and
operator procedures. Do not place Principal IDs, secrets, endpoint credentials,
or unbounded peer/service labels in metrics.

## Compatibility consequences

State wire, persistence, configuration, backup/restore, rollout, downgrade, and
mixed-generation consequences.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit |  |  |  |
| Contract |  |  |  |
| Integration |  |  |  |
| E2E |  |  |  |
| Security |  |  |  |
| Deployment |  |  |  |
| Release |  |  |  |

## Open questions

No issue may be marked implementation-ready while an open question can change
the selected external interface, trust root, persistence model, or migration
contract.

## Decision-register proposals

Propose decision and question rows here. The stage integrator, not parallel
research agents, updates `wave3-decision-register.md`.

## Recommendation

Choose exactly one:

- implement;
- prototype one named uncertainty;
- write ADR before implementation;
- defer;
- reject.

## Vertically sliced implementation issues

For each proposed slice include:

- title;
- user story;
- complete end-to-end behavior;
- acceptance criteria;
- blocked by;
- research class after this packet.

Do not publish issue files until the maintainer approves granularity and
dependencies.
