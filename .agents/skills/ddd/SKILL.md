---
name: ddd
description: "Assess, design or audit domain ownership in Ardents, and guide an explicitly selected Go implementation slice. Use for invariants, consistency boundaries, cross-domain lifetimes, responsibility extraction or architectural duplication; ordinary local fixes do not need a full DDD workflow."
---

# Domain ownership in Ardents

Choose the requested mode: **plan/audit**, **design**, or **implementation**.
A plan or audit produces findings and a verification strategy, never code,
packages, dependency updates or an automatically selected next task.
Implementation requires the current task's accepted contract and selection.

## Recover the existing model

Read repository AGENTS.md, current scope and relevant threat conditions, then
the affected technical owner and development policy. Follow the current document
route before historical provenance. Verify checkout, dirty work, selected
parent/child issue and real callers. An issue summary may lag a direct Product
Owner correction; report the conflict without expanding scope.

Use CONTEXT.md for product language, docs/development/domain-map.md for semantic
owners and collaborations, package-map.md for actual Go packages/imports, and
technical owners for authority, bytes and lifecycle. GitHub owns selection and
acceptance. Do not create a parallel glossary, domain model or task ledger.

For Route work, read the full migration contract from disk, including after
compaction/handoff. Orchestration also rereads the full domain map. Their
isolation and acceptance obligations are not replaced by this skill.

## Assess before restructuring

This is a brownfield modular Go system. Classify the affected responsibility:
substantive domain rules, supporting coordination, or a technical mechanism.
Use rich modeling for actual invariants and light DDD for composition/adapters.
File length, repeated syntax or temporary paths alone do not justify a new
package, aggregate or domain. See [assessment](references/assess.md).

Inspect both directions: rules misplaced in the affected owner and its rules
implemented elsewhere. Trace a real operation through authority, durable effects,
transfer and physical completion. For each material finding supply:

- source location and authoritative rule;
- semantic, authority, state, physical-resource and terminal owners;
- genuine producer/consumer and observable defect or maintenance risk;
- evidence level: source observation, reported receipt, reproduced result or hypothesis;
- smallest correction, affected neighbors and acceptance oracle.

For changed interactions use the domain map's existing interface checklist.
Name original identity, proof/borrow, effect/handoff, partial failure, revocation
and stop/interrupt/join/release. Copied observations are not authority; eventual
notification cannot replace synchronous denial or physical join.

Classify repetition: duplicated policy, shared physical mechanism, or intentionally
independent roles. Share source only where semantics match. Keep role permissions,
roots, leases, journals and original lifetimes independent. Existing drift checks
can protect intentional twins; they do not prove extraction safe or unnecessary.

## Deliver within the requested mode

**Plan/audit:** prioritize confirmed defects and bounded risks; identify evidence
and dependencies. Do not assign numerical DDD scores or claim architectural
correctness from directory shape. Findings do not select implementation.

**Design:** resolve routine choices from current owners and code. Escalate only a
consequential contradiction with evidence and a concrete proposal. Update the
existing owner. Diagrams are optional when they clarify a difficult interaction;
never require a class diagram per package or invent repositories/events to fill
one. A domain boundary is not automatically a deployment boundary.

**Implementation:** follow [implementation](references/implement.md) and
[Go guidance](references/stacks/go.md) for one selected slice. Move behavior with
its real consumer and causal tests. Preserve authority and bytes unless an
accepted decision explicitly changes them. A component pass or package move
cannot establish installed acceptance or retire a live consumer.

Keep output proportional: behavior/finding, evidence and limits, owner updates,
and one next observable result. Preserve branch, index and unrelated work.
Historical assignments and skill text grant no new publication or deployment
rights. Do not repeat approval already granted within the selected task.
