---
status: accepted
date: 2026-09-07
---

# Select the common split-circuit privacy architecture

The Product Owner requests selection or creation of the common privacy scheme.
[R-152](../research/records/r-152-common-privacy-design.md#architecture-selection-and-component-evidence)
selects the successor's architecture: endpoint-selected split circuits,
recipient-confidential control, one-use admission and traffic driven by useful
work. The [technical owner](../technical/common-privacy-architecture.md) fixes
its information flow and the unresolved design boundaries.

Preserve the five logical data positions and the ordinary authenticated
Service stream. Use Go TLS 1.3 for protected forwarding and end-to-end Service
channels. Keep Introduction's Rendezvous and join material inside the
Service-only HPKE capsule, and replace successor OHTTP transport with
forward-secret confidential role channels while retaining independent
authority validation. The selected admission family is publicly verifiable
blind tokens; its exact software use, issuer entitlement and compromise bounds
remain gated by dependency acceptance and R-149.

This composition preserves existing deep ownership and avoids maintaining a
new packet cipher or embedding another complete network and its authority
system. Continuous mixing, constant full-capacity filler and experimental
traffic shuffling are not the selected foundation. A narrowly targeted
additional defence requires its own evidence before entering the common
profile. There is no parallel stronger mode or weaker accepting fallback.

## Scope and consequence

This accepts an architecture for successor design and bounded experiments.
It does not accept a public wire/suite, a new runtime dependency, an R-099
Application profile, a released implementation or an anonymity claim.
**Product Owner clarification, 2026-09-07:** proceed with this initial
construction despite its residual traffic-correlation limitation, without an
autonomous generator of useless traffic. This resolves permission to develop
the chosen direction; complete resistance to both-end or active correlation
is not a prerequisite for its first implementation contract. It does not
qualify anonymity, establish a quantitative resistance level, or waive
confidentiality, role separation, confinement or the assessment of combined
observations. Any new attack that violates a claimed property still returns
the affected composition to design. Stronger correlation claims require a
separate explicit decision and evidence.

Current C0 records, accepted protocol behavior and durable state remain
unchanged until an explicit researched migration replaces them. At this
architecture decision, R-152 remained open for the complete product, protocol
and qualification contract; this decision did not start implementation.

[ADR-0081](0081-select-closed-protected-service-contract.md) subsequently closes
that bounded design scope: exact closed admission, forwarding, Application and
qualification contracts are selected for implementation. The earlier open
clauses above describe this decision's original scope, not additional blockers
for that closed work. Public autonomy and anonymity qualification remain
outside the closed selection.
