# Current documentation

Read [product scope](product/scope.md), the [threat model](security/threat-model.md)
and [domain language](../CONTEXT.md), then the owner relevant to the operation.
Accepted ADRs outrank product/security contracts; source and historical experiments
cannot silently change either. [Documentation policy](development/documentation.md)
defines promotion and retirement.

The closed Ubuntu C0 profile covers document Services and bidirectional
Application streams by Target Link under one common protection baseline.
ADR-0136 returns QUIC to the nearest closed release in stages; until the
return is qualified, TCP/TLS is its sole Carrier and a QUIC selection refuses
before effects. Qualification applies to the
exact candidates and scenarios exercised. The scope defines the limits on
platform support, independent security validation, anonymity and public
operation.
Service Names remain a future product function; the accepting Namespace and Alpha
Link routes are retired. Public transport, storage, consensus and blockchain are
unselected. The [scope](product/scope.md) owns these boundaries.

## Owners

| Question | Current owner |
|---|---|
| Selected journeys, outcomes and limits | [Scope](product/scope.md), [protected workload](product/protected-service-workload.md), [journeys](product/journeys.md); [functional map](product/functional-map.md) owns requirements and budgets, [operating model](product/operating-model.md) owns lifecycle |
| Protected information and actual claim limits | [Threat model](security/threat-model.md), [common privacy construction](technical/common-privacy-architecture.md) |
| Domain rules, collaborations, state and completion | [Domain map](development/domain-map.md), including the [interaction contract](development/domain-map.md#inter-domain-interaction-contract) and [planning checklist](development/domain-map.md#interface-checklist-for-development-plans); actual Go imports belong to the [package map](development/package-map.md) |
| Authenticated Network facts | [New Network](technical/successor-network-state.md), [Network/Node contract](technical/network-route-node.md) |
| Finite rights and physical capacity | [Admission](technical/successor-admission-boundary.md), [private admission](technical/private-admission.md), [Hosting](technical/successor-hosting-budget.md) |
| Protected physical transport | [Protocol](technical/protected-route-protocol.md), [Prefix/channel](technical/successor-route-prefix.md), [Introduction](technical/successor-route-introduction.md), [JOIN](technical/successor-route-join.md), [integration boundaries](development/route-delivery-join-boundary.md) |
| Local permission, original Job and worker cleanup | [Execution](technical/application-execution.md), [confinement](technical/application-confinement.md) |
| Publication, exact-Target proofs and logical Connection | [Service lifecycle](technical/endpoint-service-runtime.md), [Reachability](technical/private-reachability.md), [domain map](development/domain-map.md) |
| First trust, Release, Endpoint launch and Custody | [Enrollment](technical/enrollment-verification.md), [Release/update/Custody](technical/release-update-custody.md), [Endpoint launch](../internal/domain/endpoint/launch/README.md) |
| Disclosure, commands and operator procedure | [Alpha-control retirement](technical/alpha-control-transition.md), [commands](reference/commands.md), [Portable enrollment](reference/portable-enrollment.md), [qualification Release procedure](operations/qualification-release.md) |
| Engineering, dependencies and evidence profiles | [Development](development/README.md), [dependencies](development/dependencies.md), [testing](development/testing.md), [privacy acceptance](development/privacy-qualification.md) |

## Decisions, evidence and execution

[Current decisions](adr/README.md) index surviving selections. Selected
decision evidence, unresolved findings and exact audit receipts are retained
in Git history, with disposition, current owner and source identity recorded
in the pinned snapshot's
[archive index](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/README.md)
and [manifest](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/manifest.json).
Other historical editions and retired plans are recoverable from the same
pinned Git snapshot. A retired proposal is not a contract;
retiring an accepted edition does not withdraw its explicitly retained rules.
The [research queue](research/questions.md) distinguishes undecided questions
from selected execution; read its evidence only for a named decision.

GitHub [#497](https://github.com/dianabuilds/ardents-network/issues/497) owns the
domain completion plan, dependencies and acceptance criteria. The
[C0 milestone](https://github.com/dianabuilds/ardents-network/milestone/1) tracks
live work. The Product Owner's current selection and the
[execution workflow](development/agent-execution.md) govern admission.
Route changes follow the
[Route migration contract](development/route-migration-contract.md).
