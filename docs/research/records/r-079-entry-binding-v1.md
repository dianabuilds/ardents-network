---
id: R-079
title: How does a User authenticate one User-to-Initiator Route leg without a User State identity or an unverified Entry capability?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-079 — Entry Binding v1

## Decision this unlocks

Correct the unannounced R-078 Route wire before M8 connects it to a runtime.
The first C-5 leg is `User -> Initiator`; User is not a State Node and may not
be encoded as one. The Initiator must nevertheless admit only a current signed
Entry Invite and bind its mTLS peer to the exact one-use Route attempt.

## Current contract

ADR-0005/R-077 give the User a bounded State-referenced Initiator Entry Invite,
not a permanent User identity or a complete Route. R-076 requires mutually
authenticated TLS 1.3. R-078's initial LegBinding incorrectly assumed both
ends were State Nodes; it has no peer deployment or compatibility observer.

## Hypotheses

- **H1:** a separate EntryBinding carries the signed Invite, Route attachment
  ID, and a digest of a fresh self-signed mTLS client public key; the Initiator
  verifies the Invite against current State and its own key before admitting.
- **H2:** encode User as a synthetic State Node or use a persistent User key.
- **H0:** postpone native Entry use until an Application identity system exists.

## Evaluation criteria

The binding must prove current Initiator authorization, prevent a stolen Invite
from being replayed on another TLS connection, reveal no Target/Service/Route,
avoid a User identity authority, and preserve finite Invite/retry/attachment
bounds. A malformed, expired, wrong-State, wrong-Initiator, wrong-key, or
replayed binding must fail closed without fallback.

## Evidence plan

### Primary sources

- ADR-0005, ADR-0024, ADR-0025, R-077, R-078, and `CONTEXT.md`, inspected
  2026-08-23.
- RFC 8446, accessed 2026-08-23: TLS authenticates its peer certificate and
  handshake parameters, but application protocol semantics remain caller-owned.

### Experiment

M8 must run a real mutually authenticated TLS 1.3 Entry listener. Tests cover
valid presentation, every Invite/State/key/attempt mutation, concurrent replay,
restart interruption, capacity refusal, and confirmation that no stable User
identifier is persisted or exposed to downstream Nodes.

## Findings

- **Inspection:** Entry Invite v1 is signed by the State Initiator key and
  binds its State identity, profile, validity, and lineage, but has no User key.
- **Inspection:** the current Route tracer's client certificate is a local
  carrier credential, not a State User identity.
- **Inference:** the certificate public-key digest is sufficient channel
  binding for a fresh attempt; it must never be promoted to Persona, Device,
  Credential, or State authority.

## Selected format

R-078 is amended as follows. `kind = 1` is **EntryBinding**, not LegBinding:

```text
uint16(version=1) || uint8(kind=1) || profile(short ASCII, exact v1)
network-id[32] || epoch[8] || epoch-digest[32] || attachment-id[32]
initiator-node-id[32] || not-after-unix[8]
client-tls-public-key-digest[32]
uint16(entry-invite-length) || entry-invite
```

The Invite is the exact R-077 bytes, at most 1024 bytes. The User creates a
fresh Ed25519 TLS certificate for this attempt and sends EntryBinding only
after TLS 1.3 mutual handshake. The Initiator hashes the peer certificate's
Ed25519 public key, compares it to `client-tls-public-key-digest`, validates
the embedded Invite using its current State/duty/time facts and its own issuer
key, checks exact network/epoch/digest/profile/node/expiry/attachment bounds,
then atomically consumes `(invite-id, attachment-id, client-key-digest)` before
allocating work. It retains only the finite replay outcome until the Invite or
attempt expires; it stores no User identity.

`kind = 2` becomes **LegBinding** and is only Node-to-Node: its existing R-078
body remains unchanged. `kind = 3` becomes SealedIntroduction. An EntryBinding
cannot be forwarded beyond Initiator; an Initiator constructs the next
Node-to-Node binding from its local adjacent Route duty without exposing Invite
or client-key digest.

## Options

| Option | Disposition |
|---|---|
| Invite capability plus fresh TLS key digest | Choose: authenticates one bounded Entry use without a User authority. |
| Synthetic State Node or persistent User certificate | Reject: collapses separate Person/Device/Persona/transport concepts and adds an unselected identity system. |
| Server-only TLS or unchecked Invite | Reject: violates selected mutual authentication and permits unauthenticated admission/replay. |

## Recommendation

Choose H1 with high confidence. The strongest objection is operational cost of
one fresh client certificate per attempt; it is bounded by Entry's four-contact
attempt and avoids the much larger authority cost of a persistent User identity.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
ADR-0027 records the correction. M8 must replace the unannounced R-078 codec
kind map and add the stated process tests before Route consumer cutover.
