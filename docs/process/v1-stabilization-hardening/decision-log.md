# V1 Stabilization And Hardening Decision Log

## Purpose

This log records decisions that change the order, scope, architecture,
acceptance interpretation, or compensating path of the stabilization loop.

Every entry must contain:

- `Decision ID`
- `Date`
- `Domain / Scope`
- `Stage`
- `Situation`
- `Options Considered`
- `Decision`
- `Reason`
- `Impact`
- `Follow-up`
- `Status`

## Decisions

### DEC-STB-001

- `Date`: 2026-07-18
- `Domain / Scope`: repository-wide stabilization
- `Stage`: loop definition
- `Situation`: the root implementation has real vertical runtime behavior, but
  its integration, E2E, security, privacy, reachability, workload isolation,
  hosted-service readiness, and distributed-data evidence are not all at the
  same maturity level.
- `Options Considered`:
  - continue adding product surface before repairing the baseline;
  - run an isolated test cleanup without product hardening;
  - execute one gated stabilization loop from baseline repair through release
    evidence.
- `Decision`: execute one gated stabilization loop. Restore trustworthy gates
  first, then close product-grade runtime properties, then run release sweeps.
- `Reason`: feature work on a red or misleading baseline creates unverifiable
  progress and conflicts with runtime-truth and development-contract rules.
- `Impact`: no new product expansion may bypass Phase 0 and Phase 1 gates.
- `Follow-up`: start with `STB-001`.
- `Status`: `accepted`

### DEC-STB-002

- `Date`: 2026-07-18
- `Domain / Scope`: product posture for `v1`
- `Stage`: loop definition
- `Situation`: the current system concept describes a shared trusted
  environment and the runtime uses configured trust anchors. A permissionless
  workload marketplace would require a different Sybil, economic, abuse, and
  dispute model.
- `Options Considered`:
  - treat permissionless public compute as an implicit `v1` requirement;
  - stabilize `v1` as a managed trust-aware network and require a separate
    architecture decision before permissionless expansion.
- `Decision`: this loop targets a managed, trust-aware, decentralized network.
  Permissionless resource markets are out of scope unless the system documents
  are explicitly changed first.
- `Reason`: this preserves the current product concept and prevents hidden scope
  expansion from blocking concrete stabilization work.
- `Impact`: abuse protection remains mandatory, but token economics, consensus,
  staking, and dispute arbitration are not acceptance requirements here.
- `Follow-up`: document any future permissionless proposal outside this loop.
- `Status`: `accepted`

### DEC-STB-003

- `Date`: 2026-07-18
- `Domain / Scope`: network transport variants
- `Stage`: loop definition
- `Situation`: `tcp_only` is operational, `tcp_wss` exists but is not fully
  operator-configurable, and `tcp_quic` is explicitly not implemented.
- `Options Considered`:
  - make QUIC implementation a release blocker;
  - keep QUIC truthfully unsupported while completing TCP and WSS product paths;
  - hide unsupported QUIC behind fallback behavior.
- `Decision`: complete and verify TCP and secure WSS paths. Keep QUIC explicitly
  unsupported unless a dependency-reviewed need and implementation land through
  a later recorded decision. Silent fallback is prohibited.
- `Reason`: truthful capability reporting is preferable to an unverified
  transport expansion, and Waku remains the canonical foundation in all cases.
- `Impact`: Phase 3 can pass without QUIC only while diagnostics and local API
  report it as unsupported.
- `Follow-up`: retain explicit negative tests for `tcp_quic`.
- `Status`: `accepted`

### DEC-STB-004

- `Date`: 2026-07-18
- `Domain / Scope`: Network Foundation dependency graph and Waku Store
- `Stage`: Phase 1 / `STB-101`
- `Situation`: the selected graph has remediable Go, `x/net`, and `quic-go`
  findings, plus `pion/dtls/v2` with no fixed v2 release. The neutral-looking
  go-waku `WithWakuStore()` option still constructs the legacy Store v2beta4
  implementation, while a separate Store v3 client exists in the dependency.
- `Options Considered`:
  - force every dependency to its latest tag independently;
  - replace the DTLS v2 module path with v3 and assume API compatibility;
  - upgrade safe leaf lines in groups, treat Waku/libp2p/pubsub as one
    compatibility set, and remove DTLS v2 only through a supported upstream
    migration;
  - describe a go-waku patch upgrade as sufficient Store migration.
- `Decision`: use controlled dependency groups. Upgrade the Go and safe leaf
  security lines first. Keep Waku canonical and validate Waku/libp2p/pubsub as
  a set. Reject a cross-major DTLS `replace`. Treat Store v2beta4 retirement as
  an explicit adapter and wire-protocol migration with real multi-node tests.
- `Reason`: this removes known fixes without inventing compatibility between
  major module paths or hiding a protocol change behind a version bump.
- `Impact`: `STB-102` may update dependency selection but may not enable QUIC,
  WebRTC, or a second transport owner. A remaining DTLS v2 path moves to
  `STB-103` only with tested containment and a complete security exception.
- `Follow-up`: execute the ordered upgrade groups in the STB-101 matrix and
  re-run verbose vulnerability classification after each settled graph.
- `Status`: `accepted`

### DEC-STB-005

- `Date`: 2026-07-19
- `Domain / Scope`: Identity, Waku transport identity, persistent state, Data
  Substrate, and Diagnostics
- `Stage`: Phase 1 / `STB-105`
- `Situation`: identity and Waku private keys were separated from product data,
  but several files were written non-atomically and an incomplete continuity
  set could cause silent key regeneration. Backup, restore, and rotation rules
  were not canonical.
- `Options Considered`: tolerate missing keys and create replacements; store all
  keys in the main database; or keep keys separated, make writes private and
  atomic, and fail closed when retained state and key continuity disagree.
- `Decision`: keep key material separated from retained payload/state, use
  private atomic files for standalone secrets and payloads, retain bbolt/SQLite
  transactional storage, and reject partial or mismatched continuity sets.
- `Reason`: silent regeneration produces a plausible but false node identity
  and can orphan publication, trust, and retained network state. Co-locating
  decrypting material with retained ciphertext would violate the data security
  model.
- `Impact`: backup and restore operate on explicit stopped-node consistency
  groups. Identity and Waku keys do not rotate implicitly. Phase 2 must define
  capability lifecycle before introducing its persistent representation.
- `Follow-up`: validate backup restore in canonical integration/E2E coverage and
  implement the Phase 2 capability lifecycle without weakening these rules.
- `Status`: `accepted`

### DEC-STB-006

- `Date`: 2026-07-19
- `Domain / Scope`: Network Foundation / Messaging privacy protocol with
  Identity, Policy, Discovery, Publication, and Data Substrate authority
- `Stage`: Phase 2 / `STB-201`
- `Situation`: technical-alpha traffic exposes message class, owner/request
  correlation, and product payload through readable Waku content topics and
  plaintext envelopes. The earlier privacy documents did not fix wire fields,
  algorithms, replay behavior, capability delivery, or migration semantics.
- `Options Considered`: encrypt payload but keep readable per-class topics;
  dual-publish plaintext and encrypted traffic during rolling migration; derive
  future epochs from one group secret even after member revocation; or use one
  opaque capability lane, signed encrypted inner messages, fresh-secret
  revocation rotation, and a coordinated hard cut.
- `Decision`: adopt `ardents-private/1` as defined in
  `docs/network-privacy-protocol.md`: HKDF-SHA256/HMAC opaque selectors,
  XChaCha20-Poly1305 outer protection, Ed25519 sender signatures, durable replay
  rejection, scoped recipient-bound grants, HPKE grant delivery, and no
  plaintext or legacy downgrade.
- `Reason`: payload encryption alone still leaks operation meaning; epoch
  derivation alone cannot exclude a former group-secret holder; dual plaintext
  publication preserves the vulnerability being remediated.
- `Impact`: existing alpha nodes are intentionally wire-incompatible after the
  cutover. Missing capabilities deny publication/fetch rather than fall back.
  STB-202 must dependency-review HPKE and capability persistence before code.
- `Follow-up`: implement deterministic vectors and capability resolution in
  STB-202, envelope framing in STB-203, then migrate each domain path and prove
  raw Waku privacy through every enabled role.
- `Status`: `accepted`

### DEC-STB-007

- `Date`: 2026-07-19
- `Domain / Scope`: Workload Control execution adapter and production host
  support
- `Stage`: Phase 4 / `STB-401`
- `Situation`: the current executor launches arbitrary host processes through
  `os/exec`, which cannot provide the mandatory filesystem, network, identity,
  resource, isolation, or recovery boundary.
- `Options Considered`: retain host processes; integrate Docker Engine; build
  directly on containerd; support Podman simultaneously; use Kubernetes as the
  executor; or use Docker lifecycle with an OCI sandbox tier.
- `Decision`: use Docker Engine on Linux through the maintained Moby public
  client/API modules. Require gVisor `runsc` for untrusted workloads, allow
  hardened `runc` only for explicitly trusted containers, and retain host
  process execution solely as disabled-by-default trusted-development behavior.
- `Reason`: Docker supplies the complete node-local lifecycle and recovery
  substrate without making Ardents implement a container stack. gVisor provides
  a stronger kernel boundary for third-party code while preserving one
  lifecycle API.
- `Impact`: production support is Linux `amd64`/`arm64`; Windows/macOS Docker
  Desktop is development/CI only. Unsupported hosts, missing controls, stale
  Engine security patches, or missing `runsc` fail closed without fallback.
- `Follow-up`: implement labeled lifecycle and restart reconciliation in
  `STB-402`, then enforce the complete admission and resource posture in
  `STB-403`.
- `Status`: `accepted`

### DEC-STB-008

- `Date`: 2026-07-20
- `Domain / Scope`: Phase 7 release process and stabilization-loop exit criteria
- `Stage`: Phase 7 / `STB-705`
- `Situation`: the bounded soak driver was ready, but making an uninterrupted
  48-hour qualification run part of the active stabilization goal prevented the
  remaining release reviews and refactoring from progressing. Two early runs
  correctly exposed Windows orchestration defects before product scenarios.
- `Options Considered`: keep the development loop blocked for two days; claim a
  partial run as a completed soak; remove soak entirely; or separate tooling and
  smoke acceptance from the later immutable-candidate qualification.
- `Decision`: close STB-705 when the bounded driver, provenance checks,
  fail-fast behavior, cleanup, and representative cross-domain execution are
  verified. Require the uninterrupted 48-hour run as a separate pre-release
  qualification after refactoring and final candidate freeze.
- `Reason`: long-duration evidence is meaningful only for an immutable final
  candidate. Running it before release review and refactoring wastes the clock,
  while treating a partial run as success would falsify evidence.
- `Impact`: STB-706 begins immediately. STB-707 can declare readiness for
  qualification, but no public release decision is valid until the separate
  48-hour gate passes.
- `Follow-up`: preserve failed/aborted artifacts, complete review and
  refactoring, then run SOAK-001 once against the frozen release candidate.
- `Status`: `accepted`
