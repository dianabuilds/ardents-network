---
status: accepted
date: 2026-08-19
---

# ADR-0013 — Stage 6 cryptographic suite and interface boundary

## Context

Horizon 3 Stage 6 (`horizon-3-stage-6-brief.md`) requires three
cryptographic operations that are not yet fixed at the technology
level: a Name Authority signing scheme, a Recovery Policy threshold
scheme, and a resolver query-hiding scheme. R-039 fixes the product
contract; R-044 freezes the suite selection.

The H3 project has already selected Ed25519 (R-013, R-009, R-035) and
OHTTP (R-026). A new threshold scheme is required for the
`Recovery Policy` because Ed25519 cannot express a native `t`-of-`n`
recovery.

Per `AGENTS.md`, technology selection that creates meaningful lock-in
requires a research record and an accepted ADR. The Stage 6 suite is
such a selection.

## Decision

Stage 6 uses the following suite, recorded in R-044:

- **Name Authority signing:** Ed25519 (RFC 8032), 32-byte public key,
  64-byte signature, deterministic. Implemented in the Go standard
  library `crypto/ed25519`. Already in H3 supply (R-013, R-009,
  R-035).
- **Recovery Policy threshold:** BLS12-381 minimal-signature-size
  variant, 48-byte public key on G1, 96-byte signature on G2.
  Implemented by `golang.org/x/crypto/bls12381` (Go team, BSD-3-Clause).
  No new trust root: the threshold is operated by the existing Name
  Authority holder.
- **Resolver query hiding:** OHTTP. Reuses the R-026 supply
  `openpcc/ohttp v0.0.80` at commit `79bec89d8042` with the explicitly
  raised CIRCL and Go `x/*` security versions already verified for
  Gate C.
- **Hash:** SHA-256 (FIPS 180-4) for canonical encodings, the order
  key, and all commitments. No other hash is permitted.
- **KDF:** HKDF-SHA-256 (RFC 5869). Used only for connection-only
  keying material; no local primitive.

## Replaceable interface boundary

S6.2, S6.4, and S6.5 consume the suite through the following Go
interface boundary. The interface is the implementation pattern; a
future ADR may swap a concrete type without rewriting the slices.

```go
// In internal/nameauthority/sig.go (or equivalent).

type Signer interface {
    Scheme() string
    PublicKey() []byte
    Sign(payload []byte) ([]byte, error)
    Verify(publicKey, payload, signature []byte) error
}

type ThresholdVerifier interface {
    Scheme() string
    Aggregate(publicKeys [][]byte) (aggregateKey []byte, err error)
    VerifyAggregate(aggregateKey, payload, aggregateSig []byte) error
}

type QueryHider interface {
    Hide(origin, query []byte) ([]byte, error)
    Reveal(reply []byte) ([]byte, error)
}
```

Concrete types implementing these interfaces in S6.0 are
`Ed25519Signer`, `BLSThresholdVerifier`, and `OHTTPQueryHider` (the
last reusing the R-026 supply).

## Consequences

Positive:

- Every Stage 6 cryptographic import is justified by an accepted
  ADR. The repository's research discipline rule is preserved.
- The H3 supply is reused where it already exists; the only new
  dependency is the Go-team-maintained `golang.org/x/crypto/bls12381`.
- The interface boundary lets a future ADR replace a concrete type
  (for example, a new threshold scheme or a new query-hiding
  primitive) without rewriting S6.2, S6.4, or S6.5.
- SHA-256 + HKDF-SHA-256 are the only hash and KDF primitives, which
  removes a class of accidental primitive drift.

Negative:

- The BLS12-381 dependency is new to the H3 supply. It is Go-team
  maintained, but its security history is younger than Ed25519's.
  The dependency must be re-reviewed on every `golang.org/x/crypto`
  advisory.
- A future post-quantum transition (Dilithium, Kyber) is not covered
  by this ADR. A later research record and a superseding ADR are
  required to add it.
- OHTTP is the only query-hiding primitive. A future change requires
  a new research record and a new ADR.

## Compliance

- The suite is not a production cryptographic audit. The existing
  `docs/development/dependencies.md` policy continues to apply.
- A Stage 6 slice that imports a primitive not in this ADR is a
  violation. A new research record and a superseding ADR are required
  to add it.
- A Stage 6 slice that builds a custom cryptographic primitive is a
  violation. No local primitive is permitted.
- A future replacement of a concrete type must keep the interface
  boundary intact unless a superseding ADR explicitly changes it.

## Supersedes

- This ADR records the Stage 6 cryptographic suite only. It does not
  supersede R-013 (Carrier Lab), R-026 (OHTTP), or R-035 (Bridge
  state); those records retain their supply identities.

## References

- R-044 — Stage 6 cryptographic suite research record.
- R-013 — Carrier Lab technology candidates.
- R-009 — hostile bootstrap and Bridge entry.
- R-026 — Private Resolution Adapter selection.
- R-035 — H3 Bridge state.
- ADR-0009 — Go project foundation.
- ADR-0012 — Stage 5 camouflage selection.
- `horizon-3-stage-6-brief.md` — Stage 6 implementation brief.
- RFC 8032 — Ed25519.
- IETF `draft-irtf-cfrg-bls-signature-04` — BLS12-381 signature scheme.
- RFC 5869 — HKDF.
- FIPS 180-4 — SHA-256.
