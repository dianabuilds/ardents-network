# Network research queue

Status: **open**

This backlog exists to design Ardents as a network product. A question belongs
here only when its answer changes an observable network contract, a security
claim, or a later technology comparison. It is not a list of every feature we
could build.

## Current working baseline

Research starts from these reversible product boundaries:

- Ardents addresses a Service Target, not a User or infrastructure Node;
- V1 uses one active Service Instance and a portable Service Authority; routine
  migration preserves the target, while loss or compromise replaces it through
  the Service Name;
- the smallest transport is an online Service Connection carrying opaque bytes;
- the Application owns its protocol, User identity, authorization, persistence,
  semantic retry, and offline behavior;
- the core connects external local Applications and does not assume a bundled
  runtime, replicated content store, or decentralized compute layer;
- Named Unlisted Site is a Reference Application for the network, not the
  definition of the network.

Each open question must either confirm, narrow, or reject one of those
boundaries with named evidence.

## Foundation — defines the network contract

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-006](records/r-006-service-target-lifecycle.md) | What is the V1 lifecycle of a Service Target across creation, publication, migration, loss, compromise, replacement, and retirement? | **Decided:** one active Instance uses a portable Service Authority. Routine migration uses encrypted export/import and preserves the target. Loss or compromise creates a new target and rebinds the Service Name; the old target remains untrusted. | decided |
| [R-002](records/r-002-live-application-interface.md) | What is the smallest live Application Interface that lets an existing local application publish and consume a Service safely? | **Active:** P1-D1 accepts a local socket/proxy-style boundary and optional convenience-only SDKs. P1-D2 accepts one live reliable ordered byte stream. P1-D3 separates connection and administration privileges. P1-D4 accepts both Service Name and Service Target as destinations; success exposes the exact authenticated target without silent fallback. The remaining contract must define failures, resource bounds, Isolation Context, and local authorization. | active |
| R-001 | Which endpoint, local-observer, relay-collusion, and broad traffic-observer capabilities must the Interactive Route resist, and what does it deliberately expose? | A claim matrix using the threat-model format, including conditions, measurable falsification, and honest limitations. | open |
| R-003 | How does an exact Service Name bind to a Service Target, resolve without becoming a directory, survive accepted rotation, and handle registration, expiry, recovery, conflict, enumeration, query privacy, and Control Plane capture? | A naming product contract and governance/failure state machine; no registry technology is selected until this exists. | open |
| R-004 | Which routing and rendezvous families can meet the accepted Interactive Route claim and application latency budget under churn, malicious Nodes, and realistic client devices? | Comparable primary-source analysis and bounded measurements against one R-001 claim matrix; no library popularity scoring. | open |
| R-007 | What availability does the core promise when a path fails or a Service is offline, and which retries can be performed without lying to the Application about operation completion? | A failure matrix for discovery, connect, partial write, route loss, service loss, and reconnect. Retained delivery and replicated content remain outside the core unless this research proves they are necessary. | open |
| R-008 | How are local Applications separated from endpoint authority, network metadata, and each other's Isolation Context while still supporting ordinary software? | A local trust-boundary and misuse contract comparing proxy, process API, embedded library, and reference-client responsibilities. This is not a decision to run arbitrary application code. | open |
| R-019 | What generic Application Data contract should include destination, online/offline delivery, reliability, ordering, retention, and Route Profile? | **Rejected as one question:** it mixed address lifecycle, live transport, storage, routing, and Application policy. Its decisions are now isolated in R-006, R-002, R-001, R-007, and R-008. | rejected |

## Resilience — makes the product viable in a hostile network

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-009 | How does a fresh or blocked endpoint obtain enough authenticated network state and replaceable entry paths without one permanently necessary bootstrap address or trust root? | Bootstrap, partition-detection, Bridge-distribution, probing-resistance, and recovery contract with explicit compromised-source behavior. | open |
| R-010 | Which local, anonymous, and bounded costs protect connection, discovery, rendezvous, and naming capacity from flooding and Sybil capture without a global User identity? | Resource-by-resource attack economics and admission controls, including accessibility and unlinkability costs. | open |
| R-011 | How does an endpoint estimate and avoid correlated control by operator, network, family, software supply chain, and jurisdiction without collecting a new User graph? | Selection inputs, uncertainty model, privacy-preserving measurements, and failure thresholds for the accepted route claim. | open |
| R-012 | What happens when naming, bootstrap, protocol releases, or emergency governance is captured, partitioned, or unavailable? | A Control Plane power map with quorum, transparency, expiry, recovery, and fork behavior for each root. | open |
| R-020 | Why will independent contributors provide and maintain useful network roles, and which incentive or public-goods models remain viable without making a token or one sponsor a security root? | A contributor journey, cost/capacity model, abuse incentives, concentration risks, and staged sustainability options that the current one-to-one project can actually operate. | open |

## Optional network extensions — require a concrete product need

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-005 | Does Ardents need a second, delayed or cover-traffic-heavy Route Profile for a named Application job, and can it provide a measurable advantage over the Interactive Route at an acceptable cost? | A real operation, observer claim, latency/bandwidth/cover budget, and comparison. `No second profile` is an acceptable answer. | open |
| R-021 | Do multiple distinct Applications require the same retained-delivery or replicated-content semantics strongly enough to justify a standard Overlay Service? | At least two complete Application journeys, retention/deletion/abuse metadata analysis, failure model, and proof that a live connection plus application storage is insufficient. | open |
| R-022 | Is any shared application identity, Credential, Contact, Space, or Capability model required at the network boundary rather than inside Applications? | Cross-application interoperability need, linkability analysis, recovery model, and a minimal boundary. `Application-owned identity only` is the default answer. | open |

## Technology — begins after the relevant contracts stabilize

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-013 | Which maintained protocols and implementations fit each accepted addressing, transport, routing, discovery, naming, and bootstrap contract? | Build/adopt map using specifications, security reviews, maintenance, license, interoperability, replacement cost, and misuse analysis. | open |
| R-014 | Which implementation language and runtime best fit the same accepted tracer, audited dependencies, memory safety, async networking, reproducible builds, target platforms, and the one-to-one project's capacity? | Comparable bounded prototypes and release/dependency evidence. It is not a Go-versus-Rust preference vote. | open |
| R-015 | What protocol-description, versioning, negotiation, and conformance strategy permits independent implementations without freezing immature semantics or enabling downgrade? | Evolution scenarios, conformance/fuzz prototype, canonicalization rules, and compatibility/deprecation contract. | open |

## Product validation — runs beside technical research

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-016 | Which Users and Developers have a problem severe enough to adopt an internal location-private network despite latency, installation, and trust costs? | Scenario and competitor comparison now; external demand evidence remains a future gate and must not be invented from the Product Owner's preferences. | open |
| [R-017](records/r-017-named-private-site-anonymous-mailbox.md) | Is Named Unlisted Site a useful smallest Reference Application for exercising publish, name, resolve, connect, and route failure without adding messenger semantics? | Selected as an architecture tracer. This does not validate market demand and no longer implies replicated Site Bundles or an Ardents runtime. | decided |
| R-018 | Can a User and Developer understand Service Name trust, connection state, route limits, failure, and recovery without learning routing jargon? | One-to-one walkthrough can refine wording; external comprehension evidence remains a future release gate. | open |

## Decision order

R-006 selected the portable-authority lifecycle. The remaining dependency path
is intentionally short:

1. **R-002 — live Application Interface:** define exactly what software can do.
2. **R-001 — Interactive Route claim:** define what protection that connection
   promises.
3. **R-003 — Service Name:** define the human layer over the accepted target and
   catastrophe replacement.
4. **R-004 and R-009 — routing and hostile bootstrap:** compare mechanisms only
   against the accepted contracts.
5. **R-007 and R-008 — failure and local isolation:** close the minimum tracer
   safety boundary.

R-010 through R-012 and R-020 run before any public deployment claim. Optional
extensions R-005, R-021, and R-022 do not block the live network tracer. R-013
then maps existing components, and R-014 compares languages using the same
contract. No production stack is selected earlier.
