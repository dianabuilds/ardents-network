---
status: accepted
date: 2026-08-23
supersedes: ADR-0012 (Route-foundation applicability only)
partially-superseded-by: ADR-0048 (TCP-only Carrier set)
---

# ADR-0024 — Select the native Interactive Route foundation

The Route authority, TLS 1.3 authentication, recovery ownership, and native
profile remain current. ADR-0048 supersedes only this record's original
TCP-only Carrier set by adding the State-selected QUIC-v1 Carrier.

## Context

Stage 8 retains the Interactive Route information-flow contract but has no
maintained Route foundation. The H3 C-5/C2 codec is deliberately laboratory
only, and WebTunnel is only an endpoint-adjacent H3 camouflage Adapter.
Promoting either by package migration would create an unauthenticated protocol
and transport choice.

## Decision

Select `ardents-interactive-route-v1`: a native Go Route with the C-5 data and
separate C-2 Introduction logical shape, TCP carrier legs protected by mutually
authenticated TLS 1.3, and the existing reviewed HPKE Introduction primitive.
Authenticated Network State and private Service material are the sole Route
authority/discovery inputs. Endpoint-local `route` selects the full Route;
`entry` owns only bounded Entry Invite/replay/replacement state;
`service/connection` owns connection recovery and never receives a direct
network fallback.

H3 Route/Bridge frames, plan files, and WebTunnel configuration are C0 retired
for the maintained profile. `internal/camouflage` is deleted in M7, rather than
renamed into a runtime Adapter. This supersedes ADR-0012 only where it might
otherwise be read as a maintained Route foundation; ADR-0012 remains the
historical H3 experiment decision.

`v1` starts with no legacy peer reader. Its future version lifecycle is
authenticated State/publication capability selection under ADR-0006. A future
generation needs an explicit canonical wire, conformance vectors, mixed-version
and downgrade tests, the required overlap/drain policy, and Qualification; no
Node-supplied value may select a lower generation.

## Consequences

- M7 may move Bridge's durable lifecycle to `entry` and retire camouflage;
- M8 may replace `routeplan` with the target Route over opaque State/Duty/
  Resource/Entry ports;
- M9 may bind Service publication and connection recovery to this exact
  Profile; and
- no public-route, camouflage, broad-observer, or independently-operated-node
  claim is added. R-023 Qualification remains mandatory before any such claim.

## Compliance

This ADR records the selected authority boundary and required
test/Qualification work. It does not select a directory, DHT,
foreign overlay, public bootstrap, transport fallback, or a new cryptographic
primitive.
