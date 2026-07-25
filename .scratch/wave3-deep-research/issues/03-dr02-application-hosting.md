# DR-02: Decide Application Hosting ownership and lifecycle

Status: ready-for-agent
State: closed
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

- [x] The current Operator/runtime hosting journey and Application reachability gap are evidenced.
- [x] At least two materially different ownership/lifecycle designs are compared.
- [x] The selected external interface hides workload, hosting, publication, and ingress orchestration.
- [x] Authority, state ownership, lease, readiness, drain, restart, withdrawal, and recovery semantics are explicit.
- [x] Protocol, ingress, privacy, abuse, observability, compatibility, and migration consequences are explicit.
- [x] Direct client interaction is left to DR-05 behind a named dependency.
- [x] A proposed ADR decision and vertical implementation slices are ready for review.

## Blocked by

- W3-00

## Comments

- Accepted on 2026-07-25 against frozen product baseline
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.
- Packet:
  `docs/engineering/research/application-hosting.md`.
- Selected one owner-qualified leased Hosted Service aggregate over an
  immutable Operator-approved managed-workload profile. The profile pins the
  canonical service type; Hosting persists the aggregate, journal, and
  immutable profile-revision archive as one recovery group. Workload,
  readiness, ingress, and publication remain derived projections.
- Rejected arbitrary endpoint registration, independent low-level Application
  workload/publication controls, delegated Hosting, and Application-selected
  HTTPS before DR-05.
- Proposed ADR:
  `docs/adr/0012-lease-application-hosting-through-approved-profiles.md`;
  it remains Proposed pending maintainer approval.
- Reviewed AH-01 through AH-05 as dependency-ordered vertical slices. DR-05
  consumes the ownership/lease/endpoint-hiding boundary; DR-04 supplies
  deployment topology for later qualification.
- Checks passed: complete packet review, `go test ./tests/tooling/... -count=1`,
  `go run ./tests/tooling/capabilitycatalog -check` (24 capabilities,
  8 domains, 0 qualified), Markdown/trailing-whitespace checks, and
  `git diff --check`. Implementation, deployment, E2E, security, and release
  qualification were not run because this issue delivers research only.
- Canonical I/R/O/Q remains `no/no/no/no`; no release qualification is claimed.
