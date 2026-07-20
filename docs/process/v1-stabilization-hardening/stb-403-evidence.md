# STB-403 Evidence

Date: 2026-07-19
Status: completed

## Outcome

The product Docker executor now admits only strict, bounded workload
descriptions and immutable image digests from configured registries. It rejects
unknown or duplicate JSON fields, unsafe paths and arguments, secret-like
environment material, unlisted policy references, insecure remote Docker
endpoints, and unavailable isolation runtimes without fallback.

Every admitted container is numeric non-root, read-only, networkless, without
host mounts/devices or Linux capabilities, protected by `no-new-privileges`,
and bounded by CPU, memory/swap, PID, tmpfs, stop, and local-log limits. Trusted
policy references may use the configured trusted runtime; the default
untrusted class requires the configured sandbox runtime (`runsc` by default)
and fails closed when it is unavailable.

Raw workload configuration is no longer returned by status/read projections.
Docker failures are mapped to stable sanitized categories. Unexpected exits,
OOM, restart-budget exhaustion, operator action, and explicit operator restart
remain durable controller truth and cannot be erased by an immediate reconcile.

## Acceptance Checks

- Isolated Docker-in-Docker adversarial gate: 8/8 top-level tests passed against
  Docker Engine `29.6.1`, including lifecycle/recovery, resource inspection,
  real CPU throttling, read-only filesystem, denied networking, tmpfs overflow,
  PID pressure, direct OOM, controller-visible terminal OOM, unsafe mount,
  secret/config, registry/policy, and missing-`runsc` no-fallback scenarios.
  Report: `tests/.artifacts/reports/stb-403-adversarial-accepted/compose.log`.
- Focused workload integration: 18/18 passed. Report:
  `tests/.artifacts/reports/stb-403-workload/summary.json`.
- Focused policy integration: 5/5 passed. Report:
  `tests/.artifacts/reports/stb-403-policy/summary.json`.
- Canonical fast suite, `go mod verify`, code-size guard, and `git diff --check`
  passed after the final implementation.
- Containerized `govulncheck` found no new workload/Moby vulnerability. The only
  symbol-reachable result is the existing `GO-2026-4479` Waku/DTLS residual
  recorded in `docs/security-exceptions.md`. Report:
  `tests/.artifacts/reports/stb-403-govulncheck.txt`.

## Resource And Isolation Truth

All acceptance tests and the vulnerability scan ran inside Linux containers;
Windows was orchestration only. Host samples retained roughly 27-30 GB
available memory while `vmmemWSL` ranged approximately 4.7-7.0 GB during the
nested-engine runs. Disk retained about 221 GB free. No CPU, memory, or disk
exhaustion was observed, and no test containers remained after teardown.

The product does not claim secret injection in v1. Public bounded environment
values are supported; secret-like keys and values are rejected. Artifact
provenance for v1 is configured-registry admission plus immutable digest. It
does not claim signature or transparency-log verification.

## Evidence

- `docs/workload-security-policy.md`
- `docs/workload-execution-platform.md`
- `internal/workload/execution/docker_*.go`
- `internal/workload/controller/reconcile_*.go`
- `internal/policy/evaluation/workload.go`
- `internal/control/projection/snapshots.go`
- `cmd/ardd/config.go`
- `tests/integration/workload/docker_executor_test.go`
- `docs/qa/integration/workload-security-and-resources.md`

