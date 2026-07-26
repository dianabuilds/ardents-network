# PW3-10: AD-04 Exact Grants and Delegation

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R1 bounded authority implementation

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/application-discovery.md`, slice
`AD-04 — Exact grants and Delegation`.

## User story

As an Application or delegated effective Principal using Discovery Resolve, I
want Node and exact `service-type` authority to intersect through the existing
one-hop Delegation model so that no grant or Delegation can resolve a sibling
service type or another Node.

## Full vertical behavior

Deepen the accepted AD-03 implementation without changing its wire or
projection contract:

1. prove a current Node-scoped Application grant admits
   `application.discovery.resolve`;
2. prove an exact ownerless `service-type(service_type)` grant admits only the
   matching service type;
3. preserve `Actor` as the authenticated Application and set `Effective` to
   the Delegator only for a valid one-hop Delegation;
4. require the Application grant, effective Principal grant, Delegation
   actions, Delegation audience, and Node or exact Delegation scope all to
   intersect;
5. reject sibling-service authority as `Forbidden`, reject structurally
   cross-Node exact scope as `InvalidArgument`, and reject wrong-Node
   authentication or Delegation as `Unauthenticated`, always before invoking
   the locator or revealing catalogue state;
6. keep the successful response independent of `Effective`;
7. add public Operator and Go SDK examples for issuing and presenting the
   existing action and exact resource scope.

## In scope

- existing Application admission/action/resource contracts;
- existing Access Grant and one-hop Delegation artifacts;
- exact ownerless `service-type` scope matching;
- Node and exact scope intersections;
- Actor/Effective and audit attribution regression evidence;
- real admitted Discovery RPC positive and negative matrices;
- SDK Delegation presentation with Discovery Resolve;
- Operator and SDK documentation/examples for the existing action.

## Out of scope

- new actions, resource kinds, wire messages, RPCs, or SDK methods;
- Delegation chains, re-delegation, bearer authority, roles, or wildcards;
- owner-qualified Discovery resources or response personalization;
- AD-05 lifecycle, E2E qualification, or any `Q=yes` change;
- changes to discovery projection, endpoint policy, trust, persistence, cache,
  queue, or background work;
- endpoint dialing, TLS/service authentication, Hosting, Messaging, or Direct
  Service adapters.

## Dependencies

- AD-01 protected Application admission seam.
- AD-02 end-to-end Discovery tracer.
- AD-03 projection/privacy/abuse checkpoint
  `9b2076019d964089cfa34258fb5bbe858f6c76f3`.
- Existing Principal Access Grant, resource-scope, one-hop Delegation, session,
  audit, and SDK artifact contracts.

## Authority and identity

- Action remains exactly `application.discovery.resolve`.
- Resource remains ownerless `service-type(request.service_type)`.
- Accepted schemes remain execution narrowing only and are not authority.
- A direct call has `Actor == Effective == authenticated Application`.
- A delegated call has `Actor == authenticated Application` and
  `Effective == Delegator`.
- Application, Effective, and Delegation authority all intersect; none can
  widen another.
- Missing/mismatched authority is `Forbidden`; invalid authentication or
  Delegation presentation is `Unauthenticated`; projection outcomes retain the
  AD-03 privacy contract.

## State ownership and bounds

- Existing `identity/access` state remains authoritative for grants,
  Delegations, credentials, sessions, revocations, and audit attribution.
- Application Discovery owns no authority state.
- Existing artifact, action-count, lifetime, header, unary-message, and
  one-hop bounds remain unchanged.
- No request-triggered external work is added.

## Failure, restart, and recovery

- Restart reloads the same identity/access state; AD-04 adds no replay,
  migration, compensation, persistence, or cache.
- Revoked, expired, wrong-Node, wrong-Application, wrong-action, or
  wrong-resource authority fails closed through the existing typed errors.
- A Delegation never changes the projected response and cannot turn
  `Forbidden` into `NotFound`.

## Confirmed TDD seams

- real `identity/access.AdmitTarget`;
- admitted `DiscoveryService.Resolve`;
- SDK `Discovery.Resolve` with the existing Delegation interceptor;
- public Operator/SDK examples for issuing
  `application.discovery.resolve`.

## Acceptance criteria

- [x] Node-scoped direct grant admits Discovery Resolve.
- [x] Exact ownerless `service-type` grant admits only its matching type.
- [x] Sibling service authority fails `Forbidden`, structurally cross-Node
  exact scope fails `InvalidArgument`, and wrong-Node authentication or
  Delegation fails `Unauthenticated`, all before locator.
- [x] Direct calls retain `Actor == Effective == Application`.
- [x] Delegated calls retain Application `Actor` and Delegator `Effective`.
- [x] Application, Effective, Delegation action, audience, Node, and exact
  scope intersections are independently covered.
- [x] Delegation cannot widen either Principal's current grant.
- [x] Successful response is identical for direct and delegated authority.
- [x] Audit attribution retains both Actor and Effective without secrets.
- [x] SDK presents at most one canonical existing Delegation and preserves
  typed Discovery errors.
- [x] Operator and SDK examples use the existing action and exact ownerless
  resource scope.
- [x] AD-03 projection/privacy behavior and Content compatibility remain
  unchanged.
- [x] No new wire/action/resource and no AD-05 qualification change is
  included.

## Required tests and evidence

- exact Node/exact-service direct grant positive matrix;
- cross-service and cross-Node direct grant negatives;
- valid Node/exact Delegation positive matrix;
- wrong action, wrong Node, wrong delegatee, expired/revoked, and sibling
  service Delegation negatives;
- Actor/Effective and audit attribution assertions;
- locator call-count zero for authority failures;
- direct/delegated response equality;
- SDK Delegation presentation and typed-error regression;
- Content/admission compatibility;
- focused tests for changed packages, scoped `go vet`,
  `scripts/generate-api.ps1 -Check`, tooling and capability catalogue checks,
  `git diff --check`, and full `go test ./... -count=1`.

Use an external task-specific `GOCACHE`. Retain exact red/green commands,
source commit, environment, start/end time, result, and any canonical
artifacts. Local success is implementation evidence, not qualification.

## Capability impact and no-Q rule

- AD-04 deepens authority evidence for `application.discovery`.
- `application.discovery` remains `Q=no`.
- Qualification and catalogue promotion remain AD-05-only.

## Exit condition

AD-04 exits when exact grants and one-hop Delegation are explicit through the
real admitted Discovery interface, all authority intersections fail closed,
Actor/Effective attribution is retained, public examples exist, independent
Standards and Spec reviews have no actionable findings, and the clean
checkpoint is ready for AD-05. It does not exit with qualification.

## Comments

- Tracker created from the canonical research packet after accepted AD-03.
- Baseline:
  `9b2076019d964089cfa34258fb5bbe858f6c76f3`, branch
  `codex/ad-04-exact-grants-delegation`, clean worktree before this tracker
  addition.
- All Go commands used the external task-specific cache
  `C:\Users\vitek\AppData\Local\Temp\ardents-ad04-gocache`; no Go cache was
  created in the repository.
- TDD evidence:
  - exact Application grant CLI red:
    `go test ./internal/cli/identity -run '^TestGrantIssueSupportsExactApplicationDiscoveryGrant$' -count=1`,
    exit 1 (`expected code 0, got 2`);
  - exact Application grant domain red:
    `go test ./internal/identity/access -run '^TestIssueAccessGrantDerivesApplicationAudienceFromDiscoveryAction$' -count=1`,
    exit 1 (`identity artifact is invalid`);
  - the final focused green command
    `go test ./internal/identity/access ./internal/cli/identity ./internal/applicationapi/discovery ./sdk/go/client ./sdk/go/internal/adapter ./tests/testkit -count=1`
    passed `2026-07-26T16:04:42.8283479Z..16:04:52.3369711Z`;
  - principal-owned administrative issuance red:
    `go test ./internal/identity/access -run '^TestOperatorIssuanceRejectsApplicationPrincipalOwnedScope$' -count=1`,
    `2026-07-26T15:56:00.8577611Z..15:56:03.5052137Z`, exit 1;
    green `2026-07-26T15:56:20.4740107Z..15:56:22.8347491Z`, exit 0;
  - cross-Node exact pre-signing validation red:
    `go test ./internal/identity/access -run '^TestGrantProposalRejectsCrossNodeExactScopeBeforeSigning$' -count=1`,
    `2026-07-26T16:03:02.4160874Z..16:03:05.1552771Z`, exit 1;
    green `2026-07-26T16:04:34.5945491Z..16:04:36.3680830Z`, exit 0.
- Final repository gates:
  - scoped `go vet` passed
    `2026-07-26T16:12:41.2121074Z..16:12:48.0655396Z`;
  - `powershell -NoProfile -File scripts/generate-api.ps1 -Check` passed
    `2026-07-26T16:12:41.0155585Z..16:12:46.3909548Z`;
  - `go test ./tests/tooling/... -count=1` and
    `go run ./tests/tooling/capabilitycatalog -check` passed
    `2026-07-26T16:12:41.4073786Z..16:12:51.6976398Z`, reporting
    `24 capabilities, 8 domains, 0 qualified`;
  - full `go test ./... -count=1` passed
    `2026-07-26T16:12:58.9146979Z..16:13:56.2502316Z`;
  - `git diff --check` passed.
- Canonical final APP-001 passed
  `2026-07-26T16:14:13.4106652Z..16:15:20.9073410Z`; scenario execution was
  `2026-07-26T16:14:29.779566841Z..16:15:17.919613808Z`. Final artifacts:
  `tests/.artifacts/reports/application-process-ad04-final/summary.json`
  SHA-256 `588FAFF1938577D8CAC2CB1EA924705D4C5CA11D32E46099C50D7CD28F3B5E84`;
  `junit.xml`
  `A50EB355CB28E3C18B458D28AACE89E8F87F7880EF0D2AA9D048F3199DE929B5`;
  raw JSON
  `AEB49A24C61F23C6521E2B074D5C3309E8E3A3FA5B8CF49B47553F8335F5EE96`;
  before/after resource snapshots
  `6BE38534D50302A90138945C949844C2AE271A10B12FC005653BA8127B812467`
  and
  `3D75776ED4E81710CF39648FE2EC5E1F650892F2F7DFD0223E9883CF4C4B1595`.
  The canonical runner removed its temporary test binary.
- Focused security audit found no confirmed exploitable vulnerability.
  Remediation closed principal-owned administrative scope and cross-Node
  exact pre-signing validation defects. Independent post-remediation security
  revalidation found no security-relevant finding. Scoped `govulncheck` found
  no reachable vulnerability
  (`2026-07-26T16:06:15.6944267Z..16:06:18.6248192Z`).
- Independent Standards and Spec reviews of the full remediated diff each
  reported zero actionable findings. No wire, action, resource, generated
  contract, persistence, cache, queue, background work, dial, TLS, Hosting,
  Messaging, Direct Service, or qualification change was introduced.
- AD-04 adds no authority state. Existing `identity/access` durable grants,
  Delegations, credentials, sessions, revocations, and audit state remain
  authoritative and reload under the existing restart semantics. Application
  Discovery remains a bounded projection consumer and owns no authority
  lifecycle. This is implementation evidence only:
  `application.discovery` remains `Q=no`; lifecycle and qualification remain
  AD-05.
