---
id: R-061
title: In which order may Network and Namespace take separate persistence ownership?
status: proposed
owner: product research
started: 2026-08-22
reviewed: 2026-08-22
---

# R-061 — Domain-ownership transfer order

## Decision this unlocks

Select the M3/M5 sequencing amendment required to remove the current
`network/store` and `network/epoch/merkle` packages after R-060, without
making Namespace a temporary Network State client or creating a generic shared
foundation. The result changes the accepted S8.4 wave order and therefore
requires Product Owner acceptance before source deletion.

## Current contract

S8.3 assigns authenticated Network State persistence and commitments to
`network/state`, and Namespace persistence and proof semantics to
`naming/namespace`. S8.4 M3 requires deletion of `network/store` and
`network/epoch/merkle`; M5 follows M3 and is the named Namespace target wave.
R-060 proposes that neither domain own the other's representation.

The current `internal/namestore` directly imports both source packages. Its
exclusive root, immutable generations, atomic `current` pointer, durable
reopen/tamper/stale checks, compact membership proofs, and commitment rules are
therefore already live Namespace behavior. `namestore` is an existing cohesive
owner, not a new target package, but it cannot survive M3's specified deletion
without first owning its own mechanics.

## Hypotheses

- **H1 — explicit prerequisite transfer:** after R-060, add an accepted
  M3 prerequisite that transfers the currently used persistence and commitment
  mechanics into the existing Namespace owner, with Namespace-only conformance
  tests and no format change; then M3 transfers/deletes Network mechanics.
- **H2 — retain the Network implementation through M5:** let M3 retain
  `network/store` and `network/epoch/merkle` as an adapter until the later
  Namespace target wave.
- **H0 — shared foundation:** extract a new common persistence or Merkle
  package before either source deletion.

## Evaluation criteria

The accepted sequence must:

1. leave exactly one persistence/commitment implementation owner for each
   domain once M3 deletes the Network source paths;
2. preserve R-043 durable/restart/tamper/stale properties and Namespace proof
   bytes until DA-03, DA-04, and DA-07 authorize a format change;
3. preserve R-059's Network State fail-closed current-pointer behavior;
4. avoid a dual writer, a runtime format migration, new package, generic API,
   new engine, or added dependency as a side effect of ordering;
5. give both domains independent tamper, partial-write, restart, stale, and
   proof-mutation characterization before the first source-path deletion; and
6. record the revised wave prerequisite, owner, deletion point, reader/writer
   disposition, and rollback condition in the S8.4 plan before execution.

H1 is falsified if copying the mechanics necessarily changes Namespace bytes,
introduces a second Namespace writer, or a real common invariant requires one
selected owner. H2 is falsified if the retained Network path continues to
expose Namespace to Network migration or recovery policy after M3 begins.

## Evidence plan

### Primary sources

- Accepted S8.3 target architecture and accepted S8.4 refactoring plan,
  accessed 2026-08-22.
- Proposed R-060 and decided R-043/R-027/R-029/R-039, accessed 2026-08-22.
- Current `internal/namestore/{store,contract,proof,materialization}.go`,
  `internal/network/store/{contract,state,files}.go`, and
  `internal/network/epoch/merkle/{tree,proof}.go`, inspected 2026-08-22.
- Current import graph, inspected 2026-08-22: `namestore` imports exactly the
  two Network source packages that M3 promises to delete.

### Experiment

No code or format migration is authorized by this proposal. If accepted after
R-060, the prerequisite transfer uses current Namespace behavior tests as its
characterization suite and adds domain-local durable reopen, pointer/tamper,
partial-write, stale, canonical root, and mutated-proof tests. It must then
remove both Network imports from `namestore` before M3 starts its deletion
cutover. A test failure freezes the transfer and retains the old source path;
it is not repaired by an adapter or shared package.

### Failure scenarios

- M3 deletes the Network packages and makes Namespace unbuildable;
- one root lease or pointer repair affects both domains;
- identical Merkle bytes permit a Network proof as a Namespace proof;
- copying mechanics changes Namespace snapshot/proof bytes or stale handling;
- a temporary adapter becomes an indefinite cross-domain import; and
- a rollback restores an old writer after a target writer has published.

## Findings

- **Measurement:** `internal/namestore/store.go` imports `network/store`, and
  `proof.go` plus `materialization.go` import `network/epoch/merkle`.
- **Measurement:** M3 names all three source paths in its mandatory deletion
  outcome, while M5 is ordered after M3.
- **Sourced fact:** R-043 owns Namespace durable-state properties and selected
  a naming-owned boundary, not Network State recovery authority.
- **Inference:** preserving the present order without a transfer prerequisite
  either leaves the wrong cross-domain dependency alive or makes M3's deletion
  promise impossible. The sequencing problem is independent of which domain
  owns the mechanics, but it is only actionable once R-060 is accepted.

## Options

| Option | Fit and risk | Disposition |
|---|---|---|
| H1 explicit prerequisite transfer | Makes the existing Namespace owner independent before the Network source deletion; creates temporary local duplication, but preserves distinct authority and measurable behavior. | **Recommend.** |
| H2 retain Network implementation through M5 | Appears smaller, but contradicts M3's deletion outcome and retains Namespace dependence on Network State placement. | Reject. |
| H0 shared foundation | Reopens the generic-foundation option rejected by R-060 without a shared semantic owner. | Reject. |

## Recommendation

After acceptance of R-060, choose H1 with high confidence. Amend M3/M5 before
code work: an explicitly named prerequisite transfers only current Namespace
persistence and proof mechanics into `namestore`, removes its Network imports,
and changes no Namespace format or writer semantics. M3 then transfers and
deletes Network State's source implementations; M5 later performs the already
planned semantic consolidation into `naming/namespace` under its separate DA
gates.

The strongest argument against H1 is short-lived duplicate code. That cost is
visible and bounded, whereas H2 silently preserves the cross-domain authority
that R-060 is intended to eliminate. A later shared extraction remains subject
to R-060's falsification condition and separate authority.

## Disposition

- State: `proposed`; it has no effect until R-060 and this record are both
  accepted by the Product Owner.
- On acceptance, amend S8.4's M3/M5 rows and dependency paragraph before
  source moves; update the DA-05 route with both decision identities.
- No ADR, experiment code, package, dependency, or format is selected.
