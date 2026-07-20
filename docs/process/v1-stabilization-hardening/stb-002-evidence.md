# STB-002 Evidence — Tagged Integration API Migration

Captured on 2026-07-18 with the baseline workspace cache and VCS-stamping
normalization documented in `baseline-2026-07-18.md`.

## Changes

- migrated every network-foundation fixture from pointer-to-interface
  `*transport.Service` declarations to the public `transport.Service` API;
- retained all lifecycle cleanup through the public interface;
- migrated the stale node recovery test imports from removed
  `diagnostics/operations` and `diagnostics/persistence` packages to the current
  `diagnostics/operation` and `diagnostics/recorder` packages.

No concrete network transport implementation is imported by the changed tests.

## Checks

- focused `TestTransportBootstrapStatusDegradesAfterPeerLoss` (`NFI-001`):
  pass in 3.172 s;
- `go test ./... -count=1 -timeout=10m`: pass across the fast layer;
- full `go test -tags=integration ./tests/integration/...`: every tagged
  integration package compiles; eight packages pass and the workload package
  reaches execution before the already classified Windows process-inspection
  failure assigned to `STB-003`;
- focused reproduction of that remaining failure reports
  `tasklist ... ERROR: Access denied`, confirming it is not a compilation or
  network-foundation API regression.

The canonical serial runner also compiled every package before exceeding the
five-minute command envelope during workload execution. Its raw scenario
reports are retained under `tests/.artifacts/reports/stb-002-integration/raw/`.

## Conclusion

The tagged integration compilation boundary is restored. The only observed
integration failure is owned by the next task, `STB-003`, and no fast-suite
regression was introduced.
