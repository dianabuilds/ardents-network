# Common privacy architecture

Status: **selected successor construction** under
[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md).
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) completes
the closed Link-first text-Service design contract. Selection
does not qualify an implementation or change the currently running C0 bytes.

## Reader route and authoritative owners

Read the [selected workload](../product/protected-service-workload.md), the
[threat model](../security/threat-model.md#whole-system-protection-review),
then the affected contract below. This page owns the composition and ownership
relationships; detailed formats and numbers belong to their linked owners.

| Question | Current successor owner |
|---|---|
| What the User and Publisher can do, on which platform, and what refusal means | [Protected Service workload](../product/protected-service-workload.md) |
| How roles authenticate, select and join protected channels; exact wire and recovery binding | [Protected forwarding protocol](protected-route-protocol.md) |
| What permits issuance, forwarding and publication; spending and crash semantics | [Private admission](private-admission.md) |
| Which local processes and effects are confined; installation and Application attachment | [Application confinement](application-confinement.md) |
| Which implementation uses are selected and what evidence admits a candidate | [Dependency register](../development/dependencies.md) |
| Parameters, accounting, test environment, acceptance and generation migration | [Privacy qualification](../development/privacy-qualification.md) |
| How the complete work is sequenced and covered | [Integration and delivery map](../development/privacy-anonymity-map.md) |
| Why this construction was selected and what the probes demonstrate | [R-152 research](../research/records/r-152-common-privacy-design.md) |

The current [Network/Route/Node](network-route-node.md) and
[Endpoint/Service](endpoint-service-runtime.md) documents describe implemented
C0 behavior. Read them for retained interfaces and migration inputs; their old
wire is not a template for successor information exposure. The factual
[package map](../development/package-map.md) changes with implementation.

## Selected construction

~~~text
User Endpoint -> User Entry -> User Interior -> Rendezvous
                                                ^
Publisher Endpoint -> Service Entry -> Service Interior
~~~

Each endpoint selects its own leg. The two legs join at one fresh
Connection-specific Rendezvous. Application bytes are bidirectional and
additionally protected by end-to-end Service TLS with exact Target/Instance
authentication. Introduction carries setup only, with the Rendezvous, join
secret and Connection/Attachment binding inside a Service-only HPKE capsule.

Endpoint-to-role confidential TLS channels use this same forwarding
Implementation for Name resolution, private reachability, admission and
publication. Each private operation has a fresh terminal channel; an allowed
context-scoped prefix is not a reusable multi-Target Gateway session.
Pre-Route Source work carries bounded public authority/build/time evidence.

Maintain the selected TCP/TLS and QUIC Carriers behind the same Route
Interface. The receiving Node's authenticated facts select one for each
attempt. Neither a peer nor failure selects a weaker generation, shorter
path, ordinary DNS, direct Service access or an alternate Target.

Traffic follows useful work and necessary control, refresh and liveness.
There is no autonomous filler stream. Event-specific shaping, setup, failed
attempts, relay receive-plus-forward work and idle readiness all enter the
cost calculation. Resource or provider-allowance exhaustion stops new work
and bounds existing work under the same protection.

## One-use admission

Use RFC 9578 type-2 blind tokens through CIRCL's selected RFC 9474 implementation.
The [admission owner](private-admission.md) fixes the exact encoding, resource
classes, issuance rights, receiver binding, durable debit/spend and limits.

For the selected closed network, real independently pinned State and offline
permissions supply authority. This resolves the closed implementation's
authority source; it does not implement public eligibility, scarce issuance
or autonomous agreement. Those require an accepted R-149 successor contract.
A stolen issuer key can mint extra signatures; honest receiver limits still
bound local work, without promising fair availability or Sybil resistance.

## Observation and claim contract

Use the threat model's five-part claim format: protected information,
adversary, surviving conditions, measurement and honest limitation. Required
properties include payload/Target authentication, per-role field separation,
context scoping, confined ordinary-network effects, finite work and explicit
failure. Inspect whole role inputs and retained state across control, data,
restart and recovery.

The Product Owner accepts the initial residual risk of both-end timing/volume
correlation and proceeds without an autonomous useless-traffic generator.
Correlation and activity/recognizability measurements characterize limitations;
they do not create a quantitative anonymity claim. A new protocol-field link,
direct escape or violated required property remains an acceptance failure.

The selected Ubuntu text job covers both Application process trees.
Kernel, privileged host, Endpoint verification or launcher compromise defeats
claims relying on that boundary. Controlled roles and synthetic family values
do not prove independent control, public anonymity or public availability.

## Ownership and integration

| Existing owner | Responsibility in the successor |
|---|---|
| Endpoint and Broker | Local authorization, verified Application attachment, parent resource/cancellation tree and composition |
| State and Source | Actual current authority/profile verification, conflict floors and public evidence acquisition |
| Entry, Route and Node | Endpoint-owned selection, confidential forwarding, admission use, finite lanes and joined cleanup |
| Credential and Duty | Issuance protocol, authoritative debit/spend, role/window binding and restart behavior |
| Naming and Reachability | Canonical independent proofs, exact Target, currentness, scoped caches and private terminal exchange |
| Publication and Instance | Independent Introduction key/slot lifecycle, publication revision, withdrawal and non-exporting Instance custody |
| Service Connection | Immutable logical connection, fresh Attachment binding, authenticated byte order and bounded continuity |
| Application and installation owners | Bounded text job, verified confinement, artifacts, local effects and whole-tree cleanup |
| Resource, Release and diagnostics | Complete cost, owner-controlled adoption, safety floors and finite local observations |

A small Interface remains with the owner of its state and effects. No generic
security service, parallel proxy product, public crypto fork or speculative
package is selected. Selected future Application additions belong in the
confinement contract and require real callers/tests when implemented.

## Implementation handoff

The selected closed contract now satisfies the design contents of the
[task-admission rule](../development/documentation.md#task-admission-after-completed-design):
coherent wire/state transitions, real admission authority, complete local
effects, selected dependency uses, causal cost model, migration and executable
acceptance design. The integration map is the route into its dependency-ordered
implementation tasks; research
records retain decision evidence rather than an alternative specification.

The supported job and Ubuntu platform have already been selected. Full public
autonomy, every future Application, a general browser, Windows confinement and
complete resistance to a Broad Traffic Observer are not prerequisites for this
closed scope. Required protection inside the selected scope cannot be deferred
to an implementer's judgment. The design is selected; actual component updates, full wire vectors,
whole-system measurements and installed-product
qualification require the eventual implementation and belong to its acceptance.
