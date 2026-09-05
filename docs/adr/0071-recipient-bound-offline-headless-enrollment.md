---
status: accepted
date: 2026-09-05
research: R-144
extends: ADR-0025; ADR-0027; ADR-0053; ADR-0062; ADR-0064
---

# ADR-0071 — Recipient-bound offline Headless enrollment

## Context

C0-05 requires a second enrolled Endpoint to open an explicit Target Link
without hand-authored runtime JSON, fixture keys, an alternate Route selector,
or a permanent bootstrap service. The current enrollment verifier authenticates
only a static artifact; it creates neither a recipient identity nor Source
authorization. A shared signed Entry Invite is transferable to a distinct
attachment/client-key tuple, so manifest pinning or encryption alone cannot
make it an Endpoint-bound capability.

Fresh and restarted Endpoints also need Time Confidence before accepting
time-bounded State, Entry, Source, and Service inputs. An NTS bootstrap begins
with TLS/X.509 verification and therefore cannot be the only cold-start answer
when the local wall clock is untrusted. A stale signed time response is not
freshness evidence merely because it is signed.

## Decision

Select H1: an offline, purpose-scoped Endpoint-enrollment issuer approves one
canonical Endpoint request and commits one exact authenticated response before
delivery. The response is sealed only to the request's ephemeral delivery key.
The issuer has an exclusive finite journal and can reconcile only the same
approved request digest; a different digest, recipient, or generation is a
conflict. It is not a State, Route, Target, Release, Service Credential, Entry,
or Source authority and does not run as a standing network service.

Select a separate offline challenge-bound time witness. Its public trust key is
purpose-separated from the enrollment issuer. A response signs a fresh,
Endpoint-generated challenge and a bounded time interval. The Endpoint accepts
that response only during the same boot-local, finite monotonic request window
that began before the challenge was exported; delivery delay consumes that
window. A restart or loss of monotonic continuity invalidates the pending time
answer and requires a fresh challenge and response. The witness is neither a
Network State authority nor an enrollment issuer, and it supplies no Route,
Target, Source, or Credential input.

Entry and Source remain their own authorities. A later versioned Entry Invite
must commit, in its issuer-signed body, to the enrolled Entry-recipient public
key and finite generation; the receiving Initiator must require a fresh
channel-bound proof of that key before allocating an attachment. Source client
authorization remains CA-verified and leaf-key-pinned. The enrollment issuer
may coordinate approved public outputs but cannot manufacture either.

## Consequences

- A copied artifact, response, or decrypted Invite cannot create use under a
  different recipient key; copying the full protected private key remains
  Endpoint compromise and is not claimed preventable.
- The C0 operator performs an explicit finite request/approval/delivery
  ceremony. Lost keys, journals, or interrupted time delivery are terminal or
  require separately approved replacement; no empty-root recovery, automatic
  re-enrollment, or stale-good fallback exists.
- The existing static enrollment verifier remains pure. A later implementation
  needs explicit command, artifact, package-map, Entry format, Source policy,
  time-witness grammar, durable journal, and operational-owner decisions.
- No public enrollment, anonymity, independent operation, availability,
  automatic Source issuance, time-service availability, new dependency, or
  existing wire compatibility is claimed by this decision.

## Non-claims

This decision does not accept a command, package, wire grammar,
issuer key format, time protocol, concrete delay bound, source, Entry
authorization procedure, or C0-05 implementation. It does not close C0-05 or
authorize a second C0 implementation issue. Those choices require accepted
follow-on ADRs and owner contracts before code changes.
