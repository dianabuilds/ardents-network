# DR-05: Define Direct Service Interaction

Status: ready-for-agent
State: closed
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

- [x] The current endpoint publication and discovery journey is evidenced from the frozen baseline.
- [x] The selected design consumes the accepted DR-02 ownership/lifecycle model.
- [x] Discovery-only and client-adapter alternatives are compared end to end.
- [x] Principal authentication, service authorization, TLS identity, rotation, pinning, limits, retry, and errors are explicit.
- [x] The application-protocol boundary and unsupported behavior are explicit.
- [x] Privacy, abuse, observability, compatibility, and migration consequences are explicit.
- [x] A proposed ADR decision and vertical implementation slices, or an explicit defer decision, are ready for review.

## Blocked by

- DR-02

## Comments

Accepted 2026-07-25 after integrator review.

Evidence:

- `docs/engineering/research/direct-service-interaction.md`
- Proposed `docs/adr/0014-end-direct-service-interaction-at-discovery.md`
- canonical capability remains `partial/no/no/no`

Selected discovery-only v1. Ardents ends after authenticated, authorized,
privacy-filtered `Discovery.Resolve`; the Application and service own dialing,
TLS identity, credentials, authorization, protocol limits, retry, and errors.
Rejected a Direct Service adapter, proxy, Access-Grant translation, TOFU, and
Application-owned HTTPS Hosting without a separate certificate/secret
lifecycle decision.

Validation:

- Wave 3 packet and ADR contract review
- compatibility review against accepted DR-02 and DR-04 research
- documentation, architecture, and capability-catalogue tooling gates
