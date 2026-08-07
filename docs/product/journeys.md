# Network product journeys

These journeys define observable behavior of Ardents as a network. They avoid
selecting protocols, libraries, implementation languages, or application
semantics.

## J-01 — Start and join Ardents

**Actor:** User or Developer

**Start:** A newly installed local Ardents endpoint on a supported Windows or
Linux desktop/laptop

**Flow:** Endpoint Owner starts the endpoint → verify software/network state →
obtain current bootstrap information → join through an available entry path →
report ready or an exact degraded state

**Done when:** a local Application can use the Application Interface without a
phone, email, wallet, central User account, network administrator approval, or
manual routing configuration.

**V1 platform gate:** the same ready outcome is required on both Windows and
Linux; a result demonstrated only on an infrastructure server is insufficient.
On the normal non-adversarial reference network, an installed process reaches
this network-ready state within `p95 <= 5 s` on routine restart with valid state
and `p95 <= 15 s` on a clean first start. The clock does not stop at a local
socket or UI; authenticated current network state and a usable entry path are
required.

For the first 10 minutes after reporting ready, an otherwise idle required
client keeps the complete Ardents process tree at
`p95 resident memory <= 256 MiB` and mean CPU `<= 1%` of one logical core. It
must remain network-ready and continue required background and security work;
disconnecting or moving work to an uncounted helper does not satisfy the gate.

Once already joined, the same client has a secondary steady-idle efficiency
target of at most `25 MiB` of combined sent and received Ardents carrier traffic
per 24 hours. Initial bootstrap, explicit software-package payloads, and blocked
or degraded recovery are measured separately. This target never requires
disconnecting or suppressing security work and is not a hidden cover-traffic
profile.

## J-02 — Open an Unlisted Service

**Actor:** User

**Start:** An exact human-readable Service Name already known by the User

**Flow:** enter exact name → resolve and verify Name Record → obtain current
Service reachability → establish an Interactive Route → authenticate the Service
Target → expose the authenticated target in the result → open a Service
Connection

**Done when:** the Application reaches the intended live Service or receives an
explicit failure. No directory search occurs, and possession of the name is not
shown as authorization or secrecy. The Interactive Route is not a direct path
or single proxy, and no one ordinary Node links the User's location to the
Service Name, Service Target, or Service Instance location. The Service may
still recognize identity disclosed by Application Data, credentials, client
fingerprinting, timing, or behavior. The route is presented as implementing this
privacy claim only when its exact implementation candidate has current Route
Qualification; otherwise the journey is visibly an experiment or simulation.
On the normal non-adversarial reference network, the connection part of this
journey completes within `p95 <= 3 s` without a prepared Route and
`p95 <= 1 s` with current authenticated state and reusable Route state for the
same Isolation Context, measured from Application submission to an
authenticated, usable Service Connection.

## J-03 — Publish a local Service

**Actor:** Developer

**Start:** A local application server and an Ardents endpoint on a supported
Windows or Linux desktop/laptop

**Flow:** Endpoint Owner grants Authority Custody to an administration tool →
create or securely import Service Authority → obtain its Service Target → grant
per-Service administration → choose one active local listener → publish
authenticated, expiring reachability without exposing raw authority → bind or
update Service Name → accept a test Service Connection

**Done when:** a remote Application can connect while neither the User nor any
one ordinary Node can link the Service Instance's public origin address to its
Service Name or Service Target outside the declared Route Profile. Stopping the
local Service produces an explicit unavailable result, not implied offline
delivery. A routine migration can stop the old Instance, import the encrypted
authority on a new host, and republish the same Service Target. A required
publisher reference endpoint supports at least `256` concurrently open incoming
Service Connections, including at least `64` simultaneously active. This is a
minimum total publisher capacity, not a Service maximum; one Service may use the
whole budget when local policy permits. The active test keeps all `256`
connections open while `64` share `40 Mbit/s` of delivered Application Data.
Throughout the run, the complete Ardents process tree keeps
`p95 resident memory <= 1 GiB` and mean CPU `<= 100%` of one logical core. The
published Application's own work is excluded, but every connection must keep
progressing and all Ardents publication and carrier work remains counted. Under
the controlled equal-load benchmark, every connection averages at least
`500 kbit/s` and has no zero-delivery interval longer than `2 s`. At the
publisher network boundary, all Ardents bytes sent plus received remain at or
below `1.5x` the Application Data delivered in the tested direction. The other
`192` connections remain authenticated and usable as the same streams rather
than being silently evicted. Ardents queues no more than `256 KiB` of logical
Application Data per connection and direction or `64 MiB` across the publisher
per direction. If the published Application stops consuming, receiver flow
control propagates backpressure; Ardents does not hide the stall with loss,
eviction, or an unbounded memory or disk queue.

The same publisher also protects established work during a 10-minute anonymous
pre-establishment flood on a symmetric `100 Mbit/s` link. With all `256`
connections open and `64` offered the normal `40 Mbit/s` aggregate workload,
the endpoint receives `1,000` validly framed but incomplete attempts per second
at no more than `20 Mbit/s` inbound attacker traffic. All established streams
remain usable; the active set delivers at least `32 Mbit/s` aggregate, every
active stream averages at least `400 kbit/s` with no gap over `5 s`, and the
inactive set passes unpredictable canaries without reconnecting. Publisher
`p95 RSS` stays within `1 GiB` and mean CPU within one core. Ardents assumes no
IP, global User account, or stable attacker identity, bounds and cleans up
incomplete-attempt state, and never presents it to the published Application as
an accepted Service Connection. This gate protects established work; honest new
admission during the flood remains a separate requirement.

## J-04 — Integrate an Application

**Actor:** Developer

**Start:** Existing client/server application logic

**Flow:** receive a narrowly scoped Local Grant → separately authorize Service
administration when publishing is needed → use the least-privileged local
Connection Interface → receive a safe default Isolation Context or deliberately
select an additional one → supply either exact Service Name or Service Target →
resolve the name when needed → authenticate and expose the exact target → connect
or accept → read and write opaque bytes → handle close, timeout, backpressure,
and classified failure

**Done when:** the Application can use its own protocol without treating a Node
ID as an application address, embedding a mandatory Ardents SDK, or importing
routing internals. The Application remains responsible for User identity,
authorization, persistence, semantic retry, and data format. Access to connection
traffic alone does not expose Service Authority or Service administration.
Failed name resolution or target authentication never falls back to another
destination or the ordinary network. After a partial write or connection loss,
the network never claims that the remote Application processed the bytes. The
Isolation Context remains local and cannot become an application or network
identity. No Endpoint Owner or Local Grant becomes an authority over the Ardents
network. The journey remains within its declared setup-latency, throughput,
memory, CPU, fairness, and overload budgets under both honest and adversarial
load. Under the normal single-connection throughput workload, the 60-second
Application goodput in each direction has
`p05 >= min(10 Mbit/s, 50% of paired direct-baseline goodput)`; carrier overhead
and failed runs do not count as useful payload. A required client reference
endpoint also supports at least `64` concurrently open outbound Service
Connections, including at least `16` simultaneously active. This is a minimum
total client capacity, not a maximum number of connections to one published
Service. The active test keeps all `64` connections open while `16` share
`10 Mbit/s` of delivered Application Data in separate runs in each direction,
and the complete Ardents process tree keeps
`p95 resident memory <= 512 MiB` and mean CPU
`<= 50%` of one logical core. Under the controlled equal-load benchmark, every
connection averages at least `500 kbit/s` and has no zero-delivery interval
longer than `2 s`. At the client network boundary, all Ardents bytes sent plus
received remain at or below `1.5x` the Application Data delivered in the tested
direction. The other `48` connections remain authenticated and usable as the
same streams rather than being silently evicted. On stronger hardware the
endpoint may raise its finite hierarchical local budgets, while an Endpoint
Owner may cap them. Reduced limits are exposed locally and do not qualify as the
V1 performance floor; added capacity grants no Node role, authority, trust, or
security exception. The required client profile queues no more than `256 KiB`
of logical Application Data per connection and direction or `16 MiB` across the
client per direction. At a full leaf or parent queue, a write blocks or reports
would-block instead of accepting bytes it cannot retain. Timeout or cancellation
affects only the unaccepted remainder; an accepted prefix is never a claim of
remote Application delivery and is never silently discarded.

A stronger endpoint automatically selects only a previously qualified profile
compatible with its current finite resources. A claimed scale factor increases
open connections, active connections, and aggregate delivered Application Data
together in the same 10-minute workload while leaving at least `20%` of every
declared CPU, memory, and usable-link parent budget free. The first failed
profile is saturation and is not selected automatically. The Endpoint Owner may
always cap lower; an explicit higher experimental cap remains unqualified.

## J-05 — Use the Named Unlisted Site tracer

**Actors:** Developer and User

**Start:** A local HTTP server and a desired Service Name

**Flow:** publish HTTP server as Service → bind name → enter exact name in
reference client → resolve → connect → exchange HTTP bytes → migrate the
authority to a new host without changing the target → simulate compromise by
creating a replacement target and rebinding the same name

**Done when:** the site opens through the generic Service Connection; routine
migration preserves both target and name; compromise preserves only the name;
one eligible ordinary route failure preserves the same logical connection; and
terminal failure remains visible. No replicated Site Bundle, Ardents runtime,
or built-in application identity is required. On the normal non-adversarial
reference network, a running, network-ready endpoint receives the first valid
HTTP response byte from the controlled tracer within `p95 <= 4 s` without a
prepared Route and `p95 <= 2 s` with current authenticated state and reusable
Route state for the same Isolation Context. Rendering and arbitrary Service
processing are not part of this network KPI.

## J-06 — Continue through degradation or recover from a failed path

**Actor:** User or Developer

**Start:** An active or attempted Service Connection whose entry or Route is
degraded, blocked, or failed

**Flow:** authenticate target and protocol state → reject detected modification,
replay, redirection, or downgrade → classify only supported facts → obtain
alternate network state or Bridge when required → attempt bounded safe route
and Carrier Channel recovery within the same Service Connection → restore it or
return a product-level failure class or honest indeterminate result → let the
Application decide whether to open a new connection

**Done when:** with both endpoints, the same active Service Instance, and one
qualifying alternate Route still available, loss of one ordinary Node or
Carrier Channel resumes ordered delivery through the same Service Connection
within `p95 <= 5 s`, measured from the last byte delivered before failure to an
unpredictable post-failure canary delivered through the recovered path.
Pre-failure buffered bytes cannot end the clock. Target, Isolation Context,
Route Profile, and stream identity do not change, and no byte is lost or
presented twice.

If recovery has not succeeded by `15 s`, the Service Connection terminates
explicitly rather than hanging or silently reconnecting. Carrier-level
retransmission may preserve the stream, but Ardents never reissues an
Application operation or claims that interrupted work completed. Detected
active violations still fail closed; no direct fallback, Node identity, or
route topology is exposed. The outcome is mandatory for the complete V1 stack
regardless of which transport-specific Carrier Channels it uses.

The same journey is also qualified under three sequential eligible failures in
one 10-minute run. Each next failure affects the current Route only after the
previous recovery canary arrives, while the failed Node or channel instance
remains unavailable. All three recovery canaries and a final canary arrive
through the same still-usable Service Connection. Three is a test workload, not
a runtime quota or a reason to close after the third successful recovery.

When the Route remains live, a separate 10-minute degraded-path qualification
uses `300 ms` base end-to-end RTT, independent `5%` packet loss in each
direction, and `100 ms` `p95` additional per-direction jitter. In separate
Application Data directions, the same Service Connection has no zero-delivery
interval longer than `5 s`, and its `p05` 60-second goodput is at least
`min(2 Mbit/s, 25% of the paired impaired direct baseline)`. It remains
exact-target-authenticated, open, ordered, non-duplicating, and usable without
an Application-visible reconnect or security downgrade. A complete traffic
interruption is evaluated as recovery, not as success in this degraded-live
profile.

An overlapping-failure qualification also runs separately in each Application
Data direction for 10 minutes. The first failure stops the current Route; within
`1 s`, before a recovery canary arrives, the second stops a distinct ordinary
Node or Carrier Channel used by the in-progress replacement attempt. When both
endpoints, the same active Service Instance and target, and a further qualifying
Route remain, the same Service Connection delivers the final recovery canary
within `p95 <= 8 s` from the first interruption or terminates explicitly by
`15 s` from that point. The second failure never resets the clock. Recovery
retains stream order, uniqueness, identity, security, and Isolation Context
without an Application-visible reconnect or Application-operation replay.

Across every 10-minute impaired-live, single-failure, sequential-failure, and
overlapping-failure run, each complete Ardents endpoint process tree stays
within `512 MiB` `p95` RSS, `50%` mean CPU of one logical core, and `100%` `p95`
one-second CPU of one core. The `256 KiB` per-connection and direction queue cap
and every ancestor cap remain unchanged. Completed or abandoned recovery state
does not accumulate across failures. These limits apply together with the
useful-progress, deadline, and security outcomes rather than replacing them.

The impaired-live run keeps total endpoint carrier bytes at or below `2.0x`
delivered Application Data in the measured direction. Each recovery episode
adds at most `8 MiB` per endpoint over a paired no-failure run; the overlapping
pair is one episode. Across all impaired and recovery runs, each endpoint
network direction keeps `p95` one-second carrier bitrate at or below
`min(25 Mbit/s, 80% of its declared usable link budget)`. Retransmission,
abandoned attempts, control, padding, security, liveness, and background bytes
remain counted, so retry storms cannot hide inside a ten-minute average.

## J-07 — Contribute network resources

**Actor:** Network Contributor

**Start:** A host with bounded bandwidth and possibly other resources

**Flow:** install → choose explicit network role and limits → self-check → join
→ observe privacy-safe health → update → withdraw gracefully

**Done when:** the Node helps the carrier without reading Application Data,
becoming a Service or User identity, or silently retaining an unbounded duty
after exit. Every selected V1 role must demonstrate useful bounded operation on
a Linux `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS. Stronger
hardware may contribute more bounded capacity but gains no automatic role,
trust, authority, or route-selection priority.

## Cross-journey failure cases

Every implementation proposal must exercise at least these cases:

- bootstrap information is stale, conflicting, blocked, or malicious;
- one ordinary entry, relay, discovery, or rendezvous Node is malicious, slow,
  or absent;
- one Node modifies, injects, replays, redirects, delays, drops, or tags traffic;
- nominally different Nodes, including both endpoint-adjacent roles, share one
  operator, network, software supply chain, or jurisdiction;
- a Name Record is stale, expired, rolled back, or equivocating;
- a Service Descriptor is unavailable or points to no reachable Service
  Instance;
- both an old and a new host publish with copies of one Service Authority;
- a Service Authority is lost, corrupted, or suspected compromised;
- a Service goes offline before connect, during handshake, or mid-operation;
- a route fails after the Application has written some bytes;
- an Application reuses one Isolation Context across identities or contexts that
  should not be linked;
- an Application creates Services or Isolation Contexts to evade its parent
  resource budget;
- a local Application attempts to exceed connection, bandwidth, or queue limits;
- a slow reader attempts to create unbounded buffering or starve other grants;
- a censor blocks known entry addresses and protocol fingerprints;
- an official endpoint or protocol update channel is compromised or unavailable.

For an Interactive Route candidate, any forbidden endpoint, edge-observer, or
single-Node disclosure, or any silently accepted substitution, modification,
replay, redirect, or downgrade in these cases, fails Route Qualification.
