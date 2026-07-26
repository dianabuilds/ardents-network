# PW3-09: AD-03 Close Projection, Privacy, and Abuse Boundaries

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R1 bounded security implementation

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/application-discovery.md`, slice
`AD-03 — Close projection, privacy and abuse boundaries`.

## User story

As an authenticated Application using Discovery Resolve, I want every returned
target and public failure to obey one deterministic, privacy-safe, bounded
projection contract so that untrusted, unsafe, withdrawn, policy-denied, or
unsupported discovery truth cannot cross the Application Interface.

## Full vertical behavior

Deepen the accepted AD-02 tracer without changing its public action, resource,
or SDK method:

1. validate the complete accepted-scheme set as one to three unique v1 direct
   schemes in caller preference order;
2. accept only fresh, non-withdrawn, currently trusted
   `NetworkPublished` records;
3. project only direct `http`, `https`, and `tcp` endpoints with no userinfo or
   fragment, an explicit valid port, and a literal non-unspecified,
   non-loopback host;
4. apply current route policy after trust and structural endpoint eligibility;
5. reject DNS names, Unix, Waku, relay, multiaddr, QUIC, WebRTC, credentials,
   malformed ports, unsupported schemes, and unsafe address scopes;
6. sort by caller scheme order, then service ID, then endpoint bytes;
7. deduplicate exact `(service_id, endpoint)` pairs and return at most eight
   targets;
8. return one uniform public `NotFound` for every authorized-but-ineligible
   outcome;
9. prove Resolve never triggers workload observation, discovery/network
   refresh, remote fetch, health probe, dial, or other synchronous external
   work.

AD-03 changes projection and privacy behavior only. AD-04 owns exact grants,
Delegation, and cross-service/cross-Node authority matrices. AD-05 owns
lifecycle integration and qualification.

## In scope

- the AD-02 Application Discovery locator, handler, SDK adapter, and focused
  tests;
- complete query scheme validation and preference ordering;
- strict direct endpoint parsing and projection eligibility;
- current trust and route-policy filtering;
- deterministic sort, exact deduplication, and eight-target cap;
- privacy-uniform `NotFound`;
- negative matrices for absent, expired, withdrawn, wrong-mode, untrusted,
  unsafe, unsupported, policy-denied, and scheme-mismatched records;
- explicit side-effect spies proving maintained-truth reads only;
- response validation in the SDK adapter;
- architecture, documentation, Content, and capability no-Q regression.

## Out of scope

- new Application actions, resource kinds, procedures, or wire messages;
- exact `service-type` grant and Delegation matrices;
- record listing, watching, import, publication, refresh, or mutation;
- endpoint dialing, health checks, connection pooling, retries, TLS identity,
  service authentication, or credentials;
- DNS/public-name identity, TOFU, certificate pinning, or endpoint secrets;
- Hosting, Messaging, Direct Service adapters, remote Application transport,
  or non-Go SDKs;
- persistence, caches, cursors, queues, background workers, or migrations;
- APP Discovery qualification or any `Q=yes` change.

## Dependencies

- AD-01 accepted protected Application admission seam.
- AD-02 accepted end-to-end Discovery tracer at
  `baec800a283edfe7976fdcd3008adec94ed6099a`, with reviewed remediation
  `04241ab`.
- Existing discovery record, freshness, withdrawal, and trust owners.
- Existing route-policy contracts.
- Canonical `application-discovery.md` projection and privacy contract.

## Authority and identity

- Keep `application.discovery.resolve` and ownerless
  `service-type(request.service_type)` unchanged.
- Authentication and admission happen before locator invocation.
- Accepted schemes narrow execution but are not authority and never become
  part of the resource ID.
- Missing action/grant remains `Forbidden`; malformed requests remain
  `InvalidArgument`.
- Every authorized query for which no eligible target remains returns the same
  `NotFound`, without revealing catalogue presence, trust, policy, withdrawal,
  scheme, or endpoint rejection reason.
- Do not expose Principal IDs, Node IDs, publisher keys, signatures, trust
  evidence, records, workloads, topology, route scores, or policy reasons.

## State ownership

- Existing discovery storage remains authoritative for records, freshness, and
  withdrawals.
- Existing trust and policy owners remain authoritative for current
  eligibility.
- Application Discovery owns only bounded projection semantics.
- Resolve remains read-only and introduces no durable or memory-resident
  authoritative state.

## Bounds and abuse controls

- service type remains a bounded canonical resource ID;
- accepted schemes are mandatory, unique, and limited to one through three;
- only the closed v1 set `https`, `http`, and `tcp` is accepted;
- endpoint parsing is linear in already-bounded record fields;
- return at most eight targets after deterministic sort and exact
  deduplication;
- use existing unary request/response byte limits;
- no request-triggered external work or unbounded scan/result;
- metrics/log labels contain only operation and stable outcome, never query,
  endpoint, service/record ID, Principal, trust, or policy detail.

## Failure, restart, and recovery

- malformed service type, schemes, or unknown fields: `InvalidArgument`;
- invalid/missing session: `Unauthenticated`;
- action/grant mismatch: `Forbidden`;
- every authorized ineligible/absent outcome: identical non-retryable
  `NotFound`;
- truth/trust/policy store unavailable: retryable `Unavailable`;
- internal invariant failure: non-retryable `Internal`;
- SDK refreshes once only after `Unauthenticated`, never after `Forbidden` or
  `NotFound`;
- restart reads the same maintained discovery truth and current trust/policy;
  there is no AD-03 state to replay, migrate, or compensate.

## Acceptance criteria

- [ ] Accepted schemes are validated as one to three unique v1 direct schemes
  and preserve caller preference.
- [ ] Only fresh, non-withdrawn, trusted `NetworkPublished` records are
  eligible.
- [ ] Only credential-free, fragment-free direct endpoints with an explicit
  port and literal non-unspecified, non-loopback IP are projected.
- [ ] Private and link-local literal addresses remain eligible only when
  current trust and route policy allow them.
- [ ] DNS, Unix, Waku, relay, multiaddr, QUIC, WebRTC, malformed, loopback,
  unspecified, credential-bearing, and unsupported endpoints fail closed.
- [ ] Current route policy is applied without leaking its denial reason.
- [ ] Results sort by scheme preference, service ID, and endpoint bytes.
- [ ] Exact `(service_id, endpoint)` duplicates are removed and the response is
  capped at eight.
- [ ] Absent, expired, withdrawn, wrong-mode, untrusted, unsafe,
  policy-denied, and scheme-mismatched cases are externally identical
  `NotFound`.
- [ ] Resolve performs no observation, refresh, probe, fetch, dial, or other
  synchronous external side effect.
- [ ] SDK response validation rejects malformed, duplicate, unsorted,
  unsupported, or over-cap server responses as `Internal`.
- [ ] Existing AD-02 happy path, Content behavior, and APP-001 remain
  unchanged.
- [ ] No exact-grant/Delegation expansion and no capability qualification is
  included.

## Required tests and evidence

### Locator and privacy

- table-driven eligibility matrix for record lifecycle, mode, trust, scheme,
  URL components, host class, explicit port, and route policy;
- caller scheme-order, service-ID, endpoint-byte ordering;
- duplicate suppression before the eight-target cap;
- uniform public `NotFound` matrix with no private reason/detail;
- unavailable/internal classification without leaking underlying errors;
- spies proving zero observation, refresh, probe, fetch, dial, and network
  side-effect calls.

### Contract and SDK

- malformed and unknown-field rejection before locator invocation;
- public three-field response only;
- response count/order/duplicate/scheme/endpoint validation;
- typed errors and single-session-refresh regression;
- real admitted Application contract happy path plus privacy negatives;
- Content protocol/admission compatibility.

### Architecture and repository

- focused tests for all changed packages;
- scoped `go vet`;
- `scripts/generate-api.ps1 -Check`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- `git diff --check`;
- full `go test ./... -count=1` before the implementation checkpoint;
- APP-001 through the canonical Docker runner when available.

Use an external task-specific `GOCACHE`. Retain exact commands, source commit,
environment, start/end time, result, and canonical artifacts. A retry never
replaces the first result. Local success is implementation evidence, not
qualification.

## Capability impact and no-Q rule

- AD-03 deepens implementation evidence for `application.discovery`.
- `application.discovery` remains `Q=no`.
- No existing capability status changes unless a proven product-truth
  regression requires a separate correction.
- Qualification and catalogue promotion remain AD-05-only.

## Expected files and modules

- `internal/applicationapi/discovery` locator/projection and tests;
- narrow consumer-owned trust/policy seams only where needed;
- `sdk/go/internal/adapter` response validation and tests;
- focused Application contract/security tests;
- architecture/documentation acceptance only when behavior or ownership
  actually changes.

Avoid protocol regeneration unless the accepted AD-02 wire contract itself is
proven defective; such a defect is a blocker requiring integrator review, not
silent scope expansion.

## Exit condition

AD-03 exits when every eligible target is deterministically and safely
projected, all ineligible cases collapse to privacy-uniform `NotFound`, the
response is bounded to eight, request-triggered external work is disproven by
tests, SDK validation matches the server contract, independent Standards and
Spec reviews have no actionable findings, and the clean checkpoint is ready
for AD-04. It does not exit with exact-grant/Delegation or qualification work.

## Comments

- Published after accepted AD-02 implementation and remediation on
  2026-07-26.
- Baseline at publication: `main@04241ab`, clean worktree before this planning
  change.
- The implementing agent must re-check HEAD, branch, origin, worktree, and
  retained APP-001 evidence before editing.
