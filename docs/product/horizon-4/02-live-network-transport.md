# H4-2 — Reachable live network and transport operation

Status: **H4-2 implemented for the functional alpha: TCP/TLS and QUIC v1 are
maintained State-selected Carrier Profiles; no camouflage/Bridge profile is
selected. Capacity, censorship-resistance, anonymity, and availability remain
separate qualification claims.**

## Why this epic is hard

H4-2 is not a library-selection task. Ardents operates over a hostile network:
an ISP, censor, malicious peer, source, proxy, or endpoint-adjacent observer
can block, reset, delay, fingerprint, probe, replay, exhaust, or selectively
degrade traffic. A second wire protocol changes discovery, profile selection,
peer authentication, resource accounting, retry exposure, update overlap,
operator deployment, observability, and the meaning of recovery.

For this reason, a transport that succeeds in a lab is not a fallback and does
not earn a censorship-resistance claim. It becomes a supported profile only
when its exact authority, failure, exposure, and requalification rules are
known.

## H4-2 decision

The product goal is **replaceable authenticated Carrier and Entry profiles**,
not an illusion that one live TCP or QUIC session can be moved unchanged onto a
different wire protocol.

When a Carrier/Route attachment fails or is blocked, Service Connection decides
whether a bounded recovery is allowed. It may then obtain a new
State-authorized Entry/Carrier profile, bind and authenticate a new attachment,
and resume only through the existing ordered Service Connection recovery
contract. The old attachment is closed. There is no transport-local direct
fallback, no Node-selected downgrade, and no claim that bytes accepted by the
old attachment were processed remotely.

The maintained profiles are native TCP carrier legs protected by mutually
authenticated TLS 1.3 under
[ADR-0024](../../adr/0024-native-interactive-route-foundation.md) and QUIC v1
under [ADR-0048](../../adr/0048-maintain-tcp-and-quic-carriers.md). H4-2 does
not restore H3 WebTunnel frames or configuration as a compatibility path.

## The Carrier seam

Route, Entry, and Service Connection need one deep **Carrier Module** with a
small Interface. Its callers ask for one authenticated, State-authorized,
resource-reserved attempt to an exact adjacent peer/profile and receive either:

- one opaque reliable ordered Carrier bound to that attempt; or
- one classified terminal attempt result.

The Carrier Interface includes its invariants: exact peer/profile and
LegBinding authentication, bounded deadline, cancellation, backpressure,
drain/close, resource accounting, and terminal cleanup. It does **not** expose
socket addresses, packet formats, proxy URLs, QUIC connection IDs, STUN/TURN
details, transport-specific retry policy, or an Application datagram surface to
Route callers.

Concrete TCP/TLS and QUIC adapters now live behind this seam. Their transport
complexity stays private, while Node and Route use the same Carrier contract.
TCP/TLS remains the reference behavior rather than a privileged fallback.

R-094's disposable two-Adapter work supplied the evidence used to promote this
seam into maintained code. The common Module, not an Adapter, owns offer
preflight, exact peer and
LegBinding authentication, terminal classification, and return of the ordered
lane. Adapter-private code owns transport dialing, TLS/QUIC state, and bounded
wire cleanup. In particular, common graceful `Close` maps to different private
operations: direct connection close for TCP and stream FIN plus connection
closure for QUIC, while failed QUIC cleanup uses reset. Transport-specific wire
budgets also remain private Profile facts: R-094 falsified quic-go's default
1280-byte initial UDP payload at link MTU 1280 and retained the candidate only
after fixing its initial payload to 1200. The maintained QUIC Adapter therefore
pins that value, disables 0-RTT and datagrams, and returns one reliable ordered
stream through the same Carrier Interface. A later three-namespace slice showed
that the same experimental QUIC attachment survived a same-IP NAT source-port
change without exposing migration through the common seam. A subsequent
separate-host public-path slice passed the exact TCP/TLS and QUIC transcript and
cleanup oracle across local Docker Desktop and the project-operated Ubuntu
Docker host. A route-only dual-homed follow-up changed the reported Linux route
after `Open`, but its required B-path ingress counter remained zero and both
ends timed out cleanly. It therefore proves neither migration success nor
failure and gives no reason to expose migration through the common seam.
Genuine source/address or access-path change remains a possible Adapter-private
future research question, not a prerequisite or claim of the maintained alpha
profile. The local 1-vCPU/1-GiB Docker campaign is functional evidence only;
the observed UDP receive-buffer warning prevents a throughput/capacity claim.

Entry/camouflage adapters are a different class. They operate only at a bounded
endpoint-adjacent Entry seam and must not be treated as a generic replacement
for all inter-Node Carrier legs. Their source acquisition, bridge distribution,
active-probe resistance, and Direct Source Exposure consequences are explicit
profile inputs.

## Profile authority and replacement rules

Every selectable profile must be authenticated Network State/publication
capability, never a suggestion from a Node, source, or transport Adapter. The
profile fixes at least its generation, peer identity/binding expectations,
endpoint reachability authority, permitted role/domain, expiry, resource
envelope, and replacement/retry rules. An unavailable, stale, incompatible,
unsupported, or lower profile is an explicit outcome.

Profile selection must also be finite:

1. The Endpoint receives a bounded candidate/profile set from authenticated
   authority and local policy.
2. It selects only precommitted attempts permitted for the current Entry regime
   and resource reservation.
3. Every direct source, bridge, broker, proxy, or auxiliary contact updates the
   applicable exposure state before it can be reused in Route or Resolution.
4. Failure returns to Service Connection/Endpoint policy; an Adapter neither
   loops through arbitrary alternatives nor silently contacts a clearnet path.
5. Recovery either reaches the exact authenticated Service Connection contract
   or closes with a classified result. It never fabricates delivery.

Cross-profile racing, fallback, and retry are therefore security decisions, not
latency optimizations. They require their own evidence before being enabled.

## Maintained and deferred transport families

R-094 evaluated these families. TCP/TLS and QUIC v1 are maintained selections;
the remaining rows are explicitly deferred or rejected for the functional
alpha.

| Family | Potential role | Why it is interesting | Why it is not free |
|---|---|---|---|
| TCP + mutually authenticated TLS 1.3 | Maintained baseline Carrier | Mature, predictable stream semantics; selected native v1 contract. | Commonly blockable/fingerprintable; no independent alternate path. |
| QUIC v1 | Maintained second Carrier | TLS 1.3-integrated secure multiplexing and one reliable bidirectional stream without changing the Application Interface. | UDP may be blocked; no path-diversity, capacity, or censorship-resistance claim follows. |
| obfs4-like pluggable transport | Entry-only camouflage candidate | Separates traffic-obfuscation and Bridge concerns from the internal Route. | Active-probe, Bridge distribution, deployment, maintenance, and source-exposure problems remain; it is not a generic Carrier. |
| HTTP CONNECT / HTTP/2 / HTTP/3 / WebSocket-style tunnel | Narrow Entry/proxy candidate | May traverse specific constrained networks. | Requires a proxy/fronting/operator model, has identifiable behavior and abuse cost, and cannot be assumed to be a safe generic fallback. |
| MASQUE CONNECT-UDP / CONNECT-IP | Proxy-assisted Entry research candidate | Standardized mechanisms for proxying UDP/IP over authenticated HTTP. | Introduces an IP/UDP proxy, destination policy, source exposure, abuse, and proxy-operator dependence; not a replacement for the Route protocol. |
| WebRTC / Snowflake-like brokered entry | Later blocked-entry candidate | May reach users behind restrictive NAT/censorship conditions. | STUN/TURN/broker contacts, endpoint exposure, availability, operator work, and browser/runtime dependencies are substantial. |

QUIC is selected because it exercises a genuine second carrier implementation
while preserving the reliable-stream Interface. It is not mandatory: a hostile
network may block UDP while permitting TCP, and that failed attempt does not
authorize TCP. HTTP/3 is an HTTP mapping over QUIC, not an Ardents Carrier.
[RFC 9000](https://www.rfc-editor.org/rfc/rfc9000),
[RFC 9114](https://www.rfc-editor.org/rfc/rfc9114)

MASQUE and WebRTC/Snowflake-like approaches are deliberately later and
entry-only candidates. CONNECT-UDP and CONNECT-IP are proxy protocols, so they
add proxy trust, abuse and destination-policy questions rather than removing
them. [RFC 9298](https://www.rfc-editor.org/rfc/rfc9298),
[RFC 9484](https://www.rfc-editor.org/rfc/rfc9484)

## H4-2 delivery slices

### Current alpha topology selection

H4-2A first implemented Rendezvous over TCP/TLS 1.3. H4-2B now applies the same
admission, reservation, pressure, drain, withdrawal, and cleanup ownership to
the State-selected TCP/TLS or QUIC-v1 Carrier.

After the required duties exist, the first complete functional-alpha tracer
may place their separate processes on one project-operated VPS. Every process
has its own Node Identity, state root, Role Domain Assignment, listener,
resource ceiling, and terminal lifecycle. This is explicitly one host and one
operator: it proves neither independent capacity, resistance to correlated
control, host-loss availability, nor the five-distinct-operator privacy
condition. Physical-host/provider loss and recovery remain later qualification
claims; controlled multi-host and local full-system fault evidence is recorded
below.

A single combined relay process that performs conflicting Role Domains is not
this topology and remains outside the selected architecture.

The functional operating profile is deliberately narrow: one unprivileged
Rendezvous process, one Node Identity and assignment, one State-authorized
TCP/TLS-or-QUIC listener, and three finite reservations for handshakes, unmatched
authenticated legs, and active two-leg pairs. Handshakes remain identity-free
until authenticated LegBinding. Full pair capacity and `PROTECT` both refuse new
expensive work and close other pre-admission work, but only `PROTECT` is a
pressure transition; admitted pairs survive either condition. `DRAIN` closes
admission and joins every owned connection and worker inside the Work Safety
Lease. Exact capacities, timeouts, queues, cgroup limits, and hysteresis come
only from R-092's NET-01A campaign. No H3 probe number is inherited.

R-092's disposable Rendezvous tracer now gives this shape concrete feasibility
evidence without pretending to be `ardents-node`. A five-case local matrix
passed exact bidirectional pairing, duplicate-side rejection, unmatched expiry,
pre-TLS refusal while the sole pair slot was occupied, and joined drain. A
Linux race exact-pair cell and a separate-host run with two isolated local
client containers plus one resource-limited remote server also passed with
zero final reservations or connections. This selects the contract to
implement; it does not select numeric limits or satisfy State, assignment,
resource-pressure, abuse, or NET-01A qualification.

### H4-2A — One measured live TCP/TLS profile

Implement and measure the selected Rendezvous duty before composing the full
route. Then run the selected v1 Carrier with separate remote endpoints and Node
roles. Declare the exact host class, State/Entry source, topology, resource
ceilings, network links, release/profile generation, and expected
readiness/failure states. This is the first usable-alpha network foundation; it
does not wait for QUIC or censorship camouflage.

The available local/remote Docker pair is now a validated functional and
inter-host-fault fixture for this slice. It cannot replace the selected
2-vCPU/2-GiB reference-host campaign merely by applying container limits on a
stronger host.

The product `ardents-node` command has both a four-duty readiness cell and one
local functional topology cell. Separate Initiator, Introduction, Rendezvous,
and Responder processes accept their own materialization of one signed
native-profile Epoch and reach only their exact State assignment; a second
test substitutes those four product commands for the transit fixtures in one
Publisher-to-User C-2 journey. This closes the local command/fixture gap for
startup and one route. Its Linux Docker execution sends `SIGTERM` after that
journey and requires `DRAINING` then `WITHDRAWN` from every Node command. It
also has a linked signed State-successor cell that withdraws every Node, an
offline product-transit cell that returns `service unavailable` without a
Reference URL or direct-site fallback, and a Linux Rendezvous cell that drains
one held authenticated pair on `SIGTERM`. A separate full-C-2 cell holds an
established Publisher Application-to-Service path through the product Nodes,
refreshes both State Sources, observes every Node's `DRAINING` then
`WITHDRAWN`, and lets both held endpoints reach their classified terminal
outcomes. It does not qualify hostile failure handling or the NET-01A capacity
profile; the Windows compatibility harness retains forced test cleanup. These
local cells are supplemented by the selected local Docker full-system emulator:
it cross-builds exact Linux product and fixture bytes outside the repository,
mounts only those bytes read-only into one resource-bounded `--network none`
container, starts the complete held-route C-2 composition, waits until the
Publisher Application and User have both acknowledged setup, then hard-stops
the real product Rendezvous command. Both endpoint sides must finish with their
existing classified terminal outcomes. This closes the selected H4-2
full-system composition and simulated fault-domain-loss gate; it does not claim
a physical host/provider outage, public-path recovery, capacity, availability,
or independent operation.

`TestRendezvousNodeProcessBoundsIncompleteTLSHandshakes` adds one bounded
product-command abuse/error cell: three clients send an incomplete TLS record;
the State-selected handshake limit of two holds exactly two and refuses the
third, then a normal authenticated pair works after those clients release
their slots. Every native-duty plan now also requires one finite local
`admission_timeout_ms`; it bounds TLS plus the exact binding before the duty
allocates a waiting slot, relay, or delivery. The effective deadline is the
earlier of that value and current State expiry, and successful work returns to
its authenticated binding, slot, or State work deadline. The value is not a
selected capacity profile or a hidden default.
`TestRendezvousNodeProcessExpiresIncompleteTLSAdmission`
proves that the product command releases an incomplete TLS reservation by that
configured deadline without a client close, then carries a new authenticated
pair. `TestRendezvousNodeProcessKeepsActivePairPastAdmissionDeadline` verifies
the complementary boundary: after TLS and exact binding admission, the active
pair changes to its authenticated `LegBinding.NotAfter` deadline rather than
retaining the short admission deadline. This establishes bounded
pre-admission work and recovery from that particular local resource condition;
`TestIntroductionSlotExpiresAtItsRegistrationDeadline` independently verifies
that an admitted live slot closes at its authorized registration deadline.
These cells do not establish DoS resilience, the right release value for every
access path, hostile-network qualification, or a capacity profile.

The separate `make qualification-h4-2-multihost` result now closes narrow
multi-host product-Rendezvous byte-carriage and remote-Node-loss cells. On
2026-08-26 it cross-built the current
`ardents` and `ardents-node` command bytes on the Windows development host and
ran one temporary host-network Docker container on the project VPS. The
container held the State-authorized public Rendezvous listener on TCP 47926 and
two authenticated State Sources bound only to VPS loopback. Direct native
Initiator and Responder legs from the Windows host completed exact byte
carriage in 11.58 seconds, while an Initiator-certificate leg claiming the
Responder role was refused. A second cell first carried bytes through an active pair, abruptly
stopped the remote Node/container, and required both TLS legs to reach a
terminal read error rather than a timeout; it passed in 15.11 seconds. Each
run verified deletion of its exact container and temporary remote directory.
Its verbose record fixes profile
`ardents-interactive-route-v1`, State epoch 1 and digest
`d22bfef964b59bd327e7d5c53b6a80840ffa2397ba76e0e1b2244b2e31cf3971`,
the exact `ardents` SHA-256
`8b55bb78de84e16f3a4043e36b22f309b1a4e019d7e9d5d217698db1ca8fa9f7`, and
the exact `ardents-node` SHA-256
`c9f7f714e1897fe0be3c5a97a7d85e2b8a028bbd091864e4d410829ef62ebd9e`.
The remote envelope was Docker 29.4.1, Linux 5.15.0-185-generic x86_64, 4
vCPU, 8,109,136 KiB reported memory, and `golang:1.26.6` image ID
`sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6`.
On the same controlled two-host path, the test-only
`TestH42MultiHostRendezvousTCPFaultRelay` placed a transparent local TCP relay
before the real remote Rendezvous. It passed in 13.32 seconds: a 200-ms delay
in each direction retained exact carriage; a local RST made both active legs
close terminally and a fresh pair then carried bytes through the unchanged
remote Node; and a deliberate byte blackhole waited for the caller's explicit
one-second read budget. This is a test fixture, not a Node, Route, transport,
or recovery mechanism.
`TestH42MultiHostRendezvousKernelNetemRelay` then used three disposable remote
Docker bridge-network sidecars between those Windows legs and the same product
Rendezvous. Each sidecar had only `NET_ADMIN`; its static relay received only
its own binary, while the VPS `tc` and libraries were read-only mounts. Its
qdisc applied only to the sidecar's `eth0`, never to the host. The 200-ms delay
cell retained exact carriage, and the 100%-loss cell exposed no authenticated
attachment before the caller deadline while `tc -s` recorded a nonzero drop
counter. A third cell fixed 20 ms ±5 ms delay, 5% loss, and 10% reorder, then
carried one exact 256 KiB transcript while `tc -s` retained that configuration
and reported nonzero requeues. Random 5% loss is configured in that cell, but
an individual successful transcript does not establish that it encountered a
loss event; the separate 100%-loss cell is the measured loss outcome. This is
kernel-netem evidence inside
one controlled container network, not a claim about public-path loss, MTU,
NAT, probing, host loss, recovery, or availability.
This proves one controlled public two-host TCP/TLS product-Node path and
immediate terminal closure after remote Node/container loss, plus the three
test-owned TCP fault outcomes and three container-namespace netem outcomes above.
It is not a full C-2 multi-host workflow, true VPS/host-loss recovery or
availability result, independent-operator result, hostile-network result, or a
2-vCPU/2-GiB capacity qualification.

### H4-2A active evidence matrix

The current TCP/TLS baseline has deliberately narrow, reproducible cells. A
passing cell establishes only the outcome in its row; it does not promote the
unmet column to a product property.

| Condition | Current evidence | Established outcome | Still required / excluded claim |
|---|---|---|---|
| State-bound peer authentication | local and two-host `TestH42MultiHostRendezvousQualification` | exact authorized Initiator/Responder legs carry bytes; wrong-role identity is refused | full multi-host C-2 workflow, operator independence |
| Incomplete TLS at admission | local `TestRendezvousNodeProcessBoundsIncompleteTLSHandshakes` and `TestRendezvousNodeProcessExpiresIncompleteTLSAdmission` | the configured two reservations are held, one excess connection is refused, and a finite explicit timeout releases a held slot before normal work resumes | a release value for every access path, DoS resilience, capacity |
| Planned process withdrawal | local Linux `TestRendezvousNodeProcessDrainsActivePairOnSIGTERM` and full-C-2 State-successor tests | active work reaches terminal closure through `DRAINING → WITHDRAWN` | hostile failure, remote recovery, availability |
| Abrupt remote Node/container loss | two-host `TestH42MultiHostRendezvousAbruptRemoteNodeLoss` | an active pair's TLS legs terminally close rather than silently timing out | VPS/host loss, reconnect/fallback, service availability |
| Test-owned TCP delay, RST and blackhole | two-host `TestH42MultiHostRendezvousTCPFaultRelay` through a transparent local relay | 200-ms per-direction delay retains exact bytes; RST ends the pair and a fresh pair succeeds; byte blackhole is bounded by the explicit caller read budget | no link-level packet loss/reordering, MTU, NAT, active probing, network recovery, retry, drain, resource, or cleanup claim |
| Isolated kernel netem delay, loss and reorder | two-host `TestH42MultiHostRendezvousKernelNetemRelay` through disposable VPS Docker sidecars | 200-ms qdisc delay retains exact bytes; 100% qdisc loss yields no attachment before the caller deadline and records nonzero drops; fixed 20ms ±5ms / 5% loss / 10% reorder carries one exact 256 KiB transcript while retaining configured qdisc state and nonzero requeues | no public-path loss, MTU, NAT, active probing, third-host loss, recovery, retry, drain, resource, or availability claim |
| Loss of the selected simulated C-2 fault domain | local Docker `TestReferenceC2HardStopsRendezvousWithHeldRoute` | after independent Publisher and User setup acknowledgements, the actual product Rendezvous process is hard-stopped and the held route reaches its classified terminal outcome | physical host/provider outage, public-path recovery, capacity, availability, and independent operation |
| Numeric pressure/capacity | NET-01A preflight only | known 4-vCPU/~8-GiB VPS is explicitly ineligible as reference evidence | the exact Ubuntu 2-vCPU/2-GiB host campaign, raw measurements and explicit selection or refusal |

### H4-2B — Maintained Carrier seam with QUIC

Specify and test one QUIC v1 Adapter against the same peer-binding, resource,
drain, recovery, and workload rules as the TCP/TLS baseline. The first version
offers only the reliable ordered Carrier Interface. QUIC datagrams, HTTP/3, and
Application-visible unreliable data are out of scope unless a later product
claim selects them. [RFC 9221](https://www.rfc-editor.org/rfc/rfc9221)

This slice is complete. `internal/route` maintains two exact adapters behind
one `Carrier`: TCP/TLS and QUIC v1. Both authenticate TLS 1.3 peer state and the
same reciprocal `LegBinding`; QUIC exposes one ordered bidirectional stream,
uses an initial packet size of 1200, and disables 0-RTT and datagrams. The
Rendezvous listener takes its finite admission reservation before either
adapter completes authentication.

Every failed attempt and authenticated Carrier I/O failure carries one stable
transport-neutral class: `stale`, `incompatible`, `unauthorized`, `canceled`,
`timeout`, `unavailable`, or `closed`. The underlying error remains available
only inside the Carrier Module and cannot become transport selection policy.
QUIC sends bounded
keepalive traffic so a healthy idle Carrier outlives its negotiated five-second
idle window; a failed post-open authentication uses reset/abort, while normal
close uses FIN and a success close code.

Signed Network State selects the adapter. Node Record v1 remains canonical
TCP/TLS compatibility input; Node Record v2 signs one explicit profile. Unknown
profiles are deterministically rejected. Rendezvous uses its own selected
profile, while Initiator and Responder use the selected Rendezvous candidate's
profile. `OpenNodeLeg` makes exactly one attempt and contains no retry order:
tests prove both QUIC→TCP and TCP→QUIC implicit fallback are absent.

The maintained full C-2 behavior test carries the same Reference Site from
Publisher to User through separate Initiator, Rendezvous, and Responder duties
over each profile. The 1-vCPU/1-GiB, external-network-free Docker campaign runs
that product path, signed-profile acceptance/rejection, exact binding, early
admission reservation, both no-fallback directions, and the existing hard-stop
route outcome from cross-built Linux bytes.

Before promotion, TCP/TLS and the refined QUIC v1 candidate also passed the
common 15-case local oracle
and three repetitions of a 10-client/10-server Linux namespace matrix. The
matrix demonstrated protocol-selective TCP/UDP refusal without implicit
fallback, a separately authorized control attempt, nonzero 5% loss plus
delay/reorder under an exact-or-bounded-failure rule, MTU 1280, and bounded
client FD/goroutine cleanup. Three repetitions of a second two-bridge,
three-namespace matrix demonstrated exact transcript delivery after a
same-IP/different-source-port relay transition following Carrier `Open`, with
no new Adapter call or attachment. A later public-path slice used separate
Docker kernels/hosts and the same exact binary to deliver the 256 KiB oracle
over both TCP/TLS and QUIC without fallback, FD/goroutine growth, container
residue, or port residue. Both QUIC endpoints retained the measured UDP
receive-buffer warning. A checked dual-homed route-only follow-up did not
exercise its intended B path, so it establishes no migration result. Explicit
quic-go path creation/probing/switching remains Adapter-private future research
and is not part of the common Carrier Interface or the functional-alpha claim.

### H4-2C — Bounded blocked-entry profile

This slice is closed by the explicit negative selection in
[ADR-0049](../../adr/0049-defer-blocked-entry-profile.md). The functional alpha
has no declared censorship condition, Bridge distributor, broker/proxy role, or
operations model that would justify an obfs4-like, WebTunnel, MASQUE, or
WebRTC/Snowflake-like profile. Implementing one would create hidden
infrastructure and exposure semantics rather than a transport Adapter.

An unavailable Carrier therefore fails explicitly. No Bridge, proxy, broker,
fronting service, direct Internet path, or H3 compatibility mechanism is tried.
A future blocked-entry profile starts with one bounded research question and a
new ADR; it is not unfinished alpha code. Tor's distinction between unlisted
Bridges and pluggable transports remains useful evidence, not an Ardents design
selection. [Tor introduction](https://spec.torproject.org/intro/)

### H4-2D — Profile rollout and hostile-network qualification

This slice is implemented for the two maintained profiles:

- Node Record v1 is the legacy signed schema and means TCP/TLS; v2 signs the
  exact profile after its endpoint. State accepts only the release-supported
  TCP/TLS and QUIC identifiers and rejects an unknown value before assignment.
- State's narrow Node-duty view projects the local and candidate profiles.
  Node plans cannot substitute a profile and Route opens only the supplied one.
- A linked State successor changes the authenticated Epoch/duty. The running
  Node drains and withdraws instead of mutating an admitted attachment. H4-6's
  service lifecycle may start the successor duty; Service Connection owns any
  permitted logical recovery.
- Lifecycle/resource events and terminal results include the effective Carrier
  Profile. This makes a selected profile observable without exposing sockets,
  QUIC identifiers, or a retry list.
- A release supports exactly these two identifiers. There is no overlap race,
  alternate-profile loop, or implicit downgrade. A future profile requires a
  research record, accepted ADR, dependency review, and repetition of the
  common behavior and Docker qualification.

## Required evidence

Each candidate profile declares and measures:

- exact State/publication authority, peer identity, bindings, expiry, and
  generation/downgrade behavior;
- all contacted sources, Bridges, brokers, proxies, STUN/TURN services, and the
  resulting exposure/exclusion effects;
- TCP/UDP block, reset, drop, delay, reorder, MTU, NAT, active-probe, and
  profile-specific failure injections;
- connection setup, recovery, drain, closure, memory, CPU, file descriptor,
  queue, traffic, retry, and cleanup bounds;
- operator installation, port/listener, update, withdrawal, and abuse controls;
- the exact comparison against TCP/TLS under the same workload and resource
  class.

No candidate may hide a new external infrastructure dependency in an Adapter.
If a broker, proxy, Bridge distributor, fronting service, or cloud vendor is
required, it is a named source/operator role with its own failure and exposure
contract.

## Non-goals

- A project-operated topology is not independent public capacity.
- Transport diversity is not by itself censorship resistance, anonymity, or
  availability.
- No direct fallback, generic proxy, VPN, clearnet exit, system-wide DNS/route
  mutation, Node-selected version choice, or H3 compatibility reader.
- No universal transport Interface is exposed to Applications; Ardents remains
  a reliable Service Connection carrier until a distinct product contract says
  otherwise.

## Stop conditions

Stop or narrow a candidate when it requires an unbounded retry/source-contact
loop, exposes a User to a source that can later enter a forbidden role, loses
exact peer/profile authentication, bypasses resource/drain limits, forces a
global proxy/broker dependency, or cannot be maintained and qualified by the
actual project capacity.
