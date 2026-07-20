# STB-202 Evidence — Capability Resolution And Selector Derivation

Date: 2026-07-19

## Result

Identity now owns a product-grade `ardents-private/1` capability service and
Policy is a mandatory admission boundary before private material can be used.
Network privacy derives deterministic Waku selectors and envelope keys only
from an admitted capability secret; endpoint or public-principal knowledge is
insufficient.

The delivered path includes:

- canonical signed grants and signed revocations with trusted-issuer,
  subject, scope, permission, version, lifetime, and generation validation;
- encrypted bbolt persistence under a separately supplied master key;
- HMAC-derived node-local opaque references, so different nodes do not share
  bearer handles;
- recipient-bound grant delivery using Go's standard-library RFC 9180 HPKE
  implementation with an identity-signed, time-bounded X25519 delivery-key
  attestation;
- deterministic HKDF-SHA256/HMAC-SHA256 selector and XChaCha envelope-key
  derivation;
- explicit stable failure codes for missing, not-yet-valid, expired, revoked,
  denied, untrusted-issuer, and invalid cases;
- non-sensitive status projection containing only scope, generation, state,
  and expiry.

## Security And Dependency Decision

`docs/process/v1-stabilization-hardening/stb-202-dependency-safety.md` records
the selection of `crypto/hpke` from Go 1.26. No new third-party foundation or
cryptographic primitive was added.

Grant, resolved-material, secret, revocation, and status formatting was tested
for redaction. JSON does not contain channel IDs, grant IDs, local capability
references, delivery secrets, selector material, envelope keys, or capability
secrets. Persistent storage does not contain recoverable plaintext secret or
subject material.

## Interoperability Evidence

Scenario NPI-001 passed through the canonical integration runner:

- report: `tests/.artifacts/reports/stb-202/summary.json`;
- two nodes use different storage master keys and therefore different local
  references;
- the same admitted signed grant produces the exact same selector and envelope
  key on both nodes;
- signed revocation rejects the old grant;
- a fresh secret and generation produce new material.

The fixed derivation vector is:

- topic: `/ardents/1/kaciboigy5ukazbvbf2ohtalvbpupr6k/proto`;
- envelope key:
  `f7592a38caf0e765d4707203a93c1af07fcd3e4ddde3c421cb5451d1e56d1971`.

## Checks

- focused capability/privacy/policy tests: passed;
- canonical NPI-001 integration scenario: passed;
- isolated `go test ./... -count=1`: passed;
- race tests for capability/privacy/policy: passed;
- `go vet ./...`: passed;
- code-size guard for Identity API/capability, Network privacy, and Policy:
  passed with no soft or hard breach;
- test catalog: 113 tests, 27 scenarios, 113 formal bindings, 0 issues;
- `govulncheck ./...`: unchanged single reachable GO-2026-4479 in Pion DTLS,
  with no fixed version; the existing TCP/TCP-WSS-only transport enforcement
  remains the documented compensating control.

One CLI watch deadline occurred only while the full suite competed in parallel
with race, vet, and vulnerability scans. The required isolated full-suite run
passed and the failure did not reproduce.

## Acceptance Decision

Passed. The capability path changes real runtime behavior, uses mature
cryptographic foundations, covers success and denial/degraded paths, exposes
stable non-sensitive outcomes, and leaves no mandatory STB-202 property
deferred.

STB-203 may now encrypt and authenticate network envelopes using only resolved
capability material and the normative `docs/network-privacy-protocol.md`
contract.
