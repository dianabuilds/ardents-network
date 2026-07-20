# Workload Security And Resources

## Scenario ID

`WKI-004`

## Layer

`integration`

## Domain

`Workload Control + Policy`

## Category

Security and non-functional isolation.

## Goal

Prove that the Docker workload backend enforces the admitted artifact,
execution runtime, namespace, filesystem, network, environment, and resource
boundary on a real Linux Docker Engine, and that denial or exhaustion remains
observable without exposing workload configuration or secret values.

## Preconditions

- tests run against the isolated Docker-in-Docker daemon from
  `docker/docker-compose.workload-test.yml`;
- workload images use immutable digests and numeric non-root users;
- the test policy explicitly distinguishes trusted `runc` workloads from the
  fail-closed untrusted `runsc` path;
- the host Docker daemon and unrelated containers are outside the test scope.

## Steps

1. Inspect the created container and assert its exact CPU, memory/swap, PID,
   tmpfs, log, capability, privilege, user, network, mount, device, runtime,
   and read-only filesystem configuration.
2. Drive a CPU-bound workload and assert that the cgroup reports throttled
   periods.
3. Attempt mutable/unapproved images, unknown policy references, secret-like
   environment values, unsafe mounts, and an unavailable untrusted runtime.
4. Exercise a read-only-root write, network access, tmpfs overflow, PID
   pressure, and direct memory exhaustion.
5. Reconcile an OOM-killed workload through the controller and assert terminal
   failure, operator action, publication withdrawal, and no automatic restart.
6. Read control projections and verify that raw workload configuration and
   secret-like values are absent.

## Expected Result

- admitted containers match the complete bounded execution profile;
- denied capabilities fail before a container starts and do not fall back to a
  weaker runtime;
- CPU, memory, PID, and writable-disk pressure remain inside the container
  boundary;
- OOM and restart-budget state remain explicit and actionable;
- daemon errors and workload configuration do not leak secret or host detail.

## Failure/Degraded Variant

- an unavailable untrusted runtime is a policy failure, never a `runc`
  fallback;
- an OOM-killed instance becomes terminal after the configured policy and is
  restartable only after an explicit desired-state transition;
- a remote Docker endpoint without certificate-backed TLS is rejected before
  client creation.

## Related Tests

- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorEnforcesSecurityAndResourceConfiguration`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorPublishesOnlyAdmittedIngressOnInternalNetwork`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorDeniesUnsafeIntentAndRuntimeFallback`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorDeniesFilesystemNetworkProcessAndMemoryPressure`
- `tests/integration/workload/docker_executor_test.go::TestDockerExecutorControllerSurfacesOOMAndBlocksAutomaticRestart`
- `internal/workload/execution/docker_spec_test.go`
- `internal/workload/execution/docker_endpoint_test.go`
- `internal/policy/evaluation/service_test.go`
- `internal/control/projection/snapshots_test.go`
- `cmd/ardd/config_test.go`

## False Positive Risk

- accepting only a successful Docker create without inspecting HostConfig;
- treating an application-level error as proof of cgroup enforcement;
- testing OOM directly but not checking controller restart/publication truth;
- asserting that a field is hidden while the raw config remains reachable on a
  different public projection.

## False Negative Risk

- Docker Engine version differences in stats timing can delay the first
  throttling sample;
- registry or runtime availability can fail setup before the intended boundary
  is exercised;
- host-wide pressure can kill the nested daemon and mimic container-level
  exhaustion, so host CPU, memory, and disk are sampled around the run.
