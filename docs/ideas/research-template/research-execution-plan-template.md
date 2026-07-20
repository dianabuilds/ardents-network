# Feature Research Execution Plan Template

## Purpose

This document defines a research loop for checking whether a feature idea
should be implemented in Ardents.

Its goal is decision quality, not implementation throughput.

## Scope

This loop is mandatory for ideas that:

- may create a new product capability;
- may change domain ownership, runtime behavior, or operator truth;
- may introduce or reshape critical dependencies;
- may require non-trivial documentation or QA expansion;
- may conflict with the current Ardents product form.

## Source Of Truth

Before any non-trivial step, validate against:

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/canonical-network-foundation.md`
- `docs/engineering-constraints.md`
- `docs/development-contract.md`
- relevant domain requirements
- `docs/reference-invariants.md` if the idea touches
  network/discovery/messaging/publication foundation
- `docs/qa/test-model.md` if the idea changes QA or test shape

If the idea conflicts with those documents, record that as research evidence.
Do not bury the conflict inside implementation options.

## Task Statuses

- `pending`
- `in_progress`
- `done`
- `blocked`

## Research Algorithm

1. Open this execution plan.
2. Find the first `in_progress` task.
3. If none exists, pick the first allowed `pending` task.
4. Set it to `in_progress`.
5. Complete the task fully.
6. Record evidence in the evidence log.
7. If a blocker or fork in direction appears, record it in the decision log or
   final decision draft before continuing.
8. Mark the task `done` only after the evidence is explicit.
9. Move immediately to the next allowed task.

## Control Rules

- Do not convert research into stealth implementation.
- Do not accept an idea only because it sounds useful.
- Do not hide disconfirming evidence.
- Do not leave architecture conflicts implicit.
- Do not conclude "accepted" without a defined documentation and execution path.
- Do not conclude "rejected" without stating why the idea fails the product
  model or cost/risk threshold.

## Phases

### Phase 0. Hypothesis Framing

Goal:

- make the idea testable instead of vague

Tasks:

- `pending` Write the research brief with explicit claim, problem, scope, and
  disqualifiers.
- `pending` List the source-of-truth documents and initial fit/conflict status.

Checks:

- the idea is stated as a falsifiable product hypothesis
- scope and non-goals are explicit

Transition gate:

- the team can say what would prove the idea worth doing
- the team can say what would kill the idea

### Phase 1. Product And Architecture Fit

Goal:

- determine whether the idea fits Ardents as a product and system

Tasks:

- `pending` Check product fit against system concept and system frame.
- `pending` Check runtime and operator-truth implications.
- `pending` Check conflicts with fixed architectural or foundation decisions.

Checks:

- idea does not rely on fake foundations or deferred critical behavior
- domain ownership implications are explicit

Transition gate:

- architecture fit is either justified or rejected with evidence
- no major system conflict remains implicit

### Phase 2. Cost, Risk, And Dependency Review

Goal:

- understand real delivery burden and hidden risk

Tasks:

- `pending` Identify dependency, security, and support posture implications.
- `pending` Identify runtime, migration, observability, and QA cost.
- `pending` Record major risks, unknowns, and likely failure modes.

Checks:

- risk list is concrete rather than generic
- dependency posture is explicit if new or changed dependencies are involved

Transition gate:

- the team understands what the idea will really cost
- major unknowns are named, not hand-waved

### Phase 3. Evidence And Option Comparison

Goal:

- compare the idea against alternatives and gather enough evidence for a
  justified decision

Tasks:

- `pending` Compare the idea against at least one alternative path.
- `pending` Record disconfirming evidence and counterarguments.
- `pending` Decide whether more research is required or whether the result is
  already sufficient.

Checks:

- the preferred option is not selected by default
- evidence includes reasons not to implement

Transition gate:

- there is enough evidence to accept, reject, defer, or reshape the idea

### Phase 4. Decision Closure

Goal:

- close the research loop with a defended outcome

Tasks:

- `pending` Write the final decision record.
- `pending` If accepted, define required docs and execution path before any
  implementation starts.
- `pending` If rejected, deferred, or reshaped, state the exact reason and
  follow-up policy.

Checks:

- final outcome is explicit
- implementation is not opened without a documented path

Transition gate:

- research ends in a defended terminal outcome, not an ambiguous note

## Final Acceptance Gate

This research loop may be marked complete only when:

- all phases are in terminal state;
- the evidence log contains the real reasoning, not just the conclusion;
- the final decision is one of:
  - `accepted`
  - `rejected`
  - `deferred`
  - `reshaped`
  - `blocked`
- if `accepted`, required documentation and the allowed implementation path are
  defined;
- if `rejected`, `deferred`, or `reshaped`, the reason is explicit and usable.
