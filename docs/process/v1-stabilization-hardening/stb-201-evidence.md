# STB-201 Evidence — Privacy Protocol And Migration Contract

Date: 2026-07-19

## Result

`docs/network-privacy-protocol.md` now defines the normative
`ardents-private/1` contract before privacy code is introduced.

The contract fixes:

- Identity-owned, recipient-bound capability grants and Policy admission;
- initial secure provisioning and recipient-bound HPKE delivery over an
  already authorized control relationship;
- channel-secret generation, routine selector rotation, member revocation by
  signed grant revocation plus fresh-secret redistribution, and recovery;
- HKDF-SHA256/HMAC-SHA256 opaque selector derivation and the exact Waku content
  topic shape;
- a fixed 72-byte outer header, XChaCha20-Poly1305 protection, associated-data
  binding to both Waku topics, deterministic protobuf inner framing, and
  Ed25519 sender identity signatures;
- message classes and domain ownership for discovery/publication and Data
  Substrate request/response traffic;
- time, size, padding, durable replay, bounded capacity, and stable failure
  taxonomy;
- Relay, Store, Filter, and Lightpush threat/control mapping;
- explicit traffic-correlation non-claims and compromised-capability behavior;
- a coordinated hard cut from the three readable technical-alpha topics with
  no dual publication, legacy query, bridge, decoder, or plaintext fallback.

DEC-STB-006 records the security and migration decision. Privacy requirements,
architecture, and persistent-state security documents link to the normative
contract.

## Current-Path Mapping

The code audit found the complete current product wire set:

- `ardents/1/discovery-record`: node presence, service publication,
  withdrawal, and discovery-fed source truth;
- `ardents/1/blob-request`: blob fetch request;
- `ardents/1/blob-response/<requester>/<request-id>`: success or terminal
  response.

The protocol maps these to encrypted inner classes on one opaque selector per
capability generation, so neither topic nor response suffix exposes class,
principal, blob, or request meaning.

## Checks

- threat model covers each enabled Waku role, endpoint-only observers,
  unauthorized operators, authorized/compromised holders, revocation limits,
  denial of service, and traffic-correlation limits;
- every current message class has one semantic owner and required capability
  scope;
- initial trust has no circular public inbox assumption;
- no new network foundation or custom cryptographic primitive is proposed;
- exact source-of-truth document paths exist;
- test catalog remains valid: 112 tests, 26 scenarios, 0 binding/doc/issues.

## Gate Result

Passed. STB-202 may implement capability resolution and deterministic selector
vectors, but must first complete dependency safety for HPKE/secret persistence
and may not weaken the protocol's hard-cut or failure rules.
