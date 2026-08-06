# Research queue

Status: **open**

This is the decision backlog for the first tracer product. Priority means “what
must be learned first,” not implementation order.

## Foundation — blocks architecture

| ID | Question | Decision unlocked | Required evidence | State |
|---|---|---|---|---|
| R-001 | What exact observer and relay-collusion models must Interactive and Shielded Routes resist? | Privacy claims, route contracts, test strategy | Formal claim matrix, attack analysis, simulation/measurement plan | open |
| R-002 | What is the safe V1 application boundary for a Site Bundle? | Static web/client runtime, sandbox, Capability surface | Representative apps, fingerprinting study, sandbox comparison, escape/update threat analysis | open |
| R-003 | How are global and scoped Service Names registered, resolved privately, recovered, expired, and disputed? | Naming constitution and recovery contract | Consistency and privacy analysis, governance options, query-linkability experiments, failure recovery | open |
| R-004 | Which routing families can satisfy the Interactive Route latency and endpoint-location contract? | Interactive carrier architecture | Primary protocol analysis, prototype measurements, churn/collusion behavior, mobile constraints | open |
| R-005 | Which routing families can satisfy the Shielded Route metadata contract at an affordable cover budget? | Higher-privacy Application Data carrier | Anonymity simulation, latency/bandwidth budgets, active attack analysis, operator diversity assumptions | open |
| R-006 | How do Recovery Root, Devices, Personas, relationships, revocation, and multi-device state coexist without creating a universal identity? | Identity and authorization state model | State-machine scenarios, compromise/recovery analysis, existing protocol comparison, usability testing | open |
| R-007 | What retention, replication, deletion, and recovery semantics keep Site Bundles available without stable metadata beacons, and can the same substrate safely retain Application Data? | Replica/storage boundary | Failure simulation, privacy analysis, capacity model, deletion and seizure behavior | open |
| R-008 | What Client isolation model can run untrusted private applications without creating a universal fingerprint? | Application runtime and permission UX | Sandbox threat analysis, API prototype, deterministic/fingerprinting tests, update model | open |
| R-019 | What generic Application Data contract should Ardents expose: destination, online/offline delivery, reliability, ordering, backpressure, retention, and Route Profile? | Network-versus-application responsibility and transport API | Representative non-messenger and messenger applications, state/failure matrix, metadata analysis, latency/storage budgets, capability walkthrough | open |

## Resilience — blocks hostile deployment

| ID | Question | Decision unlocked | Required evidence | State |
|---|---|---|---|---|
| R-009 | How can Clients bootstrap and obtain Bridges under blocking without one partitionable trust root? | Bootstrap and circumvention architecture | Censorship cases, multi-source design, probing resistance, partition detection | open |
| R-010 | Which local and anonymous costs limit spam, flooding, and Sybil capture without global identity? | Admission and abuse policy | Attack economics, accessibility impact, unlinkability analysis, adaptive-control experiments | open |
| R-011 | How will the network measure independent operators and route around correlated ownership, ASN, software, and jurisdiction? | Contributor selection and transparency | Topology model, ownership signals, adversarial declarations, privacy-preserving telemetry | open |
| R-012 | What happens when naming, bootstrap, releases, or emergency governance is captured or unavailable? | Control-plane constitution | Power map, quorum/failure scenarios, recovery and fork behavior, appeal/expiry model | open |

## Technology — begins after contracts stabilize

| ID | Question | Decision unlocked | Required evidence | State |
|---|---|---|---|---|
| R-013 | Which existing cryptographic protocols and maintained implementations cover each accepted identity, naming, Application Data, storage, and routing primitive? | Build/adopt map | Security reviews, API fit, maintenance history, license, interoperability and misuse analysis | open |
| R-014 | Which implementation language and runtime best fit audited dependencies, memory safety, async networking, reproducible builds, mobile/desktop targets, and contributor capacity? | Production language/runtime ADR | Two tracer prototypes, dependency audit, profiling, cross-build/release comparison, team learning cost | open |
| R-015 | What protocol-description and conformance strategy permits multiple implementations without freezing immature semantics? | Wire-format and compatibility policy | Evolution scenarios, fuzz/conformance prototype, canonicalization and downgrade analysis | open |

## Product validation — runs alongside technical research

| ID | Question | Decision unlocked | Required evidence | State |
|---|---|---|---|---|
| R-016 | Which first users have a problem severe enough to accept the latency and trust trade-offs? | Initial audience and distribution | Interviews, scenario tests, alternatives used today, willingness-to-switch evidence | open |
| R-017 | Is Named Private Site + Anonymous Mailbox the smallest slice that demonstrates differentiated value? | Architecture-tracer scope; later V1 candidate | Structured Product Owner walkthrough, incumbent comparison, traced failure/recovery; later external comprehension | decided |
| R-018 | Do people understand Persona, Service Name, permissions, recovery, and route guarantees without learning network jargon? | Client information architecture | Usability prototypes, recovery drills, permission and warning comprehension tests | open |

Under the current one-to-one working model, R-016 and the external-observation
part of R-018 remain open future gates. A Product Owner walkthrough may prepare
their scenarios but must not be recorded as participant or market validation.

## Decision order

R-017 selected the Named Unlisted Site tracer and rejected a built-in Inbox.
Resolve R-019 next, then R-002, R-003, R-006, and R-001 before selecting the
production language or transport. R-016 and R-018 remain visible but do not
block reversible architecture research while external participants are
unavailable. R-004, R-005, R-007, and R-008 may then produce bounded
experiments. R-014 compares implementation environments using the same accepted
tracer and criteria; it must not begin as a Go-versus-Rust preference debate.
