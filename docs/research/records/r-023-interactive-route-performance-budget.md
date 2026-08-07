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

### P3-D3c1 — Bounded local endpoint scale-up

**Product Owner decision, accepted 2026-08-07:** the client `64/16` and publisher
`256/64` capacities are qualification floors, not fixed implementation ceilings.
A stronger Windows or Linux machine may use additional CPU, memory, and network
capacity to support more bounded connections and aggregate work.

By default, the endpoint derives a conservative finite resource profile from
the resources available to its process and applies the accepted hierarchy:
Endpoint, Local Grant or Application, Service or Isolation Context, then
connection and operation. An Endpoint Owner may set a lower cap or raise the
automatic cap within enforceable finite safety bounds. Creating additional
Applications, Services, grants, or contexts never multiplies an ancestor budget.

If effective local limits fall below the accepted client or publisher floor,
the endpoint may still operate but must expose reduced local capacity and cannot
claim the corresponding V1 performance qualification. Exhaustion remains an
explicit bounded resource result rather than a crash, hang, unbounded queue,
silent eviction, or security downgrade.

Scaling changes capacity only. It never:

- makes a client or publisher an infrastructure Node without explicit
  Network Contributor opt-in and role configuration;
- grants trust, governance authority, route-selection priority, or access to
  another Local Grant, Service, or Isolation Context;
- permits direct fallback, weaker Route Knowledge Separation, skipped target
  authentication, or forbidden cross-context state sharing;
- requires publishing exact CPU, memory, hardware identity, or configured local
  limits as network metadata.

Traffic volume, admission outcomes, and timing may still allow peers or
observers to infer rough capacity; the contract prevents an explicit stable
hardware identity, not all inference. Linear scaling is not promised. P3-D3c2
must measure where CPU, memory, bandwidth, contention, and privacy-preserving
isolation stop producing useful additional capacity on each reference class.

### P3-D3c2a — Idle client resource ceiling

**Product Owner decision, accepted 2026-08-07:** on each required Windows and
Linux client reference endpoint, a network-ready Ardents client with no open
Service Connections, no published Service, and no infrastructure Node role is
observed continuously for 10 minutes beginning immediately after it reports
network readiness. Across the complete Ardents process tree, including helper
processes, it must meet both:

- `p95 resident memory <= 256 MiB`;
- mean CPU use over the 10-minute window `<= 1%` of one logical CPU core, with
  `100%` normalized to one fully occupied logical core.

This is a top-down product ceiling, not a measured implementation claim. It is
provisional until P3-D6 defines and runs the Windows and Linux reference
hardware, process-tree attribution, uniform sampling interval, run count, and
cross-platform resident-memory calculation. The memory percentile is computed
from eligible samples rather than a convenient final snapshot; the CPU target
allows short bounded bursts but not sustained background work hidden by process
splitting.

The endpoint must remain in the accepted network-ready state throughout the
window, retain current authenticated state and a usable entry path, and perform
all background control, validation, liveness, and security work required for an
immediate Application request. It cannot satisfy the budget by disconnecting,
pausing maintenance, deferring required validation or updates, weakening
security, or moving work outside the counted process tree. Normal endpoint
background network bytes are deliberately not included in this CPU and memory
decision; P3-D3c2b gives them a separate measurable budget.

### P3-D3c2b — Idle background carrier overhead

**Product Owner decision, accepted 2026-08-07 as a secondary efficiency
guardrail:** on each required Windows and Linux client reference endpoint, an
already joined client that remains network-ready for a continuous 24-hour
steady-idle window sends and receives at most `25 MiB` of Ardents-attributable
carrier traffic in total, approximately `750 MiB` per 30 days.

The workload has no open Service Connections, no published Service, and no
infrastructure Node role, and runs on the normal stable non-adversarial
reference network. The numerator combines both directions and includes Ardents
control messages, keepalives, network-state refresh, update checks and metadata,
retransmissions, padding, and any cover traffic. P3-D6 must define the exact
operating-system attribution boundary and include transport and network framing
consistently on Windows and Linux; moving traffic to a helper process does not
remove it from the total.

The following are outside this steady-idle budget and require separately visible
measurements rather than disappearing from evidence:

- the initial bootstrap and full state acquisition of a clean first start;
- an explicitly initiated software-package payload download or installation;
- blocked-entry, partition, or degraded-network recovery covered by R-009 and
  P3-D4.

This number is a top-down, unverified optimization target rather than a
standalone V1 release blocker or a runtime quota. Exceeding it in an otherwise
qualified design requires explanation, retained evidence, and an optimization
decision, but the client must not disconnect, delay required security or
liveness work, or weaken R-001 merely to pass. The limit neither requires nor
authorizes constant-rate padding or a periodic fingerprint. Under R-001, hidden
V1 cover traffic is not part of the baseline Interactive Route claim; a stronger
profile that needs it belongs to R-005 with its own security and performance
budget.

### P3-D3c2c1 — Active client resource ceiling

**Product Owner decision, accepted 2026-08-07:** on each required Windows and
Linux client reference endpoint, the complete Ardents process tree must meet
both of the following during a continuous 10-minute active-client window:

- `p95 resident memory <= 512 MiB`;
- mean CPU use `<= 50%` of one logical CPU core, with `100%` normalized to one
  fully occupied logical core.

The window begins after `16` outbound Service Connections are authenticated and
usable and their fixed incompressible transfer begins. All `16` connections are
continuously offered an equal share of `10 Mbit/s` aggregate Application Data,
their receivers consume without Application backpressure, and every connection
must continue delivering payload throughout the run. The test is repeated as a
User-to-Service run and a Service-to-User run; resource success in one direction
cannot compensate for failure in the other. This is `10 Mbit/s` total across the
connections, not per connection.

Only Application Data delivered to the receivers establishes the useful load.
Protocol headers, encryption, routing control, retransmissions, padding, and
other carrier overhead do not inflate the `10 Mbit/s`, but their client CPU and
memory cost remains inside the measured process tree. The controlled
Applications and remote Service processes are outside the client resource
numerator; moving Ardents work to a helper process is not.

This is a top-down qualification ceiling rather than a current implementation
claim. P3-D6 must define the reference endpoint hardware, sample interval,
process-tree and cross-platform resident-memory attribution, eligible-run count,
and exact progress evidence. A run that stalls a connection, fails to sustain
the declared delivered load, loses a connection, exceeds either resource
ceiling, or bypasses authentication, Route Knowledge Separation, Isolation
Contexts, required background work, or bounded queues is a miss. The exact
fair-share threshold and active carrier-bandwidth overhead remain P3-D3c2c3.

## Remaining decisions

1. **P3-D3c2c2 — Active publisher resource ceiling:** set CPU and memory budgets
   for the accepted `64`-active publisher workload on each required publisher
   reference endpoint.
2. **P3-D3c2c3 — Active overhead, fairness, and scale-up:** set active
   carrier-bandwidth overhead, aggregate publisher goodput, queue and
   per-connection fair-progress budgets, and measure endpoint scale-up
   saturation.
3. **P3-D3b4 — Role-specific Node capacity:** after R-004 defines candidate
   units of work, set entry, relay, discovery, Rendezvous, and Bridge capacity
   floors on the accepted reference class.
4. **P3-D4 — Tail and degradation:** set jitter, loss, churn, alternate-route,
   and overload behavior without weakening R-001.
5. **P3-D5 — Hostile load:** define fairness and resource-exhaustion workloads
   and the useful honest-work floor during attack.
6. **P3-D6 — Measurement gate:** define direct baselines, topology, repetitions,
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
- **Product Owner decision:** client and publisher floors scale upward through
  conservative finite local resource profiles on stronger hardware; exact
  capacity is local, grants no authority or role, and is not required network
  metadata.
- **Product Owner decision:** for the first 10 minutes after network readiness,
  an otherwise idle required client keeps whole-process-tree
  `p95 resident memory <= 256 MiB` and mean CPU `<= 1%` of one logical core
  without leaving or weakening the network-ready state.
- **Product Owner decision:** an already joined, network-ready client uses no
  more than `25 MiB` of total sent-plus-received Ardents carrier traffic during
  24 hours of steady idle. This is a secondary efficiency guardrail rather than
  a standalone release blocker.
- **Product Owner decision:** with `16` continuously active outbound connections
  sharing `10 Mbit/s` of delivered Application Data for 10 minutes, the complete
  client process tree keeps `p95 resident memory <= 512 MiB` and mean CPU
  `<= 50%` of one logical core in separate runs for each direction.
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
publisher concurrency floors and the accepted idle client CPU and memory
ceiling. Treat the accepted idle background-traffic number as an optimization
guardrail, apply the accepted active client resource ceiling, then define active
publisher resources and fairness, infrastructure Node capacity, degradation,
hostile load, and the reproducible release gate; bounded endpoint scale-up is
already fixed.

Confidence: high for the platform boundary and desired connection experience;
the accepted latency, goodput, client-concurrency, and publisher-concurrency
targets, infrastructure reference class, idle client resource ceiling, and
active client resource ceiling remain unverified. The idle carrier budget is
also unverified and deliberately secondary; the remaining numeric targets
remain undecided. The strongest counterargument is that
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
- P3-D3c1 accepted: supported endpoints derive conservative finite hierarchical
  budgets from available resources; stronger machines may exceed the client and
  publisher floors, and Endpoint Owners may bound or raise the automatic cap.
- Operating below a floor is allowed only as explicitly reduced local capacity,
  not as a qualified V1 performance result. More hardware grants no automatic
  Node role, trust, authority, route priority, cross-context access, or security
  shortcut, and exact hardware limits are not required network metadata.
- P3-D3c2a accepted: during the 10-minute window beginning at network readiness,
  an otherwise idle required Windows or Linux client keeps whole-process-tree
  `p95 resident memory <= 256 MiB` and mean CPU `<= 1%` of one logical core.
- The client remains genuinely network-ready and performs required background
  and security work throughout the idle measurement. Process splitting,
  disconnection, deferred validation, or weakened security cannot make a run
  pass; exact sampling and platform attribution remain P3-D6 evidence.
- P3-D3c2b accepted as a secondary efficiency guardrail: an already joined,
  network-ready client uses no more than `25 MiB` of combined sent and received
  Ardents carrier traffic over 24 hours of normal steady idle.
- Clean-start bootstrap, explicit software-package payloads, and blocked or
  degraded recovery are measured separately. The guardrail is not a quota or a
  standalone release blocker, never justifies suppressing required traffic, and
  adds no hidden cover-traffic requirement to the baseline Route Profile.
- P3-D3c2c1 accepted: for separate 10-minute transfer runs in each direction,
  the complete required client process tree keeps
  `p95 resident memory <= 512 MiB` and mean CPU `<= 50%` of one logical core
  while `16` active connections share `10 Mbit/s` of delivered Application Data.
- Every connection must keep delivering payload. Lost or stalled connections,
  reduced useful load, process splitting, unbounded queues, or a security or
  isolation shortcut cannot make the resource test pass; exact measurement and
  fair-share evidence remain P3-D6 and P3-D3c2c3.
- Public DNS, naming design, bootstrap, routing, libraries, language, exact
  hardware, and the remaining numeric budgets remain unselected.
- P3-D3c2c2, the active publisher resource ceiling, is next. Active overhead,
  quantitative fairness, and scale-up saturation remain P3-D3c2c3;
  role-specific Node capacity is completed with R-004 candidate evidence under
  P3-D3b4.
- No ADR and no code.
