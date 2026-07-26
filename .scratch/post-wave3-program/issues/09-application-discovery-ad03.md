# PW3-09: AD-03 Close Projection, Privacy, and Abuse Boundaries

Status: ready-for-agent
State: closed
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

- [x] Accepted schemes are validated as one to three unique v1 direct schemes
  and preserve caller preference.
- [x] Only fresh, non-withdrawn, trusted `NetworkPublished` records are
  eligible.
- [x] Only credential-free, fragment-free direct endpoints with an explicit
  port and literal non-unspecified, non-loopback IP are projected.
- [x] Private and link-local literal addresses remain eligible only when
  current trust and route policy allow them.
- [x] DNS, Unix, Waku, relay, multiaddr, QUIC, WebRTC, malformed, loopback,
  unspecified, credential-bearing, and unsupported endpoints fail closed.
- [x] Current route policy is applied without leaking its denial reason.
- [x] Results sort by scheme preference, service ID, and endpoint bytes.
- [x] Exact `(service_id, endpoint)` duplicates are removed and the response is
  capped at eight.
- [x] Absent, expired, withdrawn, wrong-mode, untrusted, unsafe,
  policy-denied, and scheme-mismatched cases are externally identical
  `NotFound`.
- [x] Resolve performs no observation, refresh, probe, fetch, dial, or other
  synchronous external side effect.
- [x] SDK response validation rejects malformed, duplicate, unsorted,
  unsupported, or over-cap server responses as `Internal`.
- [x] Existing AD-02 happy path, Content behavior, and APP-001 remain
  unchanged.
- [x] No exact-grant/Delegation expansion and no capability qualification is
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
- Implementation began from the exact clean checkpoint
  `main@6d37f15c03679c397fbe5f5908a16006a50d99aa`, with
  `origin=https://github.com/dianabuilds/ardents-network.git`, on
  `codex/ad-03-projection-privacy`. All Go commands used the external
  task-specific cache
  `C:\Users\vitek\AppData\Local\Temp\ardents-ad03-gocache`.
- TDD evidence retained from the implementation:
  - endpoint eligibility red
    `go test ./internal/applicationapi/discovery -run TestLocatorProjectsOnlyEligibleDirectEndpoints -count=1`,
    `2026-07-26T14:05:23.7315677Z..14:06:38.6409176Z`, exit 1; green
    `2026-07-26T14:07:08.4909266Z..14:07:13.2300740Z`, exit 0;
  - deterministic ordering/dedup/cap red
    `TestLocatorOrdersDeduplicatesAndCapsTargets`,
    `2026-07-26T14:08:02.8888737Z..14:08:07.4791059Z`, exit 1; green with
    endpoint matrix `2026-07-26T14:08:51.7227984Z..14:08:56.6438423Z`,
    exit 0;
  - current route-policy filtering red
    `2026-07-26T14:10:09.1092134Z..14:10:13.8524561Z`, exit 1; green
    `2026-07-26T14:11:45.9398930Z..14:11:51.6406807Z`, exit 0;
  - strict SDK response validation red
    `2026-07-26T14:14:01.6770066Z..14:14:04.3430340Z`, exit 1; green
    `2026-07-26T14:14:52.8605090Z..14:14:55.0721289Z`, exit 0;
  - handler target-set invariant red
    `2026-07-26T14:15:27.5791056Z..14:15:32.5975975Z`, exit 1; green
    `2026-07-26T14:16:14.8886573Z..14:16:19.6470795Z`, exit 0;
  - bounded projection work red
    `go test ./internal/applicationapi/discovery -run TestLocatorRejectsProjectionWorkBeyondItsFixedBudget -count=1`,
    `2026-07-26T14:48:41.1616306Z..14:48:45.9845747Z`, exit 1; green
    `2026-07-26T14:49:30.8561029Z..14:49:37.0460448Z`, exit 0;
  - unknown JSON request rejection red
    `go test ./internal/applicationapi/discovery -run TestDiscoveryRejectsUnknownJSONFieldBeforeAdmissionOrLocator -count=1`,
    `2026-07-26T15:09:31.5037126Z..15:09:36.2060314Z`, exit 1; green
    `2026-07-26T15:11:30.1787454Z..15:11:35.7317017Z`, exit 0;
  - SDK lossy-JSON defense red
    `go test ./sdk/go/internal/adapter -run 'TestDiscoveryAdapterRejectsInvalidResponses/JSON_option' -count=1`,
    `2026-07-26T15:12:00.7200489Z..15:12:02.7699595Z`, exit 1; green
    `2026-07-26T15:12:22.1809742Z..15:12:24.3315107Z`, exit 0.
- Final focused changed-package command
  `go test ./internal/discovery/... ./internal/applicationapi/discovery ./sdk/go/internal/adapter ./sdk/go/discovery ./cmd/ardentsd ./internal/daemon -count=1`
  passed `2026-07-26T15:12:46.4287761Z..15:13:15.5804732Z`.
- Final repository gates:
  - scoped `go vet ./internal/applicationapi/discovery ./internal/discovery/... ./sdk/go/internal/adapter ./cmd/ardentsd ./internal/daemon`,
    `2026-07-26T15:18:25.3267786Z..15:18:26.8402882Z`, exit 0;
  - `powershell -NoProfile -File scripts/generate-api.ps1 -Check`,
    `2026-07-26T15:18:34.2831496Z..15:18:35.5779762Z`, exit 0;
  - `go test ./tests/tooling/... -count=1`,
    `2026-07-26T15:18:43.5755691Z..15:18:47.2209828Z`, exit 0;
  - `go run ./tests/tooling/capabilitycatalog -check`,
    `2026-07-26T15:18:52.6006359Z..15:18:53.8216148Z`, exit 0,
    `24 capabilities, 8 domains, 0 qualified`;
  - full `go test ./... -count=1`,
    `2026-07-26T15:19:03.0436043Z..15:19:53.8903864Z`, exit 0;
  - `git diff --check`, exit 0.
- Canonical APP-001 passed on the pre-review implementation candidate
  (`2026-07-26T14:30:17.4677359Z..14:31:22.0355924Z`) and again on the final
  remediated candidate
  (`2026-07-26T15:20:06.1862598Z..15:21:22.8861387Z`). Final artifacts:
  `tests/.artifacts/reports/application-process-ad03-final/summary.json`
  SHA-256 `E52CB7B5840750D81326D3BE4D135DB838B3034C06245292305C5EEDE54E1B23`;
  `junit.xml`
  `E57CAB7C6DE766B4262E770D5645CDE590AFEAF8D32CC16AA0BABBE124BFDCFC`;
  raw JSON
  `5BC9BAD808BDD03BA33F08A9B84D798CD5790168115FE8FE789C6DC6FC0CDDE4`;
  before/after resource snapshots
  `D25D85FB57D4F3106E09170EE13888DC23A53281F476F12790DFECE427B37155`
  and
  `BBAB387B1E2E7DF6247766D72852EC5867CA10F7DE8FD70AA1AC3753B7CB7834`.
  The canonical runner removed its temporary test binary.
- Independent reviews completed against the full uncommitted candidate.
  Initial Standards review found restart-incompatible retained-state caps and
  duplicated lifecycle filtering; initial Spec review additionally found the
  global intake cap out of AD-03 scope and lossy Connect JSON unknown-field
  decoding. Remediation removed every discovery intake/schema-v2 change,
  shared one lifecycle predicate, installed strict server JSON codecs, and
  pinned SDK Discovery to unknown-preserving protobuf. Repeat Standards and
  Spec reviews reported no actionable findings.
- Focused security review found one low-severity pre-projection work-amplification
  path. The bounded maintained-truth query now refuses request traversal above
  64 retained records and 256 matching endpoints, and locator preflight occurs
  before trust/policy work. Independent security revalidation reported no
  actionable finding or bypass. Scoped `govulncheck` passed with no reachable
  vulnerability (`2026-07-26T14:58:44.7827473Z..14:58:49.9611249Z`).
- AD-03 adds no projection-owned durable state. Restart continues to load the
  same maintained discovery truth and current trust/policy; oversized legacy
  truth loads unchanged and the Application projection fails closed without
  request-time scanning. This is implementation evidence only:
  `application.discovery` remains `Q=no`; qualification and exact
  grant/Delegation work remain AD-05 and AD-04 respectively.
