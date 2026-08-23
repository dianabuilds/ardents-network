# Network research queue

Status: **accepted cross-horizon decision registry; Stage 5 maintained
development completed on 2026-08-19; Stage 6 completed on 2026-08-20**

This backlog exists to design Ardents as a network product. A question belongs
here only when its answer changes an observable network contract, a security
claim, or a later technology comparison. It is not a list of every feature we
could build.

[Product scope and delivery horizons](../product/scope.md) is authoritative for
work order. Most decided rows below constrain a later Public Beta or Stable
Network claim and are not current implementation tasks. Gate B and Gate C are
complete. R-029 through R-031 are the decided sequential Horizon 3 vertical
slices; their deferred official qualification gates remain open. Accepted
R-032 authorizes S4.1–S4.3 recovery development. The Product Owner authorized
all S4.4 development on 2026-08-15; R-034 assigns Bridge-specific capacity to
Stage 5 while P3-D3b4 is frozen from measured Stage 4 role evidence.
Historical `V1` wording means the first public product contract, not the next
prototype.

## Accepted contracts across horizons

Research starts from these accepted product boundaries. Evidence may reopen a
decision explicitly, but an experiment or dependency cannot silently redefine it:

- Ardents addresses a Service Target, not a User or infrastructure Node;
- a tagged, versioned, network-bound Target Link provides the complete direct
  destination path without naming, origin reachability, or security downgrade;
- V1 uses one active Service Instance generation. The host generates a private
  Instance Key and portable Service Authority signs a public bounded Credential
  for that key; routine migration generates a new key, advances the generation,
  and preserves the target, while root loss or compromise replaces it through
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
- fresh authenticated ephemeral endpoint/session and per-leg keys provide
  Forward Secrecy for honestly completed connections against later Service/Node
  long-term-key compromise; live endpoint compromise, memory/snapshot remnants,
  and in-connection post-compromise healing remain non-claims;
- the local interface uses finite hierarchical budgets, backpressure, and
  measured performance without weakening accepted security boundaries;
- Local Grant revocation recursively blocks new work, closes custody/admin
  sessions immediately, and closes data immediately unless a finite drain was
  preselected; no ephemeral process/session bearer survives restart without
  fresh OS-local binding;
- a claim-bearing Local Grant binds to an OS-enforced or launcher-brokered
  Application Principal/process tree, not merely a desktop account, PID,
  loopback port, or copyable bearer. Indistinguishable same-user apps are one
  trust domain; malicious-sibling isolation requires platform qualification;
- carrier privacy covers only traffic submitted to Ardents. An Application-level
  location-privacy claim requires Network-Isolated Application Boundaries at
  both endpoint Applications: scoped local IPC/loopback only, no ordinary
  ingress/listeners or egress, per-context origin/storage, and no clearnet
  fallback. Generic adapters remain compatible but lack that claim;
- required client and publisher profiles cap locally queued logical Application
  Data at `256 KiB` per connection and direction, with endpoint aggregates of
  `16 MiB` per client direction and `64 MiB` per publisher direction;
- the Interactive Route is a test-gated contract: endpoint, Local Traffic
  Observer, one-malicious-Node, active-attack, and explicit broad-observer
  boundaries are fixed, while no implementation has yet earned Route
  Qualification;
- its independently hidden legs use stable disjoint Initiator, Rendezvous,
  Responder, and Introduction Role Domains. Endpoint selection uses proven
  Candidate Materializations under one authenticated logical Candidate View and
  never conditions an observable Service rejection on a hidden Entry Set;
- Name/Target/descriptor resolution is a private Destination Resolution role
  restricted to the non-adjacent Rendezvous Domain; its identities/known families
  are excluded from the same destination/context connection's Rendezvous. This
  preserves four domains, while a Node plus controlled endpoint can still perform
  active timing/volume confirmation and is an explicit non-claim;
- the first Carrier Lab family to prototype is a Tor-shaped pair of
  independently built endpoint circuits joined at a User-selected Rendezvous,
  with five data positions and a separate Introduction Path;
- Entry exposure is bounded to at most one small long-lived set for each
  activated installation × adjacent Role Domain × ordinary/Bridge regime:
  Initiator, Responder, or Introduction. Co-resident roles stay separate and one
  Bridge key proves one adjacent domain; Isolation Contexts separate every higher
  Route, destination, channel, key, session, and recovery state without forcing
  fresh Entry sampling;
- one threshold-authenticated expiring Network Epoch is authorized separately
  from its package, cache, mirror, peer, or file distribution paths. It commits a
  transparent input cutoff/root, logical complete Candidate View, length, and
  global summaries; endpoint materializations use indexed proofs, independent
  full auditors check global inclusion/completeness, and a distributor cannot
  tailor a valid subset or force silent resampling;
- direct pre-Route sources expose requester origin/public artifact. Globally
  advertised source identities/families are incompatible with Route/Resolution;
  ordinary candidates contacted directly enter a bounded Endpoint-wide
  exposure set and are excluded through derived-work expiry. Pre-contact retained
  state, sequences, retries, and growth are finite. Each mandatory artifact
  class has three beta/five stable effective authenticated source-only families
  under `40%`/`25%` caps; unauthenticated external distribution does not count;
- readiness has a Common Base and independent capability-specific prerequisites
  depending on Time Confidence; target connection, private name resolution,
  publication, and contribution never inherit one another's Entry path or hide
  behind one process-level boolean. Every live workload has a finite Work Safety
  Lease under its epoch, release, protocol/build, credential, and role bounds;
- V1 performance and release gates cover Windows 11 and Ubuntu LTS `x86-64`
  desktop/laptop endpoints for Users and Developers and an Ubuntu LTS `x86-64`
  `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS for
  infrastructure roles; other Linux variants receive no V1 claim, while macOS
  and mobile remain later targets;
- a qualified public Contributor uses a dedicated host/installation and an
  Endpoint excludes its own controlled Node identities/families. Client+Publisher
  co-residence is allowed without a same-host unlinkability claim, but standalone
  capacity floors are not additive and a combined profile must qualify;
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
- official installation and updates separate release authority from
  distribution, require `3-of-5` authorization for every public executable,
  protect version and rollback state, separate protocol transition from build
  safety/revocation, drain under signed finite deadlines, switch atomically, use
  explicit private-only/direct/offline retrieval modes, and never inherit Route
  Qualification across an untested build.

Each open question must either confirm, narrow, or reject one of those
boundaries with named evidence.

## Foundation — defines the network contract

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-006](records/r-006-service-target-lifecycle.md) | What is the public-product lifecycle of a Service Target across creation, publication, migration, loss, compromise, replacement, and retirement? | **Decided across horizons:** Carrier Lab uses ephemeral authenticated fixtures; Named Unlisted Site introduces one active Instance plus bounded Credential and ordinary migration; durable Authority recovery, compromise replacement, and retirement are later public-product lifecycle requirements. | decided |
| [R-002](records/r-002-live-application-interface.md) | What is the smallest live Application Interface that lets an existing local application publish and consume a Service safely? | **Decided:** an external socket/proxy-style boundary exposes one live logical Service Connection, both destination forms, authenticated results, honest failures, safe Isolation Contexts, endpoint-local least privilege, hierarchical resource budgets, backpressure, and measurable performance. The same stream may span bounded Carrier Channel replacement without becoming an Application reconnect. SDKs remain optional wrappers; concrete protocol remains later, while scenario-specific numeric budgets come from R-023 evidence. | decided |
| [R-001](records/r-001-interactive-route-claim.md) | Which endpoint, Local Traffic Observer, relay-collusion, and Broad Traffic Observer capabilities must the Interactive Route resist, and what does it deliberately expose? | **Decided:** the low-latency claim provides Endpoint Location Privacy against the opposite endpoint alone and role-local Route Knowledge Separation against one malicious ordinary Node with no endpoint/second observation under that adversary. Node-plus-endpoint active confirmation, Broad Traffic Observation, and arbitrary collusion are non-claims; bidirectional confirmation must still be characterized. Payload/exact-target protection survives carrier collusion, completed connections require Forward Secrecy, active integrity fails closed, and implementation claims require Qualification. | decided |
| [R-003](records/r-003-service-name-contract.md) | How does an exact Service Name bind to a Service Target, resolve without becoming a directory, survive accepted rotation, and handle registration, expiry, recovery, conflict, enumeration, query privacy, and Control Plane capture? | **Decided:** P4-D1 through P4-D6 fix distinct Name Authority, one canonical Namespace, permissionless leased claims, ASCII hierarchy and Service Link, lifecycle and generations, precommitted recovery, Private Resolution, non-administrative governance, finite technical reservations, bounded Anonymous Cost, local filtering, inspectable rules, and explicit forks. Registry, ordering, lookup, routing, cryptographic recovery, cost, and governance mechanisms remain downstream research. | decided |
| [R-004](records/r-004-routing-rendezvous-families.md) | Which routing and rendezvous families can meet the accepted Interactive Route claim and R-023 performance budget under churn, malicious Nodes, and realistic client devices? | **Decided candidate order:** test the Tor-shaped split-circuit package first, with five symmetric positions and separate Introduction. Role Domains, Candidate View, source exclusions, and endpoint-only continuity remain candidate constraints for later promotion, not current public subsystems. Carrier Lab may falsify the package; no production family is selected. | decided |
| [R-007](records/r-024-operational-product-closure.md#operation) | What availability does the core promise when a path fails or a Service is offline, and which retries can be performed without lying to the Application about operation completion? | **Decided:** only positive authenticated evidence narrows Service or Route failure; ambiguity is indeterminate. Carrier/leg/Rendezvous recovery is bounded inside the same connection and never replays an Application operation. | decided |
| [R-008](records/r-024-operational-product-closure.md#isolation-versus-cumulative-entry-exposure) | How are local Applications separated from endpoint authority, network metadata, and each other's Isolation Context while still supporting ordinary software? | **Decided:** Local Grants separate connection, Service operation, and Authority Custody with recursive revocation and fresh post-restart binding. Entry exposure is Endpoint × adjacent domain × regime bounded; all higher Route, destination, session, continuity, and diagnostics state is context-separated. | decided |
| [R-023](records/r-023-interactive-route-performance-budget.md) | What end-to-end performance budget makes the V1 Interactive Route and Named Unlisted Site useful without weakening the accepted security contract? | **Active evidence gate:** P3-D1 through P3-D5 fix useful performance, finite resources, recovery, admission, and hostile-work isolation. P3-D6a makes qualification conjunctive with hard security guardrails, and P3-D6b1 fixes the four cross-platform, two-direction controlled topology. P3-D6b2a fixes release sampling: normal short-event cells use `100` attempts and `>= 99%` success unless specifically overridden; recovery uses at least `20` episodes and `>= 95%` unless stricter; 10-minute workloads run five times, with `50` goodput windows and per-run resource gates. Nearest-rank percentiles retain failed latency as infinity and failed goodput as zero; smoke tests do not qualify. P3-D6b2b1 fixes frozen Windows 11 and Ubuntu LTS `x86-64` endpoint images on a `4 vCPU`, `8 GiB RAM`, SSD-backed, non-overcommitted base; Ubuntu LTS is the sole Linux qualification baseline and other variants receive no V1 claim. P3-D6b2b2a fixes the transport-independent normal envelope at `100/20 Mbit/s` User access, symmetric `100 Mbit/s` Publisher and Node links, `80 ms` base RTT, independent `0.1%` loss per direction, and `p95 <= 10 ms` additional jitter. P3-D6b2b2b fixes fresh `32-byte` connection canaries, an exact `512-byte` nonce-bearing HTTP tracer request with a complete `64 KiB` incompressible response, and verified pre-generated incompressible transfer streams. P3-D6b2b2c1 fixes verified `60-second` direct transfers before and after each applicable batch, a `max/min <= 1.10` drift bound, and their arithmetic-mean baseline. P3-D6b2b2c2a fixes verified clean, routine, cold, and warm state classes and rejects forbidden retained or cross-context state. P3-D6b2b2c2b1 fixes the versioned impairment manifest, generator, seed schedule, and no-fixed-packet-trace rule. P3-D6b2b2c2b2 fixes same-host monotonic clocks, raw one-second resource and traffic sampling, exact queue/security high-water evidence, complete process/helper charging, and controlled-boundary traffic attribution. P3-D6c1 fixes the immutable public Qualification Evidence Bundle; P3-D6c2 fixes requalification and the `10%` review trigger. The controlled tracer additionally hard-gates both endpoint Application network boundaries, Application Principal sibling isolation, Direct-Origin Source exposure, and Role Domain reassignment. Capability-specific startup, effective post-exclusion Node capacity/cost, the Client+Publisher combined profile, and anonymous cost-to-deny remain evidence-driven after R-013 prototypes. | active |
| [R-028](records/r-028-h3-runtime-resource-contract.md) | What finite CPU, memory, socket, goroutine, queue, GC, overload, and shutdown contract lets a Horizon 3 process fail explicitly instead of exhausting its host? | **Decided only as R-029's resource/evidence appendix:** use one fixed Ubuntu `2 vCPU`/`2 GiB` H3-S containment profile, exact Endpoint/distributor work units, cgroup-v2 and OS fuses, explicit Go runtime settings, reservation before allocation, bounded memory/disk/FD/timer/queue ownership, fault-specific verdicts, and external one-second accounting. Do not reuse endpoint floors or turn this lab workload into Node capacity. | decided |
| R-019 | What generic Application Data contract should include destination, online/offline delivery, reliability, ordering, retention, and Route Profile? | **Rejected as one question:** it mixed address lifecycle, live transport, storage, routing, and Application policy. Its decisions are now isolated in R-006, R-002, R-001, R-007, and R-008. | rejected |
| [R-024](records/r-024-operational-product-closure.md) | Is the complete product lifecycle coherent across installation, warm-up, operation, security, performance, updateability, and privacy, and which unresolved points are real bottlenecks? | **Decided:** the operating model closes capability readiness, Time Confidence, Target Links, route domains, entry exposure, recovery attachment, diagnostics, update, Control Plane, contribution, and public-launch gates. Remaining gaps are explicit feasibility, technology, operator-supply, or external-evidence gates. | decided |

## Resilience — makes the product viable in a hostile network

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-009](records/r-009-hostile-bootstrap-and-bridge-entry.md) | How does a fresh or blocked endpoint obtain enough authenticated network state and replaceable entry paths without one permanently necessary bootstrap address or trust root? | **Decided:** accept a threshold-authenticated expiring Network Epoch whose identical bytes may arrive through package, cache, mirrors, peers, or files, plus Bridge Invites bound to exactly one Initiator/Responder/Introduction adjacent Role Domain. Authorization and distribution are separate; exact components remain R-013. | decided |
| [R-027](records/r-027-h3-first-slice.md) | What is the smallest first Horizon 3 vertical slice that advances the product without silently selecting production foundations or claiming independence the one-to-one project cannot prove? | **Decided only as R-029's bootstrap appendix:** the source, Epoch, persistence, freshness, conflict, and evidence mechanics remain inputs to R-029. Its standalone synthetic-fixture recommendation and implementation order are superseded and withdrawn because they have no real product state consumer. | decided |
| [R-029](records/r-029-h3-authenticated-node-lifecycle.md) | What integrated first H3 slice joins authenticated state to a real Node lifecycle without prematurely selecting the Route? | **Decided Stage 1:** a bounded canonical Candidate View and Materialization control separately keyed Node processes through admission, role-probe work, refresh, drain, withdrawal, restart, and reassignment under exact resource/evidence gates. R-027/R-028 are appendices; the role probe is replaceable and proves no Route, capacity, anonymity, independence, or decentralization claim. | decided |
| [R-030](records/r-030-h3-real-multi-node-route.md) | Can the first Stage 2 tracer build an endpoint-chosen multi-process Route from authenticated eligible Node material without direct/short fallback or complete protected binding at one Node? | **Decided Stage 2:** begin development after the current local Stage 1 `short` pass and green `make check`; retain official Ubuntu `short`, current `churn-2h`, and independent `unattended-24h` as deferred gates before the integrated H3 verdict or stronger claim. Use four distinct role-domain processes, nested authenticated laboratory carrier legs, an end-to-end protected unpredictable canary, role-local evidence, bounded cleanup, and `pass|fail|invalid`. No production transport or public protocol is selected. | decided |
| [R-031](records/r-031-h3-service-connection-application-interface.md) | Can external client and publisher Applications use scoped OS-local IPC to exchange opaque bytes through an exact-Target-authenticated Service Connection over the Stage 2 Route, including one routine Instance migration, without receiving Ardents routing, publication, credential, or retry logic? | **Decided Stage 3 tracer:** the current clean committed Stage 2 local Docker result (`95/95`, digest `bcfd00c4e44c501dcc31be103699c4e4474eb8773e243ec68822ac00a036dfb1`) permits bounded Stage 3 development only. Freeze separate Connection, Service Administration, and external Authority Custody processes; one Target, exclusive generations 1 then 2, scoped launcher-brokered principals, `64 KiB` in each direction plus fresh `32-byte` canaries, exact authentication before success, finite backpressure/cancellation semantics, real four-position Route use, independent evidence, and no direct/ambient fallback. Official Stage 1/2 qualification and every privacy/release claim remain deferred. | decided |
| [R-032](records/r-032-h3-same-connection-recovery.md) | Can Stage 4 preserve one live exact-Target Service Connection across eligible Carrier, leg, and Rendezvous failure without Application reconnect, operation replay, security downgrade, or unbounded recovery state? | **Decided Stage 4 recovery contract:** the Service Connection Module owns endpoint-only continuity, connection-level sequence/acknowledgement, bounded replay, non-resetting deadlines, and terminal results while the Route Module supplies fresh bounded Route Attachments. Exercise Carrier repair, same-Rendezvous leg replacement, fresh Introduction/Rendezvous, sequential/overlapping failure, impairment, and cleanup under the existing R-023 gates. One retained Stage 3 local campaign passed `27/27` at commit `6c8faf9`; an independent verifier replay of the same frozen bundle also returned `27/27`. The Product Owner authorized S4.1–S4.3 on 2026-08-13. Recovery measurements are only inputs to P3-D3b4; the complete bounded R-013 role prototype and separate acceptance remain prerequisites for S4.4. | decided |
| [R-033](records/r-033-h3-stage-5-research-map.md) | What is the exact Horizon 3 Stage 5 scope and decision order? | **Decided:** limit H3 to authenticated offline/file Invite import, one finite Initiator-domain Bridge Entry Set plus cross-domain negatives, one bounded ordinary-to-Bridge transition, one replaceable Camouflage Adapter seam exercised by exactly Lyrebird obfs4 and standalone WebTunnel before one maintained selection, and one controlled blocked-entry campaign. The Product Owner accepted the scope and narrow R-036 comparison ordering exception on 2026-08-15. Public distribution, general routing, all-leg camouflage, Shielded traffic analysis, production UX, and public claims remain excluded. | decided |
| [R-035](records/r-035-h3-bridge-state.md) | What exact H3 Bridge Invite, finite Bridge Entry Set, exposure, regime-change, restart, expiry, replacement, and one-Role-Domain state machine preserves the R-009/ADR-0005 contract? | **Decided:** use one two-slot Initiator-domain Bridge Entry Set, at most one explicit replacement per slot/Epoch, one owner-approved or precommitted ordinary-to-Bridge transition, at most two contacts per member, one durable non-resetting attempt deadline, and fail-closed restart with retained exposure. The Product Owner accepted the state contract on 2026-08-15. R-036 owns candidate configuration; R-037 owns numeric clocks and evidence. | decided |
| [R-037](records/r-037-h3-blocked-entry-evidence.md) | Which controlled address-blocking, protocol-allow-list, active-probe, Invite-replay, withholding, role-conflict, restart, resource, and cleanup cells determine Stage 5 `pass|fail|invalid`? | **Decided and handed to final qualification:** the Product Owner accepted option O1 and `h3-s5-b1-v1` on 2026-08-16 and completed maintained Stage 5 development on 2026-08-19. The profile and all thresholds remain frozen; its complete 594-episode suite (564 candidate cells plus six five-episode evidence-integrity campaigns) and long-sustained run qualify the cleaned integrated candidate in S9.6 rather than blocking Stage 5 development closure. No qualification verdict has been claimed. | decided |
| [R-039](records/r-039-h3-private-naming-lifecycle.md) | What exact bounded production slice for Service Name lifecycle is required to prepare Stage 6 and what evidence/verifier split is mandatory before code coupling? | **Accepted and complete:** encoding, lifecycle, delegation, authority/recovery, Target continuity, private resolution and control-operation separation, conflicts/forks, abuse bounds, and independent `pass|fail|invalid` verdicting are implemented. The bounded S6E1 command campaign passed and the Product Owner recorded Stage 6 `complete` on 2026-08-20. | accepted |
| [R-040](records/r-040-h3-stabilization-closure.md) | How does Horizon 3 become one clean, technically documented, reproducibly verified baseline before Horizon 4? | **Accepted intent; ordering superseded by R-058:** cleanup, documentation reduction, and test/infrastructure stabilization remain mandatory, but Stage 8 now owns every planned change and Stage 9 only qualifies the frozen result. | superseded-in-part |
| [R-058](records/r-058-h3-reassessment-and-closure.md) | How should Horizon 3 reassess the product and development method, restructure the repository, reduce active documentation, and qualify one final candidate without paying for a full pre-refactor qualification twice? | **Accepted:** Stage 8 owns evidence-led reassessment and every planned product/process/code/test/document change; Stage 9 accepts only a frozen candidate and owns final qualification and closure. | accepted |
| [R-059](records/r-059-network-state-missing-current-recovery.md) | When a non-virgin authenticated Network State root lacks its `current` pointer, must restart fail closed or may it reconstruct one verified generation chain? | **Accepted:** retain R-027's fail-closed pointer rule; a unique-looking chain is insufficient authority to activate state. M3 must expose recovery-required without writing a replacement pointer. | accepted |
| [R-060](records/r-060-domain-owned-persistence-and-commitments.md) | May Network State and Namespace share a persistence or Merkle-commitment foundation without importing one domain's authority into the other? | **Accepted:** no shared domain foundation. Each target Module owns its persistence/commitment representation; duplication remains local unless one independently justified invariant owner emerges. | accepted |
| [R-061](records/r-061-domain-ownership-transfer-order.md) | In which order may M3 and M5 remove the accidental Network-owned persistence and Merkle implementations without leaving Namespace dependent on them or introducing a shared foundation? | **Accepted S8.5 sequencing:** transfer the existing Namespace mechanics into its current owner with characterization and no Network writer, then M3 moves/deletes Network mechanics; the accepted S8.4 plan records this explicit prerequisite. | accepted |
| [R-062](records/r-062-resource-platform-scope.md) | Which supported-platform resource adapters can truthfully enforce and measure the retained H3 profiles, and how must every unsupported platform fail closed? | **Accepted H1:** Linux cgroup-v2 plus rlimit is the only supported runtime adapter; every other platform returns an explicit refusal before production readiness and a protected drain on default observation. In-repository injected callbacks are behavior-test seams, not adapters. A native adapter requires new measured evidence before widening this scope. | accepted |
| [R-063](records/r-063-release-root-transaction-boundary.md) | When a consecutive release root is fully threshold-verified but later timestamp, snapshot, targets, or executable checks fail, which durable facts may survive restart? | **Accepted H1:** add the exact independently verified consecutive root archive while preserving any prior complete metadata floors unchanged; publish no new timestamp/snapshot/targets floor on rejection. Accepted metadata may advance floors atomically; no-update retains them without a write. | accepted |
| [R-064](records/r-064-h3-update-tracer-scope.md) | What update lifecycle may Stage 8 retain while Horizon 3 remains a closed technical slice? | **Accepted H1:** M2 may retain and transfer one bounded offline Update tracer, including its release authorization, local staging, journal, recovery, rollback, and terminal-result tests. It is not a supported installer, activation, custody, or public command surface; the C2 command observer expires in M13. | accepted |
| [R-065](records/r-065-naming-decision-time.md) | Which decision-time rule makes signed Name recovery and lifecycle freshness replay-safe? | **Accepted H1:** one Gateway decision time yields explicit seconds for Lease and milliseconds for signed Policy/Recovery boundaries. Recovery initiation accepts its signed start only within the existing finite admission lifetime; exact Gateway-millisecond equality is rejected. | accepted |
| [R-066](records/r-066-namespace-tracer-envelope.md) | What bounded Namespace resource envelope may Stage 8 retain without claiming product scale? | **Accepted H1:** retain only a measured 127-record, one-writer local technical tracer with a 4,096-byte proof bound and eight local concurrent exact-name readers. The former 4,096-record implementation constant has no authority; no product-scale claim is made. | accepted |
| [R-067](records/r-067-naming-profile-retention.md) | Which existing naming bytes are preserved during Stage 8 Namespace refactoring? | **Accepted H1:** retain canonical Name V1, Record/Recovery transcript semantics, fixed private-resolution envelopes, and threshold materialization semantics; internal claim, receipt, durable-chunk, and command-tracer encodings have no named observer and are C0 deletion targets. | accepted |
| [R-068](records/r-068-name-record-validity-migration.md) | How must a Target/Record validity limit become signed, durable, and resolution-enforced without weakening the accepted naming signature profile? | **Accepted H1:** Record V4 appends a signed millisecond `RecordNotAfter`; new publish and recovery-resume transitions require it within the effective own/parent Lease boundary. V3 is decode-only migration input and produces no resolvable Target binding until replaced by V4. The Ed25519 algorithm, record transcript domain, and signed-container framing remain unchanged. | accepted |
| [R-069](records/r-069-namespace-submission-result.md) | What can a private naming control result honestly assert before threshold Epoch installation? | **Accepted H1:** control returns only `submitted` or `denied`; `submitted` means the Gateway accepted one bounded volatile pending transition and is neither a signed Record nor current Namespace state. Generation, revision, state bytes, and user-visible success are reserved for a separately verified current materialization proof. | accepted |
| [R-070](records/r-070-namespace-pending-successor-record.md) | Which authenticated artifact can bridge submitted Namespace control to current materialization? | **Accepted H1:** a control submission carries an Authority-signed exact successor Record. Namespace validates and durably records the authenticated transition/successor pair as pending; only a threshold Epoch installation selected from that journal may advance current state. Gateway never invents a durable Record. | accepted |
| [R-071](records/r-071-typed-epoch-claim-winner.md) | Which typed fact may carry an accepted root claim from Epoch verification to Namespace materialization? | **Accepted H1:** `ClaimOrder.Verify` yields an opaque `ClaimWinner`; only that fact may materialize a root claim, deriving lifecycle fields from the winner, installed predecessor, materialization time, and policy rather than a late raw proof or caller-built operation. | accepted |
| [R-072](records/r-072-namespace-epoch-installation.md) | How may verified Namespace Epoch inputs become one threshold-current materialization without a caller-built Record corpus? | **Accepted H1:** an opaque Store-owned Epoch installation begins at verified current state, selects only the durable pending prefix, materializes `ClaimWinner` only through an exact Record signing port, and commits through the existing threshold statement. | accepted |
| [R-073](records/r-073-record-proof-envelope.md) | What signed Name Record envelope remains compatible with the retained fixed current-proof profile? | **Accepted H1:** the complete 127-record/16-signature worst path fits 1,996 signed Record bytes exactly; retain a 1,846-byte payload / 1,920-byte signed container limit, leaving 76 bytes of proof headroom and rejecting oversize before signing. | accepted |
| [R-074](records/r-074-epoch-claim-close-owner.md) | Which selected owner, if any, may accept opaque Namespace claim inputs and issue the complete R-042 threshold Epoch close without recreating a shared Network/Namespace foundation? | **Accepted H3:** no maintained global-close producer exists or is selected in Stage 8. Namespace retains only local opaque input admission and verification of a supplied complete close; it must not assemble a global close, and Network State must not absorb Namespace semantics without a separate protocol/owner decision. | accepted |
| R-041 | What exact canonical Service Name label length, total length, hierarchy depth, canonical encoding, and schema version freeze the parser, encoder, and Service Link behavior in S6.0 without introducing Unicode, IDNA, Punycode, public-TLD lookup, or DNS fallback? | **Decided (ASCII strict):** label 1–63 lowercase ASCII, total textual form ≤ 253 bytes, depth ≤ 127, no all-numeric root, textual `ardents://<Service Name>` link, and a distinct length-prefixed `schema_version = 1` wire form. See [r-041-canonical-name-limits.md](records/r-041-canonical-name-limits.md). | decided |
| R-042 | Which authenticated deterministic claim ordering and ordered-collision proof distinguishes ordered collision from unresolved Conflict, partition, and rule Fork in S6.0, and what is the minimum evidence an honest endpoint must produce to name a deterministic loser? | **Decided:** O1b uses Network Epoch commit/reveal, threshold-authenticated complete input/rejection roots, and lowest eligible input ordinal with a 32-claim per-Name cap. Incomplete evidence is unavailable; equivocation or incompatible rules are fork. The Product Owner accepted O1b and ADR-0017 on 2026-08-20. See [r-042-claim-ordering.md](records/r-042-claim-ordering.md). | decided |
| R-043 | Which exact persistence, restart, rollback, and cache-proof ownership boundary lets the Stage 6 Name Lease/Generation/Record state survive restart, fail closed on tamper, and remain replay-bounded without selecting a storage engine, consensus mechanism, or trust root by inertia? | **Decided and implemented (semantic storage boundary):** durable, restart-derived, and cache-bounded state fail closed on tamper/stale epoch and publish atomically through the naming-owned store over `internal/network/store`; no storage engine or consensus mechanism is selected. See [r-043-persistence-restart-rollback.md](records/r-043-persistence-restart-rollback.md). | decided |
| R-044 | Which maintained mechanism and complete protocol implement the accepted distinct `t`-of-`n` Recovery Authority trust model? | **Decided:** O2 uses `2 <= t <= n <= 8` distinct standard-library Ed25519 signatures, excludes the current Name Authority, binds generation/policy/operation/deadline, and delays recovery plus policy change for 72 hours–30 days. The Product Owner accepted O2 and ADR-0018 on 2026-08-20; ADR-0013 remains withdrawn. See [r-044-cryptographic-suite.md](records/r-044-cryptographic-suite.md). | decided |
| R-045 | Which R-010-compatible finite, measurable Anonymous Cost and local admission profile protects name claim, renewal, resolution, policy, and recovery surfaces from observation copying, front-running, flooding, rollback, and equivocation without money, account, IP reputation, stable identity, wallet, token, or personhood claim? | **Decided:** measured O1b uses scoped single-use HMAC/SHA-256 challenges with work bits `16/16/17/18`, spent caps `4096/2048/1024/1024`, in-flight caps `64/32/16/8`, and TTL at most 30 seconds. It is a local amplification guard, not Sybil resistance. The Product Owner accepted O1b and ADR-0019 on 2026-08-20. See [r-045-anonymous-cost.md](records/r-045-anonymous-cost.md). | decided |
| R-046 | Which exact field-level role matrix freezes resolution and naming control-operation inputs, observations, stable identifiers, Role Domains, known-family exclusions, and Isolation Context boundaries so no one ordinary role receives both endpoint location and exact name/lookup view? | **Decided:** use four execution roles plus a bounded observer, strict role-local fields, context-local caches/sessions, fresh retries, Rendezvous-domain resolution, known-family exclusions, commitment-only evidence, and no less-private fallback. The Product Owner accepted O1 on 2026-08-20. See [r-046-role-matrix.md](records/r-046-role-matrix.md). | decided |
| R-047 | Which narrow cryptographic profile authenticates Name Records and hides S6.2 private naming exchanges without selecting threshold Recovery Policy by inertia? | **Decided:** use Go 1.26.5 standard-library Ed25519 over exact domain/network/canonical-record transcripts plus the measured R-026 `openpcc/ohttp v0.0.80` profile and raised closure. The Product Owner accepted O1 and ADR-0014 on 2026-08-20. See [r-047-stage-6-query-hiding.md](records/r-047-stage-6-query-hiding.md). | decided |
| R-055 | Which canonical manifest, observation, cleanup, and verdict serialization lets a separately built verifier establish Stage 6 development completion without trusting runner summaries? | **Decided and implemented:** S6E1 uses strict canonical Go-struct JSON indexes, ordinal-derived stream paths, SHA-256 commitments, disjoint roots, one deterministic episode for every A0-D6 cell, and independent structural/behavioral mutation verification. The bounded command campaign passed on 2026-08-20. See [r-055-stage-6-evidence-serialization.md](records/r-055-stage-6-evidence-serialization.md). | decided |
| R-057 | How does private resolution authenticate the current Namespace without returning an unbounded parent chain? | **Decided and implemented:** a threshold-signed Network Epoch materialization commits the complete current Record/transition/rejection state; resolution returns one compact Merkle proof. S6E1 independently recomputes the complete corpus and measures the maximum-depth proof at `1667` bytes inside the fixed `4096`-byte response. Captured-threshold censorship/fork remains explicit. See [r-057-current-namespace-materialization.md](records/r-057-current-namespace-materialization.md). | decided |
| [R-048](records/r-048-h3-stage-7-contract.md) | What exact bounded contract and decision order makes H3 Stage 7 implementable after Stage 6 without selecting installer, updater, custody, IPC, or isolation foundations by inertia? | **Decided:** use distinct Release Decision, Update Transaction, Install Lifecycle, Authority Custody, Application Broker, and Application Isolation Modules with narrow Ubuntu/Windows Adapters. The Product Owner accepted the complete documentation/limitation package and authorized `start S7.1` on 2026-08-20. | decided |
| [R-049](records/r-049-stage-7-release-verifier.md) | Which maintained TUF-compatible Go verifier and exact bounded H3 metadata profile meet ADR-0006 without first-party update cryptography or a distributor authority? | **Decided:** select `go-tuf/v2 v2.4.2` behind one Release Decision Module with raised `x/crypto`/`x/sys`/`x/term`, no cache/network/repository/signing/multi-repository/delegation surface, and the measured finite profile; reject the decayed legacy fork. | decided |
| [R-050](records/r-050-stage-7-install-update-adapters.md) | Which Ubuntu/Windows Installed lifecycle Adapters and minimal Portable executable artifacts meet ADR-0015? | **Decided for development:** Ubuntu `.deb`, Windows WiX v7 MSI, and the same authenticated executable released directly as Portable implement ADR-0015. Portable has no lifecycle Adapter. Ubuntu Docker/current-Windows limitations are accepted; Windows installation and registration still require a separate Product Owner command naming artifact and mutations. | decided |
| [R-051](records/r-051-stage-7-application-principal.md) | Which Ubuntu and Windows local-channel and process-tree facts securely bind a launcher-born Application Principal rather than a PID, path, desktop user, or bearer? | **Decided:** ADR-0016 selects O2—private inherited channel, non-reusable root handle, and complete cgroup/Job tree. Named endpoints remain generic and direct binary has claim `none`. The exact bounded Windows `unsafe.Pointer` bridge is accepted with dedicated risk tests; unavailable native qualification remains deferred. | decided |
| [R-052](records/r-052-stage-7-application-isolation.md) | Which stable Ubuntu and Windows mechanisms can deny ordinary networking for a complete Application/helper process tree with scoped Ardents IPC and bounded privilege? | **Decided:** ADR-0016 selects non-setuid bubblewrap `v0.11.2` around the Ubuntu cgroup/pidfd tree and an ephemeral zero-capability AppContainer inside the Windows Job. Stage 7 isolated browser is explicitly unsupported; generic browser remains usable/unverified. Scheduled native evidence is still conjunctive. | decided |
| [R-053](records/r-053-stage-7-authority-recovery.md) | Which maintained cryptographic and platform-storage profile protects Authority Vault and Recovery Bundle state across update, export, test restore, reconciliation, and purge? | **Decided:** ADR-0021 selects strict `ardents-authority-envelope-v1`, Argon2id via `x/crypto v0.52.0` at `256 MiB/t=3/p=4`, and Go AES-256-GCM for separately purposed/passworded Vault records and Bundles. Explicit password/loss semantics and weakest-native-host/durability deferrals are accepted; scheduled S7.2 evidence remains required. | decided |
| [R-054](records/r-054-stage-7-evidence-profile.md) | Which canonical Stage 7 manifest, evidence, cleanup, and verdict profile independently proves install/update/principal/isolation behavior on both platforms? | **Decided:** accept S7E1 strict canonical JSON, four disjoint authorities, confined paths, SHA-256 commitments, the 91-cell/392-episode reference inventory, bounded resources, paired controls, and conjunctive scheduled `pass\|fail\|invalid`. Every run freezes exhaustive scheduled/pending/deferred coverage; deferred is never pass. | decided |
| [R-056](records/r-056-stage-7-desktop-browser-integration.md) | Which bounded direct-binary and desktop/browser Application Adapters open explicit Ardents destinations without changing system DNS, routes, default proxy/browser, or active VPN policy, and which profile can be claim-bearing? | **Decided:** O1 keeps one `connect`/`accept`/`browse` executable first-class in Installed/Portable. Generic browser uses an ephemeral numeric-loopback Adapter and remains unverified; isolated browser is unsupported. No extension, native host, proxy, browser bundle, DNS/route/VPN mutation, or fallback is selected. Windows registration still requires separate authorization. | decided |
| [R-010](records/r-024-operational-product-closure.md#security) | Which local, anonymous, and bounded costs protect connection, discovery, rendezvous, and naming capacity from flooding and Sybil capture without a global User identity? | **Decided product boundary:** resource-specific staged admission uses cheap stateless validation before expensive bounded state and may add scoped short-lived capabilities or bounded puzzles under load. No money, account, IP reputation, stable identity, personhood, or fairness claim. Exact mechanisms remain experiments. | decided |
| [R-011](records/r-024-operational-product-closure.md#security) | How does an endpoint estimate and avoid correlated control by operator, network, family, software supply chain, and jurisdiction without collecting a new User graph? | **Decided product boundary:** endpoints select locally from proven Candidate Materializations under one logical complete authenticated View; independent full auditors verify transparent inclusion and global summaries. Identity, Role Domain, and known family are hard constraints, while network/provider/jurisdiction/supply-chain evidence drives uncertainty and concentration gates without uploaded User route history. | decided |
| [R-012](records/r-024-operational-product-closure.md#security) | What happens when naming, bootstrap, protocol releases, or emergency governance is captured, partitioned, or unavailable? | **Decided product boundary:** separate threshold roots, expiring delegation, transparency, rollback watermarks, delayed rotation, narrow emergency power, and explicit forks. Project-only keys define a centralized test network, not public decentralization. | decided |
| [R-020](records/r-024-operational-product-closure.md#public-launch-gates) | Why will independent contributors provide and maintain useful network roles, and which incentive or public-goods models remain viable without making a token or one sponsor a security root? | **Decided for V1:** opt-in volunteer or institution-funded contribution, no token or payment system. Beta needs at least three effective families and stable five in every domain/subrole **after** the profile's maximum local exclusion union, plus capacity reserve. Route-family floors are `Σ_d(3+x_d)`/`Σ_d(5+x_d)`. Each mandatory pre-Route artifact class separately needs three/five authenticated source-only families under the same concentration cap; the same families may cover classes and count once. Thus `15`/`25`, not `12`/`20`, are the all-zero-exclusion theoretical infrastructure floors, and actual capacity may demand more. Insufficient supply blocks launch rather than weakening the Route. | decided |

## Optional network extensions — require a concrete product need

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-005](records/r-024-operational-product-closure.md#performance) | Does Ardents need a second, delayed or cover-traffic-heavy Route Profile for a named Application job, and can it provide a measurable advantage over the Interactive Route at an acceptable cost? | **Deferred to Horizon 5:** the first public product contract retains only the qualified Interactive Route claim. Do not reopen the complete security/privacy model or add Shielded implementation work before Public Beta is complete. Horizon 5 starts from Beta operational and Qualification evidence and requires a new product case, budget, adversary claim, and independent Qualification. Keep only the existing Route Module seam. | deferred |
| [R-021](records/r-024-operational-product-closure.md#disposition-of-the-research-queue) | Do multiple distinct Applications require the same retained-delivery or replicated-content semantics strongly enough to justify a standard Overlay Service? | **Rejected for V1:** no retained delivery, replicated content, or implicit storage enters the carrier. A future Overlay needs two complete journeys and its own retention, deletion, abuse, and metadata contract. | rejected |
| [R-022](records/r-024-operational-product-closure.md#disposition-of-the-research-queue) | Is any shared application identity, Credential, Contact, Space, or Capability model required at the network boundary rather than inside Applications? | **Decided:** Application-owned identity only. Ardents adds no network User identity, Contact, Space, global Persona, or cross-Application credential model. | decided |

## Technology — begins after the relevant contracts stabilize

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| [R-013](records/r-013-carrier-lab-technology-candidates.md) | Which maintained protocols and implementations fit each accepted addressing, Carrier Channel, routing, discovery, naming, and bootstrap contract? | **Carrier Lab milestone complete; broader technology selection remains active:** official Ubuntu run `31404126248` completed the frozen Direct/C-3/C-5/C2, seven-negative, resource, cleanup, and isolated Tor/Chutney sequence. Its conjunctive decision is `advance`, so the native split-circuit shape may inform a controlled next slice. It is not Route Qualification or an anonymity claim. R-075 rejects promoting the evaluated H3/external candidates; R-076 separately selects a new native Profile without retaining their wire bytes. | active |
| [R-075](records/r-075-route-carrier-foundation-selection.md) | Can Stage 8 select a maintained Route and Carrier foundation for M7--M9 without promoting the H3 laboratory tracer or importing another network's authority? | **Accepted non-selection of evaluated candidates:** native H3 C-5/C2, Tor/Arti, libp2p Circuit Relay, and WebTunnel do not satisfy the required protocol, authority, Go-maintenance, and Qualification criteria. R-076 subsequently supplies a distinct native Profile and closes DA-06; this record still forbids promoting any rejected candidate by package migration. | decided |
| [R-076](records/r-076-native-interactive-route-foundation.md) | Which maintained Route and Carrier foundation closes DA-06 without retaining Horizon 3 wire bytes? | **Accepted:** `ardents-interactive-route-v1` is a native Go split-leg Route over mutually authenticated TCP/TLS 1.3, with the reviewed HPKE Introduction primitive, Ardents State/publication authority, endpoint-owned selection, and Service Connection-owned recovery. H3 Route/Bridge/WebTunnel bytes are C0 retired; canonical v1 wire/vectors remain required M8 work under R-015. ADR-0024 records the foundation. | decided |
| [R-077](records/r-077-entry-invite-v1.md) | What Entry Invite and adjacent candidate format may serve `ardents-interactive-route-v1`? | **Accepted:** signed Entry Invite v1 binds a current State candidate and two-slot replacement lineage but carries no endpoint/key/Route/Target/carrier envelope. State remains the sole source of native TCP/TLS adjacent-candidate facts; H3 Invite/transition/WebTunnel bytes have no migration reader. ADR-0025 records the format. | decided |
| [R-078](records/r-078-interactive-route-v1-wire.md) | What canonical v1 Route wire and conformance boundary permits independent implementations without retaining H3 bytes or allowing a Node-selected downgrade? | **Accepted:** exact State-pinned TLS 1.3 ALPN, closed binary LegBinding and SealedIntroduction records, standard-library X25519/HKDF-SHA256/AES-128-GCM HPKE with visible header AAD, and synthetic vectors/mutation corpus. No legacy reader or peer-selected generation exists. ADR-0026 records the format. | decided |
| [R-014](records/r-014-language-runtime-candidates.md) | Which implementation language and runtime best fit the same accepted tracer, audited dependencies, memory safety, async networking, reproducible builds, target platforms, and the one-to-one project's capacity? | **Maintained foundation decided:** standard-library-first Go 1.26.x in one root module; CI and Carrier Lab pin Go 1.26.5. ADR-0009 records the selection and replacement rule. Safe Rust/Tokio remains only a bounded challenger after a measured Go-specific hard failure, not parallel maintained work. | decided |
| [R-025](records/r-025-carrier-lab-tool-supply.md) | How are the exact `tc netem` and packet-capture implementations supplied as content-addressed, license-reviewed Carrier Lab inputs without mutable package installation, runtime download, cgo, or first-party `unsafe`? | **Decided and smoke-proven:** exact official Ubuntu `.deb` inputs are locked and extracted offline into a disposable tooling image; namespace-sharing shapers receive only `NET_ADMIN`, capture receives only `NET_RAW`, and Application roles remain unprivileged. Native C-5/C2 consumes this closed supply interface without adding container-source files. | decided |
| [R-026](records/r-026-private-resolution-adapter.md) | Can an existing RFC 9458 OHTTP implementation support the bounded two-role Gate C Private Resolution Adapter without joining endpoint origin, exact Name/Target lookup, or Gateway identity? | **Decided and Gate C verified:** select `openpcc/ohttp v0.0.80` at commit `79bec89d8042` with explicitly raised CIRCL and Go `x/*` security versions. Online/offline supply, RFC vectors, role views, tamper/replay, fixed padding, licenses, cgo/unsafe, and reachable-vulnerability gates pass. Official run `gatec-31464163490-1` retained the required Relay/Gateway separation and returned `advance`. No local-table, direct, DNS, HTTP, alternate-namespace, cached-success, or first-party OHTTP fallback is permitted. | decided |
| [R-036](records/r-036-h3-camouflage-adapter.md) | Which replaceable Camouflage Adapter interface and pinned maintained implementation can carry the same H3 Bridge workload without selecting a production transport by inertia? | **Decided H3 Adapter profile:** select standalone WebTunnel `v0.0.6` at commit `d729fde1f38357dcefa2a751eb4752e9ca78f910` behind the replaceable candidate-neutral seam. Both pinned candidates passed the same useful-work and structural contract across stdin/SIGTERM/SIGKILL cells with zero observed DNS packets and zero cleanup residue; WebTunnel covers the protocol-allow-list profile with materially smaller candidate, state, dependency, and advisory surfaces. Lyrebird is neither packaged nor retained as fallback. ADR-0012 records the choice. R-037 and an accepted implementation brief remain before maintained Stage 5 code. | decided |
| R-015 | What protocol-description, versioning, negotiation, and conformance strategy permits independent implementations without freezing immature semantics or enabling downgrade? | **Decided for the maintained v1:** authenticated capability and exact Route Profile binding; protocol `announced → overlap-supported → preferred → required → retired`, separately build `current/superseded → vulnerable → revoked`; at least `90 days` ordinary protocol overlap except scoped expiring emergency; finite no-new-work/terminal Work Safety deadlines; highest mutually supported qualified selection and explicit `update required`. R-078/ADR-0026 select v1 canonicalization, conformance vectors, and strict refusal; a future generation reopens its own format decision. | decided |

## Product validation — runs beside technical research

| ID | Exact question | Decision and required result | State |
|---|---|---|---|
| R-016 | Which Users and Developers have a problem severe enough to adopt an internal location-private network despite latency, installation, and trust costs? | Scenario and competitor comparison now; external demand evidence remains a future gate and must not be invented from the Product Owner's preferences. | open |
| [R-017](records/r-017-named-private-site-anonymous-mailbox.md) | Is Named Unlisted Site a useful smallest Reference Application for exercising publish, name, resolve, connect, and route failure without adding messenger semantics? | **Selected and bounded tracer complete:** official Ubuntu run `gatec-31464163490-1` returned `advance` after 20/20 positives, 17/17 failures, and 5/5 migrations. It does not validate market demand and implies neither replicated Site Bundles nor an Ardents runtime. | decided |
| R-018 | Can a User and Developer understand Service Name trust, connection state, route limits, failure, and recovery without learning routing jargon? | One-to-one walkthrough can refine wording; external comprehension evidence remains a future release gate. | open |

## Decision order

R-024 closed the eventual public operating model, not the current backlog. The
dependency path is now deliberately sequential:

1. **Carrier Lab specification — complete:** R-013 freezes
   its Ubuntu fixtures, synthetic topology, per-role observations, coarse
   feasibility metrics, stop conditions, and cleanup.
2. **R-013/R-014 lab candidates — complete for the current horizon:** native
   C-5/C2 with mature controls and Go are selected for the experiment; Rust is
   the later bounded challenger. Naming, updater, public Control Plane, SDK, and
   cross-platform packaging seams remain deliberately unselected.
3. **Carrier Lab Route candidate verdict — complete:** official run
   `31404126248` returned `advance`; this retains the native split-circuit shape
   for another controlled slice without promoting its lab protocol or privacy
   claims.
4. **Named Unlisted Site Reference Application — complete:** official run
   `gatec-31464163490-1` returned `advance` for the minimum Application
   Interface, Target/Instance lifecycle, private reachability, one
   pre-provisioned exact Name, controlled HTTP tracer, failures, and migration.
   Permissionless Namespace governance remains separate.
5. **Closed test network — Stage 4 recovery development authorized:**
   accepted R-029 and R-030 join authenticated state, real Node lifecycle, and
   the endpoint-chosen multi-process Route. Accepted R-031 authorizes the
   Service Connection/Application Interface tracer. Its local development gate
   passed at commit `6c8faf9`: one retained `27/27` Docker campaign and an
   independent `27/27` verifier replay over the same frozen bundle, clean
   checks/reviews, retained digest
   `9aea2d37de910dec39cce79187fde94b49d53a10f0a6bab3a5ca14e6955162ae`, and
   complete cleanup. Accepted R-032 authorizes bounded S4.1–S4.3 same-connection
   recovery development; S4.4 remains gated on accepted P3-D3b4. Official Stage 1
   Ubuntu `short`, `churn-2h`, and
   `unattended-24h` qualification remains open and blocks the integrated H3 or
   stronger external/release claim. Role-capacity floors, Bridges, naming,
   updates, Windows, and broader Application boundaries retain their explicit
   later gates.
6. **Public qualification:** run the applicable complete R-001/R-023 and launch-
   independence gates only for a release candidate. Earlier experiments cannot
   accumulate partial public claims.

R-010 through R-012 and R-020 have product answers, but their mechanisms and
real independent participants remain public-launch gates. R-005, R-021, and
network-owned R-022 identity are outside the Product Core. R-016 and R-018 remain external
evidence gates that the one-to-one team cannot honestly close alone. No
production stack is selected before the bounded comparisons.
