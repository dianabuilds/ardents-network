---
id: R-131
title: Headless Publisher authority and Instance acquisition
status: open
owner: Product Owner
started: 2026-08-30
reviewed: 2026-08-30
---

# R-131 — Headless Publisher authority and Instance acquisition

## Decision this unlocks

Select the participant-owned operation that supplies a fresh headless Publisher
with one Service Authority-derived Target, one host-local Instance Key, and one
bounded monotonic Service Credential. This decision is required before B6 can
honestly execute `enroll/acquire -> start -> publish -> open -> bytes ->
withdraw -> restart/recovery` from enrollment-v3 artifacts without a fixture or
operator Target.

## Current contract

The [headless product boundary](../../product/network-application-separation.md),
[J-03](../../product/journeys.md#j-03--publish-a-local-service),
[NET-03/NET-04B](../../product/functional-map.md), and
[ADR-0003](../../adr/0003-bounded-service-instance-credentials.md) already
separate Service Authority, Service Target, public Credential, and private
Instance Key. The local Service Administration surface may publish and withdraw
only an already authorized Credential with its matching Instance Key. Endpoint,
Browser, Route Nodes, Network State, and enrollment are not Service Authority.

R-128/R-130 and ADR-0062/ADR-0063 resolve membership Transit Grant acquisition;
they deliberately contain no Service, Publisher, Target, or publication
authority. The maintained `ardents` runtime can consume prepared Credential and
Instance-key files, but no current enrollment-v3 command creates or imports the
Service Authority and produces those inputs. Test-only credential generation
therefore cannot close B6.

## Hypotheses

- **H1:** A separate owner-only Authority Custody operation can create or import
  Service Authority, have the Publisher host create the Instance Key, issue one
  bounded monotonic Credential, and hand the runtime only the public Credential
  plus the non-exported host-local Instance binding.
- **H2:** A deliberately narrower usable-alpha enrollment operation can
  provision one pre-authorized Publisher generation without making enrollment
  a reusable Service Authority or adding Target/Authority fields to the runtime
  plan.
- **H0:** Neither operation preserves the accepted privilege lattice at the
  current team and platform scope; B6 must then remove Publisher creation from
  the candidate or change the product contract explicitly.

## Evaluation criteria

- A fresh enrolled Publisher has an explicit acquire, publish, withdraw,
  restart, routine Instance successor, and Authority recovery path.
- Service Authority never enters Endpoint, Node, Browser, release metadata,
  Network State, command arguments, environment, or ordinary runtime output.
- The Publisher runtime receives no caller-selected raw Target or Route fact;
  Target is derived from the authenticated Service Authority/Credential.
- The Instance Key is generated for the new host, is not silently exported for
  routine migration, and is erased from live publication state on withdrawal.
- Credential generation is monotonic and bounded, and copying only the public
  Credential grants no publication or impersonation power.
- The workflow is maintainable by the Product Owner and Codex without assuming
  an administrator, registrar, online custody service, or independent operator.
- Enrollment-v3 remains a release/control inventory and does not silently
  become Service Authority custody.

## Evidence plan

### Primary sources

The initial decision evidence is the accepted in-repository product and ADR
contract linked above plus the maintained custody, Endpoint publication, and
Service publication source. Any selected operating-system keystore or external
cryptographic dependency requires its own primary-source review and dependency
decision before implementation.

### Experiment

For each admissible option, execute the exact commands from a fresh unpacked
enrollment-v3 artifact and record: Authority creation/import ownership, host
Instance generation, Credential issue, publication, remote byte exchange,
withdrawal, restart, routine successor generation, and recovery refusal cases.
Inspect process arguments, environment, bundle inventory, runtime plan, and
durable roots for Authority/Target leakage.

### Failure scenarios

- copied public Credential without the Instance Key;
- copied Instance Key without Service Authority or a valid Credential;
- reused generation, rollback, expired Credential, or wrong Network;
- interrupted issuance or publication and retry after restart;
- lost runtime state with retained Authority, and lost Authority with retained
  runtime state;
- substituted enrollment bundle, runtime plan, Credential, Instance binding,
  or custody root;
- attempted Browser, Node, State, or release-control access to Service Authority.

## Findings

- **Measurement:** the current headless User composition owns State, Entry,
  private reachability, Transit Grant acquisition, Target authentication, and
  Connection lifecycle without a caller Route or Target field.
- **Measurement:** the current Publisher composition accepts prepared
  Credential and Instance-key files; the repository has no supported
  enrollment-v3 command that acquires them from Service Authority.
- **Sourced fact:** the accepted product contract makes Authority Custody,
  Service Administration, and Connection three non-collapsing privileges.
- **Inference:** generating credentials inside the B6 test would reproduce the
  historical fixture gap and cannot qualify a normal participant journey.
- **Inference:** embedding reusable Service Authority in enrollment would make
  release distribution an undeclared custody owner and violate NET-04B.

## Options

1. **Separate local Authority Custody command.** Extend the maintained custody
   boundary with one closed Service operation and a host-local Instance
   enrollment handshake. Best contract fit; requires an exact secret-input,
   durable-floor, backup/recovery, and platform-storage design.
2. **One-generation alpha provisioning.** A Product Owner operation creates a
   non-reusable Publisher enrollment for one Target/generation and the host
   creates the Instance Key. Smaller alpha workflow, but risks making cohort
   enrollment a hidden Service registrar and needs explicit recovery limits.
3. **User-only candidate.** Qualify only `open` and omit fresh Publisher
   creation. Smallest implementation, but conflicts with the currently selected
   usable-alpha proof and cannot close B6 without a product-scope change.

## Recommendation

Do not choose during remediation. Ask the Product Owner to select the Authority
owner and recovery promise first. Prefer evaluating option 1 because it matches
the accepted privilege lattice and J-03, but its concrete host-key binding and
recovery surface are consequential decisions. Confidence: high that a decision
is required; medium that option 1 is the smallest acceptable alpha operation.
The strongest argument against option 1 is that a complete custody workflow may
exceed the bounded usable-alpha scope unless one-generation issuance and
recovery are kept deliberately narrow.

## Disposition

R-131 is open and blocks only the fresh Publisher-authority portion of B6. It
does not reopen R-128/R-130, Route/Target/wire semantics, Browser separation,
or the artifact-native enrollment/start/restart work that can proceed
independently. No ADR or implementation is authorized until the Product Owner
accepts an option or changes the selected candidate scope.
