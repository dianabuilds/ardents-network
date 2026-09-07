---
id: R-149
title: Replace administrative dependencies without weakening network safety
status: open
owner: Product Owner and Codex
started: 2026-09-06
reviewed: 2026-09-07
---

# R-149 — How does Ardents reach autonomous public operation?

## Decision this unlocks

Turn the Product Owner's accepted R-146 direction into a bounded migration:
which authority functions must change, what replaces their safety contribution,
which decisions precede implementation, and how to qualify retirement of each
administrative dependency. [ADR-0074](../../adr/0074-target-non-administrative-public-operation.md)
accepts the public authority boundary, not the detailed mechanisms below.

This record contains a proposed transition design and dependency gates. It is
not a second C0 backlog: no phase is an in-progress implementation issue, no
live network experiment is running, and no delivery dates, external operators,
miners, auditors, or
additional staff are assumed. Only the selected tracker owns live work status.

## Design phase and intended implementation handoff

**Product Owner direction, 2026-09-07:** continue product research, requirements,
architecture and ADR preparation for this system in the same repository with
the design assistant. Intended maintained implementation belongs to Terra once
a bounded slice is ready. This does not add staff, start another model task or
activate a C0 slice. The current work is design; the existing calculators are
limited analytical evidence, not a maintained agreement-system implementation.

The [collaboration policy](../../../AGENTS.md#agreement-system-design-and-implementation-responsibilities)
and [engineering handoff](../../development/documentation.md#research-to-implementation-handoff)
own the division of work and readiness evidence. Requirements remain with their
product/security owners; consequential trade-offs receive researched ADRs under
the existing authority order. Proposed mechanisms and a draft implementation
brief are not accepted decisions merely because another model will write code.

The current design priority is the
[participant-selection and voting core](#participant-selection-and-voting-core-readiness),
following the Product Owner's clarification on 2026-09-07. Establish its coherent
architecture and dependency contracts before implementing individual parts;
application scenarios can then be assessed and attached through a bounded
contract. Protective Node-role restriction remains a candidate application,
not the leading design task or a prerequisite for the core. Neither a universal
electorate nor binding human authority is selected by this priority.

The intended deliverable is a dependency-ordered set of development and
verification tasks, with requirements, necessary ADRs and acceptance evidence.
It follows architecture readiness, not merely a locally precise component
brief. The earlier public-state verifier brief remains a conditional example,
not the selected first implementation task. This is the research focus and
handoff outcome, not a second live implementation ledger.

## Current contract

The [focused voting-core assessment](r-149-voting-core-design.md) now records
the no-cryptoasset/public-pseudonym choices, candidate composition, joint
sampling/turnout/workload evidence and unresolved dependency gates. Its
[conditional work-package map](r-149-voting-work-packages.md) prepares interfaces,
development and verification work without marking the core ready.

Read [scope](../../product/scope.md), the
[public authority target](../../product/operating-model.md#public-autonomy-target),
and [threat model](../../security/threat-model.md#public-autonomy-target) first.
Affected maintained owners are [Network State/Route/Node](../../technical/network-route-node.md),
[naming](../../technical/naming.md), [Release/Custody](../../technical/release-update-custody.md),
[enrollment](../../technical/enrollment-verification.md),
[Transit acquisition](../../technical/transit-grant-acquisition.md), and
[alpha control](../../technical/alpha-control-transition.md).
[Package map](../../development/package-map.md) and
[documentation policy](../../development/documentation.md) govern ownership and promotion.

The current alpha uses Target Links and project-controlled infrastructure; it
has no supported global Namespace close producer. Its root floors, finite
credentials, source/Route separation, typed outcomes, and authority-owned writes
are assets to preserve. Accepted ADR-0071/0072 select recipient-bound offline
C0 enrollment and a separate time witness; ADR-0073 is still proposed. None of
those alpha choices establishes an autonomous public bootstrap.

**Assumptions:** preserve human-facing canonical Names, the common complete
Candidate View, Go and existing cohesive owners, no native token, no mandatory
User payment, no User traffic accounting, and the current Route/privacy bounds.
An external settlement dependency, different naming promise, economic mechanism,
or changed privacy/finality promise requires its own decision. Owner-controlled
contribution and optional services do not automatically supply consensus
security, independent relays, or a public resource budget.

## Hypotheses

- **H1:** The existing verification/ownership boundaries can admit a researched
  successor without moving trust decisions into commands, sources, or Route.
- **H2:** A minimal shared-state contract can preserve canonical Names and the
  complete View without conferring discretionary network-wide power.
- **H3:** Founder-independent entry, state/time freshness, and software use can
  coexist with finite work and explicit unavailability under the actual budgets.
- **H4:** An isolated successor and explicit adoption are safer to qualify than
  allowing either old administrative signatures or new consensus proofs to
  authorize the same live state.
- **H0:** No complete successor fits every retained product, security, economic,
  and maintenance boundary; implementation must wait or the product must narrow.

## Evaluation criteria

The proposed contract is rejected if any appointed key remains necessary for
fresh admission, state progression, time confidence, or continued network work;
if a new majority can authorize an invalid owner transition; if an unverified
source/header is treated as complete current truth; or if retirement erases
security floors, replays credentials, exposes user activity, or restores old
administrative authority after an outage.

Use the existing [functional-map budgets](../../product/functional-map.md)
as required outcomes, not claimed measurements. Before an experiment, select
exact limits for full-state storage and reconstruction, endpoint bootstrap bytes
and time, steady-state update traffic, per-record verification/queue cost,
Name finality and proof lifetime, honest resource supply, and the adversary's
resource/partition envelope. A qualitative comparison cannot select those values.
Existing independent-operation and public privacy gates remain, or their claims
must be explicitly narrowed; local tests cannot manufacture independent people.

## Evidence plan

### Primary sources

Inspected on **2026-09-06**; local source baseline
`cc82869cc4d4a3221709a093b9b40590332e1abb`, with current owner documents as the
contract. File locations below identify inspection points, not new package plans.

- [R-146](r-146-non-administrative-network-consensus.md) supplies the governance
  and resource comparison and its primary sources.
- [State verification](../../../internal/network/state/epoch.go) constructs its
  acceptance policy from configured authorities, threshold, network, profile,
  time, and predecessor.
- [Release evaluation](../../../internal/release/evaluate.go) combines artifact
  identity, root rotation/floors, build/protocol policy, and emergency checks;
  [emergency verification](../../../internal/release/emergency_threshold.go)
  enforces the distinct threshold.
- [Entry verification](../../../internal/entry/verification.go) validates the
  signed recipient-bound Invite against current State, profile, and duty facts.
- [TUF specification 1.0.36](https://theupdateframework.github.io/specification/latest/),
  sections 1.4 and 5: authenticated update processing has explicit trust and
  expiration checks; it does not select a network-wide runtime permission policy.
- [Ethereum weak-subjectivity documentation](https://ethereum.org/developers/docs/consensus-mechanisms/pos/weak-subjectivity/)
  illustrates why fresh/long-offline verification needs an explicit bootstrap
  argument even when the running consensus is permissionless. This is not a
  recommendation to select Ethereum.

The [C0 milestone](https://github.com/dianabuilds/ardents-network/milestone/1)
is accessible. Its current work remains authoritative there; this record does
not copy issue state or schedule a competing implementation stream.

### Experiment

The Product Owner selected the joint state/resource/time/Name feasibility check
on 2026-09-06. The [bounded arithmetic experiment](../../../experiments/r-149-coupled-feasibility/README.md)
predeclared its inputs and rejection criteria before execution; its results
are summarized [below](#coupled-feasibility-result). This supplies calculations
and counterexamples, not a running consensus or product qualification.
The [finite operation/quorum experiment](../../../experiments/r-149-operation-invariants/README.md)
also evaluates task-first semantic counterexamples and five quorum configurations
with its own predeclared criteria. The
[temporary committee experiment](../../../experiments/r-149-task-committees/README.md)
adds predeclared selection, certificate, turnout and availability-filter checks
on 2026-09-07. Product conformance cases remain unexecuted.
Generated receipts stay outside the repository; only disposable calculators
and human-written evidence are retained. No C0 implementation or live networking
experiment is activated.

### Failure scenarios

Founder disappearance; all designated publishers/witnesses gone; new and
long-offline clients; frozen or eclipsed sources; common-owner Sybils; rented
consensus resources; false capacity; withheld state; conflicting Name claims;
reorganization and old-key replay; partition/rejoin; a malicious release feed;
critical software flaws; old/new profile confusion; interrupted migration and
credential reuse; insufficient honest resources or unavailable independent review.

## Findings

1. **Sourced fact — current owners/code:** removing only Epoch signature checks
   would leave Release, Entry/Source access, time, Compatibility, and current
   Namespace proof dependencies. **Inference:** migrate the whole dependency
   chain of a user outcome rather than counting removed keys.
2. **Sourced fact — current owners:** Service/Name Custody and scoped Transit
   signing already have distinct powers. **Inference:** preserve private owners
   and finite delegated credentials; deleting every signer would break ownership
   and resource control without establishing autonomy.
3. **Sourced fact — naming owner:** the current runtime does not produce public
   canonical Name closes. **Inference:** there is no supported public registry to
   convert automatically. Target-Link operation can qualify first; canonical
   naming needs a later end-to-end gate and must not inherit alpha corpus claims.
4. **Inference:** topology, source diversity, honest consensus weight, useful relay
   capacity, and operator independence are different security inputs. A consensus
   quorum proves none of the others by itself.
5. **Assumption:** one Product Owner and Codex can evaluate and implement bounded
   steps. **Inference:** a custom consensus/economics/upgrade stack is unacceptable
   as an unexamined default; independently maintained components and a smaller
   launch scope deserve preference when their assumptions fit.

## Options

| Transition shape | Product/security effect | Maintenance and rejection condition |
|---|---|---|
| Replace current alpha in place immediately | Couples unselected consensus to working C0 trust, freshness, Entry, and release paths. | Reject: no successor proof/format or recovery contract exists. |
| Accept old signatures OR new proofs for one live state | Easier apparent compatibility; the old authority can still authorize state. | Reject for the autonomy target; creates replay/downgrade ambiguity and an undeleted administrative bypass. |
| Build one bounded successor, observe it without authority, then explicitly adopt it | Keeps current floors/owners enforceable while testing exact new behavior; incompatible trust is explicit. | Preferred sequence. Additional reader cost must be bounded; shadow agreement alone is not security qualification. |
| Remove global agreement by narrowing naming/discovery | May reduce consensus and maintenance burden. | Conditional product alternative if the chosen contract cannot be met; not a silent migration technique. |

## Recommendation

Use the nine outcome-specific proposals below as the working solution direction:
retain owner-scoped authority and endpoint verification, private bounded routing,
minimal shared state, and voluntary local software adoption. Preserve the
isolated-successor migration boundary. Next close the coupled state-proof,
freshness, resource and Name-renewal feasibility questions before choosing a
production agreement mechanism or starting its verifier implementation.
Confidence is higher in reusing existing ownership boundaries than in the still
unproved autonomous public resource/proof profile. No candidate is qualified.
The [first operation/candidate assessment](#operation-level-findings-and-candidate-assessment)
now separates local authorization, safe fact accumulation and the remaining
common-choice obligations using explicit counterexamples. Non-PoW agreement
remains a candidate with unresolved admission, replacement and currentness
assumptions. The [older worksheet](#coupled-feasibility-result) still rejects
only its particular work/freshness/replay combinations; it does not select a
default mechanism or reject the autonomous product direction.

### Solution proposals by outcome

Status: **working recommendations, prepared on 2026-09-06** after the Product
Owner requested solutions for all nine tasks and a path to implementation.
These are evaluated design alternatives, not accepted technology selections,
executed experiments, a public support claim, or a parallel C0 task ledger.
Every recommendation inherits the hostile-environment and residual-assumption
contract. Current technical owners describe what exists; none becomes a public
implementation merely by being reusable here.

The recommended composition is owner-scoped authority, endpoint verification,
private bounded routing, minimal shared state, and explicit local software
adoption. State distributors carry evidence; agreement producers order inputs;
relays carry bytes. No participant gains all of those powers by running more
Nodes. The still-unqualified resource and proof boundaries are called out below.

#### Reach and publish the intended Service

**Preferred:** preserve the existing Service Target -> owner-authorized Instance
-> current signed reachability descriptor -> exact-Instance authenticated
Connection chain. Name input first needs its independently verified Binding;
explicit Target input retains its own provenance. Owner-scoped publication,
replacement and withdrawal use separate local Administration and Custody powers.

**Alternatives:** a mandatory central proxy introduces a required operator;
trusting a lookup server's answer alone loses independent destination checking.
Neither is a public successor. Reusable owner proofs do not make their carrier
or distributor honest, and a correct descriptor is not proof of a live Service.

**Implementation boundary and rejection evidence:** retain
[Endpoint/Service ownership](../../technical/endpoint-service-runtime.md) and
[private reachability](../../technical/private-reachability.md). Compose public
retrieval only after the state/duty successor is selected. Replayed generations,
wrong Target/Instance and descriptor substitution must not open a Connection;
withdrawal and crash/reopen must preserve finite work and authority floors.
Service compromise exposes that Service's plaintext and usable keys; separate
Authority custody limits powers only while that boundary survives.

#### Protect Application Data and authenticity

**Preferred:** preserve the separately authenticated end-to-end Service channel,
exact Destination/Profile binding, fresh session material and authenticated
continuity across replacement Attachments. Keep ordered offsets, replay refusal,
explicit directional closure and finite recovery inside Service Connection.
Use the reviewed maintained cryptographic/runtime closure; invent no primitives
or new cryptographic suite for the autonomy change.

**Alternatives:** adjacent-hop TLS alone is insufficient for protection from
carrier Nodes; handing a relay an endpoint continuity secret defeats this
boundary. A new transport is not a reason to replace the logical Connection.

**Implementation boundary and rejection evidence:** reuse the selected
[Connection contract](../../technical/endpoint-service-runtime.md). Exercise
injection, stale attachment replay, wrong Instance, mid-stream replacement and
closure against its actual consumer boundary. Confidentiality and authenticity
need effective endpoint execution, keys and cryptography. Forward Secrecy must
be qualified for that channel; it does not automatically cover lookup messages,
a compromised intended recipient, or ongoing post-compromise secrecy.

#### Limit disclosure of locations and relationships

**Preferred:** retain the Interactive Route's independent endpoint selections,
bounded persistent Entry Sets, role/data separation, per-Application Isolation
Contexts, private Name/Target lookups and Source-to-Route exclusions. Minimize
retained observations. Known family/subnet constraints are exclusion evidence,
not proof that unknown operators are independent.

**Alternatives:** a new entry on every retry increases exposure; a permanent
single proxy concentrates it. The later Product Owner direction in
[ADR-0077](../../adr/0077-evolve-one-common-protection-baseline.md) evaluates
protection changes as evolution of one common baseline, not a separately
selected delayed mixing/cover-traffic mode. Such a successor still needs its
own measured protection and complete-cost decision; current Interactive claims
remain unchanged. A generic Application adapter still cannot prevent an
Application's ordinary-network escape or self-identifying content.

**Implementation boundary and rejection evidence:** use
[Route/Node](../../technical/network-route-node.md),
[private reachability](../../technical/private-reachability.md) and
[Namespace/Resolution](../../technical/naming.md). Measure combined observer
views, induced entry churn and configuration/size fingerprints. The existing
single-role claim does not pass collusion or broad-observer cases. Lookup key
rotation and capture need their own analysis; OHTTP does not itself give
Forward Secrecy throughout a Gateway key configuration's lifetime.

#### Retain useful reachability under bounded interference

**Preferred:** recover the same authenticated Connection over a fresh permitted
Attachment, with finite deadlines, retry counts, backoff, buffer bounds and
unchanged destination. Retain the Entry Set while trying permitted recovery;
a middle-path failure is not permission to resample every entry. Candidate
state/descriptor sources may be replaceable within their authenticated role and
privacy constraints. The proposal is metadata retrieval, not replicated
Application content or a new storage service.

**Alternatives:** flooding an unbounded number of paths spends resources and
exposes more observers; a global reputation/ban vote turns ambiguous timeout
observations into administrative power. Prefer bounded local failure memory and
objective invalid-evidence rejection. A timeout does not identify the guilty
operator or authorize a public accusation.

**Implementation boundary and rejection evidence:** preserve Route/Entry,
Source, reachability and Connection ownership. Public alternate-source and
Bridge mechanisms require their own admitted profiles; current C0 has no
unselected URL, peer or carrier fallback. Compare partial blocking with total
isolation, and record success, terminal delay, induced contacts and total work.
A reachable honest path is necessary but not sufficient: the client must be
able to find it under the selected evidence and contact limits.

#### Establish current authentic operating context

**Preferred:** bind the installed profile to an explicitly chosen network and
rule identity; independently verify received owner transitions, admitted order,
state derivation, commitment, required data and freshness; commit durably before
exposing a current View. Preserve the separate provisional/pending/conflicting
outcomes and non-decreasing floors from proposed ADR-0075. Software distribution,
state distribution and the initial trust decision have different responsibilities.

**Alternatives and remaining choice:** the coupled mechanism table below rules
out appointing a permanent quorum or treating key count as voting power. A
work-based or non-work agreement remains a candidate only after its task-specific assumptions are justified. Neither a signed root nor
a chain of headers supplies full-state validity and currentness by itself.
Full replay is a useful verification reference, but unbounded replay on every
ordinary client conflicts with the public client contract.

**Implementation boundary and rejection evidence:** State owns verification and
commit; Source owns acquisition; Endpoint consumes the result. The first real
successor verifier must use one selected proof contract, not `valid=true` or an
unreviewed consensus-plugin verdict. Fresh, restarted and long-offline clients
need an explicit time/identity/proof assumption; if every available basis is
compromised, their currentness claim is unavailable. The bounded-reader proof
path remains a blocking selection, not an implemented light client.

#### Preserve canonical naming and owner transitions

**Preferred under the current product contract:** keep canonical exact Names,
separate Name Authority, owner-authorized predecessor transitions, existing
commit/reveal ordering, complete Namespace derivation, and explicitly authorized
recovery. Isolate these semantics from who transports or produces shared state.
Publish only the permitted opaque Name inputs to the Network ordering owner.

**Alternatives:** permanent ownership, owner-relative names, or a trusted dispute
panel change current product promises. The finite-lease candidate can only
claim renewal availability within a stated inclusion/finality envelope. Early
renewal and a finite margin can absorb bounded censorship; they do not solve
indefinite censorship. The specific options are compared below without silently
changing expiry/reclaim or claiming that owner signatures prevent every loss.

**Implementation boundary and rejection evidence:** Namespace owns lifecycle
and complete currentness; Custody signs its exact operations; Resolution carries
bounded proofs privately. Test competing claims, withheld renewal through Grace,
reclaim, stale parent lineage and recovered Authority. A local receipt is not
committed renewal, and a current View cannot infer voluntary abandonment from
absence of an admitted renewal. Actual public composition remains to be selected.

#### Keep ownership and participation voluntary

**Preferred:** retain separate Connection, local Administration and Custody
powers; let each owner limit or withdraw its own resources and adopt software
explicitly. Shared agreement has no arbitrary amendment, Name-seizure, build-ban
or install command. Prepare public software policy under
[proposed ADR-0076](../../adr/0076-separate-update-freshness-from-public-operation.md):
expired/unavailable update evidence blocks the update transaction, while losing
that publisher alone does not revoke a previously authorized installed program.
All other current State, credential, protocol and finite-work checks still apply.

**Alternatives:** one rescue key or a universal publisher renewal recreates an
indispensable administrator. An on-chain vote to install or forbid arbitrary
builds transfers that same power to producers. Optional owner-selected local
advisory policy may stop that owner's process; it cannot issue network-wide law.

**Implementation boundary and rejection evidence:** Release authorizes bytes,
replacement owns stopped activation, and owners select policy. Verify publisher
loss, replayed metadata, advisory handling, interrupted activation and attempted
floor rollback. The public proposal does not relax current C0 Release Safety.
An authenticated package can still contain a vulnerability; provenance is not
proof of harmless code, and unknown flaws have no guaranteed automatic remedy.

#### Bound contribution, abuse and concentrated influence

**Preferred:** separate local resource protection from shared influence. Charge
all child scopes to their owning finite budget; bound parsing, queues, handshakes,
concurrency and retry before expensive work. Retain purpose-scoped one-use
credentials or locally justified admission cost where the owner contract selects
them. Such cost is a local amplification guard, not a global Sybil proof.

For shared influence, compare evidence of a scarce allocation that cannot be
cheaply counted repeatedly across identities. Key count, claimed bandwidth,
uptime, per-key caps and fabricated useful-traffic receipts are insufficient.
If capacity observations rank routes, they must not also silently grant voting
weight or certify independent operators. A test of claimed capacity is not a
promise that the node will forward the next request.

**Alternatives and implementation boundary:** mandatory identity registration
adds a gatekeeper; a native stake token changes the economic contract. Native
work is only the resource reference discussed below. Resource/Node/Broker keep
local enforcement; State and the selected producer protocol would own their
actual influence evidence. Test identity splitting, allocation reuse, false
measurements, saturated queues and dominant real resources independently.
No reward, backup service, payment, co-resident profile or honest volunteer
population is selected by this proposal.

#### Remain usable and maintainable

**Preferred:** evolve the existing root Go module and deep owners one bounded
vertical slice at a time. Keep the ordinary client independent of production
work; charge required helpers to the same complete role budgets. Reuse current
cryptographic, transport and update components only within their reviewed
profiles. Retain every applicable numerical floor and privacy gate in the
functional map; this record creates no easier replacement budget.

**Alternatives:** building a new ledger, token economy, transport, mixing system
and app runtime together is not a feasible default for this team. Public naming
and stronger Application isolation remain distinct qualification gates. Current
C0's manual short-lived Credential and restart restrictions are real constraints,
not silently selected permanent public operations. Any future Service-only
renewal delegation needs its own owner-scoped authority and finite horizon.

**Implementation boundary and rejection evidence:** the package map remains a
factual inventory; no package, generic interface, dependency, or platform support
is created here. Measure whole-role startup, steady state, failure recovery,
cleanup and retained storage at the selected scale. Reuse existing passing
behavior evidence instead of creating implementation tickets to reproduce it.
Independent operators and external review remain evidence gates, not personnel
assumed available to perform the plan.

#### Coupled shared-state and bootstrap alternatives

These illustrative families are not an exhaustive search or a preference order. They serve outcomes 5, 6 and 8 together. The public authority target
and retained client/Name/privacy budgets apply to every candidate.

| Candidate family | Useful property | Unresolved cost or incompatible premise | Recommendation |
|---|---|---|---|
| Authenticated replication without shared voting | May distribute independently verifiable facts without giving additional keys decision power. | Must prove that concurrent changes preserve every required invariant; eventual convergence alone supplies neither exclusive final Name allocation nor a complete current View. | Evaluate for suitable fact-distribution tasks; not an automatic replacement for the shared-state contract. |
| Agreement without computational work over the necessary conflict domain | May address exclusive outcomes without a mining chain. | A concrete protocol still needs justified participation/membership, quorum or influence assumptions, censorship/fault bounds, proof costs and founder-independent bootstrap. No such profile is assumed available. | Compare against the task contract before choosing any work or non-work mechanism. |
| Appointed signed/BFT quorum | Explicit membership and quorum-based state agreement. | The membership authority and emergency discretion remain appointed dependencies; replicating them does not satisfy ADR-0074. | Retain current C0 only; reject as the autonomous public end state. |
| One identity, one vote; age or self-reported capacity as weight | Easy participation and simple tally. | Identity splitting and measurement/reputation collusion cheaply manufacture influence. | Reject as a security basis. |
| Native resource-backed open agreement, using work as a reference | Can relate production influence to expended work without appointing producers. | Honest work funding, concentration/rental, difficulty/time behavior, reorganization, data availability, full validity and bounded bootstrap all need a coherent profile. | Conditional comparison reference under the no-token/no-payment constraints, not a selected production algorithm or a recommendation to build one now. |
| Existing external settlement | May use an existing security/maintenance ecosystem for ordering commitments. | External dependence, paid submission/funding, bootstrap trust, metadata and off-chain validity/availability remain; no free inherited security for a hash-only payload. | Consider only after an explicit dependency/economic decision; not selected. |
| Native stake-based membership | Makes consensus influence scarce through a stake asset. | Introduces the unselected economic asset/distribution system; long-offline bootstrap may need an external recent trust anchor, depending on the protocol. | Not the default under current constraints. |
| Owner-relative names and local discovery instead of a common registry | Removes some need for one global order. | Changes canonical Name and complete Candidate View requirements; does not itself prevent malicious relay selection. | A product fallback to discuss if no retained-contract candidate fits, not an implicit implementation shortcut. |

**Reader decision:** begin verification research with a bounded full-verifier
reference over a declared finite corpus. A public bounded reader additionally
needs either locally checked complete evidence within its budget or a qualified
compact validity proof plus the necessary availability/currentness argument.
A sampled leaf, trusted snapshot server, majority-approved root, or checkpoint
chosen merely by counting peers cannot supply the missing proof. No compact
proof construction, prover cost, or maintained implementation is assumed ready.

**Freshness decision:** prefer explicit time confidence with bounded monotonic
continuity and a declared initial/restart time basis over allowing producer
claims to define their own freshness. This is a candidate condition, not a claim
that local clocks or hardware are honest. Replacing real-time leases by block
heights would change their semantics and couples expiry to adversarial production;
it needs a separate decision. If no current-time basis is justified, no fresh
work is authorized. Multiple mutually consistent clocks do not establish that
basis merely by being numerous.

**Selection gate:** a candidate must jointly fill scale, actual proof cost,
initial/recovery bytes, normal and hostile capacity, finality/reorganization,
time continuity, data availability, state retention and ongoing maintenance.
A failure in any required field rejects the complete candidate. We have not yet
established such a public profile; choosing a consensus name would hide this gap.

#### Name renewal under censorship

The adversary can withhold a valid signed renewal before the authoritative
admission boundary. A complete log of admitted inputs cannot prove that no
renewal existed outside that boundary. This is a distinction between the two
inputs, not a demonstrated exploit of the current maintained runtime.

| Product option | Effect under withheld renewal | Trade-off and disposition |
|---|---|---|
| Retain finite Lease/Grace with a declared inclusion bound | Renewal committed sufficiently before expiry keeps the Name; outside the supported censorship envelope expiry/reclaim can still occur. | Preferred comparison baseline because it preserves the accepted lifecycle. No unconditional retention promise; the bound and whole submission/finality path must be justified. |
| Stop or extend all expiries whenever censorship is alleged | Can defer some loss but lets false/unverifiable allegations keep names indefinitely or halt the namespace. | Reject allegation-driven global changes. A locally uncertain verifier can withhold readiness; it cannot impose a different shared lease lifecycle. |
| Never reclaim without an owner-signed release | Indefinite withholding alone cannot release the Name. | Abandoned and lost-key Names can remain occupied indefinitely; this replaces the accepted finite-reuse product contract. Explicit alternative, not selected. |
| Replace canonical Names with owner-relative addresses | Ownership no longer depends on competing global human-name claims in the same way. | Changes the canonical-name experience and does not supply route availability. Explicit product alternative only. |

For the retained-contract baseline, select the renewal lead time and supported
inclusion/finality envelope together; receipt, admission and commitment are
separate observations. Submit through only the qualified private paths and
track committed renewal locally. Do not silently promise automatic owner signing
or a public submission tracker that leaks Name/User relationships. A longer
margin changes the attack duration, not the indefinite-censorship limit.

#### Validation designs and implementation sequence

The existing dependency table remains the sole transition sequence in this
record. The rows below specify concrete evidence at its boundaries, not live
tickets or permission to start another C0 slice. The arithmetic and finite operation/quorum worksheets have run; public conformance cases remain unexecuted.
Before any execution, freeze the exact question, candidate, corpus/topology,
reference role, finite limits, adversary schedule and evidence paths. Unknown
values are not filled by a presumed honest deployment.

| Decision/implementation boundary | Concrete next artifact or slice | Falsification / exit evidence |
|---|---|---|
| Shared-state feasibility, before production code | A task/invariant matrix with competing complete solutions, including non-PoW candidates: identify necessary agreement, actual verification/currentness evidence, admission/influence assumptions and finite costs before selecting a protocol. | Reject if any required property rests on an unverified root, recurring appointed checkpoint, unfunded honest resource assumption, missing maintained component, or budget overrun. Record infeasibility rather than scaffold a ledger. |
| Route resilience and privacy, before public fallback changes | A finite adversarial model of the selected persistent Entry/role constraints, compared with independent resampling only as a reference. Include withheld sources, false capacity and induced path failure. | Rejection if recovery breaks an exclusion/identity binding, silently exceeds its exposure/work bound, or reports success without a usable authenticated path. Model results are not real operator-independence evidence. |
| Lease-censorship semantics, before a Name runtime successor | Trace one valid timely renewal withheld before admission, ordinary owner inactivity, conflicting admissions and reclaim under the selected bound. | Reject any claim that a receipt ensures renewal, omission proves owner intent, or a local timer resolves a global conflict. If the desired guarantee exceeds finite reuse, return the product trade-off explicitly. |
| First successor verification slice, after real proof selection | The already prepared offline State verifier: authentic input, deterministic infrastructure projection, durable commit, restart and refusal. Use the actual proof and grammar in an isolated profile/root. | C01-C19 at their declared boundary, with no unchecked verdict or fixture-only trust. Required repository gates accompany actual code. |
| First connected successor journey | Add qualified acquisition/time/admission and one bounded Endpoint reader; exercise fresh/restarted Target publication, lookup, connection, replacement and drain. | Remove founder-dependent services, with declared surviving resources; preserve correct destination, currentness, privacy boundaries and finite work. Metadata failover never becomes public content replication or arbitrary peer fallback. |
| Software-policy and Namespace composition | Apply an accepted successor to Release policy and separately compose the selected Namespace ordering/validity/renewal path. | C20-C26 plus the selected lease-censorship and key-compromise cases; expired update metadata cannot install bytes, publisher loss alone does not revoke installed operation, and Name floors survive. |
| Public promotion and cutover | Qualify exact artifacts/profiles and every claimed adversary case, then use explicit adoption with separate old/new roots and authority rejection. | Resource/metadata/privacy observations fit their selected contracts; unresolved claims remain withheld. Local multi-process tests cannot substitute for independent deployment or independent review. |

#### Additional primary evidence for these proposals

Sources accessed **2026-09-06**; these are source facts followed by project
inferences, not measurements or new component selections.

- [Tor guard specification](https://spec.torproject.org/guard-spec/) treats
  persistent guards and resistance to forced replacement as exposure controls.
  **Inference:** recovery success and entry exposure must be assessed together;
  more retries are not automatically safer.
- [RFC 9458, sections 6.6 and 6.7](https://www.rfc-editor.org/rfc/rfc9458.html#section-6.6)
  does not provide Forward Secrecy throughout a key configuration's lifetime
  and describes Gateway-key compromise consequences. **Inference:** retaining
  OHTTP does not inherit Service Connection secrecy or eliminate collusion;
  key lifetime, combined observations and context separation need separate evidence.
- [Bitcoin paper, sections 4, 6 and 8](https://bitcoin.org/bitcoin.pdf) couples
  work-based agreement to honest computational resources and incentives, and
  distinguishes simplified verification from full validation under an overpowering
  attacker. **Inference:** work cost and inclusion are not enough to establish
  Ardents' complete-state, no-invalid-owner-action and bounded-client contract.
- [Ethereum weak-subjectivity documentation](https://ethereum.org/developers/docs/consensus-mechanisms/pos/weak-subjectivity/)
  describes a recent checkpoint obtained from a trusted source for bootstrap.
  **Inference:** a stake-based label cannot remove our initial/long-offline
  trust question. This source is specific to Ethereum, not a theorem about every
  possible stake protocol.
- [TUF specification, client workflow](https://theupdateframework.github.io/specification/latest/#detailed-client-workflow)
  requires initial trusted root provenance, checks metadata freshness and aborts
  an update on failed checks. **Inference:** this supports retaining safe update
  verification while separately deciding whether an installed program may keep
  participating. TUF does not select Ardents' runtime vulnerability policy.

### Operation-level findings and candidate assessment

**Status: first bounded task-first assessment completed on 2026-09-06.**
This compares concrete operation semantics and candidate roles, with
[reproducible finite counterexamples](../../../experiments/r-149-operation-invariants/README.md).
It does not select a public algorithm or complete every mechanism-specific
proof. The older native-work worksheet remains one separate example.

#### Which operations actually require a common choice

**Sourced facts — current owners, accessed 2026-09-06:** publication already
uses owner-scoped credentials, non-overlapping validity intervals, exact Target
and Instance verification, per-Target durable floors, and conflict refusal.
The [private reachability owner](../../technical/private-reachability.md#descriptor-authority-and-currentness)
specifies those checks. Node duties and Route still require current State;
preserving local verification does not authorize bypassing that dependency.
The [Namespace owner](../../technical/naming.md) separately requires an
authenticated admitted order and complete close before a root claim becomes
current. Its author signature and pending submission are not that close.

**Inference and model evidence:**

| Operation | Local check or evidence merge that is sufficient for a narrower obligation | Remaining shared/dependent obligation and hostile result |
|---|---|---|
| Local Grant, local resource ceiling, stream admission and owner-approved install | The local owner serializes its own decisions and enforces scope, replay and ancestor budgets. The eight-slot model stays at eight under 100 key labels. | No global per-person fairness follows. A hostile local execution boundary can ignore its policy; no network-wide power is introduced. |
| Authenticate a Service publication and exact destination | Validate the owner/Instance chain and finite non-overlapping authorization. Independent caches may carry identical signed evidence. | Current State, trusted time, role/privacy constraints and live Instance verification remain. A stale descriptor may fail; caching alone does not prove availability. |
| Accumulate immutable Node/owner evidence | Exact duplicate suppression and authenticated set accumulation are order independent for the tested fact set. Additional keys do not authorize changes to another owner's object. | A valid announcement is not eligibility, operator independence or current complete View. Hostile valid floods still require bounded admission/retention. |
| Apply changes to two unrelated objects | The two frozen binding changes commute under unchanged authority/lineage and no shared quota. | Different Names are not automatically independent: shared parent lifecycle, admission capacity and state-cutoff dependencies must be checked. |
| Select two competing successors of one object | Preserve both as evidence; validate each against its named predecessor. | Each passes against revision 7, but first-arrival application selects different successors in the two orders. Current exclusive effect needs an agreed conflict rule/evidence, not just author signatures. |
| Select a root Name claim | Validate pending claims and their opening/author evidence within Namespace. | The received ordinal-9 claim is displaced by a withheld eligible ordinal-4 claim. A local minimum, source vote or sender timestamp does not prove the accepted complete input or authorize early finalization. |
| Renew, recover, transfer, release or reclaim a Name | Check exact author/recovery policy, generation, predecessor and deadline evidence locally. | Renewal versus recovery/reclaim and ancestor changes interact. Holding a renewal outside admission cannot extend the Lease. New parent generation 4 does not reactivate a child of generation 3. |
| Derive a current Candidate View | Verify admitted infrastructure inputs and deterministic dispositions against one proved boundary. | Completeness is over the agreed admitted domain, not every message ever broadcast. Neither a set of reachable peers nor an ever-growing fact set supplies that boundary or common eligibility. |
| Read current authority on first start or restart | Check network/rules identity, source evidence, compatible floors and any justified time interval. | Identical transcripts can represent timely delivery or a six-hour stale world to an isolated fresh reader. Currentness must be justified separately from signatures and append-only extension. |
| Select/recover a Route and protect Application traffic | Keep Endpoint-owned selection, exact authenticated Connection, finite exposure/recovery, local resource controls and qualified privacy boundaries. | No vote on each connection or payload is needed. Agreement/replication does not establish anonymity, independent operators or a surviving usable path. |

The experiments enumerate **24 positional fact-delivery permutations**, both
orders of independent and conflicting changes, and **1,133 nonempty subsets**
across five quorum configurations and their fault reductions/checks. These are
semantic and combinatorial calculations, not real network runs. Symbolic
authorization is an assumption; no signatures, proofs or production acceptance
boundary were implemented.

#### Candidate assessment by responsibility

The columns compare roles in a composed solution. A partial fit is not a claim
that one candidate implements all nine outcomes.

| Candidate | Appropriate task and evidence | Failure or dependency found | Assessment for Ardents |
|---|---|---|---|
| Owner-scoped signatures, capabilities and finite validity | Existing local authority and exact Service authentication; preserve current owner checks and durable floors. | Compromised owner/host, stale evidence, conflicting owner successors and unavailable paths are separate cases. | Retain as the authorization foundation. No shared voting or PoW requirement follows for that check. |
| Authenticated fact replication with invariant-preserving merge | Exchange independently checked immutable facts; the finite duplicate/disjoint-change cases converge. BEC supplies a research basis for the appropriate transaction class. | Uniqueness, irreversible exclusive successor choice and latest-state claims do not follow from set convergence. Unbounded history/retries conflict with finite budgets. | Preferred direction for evidence distribution where the full operation/retention proof fits; no library, wire or CRDT is selected. |
| Per-owner append-only logs with mirrors/auditors | Preserve signed predecessor evidence, compare observed histories and expose some equivocation. | Two branches can each extend the same prefix. A censor may withhold the branch or input needed to expose it; observation after publication is not prevention of conflicting final authority. | Useful evidence component, insufficient as the sole current Name/View decision mechanism. |
| Fixed-membership threshold/quorum agreement | The 3-of-4 example has no universally required member and survives each single-fault intersection/removal check. | The roster and its replacement are inputs; four keys do not prove four independent operators or permissionless influence. A production ballot/locking protocol is not supplied by a threshold certificate. | A bounded structural reference, not the autonomous public membership solution. Do not turn it into a permanent appointed roster. |
| Federated agreement with participant-selected quorum slices, using SCP as a concrete reference | Addresses common choices without computational work or requiring a token as the agreement primitive; configurations can distribute dependency. | Split local choices, shared faulty intersections, indispensable cores and configuration turnover break the examples. Safe fresh enrollment, replacement and honest quorum assumptions still need justification. | Retain for the narrow common-decision comparison; no SCP adoption, trusted list or membership policy selected. |
| Native resource-backed agreement | A distinct way to constrain production influence; the older work worksheet remains its explicit conditional example. | Its measured-by-model delays, honest work supply and proof/history costs are specific to that candidate, not universal requirements. | Keep as a comparator, not the default or an obligatory component of another approach. |
| Existing external settlement for necessary commitments | Could delegate a bounded ordering obligation to an existing system. | Outside fees/funding, trust, censorship, privacy, availability and Namespace validation do not disappear with an anchored digest. No dependency/economic decision exists. | Conditional dependency alternative; not an adopted production path. |
| Owner-relative names or local discovery replacing shared uniqueness/View | Reduces the obligation to give everyone one exclusive answer. | Changes accepted canonical Name and complete Candidate View requirements. It does not itself solve hostile routing. | Explicit product alternative only if later selected; not a shortcut inside the maintained contract. |

**Primary evidence:** [BEC](https://arxiv.org/pdf/2012.00472), sections 2-3,
supports invariant-preserving convergence under its correct-replica
communication and cryptographic assumptions. [RFC 9162 section 11.3](https://www.rfc-editor.org/rfc/rfc9162.html#section-11.3)
describes log withholding and inconsistent views.
The [SCP paper](https://stellar.org/papers/stellar-consensus-protocol.pdf)
supplies the slice/intersection/availability and configuration-continuity
conditions used in the structural model. These sources were accessed on
2026-09-06. Their scopes do not provide Ardents bounded currentness,
resource-exhaustion or privacy guarantees.

**Calculation — non-work quorum comparison:** original 3-of-4 has five quorums,
correct intersection after each of four single-fault deletions, and no member
common to every quorum. Original 3-of-5 has sixteen quorums but fails all five
single-fault deletion cases: abc and cde can overlap only in equivocating c.
Separately safe 3-of-4 configurations abcd and aefg have disjoint bcd/efg quorums
when combined for an undecided slot. An anchor-required configuration has one
indispensable member despite its additional nodes. These are concrete reasons
to test membership/replacement separately; they are not SCP protocol executions.

**Sourced fact — implementation fit:** the official
[Stellar Core repository](https://github.com/stellar/stellar-core) describes
C++20 and Apache-2.0 licensing. **Inference:** it is not an assumed drop-in Go
component; a port, side process or extracted engine creates separate dependency,
maintenance and whole-role cost questions. No such integration is authorized
by this assessment. No independent staff or public operator pool is assumed.

#### Proposed composition and next decision

**Recommendation, with higher confidence in the separation than in any
unselected agreement mechanism:** use the existing local owner/verification
boundaries; exchange bounded authenticated evidence without giving each copy
a vote; and reserve shared decision evidence for the remaining exclusive
outcomes and common admitted-state boundary. These are responsibilities, not
new package names, a new layer hierarchy or permission to drop any C0 check.

The common part must still prove input admission/cutoff, permitted order and
projection, current Name generation/lineage, sufficient data, freshness,
commitment and safety-floor continuity. A smaller payload does not prove that
this is cheap or decentralised. The Network side continues to order only
permitted opaque Name inputs; separate Namespace validity cannot be replaced
by a producer's unchecked verdict or by copying plaintext Name state into every
Network participant.

The decisive next comparison is **open admission and replacement for the
common-decision responsibility**, assessed alongside evidence/currentness and
cost. A quorum candidate must show where the first usable configuration comes
from, how adding many identities fails to gain unsafe influence, how operators
leave without a universally indispensable member, and how old/new acceptance
remains consistent. A resource candidate must supply its distinct scarcity and
honest-supply argument; an external candidate must expose its dependency and
funding terms. The same Name-censorship, fresh-client, total-isolation and
compromised-endpoint cases apply to all.

Before implementation, each candidate must fill actual proof/metadata sizes,
start/restart acquisition and checking cost, retained history/garbage collection,
whole-role CPU/RSS, idle carrier budget, admissible workload and hostile progress
bounds. Existing 15-second clean-start/5-second restart and other role budgets
are evaluated in their original qualification conditions; none is replaced by
the old worksheet's block timing. No latency number, compact-proof construction
or admission success is inferred from the finite structural checks.

The strongest argument against this composition is that common freshness and
exclusive Name lifecycle may couple more work than the simple independent
examples suggest. Keep that as a falsification target. This assessment
completes the first operation/candidate comparison and narrows the unresolved
question; it does not claim that public agreement, currentness or availability
has been solved.

### Task-first mechanism comparison

**Product Owner correction, 2026-09-06:** a full PoW system is not required, and
PoW is not required at all. Design starts with the product task; mechanisms are
compared only after its correctness, progress, privacy and cost requirements
are explicit. The native-work worksheet is one illustrative calculation, not
the default architecture, an exhaustive search, or evidence that every
autonomous design has its confirmation delay or replay cost.

**Inference:** separate four questions that the previous next-step wording
coupled too early: whether an action is authorized; which conflicting action
has effect; how a participant obtains sufficiently current evidence; and how
much hostile work a receiving role must tolerate. A mechanism may address one
without answering the others. Local admission work, if justified, does not
imply block production, global voting power, or a mining economy.

| Concrete task / invariant | Smallest obligation to investigate | Alternatives to compare / rejection boundary |
|---|---|---|
| Verify a Service or Name owner's permitted action | Check exact authorization, scope, predecessor and replay conditions locally. | Owner-signed evidence and scoped capabilities. A vote cannot authorize an invalid signature or stale successor; signature validity alone does not settle competing successors or prove currentness. |
| Bound one host's memory, CPU, queues and forwarding | Enforce that owner's finite limits and useful-work policy. | Quotas, backpressure, finite attempts and scoped admission; compare a local cost mechanism only if needed. Its usefulness must be measured, and it grants no shared-state influence. |
| Distribute already authenticated facts | Obtain missing data and reject invalid or duplicate facts; determine which merges preserve the required invariants. | Bounded authenticated replication/reconciliation, including non-PoW convergence where justified. Eventual delivery cannot be relabeled complete current State, current authority or bounded availability. |
| Select routes and retain usable alternatives | Enforce Endpoint selection, exclusions, finite exposure and recovery with available permitted candidates. | Local selection plus independently justified evidence. A common eligible Candidate View remains required; a local peer sample cannot silently replace it or establish operator independence. |
| Resolve simultaneous claims or conflicting owner successors | Establish one admissible result under the accepted Name order, generation and no-silent-rollback rules. | Compare agreement for the necessary conflicting inputs, including non-PoW candidates; separately establish membership/influence, progress and bounded verification. A hash tie-break after eventual delivery does not by itself prove finality or absence of an earlier withheld claim. |
| Renew, expire, recover and reclaim Names | Preserve exclusive renewal and parent lineage with an explicit inclusion and deadline contract. | Compare admission/delivery, commitment and time evidence independently. Any changed lease/reclaim meaning requires its own product decision; receipt is never automatically renewal. |
| Give a fresh/restarted client an authentic current View | Justify network/rules identity, validity, completeness over the admitted domain, freshness, availability and durable floors within the client budget. | Compare direct bounded verification, authenticated state plus adequate proofs, and qualified checkpoints with their actual trust basis. No assumed chain history or compact-proof construction; distributors cannot assert unchecked verdicts. |
| Adopt compatible rules or software | Retain voluntary owner choice and exact verification of the selected artifact/rules. | Local adoption and separate provenance/safety policy. Do not add a global amendment or publisher permission-to-run operation to solve deployment. |

The table describes obligations, not eight new modules or independent public
protocols. Their composition must retain the current data/authority boundaries.
Canonical Names and the complete common Candidate View remain accepted
requirements under ADR-0074. Whether their implementation can use a narrower
ordering domain is research; changing accepted Epoch/admission/ordering
semantics requires an explicit successor decision. A local-discovery-only or
owner-relative-name design is a named product alternative, not an unnoticed
substitution.

**Primary evidence, accessed 2026-09-06:**
[Kleppmann and Howard, Byzantine Eventual Consistency](https://arxiv.org/pdf/2012.00472),
sections 2-3, characterizes a class of invariant-preserving replicated
transactions that can converge despite arbitrarily many Byzantine replicas.
Its model assumes usable communication among correct replicas and effective
cryptography; this is not bounded delivery, currentness or protection from
resource exhaustion. Independent duplicate allocations violate uniqueness in
the paper's example. **Inference for Ardents:** evaluate authenticated fact
replication separately from exclusive canonical Name allocation. Neither
ordinary gossip nor a CRDT label establishes that the complete Ardents contract
belongs to that class. No paper prototype or library is selected.

For each row, write the candidate-independent acceptance case first:
the legal action; the conflicting/hostile action; the invariant that must hold;
the minimum surviving conditions; the observable success/refusal; and its
existing role budget. Then:

1. Test whether local authorization and deterministic validation suffice for
   that obligation, without assigning any peer influence.
2. Where replicas must exchange facts, test whether concurrent valid changes
   can merge without breaking *all* relevant invariants, including revocation,
   parent lineage and current usability.
3. Where that fails, identify the exact conflict domain needing agreement and
   compare both work-based and non-work candidates. Their admission, membership,
   fault and progress assumptions must be supplied, not inferred from key count.
4. Compare complete task solutions under the same adversarial schedules and
   product budgets. Each mechanism supplies its own confirmation/delivery/proof
   parameters; the 80-confirmation and 199-minute worksheet values do not carry
   over to a different candidate.
5. Select a mechanism only after its relevant evidence and the combined journey
   fit; then prepare the implementation slice and any required successor ADR.

**Disposition of this framing:** the [operation-level assessment](#operation-level-findings-and-candidate-assessment)
now supplies the first counterexamples and responsibility-specific candidate
results. Admission/replacement for a common decision, actual currentness evidence
and whole-role cost remain open. Neither native PoW nor a non-PoW replacement
is selected in advance. The older worksheet establishes no global impossibility
result for administrator-free operation.

### Temporary task committees and willingness

**Status: conditional candidate assessed on 2026-09-07.** The Product Owner
proposed random temporary voters for one task, a provisional 24-hour window and
excluding nonresponses; subsequent refinements add immutable proposals, future
randomness, absolute thresholds, separate outcome certificates, willingness,
readiness selection and ordered alternates. The
[predeclared analytical experiment](../../../experiments/r-149-task-committees/README.md#results-and-disposition)
records exact probabilities, counterexamples and executable reproduction.
No identity, weight, human-governance power, protocol or duration is selected.

**Current contract:** ADR-0074 permits agreement over valid common facts but no
general rule-changing, forced-update or Name-seizure authority. A temporary
committee does not inherently remove continuing capture: the same coalition can
win many tasks. It can distribute work and restrict each statement to one task.
Canonical Names/View, local owner checks and separate opaque Name admission
remain binding. Proposed constitutional/emergency powers in supplied discussion
are product alternatives, not implicit amendments to that boundary.

**Sourced facts, accessed 2026-09-07:** the
[Algorand paper](https://eprint.iacr.org/2017/454.pdf), introduction and sections
5.1-5.3, describes private verifiable committee sortition, weighted membership,
future-seed/key constraints and mitigation of targeted attacks. Its weighting
uses money and its actual protocol differs from fixed equal-label sampling;
neither its economic choice nor an Algorand integration is selected. The
[drand security model](https://docs.drand.love/docs/security-model/) explicitly
describes threshold compromise, selective availability and beacon withholding.
A beacon is therefore a candidate dependency with its own roots and failure
conditions, not a self-contained solution for admission or honest participants.

**Assumptions for calculations:** fixed N=100 eligible labels, exactly K hostile,
unbiased uniform samples without replacement within each committee, effective
verification and no additional post-selection corruption. These assumptions are
not established by Node activity, readiness acknowledgements or different keys.
The majority tail is hypergeometric. The original 15-seat example with K=20 has
P(at least eight hostile)=0.179744%, but P(at least five hostile)=14.691075%.
The latter exceeds the f=4 envelope used by the illustrative ten-signature
agreement model. Neither probability is a measured attack success rate.

**Finite findings and inferences:**

| Candidate rule | Finding | Consequence for the candidate |
|---|---|---|
| Exclude missing from the denominator | 2 YES/1 NO passes with 12 of 15 missing. Turnout ten also accepts 6 YES/4 NO with five missing. | Bind the acceptance requirement to the authenticated original committee, not local arrivals. |
| Turnout plus local deadline close | Readers see 7:3 and 2:8 out of the same 15 non-equivocating voters, each with turnout ten. | Local clock and observed absence are not proof of a common complete tally. |
| Separate monotone YES/NO thresholds | n=15,A=10,R=5 permits simultaneous approval/rejection with all voters honest; R=6 permits it with one equivocator. | For exclusive signatures with f possible double voters, require A+R > n+f. Conflicting approved values also need 2A > n+f and correct signing/locking across one conflict. These are structural conditions, not a full protocol. |
| Equal turnout/approval thresholds | Receiving A YES already demonstrates at least A participants. | A separate turnout rule only adds meaning if it imposes a different obligation, such as a larger acknowledged set; it can further reduce availability. |
| Fixed proposal and no reroll | 500 independently admitted draws yield 59.323871% chance of at least one hostile majority for the example pool. | Commit-before-randomness prevents adaptation of one text, not unlimited precommitted variants. Bound admission over conflict identity and shared resource budgets. |
| One proposal per identity / cooldown / text deduplication | Multiple identities bypass per-identity limits; distinct digests need not represent distinct conflicts. | A nonce ban and natural-language similarity are not a Sybil-resistant common admission rule. Typed object/predecessor conflict identity is possible for protocol operations; arbitrary semantic equivalence remains unselected. |
| Two disjoint committees | Exact probability of hostile majorities in both is 0.000007771%, unlike the independent square 0.000323079%. | Do not assume independence or import these majority figures as whole-protocol safety; specify second-stage state and certificate continuity. |
| Voluntary standing availability | All 20 hostile volunteering and only 16 of 80 honest volunteering makes the pool 55.555556% hostile. | Readiness and independence are different properties; qualify the fault fraction of the eligible subset itself. |
| Forty candidates, readiness, then final 15 | Eight hostile and eight honest responders leave seven or eight hostile in the final 15. | Readiness can improve turnout while changing fault distribution; second randomness does not undo the filter. |
| Ordered alternates before voting | Suppressing five honest confirmations promotes five fixed hostile reserves: three hostile primary seats become eight of 15, with no reroll. | Frozen ordering alone does not protect the activation predicate; readiness closure and censorship are part of selection security. |
| Reliability exclusions for missed responses | Missing messages do not identify sleep versus censorship/outage; more eligibility changes total influence even with equal per-vote weight. | Do not install a global reputation/exclusion authority or assume penalties cannot be used to expel honest members. |

**Calculation — honest nonparticipation:** under independent response probability
p and assuming all responding honest members agree, P(at least ten replies from
15 honest members) is 15.087891% at p=0.5 and 93.894857% at p=0.8. If four selected
faulty members withhold, ten replies from the eleven honest members have
probability 0.585938% and 32.212255% respectively. These are capacity bounds in
a declared simplified model, not estimates of human behavior. Common outages
and disagreement make independent participation an unsuitable default guarantee.
Population growth by itself does not alter fixed-size committee turnout at fixed
p; task load per eligible member is D*n/M under equal selection and remains an
unmeasured design input. No human panel or public operators are assumed available.

**Proposed candidate contract:** bind every vote/certificate to the network and
rules, authenticated pool and selection context, exact conflict/predecessor,
proposal and attempt, allowed outcome and compatible decision class. Fix the
admitted proposal/pool before an independently justified future seed; for a
second draw, fix and prove the availability set before its unpredictable seed.
No author may choose the favorable commitment, suppress an alternative admitted
proposal without its defined outcome, lower the class, replace current state,
or turn valid quorum signatures into permission to bypass owner authorization.
This is a logical checklist, not a new wire format or implemented protocol.

A result is authoritative only with its defined valid proof. A local deadline
may stop spending resources and report unresolved/unavailable. It does not prove
that no remote certificate exists, automatically finalize expired/rejected, or
make a conflicting new attempt safe. Early approval, rejection, true expiry,
reopening and committee replacement need compatible locking/finalization and
freshness rules. The provisional 24 hours is neither trusted time nor a safe
Name-renewal/reclaim policy; finite leases can still be lost through censorship.
More committees cannot repair absent inclusion or bootstrap evidence.

**Recommendation:** retain temporary scoped committees as one conditional
candidate, with higher confidence in the authority separation than in admission
and availability. Compare automatic agreement for mechanically checkable common
facts separately from optional human deliberation; operating a Node does not
commit its owner to manual decisions. Human suggestions cannot automatically
change rules under ADR-0074. For any human jury candidate, first evaluate a
bounded voluntary availability period and workload, then independently assess
whether per-task confirmation is worth the additional closure, latency and
selection-bias obligations. Do not combine readiness, reputation, reserves and
multiple juries as if their partial benefits establish a safe whole.

The strongest counterargument is that the readily available eligible subset may
be substantially more adversarial than the whole network, while quorum thresholds
needed for safety impede progress among busy honest volunteers. Failure of either
admission or turnout qualification rejects a safety/availability claim rather
than lowering the threshold after seeing replies. The finite calculator passes;
its results do not choose a human identity system, algorithm, committee size,
randomness provider, payment, implementation slice or public guarantee.

#### Subsystem boundary and residual-risk objective

**Product Owner direction, 2026-09-07:** investigate a relatively independent
system carried by Ardents, with explicit conditions and minimized residual
threats rather than a promise to eliminate every attack surface. This is design
research, not approval of a new maintained package, protected Application,
general governance power or automatic protocol adoption.

**Proposed responsibility:** produce independently checkable, task-scoped
agreement evidence. A consumer still verifies its own allowed transition,
owner authorization, exact predecessor, state/Name proof and currentness before
applying anything. Committee approval is not an arbitrary command executor.

| Boundary | Proposed obligation |
|---|---|
| Task input | Typed permitted operation, conflict identity, predecessor, evidence requirements and fixed decision class; no arbitrary authority from proposal text. |
| Admission and participants | Bounded task admission; justified pool membership and readiness without assuming key count or response history proves independent operators. |
| Selection and agreement | Fixed verifiable selection, declared fault assumptions, compatible outcome certificates and bounded unresolved behavior. |
| Result consumer | Verify scope, proof, owner rights, freshness and dependencies; reject invalid or conflicting effects independently of vote count. |
| Transport and local execution | Use explicitly qualified Ardents transport and resource boundaries; the agreement layer does not inherit anonymity, delivery or trustworthy endpoints merely by being separate. |

Architectural separation must expose bootstrap cycles: if forming the committee
requires current Network State while producing that State requires the same
committee, a fresh or isolated participant has no demonstrated entry path.
Likewise, using one coalition for pool admission, randomness, time and finality
can concentrate dependence despite distinct modules. These are design gates;
no out-of-band administrator or external beacon is silently inserted to close
them. Human deliberation remains separate from automatic necessary network facts.

For each proposed operating profile declare tolerated false/conflicting-result
risk over its task lifetime, progress/unresolved behavior under withholding,
privacy exposure and retained metadata, resource-exhaustion limits and recovery
conditions. Probability calculations are conditional on stated adversary,
eligibility and sampling assumptions. A risk target must count many admitted
attempts, not merely one draw. No numerical acceptable-risk target has been
selected. Reducing one risk can worsen availability or concentrate power;
additional conditions earn inclusion only with a demonstrated invariant or
measured benefit. Every claim names its surviving local execution, cryptographic,
communication and evidence assumptions; complete compromise cannot be hidden
inside a generic promise of safe refusal.

#### No disclosure power or secret-access path

**Product Owner requirement, 2026-09-07:** no task may request another owner's
private data or private keys, and the agreement subsystem must lack the means
to obtain or release them. The subsequent clarification makes any such access
path a blocking vulnerability, including a path outside normal voting.
This is now owned by [NET-20](../../product/functional-map.md#autonomous-public-requirements),
the [operating boundary](../../product/operating-model.md#no-disclosure-authority-or-secret-access)
and the [security claim conditions](../../security/threat-model.md#no-agreement-mediated-disclosure).
It is a required constraint, not a measured achievement or a new mechanism.

**Design consequences:** task producers, voters, evidence stores and result
consumers cannot gain private owner-data, decryption, private-key custody or
export capabilities through a task, even with every vote. A recovery hook,
arbitrary command/callback, diagnostic dump or shared privileged process is
not exempt merely because it is outside the task schema. Go module boundaries
alone are not evidence of effective process or data isolation; the actual
execution/custody arrangement still needs a researched decision. No new process,
sandbox, runtime, cryptographic primitive or API is selected in this clarification.

The qualification gate must trace direct and transitive capabilities and prove
that committee/result paths cannot obtain the protected assets, including through
an owner-authorized component. Owner-selected private recovery remains separate;
committee authority cannot substitute for that owner's authorization. Missing
such evidence withholds the claim; finding a reachable unauthorized access path
blocks the affected candidate and requires remediation. Existing finite committee
arithmetic does not exercise or qualify this boundary. No extra simulation or
implementation is performed for this requirement-only clarification.

### Candidate tasks for agreement and deliberation

**Status: proposed task catalog, 2026-09-07; not an accepted committee or
implementation scope.** The Product Owner asks which tasks/proposals should
enter voting. Start from the accepted product obligations, not the availability
of a jury mechanism. The existing research question remains R-149.

**Current-owner evidence:** [ADR-0074](../../adr/0074-target-non-administrative-public-operation.md)
and [NET-20](../../product/functional-map.md#autonomous-public-requirements)
exclude general administrative and disclosure powers. The
[semantic input contract](../../product/operating-model.md#minimum-semantic-input)
requires authorized operations and deterministic projection. The
[Name close owner](../../technical/naming.md#claim-and-epoch-boundary) requires
opaque admitted inputs, authenticated order and complete close, then derives the
claim winner by the retained rule. The
[State owner](../../technical/network-route-node.md#state-transition-admissibility)
rejects ambiguous/conflicting activation. These obligations do not authorize a
human majority to replace a validity rule with its preference.

#### Necessary common facts: automatic agreement candidates

| Task | What requires a common result | Allowed effect and limitation |
|---|---|---|
| Establish one bounded admitted input boundary and permitted order | Compatible participants need one proved input domain/order when individually valid competing inputs cannot all take effect. | Certify a candidate's agreed boundary/order under the selected protocol. Admission, inclusion and censorship constraints still need their own evidence; producers cannot select favorites by discretion or declare everything ever broadcast complete. |
| Commit one admissible successor of common state | Multiple proposed candidates may extend one predecessor; all consumers must preserve a compatible finalized history. | Certify the exact successor plus required validity/data evidence. Currentness and independent verification remain necessary. This can be combined with input ordering by one future protocol; it is not necessarily another vote. |
| Close a Name claim input round | The retained winner rule depends on the authenticated ordering and closed eligible input domain. | Supply only the permitted opaque input order/close. Namespace derives and verifies the winner; there is no human choice of who deserves a Name. This can share the common-state mechanism without sharing prohibited plaintext. |
| Establish the current Candidate View from committed infrastructure inputs | The same admitted domain and deterministic eligibility rules must produce the same complete view. | Verify the projection/current commitment. No popularity-based admission, proof of operator independence by votes, or discretionary ban follows. This is a derived obligation, not an automatically separate ballot. |
| Transition agreement participation or configuration | Only needed if the selected future algorithm has such configurations. | Prove the rule-defined transition and continuity of unresolved decisions. Conditional candidate only; no elected permanent roster, inherited admission monopoly or replacement protocol is selected. |

A Node declaration, signature, authorized Name update, expiry calculation or
other deterministic fact does not need an independent ballot about whether its
rule is true. Its incorporation/order may depend on shared state. Authorization,
mechanical validity, admission, commitment and current usability remain distinct.
Successful consensus cannot be inferred from checking the same signatures.

#### Optional human deliberation: recommendations only

No mandatory human-jury task has yet been accepted for the retained core
requirements. This is an open catalog, not a finding that no such task can be
needed: the protective-exclusion question below remains under evaluation. The
following are future optional product candidates, not an
implemented forum, public staffing assumption, requirement to respond or new
power over maintainers or owners:

| Proposal | Possible human outcome | What the outcome cannot do |
|---|---|---|
| Compare protocol improvements or alternatives | A documented recommendation, rationale and unresolved trade-offs. | Change running rules, force upgrades, certify compatibility/security by popularity or authorize an unknown future action. |
| Prioritize research and desirable product behavior | A preference signal to willing maintainers and contributors. | Bind the Product Owner's backlog, spend others' resources or require absent staff. The actual team remains one Product Owner and Codex. |
| Coordinate response to a reported flaw or deprecation | A recommendation to investigate or voluntarily adopt an independently verified fix, based only on permissible evidence. | Grant emergency shutdown/disclosure powers, publish private owner evidence, bypass artifact checks or turn a jury statement into vulnerability qualification. |

These proposals may not need a jury or binding distributed finality at all. A
non-authoritative discussion/preference mechanism can be sufficient. Do not make
normal network operation depend on volunteer human turnout to justify the jury
research. Introducing binding human control of global policy would be a separate
change to the accepted authority boundary, not a higher risk class in this catalog.

#### Protective restrictions and exclusion: open task candidate

**Product Owner clarification, 2026-09-07:** not finding an obligatory human task
yet does not close the catalog. Investigate whether excluding a harmful Node or
Application is necessary and whether any part requires voting. This authorizes
research, not a global ban mechanism or a selected human admission authority.

**Current-owner evidence:** the
[Node eligibility contract](../../product/operating-model.md#node-eligibility-and-sybil-boundary)
already distinguishes discoverability from deterministic eligibility, finite
assignment, quarantine and withdrawal. It limits local Route-failure effects to
that Endpoint and prohibits uploaded User reputation/route history. The
[local Application boundary](../../technical/endpoint-service-runtime.md#local-admission)
allows an owner to revoke that Application's own local Grant. The
[glossary](../../../CONTEXT.md#actors-and-software) distinguishes Application,
Service and Node; no common human/application identity or private-data access
is introduced by calling all of them participants.

| Protective task | Scope and admissible evidence | Role for common agreement or human judgment |
|---|---|---|
| Refuse an abusive connection or revoke a local Application Grant | One owner's resources/capability; bounded local observations and owner policy. | Immediate local protection requires no network vote. It supplies no global guilt finding or power over another owner's host. |
| Withhold or end eligibility for an unsafe Node duty | Exact Node key, role, assignment/generation, permitted reproducible evidence and a rule defining the effect. | A shared current disposition may need agreement; deterministic invalidity does not need human preference. Missing reachability can mean the duty is unusable without proving malicious intent. |
| Disqualify a key from a specified role after a proven protocol violation | Example candidate: two authenticated contradictory final statements for the same conflict/step when the selected protocol forbids signing both. | Validate the exact proof locally and agree its admitted current effect under a researched sanction/re-entry policy. Different rounds, phases, predecessors or permitted protocol votes must not be mistaken for equivocation. No proof format or punishment is selected here. |
| Temporarily restrict a Node after a substantiated attack lacking a compact decisive proof | Exact role and effect; only permissible evidence, false-positive/negative targets and a bounded decision procedure. | This is a candidate for substantive judgment, including a possible human committee. A binding global effect introduces admission power that needs an explicit policy/authority decision, not automatic approval from this discussion. |
| Restrict an Application on its owner's host or warn about a known harmful artifact | Owner-local software/capability identity and evidence accessible without protected data. | Owner choice or optional advice; no automatic installation, process deletion or global permission-to-run. |
| Deny network-wide access to an Application or Service | Must first identify which entity and behavior are actually being restricted and what compatible participants can enforce without inspecting confidential traffic. | Open product/authority/privacy question. Application code, a Service Target, a Name, a Node and a Person are not interchangeable. Relays have no demonstrated universal application-identification or content-inspection mechanism. |

**Recommended framing:** assess protective role restriction, not an undefined
ban from everything. Under a future qualified rule, compatible honest consumers
could stop accepting a particular key's role evidence or selecting it for new
work. They cannot thereby turn off its machine, erase its data, identify its
operator, prevent out-of-band communication or prove a fresh key belongs to the
same actor. Broader re-entry prevention is part of the unsolved admission/Sybil
argument and cannot be inferred from one revocation certificate.

A candidate restriction task must specify the exact offense/unsafe condition,
evidence and attribution boundary, affected key/role/generation, admitted case
identity, permitted scope and maximum cumulative restriction, outcome/appeal and
re-entry rules, compatible time/finality and stale-proof handling, resource bounds
and minimum remaining honest capacity. Repeated complaints, indefinite renewal
or relabeling the same case must not bypass those limits. Selective exclusion
can concentrate the remaining routing or agreement pool; restoring service and
protecting its security require a joint check.

Required counterexamples before selection: forged/framed complaints; a single
actor submitting many reports; legitimate different-round signatures presented
as equivocation; key compromise mistaken for proof of an operator's intent;
DoS/missing responses causing honest exclusion; coalition exclusion of its
competitors; state/time partitions and old exclusion replay; turning temporary
restrictions permanent by repetition; a new key/Target escaping the old scope;
and evidence collection that exposes protected user data. Any need to obtain
such private data rejects that design under NET-20. No private logs, packet
contents, location-to-Service relationships or owner secrets become permitted
merely because the task is labeled abuse prevention.

**Disposition:** local protection and deterministic eligibility are established
responsibilities. The need and permissible scope of a shared protective exclusion
procedure are now explicitly open, including whether a narrow human judgment
step has a justified benefit. No permanent global blacklist, opinion-based ban,
universal Application registry, human uniqueness proof or enforcement mechanism
is selected. This is an addition to the task catalog, not a claim that all binding
human tasks are impossible or that all exclusion is arbitrary administration.

#### Owner decisions and forbidden tasks

The following do not enter common discretionary voting:

- local resource grants, running/stopping one's own Node, local access policy
  and explicit software adoption remain with that owner;
- Service publication and key use follow the owner's bounded authorization;
  Name changes/recovery follow owner authorization and the shared lifecycle
  rules, not permission from unrelated voters;
- private data/key acquisition or disclosure, escrow access, arbitrary execution,
  Name seizure, forced installation and global shutdown are forbidden effects
  under the retained boundary. Arbitrary unbounded exclusion is not authorized;
  bounded protective restriction is the separate open question above;
- message truth, real-world identity, guilt, alleged independent operation or
  resource capacity cannot be made proven facts by a vote. Where a future rule
  uses evidence, its admissibility and limitations need independent validation;
- appeals cannot waive owner rights, resurrect old authority or select a preferred
  historical fork. New evidence/recovery must follow a qualified transition;
- routine/protocol/constitutional/emergency labels do not grant powers or lower
  required checks. Common parameters, automatic safety policy and protocol changes
  are not a generic parameter-editing task accepted by this catalog.

#### Recommendation and admission gate for a new task type

**Retained application example, not the current research priority:** specify
one automatic common-state successor task:
from an authenticated predecessor, establish one permitted finite input
boundary/order and its proved next projection. Inspect Name input privacy,
renewal inclusion, Candidate View completeness and fresh-reader behavior within
that task before deciding whether it is implemented by a random committee or a
different mechanism. This is a logical research example, not a commitment to one
chain, total order for every object, algorithm, human jury or implementation issue.

Each additional task type must name its product need, why local authorization or
deterministic verification is insufficient, exact object/conflict identity,
permitted outcomes, minimal non-private input, authority for any side effect,
safety/finality/inclusion evidence, no-decision behavior, resource/freshness
bounds and dependencies. The decision class follows the operation's contract;
a proposer cannot mark the same effect routine to obtain weaker checks.
If no permissible side effect needs a common authoritative result, keep the
proposal advisory or local instead of inventing consensus work.

**Confidence and falsification:** high confidence in preserving the accepted
owner/no-disclosure boundary; the first automatic task's feasibility remains
unproven. A requirement for substantive human judgment that cannot be replaced
by rules and whose result truly must bind the public network would reopen the
authority/product decision. Protective exclusion is a concrete open candidate
for that assessment; its necessity and safe scope have not been established. The
random committee research remains conditional evidence, not a mandate to ship voting.

### Participant selection and voting: core readiness

**Product Owner direction, 2026-09-07:** prioritize the participant-selection
and voting system itself. Later application scenarios use its declared contract.
Preparing a complete, coherent core design precedes maintained development in
parts: an unresolved dependency can invalidate an otherwise precise component.
The eventual deliverable includes ordered development and verification tasks.
This does not select a production protocol, implementation issue or new package.

**Proposed boundary:** the core establishes a bounded voting instance, verifies
its admitted participant/selection context, handles scoped votes and exposes
checkable result evidence or an explicit unresolved/unavailable observation.
An application scenario declares its versioned task meaning, admissible inputs,
outcomes and effect authority. Its consumer checks any permitted effect against
its own current contract. A voting result is not an automatic network mutation.
Human deliberation and automatic common-state agreement remain distinct even
when some evidence-handling mechanisms can be shared.

Reusability does not mean one identity, electorate or weight for every task.
The first supported core contract must constrain the policy choices it can
safely support; arbitrary thresholds and participant filters are not harmless
configuration. A new application outside that contract requires explicit
reassessment. No remote executable proposal, unchecked policy callback,
unrestricted plugin loader or access to another owner's secrets is introduced.
The existing no-disclosure and task-admission boundaries apply throughout.

#### Whole-core dependency decisions

This is a map of unresolved design obligations, not an API specification or
evidence that any row has passed. Compare complete candidate compositions;
separately promising components do not establish a compatible system.

| Responsibility | Decision and dependency evidence required before implementation |
|---|---|
| Participation and influence | Define the admitted actor/credential for each supported policy, its scope and lifecycle, and the actual Sybil/influence assumptions. Node count, resource expenditure and willingness alone cannot certify independent operators. |
| Pool and bootstrap | Establish an authentic eligible snapshot, its admission/withdrawal rules and first/restarted participant verification. Expose any cycle in which voting establishes the pool that is needed to authenticate that same vote; supply a justified bootstrap or reject the composition. |
| Proposal admission | Fix the task/rules context, proposal/conflict identity and attempt before selection inputs become exploitable. Bound aggregate offered work and repeated equivalent effects without assuming one key means one proposer. |
| Random selection | Select a verifiable sampling rule and randomness source with precise commitment order, timing, roots and failure behavior. Include withholding, early knowledge and many admitted attempts; a replacement seed cannot silently become a reroll. |
| Willingness and workload | Choose how eligibility differs from availability, whether any confirmation or reserve stage exists, and how its closure is proved. Account for biased volunteers, selective delivery, correlated absence and per-participant task load. |
| Votes and result evidence | Define exact signed context, admissible choices, any abstention/ballot-visibility policy, double-vote/locking rules and compatible certificates. Derive thresholds from the selected fault model; the earlier arithmetic is not a complete protocol. |
| Finality, time and recovery | Specify result exclusivity, pending/unresolved behavior, late evidence, close/retry rules, membership transitions, freshness and crash/long-offline recovery. Local silence cannot prove global rejection or safe replacement. |
| Storage and delivery | Select the common facts, admission/order and data-availability obligations actually required. Supply finite acquisition, persistence, pruning and replay bounds without weakening retained floors or hiding an indispensable producer. |
| Formats and interfaces | Specify canonical proposal, membership/selection, vote and result representations; version/domain binding, parse/sign/proof limits and exact failure semantics. Identify local interfaces and any necessary remote methods, with caller rights, replay/idempotence and resource rules. |
| Privacy and composition | Inventory what each role and colluding roles can observe, their retained metadata and surviving security assumptions. Prove that task inputs, diagnostics and result consumers create no private-data/key acquisition path. |
| Components and operation | Compare whole-system adoption and maintained-component composition against the same obligations. Resolve dependency maintenance, configuration, keys, restart, upgrade and operator work for one Product Owner plus Codex; outsourcing a dependency does not resolve its trust model. |

The shared-state/Name work elsewhere in R-149 supplies integration constraints,
not an assumption that the voting core already replaces Network State or
Namespace consensus. A scenario may need additional ordering, currentness or
authorization evidence beyond a voting certificate. Describe that boundary
before claiming that a core result is sufficient for the scenario.

#### Architecture readiness and development deliverable

Before maintained core development, require one selected coherent design with
all critical dependency contracts and necessary ADRs resolved, a concrete
operating/resource profile, and end-to-end evidence appropriate to each claim.
Ready contracts must cover first entry through result verification and restart,
not rely on mock membership, assumed honest keys, an unexplained beacon or an
unchecked finality verdict. Parameters for committee size, offered load, work
limits, retention, timing and fault assumptions must be assessed together.

**Proposed falsification cases, not executed here:** partition during selection
and voting; suppression of honest readiness acknowledgements; biased willing
pool; many proposals/identities; unavailable or selectively released randomness;
opposite or cross-instance certificates; delayed evidence after local timeout;
crash/rollback and stale bootstrap; evidence/resource flooding; and an application
consumer attempting an effect outside its declared authority. Failure of a
required invariant or dependence on an unspecified component keeps the core
design unready. Local calculations or synthetic runs cannot qualify real human
turnout, operator independence or public-network security.

This gate does not require every future application or every internal coding
choice to be settled. It does require that unresolved extensions cannot change
the selected core's safety, authority, lifecycle, persistence or interface
contracts. If a gap cannot be resolved, report the design as unready or propose
an explicit scope change; do not substitute a trusted placeholder and proceed.

The final handoff package contains:

- the current core contract, selected decisions and exact residual assumptions;
- development tasks in dependency order, each with one observable outcome,
  scope, real owner, prerequisites, failure behavior and acceptance checks;
- verification work attached to those tasks, plus explicit integration,
  adversarial, resource and recovery tasks covering the complete core;
- requirement-to-task-to-check traceability, environments and evidence required
  for acceptance, and the selected issue tracker as the sole live task ledger.

Separate tests do not postpone a component's own verification until the end.
Terra receives implementation-ready tasks after the architecture gate; research
gaps remain design work. The design assistant reviews conformance and resolves
consequential changes. No task is created or activated by this preparation.

### Protective Node-role restriction: proposed requirements and procedure

**Status: design assessment and proposed requirements, 2026-09-07.** This is the
first detailed use case of the open task catalog. The Product Owner selected
its analysis, not implementation, a new ban power, a committee algorithm or a
sanction duration. No runtime experiment, human-panel exercise or new ADR is
performed by this section. Requirements below remain proposals until promoted
through the existing product/security ownership and decision process.

#### Product outcome and decision boundary

Protect participants from continuing use of an unsafe infrastructure duty while
preserving legitimate owners' rights and bounded network operation. The target
is an exact Node key and relevant role assignment/generation. The result is a
verifiable restriction on accepting its role evidence/selecting new work. It is
not proof of the operator's real-world identity, intent or guilt, and cannot
turn off a machine, seize a Service/Name or prohibit all future activity by a
Person. A new key's admission remains a separate Sybil/re-entry question.

Research questions: which harms require a common current restriction beyond
local protection; which evidence permits it without protected data; and whether
a bounded human judgment step adds enough protection to justify false exclusion,
turnout delay and collective admission power. Keep Application/Service-wide
blocking separate from this Node-role case.

#### Evidence and source assessment

**Current owners:** [Node eligibility](../../product/operating-model.md#node-eligibility-and-sybil-boundary)
requires reproducible duties and excludes uploaded User reputation/route history;
[Node lifecycle](../../technical/network-route-node.md#node-and-resource-lifecycle)
checks exact current assignment on new admission; the
[State owner](../../technical/network-route-node.md#state-transition-admissibility)
retains current/pending/conflict evidence. The
[no-disclosure boundary](../../product/operating-model.md#no-disclosure-authority-or-secret-access)
and [threat conditions](../../security/threat-model.md#no-agreement-mediated-disclosure)
apply before any case admission or decision. These owners supply constraints,
not an implemented public restriction mechanism.

**Primary sourced facts, accessed 2026-09-07:**

- [CometBFT evidence specification](https://github.com/cometbft/cometbft/blob/main/spec/consensus/evidence.md)
  separates detection, verification, dissemination, commitment and application
  consequences. Its duplicate-vote check binds signer, height, round, type,
  different block identifiers, membership and chain-bound valid signatures.
  It also describes evidence deduplication/age and possible censorship. This
  supports separating a portable proof from its consequence. It does not select
  CometBFT, stake, slashing or those fields for Ardents.
- [Tor bad-relay handling](https://community.torproject.org/relay/community-resources/bad-relays/)
  distinguishes BadExit/MiddleOnly restrictions from removal from consensus.
  [Tor rejection criteria](https://community.torproject.org/relay/governance/handling-bad-relays/rejecting-bad-relays-criteria/)
  also describe Network Health investigation and differing responses to
  misconfiguration and malicious behavior. This demonstrates narrower role
  effects but depends on Tor's administrative/human process. Ardents has no
  assumed Network Health staff, compatible exit role or adopted Tor authority.

**Inference:** a valid peer signature identifies a signing key's statement,
not an honest independent person or cause of compromise. A failed signature
check cannot attribute the fabricated message to the claimed key. A controlled
synthetic test can establish an observation under its conditions; a signed
report alone does not prove that its asserted observation occurred.

| Evidence class | Permitted interpretation | Proposed handling |
|---|---|---|
| Portable proof of a prohibited authenticated protocol act | Example: the same authorized key signed contradictory final statements for one exact conflict/step when that protocol forbids both. Requires complete rule/context and proof validation. | Eligible for deterministic verification and the rule-defined common restriction. Different rounds/phases/authorized transitions are not automatically misconduct. |
| Expired/incompatible/unqualified duty evidence | The assignment is not currently usable under the selected profile. | Deny that duty according to eligibility rules. Do not invent a guilt finding, escalating punishment or human vote on whether a signature is valid. |
| Reproducible controlled synthetic observations | A test of resources/endpoints controlled for the test, with stated path, timing, capability and attribution limits. | Candidate for bounded investigation; a shared restriction requires a separately justified evidence threshold. Observer count is not independent-operator proof. |
| Missing response, low local throughput, complaint or signed accusation | The observer reports absence/failure/allegation; censorship, framing and common outages remain possible. | Local protection and bounded investigation only by default; no automatic global sanction from complaint count or nonresponse. |
| Invalid signature, forged report or evidence from another context | Failure to establish the claimed authenticated act or applicable context. | Reject the asserted proof. Do not punish the impersonated key. Immediate local resource protection remains available. |
| Private logs, traffic content, keys, recovery secrets or identifying user relationships | Inadmissible input, even if requested to prove abuse or supplied to support a unanimous verdict. | No retrieval/disclosure capability or admitted task path. Investigate using permissible synthetic/public evidence or withhold the proposed global action. |

#### Proposed invariants and effect contract

- Each case binds the network/rules, exact accused key, role/assignment,
  challenged act, applicable decision class and evidence context. A claimant
  cannot select a weaker class or widen the effect to related people, Services,
  Names, keys or operators by assertion.
- Report submission, evidence admission, case finding, finalized restriction
  and current enforceable duty state are distinct. Filing a case has no global
  suspensive effect. Local owners need not wait to refuse unsafe local work.
- Nonresponse is neither admission of wrongdoing nor a NO vote. Multiple reports
  can refer to one incident; their number provides no legitimacy or Sybil weight.
- A strong proof is not converted into a subjective ballot. Its inclusion/current
  consequence may require shared agreement, but every consumer checks proof and
  scope independently. A valid threshold certificate cannot rescue invalid
  evidence, missing owner authority or a prohibited effect.
- An uncertain-case committee, if later justified and accepted, chooses only
  predeclared narrow outcomes over an immutable admissible evidence snapshot.
  It has no search, interrogation, arbitrary-command or private-data capability.
  Recusal/conflict-of-interest claims must not create an unproved human-identity
  system or let a claimant manipulate the roster through an accusation.
- An invalid/insufficient case does not create a global restriction. An unresolved
  case does not establish innocence, guilt or an expiring/active restriction.
  Local unavailable/avoidance decisions remain within the existing owner scope.
- Effects cover only the implicated authority. A wider effect requires a declared
  and demonstrated shared dependency, such as a genuinely shared compromised
  signing boundary, not social association or a count of linked allegations.
- Per-effect new-work/active-work handling is mandatory. Stop new assignments
  first where sufficient; terminate existing affected exposure when continuing
  integrity/confidentiality cannot be justified. A finite drain is allowed only
  when the named harm permits it. No arbitrary rerouting or privacy downgrade.
- Consumer enforcement uses current authenticated state and retained floors.
  Old verdict replay, local restart, delayed certificates and stale state cannot
  recreate old authority or discard an effective restriction by accident.

These are semantic requirements, not wire fields, package names or an accepted
co-resident deployment. A majority's preference is not a resource measurement or
proof of independent control.

#### Proposed lifecycle and contested cases

| Step | Required basis | Effect or explicit failure |
|---|---|---|
| Receive a report | Finite local admission; closed allowed evidence; no private-data collection. | A bounded report, not a restriction. Overload is explicit and does not accuse the target. |
| Validate and group the case | Exact attribution/rules/scope and semantic incident identity. Equivalent proof packaging and reordered statements cannot manufacture independent cases. | Reject inadmissible/duplicate inputs or retain bounded new evidence. A weak pending complaint cannot reserve the target and block stronger valid proof. |
| Choose the permitted path | Proof-defined case, eligibility check, or an accepted uncertain-evidence policy. | Automatic checking or a fixed bounded review procedure; no generic free-text power to invent a remedy. |
| Assess evidence | Complete valid context; if human judgment is used, only admitted evidence and fixed authority/threshold/finality conditions. | Substantiated permitted finding, insufficient basis or unresolved. Hearing the accused is possible through a bounded signed response, without disclosure or treating silence as admission. |
| Finalize a disposition | The chosen agreement protocol's compatible certificate/close and state transition. | A verifiable committed disposition; local timeout alone cannot finalize rejection, prove absence of a certificate or activate a replacement jury. |
| Activate the effect | Fresh compatible state, exact affected duty and independently valid evidence/consequence. | Consumers stop accepting the restricted role according to its new/active-work policy. They do not depend on the hostile Node voluntarily complying. |
| Correct or contest | New admissible evidence, a demonstrated verifier/context error, or a later accepted review decision with a valid successor. | Correct current state prospectively, preserving prior evidence/floors. An appeal does not automatically suspend a valid restriction, erase history or create a reroll. |
| End a provisional restriction / request readmission | A qualified shared end condition or corrective successor, followed by current ordinary eligibility and any required recovery. | The provisional effect ends, but old credentials are not resurrected. Missing fresh state means usability is unknown/unavailable, not proof of permanent guilt or automatic readmission. |

For **proven unsafe authority**, fence the affected assignment/generation rather
than treating a punishment timer as key repair. Returning requires a new allowed
assignment/current eligibility and, where the finding concerns compromised key
material, a qualified recovery/key change from a surviving owner boundary.
A human claim that the key is now safe is not recovery evidence. Whether the
same long-term key may requalify is an explicit mechanism-specific question.

For **provisional restriction based on incomplete evidence**, require a finite
scope/duration and cumulative cap for the same underlying case. Expiry ends the
provisional finding's effect; replayed reports cannot renew it. Genuine new acts
need their own applicable rule, evidence and cumulative-impact policy. No number
of reordered copies, review labels or new reporting identities creates a new
incident. Exact windows, safe end proof and cumulative policy are unselected.

An appeal for a machine-checkable proof can establish a concrete context/verifier
error; it cannot vote a valid contradiction away. Review of a judgment-based
finding requires its own acceptance and finality conditions. Evidence arriving
after a fixed snapshot needs a qualified successor/review path; the old jury must
not silently judge a different proposal. No guarantee of fair hearing or timely
correction exists under complete evidence/communication capture.

#### Same hostile cases across alternatives

The rows are analytical counterexamples and required future validation cases,
not executed network tests, measured detection rates or independent review.

| Situation | Deterministic proof path | Human review path | Required common boundary |
|---|---|---|---|
| Attacker fabricates an invalid signature under an honest key | Reject the proof; cannot attribute the act. | No discretion to declare it authenticated. | The framed key receives no global restriction. |
| Two signatures are real but belong to different permitted rounds/steps | Check the exact selected rule before treating them as contradictory. | Cannot replace missing offense context with intuition. | No false equivocation finding. |
| Ten thousand keys repeat one accusation | No new proof or effect from count. | Jury input/attention must stay bounded; repetition is not corroboration. | Deduplicate the incident without suppressing materially new valid evidence. |
| All alleged failures are missing replies during a partition | No portable proof of malicious omission from absence alone. | Multiple observers may share the same partition or controller. | Local protection; shared provisional action needs a separately qualified evidence policy. |
| A valid signed violation is withheld from some readers | Readers with proof can protect their own use; shared commitment may stall. | More votes do not force evidence delivery. | Bound acquisition and report unknown/unavailable; do not assert global removal occurred. |
| Accused identity is a current agreement producer | Verify proof without erasing prior commitments. | A complaint cannot redraw its own electorate. | Safe configuration continuity; never lower the old threshold or count excluded seats away to make progress. |
| Removal leaves too little safe capacity | Do not keep using proven unsafe authority merely to meet a count. | A temporary-review budget is not permission to ignore proven harm. | Withhold affected availability/safety claims and fail boundedly; preplanned qualified spare capacity is a separate condition. |
| A provisional restriction reaches its end while readers have different state | Require the profile's common end/currentness evidence. | Local calendars cannot manufacture a globally final expiry. | No stale ban extension or old-authority resurrection. |
| An honest key was compromised | Proof concerns the key's act, not the person's intent. | A sympathetic verdict cannot make leaked key material secret again. | Qualified owner recovery and fresh assignment; no committee access to recovery secrets. |
| The actor returns with a new key or Target | The old proof applies to its declared scope. | A claim of common ownership is not a universal identity oracle. | New admission/Sybil defenses are needed for broader prevention; no de-anonymization shortcut. |
| A coalition falsely restricts honest competitors | Invalid machine proofs are rejected by correct consumers; real common-state assumptions still matter. | A coalition inside the decision's trust envelope can abuse subjective authority. | Measure cumulative exclusion and concentration; a finite term alone does not establish safety. |
| An investigation requires real users' captures or keys | Cannot admit the required private-data path. | Unanimity does not create an exception. | Reject the design of that task or use permissible synthetic evidence. |

#### Option comparison and recommendation

| Option | Benefit | Limitation / rejection condition |
|---|---|---|
| Only local refusal and ordinary eligibility | Fast owner-controlled protection with little shared case state. | New or differently connected Endpoints may repeatedly encounter known unsafe duty evidence; a common proven consequence can be useful. |
| Local protection plus automatic verification and shared effect for portable proof | Reproducible consequences, no human turnout for an objective violation, compatible with narrow public authority. | Does not cover every malicious act, prevent evidence censorship or solve re-entry and configuration capture. Requires a real selected proof and lifecycle. |
| The previous option plus a bounded judgment procedure for uncertain evidence | Could address harmful behavior with admissible but non-decisive observations. | Adds false-exclusion/collusion, evidence availability, discretion, review and human-workload conditions. No such bound or staffed public procedure is demonstrated. |
| Human approval for every restriction, including objective invalidity | A uniform social process. | Delays deterministic protection and invites voting over facts; does not improve proof validity or guarantee fair review. Not recommended as the core path. |

**Recommendation, not an implementation selection:** develop the second option's
requirements first. Keep the third as an explicit open candidate with a named
product benefit and measurable false-exclusion/availability limits; do not close
it merely because the core has not yet justified a mandatory human jury.
No new permanent committee or unrestricted global blacklist follows.

The strongest counterargument is coverage: attackers can cause selective
withholding and subtle routing harm without generating a portable signed proof.
Local avoidance may be insufficient for fresh clients, while subjective global
review may create a stronger censorship tool. This trade-off is unresolved and
must be evaluated with allowed evidence, not settled by increasing vote counts.

#### Readiness, falsification and remaining decisions

Before a profile or ADR is accepted, specify and test the exact violation proof
and permitted projection; same-case identity/deduplication; effect scope and
new/active-work policy; finite retention/verification/submission costs; inclusion
and currentness bounds; safe restriction/end/re-entry/configuration transitions;
and the minimum surviving execution, cryptographic and communication conditions.
For a judgment path additionally specify admissible observations, maximum false
exclusion and false acceptance, cumulative restriction, reviewer selection and
workload, response/review inclusion and an independently usable corrective path.
Complaint throughput, protection latency, finalization deadline, maximum duration,
review/appeal attempts, proof/metadata sizes and spare-capacity thresholds remain
unselected. The earlier illustrative 24-hour vote is not reused as any of these.

Future qualification is rejected if the procedure can restrict an unimplicated
key by fake/foreign-context evidence; widen effects or acquire private data;
turn silence/report count into guilt; indefinitely renew a provisional case;
auto-resurrect unsafe credentials; shrink a live quorum after an accusation;
consume unbounded resources; or claim safe progress beyond its declared surviving
conditions. Required hostile rows above are specified but NOT executed. No new
calculator, signing code, package, dependency or live test is introduced.

**ADR preparation:** a consequential choice between strictly proof-defined common
restriction and binding human judgment remains open. Do not mark it accepted by
writing a draft. Current ADR-0074 and NET-20 already fix the owner/no-disclosure
boundary; a later decision must name the exact permitted effect and evidence,
including any change to the existing deterministic eligibility contract. Terra
receives a slice only after those applicable choices and acceptance cases close.

### Coupled feasibility result

**Status: analytical comparison executed on 2026-09-06; no complete public
profile selected.** The Product Owner explicitly requested the joint check.
The [predeclared worksheet and calculator](../../../experiments/r-149-coupled-feasibility/README.md)
retain parameters, formulas, captured evidence, hostile counterexamples and
falsification criteria. This is an executed arithmetic experiment; the
C01-C26 product conformance cases below remain unexecuted.

**Assumptions:** 1,000 infrastructure declarations and 10,000 Names; Node updates
every six hours plus 5% daily churn; a native-work comparison with a 60-second
reference interval, five-minute Epoch, q<=30% hostile work and 80 confirmations;
30-minute State age with a four-hour sensitivity; justified initial time within
30 seconds and surviving drift <=50 ppm; a 30-day Name Lease, seven-day Grace,
and renewal starting on day 23. Larger scale, faster churn, greater resource
capture, clock loss and longer censorship are explicit adverse cases.
These are research inputs, not adopted product values or measured deployment
facts. No native-work, timestamp, block or compact-proof format is selected.

| Coupling | Calculated result | Decision consequence |
|---|---|---|
| Work / State age | At q=30%, 80 honest confirmations average 114.29 minutes; p99 is 146.09 minutes. The selected 180-minute planning allowance plus cutoff, acquisition and clock margin totals 199 minutes. | The 30-minute candidate fails. Four hours gives conditional arithmetic room but permits older infrastructure state; neither timing nor its product acceptability is guaranteed. |
| Work / commitment risk | The exact fixed-rate concurrent race has catch-up probability 0.15645 with six confirmations and 1.33779e-7 with 80 at q=30%; at q=40%, 80 gives 0.01072. | Eighty passes only the stated per-race comparison. It supplies no arbitrary-adversary or lifetime finality bound; real network delay, partition and work variation are outside that formula. |
| Commitment / claim reveal | Five-minute E+1 does not fit waiting for commitment finality before revealing under the selected allowance. | Either explicitly assess provisional revelation followed by committed final use, or research a longer Epoch. The current E/E+1 rule alone does not decide that choice. |
| Fresh client / history | At the small scale, one year of common input history is 1.537 GiB and needs at least 132.05 seconds to transfer at 100 Mbit/s, before any verification or Route work. | Requiring that replay at normal clean start fails the 15-second target. A current snapshot without a complete acceptable proof is not the missing solution. |
| Idle client / scale | Small-scale deltas plus hypothetical 8 KiB proof/5 min and assumed overhead/reserve use 17.844 MiB/day; 64 KiB proofs use 41.469. At 10,000 Nodes the smaller proof case uses 71.238. | Small proof fits are conditional on an actual construction and measured total costs. Exceeding 25 MiB is a secondary efficiency failure, not automatic protocol insecurity or an independent release blocker. |
| Lease / censorship | Day-23 renewal plus 13 days of withholding and 199 minutes processing leaves 20 h 41 min before day-37 Grace end; 14 days finishes 3 h 19 min late. | A finite inclusion and finality envelope is required. Prolonged omission can permit a later valid reclaim without forging the old Authority. |
| Admission / finite resources | A modeled 16 MiB queue of 4,096 maximum-size Name updates fills in 415.14 seconds at 10 valid updates/s against nominal 8/min service. | A finite queue, per-key cap or separate renewal class does not establish individual inclusion. The assumed ten-minute inclusion bound remains unproved under valid Sybil input. |
| Honest supply / safety and liveness | With H=70 and A=30, losing half of honest work raises q to 46.15%, modeled catch-up to 0.33059 and mean 80-confirmation time to 228.57 minutes. | Both risk and freshness assumptions fail together. Work cost, honest funding and scarcity must be justified; Node count, claimed bandwidth and assumed lack of profit cannot stand in for them. |
| Time / restart | +/-30 seconds plus 50 ppm surviving drift becomes +/-159.6 seconds after 30 days, exceeding the +/-120-second comparison limit after 20.833 days. A saved lower watermark alone has no fresh upper bound after power-off. | Continuous operation, routine restart and clean/long-offline bootstrap need different evidence. Agreeing hostile time sources or producer timestamps cannot certify themselves. |
| Name lifecycle / proof validity | Parent Grace ending on day 20 defeats a child's own day-23 renewal schedule. A signed Record expiring on day 30 can interrupt resolution before a still-permitted renewal inside Grace commits. | Parent/own lifecycle, Record validity and current-proof freshness require separate checks; do not incorrectly equate a resolution gap with loss of the Grace renewal right. |

**Sourced fact and calculation scope:** the worksheet uses
[Grunspan/Perez-Marco's exact race analysis](https://webusers.imj-prg.fr/~ricardo.perez-marco/publications/articles/doublespend.pdf),
not Nakamoto's Poisson approximation, and checks two exact rational expressions.
The [Bitcoin paper](https://bitcoin.org/bitcoin.pdf) supplies the distinction
between full validity and header/inclusion evidence.
[RFC 8915 section 8.6](https://www.rfc-editor.org/rfc/rfc8915.html#section-8.6)
describes the separate delay problem for authenticated time. Sources were
accessed on 2026-09-06; using them does not select their systems for Ardents.

**Inference — combined hostile cases:** an isolated fresh participant receiving
a valid six-hour-old view and consistently shifted time cannot establish
currentness from those sources alone. A partition may expose two incompatible
committed histories; a correct verifier that obtains the conflict must retain
floors and refuse the affected transition, while an isolated verifier may never
observe it. Receipt of a censored renewal does not extend the Lease. Loss of
honest work, admission saturation and clock evidence can defeat all favorable
timing assumptions simultaneously. Correct local refusal itself still requires
a surviving execution boundary; the fully compromised endpoint case receives
no such promise.

**Limited recommendation from this worksheet:** reject its concrete
30-minute/finality combination and its full-history ordinary-client bootstrap.
The four-hour, small-scale delta variant is only an arithmetic comparison.
Neither result establishes a preferred architecture or constraints on every
non-PoW candidate. The [task-first comparison](#task-first-mechanism-comparison)
now owns the next research step: choose the required obligations, then compare
mechanisms and their actual evidence. An imaginary compact proof is not an
implementation prerequisite selected by a payload budget. No CPU/RSS,
proof-generation cost, public anonymity, independent-operation or performance
qualification was obtained.

### Outcome-first decision preparation

The Product Owner asked on **2026-09-06** to identify the tasks before choosing
concrete mechanisms, and explicitly required considering every part of the
environment potentially hostile. This is the next decision boundary within
R-149, not a new implementation stream or a replacement product selected by a
research record.

The [functional map](../../product/functional-map.md#outcomes-before-mechanisms)
now groups nine existing product outcomes without introducing new requirement
IDs or performance numbers. The
[threat model](../../security/threat-model.md#hostile-environment-premise-and-residual-assumptions)
owns the hostile-component cases, protected interests, residual assumptions,
measurements and non-claims, including collusion, total isolation and local
endpoint compromise. The previously proposed shared-state semantics and verifier
brief remain proposals; their existence does not answer these upstream questions.

**Sourced facts, accessed 2026-09-06:**

- [RFC 3552, sections 2 and 3](https://www.rfc-editor.org/rfc/rfc3552.html#section-3)
  separates communication security properties and models a largely attacker-controlled
  communications channel. Its usual uncompromised-endpoint assumption is explicit;
  it also discusses limiting damage from compromised systems. **Project distinction:**
  our adversary cases include endpoint compromise rather than silently excluding it.
  Any claim that still needs a surviving endpoint or key boundary must say so.
- [RFC 7258](https://www.rfc-editor.org/rfc/rfc7258.html) treats pervasive content
  and metadata monitoring as an attack, and distinguishes mitigation from complete
  prevention. **Inference:** a payload-protection result does not by itself pass
  a location, relationship, availability or global-observer requirement.
- [Tor's directory-authority policy](https://community.torproject.org/policies/dir-auth/dir_auth_expectations/)
  assigns trusted authorities responsibility for a common view and exclusion of
  harmful relays; ambiguous cases involve operator judgment. The authorities
  collectively decide membership. **Inference:** removing this appointed role
  removes a specific defensive capability. Ardents cannot claim equivalent
  protection merely by retaining path-selection heuristics or adding consensus.
- [Tor's path-selection specification](https://spec.torproject.org/path-spec/path-selection-constraints.html)
  uses bandwidth weighting and known-family/subnet constraints. **Inference:**
  capacity, identity count, known relationships, and actual independent control
  are different evidence. These facts do not select Tor routing, weights, a
  committee, or an operator-identification system for the public successor.

#### Decisions needed for the tasks

| Task boundary | Decision-relevant question | Evidence or explicit limit needed before a mechanism is selected |
|---|---|---|
| Useful reachability under interference | Which role positions, links, discovery sources and share of usable capacity can the adversary control, for how long, while the Service must remain reachable? | A declared topology/adversary/workload and bounded success/failure/overhead observations. Also run total isolation as an unavailable case; an assumed honest path is not evidence that a client can discover and use it. |
| Privacy under combined observation | Which locations and relationships must remain hidden from which combinations of observers, including a malicious intended endpoint? | The exact claim and leakage observations; current Interactive exclusions remain. A stronger claim requires its own product decision and research, not stronger wording for the same result. |
| Fresh public bootstrap | What can a first-start or long-offline participant establish when all reachable sources collude and local time or initial software provenance may be compromised? | Name the minimum independently justified identity, execution, freshness and proof conditions; otherwise state which result cannot be established. Source agreement is not a substitute. |
| Canonical ownership under censorship | What should happen when a valid owner renewal is withheld throughout Lease and Grace, followed by an otherwise admissible reclaim? | Separately evaluate authorization, input inclusion, expiry and finality. Select an enforceable bounded-inclusion assumption or explicitly reconsider the conflicting product promise; do not label missing input voluntary abandonment. No new outcome is selected here. |
| Local and shared resource abuse | Which legitimate operations must retain capacity when an attacker splits identities, reuses resources or buys most of a required role? | Whole-owner work accounting, adversarial load and role-specific influence/availability evidence, plus actual honest resource supply. Existing local limits do not establish a public Sybil or censorship threshold. |
| Compromise and continuing ownership | Which damage must stay inside the compromised Application, host, key or software channel, and which authority could still authorize recovery? | Identify a surviving boundary and distinguish future revocation from undoing past disclosure. If all relevant execution and recovery authority is compromised, no trustworthy local diagnostic or automatic safe recovery is promised. |
| Independence and maintenance | Can the selected journeys continue without founder services while the actual team can maintain the mechanism and its honest-resource requirements? | Trace all required sources, clocks, publishers, agreement and data paths. No silent appointed rescue role, presumed volunteer population, or assumed external operations team. |

These are decision questions with no selected numerical attack threshold. A
statement such as "the attacker has half the nodes" cannot fill the worksheet:
relay selection, route position, routing control, production influence and
operator independence are different dimensions. The adversary may exceed every
supported threshold; the threat model must then state which claims disappear.

**Disposition of this pass:** the product tasks and hostile-environment premise
are prepared in their current owners. Complete their claim conditions and
observable outcomes before promoting a consensus, routing or naming candidate.
The existing 26 conformance cases remain unexecuted future checks, not a proof
that all hostile scenarios are handled. No experiment, implementation issue,
new ADR acceptance, public claim or new Application/Overlay Service is selected.

### Requirements preparation results

The Product Owner authorized detailed requirements and decision preparation on
2026-09-06, after clarifying that open participation is not equal voting weight
per identity and lack of an internal reward is not a Sybil-security assumption.
The next preparation pass inspected current owners at
`26c95e12448d527ef3971e384b70d0cde31df9f3`; no experiment was run.

The [functional map](../../product/functional-map.md#autonomous-public-requirements)
now owns NET-19 through NET-28 and their fixed/working maturity. The
[operating model](../../product/operating-model.md#autonomous-shared-state-contract)
owns the working operations, derivation, activation, duplicate, and failure
semantics. [Proposed ADR-0080](../../adr/0080-separate-public-state-acceptance-obligations.md)
records the consequential validity/agreement/activation and full/bounded-reader
trade-off. These sections supersede the earlier outline below as the current
working proposal; accepted C0 technical behavior is not changed.

**Sourced fact:** [RFC 9162](https://www.rfc-editor.org/rfc/rfc9162.html),
accessed 2026-09-06, distinguishes logged submissions and Merkle inclusion and
consistency proofs. It is an experimental Certificate Transparency protocol,
not an Ardents consensus selection. **Inference:** a receipt, inclusion proof,
and extension of a particular log each prove a different proposition; none alone
establishes all valid globally submitted work, current time, or correct derived
Ardents state. This source supports the proof-obligation distinction, not a
recommendation to adopt CT's log authorities or discretionary admission.

**Sourced fact — current naming owner:** Network ordering accepts opaque admitted
Name inputs; Namespace owns current Records, lineage, recovery, and materialization.
**Inference:** independent verification must not quietly copy Name/Authority
records into every consensus producer. A public successor still owes a Namespace
validity/availability proof path. That unresolved composition is part of the
mechanism gate, not permission to trust an opaque root or a new administrator.
Unlisted names are not secret names under the glossary.

**Inference:** deterministic derivation and local readiness must use different
inputs. Two verifiers with different credible clocks can disagree about current
usability, but must derive the same projection from the same agreed reference,
predecessor, and sequence. An invalid operation correctly rejected in the agreed
input is different from a candidate claiming that invalid operation succeeded.

#### Existing budget consequences

These are calculations from the functional-map requirements, not additional
normative limits or measurements. NET-14B includes first-start and routine-restart
readiness, rather than only opening a socket. NET-14A/C time already-registered
name use; they are not registration-finality deadlines. NET-14G/H charge the
selected infrastructure and idle Endpoint profiles respectively, including
required helper work. NET-14I is an idle-traffic efficiency guardrail, explicitly
not an independent release blocker.

From NET-14I's 25 MiB/day, the mean allowance is about **303.41 bytes/second**
for all charged idle carrier traffic. Illustrative mandatory control payloads,
before authentication/transport overhead, retransmission, time, and update traffic:

| Payload refresh assumption | Derived payload per day | Interpretation |
|---|---:|---|
| 1 KiB once per minute | 1.40625 MiB | Leaves headroom, but proves no required validation/freshness property. |
| 16 KiB once per minute | 22.5 MiB | Uses 90% of the entire idle guardrail before the other costs. |
| 32 KiB once per minute | 45 MiB | Exceeds that guardrail before overhead; not by itself a security or release verdict. |

The interval and sizes are arithmetic examples, not a selected protocol.
The existing idle-client memory/CPU and startup targets motivate a bounded reader;
mandatory production or unbounded replay is not a free implementation detail.
Snapshot/proof amortization still requires correct verification, freshness,
availability, and privacy. No useful authenticated snapshot is assumed to exist
merely because its payload could fit the budget.

#### Parameter decisions that must precede implementation

| Parameter family | Existing constraint / required relationship | Prepared decision needed |
|---|---|---|
| State scale | All required roles fit their selected reference profiles; complete View/Namespace claims remain truthful. | Supported Node/Name counts, update/churn rate, accepted/rejected input limits, and adversarial offered load. Current small tracer corpora are not public-scale commitments. |
| Epoch/admission and Name finality | Commit in E and reveal in E+1 remains the Name rule; clocks cannot choose priority. | Cutoff/admission proof, Epoch duration, finality delay/risk, registration latency, and the bounded withholding/censorship outcome. Do not equate Name registration latency to NET-14A connection latency. |
| Freshness | Finite activation/lease bounds, trusted continuity, no stale resurrection. | Clock interval/error and drift, restart/long-offline bootstrap, maximum usable observation age, refresh/retry cadence, and every relevant work deadline. |
| Full and bounded verification | NET-14B/G/H and the secondary NET-14I guardrail apply to the whole charged role. | Initial/recovery input bytes, proof and parse cost, maximum history/checkpoint data, resident/retained storage, and per-refresh/idle work accounting. |
| Inputs and proofs | An admitted legal value must fit signing, persistence, proof generation, and actual transport together. | Exact grammars, maxima, canonical encoding, replay identity scope/retention, proof sizes, and overflow behavior; no independent limits that compose into an impossible value. |
| Consensus resources | Identities and relay throughput do not establish honest voting resources. | Scarcity/anti-reuse argument, honest supply and cost, hostile concentration/rental envelope, membership changes, and loss-of-quorum recovery limits. |
| Pending/conflict retention | No optimistic use and no unbounded branch/evidence store. | Maximum candidates and history, compaction/checkpoint proof, committed-floor representation, durable conflict bounds and restart semantics. |
| Software safety | Publisher loss is distinct from an advisory, objective invalidity, and selected local activation. | Initial artifact trust, supported channels, owner policy, stale-advisory handling, unknown-flaw response, and explicit fork/compatibility rules. |

Current numeric product budgets remain owned by the functional map. Unknown
parameters stay explicit decisions; they are not zero, unlimited, or an assumed
honest deployment. A candidate response must fill the table with one coherent
profile and show how the values jointly satisfy the selected outcomes.

#### Conformance cases prepared for the successor

The case identifiers below are research evidence labels, not runtime fields,
package boundaries, or a live task ledger. They define inputs and observations
for the eventual selected grammar/proof; they are not executed test results.
Each state-changing case also checks that the unaffected owner roots and
committed floors remain unchanged on refusal.

| Case | Input / situation | Required observation |
|---|---|---|
| C01 | Valid owner action, exact predecessor, complete correct committed candidate, credible active time. | One durable successor becomes usable for its permitted capability. |
| C02 | Correct signature by a different owner's key; producer claims the operation succeeded. | Candidate projection rejected; no authority change even with otherwise sufficient consensus evidence. |
| C03 | Invalid owner operation is represented as a deterministic rejection inside the protocol's admitted domain. | It grants no rights; the rejection alone does not invalidate an otherwise correctly derived candidate. |
| C04 | Same authorized operation repeated with identical bytes and replay identity. | No second semantic effect, extra eligibility, or weight solely from repetition; admission/log budget remains bounded. |
| C05 | Same replay identity reused with different bytes. | Explicit mismatch/conflict disposition; never treated as an exact retry. |
| C06 | Two competing owner successors share a predecessor and appear at different agreed ordinals. | Projection follows agreed order and per-operation predecessor rules; both cannot become current. Local arrival order is irrelevant. |
| C07 | Authenticated input prefix omits an item that its own admission commitment requires. | No accepted complete projection. A gossip message without such a commitment is not falsely presented as this proof. |
| C08 | Valid inclusion proof but false total, eligibility summary, rejection disposition, or state root. | Insufficient/invalid evidence is refused; an included leaf is not full-state correctness. |
| C09 | Producer supplies a commitment but withholds required input/state. | Unavailable within finite acquisition bounds; no optimistic current state. |
| C10 | Valid provisional competing branches without a committed contradiction. | Neither grants current owner/duty authority solely from provisional status; no forged permanent conflict is inferred. |
| C11 | A valid committed future successor before activation. | It remains non-usable; no new duty or floor interpretation grants early authority. |
| C12 | Credible time interval overlaps a validity start or end but is not wholly inside it. | No current activation under the working interval policy. Exact not-after is outside. |
| C13 | Identical committed input/reference, different honest verifier clock observations. | Same derived root/dispositions; readiness may differ conservatively. No divergent shared expiry/reclaim is invented locally. |
| C14 | Committed evidence expires, or trusted time continuity is lost on restart. | No affected fresh work/recovery or lease extension; retained floors survive and existing work follows finite owner deadlines. |
| C15 | Wrong network/rules/profile or a committed predecessor below the retained floor. | Refusal without reinterpretation, rollback, or fallback to the administrative profile. |
| C16 | Proven incompatible committed histories, or reorganization below an exposed commitment floor. | Explicit bounded conflict/fork handling under the selected model; no silent old-authority resurrection. |
| C17 | Malformed rival packet or unauthenticated assertion of a fork. | No permanent poisoning of a valid committed state from that allegation alone; selected acquisition failure semantics remain explicit. |
| C18 | Crash before durable commit, after commit, or during pending activation. | Reopen yields only a valid committed predecessor/successor or explicit storage failure; no half-published authority. |
| C19 | Data/proof/count exceeds the selected limit, or valid individual parts exceed the combined transport/store bound. | Bounded rejection before unsafe allocation/signing/publication; the verifier does not silently truncate a complete claim. |
| C20 | Founder signers, original Entry/Source services, time witness, and official publishers are gone. | Exact full fresh/restart journey succeeds only with the qualified replacement evidence and declared surviving population; cached operation alone is not success. |
| C21 | Honest resource or source availability drops below the selected model. | Explicit unavailability; no claim that attack profit is too low, no emergency administrator resurrection. |
| C22 | More keys share the same accounted resource allocation. | No cheap aggregate influence multiplication under the chosen resource rule; per-key counting cannot pass this case. |
| C23 | Network ordering receives opaque Name input but no proof path for claimed current Namespace semantics. | It may not manufacture a current Binding. Neither owner signature alone nor an opaque root completes Namespace verification. |
| C24 | A committed Name recovery/update is followed by replay of the old Authority or old parent lineage. | Old authority is refused and no local rollback makes its signatures current again. |
| C25 | A mandatory data source is also an excluded Route/Resolution family. | Existing exposure exclusions and bounded source/selection order remain enforced; no availability workaround links the roles. |
| C26 | An update feed disappears, replays expired metadata, recommends a build, or requests execution. | Distinct selected local outcomes; disappearance is not a universal kill command, stale metadata cannot authorize new bytes, and a feed cannot install code itself. |

#### Prepared first development brief

**Outcome:** one bounded offline public-state verifier accepts a real
profile-authenticated successor, derives its infrastructure projection, commits
it through the existing State ownership boundary, and returns the same truth
after reopen. A Node-owner announcement and successor exercise real authorization,
ordered dispositions, finite activation, and refusal. This is the first candidate
slice after its prerequisites pass, not implementation permission now. The
current core-first priority does not select this example as the first voting
implementation slice; it also requires the whole-core architecture gate above.

**Owner and interface:** start from `internal/network/state` and its current
private verification/commit seams. Keep source retrieval and commands outside
acceptance. Any full-consensus or proof implementation boundary must earn a
package under the existing architecture policy; this brief creates none.
Namespace, Release, Custody, Route and Endpoint retain their existing powers.
No trusted `valid=true`, fixture-only finality certificate, pluggable unchecked
verdict, live administrative bypass, or raw private key is a production input.

**Scope:** the chosen bounded full-verifier path and its actual selected proof,
not producer networking, mining, a DAO, a public Namespace runtime, a publisher,
a new user journey, or a supported public launch. A bounded Endpoint reader
requires its own honest evidence path; it is not supplied by assuming the full
verifier's success. Synthetic data can exercise the real chosen checks without
standing in for proof validity.

**Acceptance:** C01-C19 apply where their selected profile includes the feature;
any excluded case has a declared non-claim and no accepting code path. At minimum,
valid/unauthorized owner actions, duplicate/competing successors, false projection,
withheld/oversized input, future/stale state, profile mismatch, committed conflict,
and crash/reopen need actual public-boundary behavior evidence. Module and
process checks run in their selected profiles with the ordinary repository gates.

**Blocking choices, now explicit:** accepted successor semantic contract;
actual agreement/finality and data-validity proof with its trust roots; exact
input/state/proof grammar and joint size limits; selected freshness/activation
profile; committed/pending/conflict storage and compatibility mapping; and the
coherent workload/resource envelope above. Until they are resolved, this is a
reviewable implementation brief, not a development-ready consensus system.

**Tracker handoff:** after these choices pass, create/select one bounded issue
in the chosen delivery scope with this outcome, real owner, prerequisites,
conformance subset and stop conditions. No issue is started or assigned by this
record. The accessible C0 milestone remains the current implementation ledger;
this preparation does not interrupt or duplicate its live work.

### Authority inventory and replacement conditions

| Present dependency | Proposed public replacement | What must survive / retirement gate |
|---|---|---|
| Appointed Epoch authorities and materialization attesters | Open agreement on valid shared transitions plus independently checked state derivation. | Complete input/View argument, data availability, freshness, finality/fork detection, immutable snapshots, non-decreasing safety floors. Merely changing `Threshold` is insufficient. |
| Package/enrollment pin delivered by the Product Owner | Verifiable initial distribution and explicit owner selection of network/rules, with replaceable distributors. | Initial software trust remains explicit; the founder is not the sole provider of indispensable current secrets or approvals. No self-authentication by a fresh executable. |
| Alpha Entry Invites and Source CA/client-pin access | Objective bounded public admission and replaceable authenticated source access. | Recipient/possession binding, replay prevention, private discovery and finite queues. Each host may control its own resources; no central permission for joining the entire network. |
| Separate C0 time-witness key | A selected founder-independent freshness/time contract, coupled to the consensus and bootstrap design. | Monotonic continuity, persistent floors, restart/long-offline behavior, bounded clock uncertainty. A stale chain cannot certify its own freshness by circular reasoning. |
| One selected Transit issuer/duty generation | Replaceable eligible scoped duties or a separately justified alternative. | Local capacity ownership, one-use credentials, target-free requests, at-most-once journals, and receiving enforcement; no wider signing power. |
| Publisher-issued Release Safety and Compatibility policy | Separate authenticated software channels/local safety choice from objectively checked protocol compatibility. | Artifact provenance, trusted initial execution, safe explicit activation, floors, and finite work. No universal publisher heartbeat or arbitrary on-chain build ban as a hidden permission-to-run. Detailed vulnerability policy is a blocking decision. |
| Threshold Namespace close/materialization | Agreed ordering plus deterministic owner/lease/recovery transitions and current proofs. | Commit/reveal ordering, private submission, no front-running shortcut, bounded finality, parent lineage, no old-authority resurrection. Owner-selected recovery remains. |
| Emergency administrative incompatibility/revocation | Fixed rejection of objectively invalid behavior plus explicit owner decisions for new flaws/incompatible changes. | No universal automatic repair claim. Network outage or explicit fork may be necessary; a rescue key cannot be quietly reinstated. |
| Auditor, builder, qualification, or operator-family evidence | Independently checkable evidence and honest limitation of what is only attested. | No consensus transaction creates an audit, proves independent ownership, or qualifies anonymity. Unmet independence conditions withhold the claim. |

### Shared-state contract owner

The first operation outline has been promoted into the working
[operating-model contract](../../product/operating-model.md#autonomous-shared-state-contract),
with maturity and requirement IDs in the
[functional map](../../product/functional-map.md#autonomous-public-requirements).
Research findings, parameters, conformance cases and decision readiness are
recorded above rather than maintaining a second copy of the lifecycle contract.

### Dependency order and exit gates

The rows are ordering constraints, not delivery labels or runtime identities.
Each later implementation slice belongs in the selected tracker after its
contract and research gates pass.

| Step | Concrete deliverable | Exit gate |
|---|---|---|
| 1. Authority and shared-state contract | Complete the working operation contract in the operating model and its unresolved parameters; enumerate every indispensable signer/source; define profile identity, client outcomes, and full/bounded verification budgets. | Every capability has a founder-disappearance trace; no hidden permission, global data leak, or undefined conflict/freshness outcome. |
| 2. Coupled mechanism and bootstrap selection | Compare a native work reference and, only if admitted, existing settlement against the exact same state/Name and fresh-client contract. Include honest resource economics, state availability, time, partitions, and long-offline clients. | One candidate satisfies declared budgets and threat assumptions with an available maintenance path, or record infeasibility and request a narrower product/dependency decision. Consensus is not selected independently of these conditions. |
| 3. Successor verification contract | Select exact proofs/records and storage/compatibility rules; assign actual owners and imports; build the smallest bounded verifier/deterministic state transition tracer after its prerequisites pass. | Independent vectors reject forged, replayed, missing, forked, wrongly scoped, and unavailable input; crash/reopen preserves committed truth. No speculatively generic interfaces. |
| 4. Isolated state progression and entry | Exercise one full verifier and a fresh bounded Endpoint reader on an isolated network; replace founder-required discovery, admission, time, and scoped-issuer paths for one Target-Link journey. | State, admission, and time continue after their associated project services disappear. Exercise fresh/restarted publication, connection, and drain with every remaining Release dependency disclosed; this alone is not complete founder independence. Local multi-process evidence is functional only. |
| 5. Release safety and voluntary evolution | Select channel/provenance, compatibility, vulnerability, activation, and explicit-fork behavior; remove exclusive ongoing publisher permission only under that successor. | Losing all official publishers does not by itself revoke a valid installed implementation; malicious feeds cannot install code or erase floors; critical-flaw and incompatibility outcomes are honest and bounded. |
| 6. Canonical naming | Compose shared ordering, Name lifecycle/Custody, full materialization, and private Resolution through actual runtime boundaries. | Competing claims, renewal/reclaim, recovery, partitions, and reorg/finality cases preserve authority and current proofs at admitted scale. Target-Link qualification alone cannot pass this gate. |
| 7. Explicit adoption and authority retirement | Qualify the exact successor, migration/clean-start policy, user outcomes, retained evidence, and claim wording; retire the predecessor only after dependent journeys work. | Old keys are unable to authorize any successor operation; public independence/security evidence exists for every claimed property. No automatic administrator fallback or incompatible-state merge. |

Steps 4 and 5 may inform each other, but both must pass before administrator-free
Target-Link operation is claimed. Naming is designed in steps 1-2 because its
finality can invalidate the chosen consensus; its runtime composition can follow
later. Existing C0 defect repair follows its own tracker order. This plan does
not interrupt it, expand its milestone, or assume that a missing in-progress
label proves no work is underway.

### Proposed fresh-participant journey

1. The owner obtains an independently verifiable implementation and explicitly
   identifies the intended network/rules. Initial trust is disclosed; no account
   with the project or invitation from its founder is a public prerequisite.
2. The Endpoint creates its own bounded local keys/state and obtains source
   candidates through the selected discovery mechanism. Every direct contact
   follows finite source ordering and the existing source/Route exposure rules.
3. It verifies current shared state and sufficient freshness under the selected
   bootstrap/finality model. No header, mirror recommendation, quorum count,
   or untrusted local clock alone is silently promoted to complete current truth.
4. Objective admission and available independently operated duties provide any
   required scoped credentials. Refusal/overload is explicit; having an admissible
   request does not force a particular operator to supply resources.
5. Service-owned Custody authorizes its Instance independently of network
   administration. The Endpoint publishes and connects by Target Link through
   the existing ownership seams; public Names become available only after their
   separate runtime gate.
6. Expiry, partition, source exhaustion, or uncertain time blocks the affected
   capability and drains finite work. Recovery uses the chosen evidence rules;
   it never asks a hidden central signer to make a permanent exception.

### Compatibility, shadow evaluation, and cutover

**Proposed default for qualification:** use an isolated successor identity and
separate durable roots. Its early shadow reader observes bounded public/synthetic
inputs but cannot authorize current C0 work, write current floors, change a live
Route, or leak production queries. Running old/new verification side by side
must have an explicit resource budget and lifetime; it is not a permanent second
stack or an OR rule accepting either authority.

C0 has no supported public canonical Namespace, so a clean public start is the
lowest-migration-burden candidate. Do not promise to preserve every current
Target Link or credential: first audit Network/profile binding. Service/Name
private ownership does not need arbitrary export; the owner may authorize fresh
public credentials where the selected contract permits. The alpha corpus is
never silently imported as canonical Name ownership.

If continuity is required later, specify one exact predecessor boundary, source
and successor identities, state digest, credential treatment, finality/freshness
proof, and owner consent. Retain old state and its non-decreasing floors as
read-only evidence. Old/new state cannot share a mutable root or reinterpret an
old authorization under new rules. Numeric floors in unrelated profiles are
not directly comparable; any translation needs its own explicit mapping.

Before irreversible successor actions, an interrupted local activation may
return to the separately authenticated old profile if its old safety policy
still permits it. After successor actions, there is no automatic state rollback,
replay into the predecessor, or merging of two registries. Failure yields an
explicit unavailable/fork state and a selected recovery path. Removing old key
material is a final operational step; proof that clients reject its authority
is the actual retirement condition.

### Verification and qualification matrix

| Attack or outage | Required evidence before promotion |
|---|---|
| Founders, enrolled project signers, official publishers, and the C0 time witness all removed | Fresh entry, restart, state progression, and the selected Service journey still work with the declared surviving population; indefinite cached operation is not counted. |
| Enough honest producers or required data disappear | Capabilities become explicitly unavailable within their bounds; no new finalized truth, fake freshness, or old-key rescue is invented. |
| Sybil splitting, resource reuse, false capacity, and rented influence | A separately justified admission/weight argument and resource budget; no per-key cap, uptime, or traffic receipt masquerades as independent control. |
| Majority sends an invalid owner/Name transition | Honest verifiers reject it even if all producer signatures/work proofs are otherwise valid. |
| Partition, competing branches, old authority, and rejoin | The selected finality/rollback contract is followed; no finalized owner resurrection or hidden conflict reset. |
| Isolated fresh/long-offline Endpoint | Bootstrap requirements and uncertainty are visible; invalid, withheld, or stale evidence cannot establish readiness. |
| Malicious update feed, incompatible version, unknown critical flaw | Authenticated artifacts and local activation survive; no forced install; no guarantee that an unknown flaw is repaired by deterministic consensus. |
| Interrupted migration or mixed old/new proofs | No root sharing, reused one-use credentials, floor erasure, profile substitution, or administrative fallback. |
| Full and partial readers at target scale | Source and verification cost, retained state, queues, cleanup, private submission/resolution, and completeness claims fit the selected bounds. |

Behavior tests, process tests, `make quick-check`, and `make check` accompany
future maintained implementation in their checked environments. Unsupported
platforms or absent prerequisites do not count as passing skips. Independent
security review and independently operated resource evidence are public-claim
gates, not staff assumed available to perform this plan. Simulated failures
can falsify behavior; they cannot establish real-world operator independence.

## Disposition

ADR-0074 accepts the public authority direction. This preparation now supplies
NET-19 through NET-28, nine product outcomes with hostile-environment conditions,
nine solution proposals with alternatives and implementation boundaries,
shared-state/bootstrap and Name-renewal comparisons, proposed ADR-0075 and
ADR-0076, budget consequences, 26 unexecuted conformance cases, falsifiable
validation designs, a reviewable first development brief, and the executed
state/resource/time/Name arithmetic comparison with retained rejection cases.

The first task-first assessment now identifies local-only checks, conditionally
mergeable facts and the exclusive/current common-choice obligations. It includes
finite operation counterexamples and five enumerated quorum configurations;
1133 subset evaluations are not network qualification. Candidate roles and
implementation limits are recorded without selecting a public algorithm.
Admission/replacement, real validity/currentness evidence, Name inclusion and
whole-role costs remain open. No common chain, mining economy, weight or proof
format is a starting requirement. Canonical Names and the common Candidate View
remain binding. Release policy retains its proposed outcome table; exact
successor acceptance/migration is still required before changing C0.

The temporary-committee assessment adds exact sampling and turnout arithmetic,
contradictory certificate/close examples, proposal-grinding exposure and
availability-filter/alternate counterexamples. It preserves unresolved outcomes
and selects neither human governance nor automatic agreement.

The current priority is coherent participant-selection and voting-core design,
with application scenarios attached later under explicit contracts. A whole-core
dependency/readiness gate precedes maintained development in parts. Its intended
deliverable is an ordered development and verification task set; that set is
not ready or activated merely because this research record describes it.

R-149 remains open. No production agreement algorithm, committee weight,
economic system, new wire format, live networking experiment, public launch or
C0 implementation is selected. Current runtime, accepted wire identities, package map, dependency
closure and C0 operator procedures remain intact.
