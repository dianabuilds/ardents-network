# STB-303 Evidence — Mature Bootstrap And Peer Discovery

Date: 2026-07-19

## Capability Validated

Owning domain: Network Foundation / Messaging, coordinated with Discovery.

`service_node` now combines explicit static libp2p peers with Waku-compatible
signed DNS ENR trees. The running transport periodically replaces DNS knowledge,
filters it to the active TCP/WSS carrier, and replenishes toward three live
Relay peers. `local_development` rejects remote DNS discovery.

## Dependency Decision

The full assessment is retained in `stb-303-dependency-review.md`.

- accepted: existing go-waku signed DNS retrieval and explicit static peers;
- not added: Peer Exchange, because the available protocol is alpha and its
  responder population depends on Discv5;
- not enabled: Discv5, because it widens the supported TCP/WSS contract to UDP,
  intersects NAT/reachability and abuse-control work, and would weaken the
  containment assumptions of the active DTLS dependency exception;
- no new module or transport foundation was introduced.

## Runtime And Trust Contract

- at most four signed `enrtree://` roots and 128 usable addresses are accepted;
- unsigned roots, duplicate roots, ambiguous resolver input, and discovery on
  `local_development` fail before startup;
- an optional custom resolver must be an IP address and is meaningful only with
  signed roots;
- DNS addresses must be TCP and compatible with `tcp_only` or `tcp_wss`;
- refresh occurs every five minutes, with a ten-second bounded failure retry;
- each successful refresh replaces old DNS knowledge; a failed refresh clears
  stale DNS-only observations and closes their peer connections;
- an address also configured statically survives DNS removal;
- DNS results are not persisted, so the signed tree remains authoritative after
  restart;
- bootstrap source discovery, peer dial, and Relay readiness expose distinct,
  stable degraded reasons without resolver-error leakage.

## Real-Network Evidence

Tagged transport integration tests use real go-waku nodes and a locally signed
go-ethereum ENR tree:

- `TestSignedDNSColdStartAndPeerRestartRecovery` proves cold start with one dead
  static source and one valid signed DNS source, degradation on peer loss,
  recovery after peer restart, and stale-result withdrawal;
- `TestSignedDNSReplenishesToRelayPeerTarget` proves joining three Relay peers,
  detecting loss, and replenishing back to the target after restart.

Both focused tests passed. The existing full network and product scenarios also
remain green.

## Acceptance Gates

- focused unit suites for transport, readiness, process/orchestration, and
  `ardd` configuration — passed;
- focused tagged signed-DNS integration — 2/2 passed;
- formatting check (excluding unchanged generated third-party bindata),
  `go vet ./...`, canonical fast runner, and import boundary — passed;
- race suite across transport, readiness, process/orchestration, and daemon
  configuration — passed;
- handwritten production code-size guard — passed with no soft or hard breach;
- full integration report at
  `tests/.artifacts/reports/stb-303-integration`: 105/105 passed, 0 failed;
- full E2E report at `tests/.artifacts/reports/stb-303-e2e`: 14/14 passed,
  0 failed;
- test catalog: 119 tests, 32 scenarios, 119 formal bindings, 0 issues;
- `govulncheck` reconciliation: unchanged one symbol-reachable
  `GO-2026-4479` DTLS initializer/error path plus one module-only
  `GO-2026-5932` OpenPGP signal; both exactly match `docs/security-exceptions.md`.

## Acceptance Decision

Accepted. The slice changes real Waku runtime behavior, covers success and
degraded/recovery paths, preserves the canonical network foundation, avoids
stale or silently persisted trust, remains explainable through existing network
status projections, and has no deferred behavior required by STB-303. Public
reachability/NAT remains explicitly owned by STB-304 rather than falsely claimed
by bootstrap connectivity.
