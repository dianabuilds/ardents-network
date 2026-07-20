# STB-405 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Publication now emits a network service record only when the current immutable
workload generation is observed running, its paired local probes are fresh and
ready, the advertised endpoints match the active reachability mode, and policy
allows exposure. Loss of any input produces a signed empty-endpoint withdrawal;
recovery requires fresh evidence and an older generation cannot republish.

The endpoint model explicitly separates daemon-only `probe_endpoints` from
remote `endpoints`. Docker product workloads remain on a per-generation
internal, no-egress network and have no host port binding. A first-party,
non-root, scratch-image ingress proxy joins that internal network and the
node-owned ingress bridge and publishes only admitted ports. It receives no
workload secrets and uses a fixed validated target.

Receiving nodes now perform bounded periodic Waku Store fetch/authenticate/
merge after bootstrap. Replayed envelopes remain harmless, while a newer signed
publication or withdrawal converges without restarting the receiving node.

## Acceptance Checks

- Hosted-services canonical integration: 3/3 passed. HSI-002 used two real Waku
  nodes, resolved a signed remote service record, completed a real HTTP request,
  stopped the backing workload, imported the signed withdrawal on the receiving
  node, and then observed connection failure. Report:
  `tests/.artifacts/reports/stb-405-hosted-services-final/summary.json`.
- Isolated Docker-in-Docker workload gate: 9/9 passed. The ingress scenario
  proved that the workload network is internal, the workload has no published
  port, the bounded proxy exposes exactly one admitted port, and a separate test
  container can reach the service. Report:
  `tests/.artifacts/reports/stb-405-docker-ingress-final/compose.log`.
- Full canonical fast suite passed in the Linux test container after production
  changes.
- Focused Publication tests cover readiness threshold, probe ownership failure,
  generation change/regression, reachability loss/recovery, address-scope
  rejection, policy denial, withdrawal, and compensation.
- Focused Workload, Hosted Services, Node lifecycle/readiness, runtime process,
  daemon configuration, control projection, and ConnectRPC tests passed.
- `go mod verify` passed. No Go module was added or upgraded; the ingress proxy
  uses the Go standard library and the already selected Docker API dependency.
- Go formatting and the changed-production code-size gate passed after splitting
  Docker start/recovery paths. No changed file or function exceeds a soft or
  hard limit.

## Security And Architecture Review

- Waku remains the only network carrier for publication and remote discovery.
- The implementation does not substitute process state for listener readiness,
  nor metadata for real network delivery.
- Probe targets remain loopback/Unix-only and cannot turn the daemon into a DNS
  or remote SSRF client.
- Advertised LAN/public scope fails closed; unverified DNS advertisements are
  not accepted.
- Docker ingress is admitted from canonical service declarations, never from
  arbitrary container JSON. Workloads do not receive Docker credentials,
  arbitrary egress, host mounts, capabilities, or mutable proxy targets.
- Diagnostics distinguish readiness, policy/reachability denial, publication,
  withdrawal, rollback, and discovery refresh failure without exposing
  capability or secret material.

## Resource Truth

All tests ran in Linux containers. The final Docker-in-Docker run peaked at
approximately 6.5 GB for `vmmemWSL`; host memory still had approximately
27.3 GB available. Disk retained approximately 219.27 GB free. The earlier
cross-node run used approximately 3.8 GB for `vmmemWSL` with about 29.9 GB
available. No CPU, memory, or disk exhaustion was observed, and compose teardown
removed its test containers, networks, and per-run volumes.

## Evidence Surface

- `docs/hosted-service-publication-gate.md`
- `docs/workload-execution-platform.md`
- `docs/workload-security-policy.md`
- `docs/qa/integration/hosted-service-publication.md`
- `internal/publication/*`
- `internal/hosting/readiness/*`
- `internal/hosting/registry/*`
- `internal/workload/execution/*`
- `internal/workload/ingressproxy/*`
- `cmd/ardents-ingress-proxy/*`
- `internal/node/lifecycle/manager_recovery.go`
- `tests/integration/hosted-services/publication_test.go`
- `tests/integration/workload/docker_executor_test.go`
