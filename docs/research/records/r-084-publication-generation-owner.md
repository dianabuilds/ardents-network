---
id: R-084
title: What durable publication/generation owner gives M9 one crash-atomic Instance hand-off without retaining the H3 Endpoint action union?
status: open
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

## Disposition

**Open.** No M9 publication writer changes until an accepted record names its
format and recovery rule. This question does not select Custody or local IPC.
