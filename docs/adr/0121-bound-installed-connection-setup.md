---
status: accepted
date: 2026-10-14
---

# Bound installed Connection setup to thirty seconds

## Context

The Product Owner approved this bounded amendment on 2026-10-14. With full
installed-generation observations, an eight-second capsule expired during
Publisher validation, before Responder acquisition. Reader's separate
forty-five-second operation bound did not extend that capsule.

## Decision

Amend ADR-0083's unpaired JOIN ceiling and the corresponding capsule setup
ceiling to thirty seconds. Select one absolute capsule deadline before sealing,
clamped to the original caller, Work Safety, registration and recovery bounds.
Receiving Introduction, private recipient and Connection enforce that same
ceiling. Unpaired JOIN remains bounded by the earlier original request deadline
or thirty seconds from its original reservation. There are no renewals or retries.

Use one sixty-second absolute Reader operation bound, retaining its five-second
recovery cutoff, fifteen-second input bound, ten-second bootstrap and role TLS
opening bounds. Full observations, nonce admission before Responder opening,
quotas, irreversible spend and joined cleanup remain required.

Wire encodings and identities do not change. This is coordinated adoption by
the closed candidate's new owners, not mixed-version interoperability: earlier
ten-second owners may refuse longer requests. Do not add fallback, convert
persisted roots or claim old-runtime support. Earlier bounded requests remain
valid under their own original deadlines.

## Consequences

Pending work may occupy the same bounded resources longer, increasing the
duration of denial-of-service exposure. Capacity and Admission lifetimes do
not increase. Earlier cancellation always wins. This decision authorizes the
policy, not a claim that installed exchange succeeds within it. Required
evidence includes ceiling/earlier-bound refusal, both Carriers, complete
installed exchange and original joined termination.
