# Active research queue

Research is temporary decision work. Once a question is decided, its current
contract is promoted to an ADR, technical documentation, or both; the research
record is removed from the working tree. Git retains the source history when
that provenance is genuinely needed.

Product scope determines whether an active question authorizes implementation.
An active record must state a falsifiable hypothesis, inputs, evidence,
limitations, and the decision it can unlock.

| ID | Question | Current status |
|---|---|---|
| [R-092](records/r-092-native-node-operating-profile.md) | Which measured Linux operating profile can admit one native `ardents-interactive-route-v1` Node role without reusing H3 capacity? | **Active.** The disposable mTLS/LegBinding harness is only a preannouncement baseline. A profile requires complete role-carriage, resource, pressure, drain, and reference-host evidence. |

## Closed work

Closed research is not an active queue and does not remain as a second
specification. Use the current [product](../product/),
[technical](../technical/), [security](../security/), and [ADR](../adr/)
documentation. The repository history records prior research, experiments, and
stage material.
