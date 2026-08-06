---
id: R-NNN
title: Short decision-relevant question
status: open
owner: unassigned
started: YYYY-MM-DD
reviewed: YYYY-MM-DD
---

# R-NNN — Question

## Decision this unlocks

Name the product, protocol, security, or technology decision that cannot be made
responsibly without this research.

## Current contract

Link the relevant product journey, threat-model section, glossary terms, and
accepted ADRs. State what is already fixed and what remains open.

## Hypotheses

- **H1:** A falsifiable statement.
- **H2:** A competing falsifiable statement.
- **H0:** None of the evaluated options meets the contract.

## Evaluation criteria

Define thresholds or comparison rules before evaluating candidates. Include:

- exact user outcome;
- protected information and adversary;
- latency, bandwidth, storage, and availability budget;
- active attacks and failure recovery;
- operator and governance dependencies;
- implementation maturity, audit history, misuse resistance, and maintenance;
- license and distribution constraints;
- accessibility and developer-experience costs.

## Evidence plan

### Primary sources

List specifications, papers, official documentation, advisories, and relevant
source code with access dates.

### Experiment

Describe environment, inputs, procedure, collected artifacts, and how another
person can reproduce or falsify the result. Link a directory under
`experiments/` when code is necessary.

### Failure scenarios

List malicious, degraded, recovery, and governance cases rather than evaluating
only the happy path.

## Findings

Label every item as **Sourced fact**, **Measurement**, **Assumption**, or
**Inference**.

## Options

For each option, record product fit, security fit, operational dependencies,
governance roots, implementation risk, and reasons it may be rejected.

## Recommendation

State one of:

- choose an option;
- run a named follow-up experiment;
- change the product contract;
- choose none of the options yet.

Include confidence and the strongest argument against the recommendation.

## Disposition

List the question state, accepted follow-ups, documents changed, any ADR created,
and whether experiment code should be retained, rewritten, or deleted.
