# Fork Manifest: go-libp2p

## Role

This fork is a local runtime substrate dependency behind Ardents transport.
It is not an Ardents-owned domain package and must stay quarantined behind the
`internal/transport` boundary.

## Integration Contract

- `Module`: `github.com/libp2p/go-libp2p`
- `Root require`: `v0.48.0`
- `Root replace`: `github.com/libp2p/go-libp2p => ./third_party/forks/go-libp2p`
- `Upstream source`: `https://github.com/libp2p/go-libp2p`
- `Pinned upstream baseline`: `v0.48.0`
- `Pinned upstream commit`: `062200be7aa1d18a0f54eefb17b0dbe2e96f0a79`
- `Comparison date`: `2026-03-24`
- `Comparison method`: `git diff --no-index` against a temporary clone of the
  upstream `v0.48.0` tag

## Known Local Delta

### Substantive patches

1. `defaults.go`
   Added `webrtc-direct` transport and default listen addresses in the local
   fork so the transport substrate can advertise and accept this address family.
2. `config/config.go`
   Added `libp2pwebrtc.ListenUDPFn` wiring that reuses QUIC packet
   connections when the swarm is already listening on matching `quic-v1`
   addresses.

### Non-substantive patches

1. `core/metrics/reporter.go`
   Import ordering only. No runtime semantics changed.

### Tree delta against upstream baseline

- Upstream example and test-plan trees are intentionally absent from the local
  fork copy:
  - `examples/`
  - `scripts/test_analysis/`
  - `test-plans/`

## Owner

- `Transport / network foundation maintainers`

## Update Policy

- Treat this fork as pinned infrastructure, not as an opportunistic editing
  area.
- Before any change:
  compare against the exact upstream baseline tag and update this manifest.
- After any change:
  run Ardents transport, discovery, and data-substrate tests that exercise the
  affected network path.
- During dependency refresh:
  prefer reducing or upstreaming local delta over accumulating new local
  behavior.

## Return-Upstream Decision

Keep the local fork only while Ardents transport still depends on the
`webrtc-direct` delta and related packet-connection wiring. If equivalent
behavior becomes available from upstream or from a safe Waku-compatible upgrade,
remove the local fork and switch back to an upstream module version.

## 2026-03-25 Experiment Note

Single-fork upstream parity was re-tested through a temporary root `modfile`
without the local `go-libp2p` replace.

Result:

- `go test ./internal/transport ./internal/node/runtime` passed
- `go test -tags integration ./tests/integration/network-foundation ./tests/integration/discovery ./tests/integration/data-substrate` passed

This means the old blocker description is no longer sufficient evidence that
Ardents still needs this local fork in the current product path.

Combined upstream removal was also re-tested on `2026-03-25` and passed.
This fork should therefore no longer remain wired through the root `go.mod`
unless a new directly evidenced blocker appears.

See:
- `docs/process/repository-quality-control/fork-exit-experiments-2026-03-25.md`
