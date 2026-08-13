# Horizon 3 Stage 4 implementation brief

Status: **draft; Stage 3 local development evidence passed; no implementation
authority until R-032 is accepted by the Product Owner**

Authoritative inputs after that acceptance: accepted ADRs, R-001, R-002, R-004,
R-006, R-023, R-024, R-029 through R-031,
[R-032](../research/records/r-032-h3-same-connection-recovery.md), the H3
technical design, operating model, threat model, package map, dependency
register, and repository rules. Until acceptance, this document freezes a
reviewable implementation proposal and authorizes no Stage 4 code.

## Entry gate and completion levels

The Stage 3 local development evidence gate is closed. The Product Owner
reported its local DoD complete at commit
`6c8faf9daef2d0f03009d4b6708825d45be1c434`: `make check` and final
Standards/Spec reviews were clean; one retained local Docker campaign passed
`27/27` attempts over `10m47.8703023s`, and an independent verifier replay over
that same frozen 27-attempt bundle also returned `27/27`; image
`sha256:7c3453123a91232b7624ea2eddb04bd6e7f2383c9b4cbe1dd001b5c57f0fbeb2`
and retained-evidence digest
`9aea2d37de910dec39cce79187fde94b49d53a10f0a6bab3a5ca14e6955162ae`
match; Docker and private-fixture cleanup passed.

Stage 4 implementation may start only after the Product Owner additionally
accepts R-032, changes its state from `open` to `decided`, authorizes this brief,
and commits the synchronized decision documents. That promotion accepts the
bounded Stage 3 result as development readiness only, not official Stage 1/2/3
qualification.

Official Ubuntu Stage 1 `short`, current `churn-2h`, independent
`unattended-24h`, cross-platform R-023 qualification, and public independence
gates remain deferred prerequisites for the integrated H3 verdict or any
stronger external, privacy, security, or release claim. They do not become true
because this local tracer passes.

Stage 4 has two distinct completion levels:

- **recovery development complete:** S4.1 through S4.3 pass the bounded local
  recovery contract in this brief;
- **Stage 4 complete:** after the complete bounded R-013 role prototype exists,
  a later accepted R-023 P3-D3b4 decision freezes role-specific useful-work
  units and capacity floors; S4.4 then passes pressure, lifecycle, capacity, and
  scale-up gates on the accepted Ubuntu reference host and complete topology.

The first level may provide the evidence needed to design the second. It cannot
be reported as full Stage 4 completion.

## Outcome and public seams

One external client Application and one external publisher Application retain
the same scoped local byte streams and the same exact-Target-authenticated
Service Connection while an eligible Carrier Channel, Route leg, or Rendezvous
is replaced. Application bytes continue in order without loss or duplicate
presentation, or the connection terminates exactly once with an honest R-002
Connection Result by the non-resetting deadline.

Tests and callers observe only these seams:

1. the Application Interface still owns admission, read, write, close, cancel,
   backpressure, honest local byte counts, and one terminal Connection Result;
2. the Service Connection Module owns endpoint authentication, continuity,
   logical byte order, acknowledgement, bounded replay, cutover, Work Safety,
   recovery deadlines, and cleanup;
3. the Route Module owns authenticated candidate selection, a bounded fresh
   Route Attachment, its underlying Nodes/legs/Carrier Channels, and attachment
   cleanup;
4. the Resource Controller owns hierarchical reservation and externally visible
   `NORMAL → PROTECT → DRAIN → EXIT` behavior; and
5. independent qualification recomputes immutable connection binding, Route
   generations, faults, bytes, clocks, resource/traffic bounds, forbidden-path
   absence, and cleanup as `pass|fail|invalid`.

The Application interface gains no `recover`, `replace route`, `retry hop`,
`resume`, `ack operation`, topology, Route generation, or continuity-secret
surface. Recovery status may appear in privacy-safe diagnostics for the Owner or
verifier, never as routing work the Application must perform.

## Module ownership and attachment seam

### Service Connection Module

Deepen `internal/serviceconn`; do not create a second production recovery
package. One live connection control block owns:

- fixed Target, active Instance/Credential generation, Network, Isolation
  Context, Route Profile, Destination Binding, and protocol commitments, plus
  the current authenticated finite Work Safety state;
- the original endpoint-handshake commitment and one volatile connection-only
  continuity key derived from application-specific exported keying material;
- separate `uint64` send, connection-acknowledged, receive, and next-deliverable
  offsets for both directions;
- finite unacknowledged bytes, receive ranges, deduplication, and cutover state;
- one committed Route generation and bounded uncommitted proposals;
- one non-resetting recovery episode clock and terminal result; and
- every connection-owned buffer, timer, task, handle, key, observation, and
  erasure obligation.

The Module presents the same Application stream/result interface as Stage 3.
Tests exercise recovery through that interface and observable attachments; they
do not make the state machine, sequence bookkeeping, or cryptographic frames a
second public interface.

### Route Module

Deepen `internal/route` so it can return a bounded raw Route Attachment between
the two Endpoint processes. It continues to own Route selection, role-local
plans, carrier authentication, setup deadlines, process/channel lifetime, and
cleanup. It never owns or learns:

- the connection-only continuity key;
- Application logical offsets, acknowledgement ranges, or semantic completion;
- Service Authority or an Application operation;
- the exact Target-to-Application binding; or
- the R-002 terminal result.

The internal seam has one semantic operation: open a fresh attachment satisfying
immutable Network/Profile/role/exclusion constraints and a finite deadline. A
request may constrain the accepted recovery layer, such as retaining the same
live Rendezvous, but it carries no Application bytes or continuity key as
configuration. The result is an owned bidirectional byte channel plus bounded
selection/cleanup observations, not a new Service Connection and not proof of
Application success.

Keep `internal/serviceconn` independent of `internal/route`: define the smallest
concrete attachment-request/result values at the Service Connection boundary
and let the Endpoint composition code supply one narrow open-attachment
function backed by the existing route socket/process seam. The function carries
only Network/Profile/role/exclusion constraints and a deadline and returns one
owned `io.ReadWriteCloser` plus bounded observations. Do not add an exported
interface merely to name the seam; introduce one only if two maintained adapters
actually vary there. Qualification fakes do not justify expanding the
production interface. Record the exact callable and the composition root's
permitted imports in `package-map.md` in the same implementation change.

### Endpoint handshake placement

The Stage 4 attachment path places the end-to-end TLS 1.3 handshake at the
Service Connection seam. Route transports opaque handshake and protected-record
bytes but does not terminate that handshake or possess Service Instance private
material. The client validates the exact current Instance key from its bounded
Credential; the publisher proves possession of that key.

The original successful handshake derives pseudorandom connection-specific
keying material through Go's standard-library TLS exporter with an Ardents-
specific label and non-empty canonical context, then derives a domain-separated
laboratory continuity key. This is RFC 8446 Section 7.5 application-specific
exported keying material, not RFC 9266's public `tls-exporter` channel-binding
value, which must not be used as a secret key. Every later attachment performs
a fresh endpoint handshake and proves possession of the original continuity
state while binding the new handshake. No custom primitive is implemented.

The existing Stage 2 canary compatibility path may retain its test-only
publisher pin until its accepted suite is replaced. The new Stage 4 attachment
path must not give `internal/route` a Service Credential, Instance key,
continuity key, or Application result. Preserve all Stage 1–3 behavior and
tests while moving no higher-level ownership downward.

## Connection identity and continuity contract

The complete connection lifetime fixes one nonzero value for each of:

- exact Service Target;
- active Service Instance public key and Credential generation;
- Network identity and authenticated Candidate View generation;
- Isolation Context and Interactive Route Profile;
- supplied Target provenance/Destination Binding;
- connection protocol version;
- original endpoint-handshake binding commitment.

Work Safety remains finite but is not immutable. Fresh authenticated state may
extend the lease before expiry without exceeding its signed maximum. Stale,
uncertain, expired, or revoked state cannot extend it, and an earlier learned
`no-new-leg-or-recovery-after` remains authoritative.

Stage 4 never mixes routine Service Instance migration with connection recovery.
A changed Instance, Credential generation, Target, context, profile, Network,
or destination provenance requires a new Application-chosen Service Connection.

The continuity key is pseudorandom, connection-only, known solely to the two
Endpoint processes, volatile, and non-exportable. It is never:

- a public or Node-visible connection identifier;
- an Application bearer or Local Grant;
- written to plans, logs, captures, manifests, or retained evidence;
- shared across Services, connections, Applications, or Isolation Contexts; or
- restored after Endpoint process restart, crash, reboot, suspend that loses
  live state, or update.

Evidence may retain only domain-separated commitments computed before secret
erasure. A reusable secret or a verifier capable of attaching to the connection
invalidates the campaign.

Each attachment proposal uses fresh endpoint nonces, Route handles, carrier and
endpoint session keys, and a strictly increasing proposed Route generation. Its
endpoint-only proof binds:

1. the original handshake-binding commitment and connection protocol;
2. all fixed connection values above plus the current authenticated Work Safety
   lease, signed maximum, and no-new-recovery bound;
3. proposed Route generation and both fresh nonces;
4. the fresh endpoint-handshake/exporter commitment;
5. both directions' sent, acknowledged, received, and next-deliverable offsets;
   and
6. the still-current Work Safety and recovery deadline.

The bounded canonical laboratory proof may use standard-library HMAC-SHA-256,
`crypto/rand`, `crypto/subtle`, and RFC 8446 application-specific exported
keying material. It requires independent known-answer construction, mutation/
replay tests, connection, protocol, endpoint-role, and proof-direction domain
separation, constant-time comparison, and best-effort secret erasure. It is not
a selected production wire protocol.

An ordinary candidate that is unavailable, expired, or becomes ineligible may
consume its finite attempt allowance and be followed by another safe proposal
within the original deadline. By contrast, equal/lower committed generations,
reused nonces/handles, or wrong handshake binding, exporter, Target, Instance,
context, profile, Destination Binding, offsets, or safety bounds are detected
integrity/authentication/replay violations and terminate the affected Service
Connection fail-closed. They are never treated as ordinary route unavailability
or retried to improve availability. Integrity failure on the committed endpoint
stream, ambiguous fixed binding, or expired safety likewise terminates rather
than attempting a downgrade.

## Ordered byte, acknowledgement, and backpressure contract

Application bytes receive monotonically increasing per-direction logical
offsets when accepted into the finite local connection queue. Protected data
records carry one contiguous logical range and the committed Route generation.
Maximum data payload remains `16 KiB`; maximum control/proof frame remains
`4 KiB`.

The receiver authenticates a record before it affects offsets. It may retain a
bounded out-of-order range, presents bytes only when every predecessor is
present, and presents the first valid copy of a logical range exactly once.
Retransmission may map the same range to a newer Route generation. Conflicting
authenticated bytes for the same offset terminate the connection.

A connection acknowledgement means that bytes reached the remote Service
Connection Module's bounded delivery state. It does not prove that the remote
Application read, processed, persisted, or semantically accepted them. The
sender retains data until connection acknowledgement; Carrier/TCP acknowledgement
alone cannot free it.

The first recovery profile permits at most `8` disjoint received ranges per
direction. Range metadata, delayed duplicate metadata, and acknowledgements are
finite; exceeding the bound terminates safely rather than allocating more.

The accepted `256 KiB` logical cap per Service Connection and direction includes
all attributable:

- Application IPC pending bytes;
- sent but connection-unacknowledged bytes;
- receive reordering/deduplication bytes and their bounded metadata;
- cutover and retransmission bytes; and
- data retained by committed and uncommitted attachments.

Recovery grants no additional queue allowance. Physical copies are also charged
to complete endpoint process-tree RSS. At the logical or ancestor cap, local
writes backpressure and accept no more bytes. Silent loss, eviction, disk spill,
cross-connection/context borrowing, false write success, or queue reset during
recovery fails.

## Recovery state, layers, and attempt limits

One Service Connection follows:

```text
ESTABLISHED -> SUSPECT -> RECOVERING -> CUTOVER -> ESTABLISHED
       \            \          \             \-> TERMINAL
        \------------\----------\----------------> TERMINAL
```

The authoritative recovery evidence clock begins at the last Application byte
delivered before the first injected failure. Candidate detection may occur
later but cannot shift that timestamp. Another failure, proposal, retry, state
transition, or clock-source change never resets the episode clock or `15 s`
terminal deadline.

Recovery proceeds through the smallest eligible layer:

1. replace the failed Carrier Channel inside a still-valid leg using the same
   selected Node processes;
2. replace a failed leg while retaining the same live Rendezvous;
3. after Rendezvous loss, use a fresh sealed Introduction attempt and a fresh
   eligible Rendezvous; then
4. terminate explicitly when no safe eligible proposal remains or any applicable
   deadline expires.

Every successful connection-visible repair commits a higher Route generation,
including same-leg Carrier replacement. Cutover requires fresh endpoint proof,
offset reconciliation, and bidirectional reachability. Temporary old/new
transmission is allowed only during bounded authenticated cutover; only the
committed generation advances delivery. The old attachment is then closed and
erased within the cleanup deadline.

One recovery episode permits at most `3` attachment proposals and at most `2`
uncommitted proposals concurrently. Proposals may skip generation numbers, but
committed generations never decrease. Failed, superseded, timed-out, and losing
parallel proposals remain charged to traffic/resources and are cleaned without
accumulating across episodes.
The proposal allowance resets only after the connection has returned to
`ESTABLISHED` and a later distinct failure begins a new episode. It is not a
healthy-connection lifetime quota and never resets an existing episode clock.

New attachment work requires current Common Readiness Base, Candidate View,
Credential, Destination Binding, Time Confidence, and Work Safety at proposal
time. Learned supersession, Recovery Pending, Released, changed Target,
revoked/vulnerable build or protocol, invalid Time Confidence, expired
Credential, or safety deadline stops new recovery. An old still-readable stream
cannot grandfather recovery indefinitely.

A fresh authenticated state update may extend Work Safety before expiry but
never past its signed maximum. Local Grant revocation blocks new work
immediately and preserves only an explicitly authorized bounded drain; the
drain cannot exceed Work Safety and cannot start or prolong recovery.

## Vertical implementation slices

Each slice is independently reviewable, keeps all earlier suites green, and
ends with candidate, negative, evidence-integrity, cleanup, and architecture
tests. Do not begin the next slice with unresolved actionable findings.

### S4.1 — Continuity core and one Carrier failure

Deepen Service Connection to own endpoint TLS, continuity, logical offsets,
connection acknowledgements, bounded unacknowledged data, deduplication, and one
Route Attachment replacement. Deepen Route only enough to supply the raw owned
attachment and redial the same eligible Node processes after an externally
injected Carrier failure.

The first real tracer keeps the same two Application IPC streams and one active
Instance, transfers at least `4 MiB` of seeded incompressible data in the tested
direction, injects one failure at a manifest-seeded non-record-aligned offset
after at least `256 KiB` has been delivered, and requires an unpredictable
`32-byte` recovery canary through Route generation 2. Run client-to-publisher
and publisher-to-client as separate cells.

The positive S4.1 cell passes only when the same connection resumes ordered
unique bytes within the applicable `5 s` target; a terminal result is a recovery
miss and cannot pass that cell, though it must still arrive by `15 s`. The old
channel is not reused, no Application reconnect occurs, and all state cleans.
Separate negative cells require one terminal result for no alternate, explicit
cancellation, deadline, forged attachment, queue-full, and Endpoint restart.

### S4.2 — Leg and Rendezvous replacement

Add finite alternate candidates and the remaining recovery layers:

- replace each eligible failed Route role in isolated cells;
- retain the same Rendezvous while one failed leg is replaced;
- stop the Rendezvous, perform one fresh sealed Introduction attempt, and commit
  a fresh Rendezvous attachment; and
- run three sequential eligible failures during one 10-minute continuously
  loaded connection, striking the then-current Route only after the previous
  recovery canary.

Failed Node/container identities and channel instances remain unavailable.
Every event has its own pre-failure byte, recovery canary, `5 s` percentile
membership, and `15 s` terminal deadline. All three canaries and a final canary
arrive on the same local Application streams. Three is a qualification workload,
not a runtime quota.

The bounded development topology retains the Stage 2 four-position Route and
uses its Introduction-domain actor for an authenticated sealed recovery setup
step. This proves the Service Connection/Route replacement seam only. It does
not qualify the complete accepted split-leg topology or a production separate
Introduction Path.

### S4.3 — Overlap, impairment, and recovery evidence

Add one direction-specific overlapping episode: stop a current Route resource,
then within `1 s` and before recovery completes stop a distinct resource used by
the in-progress replacement. A third qualifying path remains. The positive cell
passes only when the same connection resumes within `p95 <= 8 s`,
measured from the first interruption. A terminal result is a recovery miss, not
success, but must still arrive by `15 s` from that point.

Add separate 10-minute impaired-live runs at `300 ms` base end-to-end RTT,
independent `5%` loss in each direction, and `100 ms` p95 additional
per-direction jitter without complete interruption. Each direction must retain:

- p05 60-second goodput
  `>= min(2 Mbit/s, 25% of the paired impaired direct baseline)`;
- no zero-delivery interval over `5 s`;
- the same connection, Target, Instance, context, profile, ordering, queues, and
  security; and
- no Application-visible reconnect.

During all recovery/impaired runs, each complete endpoint process tree retains
p95 RSS `<= 512 MiB`, mean CPU `<= 50%` of one logical core, p95 one-second CPU
`<= 100%` of one core, and the `256 KiB` directional logical queue cap. The
impaired-live carrier ratio is `<= 2.0` at each endpoint; one recovery episode
adds no more than `8 MiB` carrier traffic at each endpoint over its paired no-
failure run, counting combined sent plus received bytes at that endpoint; and
p95 one-second carrier bitrate in each physical direction is
`<= min(25 Mbit/s, 80% of the usable link budget)`.

S4.3 ends with a real local Docker `10–30 minute` campaign and independent
evidence. This is development evidence only. Full R-023 qualification still
requires its platform matrix, at least `20` eligible recovery episodes per cell
and direction with the accepted success floors, five repetitions of sustained
workloads, controlled hosts/links, and every conjunctive security gate.
Development runs record each episode's membership in the `5 s` or `8 s`
threshold; they do not claim a qualified percentile from a smaller sample.

### S4.4 — Pressure, role capacity, and lifecycle

Do not invent numerical Node floors in code or this brief. S4.1–S4.3 recovery
counters are evidence inputs only; they do not satisfy P3-D3b4's prerequisite.
Before S4.4 begins, the complete R-013 bounded prototype must make Entry,
Interior, Rendezvous, Introduction, discovery, and Bridge work measurable on
the accepted topology. Amend R-023 P3-D3b4 or create its explicitly linked
follow-up record only from that complete evidence. The accepted decision must
freeze each required production role/subrole separately, including Destination
Resolution where the contract treats it as a Rendezvous subrole:

- the useful-work unit and offered-load mix;
- setup, live-connection, forwarded-byte, recovery, abandoned-work, and control
  accounting included in that unit;
- reference Ubuntu `2 vCPU`/`2 GiB`/symmetric `100 Mbit/s` profile;
- maximum exclusion union and effective post-exclusion capacity floor;
- CPU, RSS, FD/socket, goroutine/thread, queue, timer, GC, traffic, latency,
  progress, cleanup, and reserve gates;
- saturation and honest overload result;
- `NORMAL/PROTECT/DRAIN/EXIT` high/low watermarks and hysteresis; and
- stronger-host scale factor, proportional useful-work requirement, and an
  evidence-derived finite reserve without extra trust or role priority.

The recovery implementation must already emit bounded neutral counters that can
inform that decision—authenticated attachment setups, live attachment time,
forwarded bytes, sealed Introduction attempts, Rendezvous joins, failed and
abandoned attempts, cleanup latency, and resources—but those counters are not
capacity units, do not cover missing roles, and are not floors until accepted by
P3-D3b4.

S4.4 then implements the accepted workload plus the existing R-023 hostile
endpoint cells: anonymous incomplete establishment pressure, honest admission
while finite slots remain, and admitted hostile read/write backpressure. It
must protect established useful work, reject/drain new expensive work before
resource collapse, never classify by IP/account/stable User identity, and never
evict or silently downgrade established connections to manufacture capacity.

S4.4 capacity evidence must run on the accepted controlled Ubuntu reference
host and complete accepted split-leg/Introduction topology; Windows Docker
Desktop or the four-position recovery fixture can provide development evidence
only. Full Stage 4 completion requires S4.4 under that later accepted contract.
A recovery-only pass leaves Stage 4 explicitly partial.

## TDD behavior order

Work one red → green behavior through the public seams at a time:

1. the no-failure Stage 3 Application stream remains byte-identical while the
   endpoint handshake moves to the Service Connection seam;
2. logical offsets and connection acknowledgements advance independently in
   each direction without changing Application-visible bytes/results;
3. Carrier acknowledgements cannot release unacknowledged Application bytes;
   full logical queues backpressure within `256 KiB`;
4. initial continuity derivation agrees only across the exact valid endpoint
   handshake and raw secret material never enters observations;
5. a fresh attachment accepts the exact handshake binding, connection bindings,
   offsets, and nonces;
   every mutated, replayed, rolled-back, cross-connection, cross-Target, cross-
   Instance, cross-context, cross-profile, cross-Network, and stale-safety value
   terminates the affected Service Connection fail-closed;
6. generation-2 retransmission of an unacknowledged range presents bytes once,
   in order; conflicting bytes terminate;
7. one same-Node Carrier failure recovers without replacing either Application
   stream or resetting the evidence clock;
8. absent candidates, expired Work Safety, Local Grant revocation/drain,
   Endpoint restart, cancellation, and `15 s` expiry each produce one honest
   terminal result and empty ownership;
9. one failed leg is replaced while the Rendezvous identity remains fixed;
10. Rendezvous loss uses a fresh sealed Introduction attempt and distinct
    Rendezvous process;
11. every Route role fails in an isolated cell, and three sequential distinct
    failures recover without failed-resource reuse;
12. a replacement proposal fails during the overlapping episode, a later one
    wins, and neither failure resets the first clock;
13. impaired-live transfer meets progress, order, queue, resource, and traffic
    bounds without recovery misclassification;
14. proposal exhaustion, malformed/oversized/partial/slow frames, delayed old
    records, slow Applications, and repeated abandonment remain finite;
15. `NORMAL/PROTECT/DRAIN/EXIT`, hostile pressure, role capacity, and scale-up
    follow only the later accepted P3-D3b4 contract; and
16. evidence mutation, missing fault/process/range/sample, contradiction,
    candidate self-report, secret retention, or incomplete cleanup is `invalid`,
    while complete candidate misbehavior is `fail`.

Use external-package behavior tests at the same interfaces as callers. Private
protocol/vector tests may verify canonical encodings and cryptographic adapter
use, but must not create a second exported testing interface. Run the smallest
affected tests after each behavior, `make quick-check` throughout, and
`make check` before every slice integration using the repository-approved Go
1.26.5 toolchain.

## Controlled development topology

The real Docker fixture contains separate processes/containers for:

1. client Application Principal with `network_mode: none` and only scoped IPC;
2. client Endpoint and Service Connection owner;
3. publisher Application Principal with `network_mode: none` and only scoped IPC;
4. publisher Endpoint and unchanged active Service Instance;
5. separately granted publication operator;
6. external/offline Authority fixture, which exits before transfer;
7. up to three separately keyed eligible candidates for each of Initiator,
   Introduction, Rendezvous, and Responder—at most `12` Node processes;
8. host-owned fault controller and external observer, outside the candidate data
   path; and
9. an independent verifier container, which never forwards Application Data.

Three candidates per role support an active path, one failed in-progress
replacement, and one further eligible replacement. They do not support a claim
of three sequential failures of the same role; that cell would require four
candidates for that role. The three-sequential development workload therefore
strikes declared distinct current resources, while isolated cells cover every
role. No stronger claim is inferred.

The client Endpoint receives one current authenticated Candidate View and makes
each complete selection. Every Node receives only its role-local plan and
adjacent current peer material. No Node receives all candidate plans, Target,
continuity state, fault schedule, or complete Route history. The publisher
Endpoint receives only the attachment information required by its role.

The host harness precommits the fault family and seed, then chooses the exact
eligible target from observed current process/container identity without
signalling it to the candidate. For overlap it observes that the replacement
resource has started and injects the second fault within `1 s`. Host management
control never becomes a socket, volume, proxy, or byte-forwarding path available
to the candidate.

Application containers have no host network, Docker socket, DNS/proxy/ambient
network, shared-memory transfer, or shared Application Data volume. Endpoint and
Node links use only declared literal Compose addresses. Each service mounts only
its own bounded role inputs. Verifier and observer evidence mounts contain no
private credentials and grant no candidate read access.

Docker Desktop provides development process/network isolation only. It does not
qualify native Ubuntu resource timing, separate physical/VM hosts, Windows IPC,
hostile same-desktop-user isolation, anonymity, operator independence, or
decentralization.

## Fault and workload contract

The host controller injects faults at the container/network boundary rather
than through candidate-only test hooks:

- a Carrier failure makes the current channel unusable while keeping the
  selected Node process alive for same-leg redial;
- a Node/leg failure stops or isolates the exact current Node/container and
  leaves it unavailable for the remainder of the cell;
- Rendezvous failure stops the exact current Rendezvous process;
- overlap stops a distinct observed resource participating in the uncommitted
  replacement; and
- impaired-live shaping applies the frozen bidirectional delay/loss/jitter
  profile without a complete interruption.

If Docker cannot inject one fault independently and prove it externally, that
cell is `invalid`; a candidate-visible debug command or cooperative self-failure
cannot substitute for it.

Qualification Applications generate and verify seeded incompressible byte
streams outside the core. The manifest fixes seeds, direction, offered load,
minimum delivered prefix, fault family, eligible injection window, and fresh
canaries before candidate behavior. Fault offsets are not aligned to protected
record boundaries and are not disclosed to the candidate. Core code treats all
content as opaque bytes and learns no canary framing.

Direct baselines are measurement-only, run on the same endpoint environment and
impairment before and after the associated batch, and can never become a
fallback. Use the exact R-023 drift and evidence rules for any claimed ratio.

## Resource and pressure ownership

Every recovery allocation retains the hierarchy:

```text
host/profile
  -> Endpoint or Node role
    -> peer / Service / Isolation Context
      -> Service Connection or Route Attachment
        -> proposal / stream / range / buffer / timer
```

Reserve before creating a goroutine, socket/FD, timer, queue/range, cryptographic
state, process, or evidence file. A child never enlarges its parent budget.
Uncommitted proposals share the existing connection and endpoint caps rather
than multiplying them.

`PROTECT` stops optional/prepared work and rejects or reduces new expensive
proposals while preserving finite control and established progress. `DRAIN`
accepts no new work, advertises negative readiness, closes/cancels by deadline,
and does not silently return to ready in the same process. `EXIT` means all
owned listeners, processes, sockets, tasks, timers, queues, keys, temporary
state, and fixture resources are gone. Return from `PROTECT` requires all later
accepted low watermarks for their continuous hysteresis interval.

Record GOMAXPROCS, GOMEMLIMIT, GOGC/profile identity, process-tree RSS/CPU,
heap/objects/allocation, GC CPU/limiter/pauses, goroutines, OS threads,
FDs/sockets, queues/ranges, timers, traffic, dropped work, progress, and
quiescence. Raising limits, splitting an omitted helper process, suppressing
security/liveness work, or reducing offered useful work cannot manufacture pass.

## Evidence and independent verdict

Use new versioned `ardents-h3-recovery-*-v1` schemas. Do not widen Stage 3
evidence until it ambiguously means both stages. The retained bundle binds:

- source commit, image/binary identities, Compose/topology digest, manifest,
  seeds, environment/profile, and verifier identity;
- one client/publisher Application process, principal, session, local IPC
  identity, and uninterrupted stream lifetime per attempt;
- fixed Target, active Instance/Credential generation, Network, context, profile,
  Destination Binding, and protocol commitments, plus the authenticated
  Candidate View and complete initial/refresh Work Safety history;
- publicly recomputable canonical connection/handshake-binding commitments and
  opaque, non-reusable continuity-key agreement observations without raw key or
  reusable credential material;
- every proposed and committed Route generation, selection/exclusions,
  role-local process/container identities, fresh handles/nonces by commitments,
  and old/new resource distinction;
- external monotonic injection time, exact stopped resource, last delivered
  pre-failure offset/time, recovery canary offset/time, and terminal deadline;
- proof that failed resources stayed unavailable and abandoned resources stopped;
- expected/observed byte ranges, digests, acknowledgements, duplicate/conflict
  outcomes, queue/range/resource/traffic samples, and backpressure;
- all mandatory negative results and structural absence of direct/short/DNS/
  proxy/ambient/shared-file/verifier data paths; and
- cleanup, private-fixture removal, evidence freeze, verifier output, and outer
  digest.

The external observer's same-host monotonic clock is authoritative for recovery
intervals. Candidate timestamps cannot start, stop, reset, or repair a timing
result. Missing pre-failure delivery, injection, canary, terminal, resource,
traffic, process identity, or clock binding makes the affected judgment
`invalid`, not a latency miss chosen by guesswork.

The independent production verifier imports no candidate Service Connection or
Route validator. It parses bounded values itself; recomputes all public
canonical commitments and selection/fault/sequence/timing/resource/cleanup
conjuncts; and does not claim to derive or recompute an erased exporter or
continuity key. Secret-dependent agreement is covered by independently
constructed known-answer adapter tests plus externally precommitted mutation,
replay, and cross-binding injections whose accept/reject, byte, generation, and
terminal effects are externally observed. Matching candidate-emitted opaque
commitments alone cannot establish pass. The verifier returns exactly:

- `pass`: evidence is complete/reliable and every frozen candidate conjunct
  passes;
- `fail`: evidence is complete/reliable and candidate behavior violates one or
  more frozen conjuncts; or
- `invalid`: fixture, observer, clock, manifest, secret handling, evidence
  integrity, verifier independence, or cleanup cannot support judgment.

Wrong bytes, duplicate/reordered presentation, hidden reconnect, wrong binding,
stale attachment acceptance, deadline/resource/traffic breach, forbidden path,
failed-resource reuse, candidate leak, or dishonest terminal result is `fail`.
Missing/mutated/contradictory/unbound/secret-bearing evidence or incomplete
cleanup is `invalid`. A candidate cannot self-report pass.

## Package and command ownership

Do not create a new production package for recovery: deepen `internal/serviceconn`
and `internal/route`, keep `internal/applicationipc` topology-blind, and keep
`cmd/ardents-service`/`cmd/ardents-route` thin.

When the first real Docker slice is implemented, the exact new qualification
boundaries are authorized after the entry gate:

- `internal/qualification/recovery`: independent bounded Stage 4 verifier;
  production imports standard library only;
- `internal/qualification/recoverysmoke`: host-owned Docker preparation,
  fault/observer lifecycle, evidence freeze, verifier invocation, and cleanup;
- `cmd/ardents-recovery-qualify`: thin independent-verifier container command;
- `cmd/ardents-qualify recovery-smoke`: thin public local campaign entry; and
- `tests/qualification/h3-recovery-v1`: human-authored Dockerfile, Compose
  topology, and bounded non-secret configuration only.

Create each package with `doc.go`, maintained implementation, behavior tests, a
non-test caller, exact package-map imports, and command ownership in the same
change. Do not add `recoverynegative`, a generic protocol package, or another
subpackage speculatively; add a cohesive boundary only when real implementation
and caller require it. Generated plans, keys, images, captures, run state, and
evidence remain outside Git.

No runtime dependency is expected. If standard-library TLS/HMAC and existing
Modules cannot implement the tracer, stop and research the dependency before
editing `go.mod` or the dependency register.

## Real Docker development gate

One public `ardents-qualify recovery-smoke` command must:

1. require a clean committed source tree and record the exact commit;
2. create new canonical, symlink-safe, mutually disjoint fixture/evidence roots
   outside Git;
3. build once, pin and inspect the image and embedded binary/source identities;
4. prepare fresh random credentials, continuity inputs, Candidate View, fault
   manifest, streams, and role-local plans;
5. launch the complete topology without candidate Docker-management access;
6. run the selected S4.1/S4.2/S4.3 cells for the requested bounded duration;
7. collect external monotonic, process/container, resource, traffic, and cleanup
   evidence while never proxying Application Data;
8. freeze the bounded attempt bundle, invoke the separate verifier container,
   and preserve its exact `pass|fail|invalid` verdict;
9. erase raw Authority/Instance/continuity/session material before terminal
   evidence; and
10. tear down twice, prove idempotent empty ownership, remove the private fixture
    root, and write the retained summary/outer digest.

Fixture and evidence roots may not equal, contain one another, resolve through a
symlink into one another, or lie inside the source tree. Evidence files become
container-readable only for the bounded verifier interval and return to private
host modes afterward. Cleanup failure is `invalid` even when candidate behavior
otherwise passed.

A short functional run may exercise S4.1 quickly, but the accepted local
development gate lasts `10–30 minutes` and includes every implemented mandatory
cell in both required directions. It neither pools cells nor contributes
partial R-023 qualification credit.

## Non-goals and stop conditions

No Service Name/private resolution, Bridge/blocked entry, installer/update/
rollback, process-restart live restoration, cross-Target or cross-Instance
migration, offline delivery, Application receipt, semantic retry, exactly-once
operation, datagram interface, production transport/wire/IPC/storage, consensus,
blockchain, anonymity, decentralization, operator independence, or public
release claim enters Stage 4 recovery.

MPTCP and QUIC remain prior art, not selected foundations. Prepared bounded
alternate state may be measured, but simultaneous multipath Application
delivery or unlimited multihoming is not required. Three recovery events are a
workload, not a maximum healthy-connection lifetime.

Stop and return to R-032 when implementation needs:

- another Application IPC admission, reconnect, hidden Service Connection, or
  operation replay;
- changed Target/Instance/context/profile/Destination Binding or weakened Work
  Safety;
- Service Connection sequence/acknowledgement semantics inside Route or Route
  topology/recovery controls inside Applications;
- online Service Authority, secret-bearing evidence, reusable public recovery
  token, or custom cryptographic primitive;
- direct/short/DNS/proxy/ambient/shared-file/verifier forwarding;
- reset clocks, unlimited attempts, additional recovery queue allowance,
  unbounded memory/disk/goroutines/timers/sockets, or accumulated abandoned
  state;
- candidate-visible fault schedule or cooperative self-failure as qualification;
  or
- a production dependency/foundation or numerical Node-capacity floor without
  the required research decision.

Stop or redesign the candidate if the honest result is retry storms, recurrent
false suspicion, inability to fit `5 s`/`15 s` timing, duplicate/lost bytes,
security downgrade, established-work starvation, traffic amplification,
GC/resource collapse, incomplete cleanup, or evidence that cannot distinguish
same-connection recovery from reconnect.

## Definition of Done

Recovery development is complete only when:

- the entry gate is recorded and R-032/this brief are accepted consistently;
- the Application interface remains unchanged and topology-blind;
- Service Connection owns endpoint handshake, continuity, byte/ack/replay,
  cutover, deadlines, Work Safety, result, and cleanup behind its small
  interface;
- Route supplies fresh role-valid bounded attachments without Target,
  continuity, Application, or semantic-result ownership;
- S4.1 Carrier, S4.2 leg/Rendezvous/sequential, and S4.3 overlap/impaired cells
  pass separately in both required data directions;
- replay/rollback/cross-binding/safety, malformed/slow, queue/backpressure,
  cancellation/restart/terminal, forbidden-path, evidence-integrity, and cleanup
  negatives pass;
- positive eligible recovery cells keep exact bytes ordered and unique on the
  same two local Application streams within the applicable recovery target;
  terminal-by-deadline counts only for declared negative/no-safe-alternate cases
  and otherwise remains a recovery miss;
- clocks never reset, failed resources never return, and all committed/failed/
  abandoned recovery state reaches quiescence within bounds;
- resource, traffic, queue, timing, process separation, role-local knowledge,
  secret handling, and verifier-independence gates pass;
- every new package/command is cohesive, mapped, tested, called, and within file
  and import rules with no new runtime dependency;
- a clean committed tree passes `make check`, then a real `10–30 minute` local
  Docker campaign passes digest and cleanup recheck; and
- final Standards and Spec reviews have no actionable findings.

Full Stage 4 is complete only when, in addition:

- the complete bounded R-013 role prototype exists and R-023 P3-D3b4 accepts
  exact per-role useful-work and effective post-exclusion-capacity contracts
  from complete role evidence, with recovery measurements as one input;
- S4.4 passes those reference-role capacity, hostile pressure,
  `NORMAL/PROTECT/DRAIN/EXIT`, leak/GC, established-work, and stronger-host
  scale-up cells on the accepted Ubuntu reference host and complete accepted
  topology; and
- the final report clearly separates development measurements, official
  qualification still owed, honest limitations, and prerequisites for Stage 5.
