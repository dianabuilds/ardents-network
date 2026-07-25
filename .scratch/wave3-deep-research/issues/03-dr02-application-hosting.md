# DR-02: Decide Application Hosting ownership and lifecycle

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R2

## Parent

`../PRD.md`

## What to build

Select one Application-facing hosting lifecycle above the existing workload,
hosting, publication, and ingress orchestration. Decide whether an Application
registers an existing endpoint or owns a managed workload, how the Application
Principal relates to the workload and published service, and how leases,
readiness, renewal, drain, crash, restart, protocol policy, local-only mode,
network publication, and withdrawal behave.

The caller must receive one coherent lifecycle rather than separate low-level
administrative interfaces.

## Acceptance criteria

- [ ] The current Operator/runtime hosting journey and Application reachability gap are evidenced.
- [ ] At least two materially different ownership/lifecycle designs are compared.
- [ ] The selected external interface hides workload, hosting, publication, and ingress orchestration.
- [ ] Authority, state ownership, lease, readiness, drain, restart, withdrawal, and recovery semantics are explicit.
- [ ] Protocol, ingress, privacy, abuse, observability, compatibility, and migration consequences are explicit.
- [ ] Direct client interaction is left to DR-05 behind a named dependency.
- [ ] A proposed ADR decision and vertical implementation slices are ready for review.

## Blocked by

- W3-00

## Comments

None.
