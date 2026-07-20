# STB-401 Dependency And Runtime Review

Date: 2026-07-19

## Decision

Accept Docker Engine as the `v1` product lifecycle backend, accessed through
the maintained `github.com/moby/moby/client` and `github.com/moby/moby/api`
modules. Require gVisor `runsc` for the untrusted workload class and allow
`runc` only for explicitly trusted container workloads. Keep raw `os/exec`
behind a disabled-by-default trusted-development mode.

`STB-402` introduced `github.com/moby/moby/api v1.55.0`,
`github.com/moby/moby/client v0.5.0`, and `github.com/containerd/errdefs
v1.0.0`. The graph passed `go mod tidy`, `go mod verify`, Apache-2.0 license
review, focused integration tests, and the canonical Docker test suite.
`govulncheck` found no Moby/client/API finding; its only reachable report is the
pre-existing, contained `GO-2026-4479` residual recorded in
`docs/security-exceptions.md`.

## Evaluation

| Candidate | Maintenance / license | Capability fit | Risk and outcome |
| --- | --- | --- | --- |
| Moby public `client` + `api` modules | Active public modules, Apache-2.0, independently versioned | Engine negotiation, create/start/inspect/stop/remove, images, stats, labels, networks and resources | Accepted. Do not use deprecated `github.com/docker/docker` or engine implementation packages. |
| Docker Engine 29 | Actively patched Engine line | Complete node-local lifecycle and recovery surface | Accepted with rolling security floor; currently `>=29.6.1`. Daemon authority is treated as a trusted control-plane boundary. |
| gVisor `runsc` | Active open-source OCI runtime, Apache-2.0, Linux amd64/arm64 | Stronger kernel isolation under Docker | Accepted for untrusted tier. Performance and syscall/network compatibility require qualification; absence fails closed. |
| `runc` | Standard Docker OCI runtime | Broad compatibility and resource enforcement | Accepted only for policy-designated trusted workloads, with hardening. Not sufficient for arbitrary third-party code. |
| Direct containerd | Active, Apache-2.0 | Low-level container lifecycle | Rejected for `v1`: would require Ardents-owned image, snapshotter, CNI/network and runtime orchestration. |
| Podman API | Active, Apache-2.0 | Similar lifecycle surface | Deferred: second engine semantics and recovery matrix have no current product requirement. |
| Kubernetes | Active, Apache-2.0 | Cluster orchestration | Rejected: changes the product boundary into a cluster scheduler. |
| Host `os/exec` | Go standard library | Process signals only | Non-product: cannot meet filesystem, network, identity, resource and recovery requirements. |

## Security And Operations

- Docker daemon access is high privilege. Workloads never receive its endpoint
  or credentials; remote endpoints require mTLS.
- The observed development host runs Docker Desktop Engine `29.1.3`, Linux
  containers, cgroup v2, seccomp and `runc`, but has no `runsc`. It is eligible
  for trusted development adapter tests only.
- Engine `29.6.1` includes multiple security fixes missing from `29.1.3`.
  Production preflight must enforce the active security floor.
- Docker defaults do not impose all resource limits. CPU, memory, PID and
  writable-storage limits must be explicit and verified.
- Rootless Docker can ignore cgroup resource flags without cgroup v2 and
  systemd delegation. Preflight must prove the required controllers.
- gVisor adds overhead and has compatibility differences; tests define the
  supported image/application set rather than assuming universal compatibility.

## Upgrade And Failure Policy

SDK API negotiation handles compatible daemon API versions, but it does not
override the security floor. A newly disclosed reachable high/critical Engine,
SDK, runtime, or container escape issue blocks production admission unless an
explicit time-bounded security exception proves containment.

Unsupported host OS, architecture, daemon version, security mode, resource
controller, runtime, or endpoint security fails explicitly. There is no
automatic fallback from `runsc` to `runc` or from containers to host processes.

## Sources

- <https://github.com/moby/moby#go-modules>
- <https://docs.docker.com/engine/release-notes/29/>
- <https://docs.docker.com/engine/containers/resource_constraints/>
- <https://docs.docker.com/engine/security/rootless/tips/>
- <https://gvisor.dev/docs/user_guide/install/>
- <https://gvisor.dev/docs/user_guide/production/>
- <https://gvisor.dev/docs/user_guide/quick_start/docker/>
