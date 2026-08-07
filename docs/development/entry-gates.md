# Development entry gates

The repository separates **research code**, **Reference Application code**, and
**production network code**. Passing time or accumulating code does not promote
one category into the next.

## Gate A — start an experiment

An experiment may be written when:

- it answers one named question from the network research queue;
- a falsifiable hypothesis and comparison criteria are written first;
- the network requirement, journey, and threat-model claim being tested are
  linked;
- inputs, environment, evidence, and cleanup behavior are defined;
- its directory is under `experiments/` and makes no compatibility promise.

An experiment may use any suitable language or maintained library. That choice
does not select the production stack.

## Gate B — start the Named Unlisted Site tracer

The Reference Application may gain a shared codebase only after review of the
minimum relevant contracts:

- accepted R-006 Service Target lifecycle;
- R-002 live Application Interface;
- accepted R-001 Interactive Route adversary, claim matrix, and Route
  Qualification gate;
- R-003 Service Name and recovery boundary;
- R-004 minimum Interactive Route evidence;
- R-007 connection and Service failure behavior;
- R-009 minimum authenticated bootstrap path;
- R-017 accepted Reference Application scope.

The tracer must use an ordinary local HTTP service over the generic Application
Interface. A browser adapter or simulated route may be part of the tracer, but
must be visibly marked. Replicated Site Bundles, offline delivery, and an
Ardents application runtime are not silently added to this gate.

## Gate C — select production language and foundations

A production language, runtime, routing foundation, storage component, or wire
format is selected only when:

- the same accepted network and tracer contract is evaluated fairly;
- security and maintenance evidence covers proposed dependencies;
- memory safety, concurrency, target-platform support, reproducible builds,
  fuzzing, interoperability, operability, and one-to-one project capacity are
  compared;
- migration and replacement boundaries are explicit;
- a research record recommends the choice;
- an ADR records meaningful lock-in and rejected alternatives.

## Gate D — claim a security property

A security or privacy property may be presented as implemented only when:

- it follows the claim format in the threat model;
- conformance and adversarial tests exercise the declared conditions;
- measurements or analysis are retained and reproducible;
- downgrade, route failure, key rotation, update, and recovery are covered;
- documentation exposes the honest limitation to Users and Developers.

For the Interactive Route, passing this gate is Route Qualification for one
recorded implementation candidate, build, configuration, workload, and threat
boundary. Its controlled topology must inspect both endpoint edges, every
ordinary Node role in turn, malicious endpoints, distinct Isolation Contexts,
and active substitution, modification, injection, replay, redirect, downgrade,
truncation, and forbidden reordering. A forbidden disclosure or accepted active
violation fails the candidate.

Broad Traffic Observer and sufficiently placed collusion correlation are
explicit non-claims, not hidden qualification successes or failures. Their
limits must remain visible. Before Route Qualification, neither the release nor
the project may present that implementation publicly as an anonymous network.

Route Qualification is one conjunctive verdict across every mandatory platform,
endpoint-side, direction, and scenario cell. Results are not averaged across
cells. Each cell must meet all applicable metrics together, and one valid
security, privacy, isolation, authentication, or integrity violation fails the
candidate regardless of performance elsewhere. Failures, timeouts, crashes, and
terminal results remain in the evidence. A run is replaced only for a confirmed
harness or reference-environment failure independent of candidate behavior;
the original artifacts and invalidation reason are retained.

The controlled endpoint matrix includes Windows 11-to-Windows 11, Windows
11-to-Ubuntu LTS, Ubuntu LTS-to-Windows 11, and Ubuntu LTS-to-Ubuntu LTS
User/client-to-publisher pairings, each in both Application Data directions.
Endpoints and every ordinary Node role run on separate physical machines or
isolated VMs with recorded finite resources and controlled links. Loopback,
shared memory, in-process Nodes, reduced test Routes, and hidden same-host fast
paths do not qualify. The candidate retains its production cryptography, target
authentication, isolation, resource controls, and fail-closed behavior.

## Gate E — call a network release usable

A release is usable only when complete journeys, not isolated primitives, pass
on supported platforms. For the first tracer this includes start/join, publish,
name, resolve, connect, exchange, close/fail, accepted rotation, alternate-path
recovery, and blocked-entry behavior.

For V1, those endpoint journeys must pass on frozen, fully patched Windows 11
and Ubuntu LTS `x86-64` desktop/laptop reference images. Both endpoint roles use
the `4 vCPU`, `8 GiB RAM`, SSD-backed, non-overcommitted base class with built-in
OS protection enabled. The infrastructure path uses an Ubuntu LTS `x86-64`
`2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS class. Other Linux
variants receive no V1 claim. Server-only success cannot substitute for a
working User or Developer endpoint; macOS and mobile do not block V1.

Exact image identifiers, OS builds, kernels, packages, host CPU, microcode,
hypervisor, storage, power mode, and caps are frozen and retained per candidate.
An image cannot be replaced after seeing its results; an updated image requires
the requalification scope defined by R-023 P3-D6c.

Normal qualification gives the User/client a `100 Mbit/s` inbound and
`20 Mbit/s` outbound access link and gives the Publisher and every ordinary Node
a symmetric `100 Mbit/s` link. The controlled network applies `80 ms`
network-only base User-to-Publisher RTT, independent `0.1%` carrier-packet loss
in each direction, and `p95 <= 10 ms` additional per-direction jitter. It injects
no complete interruption or packet reordering in the normal profile. Shaping is
below Carrier Transports, and all attributable protocol and background traffic
consumes the link caps.

Every controlled cell freezes a versioned immutable network manifest before
candidate execution. It identifies the controlled-link topology, roles and
directions, caps, delay, jitter and loss models, impairment placement, scenario
failures, generator algorithm and version, complete parameters, and ordered seed
assignment for every scheduled attempt, episode, or run. Neither the manifest
nor its seed schedule may be selected, rerolled, replaced, or extended after
results.

The harness reproduces generator inputs below Carrier Transports rather than a
fixed packet trace. Candidate packet count, timing, retransmission, congestion,
loss, delay, reordering, retry, and other packetization consequences remain
candidate results. Direct controls use the same end-to-end profile and seed
discipline without internal Ardents Route segments. The manifest, configured
inputs, and observed execution evidence remain retained. A confirmed
candidate-independent failure to apply or verify the manifest may invalidate
the affected evidence only under R-023 P3-D6a; candidate-induced effects cannot.

Every elapsed-time KPI uses start and end events from one host's monotonic clock;
wall clocks correlate logs only. Sustained windows retain contiguous raw
one-second values without smoothing or interpolation. CPU is the charged
user-plus-kernel CPU-time delta with `100%` equal to one logical core; RSS is the
sum of OS-reported resident or working-set bytes across charged processes without
subtracting shared pages; carrier bitrate uses exact controlled-interface byte
deltas separately by direction.

Latency, failure, recovery, security, and queue invariants retain native-
resolution events and exact queue high-water evidence rather than relying on
one-second snapshots. An isolated accounting boundary charges the complete
Ardents process tree and every helper; excluded Applications and the harness use
separate recorded boundaries. Controlled ingress and egress counters establish
traffic, while candidate self-reports remain diagnostic. Candidate-independent
missing attribution may invalidate evidence only under P3-D6a. Escape,
interference, hidden work, and candidate resource use fail the candidate.

Every reported usable connection immediately passes a fresh unpredictable
`32-byte` request and exact `32-byte` response canary. The site tracer sends an
exact `512-byte` nonce-bearing HTTP request and validates a complete `64 KiB`
seeded incompressible response body after observing its first byte. Sustained and
concurrent workloads use distinct pre-generated seeded incompressible streams;
only bytes verified in exact order count. A canary, body, order, or integrity
failure fails the attempt or run. Caching, compression, deduplication, external
resources, and benchmark-specific fast paths cannot make a candidate pass.

Every startup, connection, and site-tracer attempt declares and verifies its
state class before the applicable timer. Clean first start has only the installed
candidate, frozen immutable inputs, trust roots, and bootstrap manifest, with no
state generated by a prior execution. Routine restart may retain valid
authenticated persistent state but no live process or connection. Cold request
modes begin network-ready with no state for their exact Service Name, Service
Target, Isolation Context, and Route Profile. Warm modes
may retain current authenticated naming, reachability, and reusable Route state
for that same tuple, but no open Service Connection or Application/HTTP response
cache.

Preparation is repeated and its manifest and verification remain evidence for
every attempt. Clean-start creation and routine-restart validation stay inside
their startup clocks; other fixture preparation precedes the request timer. A
candidate-independent reset failure may invalidate an attempt only under R-023
P3-D6a. Candidate retention, reconstruction, hidden warm-up, response caching,
or cross-context reuse of forbidden state fails the candidate.

Every goodput target that references a direct baseline uses one verified
`60-second` direct transfer immediately before and after its complete Ardents
batch on the same endpoint machines, payload, direction, link caps, impairment
profile, and timed-transfer boundary. Both direct values must be positive and
`max/min <= 1.10`; their arithmetic mean supplies the applicable normal or
impaired baseline. Otherwise the complete batch is invalid. All evidence remains,
and replacement requires a confirmed candidate-independent harness or reference-
environment failure. Candidate-caused drift fails the candidate. A direct path
normalizes no other KPI and can never become a production fallback.

Public-Internet or uncontrolled community runs may supplement but cannot replace
or repair a failed controlled cell.

Full qualification uses `100` eligible attempts with at least `99%` success for
each normal startup, connection, and tracer cell unless a specific accepted
scenario sets another floor. Each recovery profile uses at least `20` eligible
episodes per cell and direction with at least `95%` successful continuation
where expected; stricter scenario rules still prevail. Each sustained 10-minute
workload runs independently five times. Its `p05` goodput uses `50`
non-overlapping 60-second windows, while resource and carrier percentiles pass
inside every run. Each required client OS image retains one complete 24-hour
idle carrier run as a secondary guardrail.

All percentiles use ascending nearest rank without interpolation. Failed latency
orders as positive infinity, failed goodput is zero, and every eligible sample
remains in the evidence. Additional samples must be predeclared and all count.
Shorter development or CI smoke suites never earn Route Qualification.

One failed mandatory cell blocks the usable V1 and Route Qualification claim for
that build and configuration. The artifact may remain an explicitly
unqualified research build, but passing cells cannot compensate for the failed
one and project communication cannot present it as a qualified anonymous
network.

An offline Service must be reported as unavailable. The release must not imply
that Application Data was retained, delivered, or semantically completed unless
a separate accepted Overlay contract provides and verifies that behavior.

## Repository promotion

When an experiment is promoted:

1. preserve its result and evidence in the research record;
2. design the production module from the accepted contract rather than copying
   the experiment directory wholesale;
3. delete or clearly archive obsolete experiment code;
4. add conformance, misuse, failure, and recovery tests with the first
   production slice;
5. update the functional map and decision state.
