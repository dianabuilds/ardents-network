# STB-401 Evidence

Date: 2026-07-19
Status: completed

## Outcome

The product executor, support matrix, trust boundary, lifecycle/recovery
contract, preflight requirements, alternatives, and dependency posture are
defined in `docs/workload-execution-platform.md`.

The selected normal product path is Docker Engine on Linux. Untrusted workloads
require gVisor `runsc`; trusted container workloads may use hardened `runc`.
Raw host process execution is development-only, disabled by default, and never
a fallback.

## Check Results

- Mandatory lifecycle, filesystem, network, identity, signal, resource and
  recovery properties map to concrete Docker Engine behavior.
- Unsupported platforms and modes have explicit fail-closed outcomes.
- The control-plane authority of the Docker endpoint and trust-class boundary
  are documented.
- The Moby public SDK modules are accepted before introduction; direct
  containerd, Podman, Kubernetes and product host-process execution are rejected
  or deferred with reasons.
- Current local environment truth was captured: Docker Desktop Engine `29.1.3`,
  Linux `amd64`, cgroup v2, seccomp and `runc`, no `runsc`. It is not production
  eligible under the current `>=29.6.1` Engine security floor.
- No Go dependency or runtime configuration changed in this decision task.

## Evidence

- `docs/workload-execution-platform.md`
- `docs/process/v1-stabilization-hardening/stb-401-dependency-review.md`
- `docs/domains/workload-control.md`
- `docs/workload-and-services-requirements.md`
- official sources linked from the platform and dependency-review documents

