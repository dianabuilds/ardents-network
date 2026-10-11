# Current documentation

Read [product scope](product/scope.md), the [threat model](security/threat-model.md)
and [domain language](../CONTEXT.md), then the owner relevant to the operation.
Accepted ADRs outrank product/security contracts; source and historical experiments
cannot silently change either. [Documentation policy](development/documentation.md)
defines promotion and retirement.

The selected contract is a closed Ubuntu text-Service by Target Link, with one
common protection baseline and TCP/TLS plus QUIC Carriers. Component implementations
and their receipts do not establish a complete installed Service, platform support,
independent security validation, anonymity or public-operation qualification.
Service Names remain a future product function; the accepting Namespace and Alpha
Link routes are retired. Public transport, storage, consensus and blockchain are
unselected. The [scope](product/scope.md) owns these boundaries.

## Owners

| Question | Current owner |
|---|---|
| Selected journeys, outcomes and limits | [Scope](product/scope.md), [protected workload](product/protected-service-workload.md), [journeys](product/journeys.md); [functional map](product/functional-map.md) owns requirements and budgets, [operating model](product/operating-model.md) owns lifecycle |
| Protected information and actual claim limits | [Threat model](security/threat-model.md), [common privacy construction](technical/common-privacy-architecture.md) |
| Domain rules, collaborations, state and completion | [Domain map](development/domain-map.md); actual Go imports belong to the [package map](development/package-map.md) |
| Authenticated Network facts | [New Network](technical/successor-network-state.md), [Network/Node contract](technical/network-route-node.md) |
| Finite rights and physical capacity | [Admission](technical/successor-admission-boundary.md), [private admission](technical/private-admission.md), [Hosting](technical/successor-hosting-budget.md) |
| Protected physical transport | [Protocol](technical/protected-route-protocol.md), [Prefix/channel](technical/successor-route-prefix.md), [Introduction](technical/successor-route-introduction.md), [JOIN](technical/successor-route-join.md), [integration boundaries](development/route-delivery-join-boundary.md) |
| Local permission, original Job and worker cleanup | [Execution](technical/application-execution.md), [confinement](technical/application-confinement.md) |
| Publication, exact-Target proofs and logical Connection | [Service lifecycle](technical/endpoint-service-runtime.md), [Reachability](technical/private-reachability.md), [domain map](development/domain-map.md) |
| First trust, Release, Installation and Custody | [Enrollment](technical/enrollment-verification.md), [Release/update/Custody](technical/release-update-custody.md), [Installation owner](../internal/successor/installation/README.md) |
| Disclosure, commands and operator procedure | [Alpha-control](technical/alpha-control-transition.md), [commands](reference/commands.md), [Portable enrollment](reference/portable-enrollment.md), [qualification Release procedure](operations/qualification-release.md) |
| Engineering, dependencies and evidence profiles | [Development](development/README.md), [dependencies](development/dependencies.md), [testing](development/testing.md), [privacy acceptance](development/privacy-qualification.md) |

## Decisions, evidence and execution

[Current decisions](adr/README.md) index surviving selections. The
[archive](archive/README.md) retains selected decision evidence, unresolved
findings and exact audit receipts, with disposition, current owner and source
identity. Other historical editions and retired plans are recoverable from its
pinned Git snapshot. An archived proposal is not a contract;
archiving an accepted edition does not withdraw its explicitly retained rules.
The [research queue](research/questions.md) distinguishes undecided questions
from selected execution; read its evidence only for a named decision.

GitHub [#497](https://github.com/dianabuilds/ardents-network/issues/497) owns the
domain completion plan, dependencies and acceptance criteria; the C0 milestone
owns live work. The current human instruction keeps Connection working and holds
new implementations. Neither an old plan nor available WIP capacity authorizes
starting the next domain. The [execution workflow](development/agent-execution.md)
and [Route migration contract](development/route-migration-contract.md) remain
mandatory within authorized scope.
