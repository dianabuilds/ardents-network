---
id: R-013
title: Which technology candidates may enter Carrier Lab?
status: active
owner: product research
started: 2026-08-08
reviewed: 2026-08-10
---

# R-013 — Carrier Lab technology candidates

## Decision this unlocks

Freeze the Carrier Lab slice closely enough that implementation can begin
without selecting protocol-bound foundations or importing later product
horizons.

This record selects the components used to test the first R-004 Route candidate.
It does **not** select production routing, a public protocol, naming, bootstrap,
an SDK, storage, an updater, or Windows packaging. ADR-0009 separately selects
Go as the maintained project foundation; the remaining production choices stay
behind [Gate D](../../development/entry-gates.md).

## Decision

Carrier Lab uses a small native Tor-shaped split-circuit **Route Adapter** over
TCP, with a symmetric C-5 data path and a separate C2 Introduction Path. Go is
the first experiment language under [R-014](r-014-language-runtime-candidates.md).

Only routing orchestration, bounded framing, role state, and experiment evidence
are Ardents-specific. Cryptographic operations use maintained standard
implementations:

- telescoped TLS 1.3 channels protect each endpoint-to-Node circuit layer;
- a separate TLS 1.3 session protects and authenticates the Application stream
  end to end after both legs join;
- RFC 9180 base-mode HPKE with DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, and
  ChaCha20-Poly1305 seals the one-use Introduction invitation to the
  preconfigured Service Instance fixture;
- operating-system randomness creates every key, nonce, handle, and canary.

The first comparison controls are direct end-to-end TLS and the unqualified C-3
data path. C Tor in a private Chutney network is the mature black-box reference
before any custom route can be promoted. An I2P-shaped paired-tunnel Adapter is a
conditional follow-up only after a recorded C-5 result justifies that work.

## Accepted contract and traceability

- [Product scope](../../product/scope.md) authorizes only Carrier Lab and defines
  everything excluded from it.
- [J-LAB](../../product/journeys.md) fixes the controlled journey from topology
  startup through role inspection and one explicit path failure.
- [R-001](r-001-interactive-route-claim.md) defines the limited role-local
  knowledge claim and its non-claims.
- [R-004](r-004-routing-rendezvous-families.md) selects C-5, C2 separate
  Introduction, and the comparison order without selecting a production
  mechanism.
- [R-002](r-002-live-application-interface.md) keeps the eventual Application
  Interface outside this internal harness.
- [R-006](r-006-service-target-lifecycle.md) permits only ephemeral
  authenticated Target/Instance fixtures in this horizon.
- [R-023](r-023-interactive-route-performance-budget.md) supplies future
  hypotheses; Carrier Lab uses coarse gates and earns no qualification.
- The [threat model](../../security/threat-model.md) limits Carrier Lab to
  controlled Target authentication, Application Data protection, role views,
  explicit failure, finite resources, and feasibility evidence.

## Evidence classification

- **Sourced facts:** the capabilities and limits attributed to Tor/Chutney,
  Arti, libp2p, Waku, I2P, Nym, Ubuntu, Go, TLS 1.3, and HPKE come from the
  specifications, official documentation, and source repositories linked in
  this record. All external sources were accessed on 2026-08-09.
- **Measurements:** 2026-08-10 Docker Desktop development smokes prove one
  native C-5/C2 setup, protected 64 KiB stream, real shaping/capture, role-view
  collection, image binding, fail-closed Rendezvous process loss, and cleanup.
  This is implementation evidence, not
  the frozen comparative workload, an official Ubuntu result, a Route verdict,
  or an anonymity result. No Tor/Chutney comparison exists yet.
- **Assumptions:** one Product Owner plus Codex must be able to own the selected
  stack; the fixed synthetic topology and workload are useful feasibility
  proxies; local container roles do not imply independent operators.
- **Recommendations and decisions:** use the frozen native candidate and
  controls below with the reviewed R-025 shaping/capture supply, and promote no
  Route until measured advance criteria pass.

## Why no complete existing stack is selected

| Candidate | Carrier Lab disposition | Reason |
|---|---|---|
| C Tor | External black-box control; not Ardents production foundation | It is the closest mature Introduction/Rendezvous reference, but owns its directory, path selection, circuit lifecycle, Carrier choices, and observability. An internal `.onion` fixture is never an Ardents Name or Target. |
| Arti | Reference and possible endpoint library only | Arti can act as a client and host onion services but cannot operate relays, so it cannot implement an independently operated Ardents network. Its lower interfaces also remain coupled to Tor semantics. |
| go-libp2p / rust-libp2p | Later Carrier Channel toolbox candidate only | TCP, QUIC, WebSocket, Noise/TLS, multiplexing, and resource management may be reusable. Circuit Relay is reachability through named Peer IDs, not the accepted anonymous split route. Identify, PeerStore, DHT, AutoNAT, hole punching, and Relay do not enter Carrier Lab. |
| Waku | Rejected for Route and Service Connection | Relay, Filter, Store, and LightPush carry messages and topic interests; they do not provide the required live ordered stream or C-5 knowledge separation. |
| Nym | Deferred to a separately justified stronger Route Profile | Mixing, delay, cover traffic, packet reordering, gateways, and message reconstruction target a stronger traffic-analysis claim and conflict with the current low-latency experiment. |
| Java I2P / i2pd | External architectural reference; conditional fallback | Paired inbound/outbound tunnel pools and Streaming are relevant, but importing netDb, LeaseSets, Destination identity, profiling, naming, and a Java/C++ router would answer many questions outside Carrier Lab. |

Primary sources (accessed 2026-08-09):

- [Tor onion-service protocol](https://spec.torproject.org/rend-spec/protocol-overview.html)
  and [Chutney](https://gitlab.torproject.org/tpo/core/chutney);
- [Arti current status](https://arti.torproject.org/FAQs/);
- [libp2p Circuit Relay](https://docs.libp2p.io/concepts/circuit-relay/)
  and [Peer IDs](https://libp2p.io/docs/peers/);
- [Waku protocols](https://docs.waku.org/learn/concepts/protocols/);
- [I2P Streaming](https://i2p.net/en/docs/api/streaming/) and
  [I2P tunnel routing](https://i2p.net/en/docs/overview/tunnel-routing/);
- [Nym implementation](https://github.com/nymtech/nym).

## The only external design seam in the first implementation

The **Route Module Interface** is the experiment's one external seam. It is real
rather than hypothetical because the same tracer drives two materially different
Adapters:

1. the native C-5/C2 Adapter; and
2. the external C Tor/Chutney control Adapter.

The interface is behavioral, not a frozen Go package. A caller supplies one
fixed lab Route Profile, one local Isolation Context token, authenticated
preconfigured reachability material, and a finite deadline. It receives either
one joined full-duplex opaque Route attachment or an explicit route-unavailable/
deadline result. The common lab Service Connection above this seam performs the
end-to-end TLS handshake, authenticates the exact Target/Instance, converts the
attachment into the ordered Application stream, and owns target-authentication
or end-to-end-integrity failure.

The interface exposes no Node, hop count, Introduction, Rendezvous, Carrier,
retry, or topology data. Carrier Lab performs no same-connection route repair;
one path failure closes explicitly. Consequently a public Carrier Channel seam,
storage port, clock port, codec plugin system, and generic routing virtual
machine would be hypothetical indirection and are forbidden in this slice.

## Pre-registered hypotheses

- **H1 — retain C-5:** the native C-5/C2 Adapter preserves the required
  role-local knowledge separation and carries one useful protected stream on a
  modest controlled Ubuntu host.
- **H2 — prefer adoption/recomposition:** the mature Tor control meets the same
  useful-stream need and the native Adapter demonstrates no necessary security,
  operability, evidence, continuity-seam, or Carrier-agility advantage worth a
  custom protocol.
- **H3 — test tunnel pools next:** C-5 preserves the knowledge boundary but its
  setup or failure cost is structurally poor enough to justify one bounded
  I2P-shaped paired-tunnel experiment.
- **H0 — stop Route implementation:** no tested shape plausibly combines the
  accepted knowledge boundary with a useful live stream. Reopen R-004 before
  names, public discovery, packaging, or product code.

## Fixed experiment boundary

Carrier Lab contains only:

- controlled Ubuntu 26.04 LTS `x86-64` processes;
- one User tracer, one Service tracer, and one active synthetic Instance;
- one preconfigured Target/reachability fixture and per-run project test keys;
- one connection and one deterministic opaque byte stream at a time;
- the native C-5/C2 Adapter and declared controls;
- per-role synthetic traffic/state inspection;
- coarse setup, goodput, CPU, RSS, queue, and explicit-failure evidence.

It contains no Service Name, Namespace, descriptor network, DHT, public peer
discovery, Candidate View, public Role Domains, admission, Sybil defense, Bridge,
camouflage, retained delivery, database, public local proxy, SDK, browser,
Windows build, updater, recovery, multipath, production installer, or public
privacy claim.

The native candidate is built and run with these frozen base inputs:

- Ubuntu 26.04 LTS `linux/amd64` image manifest
  `sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`;
- `go1.26.5.linux-amd64.tar.gz`, SHA-256
  `5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053`.

The pre-run manifest repeats and verifies both values. Replacing either creates
a new candidate condition; a mutable image tag or unverified host toolchain is
not evidence. Ubuntu 26.04 is the current LTS and is supported through 2031
according to the
[Ubuntu release notes](https://documentation.ubuntu.com/release-notes/26.04/).
Go 1.26.5 and its archive digest come from the official HTTPS
[Go download manifest](https://go.dev/dl/?mode=json).

## Fixed logical topology

### C-5 data path

```text
User
  -> User Entry
  -> User Interior
  -> Rendezvous
  -> Service Interior
  -> Data Service Entry
  -> Service
```

The five ordinary data positions have different per-run Node keys, processes,
containers, addresses, and configuration. Co-residence on one controlled host
does not claim independent operation.

### C2 separate Introduction Path

```text
User
  -> User Entry
  -> User Interior
  -> Introduction Forwarder
  -> Introduction Node
  -> Introduction Interior
  -> Introduction Entry
  -> Service
```

The User Entry and User Interior may carry both setup paths in separate channels
because neither receives the Target. Introduction Forwarder, Introduction Node,
Introduction Interior, and Introduction Entry are distinct from Rendezvous,
Service Interior, and Data Service Entry. The selected Rendezvous never forwards
or receives the invitation and learns no Introduction slot or Node.

Each container joins only its adjacent synthetic link networks. The User and
Service tracer processes have no ordinary network path around their configured
endpoint role. This is a harness property, not a reusable Application-isolation
claim. Role containers share no management network, global Docker DNS view,
host socket, or Docker control socket; each writes evidence through its own
bounded filesystem mount for collection after the run.

## Protected setup and stream

1. The harness generates fresh per-run Ed25519 Node certificates, an Ed25519
   Target root and active Instance leaf, random route handles, a separate X25519
   HPKE key for the synthetic Instance, and the pinned Target/Instance TLS chain.
2. The Service prepares the fixed Introduction Path and registers one opaque,
   expiring Introduction slot. There is no storage after the live path ends.
3. The User telescopes authenticated TLS 1.3 channels through User Entry and
   User Interior to a fresh Rendezvous handle. Every earlier role forwards an
   encrypted inner channel and learns only its adjacent role.
4. In parallel, the User telescopes through the same Initiator leg to the
   Introduction Forwarder and Introduction Node.
5. The User seals one bounded invitation with RFC 9180 HPKE. It binds the fixed
   profile identifier, run/network identifier, Rendezvous identity and
   reachability, a fresh 32-byte one-use join token, expiry, and the first
   endpoint-handshake nonce. The local Isolation Context token is never sent.
6. The Introduction roles forward only the sealed invitation. The Service
   rejects expiry, malformed ciphertext, a replayed join token, the wrong
   run/network/profile, or a substituted Rendezvous.
7. The Service independently telescopes through Data Service Entry and Service
   Interior, then attaches to the proposed Rendezvous using a separate random
   handle and proof of the invitation token.
8. Rendezvous pairs the two attempt handles once, erases the pending token, and
   joins two opaque byte streams. It receives no Target, endpoint key, or
   plaintext Application Data.
9. The endpoints perform a fresh TLS 1.3 handshake across the joined stream.
   The User validates the exact preconfigured Target root and active Instance
   leaf. Service Name, Target, and a target-derived SNI are absent from the
   ClientHello; any SNI/ALPN value is fixed and non-identifying.
10. Only after that handshake succeeds does the tracer expose the stream and
    start the Application workload.

TLS session resumption and early data are disabled. Every completed connection
uses TLS 1.3 only and fresh endpoint and route session keys; the lab pins X25519
as its TLS key-exchange group and records the negotiated cipher suite. These are
lab suite choices, not a production algorithm-agility decision. Carrier Lab
claims only best-effort cleanup of ephemeral memory, not physical erasure from
swap, snapshots, or a compromised host.

## Lab-only framing and limits

The native Adapter uses a deliberately non-production codec:

- one unsigned 4-byte big-endian frame length followed by one 1-byte frame type
  and its payload;
- maximum complete frame length `65,536` bytes;
- maximum Application Data payload `16,384` bytes per frame;
- 32-byte uniformly random route and join handles;
- maximum sealed invitation `4,096` bytes;
- maximum live setup lifetime `30 s` and connection-open deadline `15 s`;
- maximum queued logical Application Data `256 KiB` per connection and
  direction at every endpoint/relay process;
- no disk spill, compression, deduplication, message retention, negotiation, or
  unknown-frame tolerance.

Frame types are limited to route extension/result, introduction
register/deliver/acknowledge, rendezvous register/attach/result, protected data,
close, and terminal error. Each payload has a fixed shape in the experiment
source and is parser-fuzzed. An invalid version, type, length, state transition,
handle, replay, or deadline fails closed. These bytes carry no compatibility
promise and must not be copied into production merely because the experiment
passes.

Application queue limits count logical payload bytes. Go channels are also
bounded by fixed-size data chunks; an arbitrary message-count limit is not
accepted as a byte limit. Kernel/TCP/TLS copies remain visible in RSS and socket
evidence.

## Required role-view evidence

Every role runs in an audit build that records all configuration available to
it, every decoded field it receives after terminating its own TLS layer, state
transitions, adjacent connection identities, queue high-water marks, and
terminal cleanup. The collector also keeps per-link packet captures. All values
are synthetic, but endpoint and Target test secrets remain excluded from the
committed summary.

Evidence is finite: each role audit log is capped at `32 MiB`, each link/phase
capture at `64 MiB`, and the complete raw run directory at `2 GiB`. Setup,
canary, and negative cases retain a sufficient snap length to search all
sentinels; timed-stream traffic uses bounded header capture plus exact interface
counters. Reaching a cap invalidates the run instead of silently truncating
required evidence.

| Role | May observe | Must not receive from its protocol role |
|---|---|---|
| User Entry | User origin, User Interior, timing, volume | Rendezvous, Introduction Node, Service side, Target, plaintext |
| User Interior | User Entry plus Rendezvous or Introduction Forwarder on its separate channel | User origin, Service side, Target, invitation plaintext |
| Rendezvous | User Interior, Service Interior, random attempt handles | either Entry, either endpoint origin, Introduction role/slot, Target, endpoint key, plaintext |
| Introduction Forwarder | User Interior, Introduction Node, random channel state | either endpoint origin, Rendezvous, Target, invitation plaintext |
| Introduction Node | Introduction Forwarder, Introduction Interior, opaque slot and sealed invitation | either endpoint origin, Rendezvous, Target-to-origin binding, invitation plaintext |
| Introduction Interior | Introduction Node, Introduction Entry, opaque handles | Service origin, User side, Target, invitation plaintext |
| Introduction Entry | Service origin, Introduction Interior | User origin, data Rendezvous, Target, invitation plaintext |
| Service Interior | Rendezvous, Data Service Entry, opaque handles | Service origin, User side, Target, plaintext |
| Data Service Entry | Service origin, Service Interior | Rendezvous, User side, Target, endpoint-handshake plaintext |

The harness embeds independent random sentinels for User origin, Service origin,
Target, Instance, complete route vector, invitation plaintext, canary, and
Application plaintext. A forbidden sentinel in a role's decoded view, retained
state, configuration, or unexpected cleartext link capture is a hard failure.
Timing and volume similarity is reported but is not treated as a forbidden
sentinel or an anonymity success.

## Workload and controls

All conditions use the same seeded incompressible bytes and the same tracer
verification:

1. establish one fresh connection;
2. exchange a fresh 32-byte request/response canary;
3. stream seeded incompressible data for `60 s` User-to-Service;
4. repeat on a fresh connection for `60 s` Service-to-User;
5. verify every byte in order and count only verified Application bytes.

For each direct, C-3, and C-5 condition, collect `20` setup attempts and three
independent timed streams in each direction. Report every result, nearest-rank
`p50`/`p95` setup time, minimum/median goodput, process CPU time, RSS, queue high
water, and per-link bytes. This sampling is a coarse research comparison and
cannot satisfy R-023 qualification.

The controlled network caps every synthetic link at `100 Mbit/s`. Fixed endpoint
egress delay produces an `80 ms` round-trip floor in every condition; internal
links add no artificial geographic claim. There is no random loss, reordering,
or background traffic in the base Carrier Lab. Exact `tc netem` parameters,
container limits, host CPU/memory, kernel, Docker version, image digests, and
seed schedule are written to the immutable pre-run manifest.

The controls are sequential:

1. **Direct TLS:** measurement floor only; never a product fallback.
2. **C-3:** the same native Implementation with the shorter unqualified topology;
   its Rendezvous sees the full Node sequence, so performance cannot promote it.
3. **C-5/C2:** the actual candidate and only condition receiving a Route verdict.
4. **C Tor/Chutney:** run only after the native candidate completes enough valid
   attempts for a useful comparison. It is a mature reference, not evidence that
   native role views pass.
5. **Paired tunnel pools:** do not implement automatically. A follow-up record
   must cite the exact C-5 failure that this different Adapter is expected to
   answer.

## Mandatory negative and failure cases

Carrier Lab additionally performs one attempt for each case:

- wrong pinned Instance leaf under the otherwise valid harness;
- modified protected Application record;
- replayed Introduction invitation;
- invitation bound to the wrong profile/run/Rendezvous;
- oversized frame and invalid state transition;
- one fixed injected Rendezvous process failure during the verified stream.

Every negative case must fail closed and present no unauthenticated bytes as
Application Data. The fixed Rendezvous failure may terminate the connection;
Carrier Lab does not implement replacement. It must become an explicit terminal
result within `15 s`, stop accepting new Application bytes, and never deliver a
duplicate, reordered, or unverifiable byte prefix.

## Advance, redesign, and stop rules

### Advance the candidate

Advance C-5 to another experiment only when all of the following hold together:

- no forbidden role-view or cleartext disclosure occurs;
- every positive attempt authenticates the exact Target/Instance and every
  negative case fails closed;
- at least `19/20` C-5 setup attempts succeed;
- C-5 setup `p95 <= 3 s` in this controlled ready-state fixture;
- each direction's minimum 60-second goodput is at least
  `min(10 Mbit/s, 50% of its paired direct median)`;
- no candidate process exceeds `512 MiB` RSS for an endpoint or `256 MiB` RSS
  for an ordinary Node during the one-connection run;
- mean candidate CPU per process remains below one logical core during the
  timed stream;
- every logical queue remains at or below `256 KiB` and a slow/closed consumer
  propagates backpressure rather than allocating without bound;
- the injected failure reaches the explicit terminal result within `15 s`;
- cleanup removes live processes, sockets, temporary credentials, and unretained
  captures without leaving generated material in the repository.

These are Carrier Lab feasibility thresholds, deliberately borrowed where useful
from the eventual product budget. Passing them earns no product SLO, privacy, or
Route Qualification claim.

### Redesign before another run

Redesign the native Adapter, without expanding product scope, when knowledge and
authentication pass but a bounded implementation defect explains a missed
setup, throughput, resource, queue, or failure threshold. The replacement gets a
new candidate identity and all Carrier Lab conditions rerun.

### Stop and reopen architecture

Stop the current direction when any of these is true:

- satisfying the knowledge matrix requires trusting a role not to inspect data
  already available to that role;
- one role necessarily receives an endpoint origin plus Target/opposite endpoint
  or the complete data-route identity sequence;
- Target authentication must become a Tor onion identity, I2P Destination,
  libp2p Peer ID, DNS name, public CA name, or another foundation-specific ID;
- a shorter path, direct connection, weaker profile, cached result, or cleartext
  fallback is required to pass;
- the custom Implementation grows a DHT, naming system, public control plane,
  peer reputation, retained mailbox, token economy, or production updater to run
  the fixed experiment;
- TCP-to-QUIC replacement would require changing the Route Module Interface or
  Application stream semantics rather than one internal Carrier Adapter;
- an unresolved high/critical vulnerability, unacceptable license, unsupported
  dependency, or unauditable native/unsafe dependency is required;
- C Tor is at least as suitable for the accepted interface and evidence while
  the native route demonstrates no measured necessary advantage;
- two bounded redesigns miss the same hard knowledge, authentication, or useful-
  stream gate.

Stopping is a successful research result. It blocks naming and public-network
implementation until R-004 is explicitly revised.

## Tooling and dependency policy

The first native candidate is standard-library-first Go:

- `net`, `io`, `context`, `crypto/tls`, `crypto/x509`, `crypto/rand`,
  `crypto/hpke`, `encoding/binary`, `encoding/json`, `log/slog`, pprof, and
  `runtime/metrics`;
- no Waku, libp2p, QUIC, gRPC, Protobuf, database, cgo, or first-party `unsafe`
  in the release-style lab binary;
- built-in tests, fuzzing, race testing, `vet`, and `govulncheck`;
- Docker Compose, Linux network namespaces/bridges, `tc netem`, and
  tcpdump/tshark for orchestration and evidence.

The exact Go toolchain, module graph, container images, Tor/Chutney revisions,
and tool versions are pinned in the experiment manifest. Dependency changes
create a new candidate condition.

## Evidence retention and cleanup

Each run produces:

- immutable input and seed manifest;
- source, binary, toolchain, module, image, and tool hashes;
- complete per-role JSONL audit events and role-view verdicts;
- raw monotonic timing, CPU, RSS, queue, byte, and failure observations;
- hashes and locations of local packet captures;
- machine-readable condition verdict and a short human report;
- cleanup report listing removed containers, networks, sockets, keys, and temp
  directories.

Raw packet captures, credentials, databases, build caches, and generated
dependencies are never committed. They live in a run-specific system temporary
directory and are deleted after their hashes and permitted synthetic summaries
are retained, unless the Product Owner explicitly chooses an external evidence
archive. Committed evidence contains no private key or reusable credential.

## Open after this record

### Tool-supply prerequisite resolved on 2026-08-09

[R-025](r-025-carrier-lab-tool-supply.md) fixes one minimal supply path: 12
official Ubuntu `.deb` artifacts are content-addressed in
`carrier-lab/tools.lock`, verified and extracted without installation or
network access into a disposable tooling image, and executed only by separate
namespace-sharing sidecars. Shapers have exact effective `NET_ADMIN`; capture
has exact effective `NET_RAW`; Application tracer roles have no effective
capabilities.

Development smoke on Docker Desktop applied and reported real endpoint qdiscs
with `delay 40ms rate 100Mbit`, captured the bounded TCP synthetic marker with
real tcpdump/libpcap, retained its SHA-256, removed the raw pcap, and proved
normal and forced-failure cleanup. The run is supply evidence, not an official
native Route result. Tool supply is no longer a C-5/C2 implementation blocker;
the separate native task must still run the complete frozen scenario on the
official Ubuntu 26.04 `x86-64` runner. Direct TLS remains only its measurement
control and never a fallback.

### Native implementation status on 2026-08-10

The lab-only `internal/nativecircuit` Module now implements the frozen positive
C-5/C2 scenario without selecting a product protocol:

- nine distinct Node roles plus User and Service endpoint roles run in isolated
  adjacent Compose networks; the C2 Introduction Path is separate from C-5;
- endpoint-to-Node circuits telescope TLS 1.3/X25519 layers; RFC 9180 HPKE seals
  the one-use invitation; a separate TLS 1.3/X25519 session authenticates the
  exact ephemeral Target/Instance and protects the Application stream;
- bounded framing, queue, invitation, replay, binding, wrong-Instance,
  modified-record, invalid-frame/state, and Rendezvous-cancellation behavior is
  covered by package tests and parser fuzz seeds;
- 11 namespace-sharing shapers apply real `tc netem`; 10 capture sidecars retain
  11 per-link pcap identities using real tcpdump/libpcap. Application roles have
  no effective capabilities, while tool roles receive exactly one of
  `NET_ADMIN` or `NET_RAW`;
- application and tooling image IDs are verified against their target/base
  labels, the current qualification source digest, an identical binary digest,
  and the exact embedded tool lock before Compose starts;
- raw captures and credentials remain in one run-owned system-temporary tree.
  The retained development evidence contains bounded summaries and hashes, and
  successful cleanup removes raw captures, containers, networks, and run state.

The successful positive development smoke carried and verified 65,536 Application
bytes, authenticated the exact Instance, found neither the Target marker nor
the Application marker in cleartext captures, collected every role/tool result,
and proved cleanup. This closes the implementation slice only. It deliberately
does not produce `advance`, `redesign`, or `stop`. The paired failure smoke
kills the real Rendezvous after the first verified 16 KiB chunk and requires
explicit endpoint failure within 15 seconds without accepting the full stream.
The 20 setup attempts, six 60-second streams per condition, Direct/C-3
comparison, full workload failure evidence, official Ubuntu 26.04 runner
evidence, and later Tor/Chutney reference remain the separate R-013 experiment
run.

- full comparative Carrier Lab measurements and the Route verdict;
- the bounded Go/Rust production comparison in R-014 after a viable Route;
- whether QUIC or libp2p earns a second real Carrier Channel Adapter;
- production endpoint handshake, credential encoding, protocol negotiation,
  canonical signed records, storage, discovery, naming, bootstrap, update, and
  Windows IPC;
- all R-023 qualification and independent security review.

## Disposition

- State: `active`; the native development implementation smoke exists and is
  ready for the frozen comparative experiment, but no Route verdict exists.
- Gate A is satisfied for the named R-004/R-013 experiment.
- Gate B permits implementing the Carrier Lab slice in the root project against
  this exact record. It does not permit any later-horizon subsystem.
- ADR-0009 selects only the maintained Go project foundation; this record still
  makes no route or production-network selection.
