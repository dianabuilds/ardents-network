# STB-102 Evidence — Toolchain And Safe Dependency Lines

Date: 2026-07-18

## Selected Changes

- Go language/toolchain baseline: 1.26.1 -> 1.26.5.
- `golang.org/x/net`: v0.50.0 -> v0.55.0.
- `golang.org/x/sys`: v0.41.0 -> v0.45.0 (v0.45.0 is required by the selected
  fixed `x/net` and `x/crypto` lines).
- `golang.org/x/crypto`: v0.48.0 -> v0.52.0.
- `github.com/quic-go/quic-go`: v0.59.0 -> v0.59.1.
- `github.com/waku-org/go-waku`: v0.10.1 -> v0.10.3.

The Go subrepository group also advanced compatible transitive `x/mod`,
`x/sync`, `x/text`, `x/telemetry`, and `x/tools` selections. go-waku v0.10.3
has the same `go.mod` as v0.10.1 and preserves the Waku pubsub replacement and
declared compatibility graph; the patch contains node-stop, keepalive, filter,
and concurrency fixes. libp2p and the Waku pubsub fork were not independently
advanced.

## Focused Verification

- Toolchain group: formatting, `go vet ./...`, local-control, CLI, network, and
  transport tests passed.
- Go subrepository group: vet, identity, payload, Windows process observation,
  persistence, Connect RPC, CLI, and the complete fast suite passed.
- quic-go group: readiness/transport unit tests and real TCP-WSS, relay, and
  legacy Store multi-node integration scenarios passed. QUIC remains explicitly
  unsupported.
- Waku group: vet, all Network Foundation unit tests, runtime process tests,
  and tagged Network Foundation plus Node integration packages passed.

## Full Gate Evidence

Command:

`tests/run.ps1 -Suite all -ReportDir tests/.artifacts/reports/stb-102-all`

Result:

- exit code 0;
- 110 total, 110 passed, 0 failed;
- 110 raw reports;
- canonical `summary.json` and `junit.xml` generated;
- duration: 491.2 seconds wall-clock.

Additional gates:

- formatting: passed;
- catalog: 110 tests, 26 scenarios, 110 formal bindings, 0 issues;
- `govulncheck ./...`: reduced from 11 reachable / 5 imported-package / 28
  module-only findings to 1 reachable / 0 imported-package / 1 module-only.

The remaining reachable finding is `GO-2026-4479` in `pion/dtls/v2`, for which
no fixed v2 release exists. All remediable reachable findings are closed. Its
truthful containment and exception lifecycle are assigned to STB-103.

## Gate Result

Passed. All Phase 0 gates remain green, no remediable reachable finding remains,
and real Waku relay/Store behavior survived the controlled upgrades.
