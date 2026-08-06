---
id: R-002
title: What is the smallest live Application Interface?
status: active
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
- exact partial-write, timeout, close, and failure reporting remains P1-D5.

## Hypotheses

- **H1 — Local data interface plus control interface:** Applications exchange
  bytes through a socket/proxy-style local data path, while service creation,
  authority import/export, status, and policy use a separate bounded control
  surface.
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

## Findings

- **Product Owner decision:** mandatory SDK integration is rejected. Existing
  Applications must be able to use a local socket/proxy-style boundary, and an
  SDK is limited to developer ergonomics rather than network implementation or
  additional semantics.
- **Product Owner decision:** the only V1 data primitive is a live reliable
  ordered bidirectional byte stream. Datagram, message, retention, exactly-once,
  and automatic replay semantics are rejected from the network contract.
- **Inference:** P1-D1 and P1-D2 favor H1 or H3 over H2, but do not yet decide
  whether control and data share one protocol or how transparent the local
  integration should be.

## Options

### H1 — Local data interface plus control interface

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
an explicit bounded control surface. P1-D1 fixes the no-mandatory-SDK boundary,
and P1-D2 fixes the stream-only V1 data primitive.

Resolve the remaining contract one decision at a time:

1. **P1-D3:** which operations belong to the data path and which to control.
2. **P1-D4:** whether `connect` accepts Service Name, Service Target, or both and
   what authenticated result it returns.
3. **P1-D5:** exact connection, partial-write, timeout, and close failures.
4. **P1-D6:** how an Application supplies Isolation Context without turning it
   into a global identity.
5. **P1-D7:** local authorization for publishing and Service Authority access.

No concrete proxy protocol, serialization, library, or language is selected.

## Disposition

- State: `active`.
- P1-D1 accepted: external local socket/proxy-style integration; SDK optional,
  convenience-only, and non-authoritative.
- P1-D2 accepted: one live reliable ordered bidirectional byte stream; no
  datagrams, message boundaries, offline delivery, exactly-once semantics, or
  automatic replay.
- H1 remains the working shape; H2 is rejected as mandatory integration; H3 is
  insufficient by itself.
- P1-D3 is next.
- No ADR and no code.
