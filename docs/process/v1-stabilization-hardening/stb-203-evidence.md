# STB-203 Evidence — Encrypted Network Envelopes

Date: 2026-07-19

## Capability Validated

Network Foundation now provides a product-grade `ardents-private/1` envelope
that protects product payloads before they enter Waku and authenticates,
authorizes, replay-admits, and decrypts them before domain delivery.

Identity remains the sender-grant authority. Network Foundation owns only wire
framing, key use, Waku topic binding, bounded decoding, and replay truth.

## Delivered Runtime Path

- exact 72-byte `ARDP` v1 outer header with explicit suite, flags,
  generation, whole-second lifetime, random message ID, random XChaCha nonce,
  and exact ciphertext length;
- XChaCha20-Poly1305 encryption under the STB-202 derived envelope key;
- associated-data binding to the complete header, canonical Waku pubsub topic,
  and opaque content topic;
- generated deterministic protobuf inner message with Ed25519 sender identity
  signature, signed grant ID, class, payload version, payload, and zero padding;
- exact 1/4/16/64/128 KiB padding buckets, including boundary cases where the
  padding protobuf field requires selecting the next encodable bucket;
- maximum 128 KiB inner and 132 KiB outer limits;
- explicit version, suite, flags, time, expiry, malformed, oversized,
  authentication, signature, sender authorization, replay, and replay-capacity
  reason codes;
- Identity-owned trusted sender-grant import and receive-time revocation
  enforcement, including rejection of post-revocation backdating;
- durable bounded replay storage using only HKDF/HMAC-separated local digests,
  with restart continuity, expiry pruning, and fail-closed capacity;
- reproducible protobuf code generation from
  `internal/network/privacy/wire/private.proto`.

The deterministic unit fixture produces outer-envelope SHA-256
`e29d0284fb9ac2b66be0ef4f06a310670256ad4d7b6280967f9e97982ccbe576`.

## Runtime Security Review

The security-sensitive assets are plaintext product payloads, envelope keys,
capability references, selectors, message/grant/channel IDs, nonces, sender
authorization outcomes, and durable replay state.

Controls verified:

- plaintext exists only before Seal and after successful Open; Relay receives
  only the padded ciphertext;
- topic relocation, wrong key/generation, and byte tamper fail AEAD
  authentication;
- unauthenticated messages cannot enter the replay ledger;
- exact duplicates remain rejected after replay-ledger reconstruction;
- capacity rejects new authenticated messages instead of evicting unexpired
  entries;
- replay persistence contains neither raw capability refs nor message IDs;
- `Material`, sealed envelope, opened message, grant, resolved capability,
  status, and error formatting omit selectors, keys, bearer refs, protected
  payload, and internal cryptographic detail;
- Waku's upstream logger uses a level-compatible discard core because upstream
  structured fields can contain raw content topics; Ardents-owned readiness,
  health, publish outcomes, and stable privacy codes remain available;
- canonical test reports contain zero controlled selector, principal, payload,
  operation, and test-secret hits.

`stb-203-dependency-safety.md` records the direct use of the already-pinned
`go.uber.org/zap v1.27.0` only for Waku log suppression. No module version or
network foundation changed.

## Integration Evidence

Canonical scenario NPI-002:

- report: `tests/.artifacts/reports/stb-203/summary.json`;
- two real Waku nodes connect through Relay;
- sender encrypts before `PublishRelayEnvelope`;
- receiver subscribes only to a capability-derived opaque content topic;
- captured carrier bytes contain none of the tested plaintext, principal,
  operation, class-name, or service semantics;
- authorized receiver restores the exact payload and sender identity;
- the captured duplicate is rejected after durable replay restart;
- tampered captured ciphertext is rejected without plaintext delivery;
- Waku upstream logs emit neither the opaque selector nor message fields.

The scenario is documented in
`docs/qa/integration/network-privacy-encrypted-relay-envelope.md`.

## Acceptance Checks

- focused privacy, capability, and transport unit tests: passed;
- padding-boundary, wrong-key, relocation, tamper, malformed inner, unknown
  outer version/suite/flags, wrong generation, future/expired time, oversized,
  invalid signature/principal/class, missing/revoked/backdated sender,
  replay/restart, capacity, and pruning cases: passed;
- isolated `go test ./... -count=1`: passed;
- race tests for privacy, capability, and transport: passed;
- `go vet ./...`: passed;
- code-size guard across changed production packages: passed with no soft or
  hard breach after responsibility-based splits;
- generated protobuf before/after `go generate` SHA-256:
  `255A6FA99686FF97DF0B3CD16285D056331C18133F7FFF3FACFE5740861B6C7C`;
- test catalog: 114 tests, 28 scenarios, 114 formal bindings, 0 issues;
- `govulncheck ./...`: unchanged single reachable GO-2026-4479 in Pion DTLS
  without a fixed version; the documented TCP/TCP-WSS-only transport control
  remains enforced and no privacy dependency added a finding.

## Acceptance Decision

Passed. The slice uses Waku as the real carrier, mature AEAD/protobuf
foundations, durable fail-closed replay, Identity/Policy sender authority, and
redaction-safe outcomes. No mandatory STB-203 property is deferred.

STB-204 may now migrate Discovery and Publication payload semantics onto this
envelope path without adding readable topics, plaintext fallback, or a second
network foundation.
