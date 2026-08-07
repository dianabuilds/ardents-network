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

- a local endpoint used by a User on a Windows 11 `x86-64` desktop or laptop;
- a local endpoint used by a User on an Ubuntu LTS `x86-64` desktop or laptop;
- a local endpoint used by a Developer to publish an ordinary local Application
  on the same Windows 11 and Ubuntu LTS device classes;
- an infrastructure Node on a modest Ubuntu LTS `x86-64` server or VPS.

This does not require infrastructure Nodes to support Windows and does not
forbid them from doing so. macOS, phones, tablets, and other constrained devices
are later compatibility and measurement targets, not V1 performance or release
gates. P3-D6b2b1 fixes `x86-64`, Windows 11, Ubuntu LTS, and the endpoint
hardware class, while P3-D6b2b2a fixes the normal reference-network envelope.
Exact point releases, images, host CPU model, and reproducible impairment traces
remain candidate measurement details.

This platform decision does not select public DNS, naming, or bootstrap.
Service Name resolution remains an internal Ardents product function under
R-003, while authenticated entry and recovery remain R-009. A User connecting
from Windows 11 or Ubuntu LTS does not imply exposing a Service Name or origin
through public DNS.

Consequences:

- a benchmark that runs only on a developer workstation or Ubuntu server cannot
  establish V1 client performance;
- publishing from an ordinary Windows 11 or Ubuntu LTS device is a required
  product journey, not a server-only extension;
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

- **cold connection:** `p95 <= 3 s` when the endpoint is network-ready but no
  naming, reachability, Route, session, or cache state has been prepared for the
  exact Service Name, Service Target, Isolation Context, and Route Profile;
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
  network state is available but no prior process or live connection state
  remains;
- **clean first start:** `p95 <= 15 s` when only installed immutable inputs are
  available and no state generated by an earlier Ardents execution exists.

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

- **cold site open:** `p95 <= 4 s` when the endpoint is network-ready but no
  naming, reachability, Route, session, or cache state has been prepared for the
  exact Service Name, Service Target, Isolation Context, and Route Profile;
- **warm site open:** `p95 <= 2 s` when current authenticated naming and
  reachability state and reusable Route state for the same Isolation Context are
  already available, but no Service Connection or Application response cache is
  available.

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
Windows 11 and Ubuntu LTS `x86-64` client reference endpoint must support at
least:

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
P3-D3c2c1 transfer workload and must make Application Data progress throughout
the measurement interval; opening idle sockets or moving one token byte cannot
satisfy the `16`-connection case. P3-D6 retains the exact topology, sampling,
run count, and evidence format.

The single-connection P3-D3a goodput floor does not apply independently to all
16 active connections. P3-D3c2c1 sets their aggregate goodput and resource
ceiling, and P3-D3c2c3a sets their equal-load fair-progress floor; none is
inferred from the connection count alone.

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
reversed for accepted incoming Service Connections. P3-D3c2c2 fixes the
64-connection workload, duration, aggregate goodput, CPU, and memory, while
P3-D3c2c3a fixes its equal-load fair-progress floor. The metric does not require
the P3-D3a single-connection goodput floor independently on all 64 active
connections; P3-D3c2c3b fixes active endpoint carrier overhead.

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
A stronger Windows 11 or Ubuntu LTS machine may use additional CPU, memory, and
network capacity to support more bounded connections and aggregate work.

By default, the endpoint derives a conservative finite resource allowance from
the resources available to its process, selects no higher than a compatible
qualified profile, and applies the accepted hierarchy: Endpoint, Local Grant or
Application, Service or Isolation Context, then connection and operation. An
Endpoint Owner may set a lower cap. An explicit higher finite experimental cap
remains unqualified under P3-D3c2c3c3 and is never selected automatically.
Creating additional Applications, Services, grants, or contexts never
multiplies an ancestor budget.

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
isolation stop producing useful additional capacity on each reference class;
P3-D3c2c3c3 fixes the automatic-profile saturation rule.

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
provisional until P3-D6 defines and runs the Windows 11 and Ubuntu LTS reference
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
guardrail:** on each required Windows 11 and Ubuntu LTS `x86-64` client
reference endpoint, an
already joined client that remains network-ready for a continuous 24-hour
steady-idle window sends and receives at most `25 MiB` of Ardents-attributable
carrier traffic in total, approximately `750 MiB` per 30 days.

The workload has no open Service Connections, no published Service, and no
infrastructure Node role, and runs on the normal stable non-adversarial
reference network. The numerator combines both directions and includes Ardents
control messages, keepalives, network-state refresh, update checks and metadata,
retransmissions, padding, and any cover traffic. P3-D6 must define the exact
operating-system attribution boundary and include transport and network framing
consistently on Windows 11 and Ubuntu LTS; moving traffic to a helper process
does not remove it from the total.

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
Contexts, required background work, or bounded queues is a miss. P3-D3c2c3a
fixes the fair-progress floor, and P3-D3c2c3b fixes active endpoint carrier
overhead; P3-D3c2c3c1 fixes the combined `64`-open and `16`-active workload.

### P3-D3c2c2 — Active publisher resource ceiling

**Product Owner decision, accepted 2026-08-07:** on each required Windows and
Linux publisher reference endpoint, the complete Ardents process tree must meet
both of the following during a continuous 10-minute active-publisher window:

- `p95 resident memory <= 1 GiB`;
- mean CPU use `<= 100%` of one logical CPU core, with `100%` normalized to one
  fully occupied logical core.

The window begins after `64` incoming Service Connections are authenticated and
usable and their fixed incompressible transfer begins. All `64` connections are
continuously offered an equal share of `40 Mbit/s` aggregate Application Data,
their receivers consume without Application backpressure, and every connection
must continue delivering payload throughout the run. The test is repeated as a
User-to-Service run, with the publisher receiving, and a Service-to-User run,
with the publisher sending. Resource success in one direction cannot compensate
for failure in the other. This is `40 Mbit/s` total across the connections, not
per connection.

Only Application Data delivered to the receivers establishes the useful load.
Protocol headers, encryption, routing and publication control,
retransmissions, padding, and other carrier overhead do not inflate the
`40 Mbit/s`, but their publisher CPU and memory cost remains inside the measured
process tree. The controlled published Application and remote client processes
are outside the publisher resource numerator; moving Ardents work to a helper
process is not.

This is a top-down qualification ceiling rather than a current implementation
claim. P3-D6 must define the reference publisher hardware, sample interval,
process-tree and cross-platform resident-memory attribution, eligible-run count,
and exact progress evidence. A run that stalls a connection, fails to sustain
the declared delivered load, loses a connection, exceeds either resource
ceiling, or bypasses authentication, Route Knowledge Separation, publication
isolation, required background work, or bounded queues is a miss. P3-D3c2c3a
fixes the fair-progress floor, P3-D3c2c3b fixes active endpoint carrier
overhead, and P3-D3c2c3c1 fixes the combined `256`-open and `64`-active
workload.

### P3-D3c2c3a — Equal-load fair progress

**Product Owner decision, accepted 2026-08-07:** in every eligible client and
publisher active-resource run, each active Service Connection must meet both:

- mean delivered Application goodput over the full 10-minute run
  `>= 500 kbit/s`;
- no continuous interval longer than `2 s` with zero Application Data delivered
  to its receiving Application.

The client and publisher workloads offer every connection an equal
`625 kbit/s` share, so the mean floor is 80% of that share. Both conditions apply
independently to every connection and to the separate User-to-Service and
Service-to-User runs. Aggregate `10 Mbit/s` or `40 Mbit/s` success, a fast
connection, or success in the opposite direction cannot compensate for one
starved connection. Only delivered Application Data proves progress; carrier
control, acknowledgements, padding, and retransmitted bytes do not.

The controlled test gives the connections equal Local Grant priority and
budgets, equivalent reference paths, continuously available incompressible
payload, and non-blocking receivers. The contract therefore measures starvation
introduced by Ardents scheduling and resource handling, not a promise that
production connections with different grants, access links, Routes, remote
behavior, or Application backpressure receive equal throughput. Deliberately
unequal local policy remains allowed within the accepted finite hierarchy.

This is a top-down, unverified qualification floor. P3-D6 must define timestamp
resolution, eligible runs, path equivalence, and retained per-connection traces.
A loss, close, average below the floor, or gap above the limit is a miss rather
than an omitted sample. Meeting fairness by reducing the accepted aggregate
load, weakening security or isolation, sharing forbidden cross-context state,
or buffering without bound also fails the run. Degraded and hostile-path
fairness remain P3-D4 and P3-D5.

### P3-D3c2c3b — Active endpoint carrier overhead

**Product Owner decision, accepted 2026-08-07:** in every eligible 10-minute
client and publisher active-resource run, the active endpoint carrier ratio must
be `<= 1.5` separately for each benchmarked endpoint and transfer direction:

`(Ardents-attributable bytes sent + received) / delivered Application Data`

The numerator is measured at the operating-system network boundary of the
endpoint under test during the active transfer window. It includes transport and
network framing visible at that boundary, encrypted carrier payload, protocol
control, acknowledgements, keepalives, retransmissions, padding, cover traffic,
and required background traffic attributable to Ardents or its helper processes.
Unrelated operating-system and Application traffic is excluded. P3-D6 fixes one
cross-platform attribution boundary and packet-accounting method so Windows and
Linux results are comparable.

The denominator contains only incompressible Application Data delivered to the
receiving Application in the tested direction. Carrier bytes, a faster opposite
direction, dropped data, and bytes accepted locally but not delivered remotely
cannot inflate it. At the accepted aggregate loads, the ratio means that
`10 Mbit/s` of Application Data permits at most `15 Mbit/s` of combined sent and
received endpoint traffic on average, while `40 Mbit/s` permits at most
`60 Mbit/s`.

The ratio applies independently at each measured client or publisher endpoint;
it does not sum the traffic of intermediate Nodes. Route-wide bandwidth and
role-specific forwarding cost remain P3-D3b4 with R-004 evidence. Connection
setup is outside the already-established transfer window, while all required
maintenance during the window remains counted.

This is a top-down, unverified qualification ceiling for the baseline
Interactive Route on the normal stable non-adversarial reference network. A run
over `1.5` is a miss, but an implementation cannot pass by suppressing required
authentication, Route Knowledge Separation, integrity, isolation, liveness, or
fail-closed behavior. Hidden V1 cover traffic is not a baseline requirement; a
stronger R-005 profile may choose a different explicit security and bandwidth
budget. Loss, churn, recovery, and hostile traffic remain P3-D4 and P3-D5.

### P3-D3c2c3c1 — Combined open-and-active endpoint load

**Product Owner decision, accepted 2026-08-07:** the client and publisher active
qualification runs use the full accepted open-connection capacity throughout
the same continuous 10-minute window:

- client: `64` concurrently open outbound Service Connections, of which `16`
  carry the accepted aggregate `10 Mbit/s` active workload;
- publisher: `256` concurrently open incoming Service Connections, of which
  `64` carry the accepted aggregate `40 Mbit/s` active workload.

The active subset must simultaneously retain every accepted direction-specific
contract: client or publisher CPU and resident-memory ceiling, aggregate
delivered Application Data, per-connection `500 kbit/s` mean, maximum `2 s`
zero-delivery gap, and `1.5x` endpoint carrier ratio. The test is still repeated
separately for User-to-Service and Service-to-User transfer.

The other `48` client or `192` publisher connections remain authenticated to the
exact Service Target, open, and usable Application-facing byte streams but do
not carry the fixed active workload. They continue all required control,
liveness, rekeying, and safe Route maintenance, whose CPU, memory, and endpoint
carrier costs remain inside the same budgets. They cannot be represented by
local handles after their network state was silently closed or discarded.

P3-D6 must use unpredictable bounded Application canaries across the inactive
set throughout the window, including at its end, to prove that the same
Application-facing Service Connection can still deliver bytes without a new
connect operation or replacement Connection Result. Canary Application Data
does not inflate the active goodput or carrier-ratio denominator, while its
carrier and endpoint resource cost remains counted. A connection may perform
transparent safe internal maintenance without ceasing to be the same stream
contract.

This is a top-down, unverified qualification workload rather than a fixed
implementation limit. Any open-connection loss, canary failure, active-metric
failure, hidden eviction, unbounded buffering, process offload, cross-context
state reuse, or security downgrade is a miss. An endpoint configured below the
accepted floor may expose reduced local capacity under P3-D3c1, but cannot use
that configuration to pass V1 client or publisher qualification.

### P3-D3c2c3c2 — Queue and backpressure ceilings

**Product Owner decision, accepted 2026-08-07:** every required V1 client and
publisher reference profile applies these hard logical Application Data queue
ceilings separately in both stream directions:

- no more than `256 KiB` per established Service Connection and direction;
- no more than `16 MiB` across the complete client endpoint per direction;
- no more than `64 MiB` across the complete publisher endpoint per direction.

The client and publisher aggregate values equal the per-connection ceiling at
their accepted `64`- and `256`-connection floors. They are parent ceilings, not
capacity created for every Local Grant, Application, Service, or Isolation
Context. Child scopes share and subdivide the applicable endpoint budget and
cannot multiply it. A lower configured local limit must be visible as reduced
capacity and still preserve honest backpressure. Any higher aggregate on a
stronger endpoint belongs to P3-D3c2c3c3 scale-up evidence; the per-connection
ceiling does not rise implicitly. Enabling both outbound use and publication on
one endpoint does not automatically add the client and publisher parent caps;
the combined-role profile remains finite and requires explicit qualification.

The logical queue covers every Application Data byte retained locally by the
Ardents Application Interface or carrier while responsibility has not yet moved
to the next bounded consumer:

- outbound data counts after the local Application Interface accepts it and
  until the remote endpoint's transport flow control confirms admission into
  that endpoint's bounded receive path; this is not evidence that the remote
  Application read or processed it;
- inbound data counts after authenticated transport admission and until the
  local Application consumes it through the Application Interface.

Attributable operating-system or IPC buffers at the Application Interface are
inside this logical boundary. One logical byte is counted once per local
endpoint and direction even if an implementation has several physical or
encrypted representations. All physical representations remain bounded:
process-resident copies, retransmission structures, packet buffers,
cryptographic workspaces, and control buffers contribute to the accepted RSS
ceiling, while attributable kernel or IPC capacity is covered by the logical
cap and separately recorded OS-resource evidence. A candidate cannot pass by
moving data to an uncounted stage or by spilling an unbounded queue to disk.

Reaching either the connection or an ancestor ceiling makes the producer-facing
stream stop accepting further bytes until capacity becomes available. A
blocking interface blocks and a non-blocking interface reports its ordinary
would-block state; timeout or cancellation applies only to the portion not yet
accepted. A successful local write reports only its accepted prefix. Receiver
flow control propagates a slow consumer back toward the producer rather than
allowing another queue to grow.

Queue pressure by itself does not close or evict an established connection and
does not discard, reorder, or claim delivery of accepted bytes. A connection
that cannot continue for an independent failure still returns the existing
explicit Connection Result, and Ardents does not replay an Application
operation. If a new connection cannot obtain its finite initial resources, its
admission fails explicitly as a local resource limit instead of borrowing an
unbounded queue from another scope.

P3-D6 must record instantaneous and high-water logical queued bytes per
connection, endpoint, and direction, plus time spent backpressured. Dedicated
slow-reader and slow-writer runs fill each leaf and parent ceiling while
checking ordering, accepted-prefix reporting, peer progress, isolation, RSS,
and recovery after the consumer resumes. Any sample over a hard logical cap,
successful acceptance beyond available capacity, silent loss or eviction,
cross-scope borrowing, unbounded memory or disk growth, or security downgrade
fails qualification.

### P3-D3c2c3c3 — Scale-up saturation evidence

**Product Owner decision, accepted 2026-08-07:** a stronger client or publisher
may enable a higher automatic capacity profile only after that profile proves
useful scaled work in the complete 10-minute combined open-and-active test and
retains at least `20%` headroom in every declared parent CPU, memory, and usable
link budget.

A higher profile declares a scale factor `S > 1` against the applicable V1
reference profile. Its qualification workload scales together:

| Profile | Open connections | Active connections | Aggregate delivered Application Data | Aggregate logical queue cap per direction |
|---|---:|---:|---:|---:|
| Client | `64 * S` | `16 * S` | `10 * S Mbit/s` | at most `16 * S MiB` |
| Publisher | `256 * S` | `64 * S` | `40 * S Mbit/s` | at most `64 * S MiB` |

Connection counts are whole numbers. The claimed `S` cannot exceed the weakest
ratio actually demonstrated among open capacity, active capacity, and aggregate
delivered Application Data. More idle handles alone are not useful scale-up.
The per-connection logical queue ceiling remains `256 KiB` in each direction;
only the finite endpoint aggregate may grow with a qualified profile.

Each profile declares finite resources available to Ardents after the
operating-system reserve, conservative automatic allowance, and any lower
Endpoint Owner cap. During each direction-specific active run:

- mean whole-Ardents-process-tree CPU use is at most `80%` of the profile's CPU
  parent budget;
- whole-Ardents-process-tree `p95` resident memory is at most `80%` of the
  profile's memory parent budget;
- `p95` one-second Ardents carrier bitrate in each physical link direction is
  at most `80%` of that direction's measured usable link budget.

Declaring a budget larger than the resources safely available to the process
does not create headroom. P3-D6 must fix the operating-system reserve, usable
link baseline, sampling, repetitions, and hardware envelope for each profile.
The existing absolute client and publisher resource ceilings continue to
qualify the `S = 1` reference profiles; a higher profile uses its declared
finite parent budgets and the `20%` reserve without weakening the reference
claim.

Every other applicable accepted gate remains enabled: all declared connections
stay open and usable, the scaled useful load is delivered, every active
connection retains the fair-progress floor and maximum no-progress gap, endpoint
carrier overhead remains `<= 1.5x`, queue and backpressure semantics hold, and
authentication, Route Knowledge Separation, isolation, and required background
work are unchanged. Startup, connection latency, and single-connection goodput
remain separate applicable gates rather than being hidden inside the active
window.

Profiles are tested in increasing order for the same implementation and claimed
hardware and operating-system envelope. The first profile that fails any
applicable metric or loses the `20%` reserve is the measured saturation point;
it and larger profiles are ineligible for automatic selection on that envelope
until new qualification evidence passes. One successful larger run cannot skip
a confirmed failed profile.

At runtime, the endpoint selects no higher than the greatest qualified profile
compatible with its currently available finite resources. The Endpoint Owner
may always cap it lower. An explicit finite experimental override above the
automatic cap remains unqualified, is never selected automatically, and cannot
be presented as a V1 performance result. It still obeys every security,
isolation, queue, and explicit-overload invariant.

This defines useful capacity evidence, not linear hardware scaling. Additional
CPU, RAM, or bandwidth grants no role, authority, trust, route priority, or
security exception, and the selected profile is not required network metadata.

### P3-D4a — Single-failure Service Connection recovery

**Product Owner decision, accepted 2026-08-07:** seamless recovery of the same
logical Service Connection is mandatory in V1 when one ordinary Node or one
transport-specific Carrier Channel on its current Interactive Route becomes
unavailable and a qualifying alternate Route remains. The chosen HTTP, WSS,
TCP, UDP, QUIC, or other carrier mechanism may change how this is implemented;
it does not change the Application-facing outcome.

The eligible controlled scenario has all of these conditions:

- the Service Connection is already established and continuously carries the
  fixed incompressible Application Data workload in the measured direction
  without Application backpressure, cancellation, close, or a shorter
  Application timeout;
- both endpoints and the same active Service Instance remain online, and the
  authenticated Service Target does not change;
- exactly one ordinary Node or one Carrier Channel on the current Route stops
  carrying traffic;
- at least one alternate path exists that satisfies the same Route Profile,
  target-authentication, isolation, and resource-safety requirements;
- there is no simultaneous endpoint access outage, Service outage, second path
  failure, broad blocking event, or detected authenticity, integrity, replay,
  redirection, or downgrade violation.

Detected active violations remain governed by R-001 fail-closed semantics and
cannot be converted into a recovery success. Repeated churn, loss, jitter,
multiple failures, overload, censorship, and hostile recovery flooding remain
separate P3-D4b and P3-D5 workloads.

For each eligible run, the recovery clock begins at the end of the last
Application Data byte delivered before the injected failure. After injection,
the sender creates an unpredictable recovery canary that could not already be
buffered; the clock ends when its first ordered byte is delivered through the
same Service Connection over the recovered path. The target is `p95 <= 5 s`,
measured separately in the User-to-Service and Service-to-User directions.
Detection delay, alternate Route construction, Carrier Channel replacement,
authentication, queued predecessor bytes, and safe stream continuation are all
inside the clock.

If continuation has not succeeded by `15 s` after the last delivered byte, the
Service Connection terminates with an explicit supported Connection Result; it
cannot remain apparently open or silently become a new connection. Every such
terminal result is a recovery miss and remains in qualification evidence rather
than disappearing from the percentile population. P3-D6 fixes repetitions and
the allowed miss rate.

Successful recovery preserves the authenticated Service Target, Isolation
Context, Route Profile, byte order, and the same Application-facing Service
Connection. It neither loses nor duplicates bytes presented at the Application
Interface and requires no Application-visible reconnect. Carrier-level
retransmission is permitted only to preserve this reliable stream. Ardents does
not reissue an Application operation, and a terminal failure leaves remote
Application completion unknown under R-002.

Recovery may replace one or every Carrier Channel and may use a different
carrier mechanism if the candidate supports it. No specific carrier protocol is
selected here. If a carrier cannot provide continuity itself, the complete
Ardents candidate must provide it above that carrier; a full stack that cannot
meet the same semantics and budgets does not qualify for the V1 Interactive
Route.

Replacement cannot use a direct or ordinary-network fallback, a weaker Route
Profile, a different target, forbidden cross-context state, or a security
downgrade. Continuity state may link the old and new carrier binding at the two
endpoints as required, but cannot become a stable network identity, be shared
across Isolation Contexts, or expose the full Route to an ordinary Node.

P3-D6 must inject the eligible failure at every ordinary Route position exposed
by a candidate, generate the post-injection recovery canary, and retain delivery
traces, Connection Results, Route and Carrier Channel events, CPU, RSS, queued
bytes, endpoint traffic, and security checks. Pre-failure buffered data cannot
end the clock. The `5 s` and `15 s` values are top-down unverified product
targets, not claims that any individual transport already provides them.

### P3-D4b1 — Three sequential recoveries under ordinary churn

**Product Owner decision, accepted 2026-08-07:** one established Service
Connection must survive three sequential eligible ordinary-Node or Carrier
Channel failures during one continuous 10-minute qualification run. Each event
retains the P3-D4a `p95 <= 5 s` recovery target and `15 s` terminal deadline.

The window starts after the exact Service Target is authenticated, the Service
Connection is usable, and the fixed incompressible workload is delivering in
the measured direction without Application backpressure, cancellation, close,
or a shorter Application timeout. The test is repeated separately for
User-to-Service and Service-to-User delivery.

The three injections are sequential rather than overlapping:

- each affects an ordinary Node or Carrier Channel carrying the connection on
  its then-current Interactive Route;
- the next injection occurs only after the preceding unpredictable recovery
  canary has been delivered through the same Service Connection;
- a failed Node remains unavailable for the rest of the run, and a failed
  Carrier Channel instance cannot be reused, so later events require genuine
  continued adaptation rather than a return to the same failed resource;
- both endpoints, the same active Service Instance and Service Target, and at
  least one qualifying alternate Route remain available after every injection;
- injection timing and eligible Route position are not disclosed to the
  candidate in advance.

Each event has its own P3-D4a recovery clock from the last pre-failure delivered
byte to a newly generated post-injection canary delivered over the recovered
path. Pre-failure buffering cannot end a clock. Every event remains in the same
recovery percentile and miss-rate population; aggregating the three into one
average cannot hide a slow or failed recovery.

The run succeeds only if all three canaries arrive without a terminal
Connection Result and the same Service Connection remains authenticated, open,
ordered, non-duplicating, and usable through the end of the 10-minute window.
After the third recovery the workload and an unpredictable final canary continue
to prove that Ardents did not treat successful recovery number three as a reason
to close or silently replace the stream.

Every target, Isolation Context, Route Profile, queue, resource-safety,
fail-closed, and no-direct-fallback invariant remains enabled. Carrier-level
retransmission may preserve the stream, but neither an individual event nor the
sequence may reissue an Application operation, expose a stable continuity
identity, or share recovery state across Isolation Contexts. Recovery traffic
and CPU, RSS, queue, and link cost remain visible evidence; their degraded-load
ceilings are completed by P3-D4b2c and P3-D6.

Three is a minimum qualification workload, not a runtime recovery quota or an
availability promise for exactly three failures. Ardents cannot intentionally
terminate a healthy recovered connection merely because the third event
completed. Behavior beyond this workload remains finite, secure, and explicit,
but has no numeric repeated-churn promise until separately qualified.

### P3-D4b2a — Useful progress on an impaired but live Route

**Product Owner decision, accepted 2026-08-07:** one established Service
Connection must remain useful for a continuous 10-minute run under this
controlled degraded-path profile:

- `300 ms` base end-to-end RTT;
- `5%` independently injected packet loss in each network direction;
- additional per-direction delay variation whose `p95` is `100 ms`;
- no complete interruption of the path during the run.

P3-D6 fixes the reproducible delay and loss distributions, sampling, and
topology while preserving these accepted profile values. This is a deliberately
impaired qualification boundary, not the normal reference network and not a
claim that production impairments will be independent.

The run begins only after the exact Service Target is authenticated and the
same Service Connection is usable. A fixed incompressible workload then offers
enough data to keep the measured direction active without Application
backpressure, cancellation, close, or a shorter Application timeout. The test
is run separately for User-to-Service and Service-to-User Application Data.

For each direction, the `p05` of 60-second Application-goodput windows must be
at least:

`min(2 Mbit/s, 25% of paired impaired direct-baseline goodput)`.

The paired direct baseline uses the same endpoints, direction, payload,
duration, and injected RTT, loss, and jitter, but no Ardents Route. Only
Application Data delivered to the receiving Application counts as goodput;
carrier bytes, retransmission, buffering, or progress in the opposite direction
cannot improve the result. P3-D6 fixes the window sampling and repetition count
and keeps eligible incomplete runs in the miss population.

No continuous zero-delivery interval may exceed `5 s` anywhere in the run. The
connection remains exact-target-authenticated, open, ordered, non-duplicating,
and usable as the same Application-facing stream throughout; an
Application-visible reconnect, hidden replacement Service Connection, byte
loss, or duplicate presentation fails the run.

Carrier retransmission, congestion control, and Carrier Channel adaptation may
preserve the stream, but cannot bypass the accepted queue ceilings, direct-path
prohibition, Isolation Context, Route Profile, authentication, or fail-closed
rules. Required security and liveness work cannot be disabled to meet the
goodput or progress floor. CPU, RSS, queue occupancy, carrier traffic, and Node
load remain retained evidence; their degraded-load ceilings are still part of
P3-D4b2c and P3-D6.

A complete traffic interruption is not this workload: it invokes the P3-D4a
recovery contract and remains in its recovery evidence. The impaired-live test
cannot silently discard an outage or relabel a newly opened Service Connection
as continuation. These numbers are top-down unverified product targets rather
than a claim about any transport, routing family, or implementation.

### P3-D4b2b — Overlapping eligible failures during recovery

**Product Owner decision, accepted 2026-08-07:** the same established Service
Connection must survive one pair of overlapping eligible ordinary-Node or
Carrier Channel failures during a continuous 10-minute qualification run.
Recovery delivers an unpredictable canary through the final recovered Route
within `p95 <= 8 s` from the first interruption and remains subject to one hard
`15 s` terminal deadline from that same point.

The run starts after exact Service Target authentication and useful delivery of
the fixed incompressible workload. It is repeated separately for
User-to-Service and Service-to-User Application Data without Application
backpressure, cancellation, close, or a shorter Application timeout.

The overlapping pair has this required shape:

- the first injection stops a distinct ordinary Node or Carrier Channel needed
  by the then-current Interactive Route, preventing that Route from continuing;
- within `1 s` of the first injection and before any recovery canary arrives,
  the second injection stops a different ordinary Node or Carrier Channel used
  by the in-progress replacement attempt, preventing that attempt from
  continuing;
- both failed resources remain unavailable through the rest of the run;
- both endpoints, the same active Service Instance and exact Service Target,
  and at least one further qualifying alternate Route that uses neither failed
  resource remain available after the second injection;
- injection positions and timing are controlled evidence but are not disclosed
  to the candidate in advance. P3-D6 fixes their reproducible selection across
  every eligible Route position exposed by a candidate.

This pair is one recovery episode. Its clock begins immediately after the last
Application Data byte delivered before the first injection and ends only when
a newly generated post-second-failure canary is delivered through the final
recovered path. Detection, abandoned recovery work, Route and Carrier Channel
replacement, authentication, queued predecessor bytes, and safe continuation
all count. The second failure, an internal retry, or selection of another
carrier cannot restart either the `8 s` target clock or the `15 s` deadline.

If no valid recovery canary has arrived by `15 s`, the Service Connection
terminates with an explicit supported Connection Result. That result remains a
recovery miss; the implementation cannot hang, discard the run, or silently
open a replacement connection. Every eligible episode remains in the same
percentile and miss-rate population fixed by P3-D6.

Success preserves the exact target, active Service Instance, Isolation
Context, Route Profile, stream identity, ordering, and same Application-facing
Service Connection without lost or duplicate byte presentation. Carrier-level
retransmission and parallel or abandoned candidate work are permitted only
inside the accepted finite resource and queue bounds. They cannot replay an
Application operation, reuse either failed resource, fall back directly, share
continuity state across Isolation Contexts, or weaken target authentication,
route privacy, integrity, or fail-closed handling.

The workload covers overlapping ordinary failures only while a further
qualifying Route remains. Detected active violations still fail closed, and the
contract does not claim survival after all qualifying paths are exhausted. The
`8 s` target is a top-down unverified product boundary, not evidence that a
specific route family, carrier, or implementation already provides it.

### P3-D4b2c1 — Endpoint resource cost under impairment and recovery

**Product Owner decision, accepted 2026-08-07:** in every direction-specific
10-minute P3-D4a, P3-D4b1, P3-D4b2a, and P3-D4b2b qualification run, each
required Windows 11 or Ubuntu LTS `x86-64` endpoint participating as the
User/client or Developer/publisher side must keep the complete Ardents process
tree within:

- `p95 resident memory <= 512 MiB`;
- mean CPU `<= 50%` of one logical core;
- `p95` one-second CPU `<= 100%` of one logical core.

These ceilings apply independently at both endpoints while the accepted useful
load, impairment, injections, progress, recovery clocks, and security checks
remain enabled. They include every Ardents process and helper attributable to
the endpoint, including Route and Carrier Channel work, cryptography,
retransmission, background security and liveness work, publication work, and
recovery bookkeeping. The external User or published Application's own process
work is excluded. P3-D6 fixes OS versions, reference processors, sampling, RSS
attribution, and CPU normalization.

The accepted logical queue ceiling remains `256 KiB` per Service Connection and
direction throughout every run. Existing endpoint and ancestor aggregate caps
also remain in force; recovery creates no separate queue allowance. At a full
cap the producer-facing stream applies honest backpressure. It cannot spill
without bound, borrow from another Local Grant or Isolation Context, discard or
duplicate bytes, or report false write success.

Temporary recovery state cannot accumulate with failure count. A completed or
abandoned Route attempt, Carrier Channel, timer, cryptographic context, queued
copy, task, and operating-system handle must cease to be live when no longer
needed by the current connection or recovery episode. A reusable cache may
remain only under a declared finite cap independent of the number of completed
failures and remains charged to RSS and other applicable budgets. P3-D6 retains
pre-event and post-event counts and resource traces; monotonic growth
attributable solely to completed or abandoned episodes fails qualification.

Resource and product outcomes are coequal gates. A candidate cannot pass by
missing the accepted goodput or recovery target, terminating an otherwise
eligible connection, opening a hidden replacement connection, suppressing
required security work, weakening the Route Profile or target authentication,
or moving work outside the measured process tree. A resource miss cannot be
removed from evidence, and a performance miss cannot be excused by staying
under the resource ceiling.

The limits deliberately give one impaired or recovering connection no more
endpoint CPU or memory than the already accepted complete normal client active
workload. They are top-down unverified product ceilings, not measurements of an
existing implementation. Infrastructure-Node cost remains deferred until R-004
defines candidate roles and units of work under P3-D3b4.

### P3-D4b2c2 — Endpoint carrier cost under impairment and recovery

**Product Owner decision, accepted 2026-08-07:** degraded operation and recovery
must satisfy three simultaneous endpoint carrier limits: a full-run impaired
ratio, a per-episode excess-byte cap, and a short-window bitrate cap. Passing one
cannot compensate for missing another.

For the direction-specific 10-minute P3-D4b2a impaired-live run, the endpoint
carrier ratio must be `<= 2.0` independently at each endpoint and measured
Application Data direction:

`(Ardents-attributable bytes sent + received) / delivered Application Data`.

The numerator is measured at the endpoint operating-system network boundary
over the full run. The denominator contains only fixed incompressible
Application Data delivered to the receiving Application in the measured
direction. Carrier bytes, bytes accepted but not delivered, progress in the
opposite direction, or a shorter surviving interval cannot inflate it. The
normal stable active-workload ratio remains `<= 1.5`; this `2.0` ceiling is only
the explicit cost allowance for the accepted impaired-live profile.

Every P3-D4a recovery, each of the three P3-D4b1 sequential recoveries, and the
whole P3-D4b2b overlapping pair as one recovery episode may create at most
`8 MiB` of additional Ardents-attributable carrier traffic at each endpoint.
The cap uses combined sent and received endpoint bytes and compares the episode
against a paired no-failure run with the same endpoints, initial authenticated
state, offered Application workload, Route Profile, carrier environment, and
wall-clock interval. P3-D6 fixes the paired calculation and must prevent reduced
Application delivery, omitted background work, or a shortened interval from
artificially lowering the excess. A negative difference is treated as zero and
cannot offset another episode.

Throughout every 10-minute P3-D4a, P3-D4b1, P3-D4b2a, and P3-D4b2b impaired or
recovery run, each endpoint's `p95` one-second Ardents-attributable carrier
bitrate in each physical network direction must be no more than:

`min(25 Mbit/s, 80% of the declared usable link budget in that direction)`.

The usable link budget is a finite P3-D6 measurement input no higher than the
controlled endpoint access link available to Ardents in that run. The two
physical network directions are evaluated separately; a quiet direction cannot
compensate for a burst in the other. The cap is qualification evidence rather
than permission to saturate a production link or override a lower local policy.

All visible transport and network framing, encrypted payload, acknowledgements,
retransmission, parallel and abandoned Route or Carrier Channel attempts,
control, keepalive, padding, cover traffic, and required security, liveness, and
background bytes attributable to Ardents or a helper count. Unrelated operating
system and external Application traffic does not. Intermediate-Node forwarding
remains outside this endpoint numerator and is deferred with role-specific work
to R-004 and P3-D3b4.

Every accepted progress, recovery, CPU, RSS, queue, target-authentication,
Route-privacy, isolation, integrity, and fail-closed gate remains enabled. A
candidate cannot pass by delaying or dropping recovery, suppressing security or
liveness traffic, reclassifying failed attempts, moving bytes outside the
measured process or network boundary, using a direct fallback, or averaging one
episode against another. These are top-down unverified product ceilings rather
than current implementation measurements.

### P3-D5a — Protect established publisher work from incomplete-attempt flood

**Product Owner decision, accepted 2026-08-07:** a required Windows 11 or Ubuntu
LTS `x86-64` publisher endpoint must preserve useful established Service
Connections during a continuous 10-minute anonymous pre-establishment flood on
a controlled symmetric `100 Mbit/s` access link.

Before the attack window, the publisher has the complete accepted normal set:

- `256` exact-target-authenticated incoming Service Connections remain open;
- `64` of them continuously receive equal shares of the fixed incompressible
  `40 Mbit/s` offered Application workload in the measured direction;
- the other `192` remain authenticated, open, and inactive but usable.

The test is repeated separately for User-to-Service and Service-to-User
Application Data. The controlled User and published Applications do not apply
backpressure, cancel, close, or impose shorter timeouts.

For every one-second interval of the 10-minute attack window, the publisher
receives `1,000` new syntactically valid connection-establishment attempts aimed
at the published Service. Each reaches the supported pre-establishment or
admission path but is deliberately left incomplete and never becomes a Service
Connection. Attacker-attributable inbound bytes at the publisher operating-
system network boundary are capped at `20 Mbit/s` in every one-second interval,
so the workload tests endpoint admission isolation rather than unavoidable
physical link saturation.

The attempts provide no global User account, stable network-generated User
identity, or publisher-visible ordinary source location. The implementation
cannot make the test pass by grouping or blocking attempts by IP address,
account, or another stable attacker identity. P3-D6 fixes reproducible arrival
timing and route diversity without weakening the R-001 privacy boundary.

During the complete attack window, all of these honest-work floors apply
simultaneously:

- all `256` established connections remain authenticated, open, and usable as
  the same Application-facing streams;
- the `64` active connections deliver at least `32 Mbit/s` aggregate
  Application Data, averaged across the full attack window;
- every active connection averages at least `400 kbit/s` of delivered
  Application Data and has no continuous zero-delivery interval longer than
  `5 s`;
- every inactive connection passes unpredictable bounded Application canaries,
  including a final canary, without a new connect operation or hidden stream
  replacement. P3-D6 fixes their schedule and deadline.

Success in one connection, aggregate, or Application Data direction cannot hide
a starved, closed, or silently replaced connection. Only bytes delivered to the
receiving Application count. The retained `32 Mbit/s` is 80% of the accepted
normal publisher aggregate workload; the per-connection and no-progress floors
remain separate anti-starvation guards.

The complete publisher Ardents process tree simultaneously keeps
`p95 resident memory <= 1 GiB` and mean CPU `<= 100%` of one logical core. The
published Application's own work remains excluded, but publication, admission,
cryptography, carrier, security, liveness, canary, timeout, and cleanup work and
every helper process count. Existing connection, endpoint, Local Grant,
Isolation Context, queue, and operating-system resource caps remain in force.

Incomplete-attempt state, timers, tasks, handles, and buffers are finite, expire
or are rejected under bounded policy, and do not accumulate across the 600,000
injected attempts. An incomplete attempt is never presented to the published
Application as an accepted Service Connection. Overload outcomes remain honest
and bounded; their exact Connection Results and anonymous admission mechanism
remain R-007 and R-010 work.

The endpoint cannot pass by pre-empting established connections, reducing the
declared offered workload, discarding misses, disabling target authentication,
Route privacy, isolation, integrity, liveness, or fail-closed checks, sharing
forbidden cross-context state, moving work outside the measured process tree,
or using a direct fallback. These are top-down unverified qualification targets.

P3-D5a deliberately protects already established work only. It does not claim
that a new honest anonymous User can connect during the flood, and it does not
select proof of work, tokens, payments, global identities, IP reputation, or any
other admission mechanism. Honest new admission is the separate P3-D5b decision
and must preserve the no-global-User-identity product boundary.

### P3-D5b — Honest anonymous admission during incomplete-attempt flood

**Product Owner decision, accepted 2026-08-07:** while the complete P3-D5a
incomplete-attempt flood continues, a publisher with finite available connection
capacity must still admit new honest anonymous Users with bounded success,
latency, and client cost.

This separate 10-minute run uses the same controlled symmetric `100 Mbit/s`
publisher access link and, in every one-second interval, the same `1,000`
incomplete attacker attempts and `20 Mbit/s` inbound attacker-traffic cap. Before
the window begins, the publisher has `240` established incoming Service
Connections:

- `64` receive equal shares of the fixed incompressible `40 Mbit/s` offered
  Application workload in the measured direction;
- `176` remain authenticated, open, inactive, and usable;
- `16` ordinary Service Connection slots remain available inside the accepted
  `256`-connection publisher profile.

The test is repeated separately for User-to-Service and Service-to-User
Application Data. The established set retains the applicable P3-D5a floors:
all `240` streams remain authenticated, open, and usable; the active set
delivers at least `32 Mbit/s` aggregate, every active stream averages at least
`400 kbit/s` with no zero-delivery interval over `5 s`, and inactive streams
pass unpredictable canaries. The complete publisher process tree retains
`p95 RSS <= 1 GiB` and mean CPU `<= 100%` of one logical core.

At an undisclosed point in every one-second interval, one network-ready honest
client submits an ordinary unprivileged connect request to the same exact
Service destination. Each of the `600` attempts begins without an existing
Service Connection, privileged allowlist state, a global User account, a stable
network-generated User identity, or publisher-visible ordinary source location.
P3-D6 fixes whether the exact destination form is Service Name or Service Target
and balances both forms when their pre-admission work differs.

After an honest attempt receives an exact-target-authenticated usable Service
Connection, the harness generates unpredictable bounded Application Data,
requires it to traverse the new stream, and then closes the connection cleanly
to release the slot. No more than the available `16` honest admissions may
remain concurrently established or in the accepted final admission stage; the
offered schedule and the `15 s` deadline make the workload finite without
requiring a hidden capacity increase.

The run must satisfy both:

- at least `95%` of all `600` honest attempts reach an authenticated usable
  Service Connection and pass the canary;
- connection latency has `p95 <= 8 s`, measured from the Application's connect
  request to the exact-target-authenticated usable connection.

Every honest attempt remains in both the success-rate and latency evidence.
Failures are misses rather than omitted samples. At `15 s` without a connection
result, the attempt terminates with an explicit supported result and remains a
miss; it cannot hang or silently switch destination, Route Profile, Isolation
Context, or ordinary network. P3-D6 fixes percentile calculation, canary size
and deadline, and the allowed relation between the `95%` success floor and the
`p95` latency gate so one cannot hide the other.

If Ardents requires an anonymous network admission check under this workload,
the additional cost attributable to that check on each honest client is capped
at all of:

- `1` logical-core CPU-second;
- `64 MiB` additional peak resident memory;
- `1 MiB` combined sent and received carrier traffic.

The check cannot require money, a token balance, account registration, IP or
source-location reputation, a stable network-generated User identifier, or a
credential that links otherwise separate Services or Isolation Contexts. This
does not forbid an Application from later requesting its own identity or
payment after the Service Connection exists; that is outside carrier admission.
The concrete anonymous cost or challenge remains an R-010 decision and must be
measured on every required client platform.

All target-authentication, Route-privacy, integrity, isolation, queue,
backpressure, resource, liveness, and fail-closed checks remain active. A
candidate cannot pass by marking attacker attempts as honest, using advance
knowledge of honest arrival times, prioritizing IPs or accounts, pre-empting an
established stream, reducing the established offered workload, reusing
cross-context state, omitting failed attempts, or moving work outside the
measured boundaries.

This guarantee applies only while the declared `16` connection slots remain
available. At the configured full capacity, Ardents may return an explicit
bounded capacity result; it cannot hang, claim success, or evict another
connection. P3-D5b does not cover an attacker that also completes the selected
anonymous admission step and then hoards or abuses established connections;
that is P3-D5c.

### P3-D5c — Established anonymous connection hoarding and backpressure

**Product Owner decision, accepted 2026-08-07:** when hostile clients complete
the same anonymous admission available to honest clients and obtain valid
Service Connections, Ardents must bound their effect on other established work
and publisher resources. It does not claim to identify them or to guarantee a
new honest admission while all finite connection capacity remains occupied.

The 10-minute publisher workload uses the same controlled symmetric
`100 Mbit/s` access link and is repeated separately for User-to-Service and
Service-to-User Application Data. Before measurement begins, all `256`
connections in the accepted publisher profile are exact-target-authenticated,
open, and usable against the same published Service:

- `128` are controlled honest connections: `64` receive equal shares of the
  fixed incompressible `40 Mbit/s` offered Application workload in the measured
  direction and `64` remain inactive except for unpredictable canaries;
- `128` are controlled hostile connections that have completed any selected
  anonymous admission step and hold ordinary unprivileged Service Connections.

The harness knows which connections are hostile, but the candidate receives no
such label. The mapping is randomized and cannot be inferred through a supplied
IP address, source location, account, stable network-generated User identity,
privileged grant, or persistent cross-Service or cross-Isolation-Context state.

The two direction-specific workloads exercise both backpressure boundaries:

- in the User-to-Service run, every hostile Application continuously attempts
  to write incompressible Application Data while the controlled published
  Application deliberately does not consume those hostile streams;
- in the Service-to-User run, the controlled published Application continuously
  attempts to write incompressible Application Data to every hostile stream
  while those hostile Applications do not consume it.

P3-D6 fixes the exact write schedule, payload sizes, onset, and sampling, but it
must drive every hostile stream to its applicable queue or backpressure
boundary and hold that pressure for the measurement window. External
Application work is not attributed to Ardents, but all Ardents processes,
buffers, tasks, timers, handles, cryptographic state, control traffic, and
cleanup are.

Throughout both runs:

- all `128` honest connections remain authenticated, open, and usable;
- the `64` active honest connections deliver at least `32 Mbit/s` aggregate,
  each averages at least `400 kbit/s`, and none has a zero-delivery interval
  longer than `5 s`;
- all `64` inactive honest connections pass unpredictable canaries without an
  Application-visible reconnect;
- the complete publisher process tree keeps `p95 RSS <= 1 GiB` and mean CPU
  `<= 100%` of one logical core;
- the existing `256 KiB` per-connection and `64 MiB` publisher aggregate
  logical Application Data queue caps remain hard and separate in each
  direction; a full leaf or ancestor stops accepting producer bytes and
  propagates backpressure without blocking unrelated honest streams.

No hostile stream may create unbounded memory or disk buffering, task, timer,
handle, queue, retry, or cleanup state; borrow another connection's resource
scope; or force silent loss, reordering, eviction, reconnect, direct fallback,
or a weaker security path. Protocol violations still fail closed. A
protocol-conformant stream may be backpressured or terminated only by the same
documented finite resource or liveness policy that applies without the harness
label; Application content and User moderation remain outside carrier policy.

If all `256` slots stay occupied throughout a new connection attempt, Ardents
may return an explicit supported capacity-unavailable result, but must do so by
`15 s`. It cannot hang, queue the attempt without a finite bound, claim success,
or evict another connection to manufacture capacity. If capacity exists, the
separate P3-D5b honest-admission gate applies.

This is also an explicit product limitation. Without a scarce identity,
trusted account, payment, source reputation, or materially stronger anonymous
cost, Ardents cannot know whether `128` valid connections represent `128`
people or one Sybil attacker. V1 therefore promises bounded resource use,
backpressure, established-work isolation, and an honest capacity result—not
per-person fairness, guaranteed creation of a free slot, or a bound on how long
a protocol-conformant anonymous peer can retain one. R-010 may compare anonymous
costs within the accepted accessibility and unlinkability limits, but cannot
erase this non-claim by assumption.

### P3-D6a — Qualification matrix and pass/fail semantics

**Product Owner decision, accepted 2026-08-07:** V1 Route Qualification is one
conjunctive verdict over every mandatory performance and security cell. A pass
on one platform, endpoint side, Application Data direction, or scenario cannot
compensate for a failure in another.

Before running a candidate, the qualification plan identifies every applicable
cell by at least:

- the exact implementation candidate, build, configuration, and Route Profile;
- the required platform and endpoint side being measured;
- the declared scenario and its normal, cold, warm, impaired, recovery, or
  hostile mode where applicable;
- the Application Data direction and any other predeclared input variant whose
  behavior or cost may differ.

P3-D6b1 fixes the controlled topology and platform pairings, P3-D6b2a fixes
minimum release sample counts, P3-D6b2b2a fixes the normal network envelope,
P3-D6b2b2b fixes the Application payloads, P3-D6b2b2c1 fixes paired direct
baselines, P3-D6b2b2c2a fixes state preparation, and P3-D6b2b2c2b fixes the
remaining reference inputs. P3-D6a fixes how those cells produce a release
decision: results are
never pooled or averaged across mandatory platforms, endpoint sides,
directions, or scenarios. Each cell must satisfy all of its applicable latency,
success, progress, goodput, fairness, resource, carrier, queue, cleanup, and
liveness requirements simultaneously.

Security, privacy, isolation, authentication, and integrity requirements are
hard guardrails rather than percentiles. One valid execution that reveals
protected information in a forbidden view, accepts a forbidden substitution or
data violation, reuses forbidden cross-context state, silently downgrades, or
bypasses the declared Route hard-fails that candidate. Performance elsewhere
cannot offset it. An excluded Broad Traffic Observer or out-of-scope collusion
case remains an honest non-claim rather than either a hidden pass or failure.

Every eligible attempt, failure, timeout, explicit terminal result, crash,
premature connection loss, and measured event remains in the evidence and in
the applicable metric calculation. Warm-up, exclusion, censoring, and missing
sample rules must be declared before execution. A poor candidate result cannot
be renamed a test-infrastructure problem or omitted after inspection.

A run may be invalidated and repeated only when retained evidence confirms that
the harness or declared reference environment failed to produce the specified
input or trustworthy measurement independently of the candidate outcome. The
original artifacts, invalidation reason, affected cells, and replacement run
remain linked. Candidate timeout, crash, resource exhaustion, protocol failure,
or inability to tolerate the declared workload is a result, not a harness
invalidity. Scheduled repetitions are part of the plan and are not reruns.

If any mandatory cell misses an applicable target, or any hard guardrail fails,
the tested build and configuration do not earn V1 Route Qualification. They may
remain available as an explicitly unqualified research build, but neither the
release nor project communication may present them as a qualified V1 anonymous
network. Qualification applies only to the recorded candidate and conditions;
P3-D6c defines which later change requires partial or complete requalification.

### P3-D6b1 — Controlled cross-platform reference topology

**Product Owner decision, accepted 2026-08-07:** the V1 endpoint qualification
matrix contains all four supported User/client-to-Developer/publisher platform
pairings:

- Windows 11 to Windows 11;
- Windows 11 to Ubuntu LTS;
- Ubuntu LTS to Windows 11;
- Ubuntu LTS to Ubuntu LTS.

The arrow identifies endpoint roles and operating-system families, not the
Application Data direction. Every applicable workload is measured separately
for User-to-Service and Service-to-User data inside each pairing. Exact supported
point releases and endpoint hardware images are frozen by P3-D6b2b1 for the
tested candidate; one pairing cannot stand in for another.

The User endpoint, Publisher endpoint, and every ordinary carrier role execute
on separate physical machines or isolated virtual machines with separately
capped and recorded CPU, memory, storage, clock, and network resources. A
qualifying run cannot use loopback, shared-memory data transfer, in-process
Nodes, or an unrecorded same-host fast path. Multiple isolated VMs may share
physical hardware only when the declared caps and network path remain
enforceable and visible in the evidence.

Every measured infrastructure Node instance uses the accepted Ubuntu LTS
`x86-64` reference VPS class of `2 vCPU`, `2 GiB RAM`, and a symmetric
`100 Mbit/s` access link.
This does not require all roles to share one host or assert that all roles have
the same useful capacity. R-004 and P3-D3b4 determine the candidate's Route
shape, hop count, role set, placement, and role-specific useful-work floors.

A controlled network layer mediates every inter-machine link and applies the
declared bandwidth, latency, loss, jitter, interruption, and failure schedule.
It records configured and observed conditions. The candidate uses its actual
production Route shape, protocol, transports, cryptography, target
authentication, Isolation Context handling, resource controls, and fail-closed
behavior. A test-only direct path, trusted relay, disabled protection, reduced
hop count, shared secret unavailable in production, or hidden topology shortcut
invalidates qualification.

Every metric that uses an ordinary-network direct baseline receives a paired
baseline on the same endpoint machines and OS images, with the same Application
payload, direction, `60-second` timed-transfer duration, and declared end-to-end
impairment profile. One direct run brackets the complete associated Ardents batch
before it and another after it. The direct path exists only for measurement and
can never become a production fallback. P3-D6b2b2c1 defines how the two
measurements are combined and how much baseline drift invalidates the comparison.

Public-Internet, community-node, or uncontrolled-VPS runs may provide useful
supplementary field evidence, but they cannot replace, repair, or average into a
failed controlled qualification cell. P3-D6b1 selects no routing family,
transport, library, language, or final Route shape.

### P3-D6b2a — Qualification sample floors and percentile rules

**Product Owner decision, accepted 2026-08-07:** the following are minimum
sample floors for full V1 Route Qualification. A smaller development or CI
smoke suite may find regressions quickly but cannot contribute partial credit to
or substitute for these release samples.

For each applicable P3-D6b1 cell, every normal short-event scenario—routine
restart, clean first start, cold and warm connection, and cold and warm Named
Unlisted Site tracer—uses `100` eligible attempts for each declared mode. At
least `99` of the `100` must reach that scenario's successful end event. When an
accepted scenario already has a different explicit success floor or sample
count, that specific rule prevails; P3-D5b therefore retains its `600` attempts
and `>= 95%` honest-admission floor.

Every recovery profile collects at least `20` eligible independent episodes per
applicable cell and Application Data direction. Where recovery is expected
because the accepted conditions retain a qualifying alternate Route, at least
`19` of `20` episodes must continue the same Service Connection and every
unsuccessful episode must still terminate explicitly by its accepted deadline.
A stricter workload remains stricter: the three-event sequential-churn run, for
example, still requires every declared event and final canary in each run to
succeed. If the `20`-episode floor requires more than five scheduled runs, those
additional runs are part of qualification rather than optional reruns.

Every sustained 10-minute normal, impaired, recovery, admission-flood, or
established-hostile workload runs independently at least `5` times per
applicable cell and direction. All five must complete without a hard failure and
must satisfy every non-percentile invariant. For a `p05` 60-second goodput
metric, each run contributes exactly ten non-overlapping windows, producing
`50` ordered values across the cell. A failed, prematurely lost, or undelivered
window contributes `0` goodput.

CPU, memory, carrier-rate, and other time-series percentiles are calculated
inside each 10-minute run; every one of the five runs must satisfy its applicable
resource and carrier gates. Samples from a low-resource run cannot offset a
failed run. P3-D6b2b2c2b fixes the time-series sampling interval and platform
attribution.

The accepted 24-hour idle carrier guardrail requires one complete retained run
on each required Windows 11 and Ubuntu LTS client OS image for the candidate. It
remains a secondary guardrail rather than a standalone release blocker, but a
missing, invalid, or over-budget result must be reported and cannot be described
as a pass.

All percentiles use ascending nearest-rank order statistics without
interpolation: for percentile `p` and `N` values, select rank `ceil(p * N)`.
Thus a `p95` over `20` episodes uses rank `19`, and a `p05` over `50` goodput
windows uses rank `3`. A failed or timed-out latency observation is ordered as
positive infinity; failed goodput is `0`. Every offered eligible sample remains
in the success-rate denominator and percentile evidence.

Additional samples are allowed only when their count and inclusion rule are
declared before observing candidate results; all eligible samples then count.
Replacement remains limited to a P3-D6a confirmed harness invalidation. A
shorter smoke matrix, a successful subset, or repeated execution until a pass
never earns Route Qualification.

### P3-D6b2b1 — Official V1 endpoint OS and hardware baseline

**Product Owner decision, accepted 2026-08-07:** the generic Windows/Linux V1
platform boundary is refined to two official `x86-64` reference OS families:

- a supported, fully patched Windows 11 release available when the candidate is
  frozen;
- a supported, fully patched Ubuntu LTS point release available when the
  candidate is frozen.

Ubuntu LTS is the sole Linux qualification baseline for V1 endpoints and
infrastructure Nodes. Other Linux distributions, package bases, and CPU
architectures may happen to work, but they receive no V1 compatibility,
performance, or release claim and add no mandatory matrix cell. Supporting one
later requires an explicit product decision and its own qualification evidence;
it is not an implicit roadmap commitment.

The exact Windows edition and build, Ubuntu image and kernel, update set,
architecture, package inventory, and immutable image identifier are frozen and
recorded for each release candidate. Qualification cannot switch image after
seeing a result. Routine security updates create a new tested image rather than
permission to cite evidence from the old one; P3-D6c defines the required
requalification scope.

Both User/client and Developer/publisher reference endpoints use the same base
machine class:

- `4` dedicated virtual CPU threads;
- `8 GiB` RAM;
- SSD-backed storage;
- no required GPU or accelerator.

Each endpoint remains a separate physical machine or isolated VM under
P3-D6b1. CPU and RAM cannot be overcommitted during qualification. The host CPU
model, microcode, hypervisor and version, storage type, power mode, and any
resource caps are fixed across the applicable batch and retained in evidence.
Built-in OS security, firewall, exploit mitigations, and normal cryptographic
protections remain enabled; a narrowly required network rule may be declared,
but disabling platform protection to pass invalidates the result.

This base class is a qualification floor, not a runtime maximum. A weaker device
may expose explicitly reduced local capacity but cannot claim the V1 reference
result. A stronger device may select only a separately qualified automatic
scale-up profile under P3-D3c2c3c3; extra hardware grants no network role, trust,
authority, priority, isolation exception, or security downgrade.

The P3-D6b1 matrix labels are therefore Windows 11 to Windows 11, Windows 11 to
Ubuntu LTS, Ubuntu LTS to Windows 11, and Ubuntu LTS to Ubuntu LTS, each in both
Application Data directions. The accepted infrastructure class becomes Ubuntu
LTS `x86-64` with `2 vCPU`, `2 GiB RAM`, and a symmetric `100 Mbit/s` link.

### P3-D6b2b2a — Normal reference-network envelope

**Product Owner decision, accepted 2026-08-07:** every normal,
non-adversarial qualification cell uses one controlled, transport-independent
network envelope:

- the User/client access link provides `100 Mbit/s` toward the client and
  `20 Mbit/s` from the client;
- the Developer/publisher access link is symmetric `100 Mbit/s`;
- every ordinary infrastructure Node link remains symmetric `100 Mbit/s` under
  the accepted reference VPS class;
- the network-only base round-trip time between the User and Publisher
  boundaries is `80 ms`;
- carrier-packet loss is `0.1%` independently in each direction;
- additional per-direction delay variation has `p95 <= 10 ms`;
- the harness injects no complete interruption or packet reordering in this
  profile.

The access rates are usable carrier-link caps, not Application goodput promises.
All Ardents data, framing, handshakes, acknowledgements, retransmissions,
padding, control, security, liveness, and background traffic consume those caps.
The same operating-system image uses the client cap when acting as User and the
publisher cap when acting as Publisher.

Impairment is applied at controlled network boundaries below the candidate's
Carrier Transports. It therefore selects no protocol and applies equally to TCP,
UDP, QUIC, HTTP, WebSocket, and any later transport that can use the declared
boundary. Transport behavior caused by the candidate remains part of the result
and cannot be removed as harness noise.

The paired direct baseline and Ardents batch use the same endpoint caps and
end-to-end impairment profile. The generator, seeds, exact delay and loss
distribution, and distribution of the `80 ms` total across Route segments are
declared before observing candidate results and retained as evidence;
P3-D6b2b2c2b fixes their reproducibility and matching rules. Candidate
processing time is not included in the injected network-only RTT and remains
visible in measured latency.

This profile is a qualification reference, not a minimum connectivity
requirement or a promise that production paths match independent loss. A better
link cannot replace the mandatory cell. Complete interruption, injected
reordering, the accepted `300 ms`/`5%` impaired-live profile, churn, blocking,
and hostile traffic remain separate workloads and cannot be pooled with this
normal result.

### P3-D6b2b2b — Controlled Application payload suite

**Product Owner decision, accepted 2026-08-07:** V1 qualification uses three
transport-independent Application payload classes. Payload sizes are counted at
the Application Interface; Carrier framing and control bytes do not inflate
them.

**Connection canary:** immediately after a connection is reported as
exact-target-authenticated and usable, the User sends a fresh unpredictable
`32-byte` challenge and the controlled Service returns the same `32` bytes. The
challenge is unique to the attempt and is not disclosed to the candidate before
the send operation. Exact request and response bytes and their timestamps are
retained. A missing, changed, duplicated, or out-of-order response makes that
attempt a failure under the existing success and percentile rules. It does not
silently extend the connection-establishment timer past its defined usable-stream
milestone.

**Named Unlisted Site tracer:** the controlled Application receives one
HTTP/1.1 request of exactly `512 bytes`, including headers and a fresh `32-byte`
request nonce, with no request body. It returns fixed status and headers followed
by exactly `64 KiB` of seeded incompressible response body bound to that nonce.
The first valid HTTP response byte remains the latency observation, but the
complete response syntax, length, nonce binding, byte sequence, and digest must
validate before the attempt can pass. An invalid or incomplete body turns the
attempt into a failed latency observation rather than preserving an early
first-byte success.

**Sustained and concurrent transfer:** every measured direction carries a
pre-generated seeded incompressible byte stream. Each run and active Service
Connection has a distinct declared stream and does not loop a shorter payload
inside its timed window. The receiver validates every byte against its expected
offset and retains the final digest. Only correctly ordered verified Application
bytes count as goodput; any corruption, loss, duplication, or unexpected byte
fails the run. The corresponding direct and Ardents measurements use the same
payload artifact, direction, and duration.

Payload artifacts are generated outside timed intervals by the harness, and
their Application-side generation or validation work is outside the Ardents
process-tree resource metrics. Any Ardents helper process remains counted. HTTP
content encoding, Application caching, carrier payload compression,
payload-aware deduplication, and benchmark-specific short paths are disabled or
forbidden. No external resource is fetched by the tracer.

HTTP is only the protocol of the first controlled site workload. It adds no HTTP
method, header, URL, status, caching, or content semantics to Ardents. The V1
Application Interface remains the accepted live bidirectional reliable ordered
byte stream, and other Applications remain free to define their own protocols.

### P3-D6b2b2c1 — Paired direct-baseline combination and drift

**Product Owner decision, accepted 2026-08-07:** every goodput cell whose target
explicitly references a paired direct baseline is bracketed by one `60-second`
direct transfer immediately before and one immediately after its complete
Ardents batch. Both use the same endpoint machines, Application payload artifact,
Application Data direction, link caps, impairment profile, and timed-transfer
boundary as that batch. Connection setup is outside the direct transfer window,
as it is for the corresponding established-stream goodput measurement.

Let `B_before` and `B_after` be the two verified direct Application-goodput
results. The pair is stable only when both are finite and greater than zero and:

`max(B_before, B_after) / min(B_before, B_after) <= 1.10`

For a stable pair, the baseline used by the candidate gate is the arithmetic
mean:

`B = (B_before + B_after) / 2`

The accepted normal goodput target therefore uses
`min(10 Mbit/s, 50% of B)`. The impaired-live target independently uses
`min(2 Mbit/s, 25% of B_impaired)` from its own impaired direct pair. Results
from different directions, platform cells, network profiles, or batches are
never pooled or substituted.

If either direct run ends early, fails payload integrity, uses the wrong inputs,
or exceeds the `10%` drift bound, the entire bracketed batch is invalid and must
not produce a qualification result. The direct evidence, every candidate result,
and the invalidation reason remain retained. Invalidation cannot convert a
failed candidate run into a pass, delete it from reporting, or justify selective
reruns; a replacement requires the existing P3-D6a finding of a harness or
reference-environment failure independent of candidate behavior and reruns the
complete batch. Candidate-caused contamination or drift is a candidate failure,
not an environment invalidation.

No direct result normalizes latency, success rate, CPU, memory, carrier overhead,
security, privacy, isolation, or integrity. A direct path remains a measurement
control only and can never be selected as an Ardents production fallback.

### P3-D6b2b2c2a — Qualification state preparation and reset

**Product Owner decision, accepted 2026-08-08:** every eligible startup,
connection, and Named Unlisted Site tracer attempt begins from one declared and
verified state class. State preparation is repeated for every scheduled attempt;
an earlier sample cannot silently prepare a later one.

- **clean first start:** only the installed candidate, frozen operating-system
  image, immutable packaged configuration, trust roots, and declared bootstrap
  manifest are present. No state generated by an earlier Ardents execution is
  available, including generated endpoint/network keys, authenticated network
  snapshots, learned peers or Bridges, naming or reachability entries, Routes,
  Carrier Channels, sessions, tickets, or caches. Required local state and key
  creation and authenticated bootstrap acquisition remain inside the P3-D2b
  startup clock.
- **routine restart:** the same installed candidate and configuration may use
  valid previously authenticated persistent Ardents state. The previous process
  is fully stopped, with no live local socket, Service Connection, or Carrier
  Channel. Expiry, integrity, and rollback checks and establishment of a usable
  entry path remain inside the P3-D2b startup clock.
- **cold connection or site open:** the endpoint is already running and
  network-ready, and general authenticated network state and its entry path may
  remain. It has no state prepared for the exact Service Name, Service Target,
  Isolation Context, and Route Profile tuple: no target-specific naming,
  reachability, Rendezvous, Route, channel, session, ticket, prior connection,
  or Application/HTTP response cache may be reused. The timed request therefore
  includes every required target-specific lookup and Route operation.
- **warm connection or site open:** the same exact Service Name, authenticated
  Service Target, Isolation Context, and Route Profile may retain current
  authenticated naming and reachability data and reusable Route state. No
  Service Connection is open when the sample starts. A site attempt sends its
  fresh nonce-bearing request and receives new Application Data; no Application
  or HTTP response cache may satisfy it. State from another name,
  target, Isolation Context, or Route Profile cannot provide the warm state.

Killing the prior process or connection, clearing or preparing fixture state,
and verifying the declared precondition happen before the applicable timer.
The deliberate exception is work already assigned to a metric: clean-start
state creation and bootstrap and routine-restart validation remain inside their
startup clocks. A warm fixture may be created by a prior successful setup
connection only after that Service Connection is closed and all state not
explicitly allowed above is removed.

For every attempt, the harness retains the declared state class, the allowed
fixture manifest, preparation action, and precondition-verification result. A
candidate-independent harness failure to establish or verify the state may
invalidate the attempt only under P3-D6a, with its evidence retained. Candidate
behavior that preserves, reconstructs, or reuses forbidden state is a candidate
failure and, for cross-context reuse or a security shortcut, a hard guardrail
failure rather than a reset invalidation.

## Remaining decisions

1. **P3-D3b4 — Role-specific Node capacity:** after R-004 defines candidate
   units of work, set entry, relay, discovery, Rendezvous, and Bridge capacity
   floors on the accepted reference class.
2. **P3-D6b2b2c2b — Trace, sampling, and attribution:** define the exact
   impairment generator and seed discipline, time-series sampling, and
   cross-platform attribution.
3. **P3-D6c — Evidence and regression:** define retained artifacts,
   reproducibility, comparability, regression thresholds, invalidation review,
   and partial or complete requalification rules.

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

- **Product Owner decision:** Windows 11 and Ubuntu LTS `x86-64`
  desktop/laptop endpoints used by Users and Developers, plus a modest Ubuntu
  LTS server/VPS infrastructure Node, are the required V1 platform classes.
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
- **Product Owner decision:** a required Windows 11 or Ubuntu LTS `x86-64`
  client reference endpoint supports at least `64` concurrently open outbound
  Service Connections, including at least `16` simultaneously active
  connections. This revises the initially recorded `128/32` proposal after
  separating client and publisher workloads.
- **Product Owner decision:** a required Windows 11 or Ubuntu LTS `x86-64`
  publisher reference endpoint supports at least `256` concurrently open
  incoming Service Connections, including at least `64` simultaneously active
  connections.
- **Product Owner decision:** every selected infrastructure role must be useful
  on an Ubuntu LTS `x86-64` `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s`
  reference VPS; this is a minimum comparison class rather than a capacity
  ceiling.
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
- **Product Owner decision:** with `64` continuously active incoming connections
  sharing `40 Mbit/s` of delivered Application Data for 10 minutes, the complete
  publisher process tree keeps `p95 resident memory <= 1 GiB` and mean CPU
  `<= 100%` of one logical core in separate runs for each direction.
- **Product Owner decision:** in both equal-load active benchmarks, every
  connection averages at least `500 kbit/s` of delivered Application Data and
  has no zero-delivery interval longer than `2 s`.
- **Product Owner decision:** each endpoint and direction in the active
  benchmarks keeps combined sent-plus-received Ardents carrier bytes at or below
  `1.5x` the Application Data delivered in that direction.
- **Product Owner decision:** those active benchmarks run with the full capacity
  present: `64` client connections with `16` active and `256` publisher
  connections with `64` active. Every non-active connection remains
  authenticated, open, and usable without an Application-visible reconnect.
- **Product Owner decision:** logical Application Data queued locally is capped
  in each direction at `256 KiB` per connection, `16 MiB` across a client, and
  `64 MiB` across a publisher. Full queues apply honest stream backpressure;
  they never authorize hidden loss, eviction, cross-scope borrowing, or
  unbounded memory or disk buffering.
- **Product Owner decision:** an automatic higher endpoint profile must scale
  open and active connections and aggregate useful load together, preserve all
  accepted gates, and retain at least `20%` of each declared CPU, memory, and
  usable-link parent budget. The first confirmed miss is its saturation point
  and blocks that and larger automatic profiles for the tested envelope.
- **Product Owner decision:** the logical Service Connection is independent of
  any one Carrier Channel. Under one eligible ordinary-Node or Carrier Channel
  failure it resumes ordered delivery as the same connection within
  `p95 <= 5 s`, or terminates explicitly by `15 s`; transport choice cannot
  weaken that V1 outcome.
- **Product Owner decision:** the same Service Connection must pass three
  sequential eligible failures in one 10-minute run. Each next failure occurs
  only after recovery, each failed resource remains unavailable, and three is a
  qualification workload rather than a runtime recovery quota.
- **Product Owner decision:** on the accepted 10-minute impaired-live profile
  with `300 ms` base RTT, `5%` loss in each direction, and `100 ms` `p95`
  additional jitter, the same Service Connection has no zero-delivery interval
  longer than `5 s` and its `p05` 60-second Application goodput is at least
  `min(2 Mbit/s, 25% of the paired impaired direct baseline)` in each separately
  measured direction.
- **Product Owner decision:** one pair of overlapping eligible failures, with
  the second affecting a distinct in-progress recovery resource within `1 s`,
  remains one recovery episode. The same Service Connection recovers within
  `p95 <= 8 s` or terminates explicitly by `15 s`, both measured from the first
  interruption without a timer reset.
- **Product Owner decision:** throughout every 10-minute impaired and recovery
  run, each complete endpoint process tree keeps `p95 RSS <= 512 MiB`, mean CPU
  `<= 50%` of one logical core, and `p95` one-second CPU `<= 100%` of one core.
  The `256 KiB` per-connection directional queue cap remains, and temporary
  recovery state cannot accumulate across completed or abandoned attempts.
- **Product Owner decision:** the impaired-live endpoint carrier ratio is at
  most `2.0`; each recovery episode adds at most `8 MiB` of combined endpoint
  carrier traffic over a paired no-failure run; and `p95` one-second carrier
  bitrate per endpoint network direction is at most
  `min(25 Mbit/s, 80% of its declared usable link budget)`.
- **Product Owner decision:** during 10 minutes of `1,000` validly framed but
  incomplete connection attempts per second and at most `20 Mbit/s` inbound
  attack traffic on a symmetric `100 Mbit/s` publisher link, all `256`
  established connections remain usable. The `64` active connections retain
  at least `32 Mbit/s` aggregate, `400 kbit/s` each, and no delivery gap over
  `5 s`, while publisher `p95 RSS <= 1 GiB` and mean CPU stays within one core.
- **Product Owner decision:** during the same flood, a publisher starting with
  `240` established connections and `16` available slots receives one honest
  anonymous attempt per second. At least `95%` of all `600` attempts
  authenticate the exact target and pass a canary, with connection
  `p95 <= 8 s` and an explicit result by `15 s`; the established-work and
  publisher-resource floors remain in force.
- **Product Owner decision:** any network-mandated anonymous admission check
  adds at most one logical-core CPU-second, `64 MiB` peak resident memory, and
  `1 MiB` combined carrier traffic per honest client. It requires no money,
  account, IP or source reputation, stable identifier, or cross-Service or
  cross-Isolation-Context linkability.
- **Product Owner decision:** with the `256` publisher slots divided into `128`
  honest and `128` valid admitted hostile connections, both unread hostile
  input and non-reading hostile receivers must reach bounded per-stream
  backpressure without harming the accepted `64`-stream honest workload,
  inactive canaries, queue ceilings, or `1 GiB`/one-core publisher limits.
- **Product Owner decision:** at full connection capacity, a new attempt receives
  an explicit capacity-unavailable result by `15 s`; V1 does not promise
  per-person fairness or honest admission against an indistinguishable Sybil
  attacker that has completed admission and retains valid connections.
- **Product Owner decision:** Route Qualification is the conjunction of every
  mandatory platform, endpoint-side, direction, and scenario cell. Cells cannot
  compensate for one another; all applicable metrics must pass together, and
  one valid security, privacy, isolation, authentication, or integrity
  violation hard-fails the candidate.
- **Product Owner decision:** failures and timeouts remain evidence. Only a
  confirmed harness or reference-environment failure may invalidate a run, with
  the original artifacts and reason retained; an unqualified build may remain
  research but cannot carry the V1 anonymous-network claim.
- **Product Owner decision:** qualification covers Windows 11-to-Windows 11,
  Windows 11-to-Ubuntu LTS, Ubuntu LTS-to-Windows 11, and Ubuntu LTS-to-Ubuntu
  LTS endpoint-role pairings, each in both Application Data directions.
  Endpoints and ordinary Nodes run on separate machines or isolated VMs through
  a controlled network; loopback, shared-memory, and hidden test fast paths are
  forbidden.
- **Product Owner decision:** direct baselines bracket each applicable Ardents
  batch before and after on the same endpoints, payload, direction, `60-second`
  timed-transfer duration, and end-to-end impairment profile. Uncontrolled
  Internet results are supplementary only.
- **Product Owner decision:** normal startup, connection, and tracer cells use
  `100` attempts with at least `99%` success unless a scenario already fixes a
  different floor. Recovery uses at least `20` episodes with at least `95%`
  success unless a stricter workload requires every event.
- **Product Owner decision:** each 10-minute workload runs independently five
  times. `p05` goodput uses `50` non-overlapping 60-second windows; resource
  percentiles pass inside every run. Each client OS retains one 24-hour idle
  carrier run as a secondary guardrail.
- **Product Owner decision:** percentiles use nearest rank without interpolation.
  Failed latency is positive infinity, failed goodput is zero, all eligible
  samples count, and a smaller smoke suite never earns Route Qualification.
- **Product Owner decision:** official V1 endpoint qualification covers only
  fully patched frozen Windows 11 and Ubuntu LTS `x86-64` images on the same
  `4 vCPU`, `8 GiB RAM`, SSD-backed, non-overcommitted base class with built-in
  OS security enabled. Ubuntu LTS is the sole V1 Linux baseline.
- **Product Owner decision:** other Linux distributions and architectures are
  outside the V1 compatibility and release claim. Stronger hardware requires a
  separately qualified scale-up profile; weaker hardware cannot claim the
  reference result.
- **Product Owner decision:** the normal network uses a `100/20 Mbit/s`
  User/client access link, symmetric `100 Mbit/s` Publisher and infrastructure
  links, `80 ms` base end-to-end RTT, independent `0.1%` loss per direction, and
  `p95 <= 10 ms` additional per-direction jitter, with no injected complete
  interruption or reordering.
- **Product Owner decision:** normal impairment is applied below Carrier
  Transports and consumes link capacity with all attributable overhead. It
  chooses no transport; paired direct and Ardents measurements use the same
  declared profile.
- **Product Owner decision:** a fresh `32-byte` request/response canary validates
  each reported usable connection; a failed canary makes the attempt fail.
- **Product Owner decision:** the Named Unlisted Site tracer uses an exact
  `512-byte` nonce-bearing HTTP request and a `64 KiB` seeded incompressible
  response body. First-byte latency passes only when the whole response later
  validates.
- **Product Owner decision:** goodput and concurrency use pre-generated distinct
  seeded incompressible streams verified for exact order and integrity. Caching,
  compression, deduplication, external resources, and benchmark shortcuts cannot
  improve a result.
- **Product Owner decision:** HTTP belongs only to the controlled tracer
  Application and adds no protocol semantics to the generic byte-stream
  Application Interface.
- **Product Owner decision:** each direct-baseline goodput pair uses one verified
  `60-second` transfer before and after the Ardents batch. When
  `max(B_before, B_after) / min(B_before, B_after) <= 1.10`, its arithmetic mean
  is the baseline used by the applicable normal or impaired goodput formula.
- **Product Owner decision:** a zero, incomplete, corrupt, mismatched, or
  over-drift direct result invalidates the complete batch. All evidence remains;
  only a confirmed candidate-independent harness or environment failure permits
  a complete rerun. Candidate-caused drift is a candidate failure.
- **Product Owner decision:** a clean first start has no state generated by a
  prior Ardents execution; a routine restart may retain only valid authenticated
  persistent state and begins with no live process or connection.
- **Product Owner decision:** a cold connection or site attempt begins
  network-ready but without state for its exact name, target, Isolation Context,
  and Route Profile. A warm attempt may retain current authenticated naming,
  reachability, and reusable Route state for that same tuple, but no Service
  Connection or Application/HTTP response cache.
- **Product Owner decision:** every attempt retains its state manifest and
  precondition verification. A candidate-independent reset failure may
  invalidate it under P3-D6a; candidate retention, reconstruction, or
  cross-context reuse of forbidden state fails the candidate.
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
guardrail, apply the accepted active client and publisher resource ceilings,
enforce the accepted equal-load fair-progress floor, and apply the accepted
`1.5x` endpoint carrier ratio under the accepted combined open-and-active load.
Apply the accepted queue ceilings, backpressure semantics, and scale-up
saturation gate. Apply the accepted single-failure Service Connection recovery
gate, three-event ordinary-churn workload, and impaired-live useful-progress
profile, plus the overlapping-failure recovery gate and accepted degraded-path
and recovery endpoint resource and carrier limits. Apply both accepted
pre-establishment-flood gates for established work and honest anonymous
admission, and the accepted established-hostile-work isolation and full-capacity
non-claim. Apply the accepted conjunctive qualification and hard-guardrail
semantics and the accepted four-pair controlled topology with bracketing direct
baselines. Apply the accepted release sample floors, nearest-rank rules, and
Windows 11/Ubuntu LTS endpoint hardware baseline and the accepted normal-network
envelope, controlled payload suite, paired direct-baseline rule, and state-reset
contract, then define reproducible impairment traces, sampling, attribution,
evidence, and regression rules.
Role-specific infrastructure capacity remains deferred until R-004 supplies
candidate units of work.

Confidence: high for the platform boundary and desired connection experience;
the accepted latency, goodput, client-concurrency, and publisher-concurrency
targets, infrastructure reference class, idle client resource ceiling, and
active client and publisher resource ceilings and fairness floor remain
unverified. The active carrier ratio, combined-load workload, queue ceilings,
scale-up saturation gate, and single-failure recovery target also remain
unverified; the three-event churn workload, impaired-live profile,
overlapping-failure target, and endpoint recovery-resource ceilings are also
unverified. The impaired and recovery carrier limits are also unverified, and
the established-work, honest-admission, and established-hostile workloads are
unverified. The honest-client admission cost and all accepted sample floors are
also unverified on the required platforms. The idle carrier budget is
unverified and deliberately secondary. The normal-network envelope is also an
unverified top-down qualification input. The controlled payload suite is
unverified as a portable harness contract. The direct-baseline drift and
combination rule is also unverified against real environment variability. The
state-reset contract is also unverified as a portable cross-platform harness
boundary. The remaining numeric targets remain undecided. The `20`-episode
recovery floor makes `p95` observable only at coarse nearest-rank resolution;
exact order statistics and success counts must remain visible, and later
variability evidence may justify a larger predeclared sample.
The strongest counterargument
is that the mandatory four-pair, two-direction matrix may be expensive to
execute for a one-to-one project. That cost is accepted for release
qualification and does not expand the smaller development smoke suite.
Separately,
transport-independent continuation may require an Ardents layer above otherwise
suitable carriers, adding state, attack surface, linkability risk, and resource
cost. That is why the complete stack must earn the gate rather than assuming it
from a protocol name. Supporting both Windows 11 and Ubuntu LTS also increases
packaging and systems-integration work for a one-to-one project, but removing
either would contradict the accepted client product.

## Disposition

- State: `active`.
- P3-D1, refined by P3-D6b2b1: Windows 11 and Ubuntu LTS `x86-64`
  desktop/laptop endpoints used by Users and Developers, plus a modest Ubuntu
  LTS server/VPS infrastructure Node, are mandatory V1 benchmark and release
  classes.
- macOS and mobile remain later targets rather than V1 promises.
- P3-D2a accepted: exact-name connection establishment from a running, joined
  endpoint on the normal reference network is `p95 <= 3 s` in the cold state
  and `p95 <= 1 s` in the warm state refined by P3-D6b2b2c2a. The timer ends
  only at an exact-target-authenticated, usable stream.
- Startup, join, Application processing, degraded paths, and hostile conditions
  are excluded from these two clocks and require their own explicit budgets.
- P3-D2b accepted: an installed process reaches authenticated network readiness
  within `p95 <= 5 s` on routine restart with valid state and `p95 <= 15 s` on a
  clean first start, using the exact P3-D6b2b2c2a state classes. A local socket
  or UI without current network state and a usable entry path is not ready.
- Software installation, operating-system startup, subsequent Service
  connection work, and blocked or hostile entry are outside these startup
  clocks.
- P3-D2c accepted: the controlled Named Unlisted Site produces its first valid
  HTTP response byte within `p95 <= 4 s` cold and `p95 <= 2 s` warm from a
  running, network-ready endpoint under the P3-D6b2b2c2a state classes.
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
- P3-D3b1 revised and accepted: each required Windows 11 and Ubuntu LTS
  `x86-64` client reference endpoint supports at least `64` concurrently open
  outbound Service Connections, with at least `16` simultaneously carrying the
  declared active-transfer workload.
- The initial `128/32` client floor was superseded because it was not justified
  by the V1 client journey after publisher capacity was separated.
- P3-D3b2 accepted: each required Windows 11 and Ubuntu LTS `x86-64` publisher
  reference endpoint supports at least `256` concurrently open incoming Service
  Connections, with at least `64` simultaneously carrying the declared
  active-transfer workload.
- Both endpoint floors are minimum capacities rather than maxima. Exhaustion is
  bounded and explicit; publisher application policy may deliberately admit
  fewer connections without redefining the network benchmark.
- P3-D3b3 accepted: every selected V1 infrastructure role must demonstrate
  useful bounded operation on an Ubuntu LTS `x86-64` `2 vCPU`, `2 GiB RAM`,
  symmetric `100 Mbit/s` reference VPS.
- The reference VPS is not a capacity or trust ceiling. Stronger hardware may
  contribute more bounded work but receives no automatic additional role,
  authority, or route-selection privilege.
- P3-D3c1 accepted: supported endpoints derive conservative finite hierarchical
  budgets from available resources; stronger machines may exceed the client and
  publisher floors through compatible qualified profiles, and Endpoint Owners
  may cap lower. A finite higher experimental cap remains unqualified.
- Operating below a floor is allowed only as explicitly reduced local capacity,
  not as a qualified V1 performance result. More hardware grants no automatic
  Node role, trust, authority, route priority, cross-context access, or security
  shortcut, and exact hardware limits are not required network metadata.
- P3-D3c2a accepted: during the 10-minute window beginning at network readiness,
  an otherwise idle required Windows 11 or Ubuntu LTS `x86-64` client keeps
  whole-process-tree `p95 resident memory <= 256 MiB` and mean CPU `<= 1%` of one
  logical core.
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
- P3-D3c2c2 accepted: for separate 10-minute transfer runs in each direction,
  the complete required publisher process tree keeps
  `p95 resident memory <= 1 GiB` and mean CPU `<= 100%` of one logical core
  while `64` active connections share `40 Mbit/s` of delivered Application Data.
- Publisher application work is excluded, but publication and carrier work
  remain counted. Lost or stalled connections, reduced useful load, process
  splitting, unbounded queues, or a security or isolation shortcut cannot make
  the publisher resource test pass.
- P3-D3c2c3a accepted: in both equal-load active benchmarks, every connection
  averages at least `500 kbit/s` of delivered Application Data over 10 minutes
  and has no zero-delivery interval longer than `2 s`.
- The floor applies separately to each connection and direction under equal
  grants, offered load, and controlled paths. Aggregate success, carrier bytes,
  fast peers, unbounded buffering, or a security or isolation shortcut cannot
  hide starvation; unequal production policy and degraded or hostile paths have
  separate contracts.
- P3-D3c2c3b accepted: for each endpoint and direction in the active benchmarks,
  combined Ardents-attributable bytes sent and received are at most `1.5x` the
  Application Data delivered in that direction.
- The ratio counts carrier control, framing, retransmission, padding, and
  background work at the endpoint boundary, but not intermediate-Node traffic.
  Required security and liveness work cannot be suppressed to pass; degraded,
  hostile, and stronger-privacy profiles retain separate budgets.
- P3-D3c2c3c1 accepted: the active client test runs with `64` total open and
  `16` active connections, and the active publisher test with `256` total open
  and `64` active connections, while retaining all accepted active metrics.
- Every non-active connection remains authenticated, open, and usable as the
  same Application-facing stream. Hidden eviction, discarded network state, a
  failed unpredictable canary, or an Application-visible reconnect fails the
  combined test; maintenance and canary costs remain counted.
- P3-D3c2c3c2 accepted: each required client and publisher reference profile
  caps locally queued logical Application Data at `256 KiB` per connection and
  direction, with aggregate caps of `16 MiB` per client direction and `64 MiB`
  per publisher direction.
- Attributable local IPC buffers are inside the logical accounting boundary;
  process-resident physical copies remain subject to whole-process-tree memory
  ceilings and non-process buffers require separate bounded OS-resource
  evidence. At a full queue the stream applies backpressure and accepts no
  additional bytes; silent loss, eviction, reordering, cross-scope borrowing,
  unbounded disk or memory buffering, and false write success fail
  qualification.
- P3-D3c2c3c3 accepted: a higher client or publisher profile declares a scale
  factor `S > 1` and proves proportionally scaled open connections, active
  connections, and aggregate delivered Application Data in the complete
  combined-load test.
- The per-connection queue cap remains `256 KiB`; only the finite endpoint
  aggregate scales. Mean CPU, `p95` RSS, and `p95` one-second directional carrier
  bitrate each remain at or below `80%` of their declared finite parent budget.
- The first profile that misses any applicable metric or the `20%` reserve is
  the saturation point and is not eligible for automatic selection. The owner
  may cap lower; a finite higher experimental override is visibly unqualified
  and cannot weaken any security or overload invariant.
- P3-D4a accepted: a Service Connection is the logical Application stream above
  replaceable Carrier Channels. If one ordinary Node or Carrier Channel fails
  while both endpoints, the same Service Instance, and a qualifying alternate
  Route remain, the same connection resumes ordered delivery within
  `p95 <= 5 s`, measured separately in each direction.
- The recovery clock runs from the last byte delivered before failure to an
  unpredictable post-injection canary delivered through the recovered path and
  includes detection, Route and Carrier Channel replacement, authentication,
  queued predecessors, and continuation. At `15 s` without recovery the Service
  Connection returns an explicit terminal result; the miss remains in evidence.
- Recovery preserves target, Isolation Context, Route Profile, ordering, and the
  Application-facing connection without loss or duplicate presentation. It may
  retransmit carrier bytes but never reissues an Application operation, creates
  a hidden replacement connection, falls back directly, or weakens security.
- P3-D4b1 accepted: one continuously loaded Service Connection must survive
  three sequential eligible ordinary-Node or Carrier Channel failures in one
  10-minute run, separately in each Application Data direction.
- Each next event strikes the then-current Route only after the previous
  post-injection canary arrived. Failed Nodes remain unavailable and failed
  channel instances cannot be reused. Every event retains its own P3-D4a clock,
  percentile membership, and `15 s` terminal deadline.
- All three recovery canaries and a final canary must arrive through the same
  still-usable Service Connection. Three is not a runtime quota; Ardents cannot
  close a healthy connection merely because its third recovery completed.
- P3-D4b2a accepted: an established Service Connection remains useful for a
  10-minute run with `300 ms` base end-to-end RTT, independent `5%` packet loss
  in each direction, `100 ms` `p95` additional per-direction jitter, and no
  complete path interruption.
- In separate Application Data directions, `p05` 60-second goodput is at least
  `min(2 Mbit/s, 25% of the paired impaired direct baseline)`, and no
  zero-delivery interval exceeds `5 s`. The same connection remains open,
  ordered, non-duplicating, exact-target-authenticated, and secure without an
  Application-visible reconnect.
- A complete interruption remains a P3-D4a recovery event rather than an
  impaired-live success. P3-D6 fixes distributions, windows, repetitions, and
  retained evidence.
- P3-D4b2b accepted: one overlapping pair occurs in a direction-specific
  10-minute run. The first failure stops the current Route; within `1 s` and
  before recovery completes, the second stops a distinct resource used by the
  in-progress replacement attempt. A further qualifying Route remains.
- Both failures form one recovery episode measured from the last byte delivered
  before the first. The same connection delivers the final recovery canary
  within `p95 <= 8 s` or terminates explicitly by `15 s`; the second failure and
  internal retries cannot restart either clock.
- Target, active Service Instance, Isolation Context, Route Profile, ordering,
  and stream identity remain unchanged without loss, duplicate presentation,
  Application-visible reconnect, operation replay, direct fallback, or
  security downgrade.
- P3-D4b2c1 accepted: throughout every direction-specific 10-minute impaired
  or recovery run, each complete client or publisher endpoint process tree has
  `p95 RSS <= 512 MiB`, mean CPU `<= 50%` of one logical core, and `p95`
  one-second CPU `<= 100%` of one core.
- The `256 KiB` logical queue cap per Service Connection and direction and all
  ancestor caps remain in force. Recovery adds no queue allowance, and
  completed or abandoned recovery state cannot accumulate with failure count.
- Useful progress, recovery, security, and resources are simultaneous gates;
  no hidden reconnect, premature termination, process splitting, weakened
  security, false delivery, or omitted miss can make the resource result pass.
- P3-D4b2c2 accepted: the 10-minute impaired-live endpoint carrier ratio is
  `<= 2.0` separately at each endpoint and Application Data direction.
- Each single or sequential recovery episode, and the complete overlapping pair
  as one episode, adds no more than `8 MiB` of combined sent and received
  endpoint carrier traffic over its paired no-failure run. One episode cannot
  offset another.
- Across every impaired or recovery run, each endpoint network direction has
  `p95` one-second carrier bitrate
  `<= min(25 Mbit/s, 80% of its declared usable link budget)`.
- All retransmission, parallel and abandoned attempts, control, padding,
  security, liveness, and attributable background bytes count; no missed
  recovery, suppressed protection, direct fallback, or omitted bytes can make a
  carrier result pass.
- P3-D5a accepted: on a symmetric `100 Mbit/s` link, a publisher with the full
  `256/64` established workload receives `1,000` syntactically valid but
  incomplete attempts per second for 10 minutes, capped at `20 Mbit/s` inbound
  attacker traffic in every one-second interval.
- All `256` established connections remain authenticated, open, and usable.
  The active set retains at least `32 Mbit/s` aggregate, every active connection
  retains `400 kbit/s` mean and no zero-delivery gap over `5 s`, and the inactive
  set passes unpredictable canaries without reconnecting.
- The complete publisher process tree retains `p95 RSS <= 1 GiB` and mean CPU
  `<= 100%` of one logical core. Admission state is finite and cannot accumulate
  across the 600,000 attempts; all security, isolation, queue, and honest
  evidence gates remain enabled.
- P3-D5a assumes no IP address, global User account, or stable attacker identity
  and protects only already established work. It neither selects an admission
  mechanism nor claims that new honest attempts succeed during the flood.
- P3-D5b accepted: under the same flood, a publisher starts with `240`
  established connections and `16` available slots while one honest anonymous
  client attempts to connect in every second for 10 minutes.
- At least `95%` of all `600` honest attempts reach an exact-target-authenticated
  usable Service Connection and pass a canary; connection latency has
  `p95 <= 8 s`, and every attempt returns an explicit result by `15 s`.
- The P3-D5a established-work and publisher-resource floors remain in force.
  Failed and timed-out honest attempts remain misses, and success cannot depend
  on IP, an account, advance arrival knowledge, or cross-context state.
- Any network-mandated anonymous admission check adds at most one logical-core
  CPU-second, `64 MiB` peak resident memory, and `1 MiB` combined carrier
  traffic per honest client. It requires no money, token balance, account,
  source reputation, stable identifier, or link between otherwise separate
  Services or Isolation Contexts.
- The P3-D5b guarantee applies while the declared `16` slots are available. At
  full capacity, an explicit bounded capacity result is allowed; eviction,
  false success, or a hanging attempt is not.
- P3-D5c accepted: the publisher starts a 10-minute run at its `256`-connection
  profile with `128` honest and `128` hostile but valid admitted connections to
  the same Service. Of the honest set, `64` receive the `40 Mbit/s` workload and
  `64` remain inactive canaries.
- Separate direction-specific runs drive unread hostile input and non-reading
  hostile receivers to their queue or backpressure boundaries. The harness
  knows the labels; Ardents does not receive or infer them from IP, account,
  stable identity, privileged state, or cross-context linkage.
- All honest connections remain usable; the active set retains the P3-D5a
  `32 Mbit/s` aggregate, `400 kbit/s` per-stream, and `5 s` progress floors;
  inactive canaries pass. Publisher `p95 RSS <= 1 GiB`, mean CPU remains within
  one core, and the existing `256 KiB` leaf and `64 MiB` aggregate directional
  queue caps and per-stream backpressure remain active.
- When all `256` slots stay occupied, a new attempt may receive an explicit
  capacity-unavailable result but must do so by `15 s`; it cannot hang, enter an
  unbounded queue, claim success, or evict another connection.
- V1 explicitly does not promise per-person fairness, creation of a free slot,
  or successful new admission against an indistinguishable Sybil attacker that
  has completed admission and retains protocol-conformant connections.
- P3-D6a accepted: every mandatory platform, endpoint-side, direction, and
  scenario cell is evaluated separately and all applicable metrics must pass
  simultaneously. Results cannot be pooled or averaged across cells.
- A valid security, privacy, isolation, authentication, or integrity violation
  hard-fails the candidate regardless of performance. Failures, timeouts,
  crashes, and terminal results remain in the applicable evidence.
- A run may be invalidated only for a confirmed harness or reference-environment
  failure independent of the candidate outcome. The original artifacts,
  reason, affected cells, and linked replacement run are retained.
- One failed mandatory cell blocks V1 Route Qualification for that build and
  configuration. It may remain an explicitly unqualified research build but
  cannot be presented as a qualified V1 anonymous network.
- P3-D6b1, refined by P3-D6b2b1: the controlled endpoint matrix contains
  Windows 11-to-Windows 11, Windows 11-to-Ubuntu LTS, Ubuntu LTS-to-Windows 11,
  and Ubuntu LTS-to-Ubuntu LTS role pairings, with User-to-Service and
  Service-to-User data measured separately in each.
- User, Publisher, and every ordinary Node role run on separate physical or
  isolated virtual machines with recorded finite resources and controlled
  links. Loopback, shared memory, in-process Nodes, and hidden same-host or
  reduced-Route fast paths cannot qualify.
- Each infrastructure Node instance uses the Ubuntu LTS `x86-64` `2 vCPU`,
  `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS class. R-004 and P3-D3b4
  still determine candidate Route shape, roles, placement, and role-specific
  useful capacity.
- Applicable direct baselines run on the same endpoints and end-to-end
  impairment profile immediately before and after each Ardents batch. They are
  measurement-only and never a production fallback; uncontrolled Internet
  evidence is supplementary and cannot repair a failed controlled cell.
- P3-D6b2a accepted: every applicable normal startup, connection, and tracer
  cell uses `100` eligible attempts with at least `99` successes unless an
  accepted scenario fixes another count or success floor.
- Every recovery profile collects at least `20` eligible episodes per cell and
  direction with at least `19` successful continuations when recovery is
  expected. Stricter scenarios, including every event in sequential churn,
  remain stricter.
- Each sustained 10-minute workload has at least five independent runs. `p05`
  goodput uses their `50` non-overlapping 60-second windows; each resource and
  carrier percentile passes inside all five runs.
- Each required client OS image retains one complete 24-hour idle carrier run.
  It remains a reported secondary guardrail rather than a standalone blocker.
- Percentiles use ascending nearest rank without interpolation. Failed latency
  is positive infinity, failed goodput is zero, all eligible samples count, and
  smaller smoke suites cannot qualify.
- Public DNS, naming design, bootstrap, routing, libraries, language, exact host
  CPU model, reproducible impairment traces, and the remaining numeric budgets
  remain unselected.
- P3-D6b2b1 accepted: the Windows 11 and Ubuntu LTS images are fully patched and
  frozen per candidate on `x86-64`; Ubuntu LTS is the sole Linux qualification
  baseline. Other distributions and architectures receive no V1 claim or
  mandatory cell.
- User and Publisher reference endpoints use `4` dedicated vCPU threads,
  `8 GiB RAM`, SSD-backed storage, no required GPU, and no CPU or RAM overcommit.
  Host CPU, microcode, hypervisor, storage, power mode, caps, and exact OS images
  remain recorded; built-in OS security stays enabled.
- The infrastructure image is Ubuntu LTS `x86-64` on the accepted `2 vCPU`,
  `2 GiB RAM`, symmetric `100 Mbit/s` VPS class. Stronger endpoint capacity
  requires a qualified scale-up profile; weaker hardware cannot claim the base
  result.
- P3-D6b2b2a accepted: the normal reference envelope uses `100/20 Mbit/s` User,
  symmetric `100 Mbit/s` Publisher and infrastructure links, `80 ms` base RTT,
  independent `0.1%` loss in each direction, and `p95 <= 10 ms` additional
  per-direction jitter, with no injected complete interruption or reordering.
- The envelope is transport-independent and applied below Carrier Transports.
  Its caps include all attributable traffic, and the same declared profile
  brackets the Ardents batch through its paired direct measurements.
- P3-D6b2b2b accepted: connection success is validated by a fresh `32-byte`
  request/response canary. The site tracer uses an exact `512-byte` HTTP request
  with a fresh nonce and exactly `64 KiB` of seeded incompressible response body;
  an invalid or incomplete response makes its first-byte observation fail.
- Sustained and concurrent transfer uses distinct pre-generated seeded
  incompressible streams with exact order and digest validation. Caching,
  compression, deduplication, external resources, and benchmark short paths are
  forbidden. HTTP remains a tracer protocol rather than an Ardents semantic.
- P3-D6b2b2c1 accepted: applicable goodput batches have verified `60-second`
  direct transfers before and after. A pair is stable when both values are
  positive and `max/min <= 1.10`; its arithmetic mean supplies the applicable
  normal or impaired baseline.
- A zero, incomplete, corrupt, mismatched, or over-drift direct run invalidates
  the complete batch without erasing any evidence. Replacement requires a
  confirmed candidate-independent harness or environment failure; candidate-
  caused drift is a candidate failure. Direct results normalize only formulas
  that explicitly reference them and never provide a production fallback.
- P3-D6b2b2c2a accepted: every startup, connection, and site-tracer attempt
  begins from a declared verified clean, routine, cold, or warm state. Cold state
  has no target-tuple preparation; warm state permits current authenticated
  target data and reusable Route state but no open Service Connection or
  Application response cache.
- Candidate-independent state-preparation failure may invalidate an attempt only
  under P3-D6a. Candidate retention, reconstruction, or cross-context reuse of
  forbidden state fails the candidate and cannot be relabeled as harness noise.
- P3-D6b2b2c2b impairment traces, sampling, and cross-platform attribution is
  next, followed by P3-D6c evidence and regression rules. Role-specific Node
  capacity and cost remain deferred until R-004 candidate evidence under
  P3-D3b4.
- No ADR and no code.
