---
id: R-077
title: What Entry Invite and adjacent candidate format may serve ardents-interactive-route-v1?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-077 — Entry Invite v1

## Decision this unlocks

Give M7 a signed Entry authorization and candidate boundary compatible with
R-076. It must replace the H3 Bridge Invite/WebTunnel envelope rather than
silently retaining either under `entry`.

## Current contract

R-076/ADR-0024 select `ardents-interactive-route-v1`: an endpoint may connect
directly only to its authenticated adjacent Entry/Node over TCP/TLS 1.3, while
Route selection remains endpoint-local. ADR-0005 fixes the Initiator Role
Domain and finite Entry exposure. Current authenticated Network State already
has the exact Node identity, signing key, family, record/domain proof digest,
validity, assignment, and literal endpoint needed to authenticate that adjacent
candidate. It is not a public directory.

The H3 `ardents-h3-bi1` Invite carries a WebTunnel candidate envelope and is
bound to an H3 profile and Bridge transition frame. R-076 retires all three.

## Hypotheses

- **H1:** a compact signed Invite which names only an authenticated State
  candidate can authorize a bounded Entry without carrying transport-specific
  discovery material.
- **H2:** the Invite must carry a separate carrier envelope or use the H3
  envelope as a compatibility format.
- **H0:** no Invite can preserve State authority and bounded replacement.

## Evaluation criteria

The format must bind exact Profile/State/Duty validity, permit two finite slots
and one replacement, exclude a complete Route/Target/Service identity, support
strict decode and replay resistance, and fail closed if State, role, time,
signature, or local conflict checks disagree. Its address and TLS key must be
obtained from current authenticated State, not from the Invitation or DNS.

## Evidence plan

### Primary sources

- ADR-0005, ADR-0006, ADR-0024, R-035, R-076, and Network State's current
  candidate projection, inspected 2026-08-23.
- [RFC 8446: TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446.html), accessed
  2026-08-23; the adjacent carrier is an authenticated protected stream, not
  an authority/discovery mechanism.

### Experiment

M7 behavior tests must cover canonical round trip; truncation/mutation;
signature/key/State/domain/time/local-conflict failure; replay; slot
replacement during and after an active attempt; reopen/interruption; and a
native TCP/TLS opener whose peer key equals the State candidate. Any one of
those failures is a rejection of this decision's implementation.

### Failure scenarios

- an Invite supplies an address, TLS key, Route, Target, or WebTunnel profile;
- State rotates/revokes the candidate after Import but before use;
- a valid candidate appears in a non-Initiator Role Domain or conflicts locally;
- a replacement becomes active while work through the previous slot is live;
- a retry escapes the two-slot/four-contact bound; or
- malformed/unknown-generation bytes are treated as an older compatible Invite.

## Findings

- **Inspection:** State's current candidate projection authenticates exactly the
  Node/family/key/record/domain/assignment/validity facts that H3 previously
  repeated in an Invite, and gives its literal TCP endpoint and public key to
  the endpoint-local selector.
- **Inference:** repeating a carrier envelope creates a second mutable
  reachability authority and lets a H3 WebTunnel detail survive R-076. A
  reference-only signed Invite is both smaller and more fail-closed.
- **Assumption:** address/key changes are represented by a new authenticated
  State generation; an Entry that no longer matches it is unavailable rather
  than contacted at an old address.

## Selected format

Choose **Entry Invite v1** with the following canonical bytes:

```text
"ardents-entry-invite-v1" || uint16(body-length) || body || Ed25519-signature
```

`body` is fixed-order big-endian fields:

```text
uint16(version=1)
network-id[32] || epoch[8] || epoch-digest[32] || profile(short ASCII, 1..63)
issuer-key-id[32] || node-id[32] || family-id[32]
record-digest[32] || domain-proof-digest[32] || assignment-not-after[8]
not-before[8] || not-after[8] || slot-generation[1] || slot[1]
replacement-present[1] || replacement-invite-id[32 if present]
```

The signature input is
`"ardents-entry-invite-signature-v1\x00" || body`; the Invite ID is
`SHA-256("ardents-entry-invite-id-v1\x00" || body)`. All scalar encodings are
canonical; an unknown version, non-ASCII/non-canonical Profile, zero required
identifier, surplus bytes, invalid time ordering, invalid slot/generation, or
unrecognized replacement form is invalid, never a compatible fallback.

Validation derives the candidate address and TLS Ed25519 pin solely from the
current authenticated State projection keyed by `issuer-key-id`. It requires
exact Network/Epoch/Digest/Profile, Initiator Domain, identity/family/record/
domain/assignment equality, fresh State, time confidence, local no-conflict,
and all validity intervals. The Invite contains no address, certificate, TLS
name, carrier envelope, Target, Service Name, Route, or retry instruction.

There are exactly two slots. Generation 1 fills a never-used slot; generation 2
references its slot's generation-1 Invite. A valid replacement drains while an
attempt is live and becomes active only after terminal settlement. There is no
third generation, slot reset, or old-format reader.

## Options

| Option | Disposition |
|---|---|
| State-referenced Entry Invite v1 | choose: one authority for candidate endpoint/key and a compact signed bounded authorization. |
| Carry native TCP/TLS endpoint/key in the Invite | reject: duplicates State reachability authority and makes stale address use easier. |
| Retain H3 Invite/WebTunnel envelope | reject: contradicts R-076 C0 retirement. |

## Recommendation

Choose H1 with high confidence and create ADR-0025. The strongest argument
against it is that an Entry cannot work across a State refresh that removes its
candidate. That is intentional fail-closed behavior; contacting a stale Entry
would preserve revoked or changed authority.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
ADR-0025 records the signed protocol format. M7 creates `internal/entry` with
this sole reader/writer and deletes `internal/bridge` and `internal/camouflage`.
No H3 Invite/state migration reader is retained: historical H3 evidence remains
C4 provenance, while maintained Entry state starts C0 under its own root.
