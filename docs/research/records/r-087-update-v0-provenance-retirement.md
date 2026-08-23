---
id: R-087
title: How may the Update tracer retire its V0 custody evidence safely?
status: superseded
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-087 — Update V0 provenance retirement

## Decision this unlocks

Retire the expiring R-064 C2 `ardents-release` command and its V0
`custody_notice` representation without resetting a durable Update root or
allowing V0 runtime bytes to remain a product-like lifecycle surface.

## Current contract

R-064 retains one bounded H3 Release/Update transaction but says its command
observer expires in M13. ADR-0015 separates Release authorization from local
activation; neither it nor ADR-0021 grants Update custody authority. The V1
Update root binds each `manifest.bin` digest into `current` and journals; the
manifest embeds `EvidenceNotice`, whose V0 rendering is `custody_notice`.

## Hypotheses

- **H1:** atomically migrate one valid owned V1 root to a V2 root whose
  manifests omit EvidenceNotice, then reject V1 at runtime and retain exact V0
  inputs/results only through a C4 verifier.
- **H2:** retain a V1 runtime reader indefinitely while new writes use V2.
- **H0:** delete the bounded Update tracer and all V0 evidence.

## Evaluation criteria

The result must preserve one-writer ownership, complete inventory, exclusive
lease, monotonic transaction/generation selection, artifact and manifest
identity, rollback/recovery refusal on malformed state, and interruption-safe
conversion. It must not expose a Custody secret, make a platform activation
claim, silently reset a root, or accept a V1 root as active after migration.

## Evidence plan

### Primary sources

- R-064, ADR-0015, ADR-0021, accessed 2026-08-23.
- `internal/update` V1 encoding, inspection, current-pointer, and recovery
  tests, inspected 2026-08-23.

### Experiment

Freeze valid V1, malformed V1, interrupted conversion, converted V2,
rollback, and restart vectors. A C4 verifier independently decodes the exact
V0 command/result and manifest vectors but is absent from maintained runtime
imports.

### Failure scenarios

A malformed V1 root becomes V2; a crash selects a partial conversion; a V1
root is reactivated after conversion; a V0 evidence string reaches a V2
result; a reset discards a rollback candidate; or a verifier becomes a runtime
reader.

## Findings

- **Inspection:** V1 has one root marker and its manifest bytes are committed
  into `current`, predecessor, and journal identities; deleting one field
  in-place invalidates recovery.
- **Inspection:** the C2 command is the only JSON writer of the exact V0
  `custody_notice`, but the durable V1 manifest also carries the same
  EvidenceNotice.
- **Inference:** an owned C1 conversion is required; a permanent runtime V1
  reader would contradict the R-064 M13 expiry, while deleting the whole
  transaction contradicts R-064 H1.

## Options

| Option | Disposition |
|---|---|
| H1: one-shot C1 V1→V2 conversion and C4 verifier | Choose. Preserves recovery integrity while retiring the legacy runtime representation. |
| H2: permanent V1 reader | Reject. Retains an unbounded legacy runtime format after its observer expires. |
| H0: delete Update and evidence | Reject. Discards the accepted bounded transaction characterization without a product or security reason. |

## Recommendation

Choose H1. Confidence is high because no observer requires V1 runtime
compatibility and V1 identity commitments make an in-place omission unsafe.
The strongest counterargument is migration complexity; bounded one-shot
conversion with crash tests is smaller and safer than a permanent legacy
reader.

## Disposition

Superseded by R-088 and ADR-0030 on 2026-08-23. Their source audit found no
production root initializer or parent-pointer owner: the only V1 roots are
test fixtures and `cmd/ardents-release` receives an already initialized path.
The C1 mechanism selected here would therefore create an unowned selector.
R-088 retains the transaction behavior but replaces the unobservable V1 test
format by C0 V2 fixtures and a C4 V0 verifier.
