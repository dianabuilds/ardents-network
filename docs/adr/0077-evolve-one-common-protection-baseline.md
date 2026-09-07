---
status: accepted
date: 2026-09-07
---

# Evolve one common protection baseline

The Product Owner chooses protection in ordinary use of the whole Ardents
system. [R-150](../research/records/r-150-common-protection-baseline.md) records
the contract comparison and the former Horizon 5 provenance. The intended
product has one common protection baseline, improved through explicit protocol
evolution, rather than parallel ordinary and stronger security modes.

The Product Owner also selects a full product and technical workstream:
revisit substantial functionality and architecture, integrate selected solutions
into the maintained product, and test the resulting system on a declared test
environment. Its completion requires working integrated behavior and measured
evidence; writing requirements or reviewing a model alone is insufficient.

Security and privacy design, implementation and verification accompany every
bounded product slice. Later operational evidence informs further review; its
absence does not defer current threat analysis, prevent remediation or justify
an unsafe accepting path. A release claims only protection earned by its exact
implementation and conditions.

The [operating model](../product/operating-model.md#common-protection-baseline)
owns common behavior and migration. [NET-29](../product/functional-map.md#common-protection-requirement)
records the requirement, the [threat model](../security/threat-model.md#whole-system-protection-review)
owns review coverage, and the [engineering policy](../development/documentation.md#security-work-in-an-implementation-slice)
owns delivery practice. Route Profile remains an authenticated, versioned
contract for implementation and evidence; it is not a user-selectable safety
tier. Historical identities remain subject to their exact compatibility rules.

## Consequences

Changes belong to their responsible Modules and are verified through the whole
affected journey. Preserve the Application Interface, Service Connection
semantics and independent owner authority unless an explicit researched decision
changes the affected contract. Separate Node roles, bounded recovery and the
accepted Carrier set do not create different product protection levels.

One baseline must accommodate useful operation and finite owner-controlled
resources. A proposed protection needs measurable benefit and complete cost;
existing budgets remain binding until explicitly replaced. Missing required
protection yields bounded unavailability, not a weaker mode. Adoption and any
compatibility interval require a separate migration decision and remain
voluntary under ADR-0074; this ADR grants no forced-update power.

This replaces the future optional stronger-Route direction in product wording.
It selects no anonymity mechanism, topology, padding, mixing, new public wire,
runtime or implementation slice. Current C0 selections, limitations and gates
remain binding. No current candidate acquires a stronger security claim.
