# PW3-04: AD-01 Deepen Protected Application Admission

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R1 bounded implementation

## Parent

`../PRD.md`

## User story

As an Application module author, I want protected Application admission to use
a closed composition-time procedure registry so that Content keeps its exact
behavior and a later ownerless Discovery resource can be admitted without
adding service-specific switches or reverse dependencies to the admission
layer.

## Full vertical behavior

Extract one behavior-preserving admission seam through the real protected
Application request path:

1. move shared Application error construction out of Content ownership without
   changing protobuf names, field numbers, SDK error mapping, or wire behavior;
2. introduce a closed `ProcedureRule` registry supplied at composition;
3. make resource resolution, finalization, target-error mapping, and mutating
   classification rule-owned;
4. validate owner-required and ownerless resource shapes through registered
   resource contracts in the sealed call channel;
5. register all existing Content procedures through the new registry;
6. fail composition on duplicate procedures, invalid rules, or incomplete
   Content registration;
7. prove existing Content admission, authorization, audit, protocol, SDK, and
   Application-process behavior are unchanged.

AD-01 must be testable without adding any Discovery type, action, procedure,
wire message, handler, or SDK surface.

## In scope

- shared Application error ownership and unchanged error compatibility;
- a closed protected-procedure registry;
- composition-time validation and duplicate rejection;
- rule-owned resource resolution/finalization and target error mapping;
- sealed-call validation of both owner-required and registered ownerless
  resource contracts;
- registration of existing Content procedures;
- unchanged direct and delegated Content admission;
- unchanged mutating audit behavior;
- unit, contract, architecture, and existing Application-process regression
  evidence.

## Out of scope

- `application.discovery.resolve`, `service-type`, Discovery wire messages,
  locator, handler, SDK, endpoint projection, trust/policy filtering, or
  Discovery documentation;
- Hosting procedures, profiles, aggregates, or persistence;
- changes to Content authorization semantics or public behavior;
- a dynamically extensible runtime registry;
- remote Application transport or non-Go SDKs;
- capability promotion or release qualification.

## Dependencies

- ADR-0001 separate Application and Operator interfaces.
- ADR-0002 Principal-centered identity, exact Access Grants, and one-hop
  Delegation.
- Existing protected Application Identity and Content composition.
- Existing sealed Application call channel and Content access catalogue.
- The accepted `application-discovery.md` design defines why an ownerless
  resource shape will be needed later, but AD-01 has no dependency on
  ADR-0011 through ADR-0015.

## Authority and identity

- AD-01 adds no action, resource, grant, Principal, Credential, Session, or
  Delegation behavior.
- Existing Content resources remain owner-required and must finalize to
  `ResourceOwner == Effective`.
- The channel may accept an ownerless shape only when the registered resource
  contract explicitly requires an empty owner.
- Unknown resource kinds, unknown procedures, duplicate procedure
  registrations, and invalid rule shapes fail closed.
- Session authentication, Actor/Effective derivation, one-hop Delegation
  intersection, access invocation, sealed-call injection, and audit
  attribution remain admission-owned.
- Content continues to own its procedure rules and resource canonicalization;
  admission must not import Content-specific access catalogues after the seam.

## State ownership

- AD-01 introduces no durable state, cache, cursor, journal, queue, or new
  backup material.
- The registry is immutable composition-time state.
- Existing Identity/Session/Access/Delegation stores remain authoritative.
- Existing Content stores remain authoritative for Content.
- The sealed call remains request-scoped and cannot be persisted or reused.

## Bounds and abuse controls

- Registry size is bounded by the closed set of composed procedures.
- Lookup is exact by canonical protected procedure name.
- Duplicate or invalid rules fail daemon composition; later registration or
  caller-controlled procedure insertion is not supported.
- Existing Application request/body, field, resource-ID, session, Delegation,
  and audit bounds remain unchanged.
- Error refactoring must not reveal internal procedure names, resources,
  Principal IDs, grant contents, or target parsing detail.
- Unknown fields and malformed requests continue to fail before domain
  behavior.

## Failure, restart, and recovery

- A duplicate, invalid, or incomplete registry prevents the Application
  service from starting; it must not start partially protected.
- A missing procedure returns the existing stable public error and never
  injects a sealed call.
- Resource resolution/finalization failure must not invoke Content or emit a
  successful mutation audit event.
- Daemon restart reconstructs the same closed registry from code/configured
  composition; there is no state to replay or migrate.
- Registry extraction must not change Content retry, session refresh,
  idempotency, or recovery behavior.
- An old SDK continues to work because public wire names, field numbers, and
  error semantics remain compatible.

## Acceptance criteria

- [x] A closed registry supplies complete procedure rules at
  composition.
- [x] Duplicate procedures, nil/invalid functions, invalid action/mutation
  combinations, and missing required Content procedures fail closed.
- [x] Admission no longer imports Content-specific action/resource/error
  catalogues.
- [x] Existing Content procedures are registered through the new seam.
- [x] Owner-required Content resources still require owner exactly equal to
  Effective Principal.
- [x] A test-only registered ownerless resource requires an empty owner and
  injects correctly.
- [x] Unknown resource kinds and owner-shape mismatches are rejected before
  sealed-call injection.
- [x] Direct and delegated Content calls preserve Actor/Effective attribution
  and grant intersection.
- [x] Mutating Content success/denial audit semantics are unchanged; reads do
  not become successful-mutation audit events.
- [x] Shared Application error declarations preserve fully-qualified protobuf
  names and field numbers.
- [x] SDK typed errors and the one-time session-refresh rule are unchanged.
- [x] Existing protected-socket Application Identity/Content journey passes
  without Discovery fixtures.
- [x] Operator and Application handlers/packages remain separate.
- [x] No Discovery public type, procedure, action, resource, handler, or SDK
  symbol is introduced.

## Required tests and evidence

### Targeted unit and contract

- registry valid/duplicate/unknown/incomplete/invalid-rule matrix;
- rule-owned resolver/finalizer/error mapping matrix;
- owner-required and ownerless sealed-call shape tests;
- direct and delegated Content admission regression;
- authentication, authorization, malformed target, unknown field, and domain
  error mapping parity;
- mutating/read audit parity;
- public protobuf/error compatibility tests;
- SDK typed-error and session-refresh regression.

### Integration and architecture

- existing protected Application service composition starts with the complete
  Content registry;
- deliberately incomplete or duplicate composition fails before listener
  readiness;
- existing Application-process Identity/Content scenario remains unchanged;
- dependency/architecture acceptance proves admission is product-service
  neutral and Application/Operator surfaces remain separate.

### Commands and retained evidence

- `go test ./internal/applicationapi/... -count=1`;
- `go test ./sdk/go/... -count=1`;
- the canonical Application-process regression runner when available;
- `go test ./tests/tooling/... -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`;
- documentation contract and architecture acceptance runners;
- `git diff --check`.

Retain command, environment, source commit, start/end time, outcome, and
generated JSON/JUnit where the canonical runner provides it. Local success is
implementation evidence, not qualification.

## Capability impact and no-Q rule

- AD-01 is behavior-preserving infrastructure and does not implement
  `application.discovery`.
- `application.discovery` remains `I=no/R=no/O=no/Q=no`.
- `application.installation-content` must retain its current I/R/O values and
  remains `Q=no`.
- No capability catalogue status changes belong in this slice unless a
  behavior-preservation regression requires correcting a false claim.
- `Q=yes` is forbidden; only AD-05 plus the DR-06 matching-commit gate may
  support qualification.

## Expected files and modules

Expected change surface:

- `internal/applicationapi/admission`;
- `internal/applicationapi/call`;
- `internal/applicationapi/content` procedure rules and access catalogue;
- `internal/applicationapi/binding` or daemon Application composition;
- `api/ardents/application/v1/content.proto` and a common Application protocol
  file only if required to relocate shared errors compatibly;
- generated internal and `sdk/go/protocol/applicationv1` artifacts;
- `sdk/go/internal/adapter`, `sdk/go/errors`, and `sdk/go/client` only for
  compatibility-preserving error plumbing/tests;
- existing Application contract, architecture, and process fixtures.

Not expected:

- `internal/discovery`;
- a Discovery proto/package;
- `internal/hosting`, workload, ingress, or publication modules;
- capability or release qualification snapshots.

## Blocked by

None beyond the accepted repository baseline and fixed ADR-0001/ADR-0002
contracts.

## Exit condition

AD-01 exits when the closed registry and owner-shape seam are exercised through
the real protected Application composition, all existing Content behavior is
proven unchanged, invalid composition fails closed, no Discovery surface has
been introduced, and the reviewed implementation commit is ready for AD-02 and
AH-01 to consume.

## Comments

- This issue follows the source packet name and boundary: AD-01 is the
  protected Application admission seam.
- 2026-07-26 implementation evidence from
  `main@4b866abf4efe8b5c06376092baf485bd387106f3`:
  - the sealed, composition-time registry is complete against the generated
    `ContentService` descriptor and rejects duplicate, unknown, incomplete,
    nil, action/classification-mismatched, resource-kind-mismatched, and
    owner-contract-mismatched registrations;
  - Content Put/Get own their resolver, finalizer, target-error mapping, and
    mutation classification; direct/delegated regression tests preserve
    Actor/Effective, grant intersection, and mutation/read audit behavior;
  - a test-only ownerless `node` rule traverses real Application session
    admission and sealed-call injection with an empty owner; a deliberately
    owner-violating finalizer is rejected before handler dispatch;
  - shared `ardents.application.v1.ErrorCode` and
    `ardents.application.v1.ApplicationError` declarations retain their fully
    qualified names and field numbers; SDK typed-error mapping and the existing
    single-refresh tests pass;
  - `go test ./internal/applicationapi/... -count=1`, `go test ./sdk/go/...
    -count=1`, `go test ./tests/tooling/... -count=1`, `go run
    ./tests/tooling/capabilitycatalog -check`, `go test ./cmd/ardentsd
    -count=1`, `go vet ./internal/applicationapi/... ./sdk/go/...
    ./cmd/ardentsd`, `scripts/generate-api.ps1 -Check`, and `git diff --check`
    passed using external `GOCACHE=C:\Users\vitek\AppData\Local\Temp\ardents-go-build-cache`;
  - `go test -v -tags=e2e ./tests/e2e/applicationapi -count=1` completed with
    PASS but skipped `TestApplicationUsesDedicatedPrincipalInterface` because
    the Windows environment has no Unix domain socket runner. Docker daemon
    and WSL were unavailable, so the protected-process acceptance box remains
    unchecked for Linux/CI confirmation;
  - capability catalogue remains 24 capabilities, 8 domains, 0 qualified.
    No Discovery, Hosting, capability-status, or Q changes were made.
- 2026-07-26 remediation review and implementation:
  - replaced the one-implementation admission `Registry` interface with one
    concrete immutable registry;
  - reduced the composition contract to an independently derived required
    procedure-name set plus one product-owned rule per procedure;
  - added immutable Application action contracts so composition rejects both
    `application.content.get` marked mutating and
    `application.content.put` marked non-mutating even when the supplied rule
    is internally self-consistent;
  - the new regression cases failed before the fix and pass afterward;
  - Application API, SDK, daemon, Identity contract, vet, tooling,
    capability-catalogue, API-generation, and whitespace checks pass using an
    external task-specific `GOCACHE`;
  - the first canonical APP-001 attempts were blocked because the Docker
    Desktop Linux engine pipe did not exist. Docker Desktop was subsequently
    started and the real Linux-container scenario exposed a functional
    regression at grant revocation: the Operator grant-list preflight filtered
    out the Application-audience grant and returned `active Access Grant was
    not found`;
  - exact outside-sandbox blocker evidence: source
    `d02f8cac7bf6fd1f0a41a4d24c51e00672939833`, start
    `2026-07-26T14:37:53.2463659+03:00`, end
    `2026-07-26T14:37:56.2222350+03:00`, Windows
    `10.0.26200.0`/`windows-amd64`, PowerShell `5.1.26100.8875`, Docker client
    `29.1.3` with context `desktop-linux`; `docker version` and
    `.\ardents.ps1 test e2e -Domain application-interface -Scenario APP-001
    -ReportDir tests/.artifacts/reports/application-process` failed because
    `//./pipe/dockerDesktopLinuxEngine` was not found. The runner exited `1`
    at container inspection before selecting or executing APP-001, so no
    Application-process JSON/JUnit report was generated; the retained
    pre-run environment snapshot is
    `tests/.artifacts/resources/20260726-113753-42228-before.json`;
  - root cause was fixed in
    `e2c55d8becc7aa89179dac9bf09ec91c4e71c5b6`: the Operator list remains
    subject- and Node-scoped but now returns grants for both registered
    interfaces with each grant's actual Audience. The regression test
    `TestOperatorCanListApplicationGrantForRevocation` failed with zero
    results before the fix and passes afterward;
  - commit-bound Application-process evidence: source
    `e2c55d8becc7aa89179dac9bf09ec91c4e71c5b6`,
    `.\ardents.ps1 test e2e -Domain application-interface -Scenario APP-001
    -ReportDir tests/.artifacts/reports/application-process` ran the real
    `linux-container` scenario from `2026-07-26T11:57:23.161063669Z` through
    `2026-07-26T11:58:12.873544073Z` and passed all seven precondition, step,
    and assertion records without a skip. Retained evidence:
    `tests/.artifacts/reports/application-process/summary.json`,
    `tests/.artifacts/reports/application-process/junit.xml`,
    `tests/.artifacts/reports/application-process/raw/TestApplicationUsesDedicatedPrincipalInterface-1785067092875457668.json`,
    `tests/.artifacts/resources/20260726-115710-33952-before.json`, and
    `tests/.artifacts/resources/20260726-115710-33952-after.json`;
  - `go test ./... -count=1`, scoped Identity/Local API/CLI vet, and
    `git diff --check` pass with the external task-specific Go cache;
  - capability truth remains 24 capabilities, 8 domains, 0 qualified, and no
    Discovery or Hosting surface was added.
