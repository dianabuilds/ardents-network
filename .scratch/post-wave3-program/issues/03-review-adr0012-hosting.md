# PW3-03: Review ADR-0012 Application Hosting

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R2 architectural decision

## Parent

`../PRD.md`

## User story

As a maintainer, I want to review the proposed Application Hosting authority,
state, recovery, and compatibility contract before implementation so that an
Application cannot acquire workload, ingress, publication, or recovery
authority through an underspecified convenience API.

## Outcome

Return exactly one review result:

- `review-ready`: ADR-0012 may be accepted by an explicit maintainer decision;
- `returned with blockers`: named contract defects must be resolved before
  acceptance;
- `rejected with rationale`: the selected lifecycle is not approved.

This issue records review. It does not itself change ADR status, authorize
implementation, or qualify a capability.

## What to review

Review `docs/adr/0012-lease-application-hosting-through-approved-profiles.md`
against:

- `docs/engineering/research/application-hosting.md`;
- `docs/engineering/research/application-discovery.md`;
- `docs/engineering/research/direct-service-interaction.md`;
- `docs/engineering/research/wave3-synthesis.md`;
- ADR-0001, ADR-0002, ADR-0008, and the current workload, readiness, ingress,
  publication, backup/restore, and downgrade contracts.

The selected design is one finite leased, owner-qualified Hosted Service over
an immutable Operator-approved managed-workload profile. Arbitrary endpoint
registration and independent Application workload/publication controls remain
rejected.

## In scope

- the public Hosted Service lifecycle and its explicit non-goals;
- owner, Actor, Effective Principal, Delegation, Access Grant, workload, and
  publication-signing authority;
- immutable profile revisions and profile-pinned canonical service type;
- authoritative state, derived projections, journal, archive, garbage
  collection, and consistency-group ownership;
- lease, renewal, drain, expiry, revocation, readiness, publication, and
  withdrawal ordering;
- restart, recovery, backup/restore, migration, rollout, mixed schema, and
  downgrade refusal;
- finite resource, rate, list, retention, lease, and history bounds;
- compatibility with the accepted Application Discovery locator and the
  discovery-only Direct Service handoff;
- whether AH-01 through AH-05 are safe dependency-ordered tracer bullets.

## Out of scope

- changing `Status: Proposed` to `Accepted` without explicit maintainer
  approval;
- implementing any Hosting protocol, SDK, store, profile, workload, ingress,
  publication, or Operator surface;
- qualifying `application.hosting`;
- adding arbitrary endpoint registration, delegated Hosting, caller-supplied
  runtime material, Application-selected HTTPS, a Direct Service adapter,
  remote Application transport, non-Go SDKs, Kubernetes, QUIC, WebTransport,
  or WebRTC;
- editing the canonical capability catalogue or evidence register.

## Dependencies

- Wave 3 DR-02 research recommendation is complete.
- The Application Discovery locator boundary is an accepted design input, not
  current implementation truth.
- ADR-0001 through ADR-0010 remain accepted fixed inputs.
- ADR-0014 review is downstream and must not be accepted before this review
  preserves the Hosting ownership, lease, profile, and endpoint-hiding
  boundary.

## Authority review checklist

- [ ] The authenticated Actor Application Principal is the durable Hosted
  Service owner.
- [ ] Hosting mutations require `Actor == Effective`; presented Delegation is
  rejected in v1.
- [ ] Ensure requires exact `application.hosting.ensure` authority on
  `hosting-profile:<profile-id>`.
- [ ] `NetworkPublished` additionally requires
  `application.hosting.publish.network` on the same profile.
- [ ] Later reads and mutations target
  `hosted-service:<owner>/<service-id>` and cannot cross owners.
- [ ] The immutable profile revision, not the caller, pins the canonical
  service type.
- [ ] The Application cannot supply image, command, environment, runtime,
  resources, endpoint, port, probe, certificate, secret, policy, mount,
  device, or Docker options.
- [ ] The derived workload has no Principal.
- [ ] The Node Principal signs discovery publication without lending authority
  to the Application or workload.
- [ ] Access Grant revocation blocks later calls but does not silently destroy
  a running resource; lease expiry or explicit Hosted Service/profile-revision
  revocation owns termination.

## State and ownership review checklist

- [ ] One versioned Hosting consistency group is authoritative desired truth.
- [ ] The group contains Hosted Service aggregates, mutation journal, logical
  profile current/disabled pointers, and canonical immutable profile-revision
  bytes.
- [ ] Every aggregate pins `(profile_id, profile_digest)`; a digest without the
  archived canonical revision is insufficient.
- [ ] Workload execution, generation readiness, ingress, and publication are
  derived projections rather than competing desired-state authorities.
- [ ] Existing Operator-created workloads remain Operator-owned, unleased, and
  are never adopted.
- [ ] A reserved ownership marker prevents ordinary Operator WorkloadService
  mutation of derived Application-owned workload IDs.
- [ ] Profile update creates a new revision and advances only the current
  pointer; existing aggregates never silently change Plan or service type.
- [ ] Profile disable/logical delete blocks new Ensure but does not terminate
  or make existing pinned aggregates unrecoverable.
- [ ] Physical revision collection cannot break a current pointer, aggregate,
  terminal tombstone, or in-flight journal reference.

## Bounds and abuse review checklist

- [ ] Canonical name/profile/request IDs are at most 128 bytes.
- [ ] One Hosted Service maps to exactly one workload and one service.
- [ ] A Node admits at most 256 current logical profiles and 1,024 retained
  current-or-archived revisions.
- [ ] Lease duration is bounded to 1–60 minutes with a 15-minute default.
- [ ] Normal drain is profile-selected and capped at 30 seconds.
- [ ] Each Principal has at most eight active Hosted Services; the existing
  Node workload/policy ceiling remains the global execution cap.
- [ ] Terminal records are bounded to 32 per Principal and 1,024 per Node and
  retained for 24 hours before eligible compaction.
- [ ] Unreferenced revisions remain retained for an additional 24 hours.
- [ ] List returns at most eight owner-visible records.
- [ ] Mutation admission is bounded per Principal and per Node, with one
  in-flight mutation per service.
- [ ] RequestID idempotency history retains only the latest eight entries.
- [ ] Archive pressure rejects creation; it never evicts referenced state.
- [ ] Metrics, logs, and public errors do not expose Principal IDs, endpoints,
  images, ports, container IDs, secrets, RequestIDs, or policy internals.

## Restart, recovery, and compatibility review checklist

- [ ] Startup loads Hosting truth and journal before any Application-owned
  service can be published.
- [ ] Startup order is: load truth; mark expired/revoked resources withdrawn;
  reconcile labelled runtime; resume/compensate journal; start active desired
  resources; establish current-generation readiness; then publish.
- [ ] Expiry and revocation win over stale runtime or publication observations.
- [ ] Missing revision, digest/type mismatch, corrupt bytes, or unknown schema
  fails closed before execution, recovery, renewal, or publication.
- [ ] A wall-clock rollback greater than 30 seconds cannot extend authority and
  causes fail-closed withdrawal.
- [ ] Backup contains the complete Hosting consistency group and every
  referenced profile revision or fails.
- [ ] Restore validates the complete group before replacing live truth and
  leaves prior live truth unchanged on a missing/corrupt revision.
- [ ] Restore re-evaluates absolute expiry and revocation before execution or
  publication.
- [ ] Rollout enables writers only after compatible readers are deployed.
- [ ] Old binaries refuse downgrade while any Hosting marker, aggregate,
  journal, pointer, or archived revision exists.
- [ ] Application-owned HTTPS remains outside v1 until a separate certificate,
  private-key delivery, rotation, and same-identity readiness decision.

## Handoff review checklist

- [ ] Hosting Status may project safe service type and lifecycle state but does
  not expose or open an endpoint.
- [ ] Discovery alone discloses an eligible bounded locator.
- [ ] Resolve authority is not service-use authority.
- [ ] Direct connection, TLS, service credentials, authorization, protocol,
  retry, and application errors remain Application/service responsibilities.
- [ ] ADR-0014 can consume this boundary without changing Hosted Service
  ownership, lease, profile revision, readiness, publication, or withdrawal.

## Acceptance criteria

- [ ] Every checklist item is either confirmed or recorded as a named blocker
  with source references and required resolution.
- [ ] The proposed ADR matches the source research packet on authority, state
  ownership, bounds, restart/recovery, compatibility, and non-goals.
- [ ] AH-01 is confirmed as the durable local tracer, followed by AH-02
  managed workload readiness, AH-03 publication/withdrawal, AH-04 Operator
  recovery/migration, and AH-05 qualification.
- [ ] The review explicitly states that ADR acceptance does not imply
  implementation, reachability, operability, or qualification.
- [ ] The review explicitly keeps capability `application.hosting` at
  `I=no/R=no/O=no/Q=no` until matching implementation evidence exists.
- [ ] One of the three permitted review outcomes is recorded.
- [ ] ADR status remains unchanged unless a separate explicit maintainer
  approval is received and integrated as its own governance change.

## Required validation and evidence

- line-by-line ADR-to-research contract review;
- compatibility review against Application Discovery, DR-05, ADR-0001,
  workload/readiness/publication, backup/restore, and downgrade contracts;
- capability catalogue check:
  `go run ./tests/tooling/capabilitycatalog -check`;
- governance/tooling tests:
  `go test ./tests/tooling/... -count=1`;
- documentation and architecture acceptance checks through their canonical
  repository runners;
- `git diff --check`;
- retained review comments or checklist result tied to the reviewed commit.

These checks validate the review packet only. They are not implementation,
deployment, E2E, security, or release qualification evidence.

## Capability impact and no-Q rule

- Expected immediate capability impact: none.
- `application.hosting` remains `I=no/R=no/O=no/Q=no`.
- Existing `workload.lifecycle` and `hosting.operator-publication` claims do
  not prove Application Hosting.
- ADR acceptance alone cannot change I/R/O or Q.
- `Q=yes` is forbidden until AH-05 and the DR-06 matching-commit release gate
  are accepted.

## Expected files and modules

Review should be limited to:

- `docs/adr/0012-lease-application-hosting-through-approved-profiles.md`;
- `docs/engineering/research/application-hosting.md`;
- `docs/engineering/research/application-discovery.md`;
- `docs/engineering/research/direct-service-interaction.md`;
- `docs/engineering/research/wave3-synthesis.md`;
- this issue's comments/checklist.

No product module is expected to change in this issue.

## Blocked by

Explicit maintainer review and decision.

## Exit condition

The issue exits only when a complete review result is recorded as
`review-ready`, `returned with blockers`, or `rejected with rationale`.
Implementation remains blocked until a separate explicit maintainer approval
accepts ADR-0012.

## Comments

- Source packet slice naming is authoritative: AH-01 is the durable local
  Hosted Service tracer, not a documentation-only contract-freeze task.
