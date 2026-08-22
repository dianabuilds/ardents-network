---
id: R-069
title: What can a private naming control result honestly assert before Epoch installation?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-069 — Namespace submission result

## Decision this unlocks

G2-F024 and F029 require a result vocabulary that does not turn a Gateway's
volatile local transition result into authenticated current Namespace state.

## Current contract

R-047/ADR-0014 retain the bounded OHTTP exchange; R-057/ADR-0020 make a
threshold-attested Epoch materialization the source of current Name proof.
Current control returns `accepted`, a generation/revision, and unsigned Record
bytes after mutating a private in-memory map, although no Epoch was installed.

## Hypotheses

- **H1:** control returns only `submitted` or `denied`; submission is a
  bounded volatile Gateway fact and never current Name state.
- **H2:** preserve `accepted` and opaque state bytes as a client success.
- **H0:** call every valid control operation current before materialization.

## Evaluation criteria

The result must not assert a signature, durable commit, epoch membership,
generation, revision, or Target availability without a verified current proof.
It must preserve fixed private exchange bounds and leave ordinary denial
observable without a plaintext fallback.

## Findings

- **Inspection:** `control.Apply` mutates only memory; `Store.Commit` is a
  separate threshold path and is the sole durable current publisher.
- **Inference:** the only honest immediate fact is that the Gateway accepted a
  submission into its own volatile pending state. Any stronger result is
  indistinguishable from a fabricated Gateway reply to the client.

## Recommendation

Accept H1. Replace `accepted` with `submitted` and remove generation,
revision, and state bytes from the private control result. A caller renders
only a pending/submitted outcome; a later independently verified Namespace
proof is required for current success.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** This narrows the receipt claim; it does not yet implement the
durable pending-to-current transaction required to close F024.
