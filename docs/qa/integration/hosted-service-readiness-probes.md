# Scenario HSI-001

- `Layer`: `integration`
- `Domain`: `hosted-services`
- `Category`: `functional`, `non-functional`, and `security`

## Goal

Prove that Hosted Services derives readiness from a real generation-bound
listener rather than workload desired/running state or endpoint declaration.

## Preconditions

- a workload generation and HTTP endpoint are registered through the
  hosting-owned registry seam;
- probe policy has bounded warm-up, timeout, success/failure, and staleness
  thresholds;
- the test runs in the canonical Linux container runtime.

## Steps

1. Observe a running generation while no listener owns the endpoint.
2. Let warm-up expire and verify `not_ready`.
3. Start a real listener returning the current generation header.
4. Satisfy the consecutive-success threshold and verify `ready` and exposure
   eligibility.
5. Return a wrong generation until the consecutive-failure threshold is met.
6. Recreate the readiness controller and re-prove readiness for the recovered
   generation.
7. Change endpoint identity and verify that prior readiness is discarded.

## Expected Result

- running without a listener is never ready;
- wrong-generation listeners are rejected;
- warm-up and flapping thresholds prevent single-sample state changes;
- recovery and endpoint changes require fresh consecutive proof;
- readiness, exposure eligibility, and publication remain separate facts.

## Failure/Degraded Variant

Timeout, unsupported scheme, invalid endpoint, stale sample, stopped backing,
and caller cancellation produce bounded non-ready states with stable reasons.

## Related Tests

- `tests/integration/hosted-services/readiness_test.go::TestHostedServiceReadinessTracksRealGenerationBoundListener`
- `internal/hosting/readiness/controller_test.go`
- `internal/hosting/registry/readiness_test.go`

## False Positive Risk

- replacing the listener with a boolean fake;
- asserting only successful dial without generation ownership;
- allowing one success to reuse readiness from an older generation.

## False Negative Risk

- another host process takes the reserved test port;
- host resource exhaustion delays the bounded loopback probe;
- test timing uses wall-clock sleeps instead of explicit policy timestamps.

