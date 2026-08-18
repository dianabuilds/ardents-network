# Stage 6 readiness checklist

Status: **not ready for coding. Stage 5 advance, S6.0 decisions, corrected-document
acceptance, and evidence schema freeze remain open.**

This checklist distinguishes pre-code authorization from implementation evidence.
A checked contract item means the requirement is documented; it does not claim
that code or runtime evidence exists.

## A. Product Owner and predecessor gate

- [x] R-039 accepts the bounded Stage 6 scope without authorizing code.
- [ ] Stage 5 `advance` is explicit for S5.4 and S5.5.
- [ ] Corrected Stage 6 brief, plan, checklist, and evidence contract are accepted.
- [ ] Product Owner records the Stage 6 coding start decision after A-D pass.

## B. S6.0 decision freeze

- [ ] Exact label length, total length, hierarchy depth, canonical encoding, and
  schema version are frozen.
- [ ] Claim ordering, ordered-collision proof, Conflict, partition, and rule-Fork
  semantics are frozen.
- [ ] Persistence, restart, rollback, and cache-proof ownership is accepted.
- [ ] Cryptographic/key-management mechanism or replaceable interface is accepted
  through research and an ADR where required.
- [ ] Anonymous Cost and local admission parameters are finite, measurable, and
  compatible with R-010.
- [ ] Resolution and control-operation role fields, Role Domains, family
  exclusions, and Isolation Context boundaries are frozen.

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
- [ ] Immutable schema/profile identifiers, hashes, clocks, role views, and exact
  per-cell predicates are frozen from S6.0 decisions.

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
