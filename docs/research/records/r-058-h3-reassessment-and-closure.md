---
id: R-058
title: How should Horizon 3 reassess, restructure, and qualify the project after Stage 7?
status: accepted
owner: Product Owner and Codex
started: 2026-08-21
reviewed: 2026-08-21
---

# R-058 — Horizon 3 reassessment, restructuring, and closure

## Decision this unlocks

Replace or retain the accepted Stage 8/9 structure before either stage begins.
The decision must define:

- what Stage 8 and Stage 9 are for;
- whether they remain separate;
- which work is research, design, implementation, stabilization, or
  qualification;
- how Product Owner, Codex, and MiniMax M3 divide work by task character;
- how future Modules, packages, files, tests, and documentation are shaped;
- which source identity receives the one final Horizon 3 qualification; and
- what exact result permits `advance-to-H4`, `redesign`, or `stop`.

This record does not authorize refactoring, code cleanup, documentation
deletion, or a Stage 8 campaign. It defines the decision that must precede
those actions.

## Current contract

The following remains fixed unless separately superseded:

- accepted ADRs, the Product Core, and the threat model retain their normal
  authority;
- ADR-0009 keeps Go and one root module;
- ADR-0010 keeps the first-party monorepository through the Closed Test
  Network horizon;
- ADR-0011 separates Unit, end-to-end, and Live test surfaces and rejects a
  second evidence protocol for ordinary Module behavior;
- Stage 7 continues under its accepted contract until separately changed; and
- Horizon 4 remains conditional and cannot claim external demand,
  independent operation, or independent security review from the one-to-one
  project team.

**Subsequent disposition:** the first condition above was separately changed on
2026-08-22 when the Product Owner stopped Stage 7. S7.3-S7.7 are cancelled; the
retained S7.1/S7.2 results and R-050 evidence are inputs to the later Stage 8
product disposition, not a requirement to complete the stopped stage.

At the start of this review, R-040 required Stage 8 to produce an integrated
functional candidate and Stage 9 to clean, freeze, and qualify it. The then-current
technical design additionally put a complete adversarial matrix and multi-day
soak in Stage 8, before Stage 9 changed code, packages, tests, and documentation.
That ordering was the decision under review.

## Hypotheses

- **H1:** A revised two-stage model is cheapest and clearest: Stage 8 owns
  evidence-led reassessment plus all planned restructuring; Stage 9 accepts
  only a frozen candidate and owns final qualification and closure.
- **H2:** One merged Stage 8/9 can reassess, restructure, stabilize, and qualify
  the project without losing a clean source-freeze boundary.
- **H3:** The current Stage 8 integration followed by Stage 9 cleanup and
  requalification remains worth its duplicated campaign cost.
- **H0:** The current product or maintained project is not coherent or
  maintainable enough to justify restructuring and qualification; Horizon 3
  should stop or return to a narrower product hypothesis.

## Evaluation criteria

An acceptable structure must:

1. fit the real Product Owner plus Codex team without hidden staff;
2. make product and architecture reconsideration possible before sunk cost
   turns the current implementation into an implicit decision;
3. run the complete expensive cross-platform, Live, hostile, sustained, and
   soak qualification only against the last planned source identity;
4. retain enough integrated diagnostic execution before restructuring to
   reveal real cross-Module behavior and maintenance cost;
5. distinguish research decisions, execution tasks, regression tests, and
   qualification evidence;
6. place seams from responsibility, invariants, lifecycle, failure policy, and
   dependency direction rather than line or file counts;
7. test observable behavior through Module Interfaces and avoid retaining
   duplicate internal tests after a Module is deepened;
8. route cognitively global work to Codex and only locally specifiable,
   independently verifiable work to M3;
9. reduce mandatory agent context while retaining accepted decisions and
   security limitations as provenance; and
10. end with an explicit Product Owner disposition: advance, return for
    redesign, narrow the product, or stop.

No option passes merely because it creates more documents, more test cells, a
smaller average file, or a larger share of M3-written code.

## Evidence plan

### Primary sources

Project sources, accessed 2026-08-21:

- `docs/product/scope.md`;
- `docs/development/horizon-3-technical-design.md`;
- `docs/research/records/r-040-h3-stabilization-closure.md`;
- `docs/development/horizon-3-stage-9-brief.md`;
- ADR-0009, ADR-0010, and ADR-0011;
- `docs/development/repository-layout.md`;
- `docs/development/go-engineering.md`;
- `docs/development/testing.md`;
- `docs/development/m3-collaboration.md`; and
- the accepted S7.2 contracts and M3 briefs as current examples of the task
  formation cost.

### Repository measurements

Reproduce with read-only file enumeration and line counting from the current
working tree. Measurements are descriptive signals, not quality verdicts:

- count Markdown files and physical lines by documentation area;
- count maintained production Go files by physical-line band;
- inspect whether task contracts choose file allocation before Implementation;
- inventory current test surfaces, qualification campaigns, and sources of
  duplicated verdict logic; and
- inventory active, durable, transitional, and obsolete documentation classes.

### Failure scenarios

- a complete Stage 8 campaign qualifies code that Stage 9 intentionally
  changes;
- a merged stage continues changing source after declaring qualification;
- global refactoring is delegated as disconnected local patches;
- a new line cap replaces the old cap without measuring cognitive locality;
- package cleanup creates more shallow Modules or hypothetical seams;
- new Interface-level tests are added while obsolete internal tests remain;
- documentation cleanup deletes the only current statement of an invariant;
- documentation reduction creates another permanent disposition bureaucracy;
- accepted research history remains in every default agent context; or
- qualification findings are patched in place without returning to the
  restructuring phase and freezing a new candidate.

## Findings

1. **Sourced fact:** R-040 already recognizes that functional and adversarial
   coverage is stronger than maintainability, stage documents accumulate, and
   cleanup after qualification produces a different unqualified source.
2. **Sourced fact:** the current Stage 8 contract nevertheless requires a
   complete short adversarial matrix and multi-day unattended soak before
   Stage 9 performs planned structural changes and final requalification.
3. **Inference:** the current split pays for one near-qualification campaign
   against a deliberately transitional candidate. That campaign remains useful
   diagnostic evidence but cannot qualify the cleaned source.
4. **Measurement:** the repository contains `121` Markdown files and `32,176`
   Markdown lines. `docs/development` contains `44` files and `10,692` lines;
   `docs/research` contains `49` files and `17,339` lines. Volume alone is not
   a defect, but the default context cost supports R-040's existing reduction
   requirement.
5. **Sourced fact:** R-040 already accepts Git history, rather than an active
   archive tree, as the home for completed briefs, checklists, and temporary
   development narratives after their valid facts are promoted.
6. **Measurement:** `internal` and `cmd` currently contain `685` maintained
   production Go files. `52` are between `200` and `239` physical lines,
   including `17` between `230` and `239`, while none exceeds the hard
   production cap. The S7.2 proposal explicitly added an eighth production
   file after otherwise cohesive responsibilities exceeded the independent
   `250`-line cap.
7. **Inference:** this distribution does not prove that any particular split
   is wrong, but it shows that the cap materially shapes the Implementation and
   therefore cannot be treated as a neutral formatting rule.
8. **Sourced fact:** repository policy correctly says line count alone does not
   create a package seam, but its hard file cap and expensive package admission
   rules can simultaneously fragment files and discourage correction of real
   package seams.
9. **Sourced fact:** ADR-0011 requires ordinary independently runnable tests
   rather than a second evidence protocol around the same Module. Stage 8 must
   reconcile this with later stage-specific manifest/verifier growth instead
   of automatically retaining both.
10. **Sourced fact:** `m3-collaboration.md` is explicitly a Stage 7 rule. Its
    `70–85%` M3 code/test target is not a project-wide allocation policy.
11. **Inference:** cognitively global reassessment, Module redesign,
    cross-package refactoring, and documentation authority require one coherent
    Codex context. M3 remains useful after those decisions for bounded work
    whose correctness has a local oracle.

## Options

### O1 — Retain the current Stage 8/9 split

Stage 8 integrates and soaks the current candidate. Stage 9 then inventories,
refactors, reduces documentation, freezes, and fully requalifies it.

**Benefit:** the refactor starts from extensive observed integrated behavior.

**Cost:** it deliberately performs the most expensive campaign before planned
source changes and mixes stabilization with a second qualification program.
It also postpones product and development-method reassessment until the current
shape has accumulated more inertia.

**Disposition:** not recommended.

### O2 — Merge all remaining work into one stage

One stage researches, redesigns, restructures, deletes, stabilizes, freezes,
qualifies, and closes Horizon 3.

**Benefit:** one backlog and no artificial handoff between numbered stages.

**Cost:** mutation and proof share one status space. A long stage can appear
nearly complete while still allowing source changes, and qualification defects
can be patched without a visible return to design. The resulting execution
brief risks becoming another monolithic contract.

**Disposition:** viable only with an explicit internal freeze gate, but weaker
than O3 for authority and evidence clarity.

### O3 — Redefine two stages around the source freeze

Stage 8 owns all planned learning and mutation. Stage 9 owns proof of the
frozen result and permits no planned mutation.

**Benefit:** one complete expensive qualification, a visible return path after
defects, and a clean distinction between deciding/changing and proving. Stage 8
can contain several workstreams without pretending that each is another
delivery stage.

**Cost:** the team must resist turning the Stage 8 workstreams into another
large collection of independent stage contracts. Stage 9 findings may still
force a return to Stage 8 and a complete rerun; that cost is logically
unavoidable because the source identity changed.

**Disposition:** recommended.

### O4 — Add separate research, refactor, and qualification stages

Create Stage 8 for reassessment, Stage 9 for restructuring, and Stage 10 for
qualification.

**Benefit:** each stage has one verb.

**Cost:** adds promotion gates, documents, acceptance cycles, and historical
terminology without improving the essential mutation/freeze distinction.

**Disposition:** reject; use workstreams inside Stage 8 instead.

## Recommendation

The Product Owner accepted O3 on 2026-08-21. Confidence is high. The strongest
argument against it is that Stage 8 becomes broad. Control that breadth with a
small outcome map and explicit decision gates, not separate stage-sized
documentation packages.

### Proposed Stage 8 — Reassessment and Restructuring

Stage 8 is an evidence-led research, design, and change stage. It has no final
H3 qualification claim. Its ordered workstreams are:

1. **Current-system truth.** Assemble the Stage 1–7 candidate and run a bounded
   integrated diagnostic set sufficient to reveal real journeys, dependencies,
   failures, resource behavior, and maintenance burden. Do not run the final
   multi-day qualification campaign.
2. **Product disposition.** Reconsider the Product Core, supported journeys,
   threat claims, operational burden, and simpler alternatives. The Product
   Owner chooses `continue`, `narrow`, `redesign`, or `stop` before repository
   restructuring begins.
3. **Development model.** Decide task routing, Codex/M3 roles, review depth,
   code policy, Module and package formation, test surfaces, qualification
   ownership, and documentation lifecycle. Do not retain a universal M3 code
   percentage or universal evidence burden.
4. **Target technical shape.** Produce the smallest current Module map,
   Interfaces, real seams, dependency direction, command ownership, retained
   test surfaces, and active-document set needed by the chosen product.
5. **Execution planning.** Only after the target shape is accepted, decompose
   restructuring into behavior-preserving tracer tasks. Characterization tests
   precede risky changes. Global transformations remain Codex-owned; bounded
   local tasks may use M3.
6. **Restructuring and reduction.** Execute the accepted package/Module/code,
   test, infrastructure, and documentation changes. Promote each still-valid
   fact before deleting its transitional source. Replace tests at the new
   Interface; do not keep obsolete internal tests merely for coverage count.
7. **Readiness and freeze proposal.** Demonstrate clean checkout, current
   ordinary checks, bounded integrated smoke/Live behavior, no planned source
   or normative-document changes, and a complete final qualification plan.

Stage 8 outputs are:

- one Product Owner product disposition;
- one accepted development and collaboration model;
- one factual target Module/command/dependency map;
- one code and testing policy without architecture-by-line-count;
- one small active-document map plus completed deletion dispositions;
- one cleaned candidate with no planned refactor remaining; and
- one proposed frozen source/supply/document identity for Stage 9.

The workstreams are not seven new stages and do not each require a research
record, implementation brief, checklist, and evidence protocol. A new durable
record is created only when a consequential decision actually needs one.

### Proposed Stage 9 — Frozen Qualification and Closure

Stage 9 begins only after the Product Owner accepts the Stage 8 freeze
proposal. It owns:

1. exact source, dependency, tool, platform, configuration, and normative
   documentation identities;
2. the final independently runnable Unit, end-to-end, Live, platform,
   hostile/fault, sustained, cleanup, and multi-day soak scope required for the
   remaining H3 claims;
3. independent recomputation only where a security or qualification claim
   requires it, without wrapping every ordinary Module test in another verdict
   protocol;
4. one final evidence and limitation report; and
5. the Product Owner disposition `advance-to-H4`, `return-to-S8`, or `stop`.

Stage 9 has no planned production refactor, package change, code-rule change,
or documentation conversion. Any necessary normative or source change returns
the project to Stage 8, creates a new candidate identity, and invalidates the
affected qualification. This rerun is necessary evidence, not duplicated
planned work.

## Documentation lifecycle to decide in Stage 8

The target policy should distinguish four classes:

1. **Active truth:** the minimum product, security, architecture, engineering,
   test/qualification, dependency, and operations material needed to work on
   the current candidate.
2. **Durable provenance:** accepted ADRs and research records, retained but
   excluded from default context unless their decision is touched.
3. **Transitional work:** briefs, plans, checklists, amendments, and review
   narratives deleted after their unique valid facts are promoted.
4. **Generated evidence:** run artifacts, captures, caches, and reports kept
   outside the repository under declared retention rules.

The audit must decide individual files; R-058 does not pre-authorize deleting
any named document. Git remains the history for removed transitional text. The
disposition inventory is temporary and must not become a new permanent archive.

## Disposition

- State: `accepted`; the Product Owner accepted O3 on 2026-08-21.
- R-040 is superseded only in its Stage 8/9 ordering. Its cleanup,
  documentation-reduction, final-freeze, and post-cleanup qualification intent
  remains required.
- `horizon-3-technical-design.md` now places every planned mutation in Stage 8
  and final qualification in Stage 9.
- One Stage 8 execution brief and one replacement Stage 9 qualification brief
  own the future work. Target technical documentation is created by
  consolidation, not by adding a parallel permanent document set.
- This acceptance changes planning authority only. It does not start Stage 8,
  authorize refactoring, or delete any current code or document.
