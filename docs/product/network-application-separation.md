# Product-candidate remediation — headless Network and Application separation

Status: **selected active objective. Preliminary state discovery is complete;
truth remediation and Network/Application separation precede formal C0, deep
audit, and expensive qualification.**

## Queue placement

The Product Owner selected this objective before H4-7. It does not replace,
rename, or weaken H4-7: that identifier remains planning provenance for the
deferred protected-Application profile. No Application-level Endpoint Location
Privacy claim is selected for the generic browser path.

A preliminary read-only state review of fixed point
`0292afbd572c09b55ee5cf2589ac44af99edfa66` has already identified truth,
product-journey, ownership, and separation blockers. Its durable state and
finding-disposition map are retained in
[R-129](../research/records/r-129-headless-network-product-boundary.md). The prepared
[Deep audit campaign](../development/deep-audit.md) is activated formally only
after those known issues are dispositioned and one stable candidate is frozen.
Findings that require a new architecture still receive their normal product,
research, and ADR treatment rather than entering as generic cleanup.

The new objective solves a different problem: Ardents Network must remain a
standalone headless product, while Desktop, browser, and future Application
products remain replaceable Adapters. Reusing the H4-7 identifier would merge a
source/product architecture decision with an OS-enforced privacy claim and
would break the existing H4-7 and R-099 decision trail.

This objective is also not Horizon 5. Horizon 5 remains the later security and
privacy model review defined by its own entry conditions.

## Objective

Make Ardents Network independently buildable, runnable, testable, and releasable
without Desktop or browser artifacts, complete the supported headless product
journey, and remediate accepted preliminary findings before freezing the formal
audit candidate:

- the local Endpoint and infrastructure Node own networking behavior;
- command-line programs remain thin Adapters over maintained Modules;
- Desktop and Browser Adapters call narrow local Interfaces and own only their
  presentation and interaction behavior; and
- no UI Adapter owns Network State, Route selection, Service identity, or
  network or Authority custody.

The final candidate must preserve the preliminary baseline and finding
dispositions, receive independent change review, and then enter formal audit
and affected qualification. A successful refactor is not proof that the
baseline or final candidate had no defect.

Repository co-location remains governed by ADR-0010. This plan separates
Modules, processes, artifacts, and dependency direction; it does not select a
multi-repository organization.

## Accepted participant-acquisition dependency

The selected direction was tested by
[R-128](../research/records/r-128-headless-participant-acquisition.md). The
Product Owner accepted ADR-0062's purpose-scoped signer, durable issuer
budget/idempotency, fixed encrypted outcomes, and Endpoint-owned at-most-once
lifecycle. Participant acquisition may proceed only through that boundary:

> The enrolled Endpoint composes authenticated State and Entry, reconciles one
> exact Request ID through the State-selected purpose-scoped issuer, and keeps
> the resulting one-use transport input below the local Application Interface.

The proposed direction is falsified or must be narrowed if any of these is
required:

- Network behavior must be duplicated inside an Adapter;
- the headless Endpoint cannot complete its supported journey without XPI,
  browser, or Desktop assets;
- an Adapter must receive Route, Network State, Service Authority, or custody
  authority to perform its product job;
- UI replacement requires a network protocol or Service identity change; or
- independent artifacts create more compatibility and release work than the
  actual team can maintain.

## Current baseline

The lower product Modules are already pointed in the intended direction:

- `internal/network`, `internal/node`, `internal/route`, `internal/service`, and
  `internal/entry` do not depend on Endpoint or browser implementations;
- `cmd/ardents-node` is a separate headless Node Adapter;
- the Application Interface already separates Connection and Service
  Administration authority; and
- `internal/browserentry` and `cmd/ardents-browser-entry` already expose a
  bounded Firefox native-host handoff rather than network behavior.

The main coupling to remove is concentrated above those Modules:

- `internal/endpoint` owns Firefox-specific launch, Browser Entry state,
  loopback HTTP presentation, and alpha browser proxy lifecycle;
- `cmd/ardents` composes Network State, Entry, naming, and the browser runtime
  in one command Adapter;
- Endpoint configuration names the current Firefox-specific profile; and
- enrollment, release inventory, and end-to-end qualification currently couple
  the headless Endpoint artifact to Browser Entry host and XPI artifacts.

This is a focused extraction and Interface-deepening task, not a network
redesign.

## Integrated execution sequence

Product remediation and the later formal audit share one objective but retain
distinct candidate identities and change rules:

```text
preliminary read-only state discovery
  -> reconcile product/status/RC/custody truth
  -> select finding dispositions and bounded implementation waves
  -> complete the supported headless journey
  -> implement Network/Application separation and accepted remediation
  -> run ordinary gates and review the complete stabilization diff
  -> freeze one exact candidate at C0
  -> run formal Deep Audit A-F discovery, synthesis, and proofs read-only
  -> disposition and repair any newly proved findings in successor candidates
  -> rerun only invalidated evidence plus the final selected qualification set
  -> freeze and decide the final exact candidate
```

Formal audit therefore begins only after product completion and known
remediation, but before final promotion. The current moving working tree is a
remediation baseline, never an immutable Audit Baseline.

Preliminary discovery retains its state map and dispositions as provenance.
Formal audit independently produces the complete claim map, invariant register,
surface inventory, coverage ledger, finding register, proof results, and
qualification-impact register required by policy; the preliminary pass cannot
stand in for that coverage.

## Target dependency direction

```text
Desktop Adapter -----------> Endpoint Owner seam ----┐
                                                     |
Browser Adapter -----------> Application Interface --+--> local Endpoint
                                                     |         |
other Applications --------> Application Interface --┘         v
                                                        remote Nodes

CLI Adapters --------------> owning Module Interfaces
```

Dependencies do not point back from Network Modules into Desktop, Browser, or
presentation implementations. Closing, removing, or upgrading a UI Adapter
does not stop or replace the Endpoint.

The existing Connection and Service Administration Interfaces remain the
Application seams. An Endpoint Owner Interface is selected only when one real
Desktop caller exists alongside the CLI Adapter; no generic control framework
or placeholder package is created in advance.

## Phase A — preliminary state discovery and dispositions

### A1. Inventory the current state

Inspect one clean source baseline read-only for product-contract gaps, status
contradictions, orphaned packages, authority/RC ambiguity, unsafe coupling, and
obvious security findings. Retain raw evidence outside the repository. Do not
claim complete A-F coverage or candidate qualification.

**Done when:** the state report distinguishes verified facts, hypotheses,
known defects, and work that remains for formal audit. This preliminary pass is
complete for the pre-remediation baseline.

### A2. Reconcile product and evidence truth

Correct current product scope, research statuses, RC/evidence attribution,
candidate claims, and the audit/qualification sequence. Retire any exposed
local release seed before a future real release and record infrastructure-access
cleanup as a separate exact operational action.

**Done when:** no current document claims a nonexistent product journey,
qualified candidate, or evidence transfer between different executable bytes.

### A3. Select bounded implementation waves

For each preliminary finding, choose repair, product completion, retirement,
versioned migration, claim reduction, research return, bounded limitation,
rejection with evidence, or deferral. Architecture and persisted/wire identity
changes receive explicit owners rather than bulk cleanup.

**Done when:** every selected change belongs to a cohesive wave with behavior
tests and an evidence-impact statement.

## Phase B — Network/Application separation and remediation

The following slices implement the selected product separation and accepted
preliminary remediation that shares the same cohesive root cause. Unrelated
findings remain separate change waves.

The implementation waves have one dependency order even though they close one
product objective:

| Wave | Depends on | Coherent result |
|---|---|---|
| B1 scoped issuance | ADR-0062 and current State/Entry/Grant contracts | Purpose-scoped signer profile, durable finite budget/idempotency, and fixed encrypted outcomes without State root custody. |
| B2 Endpoint acquisition | B1 | Durable pending/reconcile/present/burn lifecycle consuming current State, Entry, and one-use transport input. |
| B3 Application Interface | B2 and existing Connection/Service owners | Narrow separately authorized Connection and Service Administration surfaces used by CLI and Browser Adapter. |
| B4 Browser extraction | B3 | HTTP/Firefox/XPI/native-host presentation outside Endpoint, with the known transparent-origin defect retained as a separate dependency. |
| B5 dependency and artifacts | B4 | Browser-free transitive command graph, separate enrollment-v3/v4 inventories and lanes, and real named packaged binaries. |
| B6 headless qualification | B5 | Unpacked artifact-native enroll/acquire/start/publish/open/bytes/withdraw/restart-recovery journey without Browser, fixtures, or operator Route facts. |

Implementation status on 2026-08-30: B1 through B5 are implemented and
covered by ordinary repository gates. ADR-0063's owner-only issuer root and
exact State-bound Initiator ingress now have supported `ardents-node issuer
initialize|serve` ownership. The unpacked B6 journey remains to be completed
and cannot substitute a fixture or raw key/profile input. The
transparent-origin Browser Entry defect also remains recorded as a
separate future security slice and was deliberately not changed here.

Each wave is independently testable and commit-scoped. Later waves may move
code exposed by an earlier one, but they may not redefine its authority,
durability, Route, Target, or wire contract implicitly.

### 1. Freeze the product and authority map

Record the responsibilities, owned state, authority, lifecycle, and permitted
dependency direction for Endpoint, Node, CLI, Desktop, and Browser Adapter.
State explicitly that Endpoint is not a Node and that a Browser Adapter cannot
connect directly to an arbitrary remote Node.

**Done when:** every product surface has one behavior owner; allowed and
forbidden dependencies are reviewable; H4-7 is recorded as deferred rather
than silently claimed or replaced.

### 2. Audit the real seams

Inventory current commands, packages, imports, browser/XPI/native-host assets,
configuration fields, release inventory, and qualification profiles. For each
coupling, name the owning Module, caller, behavior to move, and verification
that must survive. Do not create a new package or Interface until a real second
Adapter or independent reason to change exists.

**Done when:** the proposed package-map delta and artifact split are known
before source movement begins, and every change traces to one concrete coupling
from the maintained tree.

### 3. Establish the headless Network baseline

Retain Endpoint, Node, Service Connection, and Application Interface behavior
behind their owning Modules. Keep CLI programs as thin Adapters. Add dependency
checks and a headless end-to-end profile that contains no Desktop, Firefox,
native-host, or XPI requirement.

**Done when:** the supported Client-to-Publisher journey builds and runs from
headless artifacts alone; removing optional UI artifacts does not break the
Network build or test lane; a reverse import from a Network Module to a UI
Adapter fails the architecture gate.

### 4. Make Browser Companion a replaceable Adapter

Move browser presentation, Firefox launch/registration, native-host state, and
XPI lifecycle to the browser-owned side of the Application Interface. Keep
Target authentication, Service Connection, Network State, Entry, and Route
behavior in the Endpoint. Preserve explicit refusal of ordinary URLs, arbitrary
Targets, Endpoint administration, and direct-network fallback.

**Done when:** Browser Companion can be installed, removed, restarted, and
upgraded without stopping the Endpoint; the browser journey uses the same
Application Interface as another maintained caller; the Browser Adapter owns no
network authority and ordinary Internet traffic is not captured by Ardents.

### 5. Separate artifacts and qualification

Give the headless Network and optional Browser Companion independently
verifiable build and qualification lanes while preserving one monorepository
and one root Go module. Select version compatibility and release composition
only from the evidence of the real artifacts; record an ADR only if the chosen
Interface or release lifecycle creates consequential lock-in.

**Done when:** Network release evidence does not require a browser artifact;
Browser evidence identifies the exact compatible Network artifact and fails
explicitly on incompatibility; neither lane can make the other's product or
privacy claim.

## Phase C — stabilization review, C0, formal audit, and qualification

### C1. Review the complete change

Review the complete remediation-baseline-to-candidate diff independently from the
implementation pass. Reopen every affected invariant, authority, state,
lifecycle, failure, resource, persistence, release, and claim assumption.
Reject new fallback, duplicated behavior, exported authority, speculative
Interface, or test that merely restates the changed Implementation.

**Done when:** every architecture and remediation change has an explicit
evidence-reuse or requalification decision, and every newly introduced surface
is inventoried for formal audit.

Current disposition: R-130 is accepted and its owner-root bootstrap plus
State-bound issuer lifecycle are implemented. B6 and C0 remain open on R-131:
the product has no accepted publisher-side owner and command for acquiring or
importing the Service Authority, host-local Instance Key, and bounded Service
Credential required by an artifact-native `publish -> open -> bytes ->
withdraw -> restart/recovery` journey.

### C2. Freeze and activate formal C0

Pass the ordinary repository gates, select the exact source and executable
identity, declare platforms, topology, workload, claims/non-claims, external
prerequisites, and immutable read-only scope, and obtain Product Owner C0
activation under the Deep audit policy.

**Done when:** one clean reproducible candidate is distinguishable from every
development branch and known preliminary findings all have terminal
dispositions.

### C3. Run formal A-F audit and disposition findings

Execute complete architecture/invariant, security, concurrency/lifecycle,
network/wire, quality, and test-adequacy tracks read-only against the exact C0
candidate. Synthesize root causes, prove material hypotheses, and select repair,
claim reduction, bounded limitation, rejection, or release stop. Any repair
creates a successor candidate and receives independent diff review.

**Done when:** every selected claim and maintained surface has a supported
coverage verdict, every material finding has proof and disposition, and
residual uncertainty is explicit.

### C4. Requalify and freeze the final candidate

Run the ordinary repository gates plus every platform, process, live, soak,
fault, recovery, overload, cleanup, and Qualification profile invalidated by
the remediation and formal-audit findings. Preserve initial failures and exact
environment identities. Fix one final source revision and executable digest
only after all required cells have a terminal result.

**Done when:** no unresolved Blocker or Major remains in the accepted
candidate, residual risks are explicit, and the Product Owner can promote,
narrow, or reject one exact final candidate.

## Follow-up slices, not closure requirements

### Desktop Adapter pilot

Select one exact Endpoint Owner job and one operating system first. The pilot
may cover status, start/stop, enrollment, and access-profile management through
the selected local seam. It must remain reproducible through CLI and must not
become a second Endpoint. Windows and Ubuntu are separate qualification slices.

### Managed Network Access experiment

After a supported qualified Contributor Node duty exists, test one provider profile and one
short-lived entitlement for one permitted Node role, initially without billing.
The Endpoint owns the credential and provider choice; the Browser Adapter does
not. Exercise expiry, revocation, quota, outage, explicit provider switching,
and fail-closed behavior. A hosted remote Endpoint is a separate weaker product
and is not selected by this plan.

## Objective closure

The core objective closes after phases A-C when all of the following hold:

- the immutable formal Audit Baseline and complete audit registers are retained in
  their permitted evidence locations;
- every material audit hypothesis has proof, disposition, and qualification
  impact, and no unresolved Blocker or Major remains accepted;
- Ardents Network is usable and qualified headlessly through CLI;
- Endpoint and Node implementations have no Desktop or Browser dependency;
- Browser behavior is a replaceable Adapter at the Application Interface;
- UI failure or removal cannot cause clearnet, provider, or weaker-profile
  fallback;
- package-map and executable architecture checks enforce dependency direction;
  and
- product and security documents state which exact final artifact earns which
  claim and which audit or external-review claims it does not earn.

## Non-goals

- Implementing H4-7 protected Application isolation.
- Auditing a moving working tree or editing the Audit Baseline during discovery.
- Treating the internal campaign as independent external security review.
- Treating Network/Application separation as proof that unrelated audit
  findings were repaired.
- Building both Windows and Ubuntu Desktop products in one slice.
- Real billing, token, staking, or an incentive market.
- A generic provider or UI plugin framework.
- Moving first-party code into multiple repositories.
- Changing Route, Service Target, Service Connection, or public wire semantics
  merely to reorganize product presentation.
