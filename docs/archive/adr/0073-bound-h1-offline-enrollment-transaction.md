---
status: proposed
date: 2026-09-05
extends: ADR-0071; ADR-0072
---

# ADR-0073 — Bound H1 offline enrollment as one transaction

## Context

ADR-0071 and ADR-0072 select an offline, recipient-bound enrollment journey
and a separate challenge-bound time witness. They deliberately do not select
the command, persisted records, canonical grammars, or receiver boundaries
needed to implement that journey. The existing State clock is one scalar
observation and the current static enrollment verifier has no authority to
create a recipient, receive receipts, persist a replay floor, or deliver a
sealed response. Extending either by an implicit operator file or a wall-clock
fallback would bypass the selected security model.

## Proposed decision

Treat one H1 enrollment as a finite transaction with three distinct owners:

- `ardents-enrollment` is an offline command artifact. It canonicalizes one
  Endpoint request, journals its digest and exact sealed response exclusively,
  and may return the same response only for the same request digest. Its
  enrollment issuer key and time-witness signing key remain separate. It does
  not issue Entry, Source, State, Transit, or Service credentials.
- State is the only time-confidence receiver. It verifies the selected
  witness key, fresh Endpoint challenge, bounded interval, request-window
  binding, durable replay floor, and restart refusal; it then supplies only a
  bounded verification-time value to Source. Source retains its CA, hostname,
  leaf-pin, expiry, and no-wall-clock-fallback checks.
- Endpoint owns its request ephemeral delivery key and derives private runtime
  facts only after it accepts the sealed response. Entry and Source each issue
  and verify their own authorized receipt; the transaction may carry their
  exact receipts but cannot synthesize either one.

The implemented journey must make interrupted delivery, a restarted Endpoint,
a copied response, a different recipient/generation, a delayed witness answer,
and a reopened journal conflict explicit refusals. It must not treat an absent
record, untrusted local clock, or v1 Route/Entry material as recovery.

## Open contract required before implementation

This proposal intentionally does not choose cryptographic envelope formats or
numeric policy values. An accepted follow-on contract must select and test:

1. the canonical request, receipt, witness, and sealed-response grammars,
   including their versioning and fixed public-key identifiers;
2. the finite boot-local request and delivery window, witness interval limits,
   durable replay-floor representation, and exact restart behaviour;
3. the command and package ownership, artifact inventory binding, journal
   crash/reopen semantics, Endpoint persistence boundary, and maintained
   command/process test profile.

Until those choices are accepted, this document has no runtime effect, does
not activate a second C0 implementation issue, and does not authorize a
fallback, new dependency, wire compatibility claim, or automatic renewal.
