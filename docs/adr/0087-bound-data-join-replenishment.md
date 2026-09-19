---
status: accepted
date: 2026-09-14
---

# Bound data-JOIN replenishment to its admitted channel

## Context

The retained ten-minute workload requires at least 46,875,000 useful bytes per
active Connection (the fixed scheduler offers 47,343,750 with its one-percent
measurement margin). Its data-JOIN channel currently has a 32 MiB lifetime allowance.
ADR-0085 permits replenishing forwarding parents but excludes data-JOIN, so
the selected workload cannot finish on a retained Connection. The Product Owner
approved extending the same bounded replenishment semantics to data-JOIN.

## Decision

An already admitted data-JOIN channel may receive a fresh class-2 ADMIT only
on lane zero. Its existing lane-1 JOIN, counterpart, context, secret, original
HELLO, receiver, authenticated TLS exporter and original HELLO/class-2 absolute
expiry stay fixed. The JOIN request deadline continues to bound pairing setup;
it does not become the lifetime of paired data.

Charge the complete ADMIT frame to the previous reserve first. Verify the fresh
token, reserve the additional hosting work and finite termination before durable
spend, then set this side's remaining reserve to exactly 32 MiB. Never add
32 MiB to the previous remainder, refund earlier debits, or replenish the other
side or any forwarding ancestor. Each receiving owner accounts its own bytes.

A closed, expired, unpaired or revoked channel cannot be renewed. Refusal does
not extend authority or restore a spent token. Hold accepted host reservations
through actual stream termination and joined cleanup. An interrupted verification
or cleanup remains a failed attempt in qualification evidence.

All other channel types and every child lane still forbid post-initial ADMIT.
This extends ADR-0085 only for the already selected data-JOIN channel, without
a new frame kind, profile fallback, cryptographic primitive or public protocol
compatibility claim.

## Consequences

Endpoint obtains fresh tokens through normal issuance. Node reserves against its
actual shared provider period before spending them. Existing peers that refuse
data-JOIN replenishment cannot complete this workload; deploy one candidate
consistently across the closed qualification network.

Acceptance still requires real installed-worker, Route, Service and hosting
measurements. This decision is not an installed-host or performance receipt.
