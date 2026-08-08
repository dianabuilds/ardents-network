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
- a distinct Name Authority controls the Service Name binding, is unnecessary
  for ordinary publication or resolution, and remains separate from Service
  Authority so target compromise has a truthful rebinding path;
- V1 uses one canonical network-wide Namespace in which a complete Service Name
  has one meaning across honest compatible clients; local aliases and external
  namespaces never become silent resolution fallbacks;
- root canonical names use first-valid deterministic permissionless claims that
  create renewable time-bounded Name Leases for Name Authority; a parent may
  issue names only inside its subtree; bounded Anonymous Cost protects naming
  work without money, a global account, identity document, IP reputation, stable
  identity, wallet, token, or registrar and makes no personhood or fairness claim;
- canonical V1 Service Names are lowercase ASCII dot hierarchies with the parent
  on the right; `ardents://<Service Name>` is their explicit shareable form, not
  DNS, and Unicode, IDNA, Punycode, public-TLD lookup, and DNS fallback are absent;
- a Name Lease moves from Active through finite Grace to Released unless renewed;
  Grace resolves with a warning, Release and parent Release stop resolution, and
  every reclaim creates a new Name Generation that invalidates all earlier state;
- Name Authority rotation or transfer is an authenticated successor transition;
  optional recovery requires a precommitted delayed threshold Recovery Policy,
  and Recovery Pending fails closed without any administrative fallback;
- Private Resolution separates User location from the exact name/lookup view
  against one ordinary Node and across Isolation Contexts; names remain guessable,
  naming-side metadata and collusion remain visible limits, and no less-private
  resolver fallback exists;
- no administrator, project, registrar, legal or trademark claimant, or manual
  panel may seize, block, transfer, or reassign a canonical Name Lease; only a
  finite versioned technical reserved-name set exists, local filters do not alter
  canonical meaning, rules are inspectable, and incompatible forks are explicit;
- the Application Data primitive is an online logical Service Connection;
  replaceable transport-specific Carrier Channels carry it without defining its
  identity or Application semantics;
- the local interface uses finite hierarchical budgets, backpressure, and
  measured performance without weakening accepted security boundaries;
- required client and publisher profiles cap locally queued logical Application
  Data at `256 KiB` per connection and direction, with endpoint aggregates of
  `16 MiB` per client direction and `64 MiB` per publisher direction;
- the Interactive Route is a test-gated contract: endpoint, Local Traffic
  Observer, one-malicious-Node, active-attack, and explicit broad-observer
  boundaries are fixed, while no implementation has yet earned Route
  Qualification;
- V1 performance and release gates cover Windows 11 and Ubuntu LTS `x86-64`
  desktop/laptop endpoints for Users and Developers and an Ubuntu LTS `x86-64`
  `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS for
  infrastructure roles; other Linux variants receive no V1 claim, while macOS
  and mobile remain later targets;
- normal qualification uses a `100/20 Mbit/s` User access link, symmetric
  `100 Mbit/s` Publisher and infrastructure links, `80 ms` base RTT, independent
  `0.1%` loss per direction, and `p95 <= 10 ms` additional per-direction jitter
  below Carrier Transports;
- qualification validates fresh `32-byte` connection canaries, a nonce-bearing
  `512-byte` HTTP request with a complete `64 KiB` incompressible response, and
  distinct verified incompressible streams; HTTP remains only a tracer protocol;
- paired goodput baselines run directly for `60 seconds` before and after each
  Ardents batch; positive pairs within `max/min <= 1.10` use their arithmetic
  mean, while invalid or over-drift controls invalidate the complete batch;
- qualification distinguishes clean first start, routine restart, cold request,
  and warm request through a verified per-attempt state manifest; cold has no
  target-tuple state, while warm has no open connection or Application cache;
- every controlled cell freezes a versioned network manifest, generator, and
  ordered seed assignment before candidate results; it reproduces inputs below
  Carrier Transports rather than replaying a fixed packet trace;
- qualification uses same-host monotonic event clocks, raw unsmoothed one-second
  CPU/RSS/directional-carrier values, exact queue and security high-water
  evidence, whole-Ardents helper attribution, and controlled interface counters;
- every publicly claimed passing release exposes one immutable, content-addressed
  Qualification Evidence Bundle binding its exact candidate and conditions to
  complete raw results, invalidations, and a reproducible verdict;
- every changed candidate has a new qualification identity; bounded partial
  reruns require a pre-result impact scope, otherwise the complete matrix runs,
  and a comparable `10%` adverse KPI movement requires explicit release review;
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
| [R-002](records/r-002-live-application-interface.md) | What is the smallest live Application Interface that lets an existing local application publish and consume a Service safely? | **Decided:** an external socket/proxy-style boundary exposes one live logical Service Connection, both destination forms, authenticated results, honest failures, safe Isolation Contexts, endpoint-local least privilege, hierarchical resource budgets, backpressure, and measurable performance. The same stream may span bounded Carrier Channel replacement without becoming an Application reconnect. SDKs remain optional wrappers; concrete protocol remains later, while scenario-specific numeric budgets come from R-023 evidence. | decided |
| [R-001](records/r-001-interactive-route-claim.md) | Which endpoint, Local Traffic Observer, relay-collusion, and Broad Traffic Observer capabilities must the Interactive Route resist, and what does it deliberately expose? | **Decided:** the low-latency claim provides Endpoint Location Privacy against the opposite endpoint and any one malicious ordinary Node through multi-hop Route Knowledge Separation; it limits local observation without promising invisibility, excludes Broad Traffic Observer and arbitrary-collusion resistance, protects payload and exact target even under carrier collusion, fails closed under active attack, and requires Route Qualification before any implementation claim. | decided |
| [R-003](records/r-003-service-name-contract.md) | How does an exact Service Name bind to a Service Target, resolve without becoming a directory, survive accepted rotation, and handle registration, expiry, recovery, conflict, enumeration, query privacy, and Control Plane capture? | **Decided:** P4-D1 through P4-D6 fix distinct Name Authority, one canonical Namespace, permissionless leased claims, ASCII hierarchy and Service Link, lifecycle and generations, precommitted recovery, Private Resolution, non-administrative governance, finite technical reservations, bounded Anonymous Cost, local filtering, inspectable rules, and explicit forks. Registry, ordering, lookup, routing, cryptographic recovery, cost, and governance mechanisms remain downstream research. | decided |
| [R-004](records/r-004-routing-rendezvous-families.md) | Which routing and rendezvous families can meet the accepted Interactive Route claim and R-023 performance budget under churn, malicious Nodes, and realistic client devices? | **Active:** P5-D1 fixes a stable deep Route Module Interface so a stronger versioned profile can replace routing beneath the same Application Interface and Service Connection without rewriting naming, identity, publication, or Applications. Split-leg rendezvous is the front-runner Adapter, paired tunnel pools the alternative Adapter, full Tor-shaped circuits the security reference, and a delayed Nym-shaped mixnet is deferred to R-005. Exact profiles fail rather than downgrade, state is isolated by profile and context, and every Implementation qualifies independently. No route, library, cryptography, hop count, or language is selected. | active |
| R-007 | What availability does the core promise when a path fails or a Service is offline, and which retries can be performed without lying to the Application about operation completion? | A failure matrix that maps network evidence, including P2-D6 active violations, to the accepted P1-D5 classes for discovery, connect, partial write, route loss, service loss, and reconnect. It must preserve P3-D4a, P3-D4b1, P3-D4b2a, and P3-D4b2b same-connection recovery, sequential churn, impaired-live progress, and the non-resetting overlapping-failure deadline, distinguish degradation and recovery from opening a new connection, and use indeterminate failure where attack and outage cannot be distinguished. Retained delivery and Application-operation retry remain outside the core. | open |
| R-008 | How are local Applications separated from endpoint authority, network metadata, and each other's Isolation Context while still supporting ordinary software? | A local trust-boundary and misuse contract implementing Local Grants and Isolation Contexts, identifying every state forbidden to cross Applications, grants, or contexts, and testing deliberate or accidental reuse. This is not a decision to run arbitrary application code. | open |
| [R-023](records/r-023-interactive-route-performance-budget.md) | What end-to-end performance budget makes the V1 Interactive Route and Named Unlisted Site useful without weakening the accepted security contract? | **Active:** P3-D1 through P3-D5 fix useful performance, finite resources, recovery, admission, and hostile-work isolation. P3-D6a makes qualification conjunctive with hard security guardrails, and P3-D6b1 fixes the four cross-platform, two-direction controlled topology. P3-D6b2a fixes release sampling: normal short-event cells use `100` attempts and `>= 99%` success unless specifically overridden; recovery uses at least `20` episodes and `>= 95%` unless stricter; 10-minute workloads run five times, with `50` goodput windows and per-run resource gates. Nearest-rank percentiles retain failed latency as infinity and failed goodput as zero; smoke tests do not qualify. P3-D6b2b1 fixes frozen Windows 11 and Ubuntu LTS `x86-64` endpoint images on a `4 vCPU`, `8 GiB RAM`, SSD-backed, non-overcommitted base; Ubuntu LTS is the sole Linux qualification baseline and other variants receive no V1 claim. P3-D6b2b2a fixes the transport-independent normal envelope at `100/20 Mbit/s` User access, symmetric `100 Mbit/s` Publisher and Node links, `80 ms` base RTT, independent `0.1%` loss per direction, and `p95 <= 10 ms` additional jitter. P3-D6b2b2b fixes fresh `32-byte` connection canaries, an exact `512-byte` nonce-bearing HTTP tracer request with a complete `64 KiB` incompressible response, and verified pre-generated incompressible transfer streams. P3-D6b2b2c1 fixes verified `60-second` direct transfers before and after each applicable batch, a `max/min <= 1.10` drift bound, and their arithmetic-mean baseline. P3-D6b2b2c2a fixes verified clean, routine, cold, and warm state classes and rejects forbidden retained or cross-context state. P3-D6b2b2c2b1 fixes the versioned impairment manifest, generator, seed schedule, and no-fixed-packet-trace rule. P3-D6b2b2c2b2 fixes same-host monotonic clocks, raw one-second resource and traffic sampling, exact event and high-water evidence, complete process/helper charging, and controlled-boundary traffic attribution. P3-D6c1 fixes the immutable public Qualification Evidence Bundle binding exact candidate inputs and complete raw results to a reproducible verdict. P3-D6c2 fixes change-impact scoping, full and partial requalification, comparable-regression rules, and the explicit `10%` material-regression review trigger. Only role-specific Node capacity and cost remain deferred until R-004 evidence. | active |
| R-019 | What generic Application Data contract should include destination, online/offline delivery, reliability, ordering, retention, and Route Profile? | **Rejected as one question:** it mixed address lifecycle, live transport, storage, routing, and Application policy. Its decisions are now isolated in R-006, R-002, R-001, R-007, and R-008. | rejected |

## Resilience — makes the product viable in a hostile network

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-009](records/r-009-hostile-bootstrap-and-bridge-entry.md) | How does a fresh or blocked endpoint obtain enough authenticated network state and replaceable entry paths without one permanently necessary bootstrap address or trust root? | **Active:** compare Tor-like authority consensus, I2P-like independent reseeds, and a threshold-signed expiring epoch bundle distributed through package, cache, mirrors, peers, files, and optional third-party channels. The epoch-bundle plus expiring Bridge Invite is the prototype front-runner; R-012 still owns signer and fork power. No signer, quorum, broker, transport, DHT, or library is selected. | active |
| R-010 | Which local, anonymous, and bounded costs protect connection, discovery, rendezvous, and naming capacity from flooding and Sybil capture without a global User identity? | Resource-by-resource attack economics and admission controls, including accessibility and unlinkability costs. Publisher connection admission must meet P3-D5a and P3-D5b; any mandatory honest-client check is capped at one logical-core CPU-second, `64 MiB`, and `1 MiB` without money, account, IP or source reputation, stable identity, or cross-context linking. P3-D5c additionally fixes that an indistinguishable admitted Sybil can occupy finite capacity: the network must isolate established work and return explicit capacity by `15 s`, but does not promise per-person fairness or a free slot. The concrete anonymous mechanism remains open and cannot erase that non-claim by assumption. | open |
| R-011 | How does an endpoint estimate and avoid correlated control by operator, network, family, software supply chain, and jurisdiction without collecting a new User graph? | Selection inputs, uncertainty model, privacy-preserving measurements, and failure thresholds that reduce P2-D4 exposure and test actual Route Knowledge Separation rather than counting different Node IDs. | open |
| R-012 | What happens when naming, bootstrap, protocol releases, or emergency governance is captured, partitioned, or unavailable? | A Control Plane power map with quorum, transparency, expiry, recovery, and fork behavior for each root. It must preserve the P1-D7 invariant that no Endpoint Owner, sponsor, or operator is mandatory network-wide. | open |
| R-020 | Why will independent contributors provide and maintain useful network roles, and which incentive or public-goods models remain viable without making a token or one sponsor a security root? | A contributor journey, cost/capacity model, abuse incentives, concentration risks, and staged sustainability options that the current one-to-one project can actually operate. | open |

## Optional network extensions — require a concrete product need

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-005 | Does Ardents need a second, delayed or cover-traffic-heavy Route Profile for a named Application job, and can it provide a measurable advantage over the Interactive Route at an acceptable cost? | A real operation, stronger observer or collusion claim, latency/bandwidth/multipath/cover budget, and comparison. Any accepted profile reuses the Application Interface and Service Connection contract through the P5-D1 Route Module Seam, fails rather than downgrades, and earns separate Route Qualification. `No second profile` is an acceptable answer. | open |
| R-021 | Do multiple distinct Applications require the same retained-delivery or replicated-content semantics strongly enough to justify a standard Overlay Service? | At least two complete Application journeys, retention/deletion/abuse metadata analysis, failure model, and proof that a live connection plus application storage is insufficient. | open |
| R-022 | Is any shared application identity, Credential, Contact, Space, or Capability model required at the network boundary rather than inside Applications? | Cross-application interoperability need, linkability analysis, recovery model, and a minimal boundary. `Application-owned identity only` is the default answer. | open |

## Technology — begins after the relevant contracts stabilize

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-013 | Which maintained protocols and implementations fit each accepted addressing, Carrier Channel, routing, discovery, naming, and bootstrap contract? | Build/adopt map using specifications, security reviews, maintenance, license, interoperability, replacement cost, and misuse analysis. Carrier candidates are judged as part of a complete stack against the same logical Service Connection recovery contract rather than inheriting product semantics from HTTP, WSS, TCP, UDP, QUIC, or another mechanism. | open |
| R-014 | Which implementation language and runtime best fit the same accepted tracer, audited dependencies, memory safety, async networking, reproducible builds, target platforms, and the one-to-one project's capacity? | Comparable bounded prototypes and release/dependency evidence. It is not a Go-versus-Rust preference vote. | open |
| R-015 | What protocol-description, versioning, negotiation, and conformance strategy permits independent implementations without freezing immature semantics or enabling downgrade? | Evolution scenarios, conformance/fuzz prototype, canonicalization rules, and compatibility/deprecation contract. | open |

## Product validation — runs beside technical research

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-016 | Which Users and Developers have a problem severe enough to adopt an internal location-private network despite latency, installation, and trust costs? | Scenario and competitor comparison now; external demand evidence remains a future gate and must not be invented from the Product Owner's preferences. | open |
| [R-017](records/r-017-named-private-site-anonymous-mailbox.md) | Is Named Unlisted Site a useful smallest Reference Application for exercising publish, name, resolve, connect, and route failure without adding messenger semantics? | Selected as an architecture tracer. This does not validate market demand and no longer implies replicated Site Bundles or an Ardents runtime. | decided |
| R-018 | Can a User and Developer understand Service Name trust, connection state, route limits, failure, and recovery without learning routing jargon? | One-to-one walkthrough can refine wording; external comprehension evidence remains a future release gate. | open |

## Decision order

R-006 selected the portable-authority lifecycle, R-002 fixed the live
Application Interface, and R-001 closed the Interactive Route claim and its
qualification gate. The remaining dependency path is intentionally short:

1. **R-023 — performance budget:** the pre-architecture performance and
   qualification contract is fixed; role-specific Node capacity resumes only
   after R-004 supplies a candidate Route and units of work.
2. **R-003 — Service Name:** the product contract is fixed; compare candidate
   mechanisms only against P4-D1 through P4-D6.
3. **R-004 and R-009 — routing and hostile bootstrap:** compare mechanisms only
   against the accepted contracts.
4. **R-007 and R-008 — failure and local isolation:** close the minimum tracer
   safety boundary.

R-010 through R-012 and R-020 run before any public deployment claim. Optional
extensions R-005, R-021, and R-022 do not block the live network tracer. R-013
then maps existing components, and R-014 compares languages using the same
contract. No production stack is selected earlier.
