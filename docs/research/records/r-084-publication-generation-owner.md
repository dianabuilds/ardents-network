---
id: R-084
title: What durable publication/generation owner gives M9 one crash-atomic Instance hand-off without retaining the H3 Endpoint action union?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-084 — Publication generation owner

## Decision this unlocks

Select the M9 `service/publication` durable owner for exclusive Instance
generation, current publication, supersede/drain/unpublish, and crash recovery.

## Current contract

R-006 selects one active delegated Instance with monotonically higher routine
migration generation. D05 requires one generation/publisher owner with
explicit drain/cutover. The current `serviceconn` combines local session
admission, publication, connection completion, and a generation file; its
writer uses a direct `os.WriteFile`, which is neither an atomic current
publication transaction nor a crash recovery protocol. R-082 permits retiring
its tracer syntax; R-083 deliberately consumes only a publication digest.

## Hypotheses

- **H1:** `service/publication` owns an exclusive root with immutable
  generations, atomic current-pointer replacement, restart reconciliation, and
  an explicit drain lifecycle.
- **H2:** retain the current single generation text file next to endpoint
  action handling.
- **H0:** D05 needs a Custody or platform decision before it can be safely
  implemented.

## Evaluation criteria

The owner must prevent stale/concurrent publication, never reactivate an old
generation after crash, erase volatile Instance material on unpublish/drain,
and make recovery state tamper/partial-write visible. It must keep Service
Authority/Custody private material outside the publication interface, admit no
generic shared store, and preserve one active Instance without claiming
compromise revocation or multihoming.

## Evidence plan

Inspect R-006, D05, the current credential/publication/generation code, and
the M3/M5 domain-owned durable-root patterns. The target implementation must
prove sequential/concurrent publish, stale/supersede, unpublish/drain, reopen,
partial write, tamper, and private-material cleanup through its own interface.

## Findings

- **Inspection:** Network State's domain-owned control journal commits an
  immutable generation before atomically replacing a bounded current pointer;
  a non-empty root without that pointer fails closed rather than reconstructing
  a choice from directory order.
- **Inspection:** the current Service Connection generation file records only
  a number. It cannot prove which publication was current and cannot recover
  Instance private material, so treating it as a recoverable publication would
  be a false availability claim.
- **Inference:** a C1 floor-only migration preserves the one security-relevant
  old fact (no lower generation) while making post-crash publication absence
  explicit until the Service republishes with a higher generation.

## Selected publication root

Choose H1. `service/publication` owns one exclusive root with immutable
`generations/<16-hex-generation>/publication.bin` records and an atomically
replaced `current` pointer. A record contains only canonical public publication
facts and its digest; Instance private material is volatile, held only by the
live owner, and is erased on supersede, unpublish, close, or failed commit.
The root retains a monotonic floor even when no current publication exists.

The C1 migration reads the former one-number floor exactly once only when the
target root is empty, commits it as the new root's floor, and never writes the
old path again. A target root with an invalid/missing pointer, surplus
generation, digest mismatch, partial staging file, or floor/current conflict
fails closed. After restart, a valid current public record may be inspected but
does not become active because the private Instance key is absent; acquisition
returns unavailable until a higher live publication is committed. A new
publication must have a strictly higher generation than the persisted floor;
publication is current only after its immutable record and pointer are durable.

`Acquire` retains a reference to the current live generation. `Supersede` and
`Unpublish` withdraw it from new acquisition then wait for retained references
to drain before erasing private material. There is no concurrent Instance,
multihoming, Custody writer, generic storage package, or caller-authored
generation reset.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
M9 may create the target-owned C1 root and migrate the old floor exactly once.
No ADR is needed: this selects a domain recovery rule and durable format within
the accepted M9 ownership change, without selecting a storage engine, platform,
or Custody lifecycle.
