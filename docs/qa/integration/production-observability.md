# Production Observability

## Scenario ID

`OBS-001`

## Layer

`integration`

## Domain

`diagnostics`

## Category

Observability, failure explainability, and runtime security.

## Goal

Prove that the production readiness and Prometheus surfaces project the same
canonical runtime/Diagnostics truth and do not reveal resource identifiers or
secret-bearing event fields.

## Preconditions

- a real Ardents runtime is started in the canonical Linux container runner;
- the runtime reaches canonical `ready` lifecycle and Diagnostics health;
- the observability surface uses the real runtime as its public snapshot source.

## Steps

1. Query `/readyz` and `/metrics` in the healthy state.
2. Inject a bounded Diagnostics degradation through the Diagnostics-owned API.
3. Record a policy denial containing a resource identifier and a secret-bearing
   diagnostic payload field.
4. Query `/readyz` and `/metrics` again.

## Expected Result

- healthy readiness returns HTTP 200 and `ardents_node_ready 1`;
- degraded readiness returns HTTP 503;
- the health metric reports `degraded` and the policy-denial window increments;
- the metric output contains neither the resource identifier nor secret value.

## Failure/Degraded Variant

The injected Diagnostics degradation must be visible at the readiness and
metric boundaries without restarting the process and without mutating the node
lifecycle into a false healthy state.

## Related Tests

- `tests/integration/diagnostics/observability_test.go`
- `internal/observability/surface_test.go`

## False Positive Risk

The test could pass by checking only HTTP status. It therefore also checks the
real metric samples and absence of injected sensitive values.

## False Negative Risk

The test uses a real runtime but an in-process HTTP server to avoid port timing
and host-network variance. Separate daemon E2E covers process listener wiring.

## Notes

The fault is injected into Diagnostics-owned health truth, not into a parallel
observability state.
