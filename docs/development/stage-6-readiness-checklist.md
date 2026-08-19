# Stage 6 readiness checklist

Status: **not ready for coding. Stage 5 development has advanced; S6.0
decisions, corrected-document acceptance, coding authorization, and evidence
schema freeze remain open.**

This checklist distinguishes pre-code authorization from implementation evidence.
A checked contract item means the requirement is documented; it does not claim
that code or runtime evidence exists.

## A. Product Owner and predecessor gate

- [x] R-039 accepts the bounded Stage 6 scope without authorizing code.
- [x] Stage 5 maintained-development `advance` is explicit for S5.4 and S5.5
  (Product Owner, 2026-08-19); final qualification remains S9.6.
- [ ] Corrected Stage 6 brief, plan, checklist, and evidence contract are accepted.
- [ ] Product Owner records the Stage 6 coding start decision after A-D pass.

## B. S6.0 decision freeze

- [x] Exact label length, total length, hierarchy depth, canonical encoding, and
  schema version are frozen. (R-041, 2026-08-19: label 1–63 ASCII, total ≤ 253,
  depth ≤ 127, `[a-z0-9-]`, parent on the right, no leading/trailing/empty
  label, no leading/trailing or consecutive hyphen, no all-numeric top label,
  length-prefixed wire encoding, `schema_version = 1`.)
- [x] Claim ordering, ordered-collision proof, Conflict, partition, and rule-Fork
  semantics are frozen. (R-042, 2026-08-19: order key
  `(network_id, epoch_number, SHA-256(canonical claim payload))`, proof
  structure per R-041 + R-046, five-state classification with explicit
  fail-closed, coverage map for the eight brief scenarios.)
- [x] Persistence, restart, rollback, and cache-proof ownership is accepted. (R-043,
  2026-08-19: Go `Storage` interface in `internal/namelease/store.go` with
  `internal/network/store` as default implementation; durable /
  restart-derived / cache-bounded classification; tamper fail-closed;
  `epoch_number` replay-bound; atomic write semantics. No new technology;
  no ADR; future engine swap requires a new research record and a new ADR.)
- [x] Cryptographic/key-management mechanism or replaceable interface is accepted
  through research and an ADR where required. (R-044 + ADR-0013, 2026-08-19:
  Ed25519 for Name Authority, BLS12-381 for Recovery Policy threshold,
  OHTTP for resolver query hiding (reuse R-026 supply), SHA-256 + HKDF-SHA-256
  for hash and KDF. Replaceable Go interface boundary: `Signer`,
  `ThresholdVerifier`, `QueryHider` in `internal/nameauthority/sig.go`. No
  local primitive; future primitive additions require a new research record
  and a superseding ADR.)
- [x] Anonymous Cost and local admission parameters are finite, measurable, and
  compatible with R-010. (R-045, 2026-08-19: Hashcash-style SHA-256 PoW with
  per-surface bit difficulty plus per-endpoint per-epoch rate limit plus
  scoped short-lived capability for batched operations, calibrated to R-023
  `2 vCPU` / `2 GiB` reference host. Per-surface table: claim 1/20/100,
  renewal 10/16/1000, resolution 100/8/10000, policy 1/18/10, recovery
  0/22/1 per generation. Admission order
  `epoch → role → schema → counter → PoW (when required)`. Fail-closed:
  `admission-denied` per R-002, no lease mutation, no override path.)
- [x] Resolution and control-operation role fields, Role Domains, family
  exclusions, and Isolation Context boundaries are frozen. (R-046, 2026-08-19:
  five roles with per-role concrete types, forbidden combined view
  `User/Endpoint location ∧ exact Service Name / lookup value`, stable
  identifier scope, Role Domain per ADR-0005, identity/known-family exclusion
  per ADR-0005 + R-005 + R-035, Isolation Context per CONTEXT.md, fail-closed
  on missing/invalid role proof per R-002.)

## C. Scope and architecture contract

- [x] Exact private lookup only; no list, search, recommendation, DNS, HTTP,
  public resolver, local alias, or alternate Namespace fallback.
- [x] No admin, registrar, project operator, legal claimant, or manual panel can
  seize, release, transfer, or recover a Name Lease.
- [x] Lease, consistency, and recovery are independent state dimensions;
  Conflict/Fork cannot release a Lease.
- [x] Delegation is separate from authority transfer and is bounded by parent
  generation and lifecycle.
- [x] Resolver cache is proof-bounded and fail-closed.
- [x] Name-origin and direct-Target connection behavior is distinct and forbids
  silent stream retargeting.
- [x] Current naming package responsibilities and permitted imports are factual in
  `package-map.md`; any new verifier package is mapped only with real code.
- [x] No transport, storage engine, consensus, public wire, release, installer,
  Stage 7, or global utility layer is silently introduced.

## D. Evidence design contract

- [x] Manifest, evidence, and verdict authorities are disjoint.
- [x] Runner and command output cannot author or mutate a verdict.
- [x] Independent verifier semantics are `pass|fail|invalid`.
- [x] Required A0-D6 cells and expected runtime/verifier outcomes are defined.
- [x] Resolution and control-operation privacy claims identify protected
  information, adversary, conditions, measurement, and limitations.
- [x] Failure classes include `invalid-input`, `unavailable`, `stale-proof`,
  `state-conflict`, `fork-unresolved`, `recovery-policy-absent`,
  `recovery-pending`, and `admission-denied`.
- [x] Immutable schema/profile identifiers, hashes, clocks, role views, and exact
  per-cell predicates are frozen from S6.0 decisions. (`stage-6-private-naming-evidence.md`
  § Required artifact schema and § Blocking S6.0 decisions reference the frozen
  values from R-041, R-042, R-043, R-044 + ADR-0013, R-045, R-046, 2026-08-19.)

## E. Coding start rule

Stage 6 production coding may start only when every item in A-D is checked and
the Product Owner records the decision. Existing work-in-progress packages do not
waive this gate and cannot be cited as accepted Stage 6 evidence.

Current verdict: **not ready**.

## F. Implementation and completion evidence

- [ ] S6.1 encoding/lifecycle tests and evidence pass.
- [ ] S6.2 resolution and control-operation role-separation tests pass.
- [ ] S6.3 Target continuity and connection-binding tests pass.
- [ ] S6.4 authority/delegation/recovery tests pass.
- [ ] S6.5 collision/fork/abuse tests pass.
- [ ] Maintained J02/J03/J05 command paths are traceable.
- [ ] Independent verifier report is complete and uncontaminated.
- [ ] Product Owner records the final Stage 6 disposition.
