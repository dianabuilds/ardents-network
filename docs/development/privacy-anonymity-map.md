# Privacy and anonymity implementation path

Status: **selected closed design and component coverage map** under
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) and its
[authenticated child restriction amendment](../adr/0082-bind-bootstrap-restriction-to-node-child.md).
GitHub Issues in the C0 Closed Alpha milestone owns execution and status.
This document is the reader route and coverage map, not a second task ledger.

## One selected path

First implement and qualify the complete closed text-Service journey by
Target Link on Ubuntu. It includes real enrollment/State/time inputs, private
admission and reachability, publication, both protected legs, end-to-end
Service authentication, confined reader and Publisher, resource exhaustion,
recovery, migration and controlled installation. No public autonomy,
canonical Name producer, general browser or Windows confinement is assumed.

Future canonical naming needs its real authority/close producer, and public
autonomous eligibility/admission needs a separately selected R-149 design.
Neither is a next implementation authorized by this map. Neither can inherit
closed operator appointments as a permanent public authority. Stronger
traffic-correlation protection is a separately measured claim that may revise
the one common construction; it is not a parallel privacy mode.

## Authoritative reading route

| Question | Current owner |
|---|---|
| User outcome, Link-first scope, supported platform and visible failures | [Protected Service workload](../product/protected-service-workload.md) |
| Protected facts, adversaries, conditions, measurement and honest limits | [Closed successor claim contract](../security/threat-model.md#closed-successor-claim-contract) |
| Whole construction and receiving ownership | [Common architecture](../technical/common-privacy-architecture.md) |
| Exact profile, selection, lane/control bytes, introduction and Connection binding | [Protected forwarding protocol](../technical/protected-route-protocol.md) |
| Permissions, token/SPKI composition, bootstrap, debit/spend and compromise | [Private admission](../technical/private-admission.md) |
| OS boundary, installed units, local interfaces, workers and cleanup | [Application confinement](../technical/application-confinement.md) |
| Selected components and continued acceptance | [Dependency register](dependencies.md) |
| Parameters, complete test matrix and state/artifact migration | [Qualification](privacy-qualification.md) |
| Why these choices and what the experiments actually establish | [R-152 contract assessment](../archive/research/records/r-152-closed-scheme-contract.md) |

Current Network, Endpoint, Reachability, Application and Release owners state
their retained implementation and compatibility bounds; the domain map names
new owners and collaborations. There is no maintained accepting Naming owner.
Exact wire/root editions belong to their current technical contracts, without
a shared delivery-generation label. These owners do not select another live
Route or require copying old campaign implementations.

## Dependency disposition map

The [dependency register](dependencies.md) owns exact admitted versions,
findings and continued acceptance; `go.mod` and the build/tool inventories
record actual candidate inputs. This map does not maintain a second version
register. Historical exact-version receipts cannot qualify changed inputs.

| Component or obligation | Selected disposition |
|---|---|
| Go toolchain, standard TLS/RSA/Ed25519, CIRCL and quic-go | Use the current admitted baseline and reviewed HPKE/blind-token uses from the dependency and technical owners; retain both selected Carriers and qualify the same inner composition on each. No generic cryptographic framework or private fork. |
| Other runtime modules and build/test tools | Review the actual dependency closure and current advisories under the register's acceptance rule. Removed Naming/OHTTP dependencies do not remain obligations of the selected Link-first path. |
| Ubuntu/kernel/systemd | Use the selected supported family and required mechanisms; record exact security-updated package inventory and installed-artifact tests for each candidate. WSL is mechanism evidence only. |
| Source, State, time, Enrollment and Custody | Preserve independently pinned authorities and monotonic floors. Current authenticated owners, not caller-made validity flags, supply accepted facts. |
| Public Name production and autonomous admission | Separate unselected design, not a hidden dependency or pretend service inside the Link-first implementation. |
| Mixing networks, Sphinx, traffic shuffling, general sandboxes | Not selected dependencies. Retain only decision-relevant research evidence; no implementation tickets for speculative candidates. |

## Implementation order and coverage

[#497](https://github.com/dianabuilds/ardents-network/issues/497) owns the live
domain order, selected work and exact transition conditions. The
[domain map](domain-map.md) owns semantic dependencies and exclusions. The table
below explains component prerequisites, not a separate issue queue or acceptance
checklist.

| Consumer boundary | Required genuine components | Current contract owner |
|---|---|---|
| Protected Route transport | Authenticated Network observations, finite Admission rights/spend and physical Hosting reservations | [Route protocol](../technical/protected-route-protocol.md), [migration contract](route-migration-contract.md) |
| Qualified local operation | Authorized installed bytes and native invocation checks; separate Execution session, Job and joined original worker | [Installation](../../internal/successor/installation/README.md), [Execution](../technical/application-execution.md), [confinement](../technical/application-confinement.md) |
| Accepting publication | Live Execution operation, purpose-scoped Instance, Route REGISTER and independent Reachability Store ACK | [Publication/Service](../technical/endpoint-service-runtime.md), [Reachability](../technical/private-reachability.md) |
| Authenticated logical Service stream | Exact Target proof and recipient, live original Execution, Publication authority and existing Route delivery/JOIN | [Connection/Service](../technical/endpoint-service-runtime.md), [delivery/JOIN](route-delivery-join-boundary.md) |
| Complete installed journey | Accepted Service consumers, genuine installed manager/artifact inputs and the full interruption/protection matrix | [Installation](../../internal/successor/installation/README.md), [qualification](privacy-qualification.md) |

Installation's existing artifact/startup component supports Execution and
Connection without requiring full Installation acceptance. Connection consumes
existing Route transport; full Route acceptance then requires the genuine
authenticated Connection consumer. Full Installation acceptance follows that
integrated Service boundary. These are different result levels, so no pair of
issues waits for the other's full closure. Exact source-matched gates and the
current implementation hold remain in #497.

Installation grants no Job, Local Grant or Service readiness; Route grants no
Instance authentication or logical recovery. A delivered capsule, Store ACK,
local Interface conformance or successful standalone stream test cannot replace
the corresponding live owner. The trusted text clients use AAI3 and bounded
Administration snapshots; actual Endpoint composition must still retain the
original authorized Job and enforce those owners at effects and final handoff.

The retained network capacity workload is broader than the 4 MiB text job.
Its declared harness is a test input using real Service streams, never a
second supported unconfined Application. Every implementation slice includes
its own behavior and hostile-boundary tests; final trials do not postpone all
security checks until the end.

## Verification and test environment map

[Qualification P1–P11](privacy-qualification.md#acceptance-matrix) owns the
whole-candidate privacy acceptance matrix. Domain acceptance and transition
conditions belong to #497 and its selected child issues; a component pass does
not satisfy P1–P11. Test the real product commands and deep Modules. Fix exact
source/artifact/profile identities, role/family placement, platform,
impairments and prerequisites before collecting results. Real common operator
control is disclosed even when synthetic role facts exercise a positive path.

Use deterministic parsers/ledger tests for receiving boundaries, process tests
for actual command composition, and the exact Ubuntu installed-artifact profile
for confinement and the final journey. Exercise both accepted Carriers.
Cold/warm trials preserve durable quotas and authority floors; long workloads
retain NET-14AD and the complete NET-14 capacity/recovery requirements.
The easier no-loss latency cell cannot replace the retained normal cell.

Protocol-field disclosure, forged authority, unconfined admission, replay,
lost floors, unbounded work or a missed required budget fails qualification.
P9 records residual correlation diagnostically; it does not turn a successful
attack into a claim pass or a failed attack into an anonymity proof. An
unavailable prerequisite invalidates the affected run.

## Change and handoff rules

The [repository WIP policy](../../AGENTS.md#c0-work-in-progress-limit) owns
the selected count; available capacity does not authorize another slice. Follow
the [current selection](agent-execution.md#current-selection-takes-precedence)
and #497 before implementation. R-152 and R-099 have completed their bounded
selections. R-149 remains separate future design, not simultaneous
implementation. No additional human team is assumed.

An executor may choose routine internal algorithms and file splits within the
contract. A new authority, weaker condition, changed privacy relation, wire
generation, platform or required budget returns to design before dependent
implementation. Update current owners with each implementing change and retire
only behavior whose compatibility obligation has actually been replaced.
