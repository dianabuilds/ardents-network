# STB-306 Evidence — Abuse, Resource, And Peer Controls

Date: 2026-07-19

## Capability Validated

Owning domain: `Network Foundation / Messaging`, with Policy-compatible
admission and operator diagnostics.

The real go-waku product path now enforces finite limits for message size,
total and per-IP connections, concurrent/rate-limited network operations,
Filter subscribers, Lightpush requests, Store query rate and aggregate Store
results. Outbound providers receive a bounded local failure penalty and
temporary ban. Automatic restricted defense rebuilds a full provider into a
Relay-only Waku node and recovery rebuilds the steady provider shape.

Safe defaults:

- maximum Waku message: 140 KiB;
- peer connections: 64 total, 4 per IP;
- product network operations: 16 concurrent, 20/s with burst 40;
- Filter subscribers: 32;
- Store aggregate result: 128;
- provider penalty: three consecutive failures, 30-second ban.

Operator overrides are accepted through the eight
`ARDENTS_MAX_NETWORK_*`/`ARDENTS_NETWORK_OPERATION_*` environment variables
implemented in `cmd/ardd/config.go`. Negative and unsafe values fail before
startup; zero retains the safe default.

## Architecture And Dependency Fit

- go-waku/libp2p remain the only network substrate and own peer/stream-aware
  connection, Filter, Lightpush, Store, and Relay controls;
- `internal/network/transport` owns product operation admission, local provider
  penalties, protocol-shape restart, and runtime truth;
- `internal/network/messaging` owns bounded carrier inputs and Store
  aggregation;
- no alternate transport, custom protocol server, or global reputation domain
  was introduced;
- the complete dependency and RLN decision is retained in
  `stb-306-dependency-review.md`.

RLN is not falsely claimed. The current operated-realm admission path bounds
resources but is not anonymous cryptographic anti-spam. Enabling go-waku RLN
requires an accepted realm membership authority, enrollment/revocation and
credential/tree lifecycle that do not yet exist. Windows additionally lacks a
qualified native RLN lifecycle even though the dependency graph can compile.

## Runtime Security And Diagnostics

- oversized messages fail before Waku publication;
- global operation admission is non-blocking under concurrency pressure and
  explicit under token-bucket exhaustion;
- Filter and Lightpush server limits are per peer in go-waku;
- Store endpoint/topic/result inputs are bounded before query execution;
- repeated outbound failures create only a local provider penalty, never a
  network-wide malicious-identity claim;
- temporary bans expire automatically and a successful retry clears the
  provider state;
- public network status exposes state, reason, aggregate rejection counters,
  and ban count, but never provider keys, addresses, selectors, or payloads;
- restricted-defense status reports only `relay` active and explicitly reduces
  `store`, `filter_service`, and `lightpush_service`.

Assessment: passed. The delivered path contains no fake limiter, no plaintext
diagnostic detail, and no one-way protocol shutdown disguised as recovery.

## Real-Network And Failure Evidence

Formal scenario `NFI-006` runs four real-Waku tests:

- oversized rejection and a bounded Lightpush flood with observable counters;
- six retained messages bounded to three Store results plus Filter subscriber
  exhaustion;
- five same-IP clients bounded to two provider connections;
- restricted-defense rebuild to Relay-only, absence of provider protocols to a
  real constrained client, and steady rebuild/recovery.

Unit failure-path evidence additionally proves concurrency backpressure, rate
admission, unsafe configuration rejection, provider ban threshold, expiry, and
successful recovery.

## Acceptance Gates

- focused package and NFI-006 suites — passed;
- formatting, `go vet ./...`, canonical fast runner, and import boundary —
  passed;
- race suite across network, runtime, control projection, Connect RPC, and
  daemon configuration — passed;
- touched handwritten production code-size check — passed with no soft or hard
  breach; the final split keeps messaging aggregation in Messaging and mode
  transitions in Transport;
- full integration report at
  `tests/.artifacts/reports/stb-306-integration`: 113/113 passed, 0 failed;
- full E2E report at `tests/.artifacts/reports/stb-306-e2e`: 14/14 passed,
  0 failed;
- test catalog: 127 tests, 35 scenarios, 127 formal bindings, 0 issues;
- `govulncheck` reconciliation is unchanged: one registered symbol-reachable
  `GO-2026-4479`, zero imported-package findings, and one module-only
  `GO-2026-5932`, matching `docs/security-exceptions.md`.

## Acceptance Decision

Accepted. All STB-306 abuse scenarios are bounded on real Waku nodes and
operator-visible, restricted defense changes the actual live protocol shape
and recovers, and the non-RLN admission scope and Windows limitation are
explicit. No mandatory critical behavior remains deferred inside this slice.
