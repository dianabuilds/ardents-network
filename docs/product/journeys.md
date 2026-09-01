# Network product journeys

These journeys define observable behavior of Ardents as a network. They avoid
selecting protocols, libraries, implementation languages, or application
semantics.

They are completed by the accepted
[product operating model](operating-model.md), which defines package trust,
capability-specific readiness, diagnostics, update, and withdrawal.

These are cross-horizon journeys, not one implementation plan. The authoritative
order is [product scope](scope.md). Carrier Lab, the conditional Named Unlisted
Site tracer, and later bounded alpha work are retained evidence, not separate
product identities. Current implementation may compose one Product
Owner-selected headless candidate from accepted contracts and must state its
claims directly; delivery-horizon labels must not enter runtime behavior. Public
join/contribution, Bridges, and full qualification remain later promotion
gates unless separately selected.

## J-LAB — Falsify the Route candidate

**Actor:** Product Researcher

**Start:** Controlled Ubuntu LTS client, publisher, and synthetic infrastructure
fixtures; one preconfigured authenticated Target/reachability record; one active
Service Instance; project-owned ephemeral test keys

**Flow:** start the fixed topology → construct the current five-position Route
candidate and separate Introduction → authenticate the Target/Instance → exchange
one deterministic byte stream → capture each role's traffic and state → measure
coarse setup, goodput, CPU, and RSS → stop one path position → observe bounded
continuation or explicit terminal failure

**Done when:** evidence shows exactly what every role learned, whether the
Product Core knowledge boundary was violated, whether one useful stream remained
plausible on modest hardware, and whether the candidate should continue, change,
or stop. No Service Name, public discovery, Bridge, installer/updater, Windows
cell, SDK, general Application sandbox, or public network claim is present.

## J-00 — Install, repair, and remove Ardents

**Actor:** Endpoint Owner

**Start:** A supported Windows 11 or Ubuntu LTS `x86-64` host and an authenticated
Installed or Portable Distribution Profile artifact obtained through any
distribution channel

**Flow:** verify threshold release metadata, platform binding, artifact digest,
and public-network identity → install the package or directly run the Portable
executable as an unprivileged default Endpoint → create local IPC and finite
state directories → keep publishing and contribution disabled → start or remain
offline → later repair/replace the program artifact, update, create an encrypted
Authority Recovery Bundle, or remove explicitly

**Done when:** distribution is not release authority; Installed wraps the exact
platform executable released directly as Portable, with the same Interfaces,
features, claims, and protected-state compatibility. Installed owns package/registration
lifecycle; Portable remains a directly runnable executable with only unavoidable
declared companions, requires explicit pre-execution verification, and uses
stopped replacement/removal. A raw executable cannot establish its own
authenticity after it has already started. No account, wallet,
User identity, Service, or Node was silently created; remote administration is
disabled; repair preserves Authorities and freshness watermarks. A test restore
is isolated and non-signing;
restored authority remains `authority locked` until authenticated reconciliation
permits a strictly higher generation/revision, and Local Grants/runtime Instance
Keys are never derived from the Bundle. Installed uninstall and deletion of a
Portable executable never silently erase protected state. A supported state
removal retains required watermarks and either preserves a non-empty Authority
Vault or blocks until a Recovery Bundle is explicitly exported and verified at
an Owner-chosen destination. No secret or location is invented silently.
A separate confirmed destructive purge enumerates affected authority classes,
distinguishes disposable cache from authority material, warns that recovery may
be impossible, and states that secure deletion of external snapshots or backups
cannot be guaranteed.

## J-01 — Start and join Ardents

**Actor:** User or Developer

**Start:** A newly deployed local Ardents endpoint on a supported Windows 11 or
Ubuntu LTS `x86-64` desktop/laptop

**Flow:** Endpoint Owner starts the endpoint → verify release, persistent
freshness, and Time Confidence → obtain and authenticate one current Network
Epoch through a finite precommitted source sequence, skipping any authenticated
identity/family already present in retained Entry/Interior/Introduction state or
live Route/Resolution work and recording every direct contact in the
Endpoint-wide Direct Source Exposure Set → detect conflict or rollback →
verify the logical Candidate View commitment and fetch the deterministic proven
Candidate Materializations required locally → join through each enabled
capability's own Entry path → report each Capability Readiness or exact lifecycle
state

**Done when:** a local Application can use every capability reported `ready`
without a phone, email, wallet, central User account, network administrator
approval, or manual routing configuration. Target Connect, Private Name
Resolution, Publish, and Contribute readiness are distinct and share only the
Common Readiness Base. Client Initiator, publication Responder/Introduction, and
Contributor prerequisites are not inherited from one another; a local socket or
one reachable Entry cannot make unavailable capabilities appear ready. Existing
network work is bounded by a Work Safety Lease and cannot survive stale or
revoked trust indefinitely. Direct-source retries and exclusion growth are
finite; source exhaustion or a post-authentication conflict with retained
endpoint-adjacent state is explicit unavailability unless one bounded
owner-approved replacement is permitted by the existing Entry exposure policy.
An Application or Isolation Context cannot reset source exposure or force fresh
sampling.

**V1 platform gate:** the same ready outcome is required on both Windows and
Linux; a result demonstrated only on an infrastructure server is insufficient.
On the normal non-adversarial reference network, an active process reaches
**Target Connect Ready** within `p95 <= 5 s` on routine restart with valid state
and `p95 <= 15 s` on a clean first start. The clock does not stop at a local
socket or UI; the Common Readiness Base, a usable Initiator Entry path,
materialized eligible data-path roles, and a qualified profile are required.
Private Resolution, configured Publish, and Contribute preparation receive their
own capability-by-platform budgets before a usable release.

A clean first start retains only the deployed candidate, frozen configuration,
trust roots, and declared bootstrap manifest; it has no state generated by a
prior Ardents execution. A routine restart may retain valid authenticated
persistent state and a non-decreasing freshness watermark, but the earlier
process is stopped and no live Service Connection or Carrier Channel survives.
State creation on clean start and state validation on routine restart remain
inside their startup clocks. `Clock uncertain`, `stale`, `conflicting`,
`blocked`, `disabled/unconfigured`, `authority locked`, `probation/draining`,
`incompatible`, `update state unavailable`, `update required`, `build revoked`,
`unqualified`, and `local resource unavailable` are explicit non-ready outcomes.

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

**Start:** An exact canonical human-readable Service Name, explicit
`ardents://` Service Link, Service Target, or explicit Target Link already known
by the User

**Flow:** enter the complete canonical name or open its Service Link → parse one
lowercase ASCII Service Name → resolve and verify Name Record → obtain current
Service reachability → prepare a fresh Rendezvous leg and separate Introduction
Path in parallel → send one sealed invitation over Introduction → let the Service
attach its independently selected data leg to the Rendezvous → authenticate the
Service Target and current Instance Key/Credential proof with fresh ephemeral
session keys → bind the exact supplied Name generation/revision→Target or pinned
Target provenance into the connection → expose the authenticated target in the
result → open a Service Connection

A Target or Target Link follows the same reachability, Route, Introduction,
authentication, and connection flow but intentionally skips Namespace
resolution. It still uses Private Reachability Resolution for the descriptor. A
failed Name is never reinterpreted as a Target destination.

**Done when:** the Application reaches the intended live Service or receives an
explicit failure. An Active name resolves normally; a Grace name remains usable
with a visible expiry warning; a Released name and any descendant resolve
nothing. Recovery Pending also resolves nothing until its successor issues a
fresh Name Record. No directory search or silent alternate-namespace, DNS,
search-result, or local-alias fallback occurs, and possession of the name is not
shown as authorization or secrecy. A naming participant may infer the queried
name or count its popularity, but receives no User location or stable User ID;
the endpoint-adjacent role receives no exact or publicly testable name value.
An incompatible naming-rule fork is explicit. A local filter may refuse the
name with visible local-policy state but cannot present another canonical
destination or erase the accepted Name Record.
The Interactive Route is not a direct path or single proxy. One ordinary Node
acting only from its role-local view, with no endpoint/second observation/probe
source under the same adversary, receives no direct User-origin-to-Name/Target/
Service-location binding. A Node plus controlled endpoint may still confirm a
known Target through timing/volume and is an explicit non-claim. The Service may
still recognize identity disclosed by Application Data, credentials, client
fingerprinting, timing, or behavior. The route is presented as implementing this
privacy claim only when its exact implementation candidate has current Route
Qualification; otherwise the journey is visibly an experiment or simulation.
That network claim covers only traffic submitted to Ardents. An
Application-level Endpoint Location Privacy claim additionally requires both the
client and published Application to run inside qualified Network-Isolated
Application Boundaries; a generic adapter with direct DNS/socket egress remains
usable but visibly lacks that stronger claim.
For a Name-origin connection, learned Recovery Pending, Release, or authenticated
rebind to another Target stops new leg/recovery work and closes by a finite
deadline; the stream never migrates to the new Target and the Application must
reconnect. Same-Target renewal or Grace may extend it with the warning. An
explicit Target-origin connection remains pinned and deliberately has no Name
catastrophe recovery.
After an honest connection closes and ephemeral keys are erased best-effort,
later compromise of Service/Node long-term keys does not decrypt its recorded
Application Data. A live endpoint compromise and keys retained in memory dumps,
swap, hibernation, or snapshots remain outside that Forward Secrecy claim.
On the normal non-adversarial reference network, the connection part of this
journey completes within `p95 <= 3 s` in the cold state and `p95 <= 1 s` in the
warm state defined below, measured from Application submission to an
authenticated, usable Service Connection.

Cold means the endpoint is network-ready but holds no prepared naming,
reachability, Route, session, or cache state for the exact Service Name, Service
Target, Isolation Context, and Route Profile. Warm may retain current
authenticated target data and reusable Route state for that same tuple, but
begins with no open Service Connection. Neither mode may be satisfied by an
Application or HTTP response cache; the Named Unlisted Site tracer always uses a
fresh request and new response data.

## J-03 — Publish a local Service

**Actor:** Developer

**Start:** A local application server and an Ardents endpoint on a supported
Windows 11 or Ubuntu LTS `x86-64` desktop/laptop

**Flow:** Endpoint Owner grants Authority Custody to an administration tool →
create or securely import Service Authority → obtain its Service Target → have
the new host generate a private Service Instance Key → reconcile current
authority/network generation → issue a public bounded monotonic Service Instance
Credential for that public key → remove or lock root authority from the runtime
boundary → grant per-Service administration → choose one active local listener →
establish separate Responder and Introduction Entry paths → publish
authenticated, expiring reachability and rotating Introduction Paths while
proving the Credential with the private Instance Key and without exposing raw
authority → separately authorize use of Name Authority → claim a
root Name Lease or receive a delegated subordinate Name Lease under bounded
Anonymous Cost and local admission → optionally commit Recovery Policy → bind or
update Service Name → produce its explicit Service Link → accept a test Service
Connection

**Done when:** a remote Application can connect while the User receives no
Service Instance origin and one ordinary Node, acting only from its role-local
view with no endpoint/second observation/probe source under that adversary,
receives no direct protocol origin-to-Name/Target link. `Publish
Ready` requires acknowledged fresh descriptor replication and overlapping
Introduction coverage, not merely a running local listener. Stopping the local
Service produces an explicit unavailable result, not implied offline delivery.
A routine migration stops the old Instance, has the new host generate a new
private Instance Key, then uses the reconciled Authority Vault or an offline
custody tool to issue a higher-generation public Credential for that key and
republishes the same Service Target. Copying the Credential alone grants no
power, and neither the old Instance Key, Service Authority, nor Name Authority
is imported into the new runtime. Connections cannot outlive the Credential
validity and authenticated supersession deadlines. If Service Authority is
instead lost or compromised, independently held Name
Authority binds the stable name to a newly created replacement Target. A required
claim mechanism must not let an observer win merely by copying a pending revealed
name; flooding, withholding, concurrent claims, and partitions remain explicit
test cases. The cost grants no human identity, fairness, trademark right, or
guaranteed protection from a more powerful squatter.

The carrier cannot stop an arbitrary published Application from exposing an
ordinary-network listener or performing DNS, callbacks, webhooks, SSRF, or
direct socket access. A generic Service remains
compatible but receives no Application-level Service-location privacy claim. A
claim-bearing profile contains the complete published Application/helper process
tree in a Network-Isolated Application Boundary, exposes no ordinary listener,
and fails external requests
instead of falling back outside Ardents.

The same endpoint may also act as a client only with separate Initiator,
Responder, and Introduction Entry Sets and Local Grants. The host and Local
Traffic Observer may correlate those roles. Standalone client/publisher capacity
figures are not additive; a release calls simultaneous co-residence usable only
after its combined Target Connect + Publish profile passes R-023.

The required publisher reference endpoint supports at least `256` concurrently
open incoming Service Connections, including at least `64` simultaneously active.
This is a minimum total publisher capacity, not a Service maximum; one Service
may use the whole budget when local policy permits. The active test keeps all `256`
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
an accepted Service Connection.

Honest anonymous admission remains usable during that same flood when capacity
exists. With `240` established connections and `16` free slots, one ordinary
honest client starts an unprivileged connection attempt per second. At least
`95%` of all `600` attempts authenticate the exact target, receive a usable
Service Connection, and pass a canary; connection `p95` is at most `8 s`, and
every attempt ends with an explicit result by `15 s`. Established work and the
publisher's P3-D5a resource ceilings remain intact. Any network-required client
check costs at most one logical-core CPU-second, `64 MiB` peak memory, and
`1 MiB` traffic and needs no money, account, IP reputation, stable identity, or
cross-context link. At full capacity Ardents may return an explicit capacity
result, but cannot evict another connection or hang.

If an attacker completes admission and fills ordinary Service Connections,
Ardents still isolates established work rather than pretending it can identify
the attacker. In the full-capacity publisher workload, `128` honest connections
share the Service with `128` valid admitted hostile connections. The hostile
set either sends data that the published Application does not consume or stops
consuming data written to it. Per-stream and aggregate queues stay bounded and
propagate backpressure; the `64` active honest streams retain the P3-D5a useful
work floors, `64` inactive honest streams pass canaries, and publisher RSS/CPU
remain within `1 GiB`/one core. Ardents receives no harness label, IP, account,
or stable identity with which to favor the honest set. While all `256` slots
remain occupied, a new User receives an explicit capacity-unavailable result by
`15 s`; V1 does not falsely promise per-person fairness or a free slot against
an indistinguishable admitted Sybil.

## J-04 — Integrate an Application

**Actor:** Developer

**Start:** Existing client/server application logic

**Flow:** receive a narrowly scoped Local Grant → separately authorize Service
administration when publishing is needed → use the least-privileged local
Connection Interface directly through the supported binary or from an external
Application → receive a safe default Isolation Context or deliberately select an
additional one → when an Application-level privacy claim is required,
attach the complete client/server process tree through a Network-Isolated
Application Boundary → supply either exact Service Name or Service Target →
resolve the name when needed → authenticate and expose the exact target → connect
or accept → read and write opaque bytes → handle close, timeout, backpressure,
and classified failure → optionally drain then revoke, or revoke immediately →
rebind the Application to its OS-local principal after process/Endpoint restart

**Done when:** the Application can use its own protocol without treating a Node
ID as an application address, embedding a mandatory Ardents SDK, or importing
routing internals. The binary path remains fully usable without a Browser,
extension, or URI registration. No Browser Adapter is selected in the
maintained product; any future Browser surface must be separately researched
and accepted before it can use the same Interface. The Application remains
responsible for User identity, authorization, persistence, semantic retry, and
data format. Access to connection traffic alone does not expose Service
Authority or Service administration.
Failed name resolution or target authentication never falls back to another
destination or the ordinary network. After a partial write or connection loss,
the network never claims that the remote Application processed the bytes. The
same no-fallback rule does not magically constrain arbitrary Application code:
generic adapters are marked as having unverified Application networking, while a claim-bearing
profile proves deny-by-default ordinary-network access at both endpoints. The
Isolation Context remains local and cannot become an application or network
identity. Diagnostics follow the same Local Grant: this Application receives
only bounded Connection Results and its authorized readiness, never peer, Route,
other Service, authority, or Endpoint Owner state. Grant revocation immediately
denies new work and invalidates child sessions; custody/admin closes immediately,
while data closes immediately unless a finite drain was explicitly selected
first. No ephemeral bearer survives restart, and no Endpoint Owner or Local Grant
becomes an authority over the Ardents network. The journey remains within its
declared setup-latency, throughput,
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

Creating an Application or Isolation Context cannot create a fresh Entry Set.
The endpoint reuses only its bounded installation-and-entry-regime exposure while
keeping every per-context channel, key, Interior, Rendezvous, destination cache,
continuity secret, and failure history separate. Applications receive no route
topology or raw diagnostic identifiers.

## J-05 — Use the Named Unlisted Site tracer

**Actors:** Developer and User

**Start:** Carrier Lab retained a viable Route candidate; controlled Ubuntu
client and publisher Applications exist; one exact Service Name and its
Target/reachability state are pre-provisioned by the test fixture

**Flow:** start the deterministic HTTP server and controlled single-response
client in a harness that exposes only scoped local IPC/loopback and supplies no
ordinary network path → publish the HTTP server as one Service
Instance → privately resolve the pre-provisioned exact Name → authenticate its
Target/Instance → exchange one nonce-bound HTTP response → stop the Service and
observe explicit unavailability → in a separate ordinary-migration slice,
generate a new private Instance Key and issue a higher-generation public
Credential without moving Service Authority or changing the Target

**Done when:** the controlled site opens through the generic Service Connection,
private exact-name resolution does not expose the querying origin in a forbidden
role view, ordinary migration preserves Target and Name, and offline/failure
state remains visible. The slice does not implement permissionless Name claims,
leases, delegation, Recovery Policy, catastrophe Target replacement, a public
Namespace, a generic browser sandbox, or the full R-023 latency matrix. It
records latency and resources as observations only. No replicated Site Bundle,
Ardents runtime, offline delivery, or built-in application identity is required.

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

Recovery first repairs a Carrier Channel, then may attach a fresh leg to the
same Rendezvous, and after Rendezvous loss may use a fresh sealed Introduction
attempt to attach both endpoints to a new Rendezvous. Every stage proves the same
endpoint-only continuity secret with fresh handles, keys, and monotonic route
generation. No relay receives a stable connection identifier; replay,
forged proof, nonce/handle reuse, or cross-target, cross-profile, or cross-
context attachment is a detected connection-control integrity violation after
establishment and terminates the affected Service Connection fail-closed. An
ordinary unavailable, expired, or newly ineligible candidate terminates only
that attachment attempt and may be followed by another bounded safe proposal.

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

**Start:** A dedicated Ubuntu LTS host/installation with bounded bandwidth and no
V1 User connection or Service publication role

**Flow:** install without enabling contribution → declare supported role
capabilities, precommitted identity/family material, and finite owner limits →
receive a finite deterministic Role Domain assignment under public epoch rules →
reject any new duty whose maximum lifetime does not fit assignment `not-after` →
self-check → enter probation → become eligible under one authenticated Network
Epoch → serve → on reassignment publish stop-new-work and drain/quarantine the
identity and known family until every old-domain duty terminates → become eligible
in the new domain only afterward → observe privacy-safe aggregate health → drain
→ update or withdraw gracefully

**Done when:** the Node helps the carrier without reading Application Data,
becoming a Service or User identity, or silently retaining an unbounded duty
after exit. Every selected V1 role must demonstrate useful bounded operation on
an Ubuntu LTS `x86-64` `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference
VPS. Stronger hardware may contribute more bounded capacity but gains no
automatic role, trust, authority, or route-selection priority.

A co-resident development Node is explicitly unqualified and contributes no
public capacity or decentralization evidence. An Endpoint never selects a
Contributor identity or declared family it controls into its own Route. Public
Client/Publisher+Contributor co-residence requires a future threat and combined
qualification profile and is not a V1 deployment mode.

An operator may declare a family, but different keys do not prove independence.
One identity and one honestly declared family occupy only one Role Domain for the
stable assignment lifetime. Withdrawal stops new assignments first, retains
bounded drain state, and then expires; a crash does not restore old route handles.

## J-08 — Update an Endpoint, Publisher, or Contributor

**Actor:** Endpoint Owner or Network Contributor

**Start:** A running Installed or Portable Endpoint and threshold-authenticated
release metadata

**Flow:** automatically check authenticated Release Safety metadata without an
installation ID, account, Service list, cohort, `from-version`, or exact build
history → use private-only retrieval by default, an explicitly enabled direct
source, or offline import, with no silent fallback → verify the `3-of-5` public
Targets authorization for a new executable, version, expiry, hashes, size,
platform, source/dependency inputs, SBOM, applicable qualification identity, two
matching independent build attestations, rollback watermark, build safety, and
protocol phase → under Installed, stage with finite disk reserve, stop accepting
new work by the signed deadline, drain within the earlier of local policy and
signed Work Safety Lease, atomically switch, self-test, and commit or safely roll
back → under Portable, authenticate the replacement executable, stop/drain the
Endpoint, explicitly replace it while stopped, then enforce the same release
floors, state compatibility, self-test, and safe previous-build rule before new
network work

**Done when:** update distribution did not acquire signing authority; a failed
update did not corrupt Authority Vault or freshness state; rollback can reach
only an authenticated schema-compatible non-revoked build; old protocol versions
never create a silent downgrade; connections closed by process replacement are
reported honestly; and local repair plus Authority export remain available even
when normal network work cannot restart. Protocol migration and build safety are
separate: ordinary current/previous protocol overlap lasts at least `90 days`,
while a vulnerable/revoked build has no such entitlement. A normal `required`
transition waits for qualified independent capacity plus drain reserve in every
Role Domain and required control/discovery role. An expiring `4-of-5` emergency
may bypass this only for a credible exploitable flaw, compromised primitive/key,
or demonstrated safety incompatibility, with an explicit possible-network-
unavailable result. Once Release Safety expires or a build is revoked, repair
uses only a preconfigured external privacy proxy, an explicit direct-disclosure
choice, or offline import—not an Ardents Route.

## Cross-journey failure cases

Every implementation proposal must exercise at least these cases:

- bootstrap information is stale, conflicting, blocked, or malicious;
- a fresh install has a rolled-back wall clock, insufficient Time Confidence,
  selective Candidate Materialization withholding, or a logged eligible Node is
  omitted/rejected inconsistently from the epoch-committed Candidate View;
- an ordinary or Bridge key is assigned or invited into more than one adjacent
  Role Domain, or a co-resident client/publisher attempts to reuse one Entry Set;
- a duty is proposed too near Role Domain Assignment `not-after`, or reassignment
  occurs while Entry, Bridge, Introduction, Resolution, or other old-domain work
  remains live: new work is rejected, identity/family stays in drain/quarantine,
  and emergency closes work rather than creating overlapping eligibility;
- a direct bootstrap/materialization/time/update source is already present in
  retained Entry/Interior/Introduction state or live Route/Resolution work,
  forces unbounded retries/exposure growth, or later appears in a forbidden role.
  The Endpoint follows finite precommitted source/candidate sequences, preserves
  Endpoint-wide exclusions, and returns explicit unavailability when
  bounded replacement or post-exclusion reserve is unavailable;
- one ordinary entry, relay, discovery, or rendezvous Node is malicious, slow,
  or absent;
- one Node modifies, injects, replays, redirects, delays, drops, or tags traffic;
- nominally different Nodes, including both endpoint-adjacent roles, share one
  operator, network, software supply chain, or jurisdiction;
- a Name Lease is in Grace, becomes Released, or is reclaimed into a new
  generation; a Name Record is stale, rolled back, or equivocating;
- a Name Authority is rotated, transferred, lost, or compromised; Recovery Policy
  is absent, changing, captured, or has entered Recovery Pending;
- an existing Name-origin connection observes same-Target renewal/Grace,
  Recovery Pending, Release, or rebind to a replacement Target; it must never
  continue recovery indefinitely or silently retarget, while an explicit
  Target-origin connection deliberately receives no Name rescue;
- a pending claim is copied, front-run, withheld, flooded, or ordered differently
  across a partition; a powerful actor pays the Anonymous Cost at scale;
- private name resolution is blocked or a naming participant records query value,
  repetition, popularity, timing, volume, or cache metadata;
- a project, legal claimant, registrar, or operator attempts to seize or block a
  Lease; a local filter is mistaken for canonical state;
- naming rules are captured, rolled back, or fork incompatibly, and clients must
  not silently choose another Namespace or rule version;
- a Service Descriptor is unavailable or points to no reachable Service
  Instance;
- old and new hosts concurrently publish one stale or duplicated Instance
  generation, or a stolen private Instance Key plus its public Credential races
  a higher-generation successor;
- a Service Authority is lost, corrupted, or suspected compromised; a stale
  Recovery Bundle cannot reconcile monotonic state and remains authority-locked;
- a Service goes offline before connect, during handshake, or mid-operation;
- a route fails after the Application has written some bytes;
- an endpoint changes network or address while the process survives, or instead
  suspends, reboots, crashes, or updates and loses live connection state;
- an Application reuses one Isolation Context across identities or contexts that
  should not be linked;
- either endpoint Application exposes an ordinary-network listener, or a
  malicious client response or Service request tries DNS, external fetch,
  callback/SSRF, WebSocket/WebRTC, QUIC, or direct socket egress from either
  endpoint Application. A claim-bearing boundary blocks it; a generic adapter is
  reported as lacking Application-level location privacy;
- an Application creates Services or Isolation Contexts to evade its parent
  resource budget;
- a local Application attempts to exceed connection, bandwidth, or queue limits;
- a slow reader attempts to create unbounded buffering or starve other grants;
- a censor blocks known entry addresses and protocol fingerprints;
- Release Safety expires or a build is revoked while a Route is live; Work Safety
  Lease, no-recovery, and terminal deadlines close it rather than preserving old
  trust indefinitely;
- an official endpoint or protocol update channel is compromised or unavailable,
  and private-only, direct-allowed, and offline repair modes do not silently
  substitute for one another.

For an Interactive Route candidate, any forbidden endpoint, edge-observer, or
single-Node disclosure, or any silently accepted substitution, modification,
replay, redirect, or downgrade in these cases, fails Route Qualification.
