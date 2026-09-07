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
| [R-148](records/r-148-shared-resource-contributor-benefits.md) | Which concrete products using shared resources could motivate useful relay contribution, and how would people use them? | **Open; exploratory catalogue prepared on 2026-09-06.** The [product catalogue](../product/future-product-catalog.md) describes 18 user-facing concepts and possible contributor benefits without selecting priorities. Accounting, funding, privacy and recovery remain separate questions. No mechanism, experiment, C0 research execution or implementation slice selected. |
| [R-147](records/r-147-contributor-resource-controls.md) | Which owner-controlled resource policy could make co-resident relay contribution acceptable on a personal device while preserving bounded work and useful network capacity? | **Open; theoretical comparison completed on 2026-09-06.** Recommends measured presets, editable ceilings and bounded adaptation, with optional scheduling and quota pacing. No co-resident profile, experiment, C0 research execution, or implementation slice is selected. |
| [R-149](records/r-149-autonomy-transition.md) | How can Ardents meet its public product tasks in a potentially fully hostile environment without indispensable appointed administration or weakened owner rights? | **Open; voting-core composition assessed on 2026-09-07.** The [focused assessment](records/r-149-voting-core-design.md) records the no-cryptoasset/public-pseudonym choices, component comparison, executed conditional sampling/turnout/workload envelope and five unresolved architecture gates. A [conditional development/verification map](records/r-149-voting-work-packages.md) prepares the handoff; no task is implementation-ready or activated. Identity-splitting neutrality, actual admission/close/bootstrap and joint budgets remain unresolved. Canonical Names/View and no-disclosure remain binding; no public protocol, human authority or live network experiment selected. |
| [R-137](records/r-137-c0-stress-test.md) | Did the pre-C0 Linux stress spike reveal a maintained-candidate defect? | **Deferred.** The partial runner isolated a test-lifecycle artefact but did not produce a clean exact-candidate stress result. Reopen only with a selected C0 Linux budget and external evidence root. |
| R-138 | Can a bounded real-concurrency simulation validate the maintained multi-agent loop without claiming production coordination or substitute-user validation? | **Deferred outside C0.** The experiment's smoke slice and S3.6 retry logic remain unaccepted evidence; they create no C0 requirement or claim. Reopen only after a Product Owner selects this decision over C0 readiness work. |
| R-139 | Can a coordinator constrain goal-driven agents to stable consumer identities and immutable decisions while preserving real Source acceptance and an expected invalid-State rejection? | **Deferred outside C0.** S3.6.5 locally qualifies the repaired coordinator, but four-persona concurrency and any cap-exhaustion security claim remain separately gated. Reopen only after a Product Owner selects this decision over C0 readiness work. |

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
R-140 is decided and promotes the current `quic-go v0.62.0` direct dependency
to [ADR-0048](../adr/0048-maintain-tcp-and-quic-carriers.md) and the
[dependency register](../development/dependencies.md). Its external
carrier-profile matrix remains a pre-freeze C0 gate rather than an inherited
v0.61.0 result.
R-141 is decided: Authority-signed early supersession needs an unselected
Authority-currentness transport and discovery topology, so C0 retains the
expiry-gated successor limitation. This is not a permanent product requirement;
reopen only as one separately selected research question with a complete
topology, trust, privacy, availability, rollback, and operator matrix.
R-142 is decided: C0 retains explicit interactive Service Credential issuance
and no automatic renewal. Existing one exact-request retry and an intentionally
prepared future Credential do not create an unattended issuer, currentness, or
restart guarantee. Reopen only with a separately selected finite-renewal
authority, durable-state, time/revocation, topology, and operator decision.
R-143 is decided: C0 renews Source server/client leafs under the same key, CA,
hostname, and pins through controlled process restart. It selects no issuer,
hot reload, key-pin, or CA rotation; expired material admits no new connection
and recovers only from externally authenticated replacement input.
R-144 is decided and promoted to ADR-0071: C0 selects H1, an offline
recipient-enrollment issuer and one exact sealed response, with a separate
offline challenge-bound time witness. The witness has a purpose-separated trust
key, answers a fresh local challenge, and is accepted only inside one bounded
monotonic request window; a restart without that continuity requires a fresh
response. This direction is not implementation authority: its Entry, Source,
command, package, artifact, and operational owner contracts must be accepted
before one C0 implementation issue may be selected.
R-145 is decided and promoted to ADR-0072: C0 replaces the bearer Route/Entry
v1 profile with recipient-bound Route/Entry v2, adds the offline
`ardents-enrollment` artifact, and gives State the verified time-witness
receiver boundary. Its exact grammars and tests are implementation work for
the selected C0-05 issue; neither v1 fallback nor another active research
question is created.
R-146 is decided for the public authority direction under ADR-0074: owner
authorization, locally checked rules, minimal open agreement, and voluntary
protocol adoption. Its unresolved mechanism and migration decisions now belong
to R-149; no consensus, token, external chain, or public-operation claim follows
from acceptance of that direction.

[R-150](records/r-150-common-protection-baseline.md) is decided for one common
protection baseline and the product/technical workstream through integrated
implementation and test-environment trials under
[ADR-0077](../adr/0077-evolve-one-common-protection-baseline.md). The
[operating model](../product/operating-model.md#common-protection-baseline) and
[engineering workflow](../development/documentation.md#system-protection-workstream)
own the selected direction. This closes its framing question, not the security
workstream; it selects no mechanism or C0 slice and does not activate another
C0 research execution alongside the current R-149 design subject.

[R-151](records/r-151-privacy-anonymity-integration.md) is decided for the
successor integration map requested on 2026-09-07. Its
[current development owner](../development/privacy-anonymity-map.md) covers
18 areas, 10 coupled design gates, conditional implementation work and test
evidence, including the complete current module/package inventory. It selects
no anonymity mechanism, library or additional C0 execution; the scheme's
decision gates remain to be resolved through separately selected research.

Closed research is not a second specification. Start from current
[product](../product/), [security](../security/), [technical](../technical/),
[reference](../reference/), [development](../development/), and accepted
[ADR](../adr/) owners, then consult a completed record only for decision
provenance.
