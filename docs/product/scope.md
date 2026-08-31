# Product scope and audit boundary

Status: **current C0 candidate contract.** This document names the maintained
product surfaces that may enter the next architecture and security audit. It is
not a release qualification, Public Beta claim, or authorization to add missing
product behavior during audit preparation.

Accepted ADRs, this product contract, and the threat model have priority over
research records, experiments, historical campaigns, and legacy code. Planning
labels and completed campaign names are provenance only; they are not runtime,
package, release, wire, or persisted identities. Existing accepted wire or
persisted identities that contain an earlier label remain compatibility
obligations until a separately researched migration retires them.

## Product core

Ardents connects a local Application to an opaque Service Target through an
authenticated Endpoint-selected Route. The network does not turn a Person,
Device, Persona, Node identity, transport identity, Service Target, Credential,
or Capability into another concept.

The maintained contract requires:

- authenticated current Network State and explicit unavailability for stale,
  conflicting, ambiguous, or missing authority;
- Endpoint-owned Route selection, finite attachments, no peer-selected
  fallback, and typed terminal outcomes;
- distinct Connection, Service Administration, Authority Custody, Release,
  Network State, and local Application privileges;
- a host-generated non-exporting Service Instance key and a separately issued
  public Credential for publication;
- explicit Target Links and separately authenticated name resolution, with no
  ordinary DNS, Internet, same-looking name, or alternate-Target fallback;
- finite budgets, bounded work lifetimes, joined cleanup, explicit overload,
  and durable floors where rollback would change authority or safety; and
- honest privacy claims: payload encryption is not anonymity, multiple Nodes
  are not independent operators, and a generic Application Adapter is not an
  isolation boundary.

The detailed product requirements remain in the
[functional map](functional-map.md), participant behavior in
[journeys](journeys.md), and lifecycle rules in the
[operating model](operating-model.md).

## C0 Network candidate

The Network audit candidate is the headless maintained product surface:

- `ardents`, `ardents-node`, `ardents-control`, `ardents-custody`,
  `ardents-release-custody`, and `ardents-state-custody`;
- Network State and Source, Entry, Route and Carrier, Node duties, Endpoint,
  Service publication/connection/instance/reachability, naming, enrollment,
  Release, Custody, contributor, control-inspection, and resource Modules;
- the `internal/application/connection` and
  `internal/application/administration` Interfaces used by the headless CLI;
- the enrollment-v3 headless artifact lane and the maintained deterministic,
  process, race, fuzz, architecture, and purpose-named qualification profiles.

Endpoint composes authenticated State, Entry, Route, Service, and local
Application boundaries. It does not own Browser presentation, Browser Entry,
Firefox, local Application wire clients, Release authority, Network State
authority, or Authority Custody. A Publisher attachment is available only when
authenticated State projects exactly one current Introduction, Rendezvous, and
Responder; Endpoint then acquires the separate Introduction and Responder
credentials without caller-supplied Route, peer, role, Grant, or key material.

The C0 Network candidate is ready to be *audited*, not qualified for public
operation. The audit must use the exact frozen commit and artifact identities
defined at activation by the [deep-audit method](../development/deep-audit.md).

## C0 Application and Browser candidate

Application and Browser code is a separate maintained product surface in the
same repository and root Go module:

- `internal/application/broker`, `connection`, and `administration` own local
  capability admission and the shared local Interfaces;
- `ardents-browser`, `ardents-browser-entry`, `internal/browseradapter`,
  `internal/browserentry`, and `internal/browserreference` own optional Browser
  adaptation and presentation;
- the enrollment-v4 Browser artifact lane is disjoint from Network-v3; and
- Browser commands may depend on Application and Browser-owned Modules but not
  on Endpoint, Network State, Node, Route, Entry, Service, or Custody
  implementations.

This surface may be audited beside the Network candidate for boundary and
dependency correctness. It does not inherit a Network security claim, browser
isolation claim, DNS/DoH protection, Web PKI identity, general proxy authority,
or supported Firefox participant journey. The unresolved transparent-origin
defect remains outside this preparation goal.

## Excluded historical evidence

The following are not maintained candidate surfaces:

- completed experiment implementations and their runners;
- the former `reference-c2` executable fixture and its stage-specific
  qualification harnesses;
- retired release-assembly and project-control simulation commands;
- superseded planning briefs, status chronology, and split-candidate release
  ledgers; and
- Firefox/Endpoint source retained as non-executable evidence under
  `tests/compatibility/browser-endpoint-v4` by ADR-0061.

Git history and accepted research/ADR records preserve the first four items'
provenance. The ADR-0061 compatibility tree is intentionally retained but is
excluded from builds, package inventories, current qualification, and the C0
Network candidate.

## Claims withheld from C0

C0 preparation and an internal audit do not establish:

- anonymity, censorship resistance, availability, traffic-correlation
  resistance, or a Shielded Route Profile;
- independent operators, builders, auditors, control, or public governance;
- supported public deployment, capacity, hostile-load, soak, VPS, or platform
  qualification;
- a public permissionless Namespace, participant Browser Entry, signed release,
  or Public Beta;
- Application-level network isolation for a generic Adapter; or
- a complete artifact-native B6 participant journey.

Any later claim states the protected information, adversary, conditions,
measurement, and limitation, and requires its own exact candidate and evidence.

## Outside the network core

These remain Applications, optional Overlay Services, or separate future
products:

- messenger, Inbox, Contacts, social graph, presence, calls, notifications,
  offline delivery, retained history, content persistence, and replication;
- universal identity, proof of personhood, wallets, tokens, payment, staking,
  and incentive markets;
- public Service search, directories, recommendations, and discovery feeds;
- clearnet exit, VPN behavior, or general anonymous Internet access;
- multi-instance Service availability, origin replication, and automatic
  Application failover; and
- bundled browsers, arbitrary remote Application runtimes, decentralized
  compute, content-safety engines, mobile clients, and gateways to other
  networks.

An excluded item returns only through an explicit Product Owner decision,
decision-relevant research, a bounded threat/resource contract, and an
accepted ADR where it creates consequential lock-in.

## Scope-change rule

1. Only an explicitly selected bounded goal enters implementation; a roadmap
   label, audit finding, or future claim does not authorize product expansion.
2. Security is not silently deferred: the required condition is implemented
   and tested, or the corresponding claim remains withheld.
3. Audit findings that require a new protocol, dependency, architecture,
   authority, product claim, or threat boundary return to research and Product
   Owner decision instead of being implemented as cleanup.
4. Historical evidence never qualifies a changed candidate and never becomes a
   compatibility obligation unless an accepted current contract says so.
5. Go remains the maintained foundation under ADR-0009; the selected bounded
   Route, wire, and Carrier contracts do not imply a permanent public protocol.

## Current owners

| Subject | Current owner |
|---|---|
| Domain language | [CONTEXT.md](../../CONTEXT.md) |
| Product requirements and journeys | [functional map](functional-map.md), [journeys](journeys.md), [operating model](operating-model.md) |
| Security and privacy claims | [threat model](../security/threat-model.md) |
| Network/Route/Node | [Network Route and Node](../technical/network-route-node.md) |
| Endpoint/Service/Application seam | [Endpoint and Service runtime](../technical/endpoint-service-runtime.md) |
| Naming and private resolution | [Naming](../technical/naming.md), [Private reachability](../technical/private-reachability.md) |
| Enrollment and artifacts | [Enrollment verification](../technical/enrollment-verification.md) |
| Release, replacement, and custody | [Release and update custody](../technical/release-update-custody.md) |
| Commands, packages, and checks | [command reference](../reference/commands.md), [package map](../development/package-map.md), [testing](../development/testing.md) |
| Audit method | [Deep audit campaign](../development/deep-audit.md) |
