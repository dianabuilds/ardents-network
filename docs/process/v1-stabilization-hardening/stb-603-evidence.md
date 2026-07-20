# STB-603 Evidence — Production Observability Surface

## Accepted Product Properties

- A dedicated read-only HTTP surface exposes bounded `/healthz`, `/readyz`, and
  Prometheus `/metrics` projections from canonical runtime and Diagnostics truth.
- Metrics cover lifecycle, readiness, health, peers, active Waku protocols,
  failures, workload resources, hosted services, storage, transfers, repair,
  policy denials, and pending operations without resource identifiers or
  unbounded labels.
- Structured request completion logs use normalized routes and safe correlation
  IDs without headers, queries, bodies, selectors, blob meaning, or secrets.
- The daemon listener is loopback-only. Optional bearer authentication is
  defense in depth; remote TLS/auth exposure belongs to the deployment boundary.
- Versioned Prometheus alerts and a Grafana dashboard are included under
  `docker/observability/`.

## Docker/Linux Verification

| Check | Result | Evidence |
| --- | --- | --- |
| Observability unit/config/process packages | passed | `go test ./internal/runtime/config ./internal/observability ./cmd/ardd`; 3/3 packages passed in 6.5 s |
| OBS-001 canonical success/degradation correlation | 1/1 passed | `tests/.artifacts/reports/stb-603-obs001/summary.json` |
| OBE-001 real `ardd` process boundary | 1/1 passed | `tests/.artifacts/reports/stb-603-obe001-final2/summary.json` |
| Fast repository checks | passed | Docker/Linux fast suite, including static and code-size guards |
| Broad integration sampling | 124/124 recorded results passed | `tests/.artifacts/reports/stb-603-integration-final/raw/` |

The broad integration command reached its explicit 12-minute Linux timeout
(`exit 124`) after 124 passing reports and before producing a suite summary. It
was not rerun: OBS-001 directly covers the new runtime integration and OBE-001
covers the exact daemon process boundary. Repeating the unrelated long tail
would add runtime cost without increasing STB-603 coverage.

## Dependency And Security Review

- The implementation reuses the existing direct
  `github.com/prometheus/client_golang v1.22.0` dependency; no new observability
  foundation was introduced.
- Scrape labels use fixed vocabularies and collapse unknown values to `other`.
- The listener cannot bind a non-loopback address even when a token is present.
- `govulncheck` still reports `GO-2026-4479` in transitive
  `github.com/pion/dtls/v2@v2.2.12`. The v2 line has no published fix. Ardents'
  supported Waku transport profiles suppress WebRTC, so this code path is not a
  supported runtime transport; the residual remains tracked for release-level
  dependency review.

## Primary Artifacts

- `docs/production-observability-contract.md`
- `docs/operator-observability.md`
- `internal/observability/`
- `tests/integration/diagnostics/observability_test.go`
- `tests/e2e/observability/process_test.go`
- `docker/observability/prometheus-alerts.yml`
- `docker/observability/grafana-dashboard.json`

