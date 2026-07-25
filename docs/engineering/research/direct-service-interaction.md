# DR-05: Direct Service Interaction

## Metadata

- Status: accepted research recommendation; ADR-0014 remains Proposed
- Research class: R2 deep research
- Decision owner: Application Interface / Discovery / Security
- Research owner: Wave 3 DR-05
- Date: 2026-07-25
- Frozen baseline commit:
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Preparation/integration input: accepted DR-02 research at
  `f6ac1b0337182889de3ce4d08e498e46ce1713a0`; ADR-0012 remains Proposed
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research: accepted DR-02 Application Hosting
- Downstream consumers: Application Discovery and Hosting implementation,
  deployment documentation, DR-06 qualification

## Answer first

Keep Ardents v1 discovery-only for direct services. The authenticated
Application calls the accepted bounded Discovery `Resolve` interface and then
uses its own HTTP or TCP implementation. Ardents authenticates and authorizes
the resolve call; it does not connect, proxy, mint a service credential,
translate an Access Grant, interpret an application response, or automatically
retry an application operation.

This gives an Application a privacy-filtered, currently trusted locator without
making the Node a generic data-plane intermediary. A Node-signed discovery
record authenticates the publisher and locator facts; it does not authenticate
the process behind the endpoint or authorize the caller to use that process.
Those are application-protocol responsibilities.

For `https`, the client must perform normal PKIX path and DNS-ID/IP-ID
verification against the URI host. Discovery signing keys, Node Principals,
Waku Peer IDs, service IDs, and leaf-certificate fingerprints are not TLS pins.
Application-owned Hosting remains `http`/`tcp` in v1: admitting `https` needs a
separate Operator-owned certificate, private-key delivery, and rotation
lifecycle, which neither the frozen product nor DR-02 safely provides.

## User outcome

An authorized Application can resolve a bounded set of currently eligible
service endpoints, connect with its chosen HTTP/TCP client, and distinguish an
Ardents resolution failure from a transport, TLS, or application-protocol
failure without receiving Operator authority or hidden credential material.

## Scope

### In scope

- the exact point at which Ardents direct-service responsibility ends;
- use of the accepted Application Discovery `Resolve` interface;
- relationship among Application Principal authentication, Discovery Access
  Grants, service authorization, Delegation, and application credentials;
- TLS server identity, reference identity, rotation, and pinning rules;
- connection, response, concurrency, deadline, retry, and error ownership;
- effect of Hosting lease, readiness, drain, withdrawal, and service type;
- behavior for existing Operator-published `http`, `https`, and `tcp` records;
- compatibility, observability, abuse bounds, and qualification.

### Out of scope

- an Ardents `Do`, `Dial`, `Invoke`, reverse-proxy, CONNECT, tunnel, gateway,
  service-token, mTLS-client-certificate, header-injection, or sidecar surface;
- arbitrary application method/path, codec, schema, streaming, idempotency,
  status, acknowledgement, ordering, or error interpretation;
- translating an Access Grant or Delegation into a credential accepted by a
  hosted workload;
- general load balancing, health probing, circuit breaking, service mesh,
  transparent traffic interception, DNS resolver, or certificate authority;
- Application-supplied certificate, private key, trust bundle, endpoint, port,
  image, or secret through Hosting;
- remote Application Interface, non-Go SDKs, Kubernetes, QUIC, WebTransport,
  WebRTC, and release qualification.

## Current product truth

### Supported interfaces

| Boundary | Frozen-baseline surface | Disposition |
|---|---|---|
| Operator | protected Operator Interface can inspect workload, hosted-service, publication, discovery, trust, route, and endpoint detail | reachable administrative surface |
| Application | protected local Identity and Content services; Go SDK exposes enrollment, session, Delegation, and Content | no Discovery or direct-service surface |
| Internal | signed discovery store/trust/resolver, publication manager, readiness controller, Docker ingress proxy, route policy | implemented, not an Application client |
| Deployment | Operator config supplies static services, endpoints, probe endpoints, allowed ingress hosts/bind, proxy image, trust roots for Node discovery, and WSS transport certificate files | Operator-owned |
| True external | DNS/PKI infrastructure, the published service process, its application protocol and credentials, and the connecting Application's HTTP/TCP stack | not owned by Ardents |

The accepted Application Discovery packet adds only a read-only locator. The
accepted DR-02 recommendation adds an owner-qualified leased Hosted Service but
explicitly hides its endpoint and leaves connection/authentication to this
packet. Neither feature exists at the frozen baseline; all corresponding
capabilities remain unqualified.

### Reachable journey

The actual frozen-baseline publication journey is:

```text
Operator config or WorkloadService mutation
  -> workload.Runtime / persisted execution desired state
  -> Docker or trusted-process executor
  -> generation-bound readiness observation
  -> publication policy + observed network reachability
  -> Node Principal signs service record
  -> local discovery truth
  -> private Waku discovery publication
  -> remote Node imports and verifies the record
  -> Operator-only resolver/status
```

`ServiceFacts` signs service ID, service type, Node Principal, workload ID,
mode, Node public key, endpoints, issuance, and expiry. Publication withdraws a
service with an empty endpoint record. The Docker ingress adapter admits at
most 16 paired endpoints, requires an explicit port, and for network exposure
requires an allowlisted literal non-loopback IP. The first-party ingress proxy
is a bounded raw TCP forwarder; it does not terminate TLS, authenticate a
Principal, authorize an application operation, or understand HTTP.

The current Application journey ends after an admitted Content operation. It
cannot resolve a service, obtain an endpoint, connect through the Node, or
present its Principal identity to a workload. The existing administrative
resolver also performs request-triggered workload observation and exposes
diagnostic trust/route detail, so it is not an Application adapter.

The selected future journey is:

```text
Application Principal
  -> protected local Application session
  -> exact application.discovery.resolve admission
  -> bounded trusted locator snapshot
  -> Target{service_id, endpoint, scheme}
  -- Ardents responsibility ends --
  -> Application-owned dial/TLS/application authentication
  -> service-owned authorization, protocol, response and state
```

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
| Applications have no direct-service interface | `api/ardents/application/v1`, `sdk/go/client` | generated Identity/Content contracts and SDK client fields | I=no, R=no |
| service publication is readiness/policy/reachability gated | `internal/publication/sync.go`, `internal/publication/plan.go` | publication plan/unit/integration tests | implemented internally; Operator-operable |
| discovery records bind publisher identity and locator facts | `internal/discovery/records/verify.go`, `internal/publication/local_facts.go` | signature/record/schema tests | implemented |
| withdrawal is an empty-endpoint signed replacement | `internal/publication/local_facts.go`, `internal/discovery/resolution/services.go` | publication/discovery tests | implemented |
| ingress is raw bounded TCP forwarding | `internal/ingressproxy/proxy.go`, `internal/ingressproxy/admission.go` | proxy and Docker ingress tests | implemented; no TLS/auth claim |
| Docker publication currently rejects DNS hosts | `internal/workload/docker/docker_ingress.go` | `TestDockerIngressAdmissionFailsClosed` | implemented |
| accepted Application locator returns only bounded safe targets | `docs/engineering/research/application-discovery.md` | research recommendation; no implementation evidence | I=no, R=no, O=no, Q=no |
| leased Application Hosting owns lifecycle, not connection | `docs/engineering/research/application-hosting.md`, Proposed ADR-0012 | accepted research recommendation | not implemented |

All production-code claims above were checked against the frozen baseline; the
current source tree has no source diff from that commit for `api`, `cmd`, or
`internal`. Existing tests are reusable evidence, not qualification of this new
surface.

### External standards evidence

All external sources are primary official standards or language documentation,
accessed 2026-07-25:

- [RFC 8446, TLS 1.3, August 2018](https://www.rfc-editor.org/rfc/rfc8446.html):
  TLS authenticates the server and only optionally the client, and leaves
  application-protocol use and certificate interpretation to the higher-level
  protocol.
- [RFC 9525, Service Identity in TLS, November 2023](https://www.rfc-editor.org/rfc/rfc9525.html):
  the client constructs a reference identity, DNS names use DNS-ID, literal IP
  addresses use exact IP-ID, and a mismatch fails; ad hoc pinning must not
  freeze future connections to one presented certificate.
- [RFC 1034, Domain Names — Concepts and Facilities, November 1987](https://www.rfc-editor.org/rfc/rfc1034.html)
  and [RFC 1035, Domain Names — Implementation and Specification, November
  1987](https://www.rfc-editor.org/rfc/rfc1035.html): DNS name/address lookup,
  caching, refresh, and failure are resolver/name-server responsibilities.
  Ardents Discovery does not become a DNS resolver.
- [RFC 5280, PKIX Certificate and CRL Profile, May 2008](https://www.rfc-editor.org/rfc/rfc5280.html):
  certification-path validation, validity, constraints, and revocation inputs
  are PKIX responsibilities, not discovery-record semantics.
- [RFC 9110, HTTP Semantics, June 2022](https://www.rfc-editor.org/rfc/rfc9110.html):
  HTTP defines method safety/idempotency and permits automatic retry only when
  request semantics make repetition safe; a client should not automatically
  retry a failed automatic retry.
- [Go `crypto/tls`, Go 1.26.5 documentation](https://pkg.go.dev/crypto/tls):
  the standard client configuration exposes server-name/root verification
  controls; disabling verification is not a product-safe default.

These sources support the TLS and retry constraints. They do not create an
Ardents application authentication protocol.

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
| Application | Application Principal authenticated by a finite Credential and local Session | exact Discovery Access Grant; optional one-hop Delegation intersection | session, Application-owned service credentials, request payload | Application process ↔ protected local Application Interface |
| Effective Principal | Principal named by a valid Delegation, otherwise Actor | intersected resolve authority only | delegated authority | Identity/access admission |
| publishing Node | Node Principal and discovery signing key | asserts signed locator facts while current trust/policy accept them | Node key, publication truth | remote signed record ↔ importing Node |
| Hosted Service owner | Actor Application Principal under DR-02 | lifecycle mutations on its Hosted Service | aggregate, lease, profile selection | Application Interface ↔ Hosting module |
| workload | no Principal by default | only its application protocol and locally provisioned secrets, if any | service data and protocol state | ingress proxy ↔ workload |
| service operator/protocol owner | protocol-specific identity | defines client authentication and authorization | service CA, credentials, ACLs, data | Ardents endpoint ↔ external service |
| DNS/PKI operator | DNS namespace and CA keys | certifies DNS-ID/IP-ID under its policy | DNS zone, CA/intermediate keys | client resolver/trust store ↔ external infrastructure |
| Waku participant | Waku Peer ID | transport participation only | Waku connection and routing state | Waku transport |

An Application Credential authenticates an Application to a Node. It is not a
service credential. An Access Grant authorizes an Ardents operation. A
Delegation attenuates one Ardents call. A Channel Grant protects a Waku channel.
None of them is a TLS certificate or an application-protocol authorization.

## Invariants

- Ardents authorizes `Resolve` before reading or disclosing a target.
- A successful `Resolve` means only that the target passed current freshness,
  publisher trust, endpoint eligibility, route policy, and scheme preference.
- A Node Principal signature never proves possession of the endpoint's TLS
  private key and never authorizes an application operation.
- Service authorization occurs at the service before application state
  mutation; it cannot be inferred from resolve permission.
- Ardents never forwards an Application Session, Credential, Access Grant,
  Delegation, root/device key, or Node-held secret to the endpoint.
- `https` clients perform PKIX validation and DNS-ID/IP-ID matching against the
  URI host. They do not set an alternate name learned from an endpoint's DNS
  resolution result.
- Discovery record signing keys, Node Principals, service IDs, Waku Peer IDs,
  SPKI hashes, and leaf fingerprints are not interchangeable pins.
- Certificate renewal that preserves the validated reference identity and
  accepted trust path requires no discovery-record or SDK migration.
- A TLS error fails closed. The caller does not fall back from `https` to
  `http` or disable verification.
- Discovery returns no credential, trust root, certificate chain, pin, token,
  header, method, path, request body, or application retry instruction.
- Ardents performs no dial, DNS lookup, TLS handshake, probe, proxy, or remote
  fetch on behalf of the resolve caller.
- Endpoint sets, accepted schemes, response count, parsing work, ingress
  connections, timeouts, retries, and application payloads remain bounded by
  their respective owners.
- Restart reloads one authoritative discovery/Hosting truth; no client cache
  may extend record freshness or Hosting lease authority.
- Application-protocol denial and `404`/EOF are never remapped to Ardents
  `Forbidden`/`NotFound`.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
| Application admission and exact grants | in-process | Identity/access | Ardents | yes |
| safe target locator and projection | in-process | Discovery | Ardents | yes |
| discovery store/trust/route policy | in-process | Discovery/Policy | Ardents | yes |
| protected Unix Application transport | local-substitutable | Application Interface | Ardents | yes |
| imported signed service record | remote-owned | publishing Ardents Node | Ardents convergence; publisher fixes publication | signed fixture |
| Hosting aggregate/readiness/withdrawal | remote-owned relative to consuming Node | hosting Node | hosting Node | fake published record |
| Go HTTP/TCP/TLS stack | local-substitutable | embedding Application / Go runtime | Application | local listener/CA |
| DNS resolver and PKIX trust infrastructure | true-external | deployment/platform | Application/service operator | local DNS/CA fixture |
| service process and application protocol | true-external | service owner | service owner | protocol-specific fake |
| application credential/authorization store | true-external | service owner | service owner | protocol-specific fake |

There is no Ardents port for the final four dependencies because the selected
module does not call them. Introducing one would create a hypothetical adapter
and move true-external semantics into the product.

## Alternative designs

### Alternative A — discovery-only plane (selected)

- External interface: the accepted
  `Discovery.Resolve(context.Context, Query) ([]Target, error)` only.
- Internal seam: one deep locator owns current snapshot selection, trust,
  eligibility, policy, deterministic ordering, privacy, and bounds.
- State ownership: Discovery/Hosting own locator and lifecycle truth;
  Application and service own connection/protocol truth.
- Authority model: exact resolve grant only; service uses its own authentication
  and authorization.
- Failure and recovery: Ardents errors end at Resolve; transport/TLS/protocol
  errors remain native and require application policy.
- Compatibility and migration: no new wire beyond Discovery; no persisted
  direct-client state or service credential format.
- Operational cost: operators diagnose locator and service separately; no
  centralized data-plane tracing.

### Alternative B — bounded Ardents HTTP client adapter

- External interface: `Direct.Do(ctx, ServiceType, Request) Response` with a
  closed method/path/body/status contract.
- Internal seam: resolve, select, dial, TLS verify, acquire service credential,
  inject Principal proof, apply deadlines/retry, and map errors.
- State ownership: Node/SDK must own connection pools, credential rotation,
  replay policy, request limits, and partial-response outcomes.
- Authority model: needs a new exact `application.service.invoke` action plus a
  specified translation from Actor/Effective authority to a service-verifiable
  credential.
- Failure and recovery: must distinguish resolution, dial, TLS, credential,
  service denial, HTTP status, partial response, ambiguous mutation, drain, and
  retry exhaustion.
- Compatibility and migration: new wire/SDK types, service authentication
  protocol, trust configuration, credential schema, rotation, and mixed-server
  behavior.
- Operational cost: deep implementation and useful uniform controls, but every
  supported service must adopt Ardents authentication and HTTP semantics.

### Alternative C — Node data-plane proxy or service mesh

- External interface: one Node endpoint routes by service type/ID and forwards
  arbitrary TCP or HTTP.
- Internal seam: gateway/sidecar, routing, TLS termination/origination,
  identity propagation, per-route policy, health/load balancing, drain, and
  telemetry.
- State ownership: Node owns long-lived connection/routing/certificate and
  authorization state in addition to Hosting and Discovery.
- Authority model: proxy must authenticate every connection and safely convey
  identity to workloads.
- Failure and recovery: Node becomes the availability and backpressure owner
  for all service traffic.
- Compatibility and migration: deployment topology, ports, certificates,
  protocols, observability, and rollback all change.
- Operational cost: highest; this is a general service mesh and violates the
  assignment boundary.

### Decision matrix

Scores are 1 (poor) to 5 (strong); weighted total is out of 500.

| Criterion | Weight | A: discovery only | B: HTTP adapter | C: proxy/mesh | Evidence or reasoning |
|---|---:|---:|---:|---:|---|
| Module depth | 20 | 5 | 3 | 2 | A concentrates locator complexity behind one method; B/C expose protocol policy |
| Caller leverage | 15 | 3 | 5 | 4 | B makes a supported HTTP case easier; A leaves protocol work to caller |
| Change locality | 15 | 5 | 2 | 1 | application protocol changes do not touch A |
| Trust-model fit | 20 | 5 | 2 | 1 | A does not invent grant-to-service credential translation |
| Failure clarity | 10 | 5 | 2 | 1 | A preserves three distinct error domains |
| Migration cost | 10 | 5 | 2 | 1 | A adds no data-plane state/schema |
| Operability | 10 | 3 | 4 | 4 | B/C centralize telemetry but also own more failure |
| **Weighted total** | **100** | **460** | **285** | **195** | A wins despite less convenience |

The deletion test favors A: deleting the locator would force every caller to
reimplement trust, eligibility, policy, privacy, ordering, and bounds. Deleting
a thin `net/http` wrapper would mostly reveal standard-library calls.

## Selected design

### External interface sketch

No Direct Service interface is added. DR-05 accepts the Discovery interface
unchanged:

```go
type Query struct {
    ServiceType     string
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

The client selects only schemes it can use safely. A returned `http` or `tcp`
target carries no Ardents service-authentication guarantee. A returned `https`
target is usable only when the Application has a normal trust policy for the
URI host and the service's application protocol defines any client
authentication.

The SDK may document a usage example with `net/http` or `net.Dialer`, but must
not ship a wrapper that changes retries, redirects, credentials, TLS
verification, errors, or connection ownership. It may not expose a helper that
sets `InsecureSkipVerify`, trusts the discovery signing key as a TLS key, or
pins the first certificate seen.

### Internal seam and state machine

The deep module remains the Discovery locator:

```text
request
  -> validate
  -> authorize exact service-type
  -> read maintained snapshot
  -> freshness/withdrawal/mode filter
  -> current publisher trust
  -> endpoint eligibility + route policy
  -> preference/sort/deduplicate/cap
  -> targets or privacy-uniform NotFound
```

The direct connection is deliberately a separate, Application-owned state
machine:

```text
unresolved -> resolved -> connecting -> [TLS verifying] -> protocol active
     ^           |             |               |               |
     |           +-- stale ----+-- failure ----+-- failure ----+
     +---------------- re-resolve by caller policy -------------+
```

Ardents owns only `unresolved -> resolved`. It does not persist connection,
certificate, request, response, retry, circuit-breaker, or credential state.

### Authority and audit semantics

- `Actor` is the Application Principal authenticated to the local Node.
- `Effective` is the delegating Principal only when the existing valid one-hop
  Delegation is presented.
- Resolve requires `application.discovery.resolve` on exact
  `service-type:<canonical-type>` or an admitted Node scope, with the existing
  Actor/Effective grant and Delegation intersection.
- Successful resolution does not grant `connect`, `invoke`, `read`, `write`, or
  any service-defined action.
- Hosting ownership is unchanged. The Hosted Service owner controls Ensure,
  Renew, Recover, and Drain under DR-02; another Principal may resolve its
  published type only if that Principal independently has Discovery authority.
- Service authentication and authorization are application-protocol facts.
  The service may use OAuth, mTLS, an application token, signed requests, or no
  authentication, but Ardents v1 neither standardizes nor supplies them.
- Access Grant revocation blocks the next Resolve. It cannot terminate an
  already established external connection or revoke a credential the service
  owns.
- Discovery admission audits Actor, Effective, action, resource, and outcome.
  It never records endpoint credentials or application payload. The external
  service owns invocation audit.

### TLS identity, rotation, and pinning

For existing eligible `https` targets:

1. the URI host is the reference identity;
2. a DNS host requires DNS-ID matching; a literal address requires exact
   IP-ID matching in `subjectAltName`;
3. the client validates the chain and validity against an explicitly selected
   platform/application trust store;
4. SNI uses the DNS reference name when the endpoint host is DNS;
5. a name/path/validity failure is terminal for that target and must not
   degrade to plaintext;
6. redirect targets are new origins and are governed entirely by the
   application client, including credential forwarding and allowlisting.

The accepted v1 Application locator admits only literal non-loopback endpoint
hosts, so its HTTPS case uses IP-ID. DNS endpoints remain Operator-only and
ineligible for Application projection. The DNS-ID rule above governs any
future separately accepted DNS expansion and any endpoint configured directly
by the Application outside Ardents; it does not silently widen Discovery.

The discovery record does not carry a TLS public key, certificate digest,
trust bundle, or alternate reference name. Signing the URI proves that a
trusted Node advertised it, not that the Node controls DNS/CA issuance or that
the service presented the right certificate.

Certificate issuance and rotation are service/deployment responsibilities.
Normal renewal may overlap old/new valid chains and requires no Ardents record
change when the URI reference identity is unchanged. A hostname/IP change
requires a new signed endpoint publication; old endpoints disappear through
withdrawal/expiry. Because the public Target exposes no TTL, client
applications must not persist it as independent authority; they re-resolve for
a later operation group and after the one bounded failure policy below.

No TOFU pinning is supported. Operator-configured private roots may be used by
the Application, but they are application/deployment configuration and not
delivered through Discovery. Static SPKI/certificate pins, if an application
protocol requires them, are likewise preconfigured by that application and
must define overlap/recovery; Ardents does not generate or rotate them.

Application-owned Hosting does not admit `https` in v1. The current raw proxy
could pass TLS bytes, but Hosting has no safe private-key delivery, no
profile-owned certificate reference, no renewal controller, and no proof that
readiness exercised the same TLS identity. Baking a private key into an image
or public environment is forbidden. A future HTTPS Hosting design must be a
separate R2 decision owned by Operator certificate policy and must preserve the
DR-02 lease/withdrawal boundary.

## Delivery and data semantics

| Concern | Ardents guarantee | Application/service responsibility |
|---|---|---|
| target ordering | deterministic scheme preference, service ID, endpoint bytes | selection/failover policy |
| connection establishment | none | DNS, dial timeout, TLS handshake, cancellation |
| acknowledgement | Resolve unary response only | application protocol |
| deduplication | exact target pairs only | request/idempotency keys |
| ordering | no request ordering | application protocol/connection |
| expiry | signed record and Hosting lease bound target freshness | application credential/data expiry |
| payload limit | Application Interface limit for Resolve only | request/response/frame limits |
| backpressure | bounded Resolve and ingress connection admission | client concurrency and protocol flow control |
| retry | no automatic direct-service retry | method/protocol-specific policy |
| large payload | not carried by Resolve | application protocol or Content Reference by explicit design |
| terminal outcome | typed Resolve result | transport/TLS/protocol terminal outcome |

Caller guidance is deliberately conservative:

- resolve once per operation group and use at most the returned eight targets;
- apply a finite overall deadline and finite connect/TLS/header/body/idle
  limits appropriate to the protocol;
- try another returned target only before an application request might have
  taken effect, or when the application protocol supplies idempotency;
- re-resolve at most once after bounded connection failure, with jittered
  caller-owned backoff;
- never automatically replay a non-idempotent request, a partially written raw
  TCP exchange, or a streaming operation;
- never interpret Hosting readiness or successful TCP connect as application
  health.

These are documentation constraints, not behavior hidden in an Ardents SDK
adapter.

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
| malformed Resolve query | Ardents `InvalidArgument` | none | no | none |
| invalid session | Ardents `Unauthenticated` | session truth | existing one refresh only | repair enrollment/credential if persistent |
| resolve grant/Delegation mismatch | Ardents `Forbidden` | access truth | no | issue/repair exact grant |
| absent, stale, withdrawn, untrusted, unsafe or policy-denied target | privacy-uniform Ardents `NotFound` | discovery/trust/policy truth | no automatic retry | inspect Operator discovery diagnostics |
| discovery/trust store unavailable | Ardents `Unavailable` | retained discovery truth | bounded caller retry | repair store/trust subsystem |
| DNS failure after Resolve | native client resolver error | no Ardents write | application policy; optionally re-resolve once | repair DNS/publication |
| connect refused/timeout | native transport error | no Ardents write | next target only within operation safety | inspect ingress/readiness/firewall |
| TLS chain/name/expiry failure | native TLS error | no Ardents write | no plaintext fallback; another independently valid target only | rotate/repair certificate, DNS, clock or trust |
| service authentication denial | application-protocol denial | service auth truth | no Ardents/session refresh | repair service credential/policy |
| service authorization denial | application-protocol denial | service policy truth | no | repair service grant/ACL |
| HTTP 429/503 or protocol busy | application-protocol response | service truth | only per protocol and bounded `Retry-After` policy | inspect service capacity |
| EOF/timeout after partial write | ambiguous application result | service truth | no replay without protocol idempotency | reconcile through protocol |
| Hosting drain/expiry/revocation | connection refusal/closure or service response; Resolve converges to `NotFound` | Hosting aggregate and withdrawal | no assumption that old connection remains valid | inspect Hosting lifecycle |
| consuming Node restart | temporary Resolve unavailable, then snapshot reloaded/re-evaluated | discovery truth | bounded Resolve retry | repair Node if load fails |
| publishing Node restart | stale record may remain only until signed expiry; no optimistic new publication | publisher Hosting/discovery truth | re-resolve after bounded failure | recover Hosting/publication |
| certificate rotation | uninterrupted if reference identity/trust path overlap | external PKI truth | ordinary reconnect | service operator rotates |
| endpoint/name rotation | old then new signed record subject to convergence/expiry | discovery publication truth | re-resolve | publish new, then withdraw old |
| client binary upgrade | native client behavior | no Ardents direct-client state | application-owned | application owner |

## Security, privacy, and abuse analysis

- An authorized resolver can learn up to eight eligible endpoints for a service
  type. Exact service-type grants and uniform `NotFound` bound enumeration.
- A signed endpoint is untrusted input until the locator applies syntax,
  freshness, trust, mode, route policy, scheme, deduplication, and count bounds.
- The Node never dials the target for an Application, so Resolve adds no
  Node-side SSRF primitive. The Application still treats a returned private or
  link-local target as network input.
- Publisher compromise can advertise a malicious endpoint within current trust
  and policy. TLS/application authentication provides an independent check
  where configured; Operators revoke publisher trust and withdraw records.
- Endpoint publication does not carry URI userinfo, fragments, credentials,
  tokens, headers, trust bundles, certificate chains, or pins.
- Application logs and telemetry must redact URI userinfo if a protocol owner
  nevertheless configures such a URI outside Ardents; Discovery rejects it.
- A malicious service can stall, stream without bound, redirect, compress
  excessively, return huge bodies, or close after a partial request. The
  Application must set protocol-specific limits; Ardents cannot safely invent
  universal values.
- The existing ingress proxy bounds active connections globally/per-port/per-
  source and enforces dial/idle/write deadlines. Those are ingress resource
  controls, not an authorization guarantee and not a substitute for client
  response limits.
- Application Principal, Effective Principal, Access Grant, Delegation,
  session secret, root/device key, and request payload never appear in
  discovery records or service connection metadata created by Ardents.
- Metrics never label Principal, service type/ID, endpoint, hostname,
  certificate fingerprint, method, path, or application error.
- TLS 1.3 early data/0-RTT is not enabled by any Ardents adapter because no
  adapter exists. An application that enables it owns replay safety.

## Observability

Ardents observes only its control-plane responsibility:

- Resolve totals by stable outcome and scheme-set shape, without service or
  endpoint labels;
- locator duration and bounded candidate/filter counters;
- discovery store/trust/policy health;
- publication/withdrawal/readiness and ingress connection-limit metrics already
  owned by their modules;
- structured admission audit for authentication/authorization outcomes.

It does not collect request paths, payload sizes, HTTP status codes, TLS peer
certificates, application credentials, or per-service latency for direct
traffic. The Application and service own those signals.

Operator procedure:

1. classify whether failure occurred before or after successful Resolve;
2. for Resolve failures, inspect exact record, freshness, trust, policy,
   publisher readiness, lease, ingress, and withdrawal;
3. for post-Resolve failures, reproduce DNS/dial/TLS from the Application's
   deployment context without exporting credentials;
4. verify URI host against certificate SAN and trust path;
5. inspect service-owned authentication, authorization, capacity, and logs;
6. repair the owning module and let withdrawal/refresh/rotation converge;
7. never bypass verification, rewrite persisted records, or copy Node/session
   keys into a workload.

Node health does not become failed because an external service returns an
application error. Discovery becomes degraded/failed only for its own store,
trust, projection, or convergence invariant. Hosting health follows DR-02.

## Compatibility consequences

- Wire/SDK: no Direct Service protocol or SDK module is added. The only
  prerequisite is the additive Discovery interface already proposed.
- Persistence: no connection, credential, certificate, pin, retry, or
  application-error state is persisted by Ardents.
- Configuration: no CA, trust bundle, token issuer, route, retry, or service
  method configuration is added to the Node. Existing WSS transport
  certificates remain unrelated.
- Hosting: `http`/`tcp` remain the Application-selectable v1 protocols.
  Existing Operator-created `https` records remain discoverable when the
  accepted locator deems their endpoint eligible; their TLS operation is not
  claimed or managed by Hosting.
- Backup/restore: no new material. Application/service credentials and private
  PKI remain outside the Ardents backup contract.
- Rollout: Discovery can roll out additively. Old Nodes report the feature
  unavailable. There is no mixed-generation direct-client negotiation.
- Downgrade: no marker or direct-client state exists. Hosting downgrade rules
  remain those of ADR-0012.
- A future Ardents-authenticated direct protocol is not an additive helper: it
  requires a new R2 packet and ADR defining credential issuer, audience,
  revocation, replay, service integration, TLS profile, errors, migration, and
  qualification.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit | locator filtering/order/dedup/cap and no dial/probe/fetch; target carries no auth/TLS material; SDK has no implicit adapter/retry | Go unit with fake clock/store | JSON/JUnit tied to exact commit |
| Contract | exact resolve action/resource/Delegation; three target fields only; uniform `NotFound`; no Direct service/procedure or credential translation | real Application handler + SDK | protocol/SDK contract report |
| Integration | signed local/imported records; withdrawal/trust/policy change; `http`/`tcp`; existing `https` target retained as locator only; no Application-owned Hosting `https` admission | local Nodes and protocol fixtures | tagged JSON/JUnit |
| E2E | Application resolves then uses its own bounded HTTP/TCP client; service denial remains service-native; second Principal cannot resolve; drain/expiry removes target | Linux protected socket, real Nodes/Docker | scenario logs and IDs |
| Security | malicious endpoint syntax; enumeration/privacy; no credential/key forwarding; HTTPS name/path/expiry negatives; no plaintext fallback/TOFU; redirect credential isolation; partial-write non-retry | local CA/DNS/listeners plus Linux Docker | security report |
| Deployment | private-LAN/public-direct target use under accepted DR-04 topology; DNS/PKI outage separation; certificate overlap/expiry; endpoint rotation; firewall/partition; restart convergence | supported deployment, no Kubernetes | deployment evidence bundle |
| Release | static, fast, race where supported, tagged, security, deployment and required multinode gates pass once on one clean commit; docs make no service-auth claim | canonical release matrix | accepted DR-06 snapshot |

Capability `application.discovery` may advance only with its own matching
evidence. No `application.direct-service` or `application.service-auth`
capability exists at the frozen baseline. Every capability retains `Q=no`
until DR-06 accepts one exact commit.

## Open questions

None that changes the selected v1 interface, trust root, persistence model, or
migration contract.

A future product request for one uniform authenticated application protocol is
new R2 scope, not unfinished implementation of this packet. Likewise,
Application-owned HTTPS Hosting requires a new certificate/secret lifecycle
decision before it can be added to an Operator profile.

## Decision-register proposals

| Type | Proposed row | Rationale |
|---|---|---|
| Decision | Ardents v1 direct-service responsibility ends after authenticated, authorized, privacy-filtered Discovery resolution; the Application owns dial/TLS/protocol behavior | avoids a generic client proxy or mesh |
| Decision | Resolve authority is not service-use authority; Access Grants, Delegations, Sessions and Application Credentials are never forwarded or translated into service credentials | preserves identity/access boundaries |
| Decision | A Node-signed service record authenticates publisher locator facts, not the endpoint process; HTTPS uses normal PKIX plus URI-host DNS-ID/IP-ID verification | prevents identity conflation |
| Decision | Discovery carries no TLS pin; TOFU is rejected and ordinary certificate renewal under the same reference identity does not require record migration | permits safe rotation |
| Decision | Application-owned Hosting remains HTTP/TCP until a separate Operator-owned certificate/private-key delivery and rotation design is accepted | DR-02 has no safe secret lifecycle |
| Decision | Ardents defines no generic direct-service payload, retry, redirect, streaming, status, or error semantics | application protocols retain ownership |

## Recommendation

Write Proposed ADR-0014 before implementation. Then implement only the
Discovery dependency and documentation/qualification slices below. Reject a
Direct Service adapter for v1.

## Vertically sliced implementation issues

### DSI-01 — Freeze the discovery-only contract

- User story: an Application developer can tell exactly what a resolved target
  proves and where Ardents responsibility ends.
- Complete behavior: accept ADR-0014; align Discovery/Hosting product docs and
  SDK documentation; state that resolve authority is not service-use
  authority; prohibit credential forwarding, automatic direct retry, TOFU,
  verification bypass, and TLS identity conflation.
- Acceptance: architecture/doc tests and review find no `Do`/`Dial`/proxy or
  service-token surface and no claim that a signed endpoint authenticates the
  workload.
- Blocked by: ADR-0014 acceptance; accepted Application Discovery and DR-02.
- Research class after packet: R0 documentation/contract work.

### DSI-02 — Qualify locator-to-application handoff

- User story: an authorized Application can resolve a target and use its own
  bounded HTTP/TCP client while errors remain attributable to the right owner.
- Complete behavior: add protocol fixtures and examples that consume
  `Discovery.Target` without an Ardents wrapper; cover finite dial/overall
  deadlines, cancellation, response/body bounds, pre-effect target failover,
  service-native denial, drain/expiry withdrawal, and second-Principal
  isolation.
- Acceptance: contract/integration/Linux E2E evidence distinguishes Resolve,
  transport, TLS, and application outcomes; no retry crosses an ambiguous
  partial write.
- Blocked by: Application Discovery AD-01 through AD-04; Hosting AH-03 only for
  the hosted-service lifecycle scenario.
- Research class after packet: R1 integration/qualification.

### DSI-03 — Prove existing HTTPS locator semantics

- User story: an Application using an existing Operator-published HTTPS service
  either authenticates the URI host normally or fails closed.
- Complete behavior: local CA fixtures for IP-ID success through the v1
  locator; prove DNS endpoints remain ineligible; cover wrong SAN,
  expired/untrusted chain, IP rotation, certificate overlap, redirect
  credential isolation, and no plaintext/TOFU fallback. DNS-ID behavior is
  documented and tested only in an Application-owned direct-endpoint fixture,
  not as an implicit Discovery expansion. Confirm discovery carries no trust
  root or pin.
- Acceptance: security/deployment matrix with artifacts tied to one commit;
  ordinary certificate renewal under the same reference identity needs no
  discovery schema change.
- Blocked by: DSI-02; accepted DR-04 topology for deployment claims.
- Research class after packet: R1/R2 at PKI/deployment boundary.

### DSI-04 — Qualify the supported handoff

- User story: release reviewers can verify the exact discovery-only promise on
  one clean release candidate.
- Complete behavior: run the complete acceptance matrix, link retained
  evidence, verify documentation/capability claims, and prove no feature
  depends on a Node data-plane proxy or Application-owned HTTPS Hosting.
- Acceptance: all required gates pass on one source commit; failures are not
  hidden by retry; `Q` changes only through DR-06.
- Blocked by: DSI-03, relevant Application Discovery and Hosting slices,
  accepted DR-04 support topology, DR-06 gate definition.
- Research class after packet: R3 qualification.

## Cross-stage dependencies

- Upstream hard: ADR-0001, ADR-0002, accepted Application Discovery locator,
  accepted DR-02 Hosting, and Proposed ADR-0012.
- Implementation: AD-01 admission registry is required before Discovery;
  AD-02 through AD-04 provide the only public interface consumed here.
- Hosting: AH-03 supplies lease/readiness/withdrawal behavior for a managed
  service, but does not expose the endpoint through Hosting.
- Parallel cross-check: DR-04 supplies the supported private-LAN/public-direct
  topology, DNS availability, firewall, and deployment ownership used only by
  DSI-03/DSI-04 qualification.
- Downstream: DR-06 owns matching-commit evidence and capability promotion.
- Explicitly absent: no dependency on DR-01 Messaging or DR-03 Channel Grant
  authority; direct service credentials are not Channel Grants.
