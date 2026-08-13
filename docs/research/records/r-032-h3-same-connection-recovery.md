---
id: R-032
title: Can Stage 4 preserve one live Service Connection across eligible Route failure?
status: open
owner: Product Owner
started: 2026-08-13
reviewed: 2026-08-13
---

# R-032 — H3 same-connection recovery

## Decision this unlocks

Decide the exact Horizon 3 Stage 4 recovery seam and the first bounded tracer
that may be implemented after Stage 3 is accepted. The tracer must keep one
external Application stream and one exact-Target-authenticated Service
Connection alive while eligible Carrier Channels, Route legs, or a Rendezvous
are replaced. When no safe eligible replacement exists, safety state expires,
the Application cancels, or recovery misses its terminal bound, the same stream
must instead return one explicit terminal Connection Result by the accepted
deadline. A terminal result does not count as successful recovery when a
qualifying alternate remains.

The proposed contract is translated into an executable, still-unauthorized
[Stage 4 implementation brief](../../development/horizon-3-stage-4-brief.md).

This record deliberately does not invent production Node-capacity floors before
the complete R-013 bounded role prototype exists. Recovery development adds
useful counters and measurements, but cannot by itself satisfy the R-023
P3-D3b4 prerequisite or qualify roles absent from its four-position fixture.
P3-D3b4 must later set role-specific useful-work units and effective
post-exclusion capacity from the complete prerequisite evidence. Stage 4 is not
closed by recovery alone.

R-032 selects no production wire protocol, transport, cryptographic protocol,
Route construction, IPC, persistence engine, deployment foundation, or
Application SDK.

## Current contract

The accepted [J-06 recovery journey](../../product/journeys.md) and
[threat model](../../security/threat-model.md) are the product and security
authority. The canonical glossary defines
[Service Connection, Carrier Channel, Route Module, and Work Safety Lease](../../../CONTEXT.md).
`Route Attachment` is an implementation-local R-032 term for one bounded owned
byte channel plus selection/cleanup observations; it is not new product
language. Accepted
[ADR-0003](../../adr/0003-bounded-service-instance-credentials.md),
[ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md),
[ADR-0008](../../adr/0008-stage-research-before-public-network.md),
[ADR-0009](../../adr/0009-go-project-foundation.md), and
[ADR-0010](../../adr/0010-modular-monorepository.md) constrain credentials,
topology, staged claims, the runtime, and package boundaries.

R-002 defines a Service Connection as a reliable ordered bidirectional byte
stream and forbids semantic retry or an implied remote-Application receipt.
R-004 fixes endpoint-only connection continuity, replaceable Carrier Channels,
three recovery layers, fresh opaque handles, and a separate Introduction role.
R-023 fixes the recovery timings, impaired path, traffic, queue, endpoint
resource, sampling, and evidence rules. R-024 fixes immutable Destination
Binding, refreshable-but-finite Work Safety, exact failure language, and loss of
live connections on process restart. R-031 fixes the Stage 3 exact Target/active
Instance and local Application Interface seams.

The accepted local Stage 3 candidate is intentionally one-shot. One `serviceconn.Request`
contains one `Route` stream; `exchangeExact` copies one fixed byte count and the
operation closes that Route. There is no connection-level byte sequence,
acknowledgement state, replacement-Route attachment, or continuity owner. This
is a valid Stage 3 non-goal, not evidence of Stage 4 recovery.

The Product Owner reported Stage 3 complete against its local DoD at commit
`6c8faf9daef2d0f03009d4b6708825d45be1c434`: `make check` and final
Standards/Spec reviews were clean; one retained local Docker campaign passed
`27/27` attempts over `10m47.8703023s`, and an independent verifier replay over
that same frozen 27-attempt bundle also returned `27/27`; image
`sha256:7c3453123a91232b7624ea2eddb04bd6e7f2383c9b4cbe1dd001b5c57f0fbeb2`
and retained-evidence digest
`9aea2d37de910dec39cce79187fde94b49d53a10f0a6bab3a5ca14e6955162ae`
match the terminal summary; Docker and private-fixture cleanup passed. This
closes the Stage 3 local development gate only. Stage 4 development promotion
still requires Product Owner acceptance of this record. Official Stage 1 Ubuntu
`short`, current `churn-2h`, independent `unattended-24h`, and applicable R-023
qualification remain prerequisites for the integrated H3 verdict or any
stronger external, privacy, security, or release claim.

## Hypotheses

- **H1:** a deep endpoint Service Connection Module can own continuity, logical
  byte order, bounded replay, recovery deadlines, and terminal results while
  the Route Module supplies replaceable authenticated Route Attachments.
- **H2:** Route-owned recovery can preserve the same observable stream without
  moving exact Target, active Instance, Isolation Context, Application
  backpressure, or Connection Result semantics into the Route Module.
- **H0:** the outcome needs an Application-visible reconnect, operation replay,
  a direct/short path, reusable public recovery identifier, unbounded retained
  data or attempts, online Service Authority, custom cryptographic primitives,
  or a production foundation choice.

## Evaluation criteria

### Observable Application outcome

The client and publisher Applications each open exactly one admitted local
Connection surface and keep that same local stream for the complete attempt.
They do not reconnect, resubmit an operation, choose a replacement Route, learn
a Route generation, or receive a recovery interface.

Before, during, and after every successful recovery:

- the Service Target, active Service Instance and Credential generation,
  Isolation Context, Route Profile, Network identity, Destination Binding, and
  stream identity remain identical; any Work Safety extension is fresh,
  authenticated, accepted before expiry, and never exceeds its signed maximum;
- each direction presents one continuous ordered byte sequence with no missing
  or duplicate byte;
- a fresh unpredictable post-injection canary arrives only through the new
  committed Route generation after its predecessors;
- a local write count still means only locally accepted bytes, not remote
  Application processing; and
- clean close, cancellation, abrupt loss, and indeterminate completion retain
  the R-002 Connection Result meanings.

If recovery cannot remain safe or finish within its bound, the same local stream
terminates once. A hidden replacement Service Connection is failure, not
recovery.

### Deep Module seam and ownership

The Service Connection Module owns one endpoint control block per live
connection:

- original exact-Target-authenticated handshake-binding commitment and one
  connection-only continuity key;
- independent `uint64` logical byte offsets for each direction, starting at
  zero and terminating before overflow;
- sent-but-not-connection-acknowledged bytes, received ranges, the next
  Application-deliverable offset, and bounded acknowledgement ranges;
- one committed Route generation plus bounded candidate attachments;
- the non-resetting recovery episode start/deadline, current authenticated Work
  Safety lease and terminal bound, and terminal Connection Result; and
- complete ownership and erasure of buffers, attempts, timers, tasks, handles,
  keys, and evidence projections.

The Application-facing interface remains read, write, close, cancel, and one
classified terminal result. `replace route`, `retry hop`, `ack operation`, and
`resume connection` are not Application operations.

The Route Module owns endpoint-local selection and construction of a fresh
role-valid path. It returns one bounded Route Attachment capable of carrying
opaque protected records and owns every underlying Node, leg, Carrier Channel,
deadline, and cleanup obligation. It does not own or learn the continuity key,
Service Connection sequence space, Application operation, or semantic
result. A Route Attachment is neither a new Service Connection nor proof of
Application success.

This division passes the deletion test: without the Service Connection Module,
continuity, deduplication, replay bounds, Work Safety, and terminal semantics
would spread across both Applications and Route actors. Without the Route
Module, selection, role-local topology, replacement candidates, and carrier
cleanup would spread into the Service Connection implementation.

The package direction remains acyclic. `internal/serviceconn` does not import
`internal/route`; Endpoint composition supplies one narrow open-attachment
function over concrete Service Connection request/result values and the existing
route socket/process seam. The callable carries only Network/Profile/role/
exclusion constraints and a deadline and returns one owned byte channel plus
bounded observations. Its exact signature and composition-root imports must be
recorded in `package-map.md` with the implementation; no speculative production
interface or qualification-only abstraction is authorized.

### Continuity and fresh attachment contract

The initial endpoint-authenticated connection handshake derives pseudorandom,
connection-specific exported keying material known only to the two Endpoint
processes. A domain-separated continuity key is derived from that material. It
is never sent as a bearer, persisted, placed in evidence, exposed to an
Application or Node, reused by another connection, or restored after endpoint
restart.

For the laboratory tracer, the recommended adapter is TLS 1.3 exported keying
material at the Service Connection seam, with an Ardents-specific label and
non-empty canonical context. The existing Go standard library exposes this
reviewed RFC 8446 mechanism. It is a replaceable laboratory adapter, not a
production TLS selection. The Route Module's present internal end-to-end TLS
cannot be treated as the continuity owner because its state terminates with the
replaceable Route; the Service Connection seam must possess the endpoint
handshake state from which continuity is derived.

Every candidate attachment uses a new endpoint handshake, fresh nonces, fresh
opaque handles, fresh route/session keys, and a strictly increasing proposed
Route generation. Its endpoint-only continuity proof binds:

1. the original handshake-binding commitment and connection protocol version;
2. exact Target, active Instance public key and Credential generation;
3. Network identity, Destination Binding, Isolation Context, and Route Profile;
4. the current authenticated Work Safety lease, its signed maximum, and
   `no-new-leg-or-recovery-after` bounds;
5. proposed Route generation and both endpoints' fresh nonces;
6. the new endpoint handshake/exporter commitment; and
7. each direction's sent, acknowledged, received, and next-deliverable offsets.

The exact canonical laboratory frames may use standard-library HMAC only as a
reviewed primitive around this key. They require independent vectors,
mutation/replay negatives, connection, protocol, endpoint-role, and proof-
direction domain separation, constant-time verification, and secret-erasure
tests before use. This is an RFC 8446 Section 7.5 application-specific exporter
use, not RFC 9266's public `tls-exporter` channel-binding value; the latter must
not be used as a secret key. A bespoke primitive, reusable public token, or
production wire claim is a stop condition.

The laboratory commitment does not require raw TLS transcript access from Go.
It hashes the canonical connection binding together with a separate domain-
separated RFC 8446 exporter output; its value is evidence-safe only as a one-way
commitment and is never the continuity key or an attachment bearer.

Only one candidate wins a cutover. A committed generation must be greater than
the prior committed generation. An ordinary candidate that becomes unavailable,
expired, or ineligible may be abandoned before another safe proposal. Detected
stale/equal generation, replay, forged proof, nonce/handle reuse, conflicting
bytes, or cross-binding state is instead an active integrity/authentication
violation and terminates the affected Service Connection fail-closed; it is not
retried as ordinary unavailability. Skipped proposal numbers are allowed, so a
delayed abandoned attempt cannot force rollback. Nodes see fresh opaque handles
rather than a connection-stable identifier. Timing and volume can still
correlate generations under the accepted low-latency limitation.

### Ordered byte and acknowledgement contract

Application bytes are assigned offsets when accepted into the finite local
connection queue. A protected data record maps one contiguous logical range to
one Route generation. Retransmission may map the same logical range to another
generation, but the receiver presents only the first valid copy and only when
all predecessor bytes are present.

Connection acknowledgements describe bytes accepted into the remote Connection
Module's bounded delivery state. They do not prove that the remote Application
read, processed, or persisted the bytes. The sender retains logical bytes until
connection acknowledgement, and the receiver merges duplicate/overlapping
ranges. Acknowledgement metadata is finite; the initial tracer permits at most
`8` disjoint received ranges per direction and terminates safely rather than
allocating beyond the bound.

The accepted `256 KiB` logical queue cap per Service Connection and direction
includes Application IPC buffering, sent-unacknowledged bytes, receive
reordering, replay/deduplication state attributable by bytes, and recovery
cutover buffering. Recovery adds no queue allowance. Physical copies remain
inside the complete endpoint process-tree RSS gate. Backpressure stops local
acceptance at the cap; eviction, silent loss, cross-connection borrowing, or
unbounded disk spill is failure.

### Recovery state and bounded attempts

One connection follows:

```text
ESTABLISHED -> SUSPECT -> RECOVERING -> CUTOVER -> ESTABLISHED
       \            \          \             \-> TERMINAL
        \------------\----------\----------------> TERMINAL
```

Qualification measures from the last Application byte delivered before the
first injected failure. Candidate liveness detection may start later but cannot
move that evidence clock. A second failure, a new attachment, or an internal
retry never resets the episode deadline.

Recovery attempts the accepted layers without an Application-visible event:

1. replace a failed Carrier Channel inside a still-valid leg;
2. attach a fresh leg to the same live Rendezvous;
3. after Rendezvous loss, use a new sealed Introduction attempt and fresh
   Rendezvous; then
4. terminate explicitly when no safe eligible attachment remains or the
   deadline expires.

The first development profile permits at most `3` attachment proposals in one
episode and at most `2` uncommitted proposals concurrently. Only the committed
attachment may advance Application delivery; temporary old/new transmission is
allowed solely for authenticated cutover and deduplication. All traffic and
state from parallel, failed, superseded, and abandoned proposals are charged.
These limits reset only after the connection has returned to `ESTABLISHED` and
a later distinct failure begins a new episode; they do not impose a lifetime
quota or permit an episode deadline to reset.

New attachment work requires current Common Readiness Base, Candidate View,
Credential, Destination Binding, and Work Safety at that moment. A fresh
authenticated state update may extend Work Safety before expiry, but never past
its signed maximum; stale, uncertain, expired, or revoked state cannot extend
it. Learned supersession, invalid Time Confidence, Recovery Pending, Released,
changed Target, expired safety, or revoked/vulnerable state blocks attachment
and closes by the earliest terminal deadline. Local Grant revocation blocks new
work immediately and preserves only an explicitly authorized bounded drain,
which cannot exceed Work Safety. Restart, crash, reboot, suspend that loses
process state, or update is explicit connection loss, never recovery.

### Timing, impairment, and resource gates

The tracer inherits R-023 rather than creating softer Stage 4 numbers:

- one eligible Node or Carrier Channel failure while a qualifying alternate
  remains: continuation `p95 <= 5 s`; a terminal result is a recovery miss, not
  a successful positive cell, and must still arrive by `15 s`;
- three sequential eligible failures in one 10-minute direction-specific run,
  each after the prior recovery canary, each with its own clock, with failed
  resources remaining unavailable;
- one overlapping pair where the second distinct failure occurs within `1 s`
  before recovery completes and a third qualifying path remains: continuation
  `p95 <= 8 s`; a terminal result is a recovery miss and must still arrive by
  `15 s`, both measured from the first interruption;
- one 10-minute impaired-live run at `300 ms` base RTT, independent `5%` loss
  each direction, and `100 ms` p95 additional per-direction jitter: p05
  60-second goodput at least
  `min(2 Mbit/s, 25% of paired impaired direct baseline)` and no zero-delivery
  interval over `5 s`;
- each complete endpoint process tree during impaired/recovery runs: p95 RSS
  `<= 512 MiB`, mean CPU `<= 50%` of one logical core, and p95 one-second CPU
  `<= 100%` of one core; and
- impaired-live carrier ratio `<= 2.0` at each endpoint, at most `8 MiB` extra
  carrier traffic at each endpoint per recovery episode over its paired no-
  failure run, counting combined sent plus received bytes at that endpoint, and
  p95 one-second carrier bitrate in each physical direction
  `<= min(25 Mbit/s, 80% of usable link budget)`.

The initial local development campaign may run only `10–30 minutes` and one or
more episodes per implemented cell to expose regressions. It earns no partial
qualification credit and records only each episode's threshold membership, not
a qualified percentile from a smaller sample. Full qualification retains at
least `20` independent
eligible episodes per cell and direction, at least `19/20` successful expected
continuations, five independent repetitions of sustained 10-minute workloads,
and all other R-023 evidence rules.

### Stage 4 recovery topology and evidence

The development fixture retains the external client and publisher Applications,
their scoped IPC, two Endpoint processes, one unchanged active Service Instance,
the offline Authority fixture, the current four-position Stage 2 Route roles,
a finite authenticated alternate-candidate pool, fault controller, external
resource observer, and independent verifier.

Keeping the current four-position Route makes this a bounded development seam
tracer. It does not qualify the complete accepted split-leg data topology,
separate production Introduction Path, public Node role capacity, anonymity, or
operator independence.

The first profile may stage at most three eligible candidates per Route role.
Its three-failure schedule strikes distinct current resources, while isolated
fresh-fixture cells cover every role and recovery layer. The overlapping cell
retains a third qualifying route after stopping both the committed resource and
a distinct resource used by its replacement. A same-role three-sequential-fault
claim would require four candidates for that role and is not inferred. The
candidate receives authenticated candidate material but never the injection
schedule or verifier expectation.

Independent evidence binds:

- one Application process, admission, session, local socket/handle, and Service
  Connection lifetime on each side;
- fixed Target, active Instance, Credential generation, Network, context,
  profile, Destination Binding, and protocol commitments, plus the complete
  authenticated initial/refresh Work Safety history;
- publicly recomputable canonical connection/handshake-binding commitments and
  opaque, non-reusable continuity-key agreement observations without raw keys;
- every proposed/committed Route generation, fresh endpoint-handshake
  commitment, process/container identity, selected role commitments, and old/
  new resource distinction;
- fault-controller target and monotonic injection time, last pre-failure byte,
  recovery canary, terminal deadline, and proof that failed resources stayed
  unavailable;
- expected and observed logical ranges, acknowledgements, byte digests, order,
  duplicate suppression, queue/resource/traffic high water, and backpressure;
- externally injected rejection cases for stale generation, replay, wrong
  handshake binding, nonce, Target, Instance, context, profile, Network, safety
  bound, and exporter commitment; and
- cleanup of every committed, failed, superseded, and abandoned attachment,
  Route process, connection, timer, task, buffer, handle, key, socket, and
  fixture secret.

The verifier observes management evidence only and never forwards Application
Data. It recomputes public canonical commitments and does not claim to derive or
recompute the erased exporter/continuity key. Secret-dependent agreement is
tested through independently constructed known-answer adapter tests plus
externally precommitted mutation/replay/cross-binding injections and externally
observed accept/reject, byte, generation, and terminal outcomes. Matching
candidate-emitted opaque commitments alone never establishes pass. `pass`
requires every recovery, security, resource, timing, forbidden-path, and cleanup
conjunct. Complete reliable evidence of candidate violation is `fail`; missing,
contradictory, secret-bearing, unbound, unverifiable, or cleanup-incomplete
evidence is `invalid`. Candidate self-report cannot turn a result into pass.

### Implementation, distribution, and developer experience

The tracer remains standard-library-first Go 1.26.5 in the existing root module
and adds no runtime dependency, licensing obligation, production distribution
format, public protocol, or end-user installation surface. If the standard
library cannot support the exact seam, development stops for a separate
dependency and license review rather than silently changing `go.mod`.

The Application interface remains the accessible Stage 3 byte-stream/result
surface: no recovery vocabulary, timing judgment, or topology choice is added
to Application or User workflows. Developer cost is limited to one explicit
Service Connection/Route Attachment seam, versioned bounded evidence, and
external-package behavior tests; an interface that requires callers to
coordinate offsets, retries, or cleanup fails this criterion.

## Evidence plan

### Primary sources

Accessed 2026-08-13:

- accepted [R-002](r-002-live-application-interface.md),
  [R-004](r-004-routing-rendezvous-families.md),
  [R-023](r-023-interactive-route-performance-budget.md),
  [R-024](r-024-operational-product-closure.md), and
  [R-031](r-031-h3-service-connection-application-interface.md), plus the
  [operating model](../../product/operating-model.md),
  [threat model](../../security/threat-model.md), and
  [H3 technical design](../../development/horizon-3-technical-design.md);
- [RFC 8684](https://www.rfc-editor.org/rfc/rfc8684.html), especially the
  separation of connection-level data sequence/acknowledgement from subflow
  sequence, authenticated fresh-subflow join, retransmission mapping, ordered
  unique presentation, and finite connection-level windows;
- [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html), especially fresh path
  validation challenges, bounded connection-ID/path state, migration cutover,
  and its explicit linkability and server-migration limitations;
- [RFC 8446 section 7.5](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5)
  for application-specific TLS 1.3 exported keying material, plus
  [RFC 9266](https://www.rfc-editor.org/rfc/rfc9266.html) as the explicit
  warning that its public fixed-label `tls-exporter` channel-binding value is
  not a secret key; and
- Go 1.26.5 [`crypto/tls.ConnectionState.ExportKeyingMaterial`](https://pkg.go.dev/crypto/tls#ConnectionState.ExportKeyingMaterial),
  `crypto/hmac`, `crypto/sha256`, `crypto/rand`, `crypto/subtle`, `io`, `net`,
  and `context` documentation.

MPTCP and QUIC are architectural evidence, not selected transports. MPTCP joins
address-based TCP subflows and permits fallback semantics Ardents must not
inherit. RFC 9000 migration validates a new network path but does not replace an
Ardents split Route, Introduction, Rendezvous, or server endpoint. RFC 8446
exported keying material is selected only for the bounded laboratory adapter
because it binds an endpoint handshake without adding a runtime dependency.
RFC 9266's public `tls-exporter` value is not used as the continuity key.

### Experiment

Run the versioned recovery tracer on the existing developer-controlled Docker
topology with two external Applications, two Endpoint processes, the active
Service Instance, four Stage 2 Route positions, bounded alternate candidates,
an external fault controller/resource observer, and an independent verifier.
Inputs are a clean committed source tree, pinned image identity, versioned
topology/impairment manifest, deterministic public seeds, and freshly generated
private fixture material outside Git.

For each cell, transfer seeded incompressible bytes over one admitted Service
Connection, inject the manifest-selected failure externally after proven
pre-failure delivery, and observe recovery or the required terminal outcome.
Retain source/image/topology identities, public inputs, role/process facts,
monotonic timings, logical ranges and digests, queue/resource/traffic samples,
negative outcomes, cleanup record, verifier result, and outer bundle digest;
erase raw private and continuity material before evidence is retained.

Another person can reproduce or falsify the result from the committed public
qualification command and manifest after implementation. Until that command
exists, this record is a predeclared experiment contract, not measurement.

#### Required development cells

After accepted Stage 3 evidence, implement and evaluate in this order:

1. one Carrier Channel failure with the same leg and same Service Connection;
2. fresh-leg attachment to the same live Rendezvous;
3. fresh sealed Introduction and Rendezvous replacement;
4. terminal no-alternate and expired/unsafe recovery cases;
5. replay, rollback, cross-Target/Instance/context/profile/Network/handshake,
   stale-safety, malformed/oversized/partial/slow attachment, and secret-leak
   negatives;
6. three sequential failures and failed-resource non-reuse;
7. the overlapping pair and non-resetting clocks;
8. impaired-live transfer; and
9. bounded pressure, abandonment, quiescence, and repeated cleanup.

Each cell runs separately in both Application Data directions where R-023
requires it. A single full-duplex smoke may find bugs but cannot substitute for
direction-specific results.

### Failure scenarios

- malicious replay, rollback, cross-connection/cross-binding attachment,
  conflicting bytes, forged proof, malformed/oversized/partial/slow frames, and
  secret-bearing evidence;
- Carrier, leg, Rendezvous, alternate-candidate, Endpoint-process, and verifier
  loss, including overlapping and three sequential failures;
- absent candidates, proposal exhaustion, expiry, cancellation, Local Grant
  revocation/drain, invalid Time Confidence, and Work Safety refresh rejection;
- high loss/jitter/RTT, slow Applications, queue pressure, hostile incomplete
  establishment, abandoned work, traffic amplification, and cleanup failure;
  and
- captured/cooperative candidate reporting, forbidden fallback paths, reused
  failed resources, unverifiable clocks, incomplete evidence, and operator-
  independence claims unsupported by the project-controlled fixture.

### Falsification criteria

H1 is falsified, and design returns to research, if any successful cell needs:

- another Application admission, local IPC connection, Service Connection, or
  operation submission;
- a changed Target, active Instance, context, profile, Destination Binding, or
  weakened Work Safety state;
- direct, shortened, DNS, proxy, ambient, shared-file, or verifier-forwarded
  Application Data;
- continuity key or stable connection handle visible to a Node, Application,
  evidence consumer, or another connection;
- duplicate/missing/reordered Application presentation, false remote receipt,
  or bytes freed before connection acknowledgement;
- a reset recovery clock, unlimited retries/parallel attachments, extra queue
  allowance, monotonic leak, traffic amplification, or established-work
  starvation;
- Route topology, recovery controls, or Application semantics crossing the
  wrong Module seam;
- custom cryptographic primitive, online Service Authority, reusable private
  credential, or production foundation selection; or
- candidate-controlled/incomplete evidence accepted as pass.

## Findings

- **Sourced fact:** R-023 and the operating model already decide the externally
  observable recovery outcome, timing, queue, traffic, Work Safety, and honest
  failure limits. R-032 must not renegotiate them downward.
- **Sourced fact:** RFC 8684 maintains connection-level byte sequence and Data
  ACK state above replaceable subflows, authenticates a new subflow as belonging
  to the original peers/connection, and ignores later copies of already received
  connection-level data. This supports H1's shape without selecting MPTCP.
- **Sourced fact:** RFC 9000 requires unpredictable path validation and finite
  path/connection-ID state, but its migration is address/path-specific and does
  not provide Ardents Rendezvous or whole-Route replacement. It also states that
  timing and size can still correlate paths.
- **Sourced fact:** RFC 8446 TLS 1.3 exporters derive context-bound keying
  material from the handshake, and Go 1.26.5 exposes that standard mechanism.
  RFC 9266's standard channel-binding value is non-secret and is not the chosen
  input. The application-specific exporter is enough for a no-new-dependency
  laboratory continuity adapter, subject to exact attachment negatives.
- **Measurement:** the accepted local Stage 3 result at commit `6c8faf9` passed
  one retained `27/27` Docker campaign; an independent verifier replay of the
  same frozen bundle also returned `27/27`, with matching retained evidence and
  complete cleanup. Its implementation has one supplied
  Route stream, one exact bounded copy, and no connection-level recovery state.
- **Inference:** Route-owned recovery would either remain shallow or absorb
  Target/Instance binding, Application byte order, backpressure, and results,
  violating the existing Module seams. H2 is rejected for the first tracer.
- **Inference:** a numerical production role-capacity floor chosen now would be
  arbitrary. Recovery can expose authenticated attachment, forwarding,
  abandonment, and cleanup counters, but P3-D3b4 still requires the complete
  R-013 bounded prototype and role evidence before defining production units.
- **Assumption:** the bounded same-host Docker topology can expose functional,
  security-contract, resource, and cleanup faults for development. It cannot
  establish cross-host timing, anonymity, operator independence, or release
  qualification.

## Options

### H1 — Connection-owned continuity over replaceable Route Attachments

Recommended. It preserves the small Application interface and concentrates
continuity proof, ordered unique byte presentation, queue accounting, deadlines,
and terminal results in the Module that already owns Service Connection
semantics. Route selection and role-local transport remain independently
replaceable. The strongest risk is complexity in one live connection control
block; the mitigation is one external seam, explicit finite state, and tests
through that seam rather than exposing recovery internals.

### H2 — Route-owned transparent recovery

Rejected for the first tracer. The Route Module can replace paths but cannot
decide whether two streams are the same exact-Target Service Connection or when
Application bytes are uniquely deliverable without importing higher-level
state. A pass-through callback into Service Connection would split one invariant
between Modules and make failure cleanup ambiguous.

### Transport-native MPTCP or QUIC migration

Rejected as the Stage 4 contract. Both are useful prior art, but their path,
address, peer, fallback, and linkability models do not implement Ardents'
endpoint-chosen split Route and Rendezvous replacement. Either may later be
evaluated as a bounded Carrier Adapter under the same frozen interface.

### Application reconnect or replay

Rejected. It changes the Service Connection, exposes recovery policy, and risks
reissuing an Application operation whose prior remote completion is unknown.
It remains an explicit Application choice only after a terminal result.

### H0 — Stop after Stage 3

Required if H1 cannot meet the exact timing/resource/security gates without a
forbidden shortcut or premature production choice. An honest explicit failure
is preferable to disguised reconnect or unbounded recovery.

## Recommendation

Choose H1 after Product Owner review. First deepen Service Connection so it owns
one connection-level sequence/acknowledgement space and endpoint-only continuity
state; let Route supply bounded fresh attachments. Implement the recovery layers
as vertical tracers in the required order, with a `10–30 minute` local Docker
campaign after each meaningful slice and no qualification claim.

Do not define production role-capacity floors in R-032. Recovery measurements
become one input, not the prerequisite, to the complete R-013 bounded prototype
required before R-023 P3-D3b4 can decide role-specific units, controlled
saturation, effective post-exclusion capacity, NORMAL/PROTECT/DRAIN/EXIT
behavior, and stronger-host scale-up on the accepted Ubuntu reference host.
Confidence is medium-high in the seam and medium in the `5 s` target;
the strongest counterargument is that complete fresh-Route setup plus endpoint
proof may not fit that target without prepared bounded state. The first three
tracers are designed to falsify that assumption before capacity work expands.

## Disposition

- State: `open`, ready for Product Owner decision; this draft authorizes no
  Stage 4 code.
- The clean committed Stage 3 local development gate is closed; Stage 4
  development remains gated on explicit acceptance of R-032.
- The draft Stage 4 implementation brief splits work into recovery core,
  Route/Rendezvous replacement, overlapping and impaired-live recovery, then
  evidence-driven pressure/capacity work. It becomes implementation authority
  only after the entry gate and R-032 acceptance.
- P3-D3b4 role-specific production capacity remains open until the complete
  bounded R-013 role prototype and its accepted Ubuntu reference-host evidence
  exist; recovery counters are only one input.
- No ADR: the proposed decision is a replaceable tracer seam and laboratory
  adapter, not a hard-to-reverse technology selection.
- `CONTEXT.md` needs no new product term: Service Connection, Carrier Channel,
  Route Module, Work Safety Lease, and existing recovery language already
  preserve the product distinctions. `Route Attachment` remains local to the
  implementation record and brief.
- No experiment code, raw secret, generated fixture, capture, or evidence is
  added by this research record.
