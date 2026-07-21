# Hosted-Service Probe Model

## Status And Ownership

This document defines the `v1` readiness and liveness contract owned by
`Hosted Services`. Workload Control supplies observed runtime backing and an
immutable workload generation. Publication consumes exposure eligibility but
does not execute probes or own readiness state.

## Supported Endpoint Probes

Every workload-backed endpoint is actively checked. The supported endpoint
schemes are:

| Scheme | Check | Listener ownership proof |
| --- | --- | --- |
| `http` / `https` | bounded `GET` of the declared URL; status `200..399` | response header `X-Ardents-Generation` exactly equals the decimal workload generation |
| `tcp` | bounded connect, write one Ardents readiness line, read one bounded line | response exactly equals `ARDENTS READY <generation>` |
| `unix` | the same bounded challenge/response over the declared Unix socket | response exactly equals `ARDENTS READY <generation>` |

Unknown schemes, missing addresses, redirects, oversized responses, TLS
validation failures, generation mismatch, and protocol mismatch are probe
failures. Probes never execute a workload-provided command and never weaken
TLS validation.

`v1` probes only literal loopback addresses, `localhost`, and clean absolute
Unix-socket paths. It performs no DNS lookup for workload-controlled probe
targets and cannot be used to reach metadata, LAN, Internet, or arbitrary host
services. A future externally advertised endpoint is a separate exposure fact;
it does not become the daemon's probe target. Network mapping from a local
listener to an advertised endpoint is required before publication.

The generation challenge is not an authentication secret. Its role is to bind
the observed listener to the currently admitted workload generation and reject
a stale or unrelated listener. It must not be used as an authorization token.

## State Machine

The canonical service states are:

- `inactive`: runtime backing is absent or not running;
- `warming`: the current generation is running but has not reached the success
  threshold;
- `ready`: every declared endpoint passed the current-generation ownership
  check for the required consecutive samples;
- `degraded`: a previously ready service is below its failure threshold or a
  probe result is becoming stale;
- `not_ready`: warm-up expired, the consecutive failure threshold was reached,
  the endpoint set is invalid, or ownership could not be proved;
- `stale`: no current sample exists inside the staleness bound.

`workload running`, `service ready`, `exposure eligible`, and `published` are
four separate facts. A ready service is exposure eligible only while runtime
backing, current generation, endpoint identity, probe freshness, and policy all
remain valid. Network publication outcome remains Publication-owned truth.

## Default Bounds

- per-endpoint timeout: `1s`;
- warm-up: `10s` from observed workload start;
- consecutive successes required: `2`;
- consecutive failures before ready is lost: `3`;
- maximum accepted sample age: `5s`;
- HTTP response body is not read; redirect following is disabled;
- TCP/Unix response is bounded to `128` bytes.

Tests may use shorter durations through an explicit controller policy. Product
defaults are fixed and are not workload-controlled.

## Reset And Recovery Rules

- workload generation change resets all counters and prior readiness;
- endpoint set or endpoint order change resets readiness;
- a result for an older generation cannot mutate the current state;
- a stopped, failed, removed, or missing workload immediately makes the service
  inactive and ineligible;
- after daemon recovery, readiness must be re-proved for the recovered
  generation; desired state or persisted `ready` alone is insufficient;
- a late probe result after generation or endpoint change is discarded;
- endpoint recovery must satisfy the consecutive-success threshold again.

## Diagnostics And Surface Truth

Hosted-service status exposes state, reason, runtime generation, last probe
time, readiness, exposure eligibility, endpoint reachability, and publication
outcome as distinct fields. Reasons use stable categories such as
`runtime_inactive`, `warming_up`, `listener_unreachable`,
`listener_generation_mismatch`, `probe_timeout`, `probe_stale`, and
`unsupported_endpoint_scheme`. Raw response data, URL credentials, certificate
paths, and transport errors are not returned to public status surfaces.

## Acceptance Boundary

Real Linux-container tests must cover process-running-without-listener, wrong
generation listener, slow startup, consecutive recovery, flapping below and
above thresholds, timeout, endpoint change, stale sample, stopped backing, and
daemon/controller recovery. Tests use actual loopback HTTP/TCP/Unix listeners;
a fake boolean prober is insufficient acceptance evidence.
