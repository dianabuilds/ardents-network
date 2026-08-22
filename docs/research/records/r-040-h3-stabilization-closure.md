---
id: R-040
title: How does Horizon 3 become a clean stable baseline before Horizon 4?
status: superseded-in-part
owner: product research
started: 2026-08-17
reviewed: 2026-08-21
---

# R-040 - Horizon 3 stabilization and technical closure

## Decision status

Status: **accepted by the Product Owner on 2026-08-17; Stage ordering
superseded by accepted R-058 on 2026-08-21.**

Horizon 3 gains a required Stage 9 after functional integration. Stage 9 owns
code, documentation, test, verification, dependency, build, and infrastructure
stabilization and qualifies the cleaned source before Horizon 4 begins.

R-058 preserves the cleanup, reduction, freeze, and final-qualification intent
but moves every planned mutation into Stage 8. Stage 9 now accepts only the
frozen Stage 8 candidate and performs qualification and closure. The original
ordering below remains decision provenance, not current execution authority.

## Decision this unlocks

Prevent Horizon 3 from handing Horizon 4 a functionally capable but structurally
unfinished repository. Make cleanup and documentation conversion an evidence gate
rather than an optional follow-up.

## Current problem

- functional and adversarial coverage is stronger than code maintainability;
- stage-specific harnesses, schemas, fixtures, and documents accumulate faster
  than completed work is consolidated;
- development tests can pass while final-campaign blockers remain;
- completed stage plans remain discoverable and slow agent navigation;
- laboratory code risks becoming permanent through inertia; and
- cleanup after qualification would create a different unqualified source state.

## Accepted decision

1. Stage 9 is part of Horizon 3 and is required for H3 completion.
2. Stage 8 produces the integrated functional candidate; Stage 9 cleans,
   freezes, and performs the final qualification of that cleaned candidate.
3. Stage 9 adds no product functionality.
4. `internal/lab/*`, `cmd/*-lab`, disposable experiments, placeholders,
   development modes, and temporary infrastructure do not remain in the final H3
   tree.
5. Reusable verifier/harness responsibilities may remain only after promotion to
   maintained non-shipping qualification ownership with factual package mapping.
6. Completed stage briefs, development plans, readiness checklists, and temporary
   evidence narratives are deleted after their valid content is consolidated into
   current technical documentation.
7. Git history, not a duplicate active archive tree, preserves removed development
   documents. Accepted ADRs and research records remain durable provenance.
8. Product-planning documents retire from the active technical set unless they
   still define a normative contract or security limitation. The canonical
   product contract and threat model remain where required by repository authority.
9. One documentation index defines a small mandatory agent context and routes
   package-specific tasks to only relevant technical documents.
10. Final H3 checks run after cleanup and source/supply freeze. Later changes rerun
    the affected qualification scope.
11. Final H3 qualification is a dedicated pre-H4 campaign on a frozen powerful,
    non-overcommitted stand. Earlier development/regression checks support coding
    but do not substitute for that campaign or need to reproduce its full scale in
    every stage loop.

## Evaluation criteria

- no unclassified maintained repository path;
- no unfinished or laboratory production surface;
- one owner and technical source of truth per mechanism;
- small active documentation and agent reading set;
- fixed requirement-to-check traceability;
- reproducible clean-checkout build and qualification;
- separated shipped production and non-shipping verification tooling;
- no repository-local generated evidence or secrets; and
- independent final verdict for the post-cleanup source identity.

## Options

1. **Clean only after Horizon 3.** Rejected: final H3 evidence would describe the
   pre-cleanup source rather than the version handed to H4.
2. **Refactor continuously without a final gate.** Rejected as insufficient:
   local hygiene does not force removal of accumulated cross-stage artifacts.
3. **Add Stage 9 stabilization and closure.** Accepted: local cleanup continues,
   while Stage 9 performs bounded repository-wide consolidation and final
   qualification.
4. **Keep all development documents and lab code as history.** Rejected: git and
   durable decisions preserve provenance without making obsolete material part of
   normal agent context.

## Evidence plan

Stage 9 evidence includes a complete disposition ledger, before/after package and
documentation maps, placeholder/lab/dead-surface inventory, dependency and
infrastructure inventory, fixed test tiers, requirement traceability, clean build
receipts, the frozen dedicated-stand profile, final integrated campaign artifacts,
independent verdicts, and cleanup inventory.

Falsification occurs if cleanup changes behavior without decision/requalification,
if unclassified or unfinished material remains, if technical truth still depends
on completed development plans, if production ships qualification code, or if the
post-cleanup source cannot reproduce the accepted checks.

## Disposition

- Stage 9 is accepted as a mandatory Horizon 3 stage.
- `docs/development/horizon-3-stage-9-brief.md` is its execution contract.
- Stage 9 must be planned before Stage 8 final qualification so cleanup precedes
  the final source/supply freeze.
- No technology, package name, dependency, or wire format is selected by this
  record; those remain factual Stage 9 inventory decisions under repository rules.

## Supersession note — 2026-08-21

Accepted R-058 supersedes the original Disposition assignments and execution
ordering only where this record assigned cleanup, restructuring, documentation conversion, test
consolidation, or infrastructure stabilization to Stage 9. Those changes now
belong to Stage 8. Stage 9 remains mandatory, adds no product functionality,
and owns the frozen final H3 qualification. The requirements to remove obsolete
surfaces, reduce active documentation, freeze the post-cleanup identity, and
qualify that exact identity remain in force.
