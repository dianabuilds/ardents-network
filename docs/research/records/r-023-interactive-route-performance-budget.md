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
network conditions remain to be declared for endpoints; the concrete processor
baseline and software environment for the later accepted reference VPS also
remain measurement details.

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

### P3-D2b — Local endpoint readiness latency

**Product Owner decision, accepted 2026-08-07:** on the normal, non-adversarial
V1 reference network, the installed Ardents process must make the local endpoint
network-ready within:

- **routine restart:** `p95 <= 5 s` when valid previously authenticated local
  network state is available;
- **clean first start:** `p95 <= 15 s` when no saved Ardents network state is
  available.

The timer starts when the operating system starts the already installed Ardents
process. It ends only when the Application Interface is available and the
endpoint has authenticated enough current network state and established at
least one usable entry path to accept an outbound connection attempt. Merely
opening a local socket, loading a UI, or reporting a process as healthy is not
network readiness.

The clean-start clock includes local initialization, required local key or state
creation, authenticated bootstrap-state acquisition, validation, entry
selection, and joining work. It does not include downloading or installing the
software, operating-system startup, optional User interaction, or a subsequent
Service Name resolution and Service Connection. The routine-restart clock may
reuse only valid authenticated state and may not skip expiry, rollback, or
integrity checks.

The targets assume an ordinary reachable network with at least one valid entry
path and no deliberate blocking, injected failure, or overload. R-009 still
defines authenticated bootstrap and blocked-entry recovery; no public DNS
dependency is implied. An otherwise eligible failed or timed-out start is a
missed target and may not be omitted from `p95`.

These are top-down product targets and remain unverified. Reporting ready with
unverified network state, a direct Service path, a weaker Route Profile, a
shared Isolation Context, or another bypass of the accepted security contract
fails the product even if startup is faster.

### P3-D2c — Named Unlisted Site first-byte latency

**Product Owner decision, accepted 2026-08-07:** on the normal, non-adversarial
V1 reference network, a running and network-ready endpoint opening the
controlled Named Unlisted Site tracer must receive the first valid byte of its
HTTP response within:

- **cold site open:** `p95 <= 4 s` when no usable Route has been prepared for
  the request;
- **warm site open:** `p95 <= 2 s` when current authenticated naming and
  reachability state and reusable Route state for the same Isolation Context are
  already available.

The timer starts when the Reference Application submits the exact Service Name
and fixed tracer request. It ends when that Application receives the first byte
of a valid HTTP response from the authenticated Service Target. The clock
includes name resolution, reachability lookup, Route and Rendezvous work, target
authentication, request transmission, controlled reference-service processing,
and return transport.

The workload is a deterministic small HTTP request and response served without
external dependencies or intentional application delay. P3-D6 must fix their
exact bytes and reference topology before qualification. Browser rendering,
script execution, secondary assets, external resources, and arbitrary Service
computation are excluded because the Ardents network cannot bound them.

An otherwise eligible failure or timeout is a missed target and may not be
removed from `p95`. A cached page response, direct path, weaker Route Profile,
skipped target authentication, or cross-context Route reuse cannot satisfy the
metric. These are top-down product targets and remain unverified.

### P3-D3a — Single-connection sustained Application goodput

**Product Owner decision, accepted 2026-08-07:** on the normal, non-adversarial
V1 reference network, at least 95% of eligible 60-second single-connection
transfer runs must deliver Application Data at or above:

`min(10 Mbit/s, 50% of paired direct-baseline goodput)`

This is measured separately in each direction. In statistical terms, the
fifth-percentile (`p05`) goodput for User-to-Service runs and for
Service-to-User runs must each meet the formula; fast runs in one direction
cannot compensate for slow runs in the other.

The 60-second measurement window starts after the exact Service Target is
authenticated, the Service Connection is usable, and the fixed transfer begins.
Goodput is the Application Data successfully delivered to the receiving
Application during that window divided by 60 seconds. Protocol headers,
handshakes, padding, cover traffic, retransmissions, and other carrier bytes are
cost rather than useful payload and do not enter the numerator.

The paired direct baseline uses the same sending and receiving reference
devices, access links, direction, and incompressible test payload in the
controlled topology, but without the Ardents multi-hop path. It exists only as
a benchmark and never permits a production direct Service path. For example, a
20 Mbit/s direct baseline requires at least 10 Mbit/s through Ardents, while an
8 Mbit/s baseline requires at least 4 Mbit/s.

The target covers one active Service Connection with an online Service and no
deliberate blocking, injected failure, or overload. Simultaneous full-duplex
load, multiple connections, and resource ceilings remain P3-D3b. An eligible
connection failure or premature loss is a missed run and may not be removed
from `p05`.

Meeting the target by reducing required Route Knowledge Separation, weakening
target authentication or integrity, sharing forbidden cross-context state,
omitting required camouflage traffic, or bypassing any accepted security rule
fails the product contract. The target is top-down and remains unverified until
P3-D6 supplies the exact payload, topology, baseline procedure, sample count,
and retained evidence.

### P3-D3b1 — Client endpoint concurrency floor

**Product Owner decision, revised and accepted 2026-08-07:** every required
Windows and Linux client reference endpoint must support at least:

- `64` concurrently open outbound Service Connections in total;
- `16` of those connections simultaneously carrying Application Data under the
  declared active-transfer workload.

The initially recorded `128/32` floor was revised after separating ordinary
client demand from publisher capacity. It was technically plausible but lacked a
V1 client journey that justified making it a mandatory minimum.

These numbers are minimum supported capacity, not maximum product limits. An
implementation or Endpoint Owner may configure and support more when resources
allow. They are totals across the client endpoint's Applications, Local Grants,
destination Services, and Isolation Contexts; they are not a per-Service
allowance and do not limit how many Users may connect to a published Service.

An open connection has authenticated its exact Service Target, returned
success, and has not closed or failed. An active connection is offered the fixed
P3-D6 transfer workload and must make Application Data progress during the
measurement interval; opening idle sockets or moving one token byte cannot
satisfy the `16`-connection case. P3-D6 must fix the duration, traffic mix, and
progress rule.

The single-connection P3-D3a goodput floor does not apply independently to all
16 active connections. Aggregate goodput, resource ceilings, and quantitative
fairness remain P3-D3c so they cannot be inferred from a connection count.

When a configured finite budget is exhausted, a new operation returns the
accepted local resource-limit result. Load must not cause a crash, deadlock,
unbounded queue, silent connection eviction, cross-context state sharing, or
security downgrade. Each connection retains the accepted target
authentication, isolation, stream, and fail-closed contracts.

This decision covers outbound capacity of a client endpoint only. Concurrent
incoming connections to a published Service and the capacity of infrastructure
Nodes remain P3-D3b2 and P3-D3b3 respectively.

### P3-D3b2 — Publisher endpoint concurrency floor

**Product Owner decision, accepted 2026-08-07:** every required Windows and
Linux publisher reference endpoint must support at least:

- `256` concurrently open incoming Service Connections in total;
- `64` of those connections simultaneously carrying Application Data under the
  declared active-transfer workload.

These are minimum network-facing publisher capacities, not hard maxima. The
budget is total across all local published Services, but one Service may use the
entire capacity when its Local Grant and every ancestor resource budget permit
it. A Developer or Application may deliberately configure a smaller
application-policy limit; that does not reduce the capacity the controlled V1
publisher benchmark must demonstrate.

The P3-D3b1 definitions of open and active connections apply, with direction
reversed for accepted incoming Service Connections. P3-D6 must supply the fixed
Service workload, duration, traffic mix, and progress rule. The metric does not
require the P3-D3a single-connection goodput floor independently on all 64
active connections; aggregate goodput, CPU, memory, overhead, and fairness
remain P3-D3c.

When the finite publisher budget is exhausted, admission remains bounded and
the remote side receives only a supported honest failure under the later R-007
contract. Exhaustion must not cause unbounded queues, crash, deadlock, silent
success, cross-Service or cross-context state reuse, target-authentication loss,
or security downgrade.

This decision does not set infrastructure Node capacity. Entry, relay,
discovery, Rendezvous, and Bridge roles receive a separate P3-D3b3 floor because
their unit of work and resource amplification differ from a published Service.

### P3-D3b3 — Reference infrastructure Node class

**Product Owner decision, accepted 2026-08-07:** every selected V1
infrastructure role must demonstrate practically useful, bounded operation on a
modest Linux server or VPS with:

- `2 vCPU`;
- `2 GiB RAM`;
- a symmetric `100 Mbit/s` network link.

This is the minimum reference class for comparing and qualifying a role, not a
maximum, a recommended production ceiling, or a promise that smaller machines
cannot contribute. More capable hardware may provide more bounded capacity, but
hardware size alone never grants more trust, governance power, route-selection
priority, or permission to perform another role.

Each entry, relay, discovery, Rendezvous, or Bridge role is measured separately
on this class once its unit of work is defined. The requirement does not say that
one VPS must run every role simultaneously. A candidate that combines roles must
still preserve R-001 Route Knowledge Separation and is evaluated as a combined
trust and resource boundary rather than using co-location to claim efficiency.

The exact processor model or minimum benchmark score, Linux distribution and
kernel, available storage, network latency, traffic pattern, and sustained role
capacity remain P3-D6 and P3-D3b4 evidence. A role that is technically able to
start but cannot carry its later accepted workload within bounded resources is
not practically useful on the reference class.

## Remaining decisions

1. **P3-D3c — Resources, scale-up, overhead, and fairness:** define how stronger
   client, publisher, and Node hardware raises bounded capacity; set CPU, memory,
   carrier-bandwidth overhead, aggregate goodput, queue, and per-connection
   progress budgets for each reference class.
2. **P3-D3b4 — Role-specific Node capacity:** after R-004 defines candidate
   units of work, set entry, relay, discovery, Rendezvous, and Bridge capacity
   floors on the accepted reference class.
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
- **Product Owner decision:** endpoint network readiness from an installed
  process start is capped at `p95 <= 5 s` for a routine restart with valid state
  and `p95 <= 15 s` for a clean first start without saved Ardents state.
- **Product Owner decision:** the controlled Named Unlisted Site returns its
  first valid HTTP response byte within `p95 <= 4 s` cold and `p95 <= 2 s` warm
  from a running, network-ready endpoint.
- **Product Owner decision:** for a single established Service Connection, the
  `p05` 60-second Application goodput in each direction is at least
  `min(10 Mbit/s, 50% of paired direct-baseline goodput)`.
- **Product Owner decision:** a required Windows or Linux client reference
  endpoint supports at least `64` concurrently open outbound Service
  Connections, including at least `16` simultaneously active connections. This
  revises the initially recorded `128/32` proposal after separating client and
  publisher workloads.
- **Product Owner decision:** a required Windows or Linux publisher reference
  endpoint supports at least `256` concurrently open incoming Service
  Connections, including at least `64` simultaneously active connections.
- **Product Owner decision:** every selected infrastructure role must be useful
  on a Linux `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS; this is
  a minimum comparison class rather than a capacity ceiling.
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
ceilings unless evidence falsifies that shape. Use the accepted connection and
endpoint-readiness targets and the tracer first-byte target as candidate gates,
then apply the accepted single-connection goodput and separate client and
publisher concurrency floors. Define infrastructure Node capacity, resources,
scale-up, fairness, degradation, hostile load, and the reproducible release gate
in that order.

Confidence: high for the platform boundary and desired connection experience;
the accepted latency, goodput, client-concurrency, and publisher-concurrency
targets and the infrastructure reference class remain unverified, and the
remaining numeric targets remain undecided. The strongest counterargument is
that
supporting both Windows and Linux from the first V1 slice increases packaging
and systems-integration work for a one-to-one project, but removing either would
contradict the accepted client product.

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
- P3-D2b accepted: an installed process reaches authenticated network readiness
  within `p95 <= 5 s` on routine restart with valid state and `p95 <= 15 s` on a
  clean first start. A local socket or UI without current network state and a
  usable entry path is not ready.
- Software installation, operating-system startup, subsequent Service
  connection work, and blocked or hostile entry are outside these startup
  clocks.
- P3-D2c accepted: the controlled Named Unlisted Site produces its first valid
  HTTP response byte within `p95 <= 4 s` cold and `p95 <= 2 s` warm from a
  running, network-ready endpoint.
- Browser rendering, scripts, secondary assets, external resources, and
  arbitrary Application processing are not hidden inside this network metric.
- P3-D2 is complete: endpoint readiness, authenticated connection, and the
  controlled tracer first-byte experience now have separate clocks.
- P3-D3a accepted: the `p05` Application goodput over eligible 60-second runs is
  at least `min(10 Mbit/s, 50% of paired direct-baseline goodput)` in each
  direction for one established Service Connection.
- Only bytes delivered to the receiving Application count as useful payload;
  carrier overhead, failed runs, and the faster direction cannot inflate the
  result.
- P3-D3b1 revised and accepted: each required Windows and Linux client reference
  endpoint supports at least `64` concurrently open outbound Service
  Connections, with at least `16` simultaneously carrying the declared
  active-transfer workload.
- The initial `128/32` client floor was superseded because it was not justified
  by the V1 client journey after publisher capacity was separated.
- P3-D3b2 accepted: each required Windows and Linux publisher reference endpoint
  supports at least `256` concurrently open incoming Service Connections, with
  at least `64` simultaneously carrying the declared active-transfer workload.
- Both endpoint floors are minimum capacities rather than maxima. Exhaustion is
  bounded and explicit; publisher application policy may deliberately admit
  fewer connections without redefining the network benchmark.
- P3-D3b3 accepted: every selected V1 infrastructure role must demonstrate
  useful bounded operation on a Linux `2 vCPU`, `2 GiB RAM`, symmetric
  `100 Mbit/s` reference VPS.
- The reference VPS is not a capacity or trust ceiling. Stronger hardware may
  contribute more bounded work but receives no automatic additional role,
  authority, or route-selection privilege.
- Public DNS, naming design, bootstrap, routing, libraries, language, exact
  hardware, and the remaining numeric budgets remain unselected.
- P3-D3c, scale-up and resource behavior, is next; role-specific Node capacity
  is completed with R-004 candidate evidence under P3-D3b4.
- No ADR and no code.
