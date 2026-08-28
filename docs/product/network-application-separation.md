# H4 closure follow-up — deep audit and Network/Application separation

Status: **proposed next objective after the selected Horizon 4 implementation
is functionally complete; this document authorizes no current research or
implementation work.**

## Queue placement

This objective may take the next-work slot that would otherwise have gone to
H4-7, but it does not replace, rename, or weaken H4-7. H4-7 remains the
accepted, deferred protected-Application profile: no Application-level Endpoint
Location Privacy claim is selected for the current generic browser path.

The objective also activates the prepared
[Deep audit campaign](../development/deep-audit.md) against one immutable H4
candidate. The audit is a distinct read-only discovery and proof phase, not a
name for the subsequent architecture change. Findings that require a new
architecture leave audit discovery and receive their normal product, research,
and ADR treatment before implementation.

The new objective solves a different problem: Ardents Network must remain a
standalone headless product, while Desktop, browser, and future Application
products remain replaceable Adapters. Reusing the H4-7 identifier would merge a
source/product architecture decision with an OS-enforced privacy claim and
would break the existing H4-7 and R-099 decision trail.

This objective is also not Horizon 5. Horizon 5 remains the later security and
privacy model review defined by its own entry conditions.

## Objective

First investigate one immutable, functionally complete H4 candidate through the
prepared whole-codebase audit. Then make Ardents Network independently
buildable, runnable, testable, and releasable without Desktop or browser
artifacts:

- the local Endpoint and infrastructure Node own networking behavior;
- command-line programs remain thin Adapters over maintained Modules;
- Desktop and Browser Adapters call narrow local Interfaces and own only their
  presentation and interaction behavior; and
- no UI Adapter owns Network State, Route selection, Service identity, or
  network or Authority custody.

The final candidate must preserve the audit baseline, finding dispositions,
change-induced review, and affected requalification rather than treating a
successful refactor as proof that the baseline had no defect.

Repository co-location remains governed by ADR-0010. This plan separates
Modules, processes, artifacts, and dependency direction; it does not select a
multi-repository organization.

## Decision-relevant question

Before implementation begins, add a current research question using the
repository research template:

> Can the maintained Endpoint and Node form one complete headless Ardents
> product while browser and Desktop behavior is supplied by replaceable local
> Adapters, without duplicating network behavior, weakening the Application
> Interface, or creating an unsupportable release burden for the one-to-one
> team?

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

The audit and architecture work share one queue position but retain distinct
candidate identities and change rules:

```text
finish selected H4 functionality
  -> clean and reproduce one exact H4 candidate
  -> freeze immutable Audit Baseline
  -> run Deep Audit discovery, synthesis, and proofs read-only
  -> Product Owner disposition of findings
  -> select architecture work and accepted remediation explicitly
  -> implement Network/Application separation in scoped change waves
  -> review the complete baseline-to-candidate diff
  -> rerun every invalidated test and Qualification profile
  -> freeze and decide the final exact candidate
```

The audit therefore begins at the last H4 closure gate: after feature work is
complete, but before final promotion and immutable build freeze. This does not
interrupt the current H4 implementation halfway through. The current moving
working tree is never used as an Audit Baseline.

Audit discovery produces the claim map, invariant register, surface inventory,
coverage ledger, finding register, proof results, and qualification-impact
register required by the audit policy. The architecture plan consumes accepted
facts from those registers, but it does not rewrite audit history or implement
an unproved hypothesis as cleanup.

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
Application seams. An Endpoint Owner Interface is selected only after the
audit identifies one real Desktop caller alongside the CLI Adapter; no generic
control framework or placeholder package is created in advance.

## Phase A — immutable H4 deep audit

### A1. Activate and freeze

Complete the selected H4 functionality, integrate it into a clean reproducible
revision, pass its required checks, name its claims and non-claims, and record
the exact source, executable, toolchain, dependency, platform, topology, and
execution-profile identities required by the audit activation manifest.

**Done when:** the Product Owner selects one immutable Audit Baseline and the
campaign can distinguish it from every later candidate.

### A2. Discover, synthesize, and prove

Execute the complete A-F tracks, cross-track synthesis, and proof campaign from
the Deep audit policy. Discovery remains read-only. Every required surface and
coverage cell receives a supported verdict; Blocker and Major hypotheses
receive the required deterministic, environment, or Qualification proof, or
remain explicit release-blocking uncertainty.

**Done when:** the Product Owner receives one deduplicated finding and residual-
risk register with evidence levels, root causes, proof results, claim effects,
and required requalification.

### A3. Select dispositions

For each finding, choose repair, claim/profile reduction, research return,
bounded limitation, rejection with evidence, or release stop. An architecture-
changing finding is transferred into this product plan or another explicit
owner before code changes begin.

**Done when:** no discovery item can enter implementation under an ambiguous
`cleanup` or `refactoring` label.

## Phase B — Network/Application separation and remediation

The following slices implement the already selected product separation and any
accepted audit remediation that shares the same cohesive root cause. Unrelated
findings remain separate change waves.

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

## Phase C — change review, requalification, and freeze

### C1. Review the complete change

Review the complete Audit-Baseline-to-candidate diff independently from the
implementation pass. Reopen every affected invariant, authority, state,
lifecycle, failure, resource, persistence, release, and claim assumption.
Reject new fallback, duplicated behavior, exported authority, speculative
Interface, or test that merely restates the changed Implementation.

**Done when:** every architecture and audit change has an explicit evidence-
reuse or requalification decision, and every newly introduced surface has its
required audit coverage.

### C2. Requalify and freeze

Run the ordinary repository gates plus every platform, process, live, soak,
fault, recovery, overload, cleanup, and Qualification profile invalidated by
the changes. Preserve initial failures and exact environment identities. Fix
one final source revision and executable digest only after all required cells
have a terminal result.

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

After a qualified H4-5 Node duty exists, test one provider profile and one
short-lived entitlement for one permitted Node role, initially without billing.
The Endpoint owns the credential and provider choice; the Browser Adapter does
not. Exercise expiry, revocation, quota, outage, explicit provider switching,
and fail-closed behavior. A hosted remote Endpoint is a separate weaker product
and is not selected by this plan.

## Objective closure

The core objective closes after phases A-C when all of the following hold:

- the immutable H4 Audit Baseline and complete audit registers are retained in
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
