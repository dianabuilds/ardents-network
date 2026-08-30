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
- **Measurement:** `internal/custody` already distinguishes encrypted Service
  Authority records from Name Authority records, exports and test-restores
  recovery bundles, and restores them only as `authority-locked`. The maintained
  `ardents-custody` command deliberately exposes no Service Authority creation,
  issuance, or recovered-Service activation route.
- **Measurement:** the lower-level Publisher plan still names plaintext
  `CredentialFile` and `InstanceKeyFile`. `publicationInputs` decodes both and
  passes the raw private key into Endpoint. This is suitable evidence for the
  old composition but is not the selected headless Instance acquisition
  Interface.
- **Measurement:** `internal/service/publication` persists a monotonic
  generation floor and public record, retains the Instance signer only in
  volatile memory, erases it on withdrawal, and intentionally refuses to make
  a persisted record live after restart. A supported restart therefore needs
  an Instance-acquisition owner; re-reading the same raw key and Credential is
  not recovery because the publication floor rejects the same generation.
- **Inference:** adding only a custody signing command would leave the caller
  responsible for raw private-key files, import ordering, retry, and crash
  reconciliation. That would be a shallow pass-through and would not close
  R-131 or B6.
- **Inference:** the cohesive seam is a host-owned Instance enrollment Module.
  It must hide key generation, owner-only persistence, request/response
  matching, at-most-once acceptance, publication-floor reconciliation, and
  withdrawal erasure behind one narrow acquire/open outcome. Endpoint remains
  a consumer of an opened Instance binding, not its Authority or file-format
  owner.

## Decision-ready option 1 contract

Option 1 is implementable without selecting an operating-system keystore or a
new runtime dependency if the usable-alpha claim is deliberately portable and
bounded as follows:

1. **Authority Custody owns roots and issuance.** A separate interactive
   custody operation creates one encrypted Service Authority record or uses an
   already active record. A second operation accepts only one canonical public
   Instance enrollment request, advances the encrypted record's monotonic
   Service-generation watermark, and returns one public bounded Credential.
   Neither operation releases root material or accepts a password through
   argv, environment, configuration, or shared stdin.
2. **The Publisher host owns Instance enrollment.** A headless Network command
   creates an owner-only Instance root and generates the Instance signing key
   and Introduction recipient key inside it. It emits only a canonical public
   request containing their public keys, Network ID, requested validity bound,
   and a fresh request commitment. It has no Target input: Custody derives the
   Target from its authenticated Service Authority.
3. **Acceptance is at most once.** The host accepts only a Credential that
   matches its exact pending request, public keys, Network, validity bound, and
   request commitment. It atomically records either the accepted generation or
   a terminal rejected/conflicting state. Repeating the exact response is
   harmless; a different response for that request never replaces it.
4. **Runtime receives an opened binding, not files.** The supported headless
   Publisher opens the Instance root and passes a non-exporting signer plus the
   authenticated public Credential into `internal/service/publication`.
   `CredentialFile`, `InstanceKeyFile`, raw Target, and Service Authority are
   absent from the maintained headless runtime Interface. The old lower-level
   plan may remain only as non-candidate diagnostic compatibility until a
   separately reviewed retirement.
5. **Restart is fail-closed and generation-monotonic.** A crash cannot revive a
   generation whose publication floor was already committed. Reconciliation
   classifies a prepared-but-unpublished generation for its one permitted
   publish attempt, an already-consumed generation as requiring a successor,
   and malformed or ambiguous state as unavailable. Routine restart/recovery
   obtains a new host key and higher Credential when the prior generation was
   live; it never copies or silently reuses the old Instance key.
6. **Withdrawal removes runtime authority.** Withdrawal first prevents new
   publication/connection acquisition, drains retained work, erases the live
   signer, and terminally closes the accepted Instance generation. Public
   Credential bytes and monotonic floors may remain as non-secret evidence.
7. **Recovery is honestly bounded.** Existing Authority Recovery Bundle export
   and isolated test-restore remain supported. A restored Service Authority is
   `authority-locked` and cannot issue; Ardents currently has no accepted
   authenticated Service-currentness witness that could safely activate it.
   The usable-alpha journey must prove this refusal and state that loss of the
   active Authority record loses the Target. Selecting later activation is a
   separate consequential research/ADR decision, not an implicit local-floor
   reset.
8. **Enrollment remains distribution only.** Enrollment-v3 authenticates the
   real Network artifacts and may launch the acquisition commands, but contains
   no Authority, Instance secret, Credential, Target, generation choice, or
   reusable issuance power.

The portable owner-only root limits the product claim: there is no hardware
non-exportability, protection from an administrator, snapshot resistance, or
post-compromise healing. “Non-exporting” means that no supported Interface
returns the private Instance key; filesystem compromise remains endpoint
compromise under the accepted threat model.

This shape is one deep Module at the host Instance seam and one existing deep
Module at the custody seam. It does not introduce a generic signer Interface,
an online Authority service, an enrollment registrar, or a second publication
implementation.

## Exact Product Owner decision requested

Accept or reject the following statement before implementation:

> **R-131:** For usable alpha, select separate local Service Authority Custody
> plus a host-owned one-generation Instance enrollment root. Custody alone
> creates/imports the Service Authority and issues a monotonically higher
> bounded public Credential for a canonical host-generated public request. The
> host alone retains the non-exporting Instance and Introduction private keys,
> accepts the exact response at most once, and exposes only an opened binding to
> the headless Publisher runtime. Restart never revives a consumed generation;
> it requires a fresh host key and successor Credential. Withdrawal erases the
> live binding. Enrollment-v3 carries no Authority, Target, Credential, or
> private Instance material. Restored Service Authority bundles remain locked
> and issuance-unavailable until a future accepted authenticated recovery
> witness; usable alpha makes no hardware-keystore, administrator-resistance,
> or active Authority-recovery claim.

Acceptance authorizes a short ADR that fixes these ownership and recovery
semantics, followed by one test-first vertical implementation. Rejection must
select option 2 or explicitly remove fresh Publisher creation from the product
candidate; changing only command names or generating fixture credentials does
not resolve the decision.

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

Ask the Product Owner to accept the exact bounded option 1 contract above. It
matches the accepted privilege lattice and J-03 while making the missing active
Authority-recovery witness an explicit non-claim rather than inventing one.
Confidence: high that this is the smallest option that can close B6 without
moving Authority into enrollment or leaving raw-key orchestration to callers.
The strongest argument against it is operational: a separate request/response
custody ceremony is less convenient, and loss of the active Authority record
loses the Target even when an authority-locked backup survives.

## Disposition

R-131 is open and blocks only the fresh Publisher-authority portion of B6. It
does not reopen R-128/R-130, Route/Target/wire semantics, Browser separation,
or the artifact-native enrollment/start/restart work that can proceed
independently. No ADR or implementation is authorized until the Product Owner
accepts an option or changes the selected candidate scope.
