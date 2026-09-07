---
status: accepted
date: 2026-09-06
amends: ADR-0004 and ADR-0006, public authority target only
---

# Target public operation without appointed network administrators

The Product Owner accepts the architectural direction evaluated in
[R-146](../research/records/r-146-non-administrative-network-consensus.md):
owner-authorized actions, locally checked rules, minimal open agreement for
necessary shared facts, and voluntary protocol adoption. The public target has
no appointed party whose continuing permission is indispensable to ordinary
participation or continued network work. Consensus may order valid actions;
it does not confer a general rule-changing, Name-seizure, forced-installation,
or network-shutdown command.

This chooses an authority boundary over permanent single-signature, threshold
administration, or broad collective on-chain governance. It does not choose a
blockchain, consensus algorithm, validator weighting, token, payment scheme,
external settlement dependency, wire format, or launch date. Canonical Service
Names and the common Candidate View remain requirements to satisfy or change
through a separate explicit decision; local gossip is not their replacement.

## Scope of the amendment

ADR-0004's appointed public thresholds and ADR-0006's mandatory publisher-issued
permission for ongoing network work are predecessor authority arrangements,
not the selected autonomous public end state. Their current implemented checks,
C0 boundaries, and accepted safety conditions remain binding until a separately
researched and accepted successor replaces the exact affected contract.
This ADR provides no authority to bypass signatures, metadata freshness,
non-decreasing floors, finite work lifetimes, or compatibility checks.

Owner Custody, owner-selected Name recovery, local resource decisions, and
finite purpose-scoped online signing remain legitimate private powers.
Software provenance and safe local activation remain necessary; release
publisher authenticity must be separated from exclusive continuing permission
to operate. Neither consensus nor an update feed can install code by itself.

## Consequences and transition

Autonomy is conditional on adequate honest resources, available authenticated
state, fresh-client/bootstrap assumptions, and compatible implementations.
Removing administrative emergency powers accepts a harder response to unknown
flaws and loss of consensus: affected work may become unavailable, and owners
may need coordinated voluntary adoption or an explicit fork. It does not promise
universal automatic recovery, anonymity, operator independence, or freedom from
resource concentration and social influence.

The [operating model](../product/operating-model.md#public-autonomy-target) owns
the selected product boundary. The [threat model](../security/threat-model.md#public-autonomy-target)
owns its claim limits. [R-149](../research/records/r-149-autonomy-transition.md)
defines migration questions and acceptance gates. Promotion replaces one exact
contract at a time, verifies the full dependent journey, and never makes
administrative and autonomous proofs interchangeable. C0 remains the bounded
project-controlled alpha; implementation status belongs to its issue tracker.
