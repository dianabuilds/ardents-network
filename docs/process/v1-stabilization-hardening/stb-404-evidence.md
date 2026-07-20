# STB-404 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Hosted Services now owns an active, typed readiness/liveness controller rather
than inferring readiness from workload desired or running state. HTTP/HTTPS,
TCP, and Unix probes execute real bounded network operations. HTTP responses
must carry the exact current `X-Ardents-Generation`; TCP and Unix listeners must
complete the bounded `ARDENTS READY <generation>` challenge. The executor
injects that generation through the reserved, workload-non-overridable
`ARDENTS_WORKLOAD_GENERATION` environment value.

The state machine distinguishes `inactive`, `warming`, `ready`, `degraded`,
`not_ready`, and `stale`, with bounded timeout/warm-up, consecutive success and
failure thresholds, endpoint/generation reset, and fresh proof after recovery.
Public Go, protobuf/ConnectRPC, JSON, and CLI surfaces now expose readiness,
ready, exposure eligibility, generation, last probe, endpoint reachability, and
publication as distinct facts.

Probe targets are restricted to literal loopback, `localhost`, and clean
absolute Unix sockets. URL credentials, redirects, non-local/DNS targets,
unsupported schemes, oversized endpoint sets, and malformed endpoints fail
closed with stable reasons. This prevents workload declarations from turning
the daemon into an SSRF client.

## Acceptance Checks

- Canonical hosted-services integration: 2/2 passed, including a real
  executor-managed workload process that reads the injected generation and
  serves the actual listener. Report:
  `tests/.artifacts/reports/stb-404-hosted-services/summary.json`.
- Canonical local-control-surface integration: 17/17 passed with readiness and
  exposure fields preserved across local, ConnectRPC, protobuf, JSON, and CLI
  projections. Report:
  `tests/.artifacts/reports/stb-404-local-surface/summary.json`.
- Real isolated Docker-in-Docker workload gate: 8/8 passed after generation
  injection, including inspection of the reserved environment value and all
  prior security/resource/OOM scenarios. Report:
  `tests/.artifacts/reports/stb-404-docker-generation/compose.log`.
- Canonical fast suite passed for all root packages in Linux Docker.
- Focused production, real-listener, and RPC/CLI tests passed after the final
  SSRF restriction and code split.
- Go formatting, changed-production code-size guard, `go mod verify`, and
  `git diff --check` passed. No dependency was added or upgraded.

## Scenario Coverage

- running workload with no listener: warming then `not_ready`;
- wrong/stale listener: current-generation ownership mismatch;
- slow startup: consecutive recovery after listener arrival;
- flapping: readiness retained below, and removed at, the failure threshold;
- timeout and caller cancellation: bounded `probe_timeout`;
- endpoint or generation change: counters and eligibility reset;
- stale sample: `stale` and ineligible;
- fresh controller/recovery: readiness is re-proved, not restored from desired
  state;
- stopped backing: immediate `inactive` and ineligible;
- non-local probe target: rejected before DNS or dial.

## Resource Truth

All tests ran inside Linux containers. Across the canonical and DinD runs,
available host memory remained approximately 29-32 GB. The heaviest nested
Docker run raised `vmmemWSL` to about 5.2 GB and completed normally; disk
retained about 220.6 GB free. No CPU, memory, or disk exhaustion was observed,
and no test containers remained after teardown.

## Evidence

- `docs/hosted-service-probe-model.md`
- `docs/qa/integration/hosted-service-readiness-probes.md`
- `internal/hosting/readiness/*`
- `internal/hosting/registry/*`
- `internal/control/projection/hosting_status.go`
- `internal/publication/status.go`
- `internal/workload/execution/docker_executor.go`
- `internal/workload/execution/process_executor.go`
- `proto/ardents/v1/ardents.proto`
- `tests/integration/hosted-services/readiness_test.go`

