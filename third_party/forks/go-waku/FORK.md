# Fork Manifest: go-waku

## Role

This fork is the canonical Waku-backed network foundation for Ardents `v1`.
It remains a dependency-backed substrate behind Ardents transport, not
product-owned domain code.

## Integration Contract

- `Module`: `github.com/waku-org/go-waku`
- `Root require`: `v0.10.1`
- `Root replace`: `github.com/waku-org/go-waku => ./third_party/forks/go-waku`
- `Upstream source`: `https://github.com/waku-org/go-waku`
- `Pinned upstream baseline`: `v0.10.1`
- `Pinned upstream commit`: `f40bcd7e797881dfccfa57fb70ba58e7dd3cf560`
- `Comparison date`: `2026-03-24`
- `Comparison method`: `git diff --no-index` against a temporary clone of the
  upstream `v0.10.1` tag

## Known Local Delta

### Substantive patches

1. `waku/v2/discv5/discover.go`
   Added NAT discovery and UDP port mapping when `advertiseAddr` is not
   explicitly configured, so discovery can expose reachable external endpoints.

### Metadata mismatch

1. `VERSION`
   The local file still reports `0.10.0` while the Ardents module contract and
   upstream comparison baseline are `v0.10.1`. Treat the root module contract
   as authoritative until this metadata is normalized.

### Non-substantive patches

1. `tests/string_generators_test.go`
   Import ordering only.
2. `waku/v2/peermanager/connection_gater_test.go`
   Import ordering only.
3. `waku/v2/utils/multiaddr_test.go`
   Import ordering only.

### Tree delta against upstream baseline

- Upstream `examples/` content is absent from the local fork copy.

## Cross-Fork Integration Note

`third_party/forks/go-waku/go.mod` still names
`github.com/libp2p/go-libp2p v0.39.1`, but Ardents root integration replaces
that module with the local `third_party/forks/go-libp2p` tree. The root
`go.mod` contract is authoritative for Ardents builds.

## Owner

- `Transport / network foundation maintainers`

## Update Policy

- Treat this fork as pinned canonical substrate.
- Before any change:
  compare against the exact upstream baseline tag and update this manifest.
- After any change:
  run Ardents transport, discovery, node-runtime, and data-substrate tests that
  exercise Waku-backed paths.
- Revalidate dependency-security evidence whenever the local delta or upstream
  baseline changes.

## Return-Upstream Decision

Keep the local fork only while Ardents requires the NAT/discovery delta or
other explicit local compatibility patches. Prefer upstreaming generic fixes or
removing local delta during every upgrade cycle instead of growing a permanent
Ardents-specific Waku branch.

## 2026-03-25 Experiment Note

Single-fork upstream parity was re-tested through a temporary root `modfile`
without the local `go-waku` replace.

Result:

- `go test ./internal/transport ./internal/node/runtime` passed
- `go test -tags integration ./tests/integration/network-foundation ./tests/integration/discovery ./tests/integration/node` passed

This means the old blocker description is no longer sufficient evidence that
Ardents still needs this local fork in the current product path.

Combined upstream removal was also re-tested on `2026-03-25` and passed.
This fork should therefore no longer remain wired through the root `go.mod`
unless a new directly evidenced blocker appears.

See:
- `docs/process/repository-quality-control/fork-exit-experiments-2026-03-25.md`
