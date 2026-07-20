# Production Observability Process Boundary

## Scenario ID

`OBE-001`

## Layer

`e2e`

## Domain

`diagnostics`

## Category

Daemon lifecycle, authenticated monitoring, and shutdown.

## Goal

Prove that the real `ardd` process opens separate control and observability
listeners, enforces the scrape credential, projects canonical degraded truth
for an unprovisioned private node, and shuts both listeners down after a signal.

## Preconditions

- the daemon runs in the canonical Linux container test runtime;
- control and scrape credentials are separate protected files;
- loopback ports are reserved for the process test.

## Steps

1. Start the real daemon entry point with a versioned operator document.
2. Poll `/healthz` until process liveness returns HTTP 200, then query
   `/readyz` for canonical degraded truth.
3. Query `/metrics` without and with the scrape bearer token.
4. Send the process interrupt signal and wait for bounded shutdown.

## Expected Result

- the observability listener becomes live independently of the control path;
- the unprovisioned private node returns readiness HTTP 503 and matching
  degraded health metrics rather than claiming false readiness;
- unauthenticated metrics return HTTP 401 and authenticated metrics return the
  canonical Ardents samples;
- the correlation header is present;
- the process exits successfully and does not retain either listener.

## Failure/Degraded Variant

Missing scrape authority must reject only `/metrics`; it must not hide process
liveness or change canonical degraded readiness truth.

## Related Tests

- `tests/e2e/observability/process_test.go`

## False Positive Risk

The test builds and invokes the exact `cmd/ardd` process rather than
constructing handlers directly.

## False Negative Risk

Port selection is bounded to loopback and readiness uses polling with a fixed
deadline rather than a timing sleep.

## Notes

The child process inherits only the explicit configuration path and the
canonical container test environment.
