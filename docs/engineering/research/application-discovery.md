# Application Discovery Research Packet

## Decision

- Owner: Application Interface / Discovery
- Date: 2026-07-24
- Research baseline: `main@7c0965c`
- Research class: R1 bounded investigation
- Outcome: ready for implementation after one behavior-preserving Application
  admission seam extraction

The first Application Discovery interface is a read-only, bounded service
locator. It resolves a service type from the Node's current local discovery
truth and returns only direct targets that are currently fresh, trusted,
policy-eligible and safe to project.

It is not the Operator `ResolveService` response with fields removed. The
Operator query is a diagnostic interface: it includes records, trust evidence,
rejected candidates, route scores and reasons, and it couples resolution to
Waku transport observation. Those are useful administrative facts but are not
a stable or privacy-safe Application contract.

## User Outcome

An authenticated local Application can ask its Node:

> Give me a bounded set of trusted endpoints for service type `echo` that use
> one of the direct schemes I support.

The Application receives typed SDK targets or a stable typed error. It does not
need Operator credentials and does not learn the discovery catalogue, rejected
records, trust configuration, workload identity, peer topology or policy
reason.

## In Scope

- one unary Application RPC and one Go SDK method;
- exact Application action and resource extraction;
- normal direct and one-hop delegated admission;
- current, trusted, `NetworkPublished` service records;
- direct `http`, `https` and `tcp` endpoints;
- a caller-supplied, bounded accepted-scheme preference;
- a deterministic response containing at most eight targets;
- privacy-safe error projection;
- unit, contract, integration and Linux E2E evidence.

## Out Of Scope

- listing or watching the discovery catalogue;
- node, peer, route or trust diagnostics;
- record import or publication;
- `LocalOnly`, Unix, Waku, relay, multiaddr, QUIC or WebRTC endpoints;
- opening the returned connection;
- endpoint application authentication, TLS identity or request credentials;
- health probing on behalf of the Application;
- load balancing, sticky sessions or a connection pool;
- remote access to the Application Interface;
- cross-language SDKs;
- persistence or a new discovery cache.

Direct service authentication and connection behavior remain DR-05. Discovery
returns an eligible locator; it does not claim that a connection or an
application-level authentication exchange will succeed.

## Current Reachable Journey

The current Application journey ends at identity/session and Content
`Put/Get`. The Application protocol contains `IdentityService` and
`ContentService`; `client.Client` exposes only `Content` and `Session`.

The runtime already has the underlying truth:

- `internal/discovery.Service` retains signed records;
- `internal/discovery.TrustEvaluator` re-evaluates current trust;
- `internal/discovery.Resolver.ResolveService` finds fresh service records and
  applies route policy;
- Publication emits only readiness-backed `NetworkPublished` service records
  and withdrawals;
- the Operator Network service exposes the administrative resolution result.

An Application cannot reach any of that truth without an Operator session.

## Current Implementation And Evidence

### Existing domain behavior

`internal/discovery/resolution.FindService` selects non-expired,
non-withdrawn records by service type. `TrustEvaluator` verifies signed
authority and evaluates current purpose-scoped trust. `policy.AllowRouteUse`
can deny untrusted routes or schemes.

Publication already validates local network advertisements before signing:

- runtime backing and current generation are ready;
- advertised and probe endpoints are paired;
- the direct scheme is `http`, `https` or `tcp`;
- credentials, fragments, ambiguous ports and invalid address scopes are
  rejected;
- `NetworkPublished` depends on current network ingress capability;
- withdrawal replaces an unavailable publication with an empty endpoint
  record.

### Existing evidence

- discovery record, trust, merge and resolution unit tests;
- policy route-use unit and integration tests;
- workload/publication integration tests;
- tagged discovery and workload E2E scenarios;
- Application admission and Content contract tests with real access fixtures;
- an Application process E2E journey for enrollment, session and Content.

These tests establish reusable behavior. They do not qualify an Application
Discovery surface because that surface does not yet exist.

## Missing Behavior And Architectural Blockers

### 1. Application admission is Content-specific

`internal/applicationapi/admission` imports the Content access catalogue,
Content resource canonicalizer and Content protocol errors directly. A second
protected product service would otherwise add another switch and another
reverse dependency to this package.

The required seam is a closed protected-procedure registry supplied at
composition:

```go
type ProcedureRule struct {
    Action       string
    Mutating     bool
    Resolve      func(any) (ResourceTarget, error)
    Finalize     ResourceFinalizer
    MapTargetErr func(error) error
}

type Registry interface {
    Lookup(procedure string) (ProcedureRule, bool)
}
```

Content and Discovery own their procedure rules. Admission owns session and
Delegation presentation, access invocation, audit integration and injection of
the sealed call. Duplicate procedures and invalid rules fail composition.

### 2. The sealed Application call assumes every resource has an owner

`internal/applicationapi/call.validPrincipal` currently requires
`ResourceOwner == Effective`. That is correct for Content but rejects an
ownerless Node resource.

The channel must validate owner shape through the registered identity resource
contract:

- owner-required resource: owner is exactly `Effective`;
- ownerless resource: owner is empty;
- unknown resource kind: reject injection.

This preserves the sealed-call boundary while supporting both owned and
Node-scoped Application modules.

### 3. The administrative resolver is not an Application locator

The current resolver:

- returns all matching records and trust details;
- retains untrusted matches for diagnostics;
- may expose a selected untrusted or unusable candidate;
- treats Waku endpoint observation as route usability even when the service
  endpoint is direct HTTP/HTTPS/TCP;
- invokes workload observation synchronously before each service query.

Application calls must not trigger process/container observation work and must
not use Waku bootstrap reachability as proof that a direct service endpoint is
reachable.

### 4. Received record endpoint syntax is deliberately broad

The signed record validator accepts bounded printable endpoint strings. The
local publication gate is stricter, but a remote signed record can still carry
a shape that should not be handed directly to an SDK caller.

The locator therefore needs its own fail-closed projection eligibility:

- record mode is exactly `NetworkPublished`;
- URL has no user information or fragment;
- scheme is `http`, `https` or `tcp`;
- port is explicit and valid;
- host is a literal, non-unspecified, non-loopback address;
- scheme is accepted by the query;
- current trust and route policy allow the candidate.

Private and link-local literal addresses remain eligible because
`private_lan` is a supported publication scope and the publisher is explicitly
trusted. DNS and public-name identity are deferred until DR-05 defines the
authentication and name-validation profile.

Unsafe, unknown or unsupported endpoints remain visible only to the Operator
diagnostic surface.

## Proposed External Interface

### Go SDK

```go
package discovery

import "context"

type Scheme string

const (
    SchemeHTTPS Scheme = "https"
    SchemeHTTP  Scheme = "http"
    SchemeTCP   Scheme = "tcp"
)

type Query struct {
    ServiceType    string
    AcceptedSchemes []Scheme
}

type Target struct {
    ServiceID string
    Endpoint  string
    Scheme    Scheme
}

type Service interface {
    Resolve(context.Context, Query) ([]Target, error)
}
```

`client.Client` gains `Discovery discovery.Service`. SDK domain types do not
alias generated protobuf types.

`AcceptedSchemes` is mandatory, ordered by caller preference, unique and
limited to the three v1 direct schemes. This avoids silently selecting a
transport the embedding Application cannot use. A future additive helper may
construct common preferences without changing the method.

### Wire

```proto
service DiscoveryService {
  rpc Resolve(ResolveServiceRequest) returns (ResolveServiceResponse);
}

message ResolveServiceRequest {
  string service_type = 1;
  repeated string accepted_schemes = 2;
}

message ResolvedServiceTarget {
  string service_id = 1;
  string endpoint = 2;
  string scheme = 3;
}

message ResolveServiceResponse {
  repeated ResolvedServiceTarget targets = 1;
}
```

The response contains one to eight targets. It contains no source, record,
signature, public key, Node Principal, workload ID, trust state, candidate
score, policy reason or route reason.

### Resolution Semantics

1. Validate the complete request and unknown fields before admission.
2. Admit `application.discovery.resolve` for exact
   `service-type(request.service_type)`.
3. Read the already-maintained discovery snapshot; do not synchronously refresh
   workloads, the network or remote discovery.
4. Select fresh, non-withdrawn `NetworkPublished` records of the requested
   type.
5. Re-evaluate current trust.
6. Project only endpoint candidates that pass endpoint eligibility, current
   route policy and the caller's scheme set.
7. Sort by caller scheme order, then `service_id`, then endpoint bytes.
8. Deduplicate exact `(service_id, endpoint)` pairs and return the first eight.
9. If no target remains, return the uniform public `NotFound` result.

The deterministic ordering is a response contract, not a load-balancing
promise. Applications may try returned targets in order and re-resolve after a
bounded connection failure.

## Authority, Identity And Delegation

The new exact action is:

```text
application.discovery.resolve
```

The new resource contract is:

```text
service-type: non-empty canonical ID, owner not required
```

The service type is validated as a bounded printable canonical identifier.
Accepted schemes narrow execution but do not change authority, so they are not
part of the resource ID.

Application enrollment can issue the action with Node scope. An Operator can
later issue an exact `service-type` grant when an installation should resolve
only one service type.

Delegation follows the existing one-hop intersection without a discovery
exception:

- `Actor` is the authenticated Application;
- `Effective` is the Delegator when a valid Delegation is presented;
- both current grant sets, the Delegation action set and its Node or exact
  `service-type` scope must match;
- the resource remains ownerless and the response is not personalized by
  `Effective`;
- audit attribution retains both Principals.

A Delegation cannot widen the Application's own discovery grant. Callers that
do not need delegated authority omit it.

## Privacy, Security And Abuse Analysis

### Information disclosure

`NotFound` deliberately conflates:

- no record;
- expired or withdrawn record;
- untrusted publisher;
- unsupported or unsafe endpoint;
- denied route policy;
- no endpoint matching the requested schemes.

Only a missing action or mismatched grant is `Forbidden`. Authentication
failures remain `Unauthenticated`. This prevents an authorized-but-ineligible
query from enumerating trust or policy configuration.

### Endpoint safety

The Node does not connect to the returned endpoint, so this RPC is not itself
an SSRF primitive. The projection still rejects credentials, loopback,
unspecified hosts, unknown schemes, DNS and malformed ports so a trusted record
cannot turn the SDK response into an implicit local-host locator.

Trust remains security-relevant: private and link-local endpoints are returned
only for records signed by a currently trusted Node and allowed by current
policy.

### Resource bounds

- unary request and response use the existing Application message bound;
- service type uses the identity canonical resource ID bound;
- exactly one to three unique accepted schemes;
- at most eight response targets;
- no request-triggered workload observation, network probe or remote fetch;
- no streaming or server-side cursor;
- endpoint parsing is linear in already-bounded record fields.

An authenticated caller can still enumerate guessed service types. Exact
grants are the authorization control for installations that must not have
Node-wide discovery.

## Failure, Retry, Restart And Recovery

| Condition | Public result | Retryable |
|---|---|---:|
| malformed service type, scheme set or unknown field | `InvalidArgument` | no |
| missing/invalid Application session | `Unauthenticated` | no SDK retry beyond the existing one session refresh |
| action, grant or Delegation mismatch | `Forbidden` | no |
| no eligible target for any privacy-sensitive reason | `NotFound` | no automatic retry |
| discovery/trust store unavailable | `Unavailable` | yes |
| internal invariant violation | `Internal` | no |

The SDK retains its current rule: refresh a session once only after
`Unauthenticated`; never turn `Forbidden` or `NotFound` into authentication
retries.

The operation has no write to compensate. After restart, the Node loads the
existing discovery snapshot and re-evaluates freshness and current trust.
Withdrawal, trust change and policy reload affect the next call without an SDK
migration.

The wire addition is additive. Existing Content clients and grants remain
valid. Existing Applications do not receive the new action automatically.
Moving the shared `ErrorCode` and `ApplicationError` declarations from
`content.proto` into a common Application protocol file must preserve their
fully-qualified protobuf names and field numbers; it is a source organization
change, not a wire change.

## Dependency Classification

### In-process

- identity/access admission and resource contracts;
- Application sealed-call channel;
- discovery store, freshness and trust evaluator;
- route policy;
- endpoint eligibility and bounded projection;
- Application protocol handler and SDK adapter.

### Local-substitutable

- generated Connect handler/client;
- protected Unix Application listener;
- an in-memory trusted-record fixture for contract tests.

### Remote but owned

- signed service records delivered asynchronously through Ardents private
  discovery;
- Publication withdrawal and refresh convergence.

The Resolve call never performs a synchronous remote operation.

### True external

- the service process behind a returned endpoint;
- realm/operator provisioning that decides which Node Principals are trusted.

Neither is called while resolving.

## Alternatives

### A. Expose the Operator Network service to Applications

Rejected. It crosses the listener, session, action and generated-package
boundary and exposes administrative topology.

### B. Reuse `Resolver.ResolveService` and remove fields in the handler

Rejected. It keeps request-triggered workload observation, diagnostic
untrusted candidates and Waku route-usability semantics inside the public
path. The handler would become responsible for security filtering.

### C. Return one selected target

Rejected for the first interface. A deterministic single target makes an
unreachable first instance a permanent result and forces load-balancing state
into Discovery. A bounded target set enables caller failover without exposing
records or arbitrary catalogue size.

### D. Return every matching record

Rejected. It leaks catalogue size and topology, makes SDK consumers implement
trust and policy rules, and creates an unbounded response.

### E. Resolve and connect through the Node

Deferred to DR-05. It requires connection ownership, TLS/application
authentication, retry, backpressure and protocol-specific error semantics.

## Observability And Operator Actions

- structural/authentication/authorization denial continues through the
  identity access audit path;
- the read does not emit a successful-mutation audit event;
- policy denial may remain Operator-visible but its internal reason is never
  returned to the Application;
- metrics use operation and stable outcome labels only, never service type,
  endpoint, Principal or record ID;
- Operator discovery commands remain the place to inspect why a target was
  absent.

Operator recovery is:

1. inspect discovery status and the exact service record;
2. inspect trust and route policy;
3. inspect publisher readiness/publication and withdrawal state;
4. correct trust, policy, reachability or service lifecycle;
5. let normal refresh converge, then repeat the Application call.

## Acceptance Matrix

| Level | Required evidence |
|---|---|
| Unit: admission seam | Content behavior is unchanged; duplicate/unknown procedures fail closed; owner-required and ownerless resources inject correctly; mutating audit remains Content-only |
| Unit: locator | filters expired, withdrawn, untrusted, wrong-mode, unsafe, unsupported and policy-denied endpoints; honors scheme order; deduplicates; sorts deterministically; caps at eight |
| Unit: side effects | Resolve never calls workload observation, a remote fetch or a network probe |
| Contract: protocol | valid admitted request returns only the three public target fields; unknown fields and malformed bounds fail before the locator |
| Contract: authority | direct Node grant works; missing action fails; exact service-type grant cannot resolve a sibling type |
| Contract: Delegation | Actor/Effective attribution is retained; both grant sets and exact scope intersect; response is not owner-personalized |
| Contract: privacy | absent, untrusted, unsafe, policy-denied and scheme-mismatched cases are externally indistinguishable `NotFound` |
| SDK | domain types do not expose protobuf; typed errors and one-time session refresh match Content behavior |
| Integration | trusted local and imported records resolve; untrusted records do not; withdrawal and trust/policy changes affect the next call |
| Linux E2E | Operator ticket includes the discovery action; Application enrolls, authenticates and resolves through the protected socket; Operator credentials never reach the Application |
| Architecture | generated service is composed once on the Application listener; Operator and Application handlers/packages remain separate |

Canonical Docker and Linux E2E evidence is required before the feature can be
called qualified. Unit and local contract success only make it
`locally_verified`.

## Issue Slices And Dependency Order

### AD-01 — Deepen protected Application admission

Behavior-preserving prerequisite:

- move shared Application error construction out of Content;
- introduce the closed procedure registry;
- make resource finalization rule-owned;
- allow the sealed call channel to validate registered ownerless resources;
- register existing Content rules through the new seam;
- prove Content protocol, admission, audit and E2E fixtures are unchanged.

Exit: no Discovery type or procedure is needed to test the seam, and Content
has no observable behavior change.

### AD-02 — Resolve a trusted service end to end

First product tracer bullet:

- add `application.discovery.resolve` and `service-type`;
- add the minimal wire messages and handler;
- add the narrow locator over current discovery truth;
- add SDK `discovery.Service` and `Client.Discovery`;
- resolve one trusted `NetworkPublished` endpoint through a real admitted
  Application contract test.

Exit: valid happy path and stable typed failures exist through the same public
interface a caller uses.

### AD-03 — Close projection, privacy and abuse boundaries

- accepted-scheme validation and preference;
- strict direct endpoint eligibility;
- trust and policy filtering;
- deterministic deduplication and eight-target cap;
- uniform `NotFound`;
- negative matrices for malformed, untrusted, withdrawn and unsafe records;
- prove no request-triggered refresh/probe/fetch.

Exit: no administrative discovery fact crosses the public response or error.

### AD-04 — Exact grants and Delegation

- exact `service-type` grants;
- Node and exact Delegation scopes;
- Actor/Effective audit attribution;
- cross-service and cross-Node negative tests;
- operator and SDK examples for issuing the new action.

Exit: authority behavior is explicit and independently regression-tested.

### AD-05 — Lifecycle and qualification evidence

- trusted imported-record integration;
- withdrawal, trust reload and policy reload convergence;
- protected-socket Linux E2E;
- API generation and architecture acceptance updates;
- capability/evidence catalogue update;
- current-head clean release evidence for this slice.

Exit: Application Discovery is operable and qualified for the selected release
environment.

## Recommendation

Implement AD-01 through AD-04 as bounded work. Keep AD-05 as the qualification
gate rather than treating it as cleanup.

Do not combine this work with Messaging, Hosting, remote Application transport
or direct service authentication. Application Discovery is a small deep module
only if trust, endpoint eligibility, policy filtering and record projection
remain behind its locator interface.
