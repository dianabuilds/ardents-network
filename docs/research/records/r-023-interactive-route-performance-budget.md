---
id: R-023
title: What performance budget makes the Interactive Route useful?
status: active
owner: product research
started: 2026-08-07
reviewed: 2026-08-07
---

# R-023 — Interactive Route performance budget

## Decision this unlocks

Define the end-to-end V1 performance and resource contract before comparing
routing families, transports, libraries, implementation languages, or path
shapes. The result must say what a User experiences, on which supported device
and network classes, at which percentile, and which security invariants may
never be traded away to meet the budget.

## Current contract

- [Product vision](../../product/vision.md)
- [Network functional map](../../product/functional-map.md)
- [Network product journeys](../../product/journeys.md)
- [Threat model](../../security/threat-model.md)
- [R-001: Interactive Route claim](r-001-interactive-route-claim.md)
- [R-002: Live Application Interface](r-002-live-application-interface.md)

Already fixed: security and performance are coequal gates; the Interactive
Route is a multi-hop low-latency Route Profile; no performance optimization may
use a direct path, weaker target authentication, shared Isolation Context,
unbounded queue, silent fallback, or automatic Application Data replay; and the
Named Unlisted Site is the first complete workload.

### P3-D1 — Required V1 platform classes

**Product Owner decision, accepted 2026-08-07:** Ardents is client software that
runs on ordinary User and Developer devices. V1 performance and release gates
must cover:

- a local endpoint used by a User on a Windows desktop or laptop;
- a local endpoint used by a User on a Linux desktop or laptop;
- a local endpoint used by a Developer to publish an ordinary local Application
  on the same Windows and Linux device classes;
- an infrastructure Node on a modest Linux server or VPS.

This does not require infrastructure Nodes to support Windows and does not
forbid them from doing so. macOS, phones, tablets, and other constrained devices
are later compatibility and measurement targets, not V1 performance or release
gates. Exact supported OS versions, CPU architectures, reference hardware, and
network conditions remain to be declared before numeric budgets are accepted.

This platform decision does not select public DNS, naming, or bootstrap.
Service Name resolution remains an internal Ardents product function under
R-003, while authenticated entry and recovery remain R-009. A User connecting
from Windows or Linux does not imply exposing a Service Name or origin through
public DNS.

Consequences:

- a benchmark that runs only on a developer workstation or Linux server cannot
  establish V1 client performance;
- publishing from an ordinary Windows or Linux device is a required product
  journey, not a server-only extension;
- the same security contract and user-visible results apply on both endpoint
  operating systems;
- platform-specific optimizations are allowed only when they preserve the same
  Route Profile and Application Interface semantics;
- mobile feasibility may be measured early, but it cannot lower or replace the
  accepted desktop V1 gate.

### P3-D2a — Authenticated connection establishment latency

**Product Owner decision, accepted 2026-08-07:** on the normal, non-adversarial
V1 reference network, a running and already joined endpoint opening a known
exact Service Name must reach an authenticated Service Connection within:

- **cold connection:** `p95 <= 3 s` when no usable Route has been prepared for
  the request;
- **warm connection:** `p95 <= 1 s` when current authenticated naming and
  reachability state and reusable Route state for the same Isolation Context are
  already available, but no Service Connection is yet open.

For both metrics, the timer starts when the Application submits a valid connect
request to the Application Interface. It stops only when the Application
receives success bound to the exact authenticated Service Target and can use the
byte stream. Name
resolution, Service reachability lookup, route work, rendezvous, and target
authentication performed inside that interval all count toward the result.

The targets apply when the Service is online and reachable, the endpoint is
already joined, and the measured topology has no deliberate blocking, injected
failure, or overload. P3-D6 must still specify the reference link properties,
geographic topology, sample count, and allowed failure rate. An otherwise
eligible failed or timed-out attempt is a missed target, not a fast sample that
may be omitted from the percentile.

These are top-down product targets, not claims about an existing
implementation. Passing them never permits a direct path, weaker Route Profile,
cached unauthenticated state, skipped target authentication, shared Isolation
Context, or any other R-001 violation. A candidate that needs such a shortcut
fails the product contract even if its latency is lower.

Not included in these two clocks:

- process startup and initial or resumed network join;
- Application request processing or time to the first useful Application byte
  after the stream becomes usable;
- an offline Service, blocked entry, active attack, route failure, or overloaded
  network.

Those scenarios need separate budgets rather than being averaged into the
normal connection metric.

## Remaining decisions

1. **P3-D2b — Startup and useful-data latency:** define endpoint start/join and
   first useful Application byte, including resumed, first-ever, and degraded
   cases, then set percentile budgets.
2. **P3-D3 — Sustained service:** set throughput, concurrent-connection, CPU,
   memory, and bandwidth budgets for each reference class.
3. **P3-D4 — Tail and degradation:** set jitter, loss, churn, alternate-route,
   and overload behavior without weakening R-001.
4. **P3-D5 — Hostile load:** define fairness and resource-exhaustion workloads
   and the useful honest-work floor during attack.
5. **P3-D6 — Measurement gate:** define direct baselines, topology, repetitions,
   artifacts, regression thresholds, and release failure rules.

## Hypotheses

- **H1:** One security-preserving Interactive Route contract can meet a useful
  Named Unlisted Site experience on every P3-D1 V1 platform class.
- **H2:** The same observable product contract needs separate numeric budgets
  for endpoint and infrastructure classes or for cold and warm operation.
- **H0:** No measured route candidate can meet both the R-001 security contract
  and the minimum useful V1 performance budget on all required platforms.

## Evaluation criteria

Every numeric target must state:

1. the complete User or Developer scenario and its start and end events;
2. reference hardware, operating system, network, topology, and workload;
3. percentile, sample count, warm-up, duration, and allowed failure rate;
4. direct local or ordinary-network baseline and the Ardents overhead;
5. CPU, memory, bandwidth, queue, and concurrency cost at every endpoint and
   Node role;
6. the R-001 and resource-bound invariants that remain enabled;
7. the condition that falsifies the target or blocks a release claim.

Average latency alone, an unloaded loopback test, or a benchmark on only one
required platform is insufficient.

## Evidence plan

### Primary sources

Use official performance methodology and published measurements from relevant
low-latency anonymity and overlay systems only to shape experiments and identify
known costs. Ardents budgets are product decisions and must not be copied from a
reference system without matching workload, topology, threat boundary, and
hardware.

### Experiment

Create reproducible local and geographically distributed topologies only after
P3-D2 through P3-D5 define their workloads. Retain configuration, build identity,
platform information, raw samples, percentile summaries, failures, resource
traces, and direct-baseline results. Disposable experiment code belongs under
`experiments/r-023-interactive-route-performance/`.

### Failure scenarios

- slow or lossy endpoint access link;
- a slow, overloaded, malicious, or disappearing ordinary Node;
- route construction and failure during cold and warm connection attempts;
- blocked entry and alternate Bridge acquisition;
- slow Application reader or writer applying backpressure;
- one Application or Local Grant attempting to monopolize the Endpoint;
- nominal throughput success hiding unacceptable tail latency or failure rate;
- an optimization meeting its number only by bypassing an R-001 invariant.

## Findings

- **Product Owner decision:** Windows and Linux desktop/laptop endpoints used by
  Users and Developers, plus a modest Linux server/VPS infrastructure Node, are
  the required V1 platform classes.
- **Product Owner decision:** from a running, joined endpoint on the normal
  reference network, exact-name connection establishment is capped at
  `p95 <= 3 s` cold and `p95 <= 1 s` warm, ending only at an authenticated,
  usable Service Connection.
- **Assumption:** these classes can share one user-visible performance promise;
  measurement may require separate numeric resource ceilings.
- **Assumption:** macOS and mobile support can follow without changing the V1
  network contract. This remains unverified and is not a current release claim.

## Options

- **One universal numeric budget:** easiest to communicate, but may hide large
  differences between endpoint and infrastructure constraints.
- **One user-visible budget with class-specific resource ceilings:** preserves a
  consistent experience while allowing honest CPU and memory limits per class.
- **Platform-specific user promises:** may be necessary if evidence diverges,
  but risks making the same Route Profile unpredictable across platforms.
- **Choose no production route yet:** required if no candidate meets the
  security contract and minimum useful performance on every V1 class.

## Recommendation

Keep one user-visible connection contract and measure class-specific resource
ceilings unless evidence falsifies that shape. Use the accepted cold and warm
connection targets as route-candidate gates, then define startup and
first-useful-byte latency, sustained load, degradation, hostile load, and the
reproducible release gate in that order.

Confidence: high for the platform boundary and desired connection experience;
the accepted latency targets remain unverified and the other numeric targets
remain undecided. The strongest counterargument is that supporting both Windows
and Linux from the first V1 slice increases packaging and systems-integration
work for a one-to-one project, but removing either would contradict the accepted
client product.

## Disposition

- State: `active`.
- P3-D1 accepted: Windows and Linux desktop/laptop endpoints used by Users and
  Developers and a modest Linux server/VPS infrastructure Node are mandatory V1
  benchmark and release classes.
- macOS and mobile remain later targets rather than V1 promises.
- P3-D2a accepted: exact-name connection establishment from a running, joined
  endpoint on the normal reference network is `p95 <= 3 s` without a prepared
  Route and `p95 <= 1 s` with current authenticated state and reusable Route
  state for the same Isolation Context. The timer ends only at an
  exact-target-authenticated, usable stream.
- Startup, join, Application processing, degraded paths, and hostile conditions
  are excluded from these two clocks and require their own explicit budgets.
- Public DNS, naming design, bootstrap, routing, libraries, language, exact
  hardware, and the remaining numeric budgets remain unselected.
- P3-D2b, startup and useful-data latency, is next.
- No ADR and no code.
