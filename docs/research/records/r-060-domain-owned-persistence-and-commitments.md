---
id: R-060
title: May Network State and Namespace share persistence or commitments?
status: proposed
owner: product research
started: 2026-08-22
reviewed: 2026-08-22
---

# R-060 — Domain-owned persistence and commitments

## Decision this unlocks

Resolve DA-05 before M3 moves `internal/network/store` or
`internal/network/epoch/merkle`, and before M5 creates `naming/namespace`.
The decision determines whether a common foundation is justified or each domain
must own its own representation and adapter boundary.

## Current contract

S8.1 preserves Network State and canonical Namespace semantics but permits
their package and representation replacement. S8.3 assigns Network current/
pending publication to `network/state`, while `naming/namespace` owns Name
Authority, Lease, Claim, Recovery, admission, and its durable generation.
Neither Module owns the other's freshness, authorization, lifecycle, root, or
recovery policy.

Current `namestore` imports `network/store` for its root/generation pointer and
imports `network/epoch/merkle` for Namespace materialization proofs. R-043
previously selected a *naming-owned* storage interface backed by the current
Network store, but also states that the interface/adaptor did not yet exist and
does not select a generic engine. The current direct imports therefore do not
implement R-043's intended boundary and couple Namespace persistence to Network
State path, recovery, and commitment placement.

## Hypotheses

- **H1 — domain-owned representations:** `network/state` and
  `naming/namespace` each own their persisted generation and commitment
  representation. Identical algorithmic mechanics may remain local until one
  real shared invariant owner and callers exist.
- **H2 — shared domain foundation:** one new storage/commitment Module owns
  shared durable generations and Merkle representation for both domains.
- **H3 — retain Network foundation:** Namespace imports the Network store and
  Merkle implementation through an interface/adapter.
- **H0 — new storage engine:** a selected engine is necessary before either
  target Module can meet its contract.

## Evaluation criteria

Before source changes, a candidate must:

1. give exactly one domain owner for every root, current pointer, recovery
   rule, state transition, and proof semantic;
2. prevent a naming representation change from requiring a Network State
   migration and vice versa;
3. preserve each domain's fail-closed tamper, stale, conflict, and bounded
   recovery behavior without a shared authority record;
4. permit identical reviewed byte-level tree mechanics only when the semantics,
   domain separation, format version, and failure rules remain domain-local;
5. avoid a speculative `store`, `merkle`, `common`, or interface package; and
6. select no storage engine, database, consensus system, or crypto primitive by
   refactoring.

H1 is falsified if both domains demonstrably share one changing invariant,
failure/repair policy, format lifecycle, and real callers such that separate
owners would duplicate authority rather than mechanics. H2 is falsified if any
shared API admits a domain-specific field, recovery decision, or import cycle.

## Evidence plan

### Primary sources

- R-043 persistence/restart/rollback record, accessed 2026-08-22.
- R-027 Network State atomic-pointer contract and R-039 Namespace lifecycle,
  accessed 2026-08-22.
- S8.0 F020/F021 in the Network State delta review, accessed 2026-08-22.
- Current `internal/namestore/{store,contract,proof,materialization}.go`,
  `internal/network/store/`, and `internal/network/epoch/merkle/`, inspected
  2026-08-22.

### Experiment

No shared package experiment is authorized. M3/M5 must instead provide two
domain-owned conformance suites: tamper, stale, partial-write, restart, and
recovery for each persistence representation; canonical/proof mutation and
domain-separation tests for each commitment representation. A later extraction
is considered only after real shared callers/invariants appear and the
falsification criterion is met.

### Failure scenarios

- a Network pointer repair changes Namespace publication;
- a naming epoch/freshness rule alters Network source acceptance;
- a reused Merkle byte encoding accepts a proof from the wrong domain;
- a generic transaction/migration API permits two writers or mixes root locks;
- a new engine is introduced because a refactor needs an implementation detail;
- a future shared helper reintroduces `network` imports into Namespace.

## Findings

- **Sourced fact:** R-043 selected a naming-owned boundary, not Network State
  ownership of Namespace data, and explicitly left its interface/adapter absent.
- **Measurement:** current `namestore` imports both Network storage and Network
  Merkle paths directly; the package graph therefore encodes cross-domain
  implementation knowledge rather than a consumer-owned port.
- **Sourced fact:** S8.0 F020/F021 identifies precisely this shared-generation
  and commitment placement as a design decision, not a harmless extraction.
- **Inference:** the domains share filesystem and tree *mechanics*, but not a
  writer, activation condition, freshness, proof meaning, or recovery policy.
  This is duplication of mechanics, not a shared authority boundary.

## Options

| Option | Fit and risk | Disposition |
|---|---|---|
| H1 domain-owned representations | Keeps State and Namespace authority/format migrations independent; may temporarily repeat reviewed mechanics. | **Recommend.** |
| H2 shared domain foundation | Would centralize a cross-domain API before a shared semantic owner exists, risking a generic dumping ground and coupled repairs. | Reject. |
| H3 retain Network foundation | Preserves current cross-domain imports and makes Namespace dependent on Network State placement. | Reject. |
| H0 new engine | Adds unselected technology and no demonstrated missing property. | Reject. |

## Recommendation

Choose H1 with medium-high confidence. M3 folds current Network storage and
Merkle ownership into `network/state`; M5 separately creates Namespace-owned
persistence and commitment behavior. R-043 is superseded only where it names
`network/store` as the naming adapter candidate; its durable/restart/tamper/
stale property requirements remain binding. No ADR is required because this
selects no technology and removes an accidental package dependency rather than
creating a durable cross-domain lock-in.

## Disposition

- State: `proposed`; requires Product Owner acceptance before M3/M5 move the
  current Network Store or Merkle implementations.
- On acceptance, update R-043's supersession note and DA-05; M3 owns Network
  fault/conformance characterization, while M5 owns Namespace replacement.
- No experiment code or shared package is retained.
