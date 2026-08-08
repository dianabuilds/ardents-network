---
id: R-004
title: Which routing and rendezvous family should carry the Interactive Route?
status: active
owner: product research
started: 2026-08-08
reviewed: 2026-08-08
---

# R-004 — Routing and rendezvous families

## Decision this unlocks

Select the smallest route and rendezvous architecture worth prototyping against
the accepted Interactive Route security and performance contracts. This record
does not select a library, cryptographic construction, implementation language,
wire protocol, or production route.

## Accepted gate

A candidate must preserve all of the following together:

- both endpoint locations remain hidden from the opposite endpoint;
- no one ordinary Node links an endpoint location to the opposite endpoint,
  Service Name, or Service Target;
- each ordinary Node receives only adjacent and role-specific information,
  short-lived opaque handles, timing, direction, and volume;
- Application Data and exact-target authentication remain end-to-end protected
  even when every carrier Node colludes;
- a direct path, one trusted proxy, or weaker fallback never substitutes for the
  Interactive Route;
- Broad Traffic Observer and arbitrary-collusion resistance are explicit
  non-claims, not hidden goals that silently add cover-traffic cost;
- a warm exact-name connection has `p95 <= 1 s`, a cold one has `p95 <= 3 s`,
  and one eligible route failure resumes the same Service Connection within
  `p95 <= 5 s` or terminates explicitly by `15 s`;
- the required client sustains the accepted goodput, concurrency, CPU, memory,
  queue, and traffic-overhead budgets on ordinary Windows and Ubuntu devices.

These are conjunctive gates. Familiar vocabulary such as onion, garlic, mix,
DHT, or peer-to-peer is not evidence that a candidate passes.

## Primary-source findings

### Tor-shaped onion-service rendezvous

Tor onion services separate descriptor storage, Introduction Points, a
client-selected Rendezvous Point, and two independently built circuits. The
service publishes a descriptor containing current Introduction Points; the
client obtains it, builds a circuit to a chosen Rendezvous Point, sends a sealed
introduction, and the service builds its own circuit to that rendezvous. The
Rendezvous Point joins the circuits while the endpoint handshake authenticates
the service. This is the cleanest mature reference for two-sided location
privacy and role-separated rendezvous.

Tor also depends on a multiply signed directory consensus and compiled fallback
directory list. Those are Control Plane choices, not necessary consequences of
the rendezvous shape, and cannot be inherited without separate R-009 and R-012
review.

Sources:
[Tor protocol](https://spec.torproject.org/rend-spec/protocol-overview.html),
[Tor consensus](https://spec.torproject.org/dir-spec/outline.html), and
[Tor fallbacks](https://spec.torproject.org/dir-list-spec.html).

### I2P-shaped paired tunnel pools

I2P uses separately built unidirectional inbound and outbound tunnels. A round
trip uses an outbound and inbound tunnel for each party. Signed, expiring
LeaseSets describe destination reachability without publishing the endpoint's
ordinary location; netDb floodfill peers store and retrieve them, and sensitive
stores and lookups travel through tunnels with end-to-end garlic encryption.
Multiple reusable tunnels make warm use and path replacement natural, but the
distributed database, peer profiling, daily keyspace movement, and four-tunnel
round trip create a large coupled mechanism and Sybil surface.

Sources: [I2P garlic routing](https://geti2p.net/en/docs/how/garlic-routing),
[I2P network database](https://i2p.net/en/docs/overview/network-database/), and
[I2P encrypted LeaseSet specification](https://geti2p.net/spec/encryptedleaseset).

### Nym-shaped packet mixnet

Nym uses source-routed Sphinx packets, layered mix nodes, randomized delay,
reordering, single-use reply blocks, and cover traffic. That is aimed at a
stronger traffic-analysis claim than Ardents currently makes. Its messages are
routed independently rather than through a FIFO circuit, so an ordered live
byte stream needs substantial reconstruction above the mixnet. Random delay,
cover traffic, fixed packetization, and the topology/control plane add direct
risk to the accepted latency, goodput, idle-traffic, and no-token boundaries.

Source: [Nym network whitepaper](https://nymtech.net/nym-whitepaper.pdf).

### Session, Lokinet, libp2p, and Waku

Session's current Onion Requests are a useful example of a simple three-hop
request path, but Session documents that this message-over-TCP mechanism is not
well suited to large transfers or responsive voice/video. Its recipient swarm
and retained-message behavior also solve a messenger problem outside Ardents.

Lokinet is a more relevant architecture reference: its SNApps publish DHT
descriptors with introducers, and its low-latency overlay is described as a Tor/
I2P hybrid. Its service-node admission and topology depend on a token and
blockchain, so its complete network is not compatible with the accepted Ardents
boundary.

libp2p remains a possible implementation toolbox, not an anonymity architecture.
Its Circuit Relay documentation explicitly says relaying is not anonymous and
all participants use Peer IDs. Hole punching deliberately replaces relaying with
a direct connection, which cannot be an Interactive Route. Waku is likewise a
message dissemination family; its Store and Filter modes directly link Peer IDs
to requested topics or filters and do not provide the required live stream or
Route Knowledge Separation.

Sources: [Session network documentation](https://docs.getsession.org/),
[Lokinet SNApps](https://docs.oxen.io/oxen-docs/products-built-on-oxen/lokinet/snapps),
[libp2p relay](https://docs.libp2p.io/concepts/circuit-relay/), and
[Waku security](https://docs.waku.org/learn/security-features/).

## Option A — Full two-sided onion circuits

The User and Service each build a complete multi-relay circuit. The Service
maintains separate introduction circuits; the User chooses a Rendezvous Node;
the Service builds another circuit to it; and the rendezvous joins both legs.

### Advantages

- strongest direct correspondence with a mature two-sided hidden-service flow;
- descriptor, introduction, rendezvous, and data responsibilities are distinct;
- neither endpoint needs an inbound public listener;
- additional relays on each side reduce what one compromised position can infer
  and make guard-style route-selection research reusable.

### Costs and risks

- several circuit builds sit inside a `1 s` warm and `3 s` cold connection gate;
- each byte crosses two full circuit legs, raising CPU, queue, bandwidth, and
  tail-latency cost for `64` client and `256` publisher connections;
- a failed circuit normally destroys the underlying stream unless Ardents adds
  its own end-to-end same-connection continuity layer;
- copying Tor's directory authorities or fixed circuit length would introduce
  unrelated Control Plane and performance decisions.

### Assessment

Security reference: **strong**. Performance fit: **uncertain to weak**. It should
remain a comparison control, not the default implementation plan.

## Option B — Paired reusable unidirectional tunnel pools

Each endpoint maintains several outbound and inbound tunnels. A signed,
short-lived Service Descriptor advertises only anonymous inbound tunnel leases.
The User sends through one outbound tunnel into a Service inbound tunnel; the
reply uses a Service outbound tunnel and a User inbound return lease. A logical
Service Connection schedules protected stream data over replaceable tunnel pairs.

### Advantages

- prepared tunnel pools make warm connection and route replacement plausible;
- forward and reverse directions can use different Nodes and failure domains;
- no single data Rendezvous becomes a mandatory bottleneck;
- signed expiring reachability resembles the accepted Service Descriptor model.

### Costs and risks

- four tunnel directions and their pools produce the largest endpoint state and
  scheduling surface;
- ordered delivery, congestion control, retransmission, and duplicate suppression
  across changing one-way paths are difficult;
- descriptor DHT storage and lookup create eclipse, Sybil, enumeration, metadata,
  and overload problems that must be solved rather than inherited;
- reusable tunnels can link connections unless Isolation Context separation is
  enforced below every pool and peer-selection cache.

### Assessment

Security fit: **strong in principle**. Warm-use and recovery fit: **promising**.
Complexity and DHT risk: **high**. This is the principal alternative to prototype
against the front-runner.

## Option C — Ardents split-leg rendezvous

Use the accepted claim to design a smaller route rather than copy a reference
network's full path:

```text
User -> User Entry/Bridge -> Rendezvous -> Service Entry -> Service
```

The three ordinary data-path positions must be under separately selected control.
The Service maintains multiple outward introduction and entry choices and
publishes only short-lived opaque introduction material in its authenticated
Service Descriptor. The User privately retrieves the descriptor, prepares its
entry leg, selects a fresh Rendezvous, and sends a sealed one-use invitation
through an introduction path. The Service attaches an independently selected
entry leg to the Rendezvous. The Rendezvous joins two random handles; the
endpoints then authenticate the exact Service Target and create the protected
Service Connection end to end.

The data-path views are deliberately narrow:

- User Entry knows User location and Rendezvous, but no Service destination;
- Service Entry knows Service location and Rendezvous, but no User origin;
- Rendezvous knows both entry Nodes and one-use handles, but no endpoint
  location, Service Name, Service Target, or plaintext;
- introduction delivery cannot reveal both an endpoint location and descriptor
  identity to one ordinary Node;
- route recovery replaces one or both legs, or the Rendezvous, beneath the same
  endpoint-authenticated Service Connection.

### Advantages

- it is the smallest current shape that robustly separates both endpoint-adjacent
  views from destination-specific introduction and rendezvous state;
- one bidirectional data route is substantially cheaper than two full circuits or
  four unidirectional tunnel pools;
- prepared entry legs and service introduction state can optimize the warm path;
- Bridge entry replaces the User Entry transport without changing the security
  profile or exposing a direct path;
- rendezvous state is per-connection, short-lived, and horizontally distributable.

### Costs and risks

- this exact construction is not inherited from a deployed protocol and must earn
  every security claim through information-flow tests and a prototype;
- the introduction path is the subtle part: public descriptor material must not
  let User Entry, Rendezvous, or Introduction roles derive the Service Target;
- three data-path Nodes are the minimum, so unknown common control or poor role
  diversity has a larger effect than in longer circuits;
- the Rendezvous is a per-connection availability and traffic-analysis point;
- same-connection recovery needs a new authenticated route-attachment mechanism
  that prevents replay, rollback, stream duplication, and cross-context linkage.

### Assessment

Contract fit: **best**. Expected performance fit: **best hypothesis**. Evidence:
**none yet**. Advance this option to a bounded prototype while retaining Option B
as the alternative and Option A as the security reference.

## Option D — Delayed packet mixnet

Use independently routed fixed-size packets, randomized mixing delay, reordering,
reply blocks, and cover traffic for the baseline connection.

### Assessment

This targets the Broad Traffic Observer that the Interactive Route explicitly
does not claim to resist. It conflicts structurally with FIFO streaming and the
accepted latency, idle-traffic, and overhead budgets. Do not prototype it for the
baseline. It remains a possible R-005 Route Profile only after a concrete
Application need justifies the stronger claim and cost.

## Comparative result

| Criterion | A: full onion circuits | B: tunnel pools | C: split rendezvous | D: mixnet |
|---|---|---|---|---|
| One-Node knowledge separation | Strong reference | Strong if pool roles remain separate | Exact intended contract | Stronger than required |
| Warm `p95 <= 1 s` hypothesis | Weak to uncertain | Promising with prepared pools | Best | Poor |
| `10 Mbit/s` and overhead hypothesis | Highest circuit cost | Medium to high scheduling cost | Best | Poor |
| Same-connection `p95 <= 5 s` recovery | Requires added continuity | Natural path alternatives, complex ordering | Requires added leg attachment | Requires stream reconstruction |
| Endpoint state | Medium-high | Highest | Lowest current hypothesis | High |
| Shared lookup/control risk | HSDir and consensus if copied | Floodfill/DHT eclipse and Sybil | Still needs descriptor and bootstrap design | Epoch topology and admission root |
| Mature complete analogue | Tor | I2P | None | Nym |
| Current disposition | Comparison control | Alternative prototype | Front-runner prototype | Defer to R-005 |

The grades are architectural hypotheses, not measured results or Route
Qualification.

## Prototype decision gate

No production implementation is justified yet. A throwaway candidate must first:

1. model every Introduction, Entry, Rendezvous, descriptor, and recovery view;
2. make each ordinary role malicious in turn and prove no forbidden value appears
   in live state, traffic, handles, logs, or retained state;
3. run Option C with three data-path positions and Option B with the smallest
   viable tunnel pool on the same controlled topology;
4. measure cold and warm connection setup, one-stream goodput, `64`/`256`
   connection workloads, CPU, RSS, queues, and endpoint traffic overhead;
5. inject one leg, Rendezvous, Introduction, Carrier Channel, and descriptor-path
   failure and test the accepted `5 s`/`15 s` outcomes;
6. test replay, target substitution, route downgrade, timing and loss tagging,
   descriptor enumeration, pool reuse, and Isolation Context crossover;
7. retain all failed runs as evidence and reject any candidate that passes only
   by reducing the Route, bypassing target authentication, or reusing forbidden
   state.

## Open mechanism questions

- whether the three-position data route passes hostile information-flow review or
  needs one additional interior position;
- how introduction material is delivered without giving one Node a stable
  target-to-entry mapping;
- how many prepared entry, introduction, and recovery choices fit the idle and
  endpoint-resource budgets;
- how a replacement leg attaches to the same Service Connection without exposing
  a stable network identifier or accepting replay;
- how R-011 operator-diversity evidence affects route selection without creating
  a User graph;
- which descriptor-distribution family can implement R-003 Private Resolution
  and the R-009 hostile bootstrap result.

## Recommendation

Advance **Option C, Ardents split-leg rendezvous**, to an information-flow model
and bounded prototype. Prototype **Option B** beside it as the strongest
performance/recovery alternative. Keep **Option A** as the mature security-flow
reference and reject **Option D** for the baseline unless R-005 later creates a
stronger Route Profile.

This recommendation is reversible. It selects what to test next, not what Ardents
will ship.

## Disposition

- State: `active`; no Product Owner architecture decision has been accepted.
- Tor, I2P, Nym, Session, Lokinet, libp2p, and Waku are references or component
  sources, not selected dependencies.
- No route length, library, cryptography, DHT, wire protocol, language, or
  production mechanism is selected.
- No ADR and no production code.
