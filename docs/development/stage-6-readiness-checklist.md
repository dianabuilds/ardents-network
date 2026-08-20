# Stage 6 readiness checklist

Status: **maintained S6.1-S6.6 implementation and mutation coverage are complete. The Product Owner accepted R-042 O1b,
R-044 O2, R-045 O1b, S6E1, ADR-0017 through ADR-0019, and authorized maintained
S6.3-S6.6 implementation on 2026-08-20. The bounded command campaign received
independent `pass`; the Product Owner accepted the result and recorded Stage 6
`complete` on 2026-08-20.**

A checked contract item means the requirement is documented. It does not claim
that maintained code or runtime evidence exists.

## A. Product Owner and predecessor gate

- [x] R-039 accepts the bounded Stage 6 scope without authorizing code.
- [x] Stage 5 maintained development advanced on 2026-08-19; final R-037
  qualification remains S9.6 and is not a Stage 6 predecessor.
- [x] Product Owner accepts the corrected brief, plan, checklist, and evidence
  contract after all S6.0 decisions are supported.
- [x] Product Owner records the Stage 6 coding start decision after A-D pass on
  2026-08-20.
- [x] Product Owner separately authorizes the bounded S6.1 and S6.2
  implementation slices on 2026-08-20; this does not waive the general gate.

## B. S6.0 decision freeze

- [x] R-041 freezes textual Service Name limits, textual Service Link behavior,
  a distinct length-prefixed wire form, and `schema_version = 1`.
- [x] R-042/ADR-0017 freeze an authenticated eligible-set/order proof that covers
  copying, front-running, withholding, flooding, partition, rollback,
  equivocation, and rule fork without local arrival time.
- [x] R-043 freezes durable, restart-derived, and cache-bounded property classes,
  tamper/stale failure, and atomic-commit semantics. Its future naming-owned
  interface and `internal/network/store` adapter must pass conformance tests.
- [x] R-044/ADR-0018 select and measure a maintained mechanism for distinct scoped
  `t`-of-`n` Recovery Authorities. S6.2 authentication/query hiding is decided
  separately by R-047 and accepted ADR-0014.
- [x] R-045/ADR-0019 select measured per-surface Anonymous Cost/admission limits
  on the development endpoint and weaker `1 vCPU/512 MiB` Linux profile. O1 is
  rejected; O1b is accepted.
- [x] R-046 contains the exact per-operation field matrix for requests,
  responses, logs, errors, retries, caches, identifiers, and evidence, with
  Role Domain and Isolation Context rules.

## C. Scope and architecture contract

- [x] Exact private lookup only; no list, search, recommendation, DNS, HTTP,
  public resolver, local alias, or alternate Namespace fallback.
- [x] No admin, registrar, project operator, legal claimant, or manual panel can
  seize, release, transfer, or recover a Name Lease.
- [x] Lease, consistency, and recovery are independent state dimensions;
  Conflict/Fork cannot release a Lease.
- [x] Delegation is separate from transfer and bounded by parent generation and
  lifecycle.
- [x] Resolver cache is proof-bounded and fail-closed.
- [x] Name-origin and direct-Target connection behavior is distinct and forbids
  silent stream retargeting.
- [x] Current packages are factual in `package-map.md`; future interfaces,
  adapters, verifier packages, and commands are documented only with real code.
- [x] No transport, storage engine, consensus, public wire, release, installer,
  Stage 7, or generic utility layer is selected by implication.

## D. Evidence design contract

- [x] Manifest, evidence, and verdict authorities are disjoint.
- [x] Runner and command output cannot author or mutate a verdict.
- [x] Independent verifier semantics are `pass|fail|invalid`.
- [x] A0-D6 is the Stage 6 behavior inventory; it is not the Stage 5 R-037
  campaign and creates no S9.6 claim.
- [x] Privacy claims state protected information, adversary, conditions,
  measurement, and limitations.
- [x] Required failure classes are identified as a draft taxonomy.
- [x] S6E1 freezes canonical manifest, observation, cleanup, and verdict serialization;
  profile identity; clocks; commitments; and exact per-cell predicates are
  frozen after R-042/R-044/R-045/R-046 close.
- [x] S6E1 freezes mutation requirements for unknown, missing, duplicated,
  reordered, stale, contaminated, and non-canonical evidence.

## E. Coding start rule

Every item in A-D is checked and the Product Owner authorized S6.3-S6.6 on
2026-08-20. Maintained implementation may proceed; no slice is complete until
its behavior and independent evidence gates pass.

Current verdict: **Stage 6 complete; bounded S6E1 command campaign `pass`;
Product Owner disposition accepted 2026-08-20**.

## F. Implementation and completion evidence

- [x] S6.1 maintained parser, encoding, lifecycle, replay, and parent-binding
  behavior tests pass.
- [x] `ardents-name` command smoke reaches strict canonical accept/reject,
  private resolution, and all eight private naming control outcomes.
- [x] S6.1 independent evidence passes under the frozen S6E1 schema.
- [x] S6.2 maintained role-separation behavior/failure tests pass.
- [x] S6.2 independent evidence passes under the frozen S6E1 schema.
- [x] S6.3 Target continuity and connection-binding tests pass.
- [x] S6.4 authority/delegation/recovery tests pass.
- [x] S6.5 collision/fork/abuse tests pass.
- [x] S6.6 independent evidence/verifier campaign passes.
- [x] S6.6 mutation vectors prove every frozen structural defect becomes
  `invalid` and behavioral breaches become `fail`.
- [x] Maintained J02/J03/J05 command paths are traceable in
  `stage-6-journey-trace.md`.
- [x] Product Owner records the final Stage 6 `complete` disposition on
  2026-08-20.
