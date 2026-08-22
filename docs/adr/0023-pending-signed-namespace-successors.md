---
status: accepted
date: 2026-08-23
supersedes: none
---

# ADR-0023 — Persist signed Namespace successors before materialization

## Context

Namespace control authenticates a predecessor transition but currently yields
an unsigned in-memory Record. Current Namespace materialization accepts only
Authority-signed Records, so submission, durable history, and threshold-current
state have no authenticated connection.

## Decision

A private Namespace control submission carries an Authority-signed successor
Record. Namespace derives the successor from the authenticated transition and
its sole decision time, requires exact equality and verifies the existing
Ed25519 Record signature before atomically recording pending state. Only a
threshold Epoch materialization selected from that durable pending state may
advance current Namespace state; a submission receipt remains non-current.

The existing Record signature transcript and compact current-proof structure
remain the current artifact. Internal control and pending-journal encodings are
C0 tracer formats under R-067.

## Consequences

- Gateway cannot invent a durable/current Record or convert an unsigned result
  into Authority state;
- Custody prepares and signs an exact successor before submission;
- restart restores verified pending state but never promotes it without Epoch
  installation; and
- Store installation becomes a Namespace-owned transaction rather than an
  arbitrary caller-supplied corpus.

## Compliance

[R-070](../research/records/r-070-namespace-pending-successor-record.md)
contains the decision evidence and required failure/restart tests. This does
not change the selected Ed25519 algorithm, OHTTP privacy boundary, Recovery
threshold authorization, or threshold materialization authority.
