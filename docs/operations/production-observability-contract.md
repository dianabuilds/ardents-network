# Ardents Production Observability Contract

## Purpose And Ownership

The production observability surface is a read-only projection of canonical
runtime and Diagnostics truth. It does not own lifecycle, health, network,
workload, service, storage, transfer, repair, or policy state and does not read
domain persistence directly.

The daemon exposes a dedicated HTTP listener with three fixed routes:

- `/healthz` proves that the daemon process can serve requests;
- `/readyz` projects canonical node lifecycle and Diagnostics health;
- `/metrics` exports bounded Prometheus metrics.

The listener defaults to `127.0.0.1:9090` and accepts loopback addresses only.
Remote exposure belongs to the deployment boundary and must use an authenticated
TLS proxy or equivalent operator-controlled transport. An optional protected
scrape token provides defense in depth on loopback; it is not the control API
credential and never appears in effective configuration, logs, diagnostics, or
metrics. Health and readiness responses remain minimal and unauthenticated so
container and orchestrator probes do not need operator authority. Metrics
require the scrape token whenever one is configured.

## Metric Truth

Metrics are collected at scrape time from public runtime snapshots. They may
aggregate facts but may not invent a second counter or state machine.

The bounded metric families cover:

- node lifecycle, readiness, and Diagnostics health;
- peer counts and active Waku protocol capabilities;
- cumulative network rejection counters and recent message/privacy failures;
- Waku Store retained-message count, configured message/byte capacities,
  combined SQLite database/WAL/shared-memory bytes, and the greater of message
  or disk capacity utilization;
- workload state, configured resource limits, restarts, and OOM outcomes;
- hosted-service readiness;
- storage inventory and byte totals;
- transfer state and recent repair outcomes;
- recent policy denials and pending operations;
- Go process/runtime resource metrics;
- HTTP request count and duration for fixed observability and control routes.

`recent` and `window` metrics are gauges over the bounded Diagnostics event
window. They are not presented as lifetime counters.

For persistent Store profiles, `ardents_waku_store_usage_ratio` at or above
`0.90` projects the network abuse/pressure state as `degraded`. Failure to read
the Store count or inspect its required database file projects that state as
`failed`.
`constrained_light_client` and automatic `restricted_defense` do not run a
persistent Store and therefore omit these Store gauges.

## Cardinality And Redaction

Labels use fixed vocabularies for state, domain, protocol, direction, failure
category, and route. Unknown values collapse to `other`. Labels never contain
node, peer, workload, service, operation, blob, manifest, selector, route target,
filesystem path, endpoint, principal, capability, correlation, or error text.

No metric or probe response contains payload meaning, selectors, private
channel references, tokens, keys, ciphertext, peer addresses, or object/blob
identifiers. Structured logs use fixed event names and safe summaries; request
headers and bodies are never logged.

## Correlation And Logging

Every HTTP request receives an `Ardents-Correlation-ID`. A caller-provided value
is accepted only when it is 1-64 characters from `[A-Za-z0-9._-]`; otherwise a
cryptographically random value is generated. The same value is returned in the
response and included in the structured completion log.

Request logs contain correlation ID, normalized route, method, status, and
duration. They never contain authorization headers, query strings, request or
response bodies, raw unknown paths, or domain resource identifiers.

## Probe Semantics

- `/healthz` returns HTTP 200 while the serving process is alive.
- `/readyz` returns HTTP 200 only when canonical node readiness is true and
  Diagnostics health is `ready`; initializing, stopped, degraded, and failed
  states return HTTP 503.
- Probe bodies are bounded JSON with status, normalized lifecycle state, and
  normalized health state only.

## Acceptance

The surface is accepted only when Docker/Linux tests prove that:

- success and injected failure produce matching Diagnostics, readiness, metrics,
  and structured-log truth;
- the daemon rejects non-loopback observability listeners even when a scrape
  token is configured;
- invalid and valid correlation IDs follow the contract;
- secrets, selectors, blob meaning, resource IDs, and unbounded labels are absent;
- metric gathering remains successful with empty and populated runtime state.
