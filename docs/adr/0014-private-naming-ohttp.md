---
status: accepted
date: 2026-08-20
---

# ADR-0014 — Authenticate and hide Stage 6 private naming exchanges

## Context

Stage 6 must prevent one ordinary Node from receiving both Endpoint location
and an exact Service Name or publicly testable lookup value. Role assignment
alone cannot hide plaintext from an endpoint-adjacent Node. R-026 measured an
RFC 9458 Adapter and exact dependency closure for Gate C, while R-046 defines a
field-level role split. R-044 remains open for threshold Recovery Authority
cryptography and must not be partially implemented by S6.2.

## Decision

Accept R-047 Option O1: use Go 1.26.6 standard-library Ed25519 with the exact
domain-separated transcripts defined by R-047, and promote
`github.com/openpcc/ohttp v0.0.80` at commit
`79bec89d804248df1a71a0f56c882b116579035d` and its registered raised closure
for one product-owned private naming exchange.

The naming Module owns canonical Authority keys and signed records, fixed
4096-byte plaintexts, fresh nonces and
HPKE contexts, network/operation/name/deadline/response binding, replay state,
authenticated common Gateway configuration, Role Domain checks, bounded
results, and no-fallback behavior. A thin OHTTP Adapter owns encapsulation and
decapsulation only. Relay and Gateway remain separate identities, families,
processes, and durable roots in maintained evidence.

The common Gateway configuration is signed by the Gateway Node's existing
Ed25519 identity and verified against its authenticated Network State record;
R-047 freezes the exact transcript. A self-computed configuration digest is not
proof, and the Module does not expose a serialized execution plan as an
authority source.

This decision does not select Recovery Policy threshold cryptography, claim
ordering, Anonymous Cost, Namespace replication, public
wire discovery, Gateway operators, or a general HTTP proxy. ADR-0013 remains
withdrawn and R-044 remains open.

## Consequences

- The exact OHTTP closure becomes a maintained product dependency as well as a
  Gate C laboratory dependency; `dependencies.md` must name both owners.
- S6.2 can authenticate Name Records and test the R-046 resolution slice
  without inventing cryptography or a less-private fallback.
- Gateway key configuration is authenticated, finite, common to an anonymity
  set, and pre-provisioned. Unique/per-client configuration and runtime
  discovery are forbidden in this slice.
- OHTTP does not hide timing/volume, low-entropy names, or popularity and does
  not protect against Relay/Gateway collusion, Correlated Control, endpoint
  compromise, Sybil families, or a Broad Traffic Observer.
- A version, dependency, suite, key-distribution, or public-wire change requires
  new research. Failure selects `stop`, not first-party crypto or plaintext.

## Compliance

- [R-047](../research/records/r-047-stage-6-query-hiding.md) contains the
  production-selection evidence and recommendation.
- [R-046](../research/records/r-046-role-matrix.md) defines the role matrix.
- [R-026](../research/records/r-026-private-resolution-adapter.md) records the
  exact implementation and Gate C measurements.
- [RFC 9458](https://www.rfc-editor.org/rfc/rfc9458.html) defines OHTTP and its
  security/privacy limitations.
- `docs/development/dependencies.md` remains authoritative for the exact module
  closure and removal gates.
