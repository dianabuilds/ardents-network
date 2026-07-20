# STB-502 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Ardents now performs capability-safe encrypted replica placement over the
canonical Waku private data-exchange channel. Placement discovers current node
records, obtains operation-bound and node-signed capacity observations, applies
trust, policy, route, freshness, diversity, quota, and headroom checks, and only
then executes reserve and commit.

The receiver persists reservations and commitments, reserves bytes before
transfer, stores only encrypted bytes, derives and verifies content identity,
uses single-use token digests, and treats a signed commit acknowledgement as the
first remote-copy truth. Restart restores reservation, quota, and commitment
state without turning partial work into availability.

## Implemented Contract

- Replica-control messages use private Waku `BLOB_REPLICA_CONTROL` envelopes and
  an additional Ed25519 node signature binding action, operation, source,
  target, public key, and body.
- Non-target capability holders silently ignore addressed control traffic; the
  named target verifies the complete signed binding and fails closed.
- Capacity discovery examines at most sixteen deterministic node candidates,
  runs at most four three-second queries concurrently, and accepts only fresh
  target-signed observations.
- Selection requires the greater of 64 KiB or five percent free-capacity
  headroom and preserves stable denial codes without raw errors or selectors.
- Replica storage is byte-bounded. An otherwise unconfigured node uses a safe
  1 GiB replica quota instead of interpreting zero as unlimited.
- Reservation protocol version `1` supports canonical `aes-256-gcm` encrypted
  Blobs up to 64 KiB inline. Larger Blobs and unknown protocol/cipher versions
  fail explicitly as `transfer_unsupported` until STB-503 supplies chunking.
- Reservation acceptance is idempotent and bound to peer, operation, Blob CID,
  intent version, encrypted bytes, expiry, nonce, and a single-use token.
- Commit validates token, peer, lease, exact ciphertext size, encrypted Blob
  metadata, derived hash/CID, operation replay, and response bindings before a
  commitment is observed.

## Acceptance Checks

- Canonical fast suite passed in the Linux test container in 44.47 seconds.
- Final full Data Substrate integration passed 11/11 scenarios. The tests used
  43.35 seconds; the named container completed in 48.35 seconds.
- The real two-node Waku placement scenario selected its target from signed
  observed capacity, reserved quota, transferred ciphertext, committed it, and
  persisted matching source/target commitment truth in 7.52 seconds.
- Focused unit tests cover success, duplicate reserve/commit, quota refusal,
  untrusted peer, wrong CID, partial commit, expired reservation, conflicting
  replay, excessive lease, unsupported protocol/size, capacity accounting,
  persistence across restart, selection freshness/diversity/headroom, stable
  denials, signature tampering, wrong target, and non-target ignore behavior.
- Linux race tests passed for `internal/data/placement`,
  `internal/data/replication`, and `internal/data/transfer`.
- `go vet ./...` passed after the final changes.
- `go mod verify` reported `all modules verified`; no dependency was added or
  upgraded by STB-502.
- Test catalog validation reported 140 tests, 140 formal bindings, 39 scenarios,
  zero missing bindings/documents/scenarios, and zero issues.
- Changed-production code-size gate passed with no soft or hard finding.

## Security And Architecture Review

- Waku remains the only network carrier. Waku Store or relay retention never
  counts as a durable replica.
- Carrier payloads remain encrypted by the canonical capability channel;
  replica control adds node-identity authentication rather than replacing
  capability authorization.
- Relay nodes and unrelated capability realms receive no plaintext Blob
  payload. Replica receivers persist only already-encrypted bytes.
- Raw policy errors, operation payloads, keys, capability material, selectors,
  and routes are absent from placement denial and diagnostic reasons.
- Public-key identity, signed body, operation ID, source/target, capacity
  observation, acceptance expiry, and commitment fields are cross-bound.
- Reservation tokens are persisted only as SHA-256 digests and compared in
  constant time.
- Resource admission is fail-closed and checked both optimistically during
  selection and authoritatively under the receiver lock during reservation.

## Resource Truth

All tests executed in Linux containers. The final resource snapshot is
`tests/.artifacts/resources/stb-502-final.json`: `vmmemWSL` used approximately
4.20 GiB, 25.18 GiB host memory remained available, and drive C retained
216.94 GiB free. No CPU, memory, or disk exhaustion was observed.

The earlier 12-minute UI wait was reproduced with the same code-size command:
the named container completed successfully in 6.05 seconds. Docker contained no
running test container while the UI still displayed the command, proving an
orchestration/waiter stall rather than a test or resource deadlock. Subsequent
validation used named detached containers plus short `inspect`, `logs`, and
resource checks.

## Evidence Surface

- `docs/data-availability-replication-semantics.md`
- `docs/network-privacy-protocol.md`
- `docs/qa/integration/data-replica-reservation-placement.md`
- `internal/data/placement/*`
- `internal/data/replication/*`
- `internal/data/replica_placement.go`
- `internal/data/replica_placement_test.go`
- `internal/data/transfer/private_exchange.go`
- `internal/network/privacy/wire/private.proto`
- `internal/runtime/authority/controller_data.go`
- `tests/integration/data-substrate/replica_placement_test.go`
- `tests/.artifacts/resources/stb-502-final.json`
