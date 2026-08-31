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

R-093 is deferred without a selected co-resident Contributor experiment and is
not active work. R-097, R-105, R-113, and R-129 are decided and
implementation-linked; their maintained contracts are owned respectively by
[Naming](../technical/naming.md), [Endpoint and Service runtime](../technical/endpoint-service-runtime.md),
[Alpha-control transition](../technical/alpha-control-transition.md), and the
[C0 product scope](../product/scope.md).

Closed research is not a second specification. Start from current
[product](../product/), [security](../security/), [technical](../technical/),
[reference](../reference/), [development](../development/), and accepted
[ADR](../adr/) owners, then consult a completed record only for decision
provenance.
