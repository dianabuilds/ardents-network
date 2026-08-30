---
id: R-129
title: Can Endpoint and Node form a complete headless Ardents Network product with replaceable local Adapters?
status: active
owner: Product Owner and Codex
started: 2026-08-30
reviewed: 2026-08-30
---

# R-129 — Headless Network product boundary

## Decision this unlocks

Decide the maintained product, process, Module, artifact, and dependency
boundary between Ardents Network and optional Browser or future Desktop
Adapters before selecting the exact C0 audit candidate. The result determines
which coupling must be removed now and which co-located code may remain behind
a narrow local Interface.

## Current contract

The [product-boundary objective](../../product/network-application-separation.md),
[product scope](../../product/scope.md),
[threat model](../../security/threat-model.md), and
[glossary](../../../CONTEXT.md) apply. ADR-0009 selects Go as the maintained
runtime foundation. ADR-0010 permits repository co-location; it does not permit
a UI Adapter to own Network State, Route choice, Service identity, or custody.
The Application Interface remains the local Connection and Service
Administration seam. H4 is roadmap vocabulary only and supplies no runtime or
package boundary.

## Preliminary state report

The completed preliminary read-only discovery inspected the clean source fixed
point `0292afbd572c09b55ee5cf2589ac44af99edfa66` on 2026-08-30. That commit is
the immutable comparison baseline for the remediation diff; it is not a C0
candidate and received no qualification verdict. Raw transient inspection
output is not a product specification. This record is the durable state and
finding-disposition map required by phase A1.

| Finding | Verified baseline state | Selected disposition before C0 |
|---|---|---|
| P-001 product/evidence truth | Current documents transferred some RC1/RC2 facts and mixed historical H4 plans with current product claims. | Repair factual owners and require independent truth review. |
| P-002 planning identity in runtime | Fresh Rendezvous contributor artifacts emitted an H4-derived profile identity. | Migrate writers to a domain profile, retain only bounded legacy readers, and test both directions. |
| P-003 fixed candidate assembly | RC1/RC2-specific release assembly remained in the current custody command and Module. | Retire the candidate-specific routes and Implementation under ADR-0059; retain immutable evidence only. |
| P-004 completed planning generators | Four accepted project-control simulators remained exposed as current commands and packages. | Retire their generators under ADR-0060 and preserve unique rejection/lifecycle behavior in domain tests. |
| P-005 headless product gap | Lower Network dependency direction was viable, but Browser-specific composition and fixture-led H4-3 evidence obscured a complete supported headless journey. | Complete the headless product/artifact lane and separate optional Adapter ownership under R-129 before C0. |
| P-006 participant acquisition | The maintained Endpoint accepts an operator plan; no accepted normal participant owner/lifecycle supplies current State, Entry, and finite one-use transport input. | Return the authority/lifecycle choice to bounded research; do not invent it during refactoring. C0 remains blocked until the selected product scope has an honest journey. |
| P-007 candidate-specific secrets/access | A release seed and temporary qualification access were used during historical qualification. | Treat historical material as exposed for any future release, rotate/retire exact secrets before reuse, and clean only explicitly identified temporary access. |
| P-008 deterministic gates | Preliminary and remediation gates exposed scheduler-sensitive pacing and file-publication races; one exact-limit HTTP failure still required diagnosis. | Repair causes without weakening oracles, preserve first failures, and require the complete ordinary gate before candidate freeze. |

The map is intentionally bounded to findings verified during preliminary
discovery. Formal A-F audit may discover additional defects only after one
stable C0 candidate is activated. A disposition is terminal for C0 only when
its repair, retirement, explicit limitation, or accepted product-scope change
is present in the exact candidate and its required evidence is green.

## Hypotheses

- **H1:** the maintained Endpoint and Node can build, run, and exercise the
  supported participant journey without Browser, XPI, or Desktop artifacts,
  while optional Adapters use narrow local Interfaces.
- **H2:** repository co-location can remain maintainable only if artifact lanes
  and dependency direction are explicit even when source remains in one Go
  module.
- **H0:** completing the headless journey requires duplicated Network behavior
  or gives an Adapter Network State, Route, Service Authority, or custody
  ownership; in that case the proposed boundary is invalid and must be
  redesigned.

## Evaluation criteria

- headless Endpoint and Node build and pass their maintained behavior profiles
  with no Browser/Desktop artifact prerequisite;
- a normal participant has an explicit acquire, start, publish or open,
  withdraw, stop, and recovery path for the selected product scope;
- Network Modules do not import presentation or Browser implementations;
- Browser and future Desktop Adapters cannot select Route, mutate Network
  State, obtain Service Authority, or become custody owners;
- Network, Browser, and future Desktop release artifacts are independently
  enumerable and cannot acquire an undeclared companion at runtime;
- the one-Product-Owner-and-Codex team can maintain the resulting Interfaces,
  tests, and compatibility surface without speculative frameworks.

Any required ownership inversion, duplicated Network implementation, hidden
artifact dependency, or unsupported lifecycle falsifies the candidate. Passing
tests alone does not establish the product boundary if current documentation or
release inventory contradicts it.

## Evidence plan

### Primary sources

- accepted ADRs, current product contract, package map, build profiles, command
  implementations, and maintained behavior tests, accessed 2026-08-30;
- Go package imports and produced artifact inventories from the exact candidate
  revision, captured during the bounded separation change;
- ordinary repository gates and the separately declared headless build/check
  profile from the exact candidate revision.

This is an internal product-architecture question. External technology
selection is not authorized by it; any such selection requires its own primary
sources and research record.

### Experiment

For each candidate revision, reproduce the documented headless check, inspect
its artifact inventory, run package-direction checks, and execute the supported
headless participant behavior without installing or invoking a Browser/XPI.
Then run the ordinary repository gates to detect accidental loss of the
co-located optional products. Exact commands and receipts are recorded with the
owning implementation slice rather than invented before the Interface exists.

### Failure scenarios

- deleting or omitting Browser assets prevents Endpoint or Node start;
- a Browser/Desktop Adapter imports Network implementation or reads protected
  Network State directly;
- the CLI and a UI duplicate Route or Service lifecycle behavior;
- a headless artifact silently downloads or launches an optional Adapter;
- Adapter removal mutates protected Network or Authority state;
- independent artifact versioning creates an unbounded compatibility matrix;
- a fixture-only path is mistaken for the supported participant journey.

## Findings

- **Measurement:** preliminary import inspection found that lower
  `internal/network`, `internal/node`, `internal/route`, `internal/service`, and
  `internal/entry` Modules do not depend on Endpoint or Browser
  implementations.
- **Measurement:** `cmd/ardents-node` is already a separate headless process,
  and the local Application Interface already separates Connection from
  Service Administration authority.
- **Measurement:** Browser-specific launch, Browser Entry state, loopback HTTP
  presentation, and alpha-browser proxy lifecycle remain inside Endpoint
  composition, while current H4-3 evidence relies substantially on fixture
  orchestration rather than a complete supported participant command journey.
- **Inference:** the lower dependency direction is compatible with H1, but it
  is insufficient to select C0 until the participant journey, ownership map,
  headless artifact lane, and remaining Browser coupling are dispositioned.
- **Assumption:** one repository and one root Go module remain less burdensome
  than a repository split for the current two-member collaboration model; this
  assumption is rejected if independent release maintenance becomes
  unbounded.
- **Implementation measurement (2026-08-30):** the maintained Endpoint now
  exposes a Service-Link-only local Connection Interface; `ardents` and
  `ardents-control` have Browser-free transitive dependency gates; Browser
  presentation runs in a separate `ardents-browser` Adapter; Network `Verify`
  rejects enrollment-v4 while the Browser lane requires it; and
  `make headless-check` builds, packs, unpacks, hashes, and byte-compares real
  host-named Endpoint/control binaries. The separate `make browser-check` lane
  builds the real host-named Browser Adapter/native-host commands, packages the
  exact bytes into a deterministic Browser-only archive, unpacks and
  byte-compares them, and separately verifies the enrollment-v4 companion
  inventory. This is build evidence, not signed-XPI or release qualification.
- **Implementation measurement (2026-08-30):** the CLI now uses that same
  local interface for `open` and the separate administration socket for
  `publish`/`withdraw`. Deterministic Endpoint/C-2 tests carry bytes and cover
  issuer budget, idempotency, fixed outcomes, and at-most-once restart phases.
- **Blocking fact (2026-08-30):** no non-test process owns
  `credential.NewIssuer`. Its raw signer key/static Initiator input and
  profile-before-State bootstrap cycle require the R-130 Product Owner
  decision. Consequently B6 and C0 remain open; composing a fixture-only
  artifact test would not satisfy the evaluation criteria.

### Planning-label disposition

| Material | Disposition | Rule |
|---|---|---|
| Historical qualification schemas, environment variables, directories, and Make entrypoints containing H4/RC identities | **Preserve** | They identify immutable evidence and are never imported into the headless product/runtime schema. Heavy qualification remains deferred until final freeze. |
| `h4-5-rendezvous-alpha-v1` resource profile | **Generalize** | Current writers emit `ardents-rendezvous-dedicated-host-v1`; the H4 identity remains read-only compatibility for already pinned input and receives no new writer. |
| Fixed RC1/RC2 assembly commands and candidate identities | **Retire** | ADR-0059 removed current assembly routes. Reference evidence remains immutable and cannot seed a successor architecture or transfer qualification. |
| H4 as a runtime/domain/package/schema boundary | **Retire** | H4 remains roadmap provenance only. New Application, enrollment, artifact, and local-interface schemas use purpose-owned identities. |

## Options

1. Keep current composition unchanged. This minimizes immediate edits but
   leaves the headless product contract and artifact boundary unverifiable.
2. Keep one repository while separating cohesive Modules, process ownership,
   artifact profiles, and dependency direction. This is the active candidate.
3. Split repositories or introduce a generic plugin framework now. Neither is
   justified by a second maintained consumer and both increase release burden.

## Recommendation

Evaluate option 2 through bounded implementation slices. Preserve one root Go
module, create no speculative Interface, and add an Endpoint Owner Interface
only when an actual Desktop caller exists beside the CLI. Confidence is
moderate: the import direction is promising, but normal participant
acquisition and Browser extraction remain unproven. The strongest argument
against the recommendation is that independent artifact compatibility may cost
more than co-location saves for the current team.

## Disposition

R-129 remains **active**. Preliminary discovery is durable here but is not the
formal A-F audit and not qualification. Truth reconciliation, bounded
remediation, headless journey completion, dependency/artifact separation, and
independent review precede selection of exact C0. Consequential final ownership
or artifact decisions require an accepted ADR; implementation evidence and a
falsification result are recorded before this question can be decided.
