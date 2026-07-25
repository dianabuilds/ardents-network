# PW3-06: AH-01 Persist a Durable Local Hosted Service Tracer

Status: needs-info
State: open
Labels: needs-info
Research class: R1 bounded implementation

## Parent

`../PRD.md`

## User story

As an authorized local Application, I want to Ensure one Operator-approved
local-only Hosted Service and read the same owner-safe durable status after a
Node restart so that the first Hosting slice proves the public contract,
authority, profile pinning, persistence, idempotency, expiry, and recovery
boundary before real workload and publication orchestration are added.

## Full vertical behavior

Implement the smallest durable Application Hosting tracer through the real
protected Application interface:

1. an Operator creates an immutable approved local-only profile revision whose
   canonical bytes pin one service type and a closed fake-runtime Plan;
2. the authenticated Application calls Ensure with only logical profile ID,
   permitted `http` or `tcp` protocol, `LocalOnly` exposure, name, bounded
   lease, and RequestID;
3. admission checks exact profile authority, rejects Delegation and arbitrary
   runtime material, resolves the current immutable revision, archives its
   canonical bytes, and persists one owner-qualified aggregate plus mutation
   journal;
4. the local fake runtime tracer derives a safe status without exposing
   workload, endpoint, port, image, policy, or runtime details;
5. Get/List return only owner-visible bounded status;
6. after Node restart, the aggregate resolves the exact archived revision and
   returns the same pinned service type, revision, lease, and safe lifecycle
   state;
7. expiry, corruption, missing revision, stale revision, replay, ownership,
   profile disable/delete/revoke, and archive-pressure cases fail according to
   the selected contract.

AH-01 establishes authoritative Hosting truth. It does not drive the existing
real workload runtime, readiness, ingress, or network publication.

## In scope

- logical profile catalogue with current/disabled pointers;
- canonical immutable content-addressed profile revisions;
- profile-pinned canonical service type;
- owner-qualified Hosted Service aggregate and bounded mutation journal;
- versioned `application-hosting/v1` consistency-group schema;
- exact Ensure and read authority/resources;
- Application wire and Go SDK Ensure/Get/List;
- `LocalOnly` and `http`/`tcp` validation;
- lease, revision, RequestID, idempotency, quota, list, tombstone, archive,
  privacy, and rate bounds;
- owner-safe typed status and errors;
- one local fake-runtime tracer;
- restart/load, expiry, missing/corrupt revision, backup schema, old-Node
  feature absence, and downgrade-marker contract tests.

## Out of scope

- real managed workload Apply/Recover, generation readiness, crash budget, or
  executor integration (AH-02);
- network publication, ingress, Renew, Drain, withdrawal, connection drain, or
  carrier failure (AH-03);
- complete Operator recovery/runbook, transactional full-system
  backup/restore execution, archive GC operations, migration rollout, or
  downgrade implementation (AH-04);
- deployment/security/release qualification (AH-05);
- arbitrary endpoint or existing-process registration;
- caller-supplied image, command, environment, runtime, resources, endpoint,
  port, probe, service type, certificate, secret, policy, mount, device, or
  Docker option;
- Delegation;
- Application-selected HTTPS;
- a Direct Service `Do`, `Dial`, proxy, tunnel, credential, or retry adapter;
- changing ADR-0012 status or promoting Q.

## Dependencies

Hard gates:

1. explicit maintainer acceptance of ADR-0012;
2. AD-01 completed and integrated, providing the closed protected-procedure
   registry, rule-owned resource finalization, owner-shape validation, and
   shared Application errors.

Fixed inputs:

- ADR-0001 and ADR-0002;
- current identity/session/access admission;
- current workload/hosting/publication contracts as downstream ports only;
- accepted Application Discovery locator and DR-05 endpoint-hiding boundaries.

Not dependencies for this local slice:

- ADR-0013 or real DR-04 three-host evidence;
- ADR-0014;
- Channel Grant authority, Messaging, or multi-host implementation.

## External contract

The Application supplies only:

- canonical Hosted Service name;
- logical profile ID;
- protocol `http` or `tcp` allowed by the pinned revision;
- exposure `LocalOnly`;
- admitted lease duration;
- RequestID.

Ensure has no caller-selected service-type field. Status safely projects the
service type pinned by the exact archived profile revision.

Required Application actions/resources for this slice:

```text
application.hosting.ensure
  on hosting-profile:<profile-id>

application.hosting.read
  on hosted-service:<owner>/<service-id>
```

The aggregate schema should remain compatible with later
`application.hosting.renew`, `.recover`, `.drain`, and `.publish.network`, but
those behaviors are not exposed by AH-01.

## Authority and identity

- Actor is the authenticated Application Principal and becomes the durable
  owner.
- Effective must equal Actor; any presented Delegation is `Forbidden`.
- Ensure authorizes the exact logical profile resource and therefore the
  Operator-controlled current revision, including its pinned service type.
- Get/List are owner-filtered; another owner's ID, absent ID, expired hidden
  record, or unauthorized record is privacy-uniform `NotFound`.
- A Node-wide Hosting grant may be supported deliberately, but exact profile
  scope is preferred and must not authorize a sibling profile accidentally.
- The fake derived workload identity has no Principal and cannot acquire
  Application or Operator authority.
- The Node Principal is not used to lend authority to the aggregate.
- Access Grant revocation blocks later calls but does not substitute for lease
  expiry or explicit Hosted Service/profile-revision revocation.
- Audit records stable Actor/owner/action/resource/request/outcome facts while
  excluding raw profile Plan, endpoints, runtime details, secrets, and policy
  internals.

## Authoritative state

One versioned Hosting consistency group owns:

- owner-qualified Hosted Service aggregates;
- bounded mutation journal;
- logical profile current and disabled pointers;
- canonical immutable profile-revision bytes and digests;
- aggregate references `(profile_id, profile_digest)`;
- pinned safe service-type projection;
- absolute lease timestamps, desired state, revision, bounded RequestID
  history, fake derived identity, and stable outcome;
- schema/downgrade marker.

The archived canonical revision bytes are required state; a digest alone is
not sufficient. Every read recomputes the digest and verifies schema and
service type.

For AH-01 the fake runtime snapshot is a derived test projection. It is not
authoritative desired truth and must be replaceable by the real RuntimePort in
AH-02 without changing the public aggregate.

## Required finite bounds

- canonical name, profile ID, and RequestID: at most 128 bytes each;
- one Hosted Service maps to exactly one derived workload and one service;
- at most 256 logical profiles with active current revisions per Node;
- at most 1,024 retained current-or-archived revisions per Node;
- lease default 15 minutes, minimum 1 minute, maximum 60 minutes;
- at most eight live Hosted Services per Application Principal;
- at most 32 retained terminal records per Principal and 1,024 per Node;
- List returns at most eight owner-visible services, active first and then
  newest terminal records;
- terminal tombstones are retained for 24 hours;
- an unreferenced revision remains retained for an additional 24 hours;
- one mutation in flight per service;
- stale expected revision returns `Conflict`;
- latest eight RequestID outcomes retained per aggregate;
- per-Principal mutation bucket: one per second, burst eight;
- Node-wide mutation bucket: 32 per second, burst 64;
- archive and tombstone pressure reject new Ensure; referenced state is never
  evicted to make room.

All time, quota, list, journal, history, and archive tests use deterministic
fake clocks/stores.

## Failure, restart, and recovery

- Mutation acknowledgement occurs only after authoritative intent/result is
  durable.
- Same RequestID plus the same declaration returns the durable prior result;
  replay cannot extend the lease.
- Same name with a different declaration conflicts.
- Stale revision conflicts with current safe status.
- Store/journal failure before effects preserves prior truth and returns
  retryable `Unavailable` for the same RequestID.
- Lease expiry is terminal; it cannot be renewed or recovered by replay.
- Profile disable/logical delete blocks new Ensure but existing aggregates
  remain pinned and readable.
- Explicit Hosted Service/profile-revision revoke is terminal.
- Startup loads and validates the complete Hosting consistency group before
  reconstructing any fake projection.
- Active unexpired state may return a conservative pending/local status after
  restart but never a fabricated ready or published claim.
- Expired/revoked truth wins over fake runtime state.
- Missing revision, digest mismatch, projected-type mismatch, invalid
  canonical bytes, or unknown revision schema fails Hosting closed and never
  substitutes the profile's new current revision.
- Clock rollback greater than 30 seconds cannot extend authority and produces a
  failed/withdrawn-safe state requiring Operator clock repair and later
  explicit recovery in AH-02/AH-04.
- The backup schema must be self-contained and require every referenced
  revision; full backup/restore execution is completed in AH-04.
- Old Nodes report Hosting unavailable.
- Old binaries must refuse downgrade when any Hosting marker, aggregate,
  journal, profile pointer, or archived revision exists.

## Privacy and observability

- Public status may contain opaque service ID, safe name, pinned service type,
  protocol, exposure, stable state, ready/published booleans, lease expiry,
  revision, and a stable safe reason.
- AH-01 `LocalOnly` must never claim Published.
- Public status/errors exclude workload ID, image, command, endpoint, port,
  container, runtime, policy, profile Plan, trust detail, and secrets.
- Metrics use only operation, stable result/state, and exposure labels.
- Metrics/logs must not label Principal, service ID/name, profile, RequestID,
  endpoint, image, port, or container.
- List and `NotFound` behavior must not reveal another owner's aggregate or
  profile catalogue.

## Acceptance criteria

- [ ] AH-01 does not begin before ADR-0012 is explicitly accepted and AD-01 is
  integrated.
- [ ] Operator profile creation produces canonical immutable bytes, one digest,
  one pinned service type, and an atomic current pointer.
- [ ] Ensure accepts only the closed local declaration and rejects every
  caller-supplied runtime/endpoint/service-type field and unknown field.
- [ ] Exact profile grant succeeds; missing action, sibling profile scope,
  network-only action, and Delegation fail before mutation.
- [ ] Actor equals Effective and becomes the owner.
- [ ] Aggregate and journal become durable before acknowledgement.
- [ ] Get returns the same owner-safe pinned status after restart.
- [ ] List is owner-filtered, deterministically ordered, and capped at eight.
- [ ] Another Principal cannot observe, replay, mutate, or infer the service.
- [ ] Profile update does not change an existing aggregate's Plan or service
  type.
- [ ] Profile disable/logical delete blocks new Ensure while leaving the exact
  archived revision reconstructable.
- [ ] Explicit revision/Hosted Service revoke is distinguishable from
  disable/delete and terminal.
- [ ] Missing/corrupt/mismatched/unknown revision fails closed and is never
  replaced by the current revision.
- [ ] Lease, expiry, clock rollback, stale revision, duplicate name,
  RequestID replay, rate, quota, list, tombstone, archive, and GC eligibility
  bounds pass deterministic tests.
- [ ] Archive cap never evicts referenced state.
- [ ] Backup schema cannot omit a referenced revision.
- [ ] Old-Node feature absence and downgrade-marker refusal contracts pass.
- [ ] No real workload, ingress, publication, HTTPS, or Direct Service surface
  is exposed.

## Required tests and evidence

### Unit

- aggregate state transitions and terminal states;
- canonical profile bytes/digest/type pinning;
- profile update/disable/delete/revoke distinctions;
- lease default/min/max, expiry, clock rollback, and no resurrection;
- revision and RequestID idempotency/conflict;
- archive reference graph, 24-hour tombstone, extra 24-hour revision retention,
  deterministic GC eligibility, and 1,024-revision cap;
- per-owner/global quotas, rate buckets, List ordering/cap;
- corruption, missing revision, digest/type mismatch, unknown schema;
- startup ordering and conservative post-restart status;
- stable safe status/error projection and metric label bounds.

### Contract

- real protected Application handler and SDK Ensure/Get/List;
- exact action/resource, owner, and no-Delegation matrix;
- Ensure has no service-type or arbitrary runtime material;
- unknown fields and malformed bounds fail before admission/domain mutation;
- owner-filtered privacy and typed error parity;
- old-Node feature negotiation/absence;
- shared AD-01 registry composition remains closed and duplicate-safe.

### Integration tracer

- in-memory or local durable store plus fake RuntimePort;
- Ensure one approved LocalOnly service through the public SDK;
- restart the Hosting controller/Node composition;
- Get the same service ID, pinned profile digest/type, lease, revision, and safe
  status;
- change/disable/delete the logical profile and prove the existing aggregate
  still reconstructs the old archived revision;
- corrupt/remove the pinned revision and prove fail-closed behavior.

### Commands and retained evidence

- targeted tests for new Hosting aggregate/profile/store/controller packages;
- `go test ./internal/applicationapi/... ./internal/hosting/... ./sdk/go/... -count=1`;
- relevant deterministic store/restart contract tests;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- documentation contract and architecture acceptance runners;
- `git diff --check`.

Retain exact source commit, commands, environment, timestamps, outcomes, and
JSON/JUnit where supported. Local/fake-runtime success is not Docker,
deployment, security, E2E, or release qualification.

## Capability impact and no-Q rule

- Before implementation:
  `application.hosting = I=no/R=no/O=no/Q=no`.
- AH-01 may support an evidence-reviewed `I=partial` or local contract claim,
  but cannot by itself establish complete reachability or operability.
- It does not change `hosting.operator-publication`,
  `workload.lifecycle`, `application.discovery`, or
  `service.direct-interaction`.
- Capability catalogue changes, if any, must describe only evidence actually
  present at the exact implementation commit.
- `Q=yes` is forbidden. AH-05 plus the DR-04 topology requirements and the
  DR-06 matching-commit release gate own qualification.

## Expected files and modules

Expected change surface:

- `api/ardents/application/v1` additive Hosting protocol;
- generated `internal/applicationapi/protocol/applicationv1` and
  `sdk/go/protocol/applicationv1` artifacts;
- `internal/applicationapi/hosting` handler, procedure rules, resource
  canonicalization, mapping, and contract tests;
- `internal/applicationapi/admission` registry composition only to register
  the new rules through the AD-01 seam;
- `internal/hosting` aggregate, profile catalogue/archive, journal,
  controller/store, status projection, and fake RuntimePort tracer;
- daemon/Application listener composition and startup ordering;
- `sdk/go/hosting`, `sdk/go/internal/adapter`, and `sdk/go/client`;
- versioned persistence/backup schema contracts and focused fixtures;
- architecture and documentation contracts for the new Application service.

Later-slice modules that must not be materially implemented here:

- real `internal/workload` execution/readiness integration;
- `internal/ingressproxy`;
- `internal/publication`;
- network topology/deployment tooling;
- Direct Service client code.

## Blocked by

- explicit maintainer acceptance of
  `docs/adr/0012-lease-application-hosting-through-approved-profiles.md`;
- PW3-04 / AD-01 completed and integrated.

Change this issue to `Status: ready-for-agent` only after both blockers are
closed without changing the external Hosting contract.

## Exit condition

AH-01 exits when an authorized Application can Ensure one approved LocalOnly
service and retrieve the same owner-safe, profile-pinned durable status after
restart through the public SDK; authority, privacy, idempotency, expiry,
archive, corruption, bounds, old-Node, and downgrade contracts pass; no real
workload/network behavior is claimed; and capability truth contains no
premature reachability, operability, or qualification claim.

## Comments

- This issue follows the source packet name and boundary: AH-01 is the durable
  local tracer. AH-02 owns the real managed workload/readiness integration.
