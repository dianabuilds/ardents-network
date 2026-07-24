# Workload Execution Platform

## Status And Ownership

This document is the `v1` platform contract for the infrastructure side of
`Workload Control`. The domain continues to own desired and observed workload
truth; the executor is an adapter and does not become a second scheduler,
registry, or public API.

The selected product backend is the Docker Engine API on Linux. Ardents uses
the supported `github.com/moby/moby/client` and `github.com/moby/moby/api` Go
modules. It does not import the deprecated `github.com/docker/docker` module or
the Moby engine implementation module.

## Threat Boundary

The Ardents daemon and its executor adapter are a trusted local control plane.
Access to a Docker daemon is equivalent to broad control of that daemon and can
lead to host control in a conventional rootful deployment. Therefore:

- the Docker endpoint is local by default and is never mounted into workloads;
- remote daemon endpoints require mutually authenticated TLS and an explicit
  operator configuration;
- workload input cannot select privileged mode, host namespaces, devices,
  arbitrary mounts, daemon credentials, security-profile relaxation, or a
  different OCI runtime;
- image identity is an immutable digest, not a mutable tag;
- `v1` accepts only bounded public environment configuration and rejects secret
  values; it does not advertise a workload secret-delivery channel;
- the executor injects the reserved non-secret
  `ARDENTS_WORKLOAD_GENERATION` value; workload input cannot set or override it,
  and hosted-service probes use it to prove current listener ownership as
  defined in `docs/operations/hosted-service-probe-model.md`;
- every created object carries Ardents ownership labels and is reconciled by
  those labels after restart;
- absence of an enforceable isolation or resource property is an admission
  failure, not a warning or silent fallback.

Container isolation alone is not a complete boundary against hostile code.
Untrusted or third-party workloads require the `runsc` OCI runtime provided by
gVisor. The ordinary `runc` runtime is accepted only for workloads explicitly
admitted as trusted container workloads. A workload cannot promote its own
trust class.

## V1 Support Matrix

| Host / backend | Trust class | V1 status | Required behavior |
| --- | --- | --- | --- |
| Linux `amd64` or `arm64`, Docker Engine, `runsc` | untrusted / third-party | production | Required product path; preflight must prove `runsc`, cgroup and security controls before admission. |
| Linux `amd64` or `arm64`, Docker Engine, `runc` | trusted container | production, policy-gated | Explicit trust admission plus all container hardening and resource controls. No fallback from `runsc`. |
| Docker Desktop Linux containers on Windows/WSL2 or macOS | trusted test workload | development and CI only | Exercises the Docker adapter; must not be advertised as production host support. |
| Native Windows containers | any | unsupported | Fail configuration or admission explicitly. |
| Raw host process (`os/exec`) | trusted developer command | development only | Disabled by default; requires an explicit trusted-development mode and is never a product fallback. |
| containerd direct client | any | unsupported in `v1` | No lifecycle adapter is shipped. |
| Podman API | any | unsupported in `v1` | No compatibility claim or silent Docker emulation assumption. |
| Kubernetes | any | out of scope | Ardents does not become a cluster scheduler in `v1`. |

Production nodes must run a currently security-supported Docker Engine release.
At the decision date the minimum accepted patch level is `29.6.1`, because that
release contains multiple Engine security fixes absent from the observed local
`29.1.3`. The version floor is operational policy, not a promise that an old
floor remains safe: newly published high/critical fixes raise it. An upgrade may
be held only by a recorded security exception with containment and expiry.

## Startup Preflight

Before the product executor becomes ready it must query the Engine and prove:

1. the endpoint is reachable, API negotiation succeeds, and the daemon reports
   Linux containers on `amd64` or `arm64`;
2. the Engine satisfies the configured security floor;
3. cgroup v2 and enforceable CPU, memory, and PID limits are available;
4. seccomp is enabled and at least one host LSM (`AppArmor` or `SELinux`) is
   active for `runc` workloads;
5. `runsc` is registered when the node admits untrusted workloads;
6. the configured storage driver and writable-layer quota strategy can enforce
   the advertised disk bound;
7. rootless mode, when selected, uses the systemd cgroup driver with the needed
   controllers delegated; ignored resource flags are a hard failure;
8. the daemon is not configured through an unauthenticated remote TCP socket.

Readiness output must distinguish at least `backend_unreachable`,
`unsupported_host`, `engine_security_floor`, `resource_control_missing`,
`security_profile_missing`, `runtime_missing`, and `endpoint_insecure`.

## Required Container Contract

Every product container is created with a deterministic name derived from a
stable workload ID and immutable generation, plus labels for:

- Ardents ownership and schema version;
- node identity;
- workload identity;
- workload generation;
- trust class and selected runtime.

The adapter implements prepare/create, start, inspect, graceful stop, forced
termination after a bounded deadline, restart by new generation, and removal.
It lists only labeled objects during recovery. Unknown objects are never
adopted. Duplicate commands converge on the current state. A name or label
collision with inconsistent identity is a conflict, not an adoption signal.

Inspection maps real Engine state into observed workload state and retains the
container ID, image digest, generation, start/finish timestamps, exit code,
OOM flag, health state where applicable, restart count/budget, and a stable
operator-facing failure reason. Engine errors must retain safe causal context
without leaking registry credentials, environment secrets, or daemon paths.

## Mandatory Isolation Defaults

The lifecycle contract may not create a product container that lacks its
required bounds. The minimum creation posture is:

- non-root numeric user; no privilege escalation;
- `CapDrop=ALL`, no added capability unless specifically admitted;
- no privileged mode, devices, host PID/IPC/UTS/user namespaces, or daemon
  socket;
- read-only root filesystem with explicit bounded writable areas;
- default network `none`; a workload with admitted hosted-service ingress uses
  one Ardents-owned per-node internal bridge with no default external route;
- the workload container never owns a published host port; an Ardents ingress
  proxy on a separate ingress bridge publishes only ports derived from canonical
  service endpoint/probe pairs and forwards them into the internal bridge;
- mandatory CPU, memory, PID and writable-storage bounds;
- bounded stop timeout and disabled Engine automatic restart policy, because
  Workload Control owns restart budgets and observed truth;
- the configured trusted `runc` or untrusted `runsc` runtime selected by policy,
  never by the workload.

The closed configuration schema, concrete resource ceilings, image provenance,
trust selection, secret posture, redaction rules, and adversarial acceptance
boundary are defined in `docs/security/workload-security-policy.md`.

## Recovery And Orphan Rules

At daemon start the controller lists Ardents-labeled containers for its node,
validates the complete identity tuple, and reconciles them with the persistent
workload registry. It then:

- reattaches a matching current generation and inspects actual state;
- records older generations as superseded and removes them only through the
  controlled cleanup path;
- treats a labeled container with no matching registry generation as an orphan;
  the `v1` policy is fail-closed bounded stop followed by controlled removal,
  and any cleanup failure aborts startup with causal context;
- refuses a label mismatch or duplicate current generation as an identity
  conflict;
- never infers `running`, `ready`, or `healthy` from desired state.

Ingress proxy identity is retained on the immutable workload container as a
bounded encoded set of admitted bindings. Startup reconciliation treats the
proxy as ancillary runtime owned by the current workload generation: a missing
or stopped current proxy is recreated, while a proxy with no current registry
generation is stopped and removed. Recovery never reads arbitrary port intent
from container configuration and never adopts an unlabeled proxy. Before proxy
creation or recovery, the executor inspects the locally available immutable
image and requires `io.ardents.ingress.protocol=1`; a missing or different label
is an incompatible runtime component and fails closed.

The Node supervises every current proxy during observed-state refresh. A stopped
or missing proxy is recreated without restarting its backing Workload.
Repeated recovery failures use a bounded `1s, 2s, 4s, 8s, 16s, 30s` backoff;
the refresh remains degraded and records the causal ancillary-runtime error
until recovery succeeds.

The proxy treats RST, stream-copy, deadline, close, and half-close failures as
connection-local. They never terminate another connection or its listener.
The released defaults are a five-second backend dial timeout, a 30-second
whole-connection inactivity deadline, a ten-second write deadline, 128 active connections globally,
64 per admitted port, and 16 per source address. The image CLI exposes these as
`--dial-timeout`, `--idle-timeout`, `--write-timeout`, `--max-connections`,
`--max-connections-per-port`, and `--max-connections-per-source`; invalid or
non-positive combinations fail startup. This keeps global resource protection
while reserving capacity across ports and sources.

Every admission and abnormal connection close is emitted as a structured JSON
container-log event. Rejections use the stable reasons `global_limit`,
`port_limit`, or `source_limit`, so saturation and fairness failures are
operator-diagnosable without logging payload data.

Docker daemon restart, Ardents daemon restart, and workload process restart are
separate events and must remain distinguishable in diagnostics.

## Control-Plane Availability Contract

Every Docker control-plane operation inherits caller cancellation and has a
finite adapter deadline (10 seconds by default); the earlier caller deadline
wins. Runtime observation applies a tighter 2-second budget so an unavailable
engine cannot indefinitely block workload API or metrics collection.

External Docker I/O is performed without holding runtime or execution-state
mutexes. Observation and shutdown take an in-memory snapshot, perform Docker
calls, then conditionally apply results if the state has not changed. This keeps
unrelated reads responsive while the engine is slow or hung.

A successful observation is cached for at most 30 seconds. If a refresh fails
within that window, API and metrics receive the cached snapshot explicitly
marked `observation_degraded`, with `observed=degraded`, its `observed_at`
timestamp, the Docker failure reason, and an operator-action flag. A stale or
missing cache is an error and is never presented as current engine truth.

## Dependency And Alternative Decisions

- **Moby client/API modules: accepted.** They are the maintained public Docker
  Engine SDK surface, independently versioned and Apache-2.0 licensed. Version
  selection and vulnerability/license scans occur when they enter `go.mod`.
- **gVisor `runsc`: accepted as an external production runtime requirement.** It
  supplies the stronger isolation tier and integrates through Docker's OCI
  runtime selection. Compatibility and performance must be qualified with the
  supported workload set; it is not a Go library or a lifecycle backend.
- **Direct containerd: rejected for `v1`.** It is a lower-level substrate and
  would make Ardents own image, snapshot, network, OCI runtime, and recovery
  orchestration that Docker already provides.
- **Podman: deferred.** It is maintained, but a second engine contract would
  multiply compatibility and recovery behavior before the first contract is
  proven.
- **Kubernetes: rejected as the local executor.** It introduces cluster
  scheduling and control-plane ownership outside the node product boundary.
- **Raw host processes: non-product only.** They cannot enforce the mandatory
  filesystem, network, identity, and resource boundary.

## Acceptance Environment

Production support requires real Linux tests proving the applicable preflight,
isolation, failure, and recovery paths.

All tests are run in Linux Docker containers. Docker-in-Docker or a deliberately
scoped test daemon may be used for adapter integration; the test must not mutate
unrelated host containers. Windows host execution is not an acceptance path.

## Primary References

- Moby supported Go modules and deprecation notice:
  <https://github.com/moby/moby#go-modules>
- Docker Engine SDK overview:
  <https://docs.docker.com/reference/api/engine/sdk/>
- Docker Engine 29 release and security notes:
  <https://docs.docker.com/engine/release-notes/29/>
- Docker resource constraints:
  <https://docs.docker.com/engine/containers/resource_constraints/>
- Docker rootless resource-limit requirements:
  <https://docs.docker.com/engine/security/rootless/tips/>
- gVisor installation and platform support:
  <https://gvisor.dev/docs/user_guide/install/>
- gVisor Docker integration:
  <https://gvisor.dev/docs/user_guide/quick_start/docker/>
- gVisor production guidance:
  <https://gvisor.dev/docs/user_guide/production/>
