---
id: R-074
title: Which selected owner, if any, may accept opaque Namespace claim inputs and issue the complete R-042 threshold Epoch close without recreating a shared Network/Namespace foundation?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-074 — Epoch claim-close owner

## Decision this unlocks

M5 F023/F028 has a local `EpochClaimInput` admission and a Namespace verifier
for a supplied R-042 close, but no owner that collects all Epoch inputs,
commits their global roots, and threshold-signs a complete close. This record
decides whether Stage 8 may invent that owner or must leave the absent protocol
boundary explicit.

## Current contract

R-042 and ADR-0017 require commit in Epoch `E`, reveal in `E+1`, one complete
threshold-authenticated input root, materialization root, and rejection root.
Incomplete evidence mutates no Lease. ADR-0020 keeps a distinct threshold
Namespace materialization as the sole current-state publisher. R-060 forbids a
shared Network/Namespace persistence or commitment foundation; R-061 removed
the old cross-domain implementation dependency. R-072 permits Namespace to
consume a verified winner through a local installation but expressly leaves the
complete global close to its owner. No selected public Network Epoch protocol,
transport, consensus system, or global log implementation exists.

## Hypotheses

- **H1:** Namespace collects inputs and creates/signs the global close locally.
- **H2:** Network State adds a generic-looking claim-log/close facility now.
- **H3:** Stage 8 retains only opaque local admission and supplied-close
  verification; a future selected Network Epoch protocol names and implements
  the producer with its authority, availability, transport, and audit evidence.

## Evaluation criteria

The decision must preserve R-042 global completeness and ordinal semantics;
keep Namespace free of a Network State import and Network State free of
Namespace lifecycle ownership; avoid selecting a consensus/log/transport or
claiming availability; make incomplete, forked, or omitted closes fail closed;
and avoid a placeholder interface with no non-test owner.

## Evidence plan

### Primary sources

- R-042, ADR-0017, ADR-0020, R-060, R-061, R-071, and R-072, accessed
  2026-08-23.
- Accepted Stage 8 target architecture and refactoring plan, accessed
  2026-08-23.
- Current `internal/naming/namespace/claim_{contract,ingestion,verify}.go` and
  `internal/network/state/`, inspected 2026-08-23.

### Experiment

No experiment can establish an absent distributed producer. The falsification
check is structural: a candidate must name one real non-test owner, its
authority and complete global input/rejection source, and the R-042 close it
signs. Until then no code/format mutation may present a partial per-Name proof
as a complete global close.

### Failure scenarios

- Namespace accepts only one Name's claims and calls the resulting roots global;
- Network State stores Namespace claim semantics under a generic commitment
  label, recreating the forbidden shared foundation;
- a local Gateway or signer emits a threshold-looking close without the complete
  Epoch corpus;
- an unavailable, omitted, or forked close advances a Namespace Lease; and
- a speculative producer interface becomes an indefinitely retained mock.

## Findings

- **Sourced fact:** R-042 requires the signed close to bind global input,
  materialization, and rejection roots and lengths, not merely one Name's
  collision set.
- **Inspection:** `ClaimProof` verifies a supplied close and one Name's bounded
  reveal evidence; `EpochClaimInput` is opaque local admission input. Neither
  is an Epoch collection/log or signing process.
- **Inspection:** `internal/network/state` has no Namespace import or
  `ClaimProof`/`EpochClaimInput` owner after the R-061 transfer.
- **Inference:** H1 would make Namespace an unselected global log and threshold
  authority. H2 would reintroduce cross-domain commitment ownership before a
  Network Epoch protocol selects its semantics. Neither can meet the
  completeness criterion through a package refactor.

## Options

| Option | Fit and risk | Disposition |
|---|---|---|
| H1 Namespace-local producer | Violates R-042 completeness and turns Namespace into an unselected global authority. | Reject. |
| H2 Network State placeholder | Violates R-060's ownership boundary and selects protocol choreography without a real owner. | Reject. |
| H3 explicit deferred producer | Preserves local fail-closed verification and makes the missing global authority visible without a fake implementation. | Choose. |

## Recommendation

Accept H3 with high confidence. `AdmitClaimCommitment`, `EpochClaimInput`, and
`VerifyClose` remain Namespace-local mechanisms. They neither collect global
Epoch inputs nor manufacture a close. A future producer requires a new
decision tied to the selected Network Epoch protocol and must name its
threshold authority, complete input/rejection corpus, transport/availability
conditions, fork handling, audit evidence, and one non-test caller before it
may feed `EpochClaimInput.VerifyClose`.

The strongest argument against H3 is that it leaves an R-042 root claim unable
to become current in the maintained runtime. That limitation is true today;
making a local substitute would silently claim a global authority the project
has not selected.

## Disposition

**Accepted H3 on 2026-08-23 under the Product Owner's standing Stage 8
delegation.** No ADR is needed: no protocol, technology, persistence engine,
or trust root is selected. M5 must retain its local close-verification tests,
record that global claim-current behavior is unavailable without the future
producer, and must not add a generic Network/Namespace bridge. A future
producer reopens this question and the applicable Network protocol authority.
