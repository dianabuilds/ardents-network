# Stage 6 readiness checklist

Status: **S6.1 and S6.2 implementation are authorized. The general Stage 6
coding gate is not ready: R-042, R-044, R-045, evidence serialization, document
acceptance, and general coding authorization remain open.**

A checked contract item means the requirement is documented. It does not claim
that maintained code or runtime evidence exists.

## A. Product Owner and predecessor gate

- [x] R-039 accepts the bounded Stage 6 scope without authorizing code.
- [x] Stage 5 maintained development advanced on 2026-08-19; final R-037
  qualification remains S9.6 and is not a Stage 6 predecessor.
- [ ] Product Owner accepts the corrected brief, plan, checklist, and evidence
  contract after all S6.0 decisions are supported.
- [ ] Product Owner records the Stage 6 coding start decision after A-D pass.
- [x] Product Owner separately authorizes the bounded S6.1 and S6.2
  implementation slices on 2026-08-20; this does not waive the general gate.

## B. S6.0 decision freeze

- [x] R-041 freezes textual Service Name limits, textual Service Link behavior,
  a distinct length-prefixed wire form, and `schema_version = 1`.
- [ ] R-042 freezes an authenticated eligible-set/order proof that covers
  copying, front-running, withholding, flooding, partition, rollback,
  equivocation, and rule fork without local arrival time.
- [x] R-043 freezes durable, restart-derived, and cache-bounded property classes,
  tamper/stale failure, and atomic-commit semantics. Its future naming-owned
  interface and `internal/network/store` adapter must pass conformance tests.
- [ ] R-044 selects and measures a maintained mechanism for distinct scoped
  `t`-of-`n` Recovery Authorities. S6.2 authentication/query hiding is decided
  separately by R-047 and accepted ADR-0014.
- [ ] R-045 selects measured per-surface Anonymous Cost/admission limits on the
  R-023 host and a weaker client. Former numeric values are not accepted data.
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
- [ ] Canonical manifest, observation, cleanup, and verdict serialization;
  profile identity; clocks; commitments; and exact per-cell predicates are
  frozen after R-042/R-044/R-045/R-046 close.
- [ ] Independent schema mutation vectors prove unknown, missing, duplicated,
  reordered, stale, contaminated, and non-canonical evidence becomes `invalid`.

## E. Coding start rule

Except for the recorded S6.1 and S6.2 slices, Stage 6 maintained coding may start only
when every item in A-D is checked and the Product Owner records the decision.
Earlier foundation commits remain feasibility work and are not accepted Stage 6
evidence.

Current verdict: **not ready**.

## F. Implementation and completion evidence

- [x] S6.1 maintained parser, encoding, lifecycle, replay, and parent-binding
  behavior tests pass.
- [x] `ardents-name` command smoke reaches strict canonical accept/reject
  outcomes without performing a naming control operation.
- [ ] S6.1 independent evidence passes after the evidence schema is frozen.
- [x] S6.2 maintained role-separation behavior/failure tests pass.
- [ ] S6.2 independent evidence passes after the evidence schema is frozen.
- [ ] S6.3 Target continuity and connection-binding tests pass.
- [ ] S6.4 authority/delegation/recovery tests pass.
- [ ] S6.5 collision/fork/abuse tests pass.
- [ ] S6.6 independent evidence/verifier campaign passes.
- [ ] Maintained J02/J03/J05 command paths are traceable.
- [ ] Product Owner records the final Stage 6 disposition.
