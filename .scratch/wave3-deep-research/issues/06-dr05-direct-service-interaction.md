# DR-05: Define Direct Service Interaction

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R2

## Parent

`../PRD.md`

## What to build

Decide whether Ardents ends at trusted service discovery or also provides a
bounded client adapter for direct service use. Define Principal-to-service
authentication, service authorization and its relationship to Access Grants
and Delegation, TLS identity, certificate rotation and endpoint pinning,
request/stream limits, retry and error semantics, and the exact boundary where
Ardents ends and the application protocol begins.

The result must consume the accepted Application Hosting ownership model and
must not turn Ardents into a general service mesh.

## Acceptance criteria

- [ ] The current endpoint publication and discovery journey is evidenced from the frozen baseline.
- [ ] The selected design consumes the accepted DR-02 ownership/lifecycle model.
- [ ] Discovery-only and client-adapter alternatives are compared end to end.
- [ ] Principal authentication, service authorization, TLS identity, rotation, pinning, limits, retry, and errors are explicit.
- [ ] The application-protocol boundary and unsupported behavior are explicit.
- [ ] Privacy, abuse, observability, compatibility, and migration consequences are explicit.
- [ ] A proposed ADR decision and vertical implementation slices, or an explicit defer decision, are ready for review.

## Blocked by

- DR-02

## Comments

None.
