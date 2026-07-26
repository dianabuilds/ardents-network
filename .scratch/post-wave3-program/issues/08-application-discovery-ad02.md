# PW3-08: AD-02 Resolve a Trusted Service End to End

Status: ready-for-agent
State: open
Labels: ready-for-agent
Research class: R1 bounded implementation

## Parent

`../PRD.md`

## Canonical source

`../../../docs/engineering/research/application-discovery.md`, slice
`AD-02 — Resolve a trusted service end to end`.

## User story

As an authenticated local Application, I want to resolve one trusted
`NetworkPublished` service through the protected Application Interface and the
Go SDK so that the public Discovery journey exists end to end without exposing
Operator diagnostics or opening the returned connection.

## Full vertical behavior

Implement the first narrow Application Discovery tracer bullet:

1. add the exact Application action `application.discovery.resolve`;
2. add the ownerless `service-type` identity resource contract;
3. add the additive `DiscoveryService.Resolve` wire request, bounded public
   target response, generated handlers, and clients;
4. register the procedure through the accepted AD-01 composition-time
   admission registry;
5. add an Application-owned locator seam over already-maintained discovery
   truth;
6. resolve one fresh, trusted, non-withdrawn `NetworkPublished` direct endpoint
   without triggering observation, refresh, probing, fetching, or dialing;
7. expose SDK-owned `discovery.Query`, `discovery.Target`,
   `discovery.Service`, and `Client.Discovery`;
8. prove the valid direct Application path and stable typed failure mapping
   through the same public interface used by callers.

This slice creates the real public vertical path. AD-03 remains responsible
for the complete projection, privacy, scheme-preference, deterministic
deduplication/cap, and adversarial negative matrices.

## In scope

- one unary protected Application Discovery RPC;
- exact action and ownerless resource extraction;
- complete composition through the AD-01 registry and Application listener;
- a narrow locator interface owned by the Application Discovery module;
- current discovery snapshot, freshness, withdrawal, and trust truth needed
  for one valid trusted tracer;
- minimal public target projection containing only service ID, endpoint, and
  scheme;
- Go SDK domain types and client integration;
- direct Application admission with a Node-scoped grant;
- stable typed `InvalidArgument`, `Unauthenticated`, `Forbidden`, `NotFound`,
  `Unavailable`, and `Internal` mapping at the public seam;
- focused unit, contract, SDK, architecture, and real admitted Application
  contract evidence.

## Out of scope

- the full AD-03 unsafe-endpoint, policy-denial, privacy-uniformity,
  scheme-order, deduplication, eight-target, and side-effect negative matrix;
- exact `service-type` grant and Delegation matrices owned by AD-04;
- protected-socket Linux qualification, lifecycle convergence, capability
  promotion, or release evidence owned by AD-05;
- Operator discovery response reuse or changes to Operator commands;
- listing, watching, importing, publishing, or refreshing discovery records;
- workload observation, network probing, remote fetch, endpoint dialing,
  health checks, TLS or service authentication, credentials, retries, load
  balancing, or connection pooling;
- Hosting, Messaging, direct-service adapters, remote Application transport,
  cross-language SDKs, or new persistence.

## Dependencies

- AD-01 accepted at
  `e2c55d8becc7aa89179dac9bf09ec91c4e71c5b6`.
- ADR-0001 separate Application and Operator interfaces.
- ADR-0002 Principal-centered identity and exact Access Grants.
- Current signed discovery record, freshness, withdrawal, and trust owners.
- The canonical `application-discovery.md` contract.
- No Proposed Wave 3 ADR is treated as accepted or required by this slice.

## Authority and identity

- `application.discovery.resolve` is an Application action only.
- The admitted resource is exact `service-type(request.service_type)` with an
  empty `ResourceOwner`.
- Authentication establishes the Application Principal but grants no
  authority; a current matching Application Access Grant remains mandatory.
- This slice proves the direct Node-scoped grant path. It must not special-case
  or weaken the existing Actor/Effective/Delegation machinery; the exhaustive
  exact-scope and Delegation matrix belongs to AD-04.
- Operator credentials, sessions, handlers, diagnostic results, trust reasons,
  route reasons, record signatures, Principal IDs, workload IDs, and topology
  facts never cross the Application response.

## State ownership

- Existing discovery storage remains authoritative for signed records,
  freshness, withdrawals, and trust evaluation.
- The new Application locator owns projection behavior, not discovery truth.
- Resolve is read-only and introduces no database, cache, cursor, journal,
  queue, persisted history, background observer, or recovery record.
- SDK domain types own the public caller model and do not alias generated
  protobuf messages.

## Bounds and abuse controls

- use the existing Application unary message bound;
- validate service type as a bounded canonical identity resource ID;
- retain the final wire contract for one to three unique accepted direct
  schemes and at most eight public targets, while AD-03 supplies the exhaustive
  preference/cap negative matrix;
- return only service ID, endpoint, and scheme;
- keep endpoint parsing linear in already-bounded record fields;
- perform no request-triggered observation, probe, refresh, fetch, dial, or
  streaming work;
- never label metrics or logs with service type, endpoint, record ID, or
  Principal ID.

## Failure, restart, and recovery

- malformed request or unknown fields fail before domain behavior;
- missing/invalid session is `Unauthenticated`;
- action or grant mismatch is `Forbidden`;
- no valid tracer target is `NotFound`;
- authoritative store/trust unavailability is `Unavailable`;
- invariant failure is `Internal`;
- the SDK keeps the existing single refresh only after `Unauthenticated`;
- restart reconstructs Application composition and reads the existing
  discovery snapshot; Resolve has no write to replay or compensate;
- Content clients, grants, protobuf names, and error semantics remain
  compatible.

## Acceptance criteria

- [ ] `application.discovery.resolve` and ownerless `service-type` are
  registered in the canonical Application identity contracts.
- [ ] Additive Discovery protobuf messages and service generate cleanly.
- [ ] The procedure is registered through AD-01 without a Discovery switch or
  reverse dependency in admission.
- [ ] A real admitted Application call resolves one fresh trusted
  `NetworkPublished` direct endpoint.
- [ ] The public response contains only service ID, endpoint, and scheme.
- [ ] Resolve reads maintained discovery truth and performs no refresh, probe,
  fetch, observation, or dial.
- [ ] Missing action/grant fails before locator invocation.
- [ ] Unknown fields and malformed service type/scheme input fail before
  locator behavior.
- [ ] Stable typed public errors are preserved through generated protocol,
  adapter, SDK, and one-time session refresh behavior.
- [ ] SDK domain types do not expose or alias protobuf messages.
- [ ] Existing Content Application behavior and APP-001 remain unchanged.
- [ ] Operator and Application Discovery packages, handlers, and response
  models remain separate.
- [ ] No capability is promoted and no `Q=yes` claim is made.

## Required tests and evidence

### Targeted

- identity action/resource contract tests;
- procedure registration and incomplete/duplicate composition tests;
- locator happy path against a trusted current `NetworkPublished` fixture;
- no-locator-call authorization denial test;
- malformed/unknown-field contract tests;
- handler public-field and typed-error mapping tests;
- SDK domain/adapter/client and one-refresh regression;
- Content admission and protocol compatibility regression.

### Integration and architecture

- generated API check;
- architecture acceptance proving no Operator/Application package collapse;
- documentation contract and import guard;
- real protected Application contract test using the existing access fixture;
- APP-001 regression when the canonical Docker runner is available.

### Commands

- focused tests for changed packages;
- `scripts/generate-api.ps1 -Check`;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- scoped `go vet` for changed packages;
- `git diff --check`;
- full `go test ./... -count=1` before the final implementation checkpoint.

Use an external task-specific `GOCACHE`. Retain exact command, environment,
source commit, start/end time, result, and generated JSON/JUnit for canonical
runners. Local success is implementation evidence, not qualification.

## Capability impact and no-Q rule

- This slice begins implementation of `application.discovery`; it may update
  implementation evidence only when the product behavior is actually present.
- `application.discovery` remains `Q=no`.
- Existing capability claims must not change unless a regression reveals
  incorrect canonical truth.
- Qualification and catalogue promotion belong only to AD-05 with complete
  matching-commit evidence.

## Expected files and modules

- `api/ardents/application/v1/discovery.proto`;
- generated Application protocol packages;
- `api/ardents/identity/v1/contract.go`;
- `internal/applicationapi/discovery`;
- Application binding/listener composition and admission registry inputs;
- narrow consumer-owned seams over `internal/discovery` and trust truth;
- `sdk/go/discovery`, `sdk/go/internal/adapter`, and `sdk/go/client`;
- focused contract, architecture, and Application tests.

Do not move Operator Network handlers or generated types into these packages.

## Exit condition

AD-02 exits when one trusted `NetworkPublished` endpoint resolves through the
real protected Application procedure and Go SDK, public failures are stable,
the locator performs no synchronous external work, existing Content behavior
remains compatible, the implementation has passed strict review, and the clean
checkpoint is ready for AD-03. It does not exit on unit-only locator success or
with any qualification claim.

## Comments

- Published after maintainer acceptance of AD-01 on 2026-07-26.
- Baseline at publication:
  `main@3638b2a13704707580f8abd0c1714de65baeef31`, clean worktree,
  `origin/main@2205bcc8542c16d4dc8abd95df970c546d5855ac`.
- The implementing agent must re-check HEAD, branch, origin, and worktree
  before editing and must preserve unrelated user changes.
