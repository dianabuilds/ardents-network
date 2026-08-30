---
status: accepted
date: 2026-08-30
supersedes: 0039-state-authorized-transit-grants-v1.md (dynamic signer authority and custody only); 0047-dynamic-membership-transit-grants.md (online signer and participant lifecycle only)
---

# ADR-0062 — Scope online Transit Grant signing away from State authority

## Context

ADR-0039 and ADR-0047 require a current State-authorized Transit Grant for
dynamic Introduction admission. The maintained issuer currently retains a raw
Network State authority private key, while ADR-0053 fixes the functional-alpha
State authority as a separately encrypted Product Owner-held seed which must
not enter an online Node or VPS. That composition is not deployable honestly.

A supported headless participant also needs a restart-safe answer when an
issuer may have created a one-use Grant but the Endpoint did not receive the
response. Treating every ambiguity as a new request can multiply a finite
issuer budget; treating every crash as silent loss cannot support the selected
restart/recovery journey. The present response also cannot distinguish
exhaustion, withdrawal, and ordinary unavailability inside the encrypted
exchange.

## Decision

One authenticated Network State generation may declare exactly one
purpose-scoped online Transit Grant signing public key in the selected
`transit-issuance` profile. The profile is authenticated by State and by its
assigned issuer Node, but the corresponding private key is generated, stored,
and used only by that issuer duty. It is not an Epoch authority and is never
accepted for State, Route selection, Target, Namespace, Release, enrollment,
or any generic signing operation. Network State and Epoch private keys never
enter the issuer process or its durable root.

The existing Transit Grant v1 bytes and signature domain remain unchanged.
Receiving Introduction and Responder duties verify the Grant only against the
single current purpose-scoped Grant signer projected by their authenticated
State. Historical State-authority-signed Grant evidence remains readable under
its historical profile, but it is not an accepted headless-candidate issuance
path.

Each credential request adds one fresh, Endpoint-generated 32-byte Request ID.
The signer-side duty owns one finite durable budget for one exact State duty
scope and one bounded idempotency ledger. Before signing, it atomically records
the exact request digest and a reserved Grant ID. Recovery completes that same
request deterministically; the same Request ID and request digest return the
same Grant, while a reused Request ID with different bytes returns
`unavailable`. A new valid request consumes exactly one budget unit. Duty
withdrawal prevents new reservations; budget exhaustion cannot be replenished
inside the active duty.

Every syntactically valid OHTTP exchange returns one fixed-size, versioned,
encrypted outcome: `issued`, `exhausted`, `withdrawn`, or `unavailable`.
`issued` carries exactly one Transit Grant v1. The other outcomes carry no
route, Target, participant, budget count, or operator detail. Transport,
decapsulation, authentication, and malformed-message failures remain locally
classified as unavailable and cause no fallback.

The Endpoint owns the corresponding at-most-once lifecycle. Before exchange it
durably records the Request ID, exact target-free request tuple, one-use TLS
key, and State-duty identity in its protected state. It reconciles an ambiguous
or interrupted exchange only by resubmitting those exact bytes and Request ID;
it never creates a replacement request implicitly. A verified `issued` result
promotes that exact key/Grant pair to ready. Once presentation to the receiving
Node begins, the Endpoint durably burns the attempt on every success, refusal,
timeout, cancellation, or crash and erases its private key. No Application
operation is replayed automatically. A State successor, explicit duty
withdrawal, expiry, or an authenticated terminal outcome erases the pending
key and records the bounded terminal result.

## Consequences

- Compromise of the online signer can spend only the remaining budget of its
  current authenticated duty and mint matching Transit Grant v1 records. It
  cannot sign an accepted State successor or obtain the State root key.
- A selected Initiator and issuer may collude, correlate timing, withhold
  service, or exhaust the global duty budget. This alpha claims no unlinkable
  per-participant quota, hostile-peer membership control, anonymity, or public
  availability.
- The issuer ledger is bounded by its finite duty budget and expires with the
  duty. Invalid requests and ordinary transport failures do not create durable
  entries. Durable corruption or rollback fails the duty closed.
- Reconciliation prevents duplicate budget consumption; it does not prove
  whether a receiving Node spent a Grant after presentation began. The Endpoint
  therefore burns every ambiguous presentation rather than retrying it.
- The State-authenticated issuer profile and credential request/outcome grammar
  are versioned changes. Route setup, Target, Descriptor v2 semantics, and
  Transit Grant v1 remain unchanged.

## Compliance

[R-128](../research/records/r-128-headless-participant-acquisition.md) records
the Product Owner decision and rejected alternatives. The complete operational
contract is [Transit Grant acquisition](../technical/transit-grant-acquisition.md).
ADR-0053 remains the State root-custody owner; ADR-0047 remains the dynamic
membership-level Introduction decision outside the signer and lifecycle parts
superseded here.
