# Mavis (M3) collaboration workflow

Status: **accepted Stage 7 collaboration rule on 2026-08-21.** This document is
the durable record of how MiniMax Mavis (M3) is integrated into the Stage 7
slice pipeline. It exists because the S7.1 first pass shipped a coherent
package but missed the normative contract on several cells, and a follow-up
review had to fix the result. The workflow below prevents that pattern from
repeating.

## Roles

The Stage 7 pipeline is split across four roles, and M3 owns at most two
of them in a single task. M3 must never bundle all four:

| Role | Who | What |
|---|---|---|
| Contract author | Owner + Codex | Product behavior, ADR, security boundary, or consequential public contract change; Codex holds the standing engineering delegation below |
| Implementer | M3 (`M3-autonomous`) | Bounded implementation against an already-accepted contract |
| Worker reviewer | M3 (`M3-after-contract`) | Independent review of the implementation against the same contract. **No fixes** — only findings. Must run in a fresh context, not the same session that wrote the code |
| Standards + Spec reviewer | Codex | Two-axis review of the small diff: Standards (architecture, dependency surface, naming) and Spec (every acceptance criterion mapped to a concrete test) |
| Acceptance | Owner | Final disposition of the slice |

### Product Owner standing engineering delegation

On 2026-08-21 the Product Owner authorized Codex to decide and accept routine
Stage 7.2 engineering amendments without another confirmation when they follow
the accepted codebase direction. This includes file/responsibility splits,
test seams, private recovery algorithms, feasible task caps, remediation text,
and other reversible implementation-contract details already bounded by the
accepted ADRs, threat model, product contract, and repository rules.
It also covers additive declarations inside an `internal/` Module and additive
owned-state record versions when the accepted product behavior requires them,
the prior version remains byte-exact supported, and no cross-Module authority
or compatibility claim is widened.

Codex records those decisions in the repository and still obtains independent
evidence before authorizing M3 implementation. The delegation does not cover a
new product behavior, a weaker security/privacy claim, a new trust root, a
consequential externally public Interface or backward-incompatible wire/storage
decision, an irreversible external action, or scope outside Stage 7.2. Those
remain joint Owner + Codex contract decisions. Final slice acceptance remains with the
Product Owner unless the Owner explicitly delegates that gate too.

## Slice shape

M3 is only given slices that satisfy all of the following. A slice that
violates any of these is rejected before M3 starts:

- one cohesive responsibility, named without `and everything else`;
- default planning envelope: ≤ 1500 production LOC, ≤ 15 files, ≤ 1 new
  `internal/<pkg>` + ≤ 1 new `cmd/<name>`, ≤ 10 new B-cell tests, and ≤ 2
  `package-map.md` row changes. A larger task-specific envelope requires a
  committed factual inventory, an explicit cohesion/security review, and a
  named accepted amendment; it is never inferred from an overrun;
- a contract with explicit acceptance criteria already accepted by Codex;
- a frozen public vector or fixed input table the worker reviewer can use
  as an independent oracle;
- a single commit at the end;
- no architectural decisions left to M3; no slice-closure claim by M3.

The recommended size is "one small vertical tracer or one contract with
tests, roughly up to a few hundred LOC diff."

The controller freezes caps from a physical baseline inventory and a named
file/responsibility map. Optimistic arithmetic is not evidence that a full
oracle and implementation fit. When the task has a large independent oracle,
delivery is split into a test-only red gate and a production gate in the same
isolated worktree. The first gate creates no commit; Codex must observe that the
specified behavior, rather than a fixture or compile error, is red before the
production gate starts.

## Parallel execution and controller stop rules

The Product Owner authorizes Codex to run independent M3 work in parallel for
Stage 7.2. Parallelism never weakens dependency or ownership boundaries:

- only one writer may change a package or one dependency chain at a time;
- every writer uses a distinct temporary worktree at an exact accepted SHA;
- read-only contract audits, worker review, and independent-oracle review may
  run concurrently with a writer when they do not consume its unaccepted diff;
- a later implementation never starts from an earlier unaccepted task result;
- Codex alone selects the integration order and no M3 process merges or pushes.

Codex monitors the actual worktree, session evidence, file counts, and focused
commands rather than waiting for or trusting only the final prose report. It
stops a run immediately when the worker proposes deleting an acceptance row,
recovery case, security check, or required test to fit a cap. A discovered cap
breach is `scope-blocked`; a semantic gap is `contract-blocked`. Neither permits
"aggressive compression", a responsibility grab by a convenient file, or a
partial implementation commit.

## Status flow

```
specified → implemented → worker-reviewed → Codex-reviewed → owner-accepted
```

M3 may set only `implemented` and `worker-reviewed`. Setting `accepted` or
`closed` is a contract violation.

## Required output format

Every implementation result must end with the literal phrase
**`implemented, not accepted`** and include the following:

- baseline SHA and итоговый SHA;
- список изменённых файлов;
- каждый acceptance criterion → конкретный тест;
- выполненные команды + точный результат;
- известные ограничения;
- подтверждение отсутствия изменений вне разрешённого scope.

A worker-review result returns a list of discrepancies against the same
acceptance criteria, in the same format. The discrepancy list is the
input to the next remediation task, not a request for M3 to fix in place.

## Cost distribution

Target split:

- M3: 70–85 % of code and test writing;
- Codex: contract, task formalisation, focused review;
- Owner: product decisions, final acceptance.

M3 must not re-implement large slices after review. When a worker review
or a Codex review returns a list of discrepancies, M3's next task is a
narrow remediation brief, not a rewrite.

An uncommitted failed worktree may be retained as negative review evidence, but
it is not a code source for the next attempt. Reuse requires an explicit
finding-by-finding salvage decision from Codex; security-sensitive recovery or
admission code defaults to a clean accepted baseline.

## S7.1 lessons the workflow must prevent

The first S7.1 pass shipped a coherent implementation that did not match
the normative contract on several cells. A follow-up fixup commit was
required. The workflow above is designed to catch each of the following
classes of error before commit:

1. Tests named after evidence cells (B0–B14) that did not actually prove
   the cell's contract. Example: B9 checked only that classifications
   were non-empty, not that the build / protocol state machines drove the
   decision.
2. New trust roots used to verify metadata before their durable
   publication. The spec requires the verified root chain and successor
   floors to be published atomically before `release-accepted`.
3. `protocol_phase`, `qualification`, and the build state machine values
   present in the candidate but with no influence on the decision.
4. Builder attestations that were strings carrying the right suffix
   instead of a binding to artifact digest, source revision, and build
   inputs.
5. No mandatory frozen public vector. The expected outcome was computed
   by the same dynamic fixtures that tested the implementation, so a
   defect in the implementation was reflected in the oracle.
6. Bounded-input check that was superficial: TOCTOU window, and the
   ability to read a directory with millions of foreign files before the
   bound fired.
7. H3 limit (project-controlled threshold identities and rebuild
   records) not enforced.
8. Test `Store` implementations that reused the production validator.
   A defect in the production validator was also a defect in the test
   oracle, and the test passed.
9. Hidden `Close` and cleanup errors; dependency documentation that
   did not match the actual import path.
10. Trust in green tests without an explicit acceptance-criterion →
    test mapping.

Every entry above is now blocked by an explicit step in the workflow:
frozen vectors are part of the contract (5), the durable publication
precondition is a named acceptance criterion (2), Mavis/Codex dual
review is mandatory (1, 3, 4, 6, 7, 8, 9, 10).

## Anti-patterns

- "Close the whole Stage 7.x in one go" — a single M3 run bundling
  contract design, implementation, oracle writing, adversarial review,
  and acceptance. This is the pattern that produced the S7.1 rework.
- "Tests named B0–B14" without the underlying contract asserted. The
  test name is a label, not evidence.
- "Same model, same session, both builder and reviewer." Use a fresh
  context for the worker review.
- "M3 fixes the discrepancies inline." Remediation is a separate task
  from the original implementation.
- "Fit by deletion." Removing an untested crash row, invariant, or cleanup
  acknowledgement after discovering the cap is a hard controller stop, not a
  refactoring opportunity.
- "One giant red-green run." Where Codex must independently observe the red
  oracle, production work cannot begin in the same uninterrupted model turn.
