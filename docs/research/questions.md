# Open research queue

Research is temporary decision work. Once a question is decided, its current
contract is promoted to an ADR, product, security, technical, reference, or
development owner. The completed record remains provenance under
`docs/research/records/`, but does not remain in this active route.

An open record must state a decision-relevant question, falsifiable hypothesis,
inputs, evidence, limitations, and the decision it can unlock. Delivery labels
inside historical records are provenance only and authorize no implementation.

| ID | Open question | Current status |
|---|---|---|
| [R-099](records/r-099-protected-application-profile.md) | Is there one narrow application job and supported platform on which an OS-enforced or brokered boundary can deny ordinary-network escape well enough to support a bounded Application-level location claim? | **Open; no profile selected.** Generic Browser and local Application adapters make no isolation or privacy claim. Work begins only after the Product Owner selects one exact job, platform, adversary, enforcement boundary, falsification matrix, and resource budget. |
| R-139 | Can a coordinator constrain goal-driven agents to stable consumer identities and immutable decisions while preserving real Source acceptance and an expected invalid-State rejection? | **Open; S3.6.5 locally qualifies the repaired coordinator.** Four-persona concurrency and any cap-exhaustion security claim remain separately gated. Owned by `experiments/multi-agent-real-2026-09-04/`. |

R-093 is deferred without a selected co-resident Contributor experiment and is
not active work. R-097, R-105, R-113, R-129, and R-134 are decided and
implementation-linked; their maintained contracts are owned respectively by
[Naming](../technical/naming.md), [Endpoint and Service runtime](../technical/endpoint-service-runtime.md),
[Alpha-control transition](../technical/alpha-control-transition.md), the
[C0 product scope](../product/scope.md), and
[ADR-0067](../adr/0067-retire-completed-local-alpha-ceremonies.md).
R-135 is decided and promoted to [ADR-0068](../adr/0068-bind-transit-issuer-roots-to-state-generation.md);
its maintained contract belongs to [Transit Grant acquisition](../technical/transit-grant-acquisition.md).
R-136 is decided and promoted to [ADR-0070](../adr/0070-own-volatile-user-route-orchestration.md);
its maintained Route/Endpoint boundary belongs to [Network Route and Node](../technical/network-route-node.md)
and [Endpoint and Service runtime](../technical/endpoint-service-runtime.md).

Closed research is not a second specification. Start from current
[product](../product/), [security](../security/), [technical](../technical/),
[reference](../reference/), [development](../development/), and accepted
[ADR](../adr/) owners, then consult a completed record only for decision
provenance.
