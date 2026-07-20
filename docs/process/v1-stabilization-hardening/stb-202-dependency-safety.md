# STB-202 Dependency Safety — Capability Cryptography

Date: 2026-07-19

## Decision

No new cryptographic dependency is required for capability resolution,
selector derivation, or recipient-bound grant delivery.

| Function | Selected implementation | Posture |
| --- | --- | --- |
| grant signatures | standard `crypto/ed25519` | existing identity primitive |
| selector derivation | standard `crypto/hkdf`, `crypto/hmac`, `crypto/sha256` | RFC 5869 / HMAC, no added module |
| recipient-bound grant delivery | Go 1.26 `crypto/hpke` | standard-library RFC 9180 implementation |
| HPKE KEM/KDF/AEAD | `DHKEM(ecdh.X25519())`, `HKDFSHA256()`, `ChaCha20Poly1305()` | exact protocol suite |
| capability-store encryption | `golang.org/x/crypto/chacha20poly1305.NewX` | already selected at patched `x/crypto v0.52.0` |
| transactional state | bbolt through `internal/persistence` | existing mature engine |

## Alternatives Rejected

- `github.com/cloudflare/circl/hpke`: maintained and RFC 9180-capable, but it
  would add a large foundational crypto module when the current Go toolchain
  supplies the required stable primitive.
- older standalone Go HPKE projects: smaller support surface and unnecessary
  compatibility/security ownership.
- handwritten HPKE or Ed25519 conversion: prohibited because protocol and key
  conversion details are security-critical and standard support exists.

## Support And Security Posture

- minimum toolchain remains the repository's selected Go 1.26.5;
- Go standard library has the same BSD-3-Clause family posture as the
  toolchain and follows the Go security release process;
- `x/crypto v0.52.0` is already in the remediated graph and the active scanner
  register is reconciled in STB-103/STB-105;
- the selection does not enable QUIC, WebRTC, DTLS, or another network
  foundation;
- `govulncheck` and all Phase 0 gates remain mandatory after implementation.

## Gate

Passed for implementation. Any change of HPKE suite, crypto module, or
secret-store foundation reopens dependency safety review.
