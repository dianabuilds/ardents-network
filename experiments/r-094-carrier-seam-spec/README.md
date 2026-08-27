# R-094 — Two-adapter Carrier seam experiment

## Question

Can TCP/TLS and QUIC v1 supply one *authenticated ordered-carrier result* to a
Route/Entry experiment without giving a Node transport choice, silently falling
back, or changing the outcome of cancellation, drain, and recovery?

## Hypothesis

Two transport-private Adapters can implement one small Carrier Module Interface
when the Module, rather than either Adapter, owns offer validation, exact peer
authentication, LegBinding confirmation, error classification, and return of an
authenticated ordered byte lane. The hypothesis is falsified if either Adapter
needs a transport-specific success result, performs an implicit fallback, leaks
work after its deadline, or cannot distinguish graceful close from failed
attachment cleanup.

## Preconditions

- This is not maintained code and does not change the TCP/TLS v1 selection.
- R-094's 2026-08-24 dependency review accepts the exact pinned
  `quic-go v0.61.0` checksum solely for this build-ignored experiment. The module
  remains indirect because no maintained runtime import is selected.
- Two process identities, certificate chains, and a State-authorized profile
  offer are generated only for the experiment. No Ardents Authority, real Entry
  Invite, public endpoint, proxy, bridge, STUN/TURN service, or legacy H3 input
  is used.
- The experiment runs the same topology and workload for both candidates. A
  failure to build or bind the same proof on QUIC is a negative result, not a
  reason to weaken the TCP/TLS oracle.

## Implemented local slice

All Go files have `//go:build ignore`; the experiment is excluded from the root
module's maintained package graph. The private Adapter seam returns an ordered
deadline-capable lane, TLS connection state, and cleanup operation to the common
Module. Only the Module verifies the selected profile, exact Ed25519 peer,
TLS 1.3/ALPN state, and reciprocal `route.LegBinding`. Its external result
contains byte-lane operations and one transport-neutral terminal result class;
it exposes no `net.Conn`, QUIC type, endpoint resolver, fallback handle,
datagram, or migration promise.

The local executable covers both profiles with happy path, wrong peer, wrong
binding, stalled handshake, stopped-reader backpressure, and loss after
authenticated binding. It also proves that expired and unknown profiles are
rejected before dial and that recovery is a distinct State-shaped attempt with
a fresh attachment identifier.

The fault executable adds separate deterministic-identity server and client
processes. The checked runner places them in separate Docker network namespaces,
uses `tc` only inside `NET_ADMIN` lab containers, exchanges a deterministic
256 KiB transcript, and covers baseline, protocol-selective refusal,
delay/loss/reorder, and MTU 1280. A second checked runner uses three namespaces
and an opaque two-sided UDP relay to cover same-IP NAT port rebinding after the
QUIC Carrier is authenticated. A later dual-homed runner attempted a
post-`Open` route-only source/interface change. Its stable-path control passed,
but the changed-route case sent no observed packet over the required B path, so
it failed the predeclared path proof and establishes neither migration success
nor migration failure. Physical access-path change remains untested.

## Candidate contract (not a maintained Go interface)

One `CarrierAttempt` is created by the experiment's **Service Connection**
owner. It contains only:

- an authenticated State-derived offer: profile identifier, peer identity,
  endpoint, expiry, and attempt identifier;
- an expected Route/Leg binding and the exact peer-authentication rule;
- deadline and finite resource budget; and
- an explicit profile selected by the authenticated offer.

A TCP/TLS or QUIC adapter either returns one `AuthenticatedCarrier` result or
one classified failure. The result is exactly one ordered, reliable,
bidirectional byte lane plus authenticated peer identity and a binding proof.
Neither adapter returns a resolver, Node-selected fallback, unbounded retry
handle, datagram lane, browser proxy, or migration promise. QUIC uses one
bidirectional stream only, with application datagrams and 0-RTT disabled.

The common acceptance oracle is:

1. the offer has not expired and names the selected profile;
2. the authenticated remote identity equals the offer;
3. the Route/Leg binding verifies from the carrier's authenticated TLS state;
4. the returned byte lane delivers an ordered, exact test transcript within its
   budget; and
5. cancellation, close, drain, and resource exhaustion terminate the attempt
   once and leave no live listener, goroutine, stream, or retry outside the
   Service Connection owner.

Any failed condition yields a classified failed attachment. A subsequent
attachment is a new State-authorized attempt; the experiment must never claim
that a TCP connection or a QUIC stream continued across transports.

## Procedure and falsification matrix

Record exact OS/kernel, Go toolchain, candidate module version and checksum,
topology, test certificate fingerprints, offer bytes/digest, all configured
budgets, packet-fault rules, result class, elapsed time, byte counts, peak
goroutines, open descriptors/sockets, and cleanup result. Values for deadlines
and ceilings must be checked into the harness before a run; changing them
invalidates comparison with the other adapter.

| Case | Injection | Required result / falsifier |
|---|---|---|
| Baseline | No fault; exact fixed transcript and bounded concurrent writers | Both candidates return the same authenticated transcript. Any difference in identity/binding or duplicate/lost bytes fails the candidate. |
| Peer rejection | Wrong certificate, wrong expected identity, or wrong binding | No byte lane is exposed. Acceptance by either candidate fails it. |
| Profile authority | Offer expired, profile absent, lower/unknown profile supplied by a simulated Node | Attempt is rejected before transport dialing. A Node influence or an implicit alternative profile fails the seam. |
| Deadline and cancel | Stall handshake, stall reads, cancel at each phase | Exactly one classified failure/close by deadline; no retry outside Service Connection. Hang, leaked work, or post-cancel traffic fails it. |
| Backpressure and drain | Reader stops below the declared budget; request normal drain | Bounded buffering and deterministic close/drain. Unbounded memory/work or a stranded listener fails it. |
| UDP refusal | Firewall/drop all UDP to the QUIC endpoint while TCP remains available | QUIC reports its own bounded failure. It must not dial TCP unless a *new* State-authorized TCP attempt is created. |
| TCP refusal | Block/reset TCP while UDP remains available | TCP reports bounded failure; QUIC can succeed only from its own authorized attempt. |
| Loss and MTU | Linux network namespace or equivalent controlled loss/reorder and reduced MTU | A candidate either preserves the exact ordered transcript within its declared budget or classifies failure/cleanup. A partial successful transcript or an unbounded stall fails it. |
| Path change | Change the client source path during an active QUIC run, then separately force attachment loss | Record whether QUIC validates a path; in both outcomes, route-level recovery must be a new authorized attachment. Treating path migration as a cross-profile continuation fails it. |
| Attachment loss | Kill/close the remote adapter after binding | Service Connection creates at most its declared new authorized attempt. No duplicate application effect or uncontrolled retry is permitted. |

### NAT-rebinding slice — oracle

Use three processes in three namespaces and two isolated Docker bridges: the
client can reach only an opaque UDP relay, the server can reach only the relay,
and the relay forwards encrypted QUIC datagrams without parsing them. First run
an unchanged-port relay baseline. In the rebinding case, after `Open` has
completed TLS and LegBinding, a separate lab control message makes the relay
replace its connected upstream UDP socket. The client-facing address remains
stable while the server observes the same relay IP with a different source
port.

The success oracle requires one unchanged QUIC Adapter call and attachment
identifier, different old/new relay source ports, packets forwarded after the
change, the exact 256 KiB transcript proof, joined cleanup, and no new Carrier
attempt. A classified timeout with the same cleanup would reject transparent
rebinding for this candidate but would not weaken the rule that subsequent
Route recovery requires a new State-authorized attachment. A relay that does
not actually change the server-observed tuple, a partial transcript reported as
success, or any exported migration control falsifies the slice.

This emulates NAT port rebinding only. It is not evidence for a changed client
interface/IP, a new access network, physical path diversity, or cross-profile
continuation.

### Dual-homed source-address/interface slice — oracle

This later slice asks whether the already-open experimental QUIC attachment
survives an actual Linux source-IP and egress-interface change without a new
Carrier call or an exported migration control. It does not use quic-go's public
client `AddPath` API. Exact v0.61.0 source inspection shows that `DialAddr`
creates an IPv4-wildcard UDP socket, while a server processes a new remote tuple
through path validation and changes its active remote address only after the
new path is accepted.

Use two isolated Docker bridges. One server and one client process are each
dual-homed, with distinct A and B addresses. The server binds its fixed QUIC
endpoint specifically to server-A. The client initially reaches that exact
endpoint directly from client-A. After Carrier `Open` has completed mutual TLS
and LegBinding, the client pauses at a lab-only control point. The runner then
replaces only the kernel route for server-A so packets leave the client's B
interface with client-B as source and traverse server-B as the next hop; the
server endpoint address, profile, Adapter call, attachment identifier, and
Application transcript remain unchanged.

Run an unchanged dual-homed baseline first. For the migration case, record the
four addresses, interface mapping, route before and after, an initially zero
server-B ingress filter counter and a nonzero post-change counter for UDP from
client-B to server-A:19443, the server's QUIC remote address before/after path
validation, exact transcript result, Adapter-call count, descriptors,
goroutines, and cleanup. Both containers are read-only, resource-limited, and
capability-free at runtime except for lab-only `NET_ADMIN` route/counter
operations; the client process itself remains an unprivileged UID.

The baseline must succeed. The migration cell may produce either the exact
256 KiB success transcript or the predeclared bounded timeout, but in both
outcomes the route and ingress counter must prove that packets actually used
client-B/server-B, the client must make exactly `tcp=0,quic=1`, and every owned
process/socket/container/network must terminate. On success, the server remote
IP must change from client-A to client-B while its port may remain stable. A
second Adapter call, fallback, changed endpoint or attachment, partial success,
no packets on B, unclassified failure, deadline overrun, or residue falsifies
the slice.

This is a real source-address and Linux-interface change inside two namespaces,
but both bridges, namespaces, and interfaces still share one Docker Desktop
host, WSL2 kernel, and operator. It is not a new physical access network,
physical-path diversity, mobile handoff, censorship resistance, or supported
host evidence. A success retains automatic same-profile path behavior as an
Adapter-private fact; a timeout retains Route recovery as a fresh authorized
attachment rather than adding migration to the common Carrier Interface.

### Two-host WAN slice — oracle

Run the same deterministic fault server in a resource-limited container on the
declared remote Ubuntu Docker host and the fault client in a separate container
on Docker Desktop. The containers share no filesystem, Docker network, kernel,
or private bridge. Publish one previously unused high TCP port for the TCP/TLS
case and the same-numbered UDP port for the QUIC case; do not use host networking
or alter the host firewall. Each case gets a fresh token and deadline.

Before execution, record both Docker engine/kernel identities, the exact binary
and image digests, remote port-listener/container absence, CPU/memory/PID limits,
and the public endpoint. Both profiles must independently complete the exact
256 KiB authenticated transcript, call only their selected Adapter, return to
their baseline client FD/goroutine counts, and produce matching successful
server outcomes. A partial transcript, identity/binding mismatch, fallback,
deadline overrun, leftover container/listener, or interference with an existing
remote workload falsifies the slice. A closed provider firewall is an
environment result, not permission to weaken the oracle or reuse ports 80/443.

This slice can establish separate-host and real public-path behavior for the
named endpoints. Both hosts remain under one operator, and the run does not
establish independent operation, path diversity, censorship resistance,
active-probe resistance, anonymity, or a production capacity.

## Environment limits

The Docker Desktop fault run has two real Linux network namespaces and
demonstrable per-namespace link controls. The rebinding run has three namespace
processes and two isolated Docker bridges; the client and server have no common
bridge, and the relay alone joins both. All still share one WSL2 kernel, one
physical host, and one operator. They can validate the named fault, lifecycle,
and synthetic same-IP NAT-port change; they cannot validate independent hosts,
access networks, source-address/interface migration, physical-path change,
censorship, anonymity, scale, or independent operation.

The pinned tooling image is a current-machine lab prerequisite with an inspected
package lock and immutable image digest; its builder is not present on `main`.
Therefore the checked script reproduces this captured lab on the current host,
not a portable test environment. A maintained/current-repository tooling build
or an independently provisioned Linux lab remains necessary before release
qualification.

## Run

From the repository root, name every build-ignored file explicitly:

```text
go run experiments/r-094-carrier-seam-spec/main.go experiments/r-094-carrier-seam-spec/contract.go experiments/r-094-carrier-seam-spec/identity.go experiments/r-094-carrier-seam-spec/peer.go experiments/r-094-carrier-seam-spec/tcp_adapter.go experiments/r-094-carrier-seam-spec/quic_adapter.go
```

The program emits one `ardents-r094-environment-v1` JSON record and fifteen
`ardents-r094-carrier-case-v1` records, exits zero only when every record passes,
and retains no keys, certificates, listeners, or build artifact.

The controlled Linux namespace matrix requires PowerShell 7, a running Docker
engine, and the exact already-local tooling image pinned in the script. It does
not pull an image or install a host package:

```text
pwsh -NoProfile -File experiments/r-094-carrier-seam-spec/run-fault-lab.ps1
```

The runner builds the fault binary with `-trimpath` outside the repository,
verifies the tooling image digest, labels every temporary container/network,
and removes only those exact resources plus its temporary binary in `finally`.
It requires 10 passing client outcomes and 10 passing server outcomes. For the
loss row, success with the exact proof or a bounded classified timeout is the
predeclared oracle; an unclassified/partial result or failed cleanup is not.

The synthetic NAT-port-rebinding runner requires PowerShell 7, a running Docker
engine, and the exact already-local Ubuntu image pinned in the script. It does
not pull an image or mutate host networking:

```text
pwsh -NoProfile -File experiments/r-094-carrier-seam-spec/run-rebinding-lab.ps1
```

It builds outside the repository, creates two labeled isolated bridges, and
removes its three per-case containers, bridges, and binary in `finally`. It
requires two client, two server, and two relay outcomes: an unchanged-port
baseline and one same-IP/different-port transition after Carrier `Open`.

The dual-homed source-address/interface runner uses the pinned R-094 tooling
image because Linux `ip` and `tc` are needed only for lab route and ingress-
counter control:

```text
pwsh -NoProfile -File experiments/r-094-carrier-seam-spec/run-path-migration-lab.ps1
```

It builds outside the repository, creates two labeled isolated bridges, starts
dual-homed server/client containers with bounded resources, runs a stable-path
control followed by one post-`Open` route switch, and removes only its exact
containers, bridges, and binary in `finally`. The command currently exits
nonzero for the migration cell because the required B-path counter remains
zero; this captured falsifier is the result, not a passing qualification run.

## Captured evidence — 2026-08-24

- Five sequential Windows/amd64 runs of the pre-MTU-refinement seam under Go
  1.26.6 and `quic-go v0.61.0` each passed 15/15 cases. TCP backpressure stopped
  after 79,768 bytes and QUIC
  after 32,768 bytes in every run; the slowest case was QUIC backpressure at
  1,639–1,640 ms. These byte counts are properties of this local budget and are
  not throughput results.
- One cross-compiled Linux/amd64 run in local Ubuntu 24.04 WSL passed 15/15
  cases. TCP stopped after 65,536 bytes, QUIC after 32,768 bytes, and the
  slowest case was TCP backpressure at 1,905 ms. The disposable binary SHA-256
  was `8726d213a20429e8ed547d6f87f3ad8996bf76057080807b0895b572a7b5393a`.
- The Linux run repeated quic-go's warning that the UDP receive buffer grew
  only from 208 KiB to 416 KiB rather than the requested 7 MiB. No throughput
  or host-capacity conclusion is drawn from that environment.
- The first implementation failed the QUIC happy path because common cleanup
  mapped `Close` to `CancelWrite` plus connection abort, resetting a transcript
  that TCP closed gracefully. A later run also showed that closing a TLS
  wrapper after a timed-out TCP write could consume the outer five-second test
  deadline. The passing design keeps both facts private: QUIC uses stream FIN
  for graceful close and reset for abort, while TCP terminal cleanup closes the
  underlying connection directly. This negative result is why the public seam
  owns one `Close` outcome but does not prescribe one wire-level close action.
- The first MTU 1280 run falsified the default quic-go packet-size profile:
  TCP completed, while QUIC timed out after about one second and temporary
  `tcpdump` observers saw zero UDP packets on either namespace. In pinned
  v0.61.0, quic-go's default initial UDP payload is 1280 bytes and its public
  Config warns that an initial size too large for the path can time out the
  handshake. RFC 9000 requires support for a 1200-byte UDP payload and explains
  that a 1280-byte IP path leaves only 1232 bytes over IPv6 or 1252 over IPv4.
  [quic-go Config](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/interface.go),
  [quic-go default](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/internal/protocol/params.go),
  and [RFC 9000 §14](https://www.rfc-editor.org/rfc/rfc9000.html#section-14)
  (accessed 2026-08-24).
- The revised disposable QUIC candidate explicitly fixes
  `InitialPacketSize=1200`. The common Windows oracle then passed 15/15 again,
  and both TCP/TLS and QUIC completed the exact 256 KiB proof with link MTU
  1280 on both network namespaces. This is a candidate Profile parameter, not a
  reason to change the common Carrier Interface.
- Three complete checked fault-lab runs under Linux
  `6.6.87.2-microsoft-standard-WSL2`, the pinned tooling image
  `sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16`,
  and the same trimmed binary SHA-256
  `92339074961e1f27150bf12e11a0c5c0111f1354efdceda1b8af624587fea3b9`
  each passed 10/10 client and 10/10 server outcomes. Every
  client attempt called only its selected Adapter, retained FD count 7→7, and
  returned to one or two goroutines from a baseline of one within one second.
- With 100% UDP-only drop, the QUIC attempt returned `timeout` and made no TCP
  call; a separate authorized TCP control attempt succeeded while the same
  filter remained installed. The mirror TCP-only drop returned `timeout` and a
  separately authorized QUIC control succeeded. The first checked run counted
  six UDP and five TCP drops respectively.
- After adding the relay/rebinding harness, the complete fault matrix was run
  once more with its new binary SHA-256
  `0024f74f60e9d7f6adad00fc8989d68767b4f6c1a924dd9c8153e7fffaad3f15`.
  It again passed 10/10 client and 10/10 server outcomes: eight exact successes
  and the two predeclared selective-refusal timeouts. This is a regression
  check, not a fourth repetition of the earlier identical-binary sample.
- The symmetric netem row used, on both namespaces, queue limit 256, delay
  20 ms ± 5 ms with 25% correlation, random loss 5%, reorder 10% with 25%
  correlation, and seed 94095. The three checked runs completed the exact
  transcript on both profiles and each runner enforced nonzero drop counters on
  both sides. One earlier run of the same nominal TCP row instead returned a
  clean timeout after 28,536 ms with seven client drops. Therefore the evidence
  supports the predeclared “exact success or bounded failure” oracle, not a
  stable latency, availability, or TCP-versus-QUIC performance conclusion.
- quic-go continued to report that the UDP receive buffer grew from 208 KiB to
  416 KiB rather than the requested 7 MiB. Docker Desktop did not expose
  `/proc/sys/net/core/{rmem,wmem}_max` inside these containers, so the warning is
  the observation; the lab does not infer the host sysctl or throughput.
- Three complete synthetic rebinding runs used pinned Ubuntu image
  `sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517`
  and the same trimmed binary SHA-256
  `0024f74f60e9d7f6adad00fc8989d68767b4f6c1a924dd9c8153e7fffaad3f15`.
  Every run passed 2/2 client, 2/2 server, and 2/2 relay outcomes. The QUIC
  client made exactly `tcp=0,quic=1`, transferred the exact 262,144-byte proof
  with SHA-256
  `88a27acc92907475c5f16c76b14633def7715e1a1f7672fd362564229c05ce96`,
  retained FD count 7→7, and returned to its one baseline goroutine.
- In the three rebinding cases the opaque relay replaced its connected
  upstream socket after Carrier `Open`. The server-facing source changed,
  respectively, from ports `36956→33828`, `38855→33281`, and `33952→39421`
  while retaining relay IP `172.19.0.3`; nine packets preceded every change and
  253, 269, and 253 packets followed it. The attachment and selected QUIC
  Adapter did not change. The unchanged-port controls also completed, and each
  relay returned from seven to seven descriptors and one to one goroutines.
- The first two runner attempts failed before the experiment because
  PowerShell passed `-expect-rebind=` without its boolean value. The relay
  reported a flag-parse error and no client case ran. Parenthesizing the
  composed argument corrected the orchestration defect; these runs are not
  transport failures.
- Four dual-homed-runner attempts failed before case traffic while the checked
  orchestration was being made deterministic: Docker had not assigned inspectable
  addresses to created-but-not-started interfaces, PowerShell could not convert
  `0xffffffff` to the requested unsigned type, and Docker reported the accepted
  capability as `CAP_NET_ADMIN` rather than the runner's first expected spelling.
  Every attempt removed its exact labeled containers, bridges, and temporary
  binary. These are lab-construction failures, not transport results.
- Two later dual-homed attempts reached both data-plane cells and reproduced the
  same result under Linux `6.6.87.2-microsoft-standard-WSL2` and tooling image
  `sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16`.
  The exact trimmed binary SHA-256 was
  `92df951fbb999893168684e01c8a4b8535b5b85a40d541fc695f591c93d0063e`.
  The stable-path control delivered the exact 262,144-byte transcript with
  SHA-256
  `88a27acc92907475c5f16c76b14633def7715e1a1f7672fd362564229c05ce96`,
  one QUIC Adapter call, client FD `7→7`, client goroutines `1→1`, and joined
  cleanup.
- In the changed-route cell, Linux reported server-A via the B gateway and
  client-B source, but the exact server-B ingress filter counted zero packets
  and zero bytes. Client and server therefore reached the predeclared bounded
  timeout at about 5.9 seconds; the server remote tuple remained on client-A,
  the client still called exactly `tcp=0,quic=1`, and all labeled resources were
  removed. Because the oracle required a nonzero B-path counter in both the
  success and timeout outcomes, the slice is falsified. It does **not** show
  that QUIC migration failed: the intended alternate path was never observed.
- Pinned quic-go v0.61.0 exposes explicit client-side `AddPath`, `Path.Probe`,
  and `Path.Switch` operations; its own integration test creates a second
  `Transport`, probes it, and switches only after validation. That is a
  QUIC-specific policy and lifecycle surface, not evidence that a route-only
  mutation must move the existing socket and not a reason to export migration
  through the common Carrier contract.
  [connection source](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/connection.go),
  [path source](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/path_manager_outgoing.go),
  and [migration integration test](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/integrationtests/self/connection_migration_test.go)
  (accessed 2026-08-24).

A real two-host public-path run passed both single-attempt profiles
with the same binary and transcript oracle; R-094 records the exact hosts,
container limits, results, and cleanup. The later route-only dual-homed attempt
did not prove use of its alternate path. Genuine source/address or physical-path
migration, active probing/DPI, phase-by-phase cancellation, CPU/memory peak, and
a complete server-side descriptor census were not captured. Those remain later
profile gates rather than implied successes.

## Result

The Carrier seam hypothesis survives both the local and controlled-namespace
slices. Two actual Adapters satisfy the same authenticated ordered-carrier and
terminal-failure oracle without transport-local fallback or exported transport
types. The MTU falsification also shows that packet sizing is an exact Profile
fact below the seam.

This is sufficient to retain the Carrier Module direction and the refined QUIC
candidate, and it rejects the narrower concerns that a same-IP NAT source-port
change or the tested separate-host public path necessarily destroys this
experimental attachment. It is insufficient to select QUIC as a maintained
profile or direct dependency. The route-only dual-homed hypothesis did not
survive its own path proof: a route lookup changed, but no packet was observed
on B. If QUIC is later selected as an implementation candidate, a new
QUIC-private experiment may evaluate explicit `AddPath`/`Probe`/`Switch` or a
genuine access-network transition with phase cancellation and complete
process/socket/resource accounting. That work is not a prerequisite for the
maintained TCP/TLS-first slice.

## Result rules

Promote no library or protocol from a clean baseline alone. A QUIC candidate is
rejected for the alpha if it cannot meet every common oracle or its failure
budget with reproducible cleanup. A passing bounded lab result merely unlocks a
separate H4-2 decision: whether the added maintenance and UDP-blocked outcome
are worth a maintained second Carrier profile. It does not authorize a pluggable
transport, bridge, proxy, or censorship-resistance claim.

## Disposition

The dependency prerequisite, local seam slice, controlled-namespace fault
matrix, and synthetic same-IP NAT-port-rebinding slice are complete for
disposable code only. The separate-host slice is complete, and the route-only
dual-homed slice is closed as a failed path proof rather than as a migration
verdict. Retain the harness only while its unique evidence is being promoted;
any future explicit-QUIC or physical-path experiment is a new bounded question.
The harness must not become a maintained Carrier Module by gradual cleanup.
