# STB-601 Evidence

Date: 2026-07-20
Status: completed

## Outcome

Ardents now has one strict, versioned operator configuration contract that is
validated before partial startup and mapped to the real Node Runtime, Waku
transport, privacy channels, Workload Control, Hosted Services, Data, Policy,
logging, and diagnostics behavior. The local CLI and ConnectRPC surface expose
the effective redacted configuration and transactional reload outcome.

## Implemented Contract

- `ardents.config/v1` is a bounded strict JSON document selected by
  `ARDENTS_CONFIG_FILE`; duplicate, unknown, deprecated, oversized, and invalid
  fields fail before runtime construction.
- Context defaults derive node state and Waku Store paths from the configured
  node name/data directory instead of silently using unrelated process paths.
- Every accepted workload, service, data, and policy field is either mapped to
  its owning runtime service or rejected by cross-field validation. Initial
  workloads carry capability, policy-ref, lifecycle, and restart-policy truth.
- Private mode assembles real encrypted capability storage and distinct
  discovery/data channels with protected replay ledgers. Invalid material,
  permissions, issuer/subject bindings, and grants fail closed without a
  plaintext fallback.
- Reload distinguishes immutable, restart-required, safely reloadable,
  unchanged, invalid, and rolled-back candidates. Reverting a pending restart
  candidate to the active source clears pending state.
- Live reload applies Policy, discovery refresh, logging, and bounded
  diagnostics behavior through their owning services. Diagnostic detail and
  replica defaults affect runtime behavior rather than display-only state.
- `ard config show` and `ard config reload` use the canonical ConnectRPC
  boundary and require exact `config.effective` / `config.reload` actions.

## Failure And Security Proof

- Invalid version, schema, field, secret source, privacy material, workload
  executor/spec, service probe, replica minimum, retention ceiling, and policy
  contradictions are covered.
- Missing/wrong privacy keys, wrong identity/issuer binding, open file
  permissions, revoked/invalid grants, and unavailable protected stores fail
  before node construction.
- Effective snapshots and reload errors redact tokens, secret values,
  capability references, protected paths, selectors, keys, peer identifiers,
  and raw runtime errors.
- Invalid and mid-commit reload failures preserve the active generation and
  behavior. Restart-required candidates never masquerade as active.
- No alternate network substrate, fake privacy foundation, or plaintext
  compatibility path was introduced; Waku remains canonical.

## Acceptance Checks

- Focused Docker tests passed for `internal/runtime/config`, `cmd/ardd`,
  `internal/runtime/process`, `internal/runtime/orchestration`, and
  `internal/workload/execution`.
- Final Docker fast suite passed.
- Full Docker integration passed 129/129 scenarios with zero failures in
  350.775 seconds. OCI-001 proved effective inspection, successful reload, and
  rejection of an invalid candidate through the real CLI/RPC surface.
- Full Docker E2E passed 16/16 scenarios with zero failures in 126.505 seconds.
- `go vet ./...`, `go mod verify`, import-boundary checks, test-catalog
  validation, and production code-size validation passed.
- Code-size review found and removed one soft-limit breach by extracting
  workload policy admission without changing domain ownership. No hard or soft
  breach remains in the STB-601 production files.

## Resource And Orchestration Truth

All tests ran in Linux Docker containers. Final suites used detached named
containers and explicit Linux `timeout` bounds. The final integration container
ran for 10 minutes 7 seconds and exited `0`; it was not OOM-killed. A Docker CLI
monitoring call remained stuck after the container had finished, which made the
desktop UI incorrectly look active for hours. That monitor was terminated and
subsequent E2E progress was read from report files rather than long-lived
`docker logs` calls.

The final resource snapshot records approximately 2.89 GiB for `vmmemWSL` and
213.17 GiB free on drive C. No CPU, memory, or disk exhaustion occurred.

## Evidence Surface

- `docs/operator-configuration-contract.md`
- `docs/qa/integration/operator-configuration.md`
- `internal/runtime/config/`
- `cmd/ardd/operator_config_load.go`
- `cmd/ardd/operator_config_mapping.go`
- `cmd/ardd/operator_privacy.go`
- `internal/runtime/process/operator_config.go`
- `internal/transport/connectrpc/server_configuration.go`
- `boundary/cli/command_config.go`
- `tests/integration/local-control-surface/configuration_test.go`
- `tests/.artifacts/reports/stb-601-integration-final2/summary.json`
- `tests/.artifacts/reports/stb-601-integration-final2/junit.xml`
- `tests/.artifacts/reports/stb-601-e2e-final/summary.json`
- `tests/.artifacts/reports/stb-601-e2e-final/junit.xml`
- `tests/.artifacts/resources/stb-601-final.json`

## Acceptance Decision

Passed. The operator configuration is versioned, validated before startup,
mapped to enforceable runtime behavior, safely inspectable and reloadable, and
covered by failure, security, integration, and E2E evidence without deferred
critical behavior.
