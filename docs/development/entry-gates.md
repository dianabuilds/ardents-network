# Development entry gates

The repository separates **research code**, **Reference Application code**, and
**production network code**. Passing time or accumulating code does not promote
one category into the next.

[Product scope and delivery horizons](../product/scope.md) is authoritative.
Only the current Delivery Horizon may enter the backlog; later fixed contracts
are claim and promotion gates, not permission to build speculative subsystems.

## Gate A — start an experiment

An experiment may be written when:

- it answers one named question from the network research queue;
- a falsifiable hypothesis and comparison criteria are written first;
- the network requirement, journey, and threat-model claim being tested are
  linked;
- inputs, environment, evidence, and cleanup behavior are defined;
- its directory is under `experiments/` and makes no compatibility promise.

A disposable comparison experiment may use a suitable tool, but maintained Go
code remains in the root project under ADR-0009. An experiment cannot create a
second project stack by inertia.

## Gate B — start Carrier Lab

Carrier Lab may start only after one experiment record freezes:

- the Ubuntu LTS `x86-64` client, publisher, and controlled infrastructure
  fixtures;
- one active Service Instance, deterministic byte-stream workload, ephemeral
  project-owned test keys, and preconfigured authenticated Target/reachability;
- the current five-position split-circuit candidate, separate Introduction, and
  only the comparison controls needed to falsify it;
- per-role traffic/state observations for the R-001 knowledge boundary;
- coarse setup latency, one-stream goodput, CPU, RSS, and one injected path-
  failure measurement;
- explicit pass, redesign, and stop conditions plus evidence cleanup.

[R-013](../research/records/r-013-carrier-lab-technology-candidates.md)
now freezes this experiment contract. Gate B permits implementing only the
declared Carrier Lab behavior in the root project. It does not imply a passing
Route result or select routing, transport, storage, or wire foundations.

Carrier Lab stays the only maintained project behavior slice. It contains no Service Name, public
discovery/contribution, Bridge, production installer/updater, Windows build,
public SDK/proxy/browser, general Application sandbox, multiparty Control Plane,
or complete R-023 matrix. It makes no privacy, anonymity, decentralization, or
release claim. Passing Gate B only answers whether the Route candidate deserves
another experiment.

Official Ubuntu run
[`31404126248`](https://github.com/dianabuilds/ardents-network/actions/runs/31404126248)
answered that bounded question with `advance`. Gate B is complete for the
current native C-5/C2 candidate; repeating or enlarging Carrier Lab is not a
prerequisite for Gate C unless the candidate inputs change.

## Gate C — start the Named Unlisted Site Reference Application

Named Unlisted Site work starts only after Carrier Lab evidence retains a viable
Route candidate. Its first controlled slice is Ubuntu-to-Ubuntu and adds:

- the stable minimum Application Interface and one-Service Target/Instance
  Credential lifecycle from R-002/R-006;
- private reachability resolution and one pre-provisioned exact Service Name;
- a deterministic HTTP Service and single-response client with no ordinary
  network access by construction;
- exact target authentication, explicit offline/failure behavior, and ordinary
  one-Instance migration.

Permissionless Name claims, leases, delegation, catastrophe recovery, Bridges,
public Contributor admission, production updates, general browser support, and
cross-platform qualification do not enter this gate. A Reference Application
may begin as another experiment; it gains shared production code only after Gate
D. Replicated Site Bundles, offline delivery, and an Ardents application runtime
remain outside scope.

The advancing R-013 Carrier Lab result satisfies this gate's Route-candidate
precondition. The bounded Ubuntu-to-Ubuntu Named Unlisted Site tracer is now the
next permitted implementation slice. Gate D and Gate E remain unsatisfied for
protocol-bound production foundations and public security claims.

## Gate D — select production language and foundations

A routing foundation, storage component, wire format, or other protocol-bound
foundation is selected only when:

- the same accepted network and tracer contract is evaluated fairly;
- security and maintenance evidence covers proposed dependencies;
- memory safety, concurrency, target-platform support, reproducible builds,
  fuzzing, interoperability, operability, and one-to-one project capacity are
  compared;
- migration and replacement boundaries are explicit;
- a research record recommends the choice;
- an ADR records meaningful lock-in and rejected alternatives.

The language/runtime portion is satisfied for the maintained project by R-014,
the Product Owner's explicit promotion, and ADR-0009. That decision does not
satisfy any remaining foundation gate or authorize a later product horizon.

## Gate E — claim a security property

A security or privacy property may be presented as implemented only when:

- it follows the claim format in the threat model;
- conformance and adversarial tests exercise the declared conditions;
- measurements or analysis are retained and reproducible;
- every downgrade, failure, key/lifecycle, update, and recovery path that can
  affect the exact declared claim is covered; unrelated later-horizon systems
  are neither assumed nor smuggled into its scope;
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

The role-local one-Node claim likewise excludes a Node adversary that also
controls/observes an endpoint or active probe source. The evidence bundle still
runs at least `100` blinded balanced positive/negative confirmation episodes in
each endpoint direction under a precommitted probe schedule and classifier and
publishes its confusion matrix, accuracy, and false-positive rate. Expected
correlation has no anonymity pass threshold; a direct forbidden carrier field or
wording that includes the combined adversary fails the claim review.

Qualification attempts external connection/scan of every endpoint Application
listener plus ordinary DNS, external fetch, callback/SSRF,
WebSocket/WebRTC, QUIC, and direct socket egress from both endpoint Application
process trees. A claim-bearing profile fails on any ordinary listener, escape, or transparent
clearnet fallback; a generic adapter may pass only the narrower carrier claim
with its limitation visible. Direct-origin source identity/family conflicts,
finite source exhaustion, retained Entry-set collisions across restart, and Role
Domain assignment expiry/reassignment are adversarial cells: no contact may
overlap retained/prepared/live forbidden state, no duty may outlive assignment,
and emergency may close work but never create cross-domain eligibility.
The platform also runs a same-desktop-user hostile sibling that attempts local
interface attachment, bearer theft/replay, PID reuse, process/Endpoint restart,
and another Application's Service/Context/diagnostics/authority. Failure narrows
the supported model to broker-launched or OS-isolated Applications; it cannot be
waived as a loopback implementation detail.

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

## Gate F — call a Public Beta candidate locally usable

A Public Beta candidate is locally usable only when complete journeys, not
isolated primitives, pass on supported platforms. This later horizon includes
unprivileged install and repair, capability-specific start/join, Target Link
connect, publish, name
and private resolve where claimed, exchange, close/fail, bounded Instance
Key/Credential rotation, connection/Work Safety expiry, Local Grant revocation,
alternate-path recovery, blocked entry, grant-scoped diagnostics, finite resource
exhaustion, private/direct/offline update modes, update/drain, safe rollback, and
Authority Recovery Bundle locked restore/export. It also includes finite direct-
source sequencing and post-exclusion readiness, non-overlapping Role Domain
reassignment, and blocked ordinary ingress/egress at both controlled tracer
Applications, plus same-user hostile-sibling Application Principal isolation.
One generic `network ready` flag cannot substitute for separate Target Connect,
Private Name Resolution, Publish, and Contribute readiness results.

For Public Beta, those endpoint journeys must pass on frozen, fully patched Windows 11
and Ubuntu LTS `x86-64` desktop/laptop reference images. Both endpoint roles use
the `4 vCPU`, `8 GiB RAM`, SSD-backed, non-overcommitted base class with built-in
OS protection enabled. The infrastructure path uses an Ubuntu LTS `x86-64`
`2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS class. Other Linux
variants receive no Public Beta claim. Server-only success cannot substitute for a
working User or Developer endpoint; macOS and mobile do not block Public Beta.

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

A release claiming Route Qualification must publish one complete immutable
Qualification Evidence Bundle for the exact candidate and conditions. It binds
source and binary identity, build and dependency inputs, configuration,
platforms, resources, manifests, scheduled cells, and harness and calculator
versions to every raw success, failure, timeout, crash, security event, and
invalidation. A schema-versioned deterministic calculator must reproduce every
per-cell metric and the conjunctive verdict; a human report is not a substitute.

Pre-run manifests are fixed before results, and the completed bundle is content-
addressed and append-only. Corrections create linked successors, while original
and invalidated evidence remains. The complete bundle stays publicly available
while the qualification claim is used and contains only synthetic test activity,
never real User or Developer traffic, production secrets, or persistent private
authority. Selected successes, unexplained redaction, deletion, corruption,
unavailability, or an unreproducible verdict cannot support qualification.

Every change to a tested candidate creates a new qualification identity. A
partial rerun requires a change-impact scope fixed before results and evidence
that every omitted cell cannot execute, read, share state with, or depend on the
change. Unknown, indirect, shared, core, protocol, security, routing, naming,
isolation, admission, recovery, transport, relevant dependency or runtime,
qualification-contract, or measurement-semantic impact requires the complete
mandatory matrix. External documentation and tooling require no Route rerun only
when the tested candidate and all qualification inputs remain byte-identical.

Installer-only, fixture, verdict-calculator, collector, or harness changes may
repeat or recalculate only their proven affected scope under P3-D6c2. A runtime
launcher repeats the complete matrix for its OS, and a shared or semantic harness
change expands to the full matrix. Absolute product gates and every security
invariant remain blocking. Between otherwise identical comparable bundles, an
adverse movement of at least `10%` in any numeric KPI blocks automatic release
promotion until an explicit explained Product Owner decision; it does not erase
Route Qualification while every absolute gate still passes.

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
authenticated persistent state but no live process or connection. Cold
connection modes begin Target Connect Ready; cold name-based tracer modes also
begin Private Name Resolution Ready. Neither has state for its exact Service
Name, Service Target, Isolation Context, and Route Profile. Warm modes
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

One failed mandatory cell blocks the usable Public Beta and Route Qualification
claim for that build and configuration. The artifact may remain an explicitly
unqualified research build, but passing cells cannot compensate for the failed
one and project communication cannot present it as a qualified anonymous
network.

An offline Service must be reported as unavailable. The release must not imply
that Application Data was retained, delivered, or semantically completed unless
a separate accepted Overlay contract provides and verifies that behavior.

## Gate G — make a public network claim

A locally usable build is not yet a decentralized public network. Public Beta
requires all of the following at the same time:

- one exact build and configuration has passed the complete R-001/R-023 Route
  Qualification matrix without a security or performance exception;
- five real independent Control Plane custodians operate the accepted `3-of-5`
  baseline and `4-of-5` expiring emergency thresholds; project-only keys are
  visibly limited to a centralized test network;
- at least two full Candidate View auditors, independent from each other, the
  epoch signer threshold, and audited Candidate operator families, publish
  retained input-inclusion, canonical-view, global-summary, concentration, and
  control-independence evidence;
- every Role Domain and required subrole, including Destination Resolution, has
  an effective post-exclusion pool of at least three independent eligible
  operator families, no family supplies more than `40%`, and workload reserve
  remains after the profile's maximum union of own-family, Direct Source
  Exposure, exact resolver-family, drain/quarantine, and other mandatory local
  exclusions. Four domains give only a theoretical pre-exclusion beta floor of
  `12`; with maximum distinct exclusion union `x_d` in domain `d`, the actual
  route-family floor is at least `Σ_d(3 + x_d)` and may be higher for capacity.
  every mandatory pre-Route artifact class additionally has at least three
  effective authenticated source-only families with the `40%` cap. The same
  three may cover several classes and count once; external/CDN/file sources
  without authenticated family evidence do not count. Thus all-zero-exclusion
  theoretical infrastructure supply starts at `15`, and every `x_d` is fixed
  before qualification;
- supported packages are reproducible, retain source/dependency inputs and an
  SBOM, and have at least two matching build attestations from builders
  independent of each other and of the release-Targets threshold;
- every capability called usable has a platform startup/resource budget, and a
  protocol cannot become required until every Role Domain and required
  control/discovery role has qualified independent capacity plus drain reserve;
- hostile bootstrap, Direct Source collision/exhaustion, Candidate omission/
  materialization withholding, clock, rollback, update, fork, Role Domain
  reassignment, drain, connection/Work Safety expiry, Local Grant revocation,
  Authority Recovery Bundle, Application Principal hostile-sibling isolation,
  endpoint-Application network isolation, anonymous admission/
  cost-to-deny, and effective post-exclusion concentration drills pass and retain
  evidence;
- an external usability review and an independent security review cover the
  claims presented to Users and operators.

Human Service Names are called stable only after the selected mechanism passes
its convergence, front-running, Private Resolution, accessible-cost, and
non-administrative governance gates. Until then, Target Links remain the complete
authenticated destination path and names remain experimental.

A stable decentralization claim additionally requires at least five effective
post-exclusion independent operator families in every Role Domain and required
subrole, no family above `25%`, and workload reserve. `20` is only the theoretical
pre-exclusion four-domain floor; the actual route-family floor is at least
`Σ_d(5 + x_d)`. Each mandatory pre-Route artifact class also has five effective
authenticated source-only families under the `25%` cap; the same five may cover
several classes and count once. The all-zero-exclusion theoretical infrastructure
floor is therefore `25`, and capacity may require more. These thresholds do not
prove hidden independence; evidence of common control is counted as one family,
and uncertainty remains public.

If contributor capacity, custodians, attestations, or external review do not
exist, launch remains an explicitly centralized or unqualified test network.
The project does not compensate by shortening Routes, relaxing admission,
inventing operator independence, or adding a token without a new product
decision.

## Repository promotion

When an experiment is promoted:

1. preserve its result and evidence in the research record;
2. design the production module from the accepted contract rather than copying
   the experiment directory wholesale;
3. delete or clearly archive obsolete experiment code;
4. add conformance, misuse, failure, and recovery tests with the first
   production slice;
5. update the functional map and decision state.
