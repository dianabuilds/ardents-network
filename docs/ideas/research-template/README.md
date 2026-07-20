# Feature Research Template Kit

## Purpose

This directory contains templates for feature research loops in Ardents.

The goal is not to "start implementation faster".
The goal is to decide whether an idea deserves implementation at all, and only
then define the product-grade path for it.

This template set is used when the team needs to:

- check a feature hypothesis;
- understand whether the idea fits the Ardents product shape;
- identify blockers, risks, and hidden costs early;
- decide whether to accept, reject, defer, or reshape the idea;
- prepare the minimum documentation and execution path if the idea is accepted.

## What This Template Must Produce

A completed research package must answer:

- what problem the idea solves;
- why the idea is worth doing or why it should be rejected;
- how the idea fits or conflicts with system documents;
- what product, architecture, dependency, security, runtime, and QA risks exist;
- what evidence supports the conclusion;
- what documents must exist before implementation starts;
- what execution path is allowed if the idea is accepted.

## Recommended Document Set

- `research-brief-template.md`
  Initial feature hypothesis, scope, assumptions, and evaluation criteria.
- `research-execution-plan-template.md`
  A research-specific execution loop focused on evidence gathering and decision
  closure, not feature delivery.
- `research-evidence-log-template.md`
  Structured place for findings, comparisons, experiments, open questions, and
  disconfirming evidence.
- `research-decision-template.md`
  Final outcome: accept, reject, defer, or reshape.
- `research-prompt-template.md`
  Start prompt for an agent conducting the research loop.

## How This Differs From Delivery Loops

Feature research is not a delivery loop in disguise.

Its primary output is a justified decision, not implementation progress.

That means:

- weak ideas must be allowed to fail;
- disconfirming evidence must be captured, not hidden;
- architecture mismatch is a valid rejection reason;
- "can be prototyped later" is not a research conclusion;
- if implementation becomes the only way to answer a question, that must be
  made explicit as a follow-up execution path, not smuggled in as research.

## Mandatory Source Of Truth

Before using these templates for a real research loop, validate the idea
against:

- `docs/system-concept.md`
- `docs/system-frame.md`
- `docs/system-properties.md`
- `docs/canonical-network-foundation.md`
- `docs/engineering-constraints.md`
- `docs/development-contract.md`
- relevant domain requirements
- `docs/reference-invariants.md` if the idea touches
  network/discovery/messaging/publication foundation
- `docs/qa/test-model.md` if the idea changes the required QA or test shape

If the idea conflicts with these documents, the research result must say so
explicitly. It must not silently create a second product direction.

## Completion Rule

Research is complete only when there is an explicit decision with evidence.

Allowed terminal outcomes:

- `accepted`
- `rejected`
- `deferred`
- `reshaped`
- `blocked`

"Needs more thought" is not a terminal outcome.
