# Product scope and audit boundary

## Selected release Carrier scope

[ADR-0136](../adr/0136-return-quic-from-patched-official-source.md) returned QUIC
to the Carrier set of the nearest closed release on 2026-11-28, superseding
[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) for the Carrier set.
The return is delivered in stages. Until the Carrier selection admits QUIC
(#1081) and the both-Carrier qualification is accepted (#1082), QUIC stays
deferred, with explicit pre-effect refusal and no transport fallback. Historical
both-Carrier evidence qualifies only its original candidate. All other behavior,
protection, installed journey and privacy limits remain binding on both Carriers;
selection is not implementation proof.

[ADR-0119](../adr/0119-bind-protected-endpoint-generation-to-release.md)
selects one authenticated protected Endpoint distribution and one independently
delivered initial pin. Executable and static document worker generation are separate
targets in the same signed Release set; installation-specific plans and mutable
roots remain separate. Selection alone does not establish implementation
or qualification of the complete installed journey. Product acceptance still
requires two separate installed Endpoints, the selected TCP/TLS Carrier and honest
restart/refusal outcomes.

## Software delivery and infrastructure deployment

[ADR-0125](../adr/0125-separate-software-delivery-and-infrastructure-deployment.md)
removes Installation as a standalone product domain. Application delivery and
update use platform delivery mechanisms; infrastructure deployment is a separate
operator concern. Enrollment and Release retain exact artifact acceptance and
trust floors. Runtime owners validate their configuration, admit and complete
their own work, and retain their durable data. Ardents does not introduce a
general installer or a separate lifecycle manager.

The selected Ubuntu delivery uses the host service manager to start and
stop the protected Endpoint and fixed infrastructure roles. Endpoint
delivery and infrastructure deployment retain exact artifact and configuration
binding, predecessor completion, joined cleanup, and existing data and floors.
The [technical ownership map](../development/domain-map.md#software-delivery-and-infrastructure-deployment)
and [command reference](../reference/commands.md) describe the current owners
and entry points.

Windows, Ubuntu and macOS are mandatory product targets. The product contract
and every behavior that can be implemented portably are shared across them.
Platform-specific code is reserved for required host operations or guarantees
that cannot be supplied portably. The C0 qualification profile below covers
the selected Ubuntu LTS `x86-64` candidate.
[ADR-0129](../adr/0129-require-platform-portability.md) owns the portability decision.

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
For the exact closed Service identities inventoried in
[R-168](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/research/records/r-168-service-v3-format-reset.md),
[ADR-0117](../adr/0117-reset-closed-service-formats-to-v3.md) selects a
fresh-root v3 replacement instead: there are no supported old Service clients,
and old formats receive only refusal without conversion or automatic deletion.
This selection does not change unrelated Route, State, or Application identities
and is not evidence that the v3 replacement has been integrated or qualified.

## New-installation transition

[ADR-0124](../adr/0124-start-new-installations-without-legacy-data-transfer.md)
selects independent new installations: no predecessor deployment, key or issued
right must continue across the owner transition. Cross-owner history conversion
and key import are not required. Provision new purpose-specific material and
independently admitted trust; a matching schema or empty directory is not
migration authority. This narrows predecessor deployment compatibility, not the
accepted public grammar or the new installation's own durable-state contract.
New-installation restart, replacement and recovery must preserve its Admission
spend history, exact retries, keys and authority/freshness floors. Old code retirement follows
its actual remaining consumers; it does not automatically delete stored files.

## Product core

Ardents connects a local Application to an opaque Service Target through an
authenticated Endpoint-selected Route. The product target is protected
exchange of arbitrary Application bytes. Messages, images, audio, documents
and files are Application uses of that Connection. The network carries
ordered bytes and the Application owns their meaning and presentation. The network does not turn a Person,
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
- explicit Target Links for the selected journey, with no ordinary DNS,
  Internet, same-looking name, or alternate-Target fallback; future authenticated
  Service Name access remains a separate unselected model;
- finite budgets, bounded work lifetimes, joined cleanup, explicit overload,
  and durable floors where rollback would change authority or safety; and
- honest privacy claims: payload encryption is not anonymity, multiple Nodes
  are not independent operators, and a generic Application Adapter is not an
  isolation boundary.

The [functional map](functional-map.md), [journeys](journeys.md), and
[operating model](operating-model.md) retain the broader public-product
requirements, behavior, and lifecycle. Read their relevant sections when a C0
surface depends on them; they are not an additional C0 checklist. The profile
below and the linked current technical owners describe the maintained subset.

The [public autonomy target](operating-model.md#public-autonomy-target) under
[ADR-0074](../adr/0074-target-non-administrative-public-operation.md) replaces
permanent appointed administration as the intended public end state. It does
not expand C0 or retire its current authentication and safety checks; each
successor mechanism and migration requires its own accepted contract and gate.

The [common protection baseline](operating-model.md#common-protection-baseline)
under [ADR-0077](../adr/0077-evolve-one-common-protection-baseline.md) makes
security and privacy properties of ordinary system use. Its
[product and technical workstream](../development/documentation.md#system-protection-workstream)
covers review, design, implementation and trials of the changed system in a
declared test environment. This selects a development direction, not an
additional C0 implementation slice or a stronger claim for existing bytes.
[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md) selects the
[protection architecture](../technical/common-privacy-architecture.md).
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) sets the
[closed Service workload](protected-service-workload.md) with confidential
control and admission, Target Links and confined Ubuntu workers.
[ADR-0132](../adr/0132-admit-protected-bidirectional-application-streams.md)
is superseded by [ADR-0134](../adr/0134-use-one-application-byte-stream.md):
all files use one bidirectional Application byte stream.

## C0 Closed Alpha readiness profile

This is the selected narrow delivery profile for the current C0 candidate. It
is a readiness target, not evidence that the candidate is qualified, released,
publicly supported, anonymous, independently operated, or censorship
resistant.

One person may operate the two Endpoints using an authenticated, manually
installed Ubuntu LTS `x86-64` artifact against project-controlled
infrastructure to publish one active Service Instance and connect through an
explicit Target Link. That describes Endpoint staffing, not the number of
infrastructure Operator Families. A complete Route still requires the distinct
Direct-Origin Source families and non-overlapping Role Domain assignments of
the accepted Network contract. Infrastructure under one known effective
controller is one family regardless of machine count or declared labels; if
it cannot supply those assignments, the full installed journey is unavailable.
Same-operator artifact and host diagnostics do not qualify that journey.
Participants use one command, `ardents`. The C0 document application uses
`ardents publish`, `ardents link` and `ardents read`. The Service Connection
carries opaque Application bytes. The protected Ubuntu Endpoint candidate
implements the bidirectional profile selected by
[ADR-0132](../adr/0132-admit-protected-bidirectional-application-streams.md).
Participants use `ardents stream publish`, `ardents link` and
`ardents stream connect` to exchange arbitrary data. Installed qualification
exercises byte-exact delivery in both directions through the public CLI.

The participant installs the v6 Endpoint distribution selected by
[ADR-0131](../adr/0131-distribute-authority-custody-separately.md).
Its protected Linux variant supplies the Application worker and systemd
resources. Systemd starts the worker on Endpoint's request. The offline
Service or Admission Authority operator obtains and verifies the separate
`ardents-custody` artifact before running any Vault ceremony.

Infrastructure operators use `ardents infrastructure provision` for offline
Network inputs and `ardents infrastructure sign-release` for fixed Release
plans and signatures. Operator instructions must describe the command order
without requiring edits to JSON, test-fixture keys, package knowledge, or ADR
lookup.

The readiness journey starts with independent artifact verification, supported
local state setup, and startup of the selected components. For each workload,
one installed Endpoint publishes a Service Instance and presents a Target Link.
A second installed Endpoint uses that Link to read the document or exchange
byte-exact data in both directions. The byte-stream profile requires explicit
refusal of incompatible profiles and authenticated terminal completion.
The readiness gate also covers stop and restart with required durable state
preserved and explicit authenticated replacement. Automatic update is outside
this profile. See the [workload acceptance contract](protected-service-workload.md#acceptance-and-limits).

Windows, Ubuntu and macOS are required product targets. The installed C0
qualification profile is Ubuntu LTS `x86-64`. Component portability evidence
has the scope of the platform and mechanism exercised. Browser and GUI
delivery, mobile, public or permissionless operation, independently audited
operator control, canonical Service Names, Namespace governance, additional
Carriers and high-load operation remain outside the selected profile. Distinct
known families establish only eligibility under the accepted assignment rules.
The [claim limits](#claims-withheld-from-c0) govern what can be inferred from
closed evidence.

Current C0 delivery status and work sequencing belong in GitHub Issues under
the [`C0 Closed Alpha` milestone](https://github.com/dianabuilds/ardents-network/milestone/1),
not in this contract or research records. The product contract remains stable;
the tracker is the one live task ledger.

### C0 command roles, state, and outcomes

The `ardents` product CLI owns Endpoint enrollment, runtime, document
publication and reading, and public verification. Its
`source` and `node` subcommands run the selected infrastructure roles.
The offline infrastructure entrypoints prepare and sign their respective
State and Release artifacts. `ardents-custody` creates the Service Authority
and issues public credentials while retaining private Authority material.
Their durable inputs remain separate: immutable artifact and enrollment inputs,
project Node/issuer roots, Authority Vault, Service Instance root and public
request/response, Endpoint State, Entry, Application Stock and Publication roots, and local
Application/Administration sockets. No role may borrow another role's root or
private material.

The [command reference](../reference/commands.md) maps each role to its delivery
and first command. The v6 participant archive contains the Endpoint program and
its applicable signed resources. Custody is distributed to the offline Authority
operator as a separate authenticated executable under ADR-0131. The installed
Application worker is invoked by the selected host service manager. Qualification
workers are test executables.

The C0 operator route must expose stable, actionable failures for a rejected
artifact or enrollment input; stale, conflicting, revoked, expired,
incompatible, or unavailable current State; rejected or conflicting Service
Credential response; unauthorized or unavailable Route; exhausted local
resource; cancellation or timeout; and closed Service Connection. A lower
level diagnostic may add detail but cannot silently reinterpret one of these
classes as success or fallback.

Endpoint delivery uses the protected platform unit. The read-only
`ardents enrollment verify` operation checks the manifest against an
independent pin before parsing enrollment facts. Enrollment grants no
Resolution or Route authority. The
[Enrollment owner](../technical/enrollment-verification.md) defines the retained
bundle grammar and the [Endpoint owner](../technical/endpoint-service-runtime.md#v1-startup-retirement)
defines the pre-effect refusal of persisted v1 plans. V2 remains the sole
accepted headless startup schema.

The closed CLI accepts explicit Target Links. Retired Alpha links, corpus
intake, old Node and Source starts, and the former generic AAI2 Connection
remain outside the accepted surfaces. Existing retained data follows its
owner's compatibility rules. The exact
[Alpha destination boundary](../technical/endpoint-service-runtime.md#alpha-destination-retirement),
[generic Connection boundary](../technical/endpoint-service-runtime.md#generic-connection-command-retirement),
and [retired-start boundary](../technical/network-route-node.md#old-start-retirement)
belong to their technical owners. Future protected Service Names require a
separate accepted contract covering authority, resolution, governance and
Application integration.

## C0 Network candidate

The Network audit candidate is the headless maintained product surface:

- the participant and infrastructure command `ardents`, the offline
  `ardents-custody` command and the signed systemd Application worker for the
  file and interactive byte-stream operations;
- Network State and Source, Admission, Hosting, Entry, Route and Carrier,
  Node duties, Endpoint, Service Publication, Connection, Instance and
  Reachability, Enrollment, Release, Custody and process resource protection;
- the Local Grant and session lifecycle owned by `internal/domain/execution`;
- the typed local Connection Interface in `internal/application/endpointstream`
  and separately authorized Administration Interface in
  `internal/application/administration`;
- the enrollment-v6 headless artifact lane and the maintained deterministic,
  process, race, and fuzz profiles, the architecture gate, and purpose-named
  qualification profiles.

Endpoint composes authenticated State, Entry, Route, Service, and local
Application boundaries. It does not own Browser presentation, Browser Entry,
Firefox, local Application wire clients, Release authority, Network State
authority, or Authority Custody. A Publisher path requires State's current
accepted closed Route profile joined to the same authenticated Epoch and exact
recipient Node Records.
Missing, conflicting, expired, or mismatched State makes that path unavailable;
the Application cannot choose Route peers or State identities.

These surfaces define the permitted audit boundary, not a claim of complete
domain acceptance or installed readiness. Audit activation freezes the exact
source/artifact identities and admitted scope under the
[deep-audit method](../development/deep-audit.md); live acceptance status belongs
to the selected GitHub ledger. An audit does not qualify public operation.

## C0 Application Interface

The local [Connection Interface](../technical/endpoint-service-runtime.md#ownership)
owns the AAI4 bidirectional byte-stream requests for all content in
`internal/application/endpointstream`. The Endpoint applies the same authority and confinement rules to all bytes.
The authenticated Service context and Connection continuity have no content-type selector.

The separately authorized v1 Administration Interface in
`internal/application/administration` owns publication and withdrawal and the
fixed byte-stream upgrade for the Publisher attachment. The Interface owners
define versioned framing, bounds, local transport, refusal and cleanup.
The former AAI2 request remains refused before Application owner I/O.

[`ownership.json`](../development/ownership.json) is the machine-checked source,
test, command, packaging, qualification, Interface, and historical-evidence
inventory. Each maintained owned file and lane has exactly one owner. The
current artifact lanes include the headless commands and fixed document
worker inventory defined by [Enrollment](../technical/enrollment-verification.md).
The Application Interface remains a neutral shared seam; the isolated new
command consumers have their explicit [command contracts](../reference/commands.md)
and do not imply full installed product acceptance.

The former Browser implementation and qualification lanes are retired by
[ADR-0069](../adr/0069-retire-active-browser-implementation.md), and
[ADR-0091](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
in-tree Firefox/Endpoint compatibility source. Its accepted ADRs, research
records, and immutable audit receipts remain non-executable evidence only.
They do not inherit a Network security claim, Browser isolation claim, Web PKI
identity, general proxy authority, or supported Firefox participant journey.

## Excluded historical evidence

The following are not maintained candidate surfaces:

- completed experiment implementations and their runners;
- the former `reference-c2` executable fixture and its stage-specific
  qualification harnesses;
- retired release-assembly and project-control simulation commands;
- superseded planning briefs, status chronology, and split-candidate release
  ledgers; and
- Firefox/Endpoint source formerly retained under
  `tests/compatibility/browser-endpoint-v4`, retired by ADR-0091.

Git history and accepted research/ADR records preserve every item's
provenance. None of them is part of builds, package inventories, current
qualification, or the C0 Network candidate.

## Claims withheld from C0

C0 preparation and an internal audit do not establish:

- anonymity, censorship resistance, availability, or traffic-correlation
  resistance;
- independent operators, builders, auditors, control, or public governance;
- supported public deployment, capacity, hostile-load, soak, VPS, or platform
  qualification;
- a public permissionless Namespace, participant Browser Entry, public release
  or Public Beta;
- Application-level network isolation for a generic Adapter; or
- a complete participant journey using only the supplied artifacts and
  supported operator commands.

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
| Domain boundaries and collaborations | [Domain map](../development/domain-map.md) |
| Product requirements and journeys | [functional map](functional-map.md), [journeys](journeys.md), [operating model](operating-model.md) |
| Security and privacy claims | [threat model](../security/threat-model.md) |
| Network/Route/Node | [Network Route and Node](../technical/network-route-node.md) |
| Endpoint/Service/Application seam | [Endpoint and Service runtime](../technical/endpoint-service-runtime.md) |
| Exact-Target private reachability | [Private reachability](../technical/private-reachability.md) |
| Future Service Names and retired Naming surface | [Current scope](#c0-command-roles-state-and-outcomes); no accepting Naming owner is selected |
| Enrollment and artifacts | [Enrollment verification](../technical/enrollment-verification.md) |
| Release, replacement, and custody | [Release and update custody](../technical/release-update-custody.md) |
| Commands, packages, and checks | [command reference](../reference/commands.md), [package map](../development/package-map.md), [testing](../development/testing.md) |
| Audit method | [Deep audit campaign](../development/deep-audit.md) |
