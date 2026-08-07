---
id: R-002
title: What is the smallest live Application Interface?
status: decided
owner: product research
started: 2026-08-07
reviewed: 2026-08-07
---

# R-002 — Live Application Interface

## Decision this unlocks

Define the smallest product boundary through which ordinary local software can
publish and consume Ardents Services. The result must be concrete enough to
specify the first tracer and later compare transport foundations without
selecting a programming language or wire protocol.

## Current contract

- [Network functional map](../../product/functional-map.md)
- [J-04: integrate an Application](../../product/journeys.md#j-04--integrate-an-application)
- [R-006: Service Target lifecycle](r-006-service-target-lifecycle.md)
- [Domain language](../../../CONTEXT.md)
- [Threat model](../../security/threat-model.md)

Already fixed: Ardents connects external Applications to Service Targets; V1
has one active Service Instance; Application Data is opaque; the network does
not own User identity, application authorization, persistence, semantic retry,
or offline delivery; and the V1 data primitive is one live reliable ordered
bidirectional byte stream.

### P1-D1 — External local integration

**Product Owner decision, accepted 2026-08-07:** an existing Application must be
able to use Ardents through a local socket/proxy-style Application Interface
without embedding a mandatory Ardents SDK or networking library. An SDK is only
a Developer convenience wrapper over that interface; it is not a network layer
and cannot add transport behavior or guarantees unavailable without the SDK.

Consequences:

- the Application and Ardents endpoint may be separate processes;
- a normal local server can remain bound to localhost while Ardents publishes
  it as a Service;
- a normal local client receives a familiar connection boundary rather than
  route, relay, rendezvous, or descriptor internals;
- language-specific SDKs may wrap the interface for convenience, but do not
  become the authoritative network contract;
- routing, rendezvous, target authentication, encryption, connection state, and
  resource enforcement remain implemented by the Ardents endpoint rather than
  independently inside each SDK;
- an Application that does not use an SDK can perform the same authoritative
  operations and observe the same results through the Application Interface;
- the accepted outcome does not yet select SOCKS, HTTP CONNECT, Unix sockets,
  named pipes, RPC framing, or a particular operating-system API.

### P1-D2 — Stream-only V1

**Product Owner decision, accepted 2026-08-07:** V1 exposes exactly one
Application Data primitive: a live reliable ordered bidirectional byte stream.
The Service Connection preserves byte order in each direction while it exists,
but does not create application message boundaries or promise that a completed
local write was processed by the remote Application.

Consequences:

- connection closure or failure is observable rather than converted into
  retained delivery;
- datagrams, offline queues, delivery receipts, exactly-once semantics, and
  automatic replay or reconnect are not V1 network functions;
- framing, semantic acknowledgements, idempotency, reconnect, and retry belong
  to the Application protocol;
- future transport primitives may be added alongside the stream, but cannot
  silently change its accepted semantics;
- exact partial-write, timeout, close, and failure reporting is fixed by P1-D5.

### P1-D3 — Separate connection and administration privileges

**Product Owner decision, accepted 2026-08-07:** the Application Interface has
two logically separate privilege boundaries. The least-privileged Connection
Interface carries connection operations and Application Data; Service Authority,
publication, and configuration require a separately authorized Service
Administration Interface.

The operation boundary is:

- Connection Interface: connect, accept an already authorized Service's
  connections, read, write, close, cancel, and observe connection errors;
- Service Administration Interface: create, import, or export Service Authority;
  publish or unpublish a Service; bind its local endpoint; initiate target
  replacement under the accepted R-006 lifecycle; and change Service
  configuration or policy;
- access to the Connection Interface never grants Service Administration
  Interface access.

Consequences:

- compromising an ordinary Application does not by itself expose Service
  Authority or permit rebinding, unpublishing, or reconfiguring a Service;
- an SDK may wrap either interface but cannot merge their authority boundaries;
- this is a logical and authorization boundary, not yet a choice of separate
  processes, sockets, protocols, or binaries;
- local access and scope are fixed by P1-D7.

### P1-D4 — Name and target destinations

**Product Owner decision, accepted 2026-08-07:** an outbound connection accepts
either an exact Service Name or a Service Target. A supplied name is verifiably
resolved to its current target; a supplied target bypasses naming and selects
that exact machine-verifiable identity.

The authoritative result boundary is:

- connection success is reported only after the selected Service Target has
  been authenticated;
- the authenticated Service Target is available to the Application as part of
  the connection result, including when the input was a Service Name;
- successful name resolution alone is not connection success;
- resolution or authentication failure is explicit and never causes silent
  fallback to another name source, target, namespace, or ordinary network.

Consequences:

- human-facing Applications may use stable Service Names that follow an accepted
  target replacement;
- machine integrations may connect to or pin an exact Service Target without
  depending on the naming system;
- the interface does not yet choose destination syntax, metadata encoding, or a
  concrete proxy protocol;
- P1-D5 defines the exact observable failure classes.

### P1-D5 — Honest bounded connection results

**Product Owner decision, accepted 2026-08-07:** the Connection Interface reports
only supported product-level outcomes. It never guesses an unavailable root
cause, exposes Node identities or route topology, or turns a transport event
into a claim about Application-level delivery.

The V1 outcome classes are:

- invalid destination or Service Name resolution failure;
- local authorization or policy denial, including a local resource limit;
- Service unavailable, when the network has evidence for that distinction;
- Route unavailable, when the network has evidence for that distinction;
- Service Target authentication failure;
- local timeout or cancellation;
- authenticated connection established;
- clean transport close or abrupt connection loss;
- indeterminate failure when no narrower supported class is justified.

Stream semantics at failure are:

- a successful local write reports only bytes accepted by the local Connection
  Interface; it is not proof that the remote Application read or processed them;
- after partial write, timeout, or connection loss, remote Application completion
  is unknown unless the Application protocol supplied its own acknowledgement;
- a clean transport close is not an Application success receipt;
- Ardents may perform safe bounded route work while establishing or maintaining
  the same Service Connection, but never silently reissues an Application
  operation, reconnects as a new connection, or replays Application Data;
- exact causes that cannot be distinguished in a hostile network collapse to
  the indeterminate class rather than a fabricated diagnosis.

Consequences:

- Applications can react to stable product classes without learning relay or
  topology internals useful for probing;
- authentication failure remains distinct and cannot silently downgrade or
  fall back;
- R-007 must define the evidence and bounded retry behind Service unavailable,
  Route unavailable, and indeterminate results;
- error names, numeric codes, serialization, and operating-system mappings are
  implementation choices made later.

### P1-D6 — Safe local Isolation Context

**Product Owner decision, accepted 2026-08-07:** every locally authorized
Application receives a distinct default Isolation Context. An Application may
deliberately create or select additional opaque contexts, but the absence of an
explicit value always uses its own safe default and never a global shared
context.

The product boundary is:

- an Isolation Context is local to the Ardents endpoint and is not transmitted
  to a Service or carrier Node;
- it is never a User identity, Service identity, address, credential, or public
  application profile;
- different contexts cannot share linkable routing, rendezvous, connection-pool,
  session-resumption, or other network-visible correlation state;
- the same context permits safe reuse where the Route Profile allows it, but
  does not assert that its connections belong to one real-world identity;
- the boundary prevents linkage introduced by forbidden endpoint-state reuse;
  it does not by itself defeat correlation through Application Data, timing,
  volume, or an observer of the local network;
- an Application may rotate or discard an additional context according to its
  own profile lifecycle;
- exact handle format, local transport, storage, and operating-system mapping
  are implementation choices made later.

Consequences:

- ordinary Applications receive a privacy-safe default without an SDK or
  explicit isolation configuration;
- a multi-profile Application can request stronger separation without creating
  network-wide Personas;
- deliberate reuse of one context tells Ardents that state reuse is permitted;
  the network cannot protect profiles that the Application intentionally places
  in the same context;
- R-004 and R-008 must identify and test every implementation state forbidden to
  cross this boundary;
- P1-D7 defines how the endpoint authorizes the local Application to which the
  default belongs.

### P1-D7 — Endpoint-local least privilege

**Product Owner decision, accepted 2026-08-07:** an Endpoint Owner grants access
only on one local Ardents endpoint. The endpoint enforces narrowly scoped Local
Grants; neither the owner nor any grant becomes a network-wide administrator,
credential, or approval root.

The V1 grant families are:

- **Connection Grant:** lets one Application open outbound Service Connections
  within local policy and, when explicitly scoped to a Service, accept that
  Service's incoming connections;
- **Service Administration Grant:** lets one Application publish, unpublish, and
  configure a specified Service without receiving its raw Service Authority;
- **Authority Custody Grant:** separately permits creating a new Service
  Authority or importing, exporting, and initiating R-006 replacement for a
  specified authority. This is the strongest local grant.

The authority rules are:

- every grant is scoped to one Application, allowed operations, and an optional
  Service; an ordinary Application receives only the Connection Grants it needs;
- Connection access never implies Service administration, Service administration
  never implies raw Authority export, and no privilege is inherited silently;
- there is no shared all-Application admin token;
- Local Grants and their identifiers remain on the endpoint and are never sent
  to a Service, carrier Node, resolver, or naming system;
- an Endpoint Owner may issue grants through a future local UI, CLI,
  configuration, operating-system policy, or automation interface, but no remote
  network actor can issue or require them;
- an unattended server may preconfigure local grants without contacting a
  central operator.

The decentralization boundary is:

- joining, connecting, or publishing requires no central administrator approval;
- compromise or seizure of one Endpoint Owner grants no network-wide
  administrative power, although it compromises that endpoint and the Service
  Authorities held there;
- disappearance of one Endpoint Owner cannot prevent independent endpoints from
  joining, connecting, or publishing their own Services;
- a V1 Service still depends on the holder of its own Service Authority, but the
  Ardents network does not;
- naming, bootstrap, releases, and emergency policy remain separate Control
  Plane risks under R-012 and must not recreate one mandatory operator.

Consequences:

- a compromised ordinary Application cannot acquire Authority custody merely by
  using or serving traffic;
- a Service can be automated on a server without inventing a global Ardents
  account or administrator;
- exact capability encoding, user interaction, operating-system identity, IPC,
  storage, revocation, and audit mechanisms are implementation choices made
  later;
- P1-D8 closes the remaining local resource and backpressure contract.

### P1-D8 — Bounded resources with a performance gate

**Product Owner decision, accepted 2026-08-07:** resource safety and honest-use
performance are coequal requirements. The Application Interface uses finite,
hierarchical budgets and stream backpressure, while every security control is
measured for its latency, throughput, CPU, memory, fairness, and overload cost.

The budget hierarchy is:

1. the Endpoint owns a finite parent budget;
2. each Local Grant and Application receives a child budget;
3. Services and Isolation Contexts consume that parent rather than creating new
   capacity;
4. connections, handshakes, active operations, buffers, queues, bandwidth, and
   processing work consume bounded leaves.

The overload contract is:

- creating additional Services, Isolation Contexts, or connections never
  multiplies an ancestor budget;
- a slow reader or writer causes bounded backpressure before memory growth;
- the reliable stream never silently drops accepted Application Data to relieve
  pressure;
- new work is rejected before an exhausted queue grows without bound;
- if existing work cannot continue safely, it terminates with an explicit
  Connection Result rather than apparent success;
- scheduling prevents one Local Grant or Service from monopolizing all Endpoint
  progress;
- limits are finite, observable, and locally configurable, but exact values and
  operating-system mappings are selected only from measurements.

The performance evidence must cover:

- cold and warm connection-setup latency, including tail latency;
- steady throughput and latency under concurrent honest connections;
- CPU and memory per idle and active connection;
- fairness across Local Grants and Services;
- backpressure, overload recovery, and behavior under adversarial churn;
- incremental cost of authentication, encryption, route construction, Local
  Grants, and Isolation Context separation against a declared direct baseline;
- representative client, server, and later constrained-device classes.

Consequences:

- a security mechanism is not accepted merely because it survives attack; its
  honest-workload cost must fit the product budget;
- a performance optimization is not accepted if it bypasses target
  authentication, isolation, least privilege, or resource bounds;
- P1-D8 fixes semantics and required measurements, not invented numeric targets;
- R-023 sets scenario-based V1 budgets, while R-004 and R-014 test routing and
  implementation candidates against the same evidence.

## Hypotheses

- **H1 — Connection plus Service Administration Interfaces:** Applications
  exchange bytes through a socket/proxy-style Connection Interface, while
  Service creation, authority import/export, publication, and policy use a
  separately authorized Service Administration Interface.
- **H2 — One native Ardents API:** all data and control operations use one
  Ardents-specific RPC or SDK contract.
- **H3 — Transparent proxy only:** Ardents intercepts ordinary application
  networking with no explicit control interface.
- **H0 — No accepted interface:** none provides sufficient isolation, failure
  semantics, and developer usability for the tracer.

## Evaluation criteria

1. An existing HTTP-like client and server can integrate without application
   protocol changes.
2. No Application needs relay identities, route construction, discovery records,
   or cryptographic implementation details.
3. Service Target authentication and connection failure remain observable.
4. Different Applications and Isolation Contexts cannot silently share local
   authority or forbidden routing state.
5. Backpressure, timeout, cancellation, close, and partial-write behavior can be
   specified without inventing application-level delivery guarantees.
6. A malicious local Application cannot export Service Authority or control
   another Application by default.
7. The contract can be implemented on the intended desktop and later mobile
   platforms and wrapped by multiple programming languages.
8. Optional datagrams or future transport types can be added without changing
   the semantics of an accepted stream.
9. A slow or malicious Application cannot create unbounded work or multiply its
   budget by creating child scopes.
10. Honest setup latency, throughput, CPU, memory, fairness, and overload
    recovery are measurable without disabling accepted security boundaries.

## Evidence plan

### Primary sources

Compare the official Tor SOCKS and control specifications, I2P SAM and I2CP
interfaces, I2P streaming semantics, and applicable IETF socket/proxy standards.
For each, record destination addressing, server publication, isolation signals,
authentication, error detail, local trust assumptions, portability, and known
metadata hazards.

### Experiment

No network implementation is required for the product decision. If interface
behavior remains ambiguous, create a disposable local-only contract harness
that maps an ordinary HTTP client and server through simulated `connect`,
`accept`, `read`, `write`, `close`, timeout, and route-loss events.

### Failure scenarios

- no Ardents endpoint is running;
- Service Name resolution fails before connect;
- the target is authenticated but no Instance is reachable;
- the route fails before any bytes, after a partial write, or during close;
- the local Application stops reading and creates backpressure;
- an untrusted local process attempts to publish with another Service Authority;
- two logical identities accidentally reuse one Isolation Context;
- the proxy reports success before target authentication finishes;
- an SDK wrapper changes or hides an authoritative interface error.
- one Application creates Services or Isolation Contexts to evade its budget;
- an honest workload becomes unusable under security or isolation overhead.

## Findings

- **Product Owner decision:** mandatory SDK integration is rejected. Existing
  Applications must be able to use a local socket/proxy-style boundary, and an
  SDK is limited to developer ergonomics rather than network implementation or
  additional semantics.
- **Product Owner decision:** the only V1 data primitive is a live reliable
  ordered bidirectional byte stream. Datagram, message, retention, exactly-once,
  and automatic replay semantics are rejected from the network contract.
- **Product Owner decision:** connection traffic and Service administration are
  separate privilege boundaries. Connection access cannot expose Service
  Authority or grant publication and configuration operations.
- **Product Owner decision:** both Service Name and Service Target are accepted
  destinations. Connection success exposes the exact authenticated target, and
  failed resolution or authentication never silently changes the destination.
- **Product Owner decision:** Connection Results use bounded, evidence-supported
  classes without route disclosure. Partial writes, transport close, and failures
  never imply remote Application completion or automatic replay.
- **Product Owner decision:** every Application receives a distinct local default
  Isolation Context and may create additional contexts. Contexts are never
  network identities, and different contexts cannot share linkable state.
- **Product Owner decision:** Local Grants separate connection use, per-Service
  administration, and Authority custody. Endpoint Owners are strictly local and
  no central administrator approves network participation.
- **Product Owner decision:** finite hierarchical budgets, backpressure, fair
  scheduling, explicit overload, and measured performance are part of the same
  security contract. Numeric targets require R-023 evidence.
- **Inference:** P1-D1 through P1-D8 select H1 as the V1 product shape. The
  accepted logical separation does not require separate protocols or processes.

## Options

### H1 — Connection plus Service Administration Interfaces

- Product fit: lets ordinary applications keep their data protocol while Ardents
  exposes explicit publication, authority, isolation, and status operations.
- Security fit: data and privileged control can receive different local
  authorization and audit treatment.
- Main cost: two related interface surfaces and their lifecycle must remain
  consistent.

### H2 — One native Ardents API

- Product fit: richest typed errors and features.
- Security fit: one explicit authority boundary.
- Reason not selected as the baseline: forces every Application and language to
  adopt Ardents-specific integration and risks making an SDK the real protocol.

### H3 — Transparent proxy only

- Product fit: minimal application changes for outbound connections.
- Limitation: service creation, authority handling, Isolation Context, and rich
  failure state still require configuration or another interface.
- Risk: hidden interception can make destination and downgrade behavior unclear.

## Recommendation

Keep **H1** as the working shape: an implementation-neutral local data path plus
an explicit separately authorized Service Administration Interface. P1-D1 fixes
the no-mandatory-SDK boundary, P1-D2 fixes the stream-only V1 data primitive,
P1-D3 fixes their privilege separation, and P1-D4 fixes destination and target
authentication semantics. P1-D5 fixes the observable result and failure boundary.
P1-D6 fixes safe default and optional Application-controlled isolation. P1-D7
fixes endpoint-local least privilege without a network administrator. P1-D8
fixes hierarchical resource, backpressure, fairness, and performance evidence.

**R-002 is decided.** H1 is the accepted V1 product boundary. R-023 supplies
numeric performance budgets, R-008 validates local isolation and grant
enforcement, and later technology research must implement rather than redefine
this interface.

No concrete proxy protocol, serialization, library, or language is selected.

## Disposition

- State: `decided`.
- P1-D1 accepted: external local socket/proxy-style integration; SDK optional,
  convenience-only, and non-authoritative.
- P1-D2 accepted: one live reliable ordered bidirectional byte stream; no
  datagrams, message boundaries, offline delivery, exactly-once semantics, or
  automatic replay.
- P1-D3 accepted: the Connection Interface is least-privileged and cannot grant
  access to the separately authorized Service Administration Interface.
- P1-D4 accepted: both Service Name and Service Target are valid destinations;
  success exposes the exact authenticated target and failures never silently
  fall back to another destination.
- P1-D5 accepted: Connection Results use bounded honest classes, expose no route
  internals, and never claim remote Application completion after write, close,
  or failure.
- P1-D6 accepted: every Application receives a safe local default Isolation
  Context, may add opaque subdivisions, and never exposes a context as network
  identity.
- P1-D7 accepted: Local Grants separate connection, per-Service administration,
  and Authority custody; each Endpoint Owner is local and no central administrator
  approves joining, connecting, or publishing.
- P1-D8 accepted: resources are finite and hierarchical; backpressure, fairness,
  explicit overload, and measured honest-use performance are mandatory.
- H1 is the accepted V1 shape; H2 is rejected as mandatory integration; H3 is
  insufficient by itself.
- R-002 and R-001 are closed; R-023 is the next foundation decision and defines
  the performance budget before routing comparison.
- No ADR and no code.
