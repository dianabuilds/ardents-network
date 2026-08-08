---
id: R-004
title: Which routing and rendezvous family should carry the Interactive Route?
status: decided
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

**Scope clarification, accepted 2026-08-08:** `decided` means that the
Tor-shaped five-position split-circuit family is the first Carrier Lab candidate,
not that production routing is frozen. The durable Product Core fixes the
knowledge-separation claim and stable Application Interface. Five positions,
separate Introduction, four Role Domains, Candidate View, and source exclusions
remain a coherent candidate package to falsify; failure reopens that package
before later-horizon systems are built.

## Accepted gate

A candidate must preserve all of the following together:

- both endpoint locations remain hidden from the opposite endpoint;
- one ordinary Node acting only from its role-local view, with no endpoint,
  second observation position, or active probe source under that adversary,
  receives no direct endpoint-origin-to-opposite-endpoint/Name/Target binding;
- each ordinary Node receives only adjacent and role-specific information,
  short-lived opaque handles, timing, direction, and volume;
- Application Data and exact-target authentication remain end-to-end protected
  even when every carrier Node colludes;
- a direct path, one trusted proxy, or weaker fallback never substitutes for the
  Interactive Route;
- Broad Traffic Observer and arbitrary-collusion resistance are explicit
  non-claims, not hidden goals that silently add cover-traffic cost;
- Node-plus-endpoint active timing/volume confirmation is also an explicit
  low-latency non-claim and mandatory characterization cell;
- a warm exact-name connection has `p95 <= 1 s`, a cold one has `p95 <= 3 s`,
  and one eligible route failure resumes the same Service Connection within
  `p95 <= 5 s` or terminates explicitly by `15 s`;
- the required client sustains the accepted goodput, concurrency, CPU, memory,
  queue, and traffic-overhead budgets on ordinary Windows and Ubuntu devices;
- a stronger future Route Profile can replace the routing family beneath the
  same Application Interface and Service Connection without rewriting naming,
  identity, Service publication, or Applications.

These are conjunctive gates. Familiar vocabulary such as onion, garlic, mix,
DHT, or peer-to-peer is not evidence that a candidate passes.

## P5-D1 — Strengthening without an Application rewrite

**Product Owner decision, accepted 2026-08-08:** the routing family is a
replaceable Implementation behind one deep Route Module Interface. The first
Interactive Route candidate is an Adapter, not the permanent topology of the
whole product.

The stable layering is:

```text
Application
    -> Application Interface
        -> Service Connection
            -> Route Module (exact Route Profile + Isolation Context)
                -> Route Adapter
                    -> replaceable Carrier Channel Adapters
```

The Service Connection owns the authenticated Service Target, reliable ordered
byte-stream semantics, connection identity, bounded recovery, duplicate
suppression, and Application-visible failure. The Route Module receives only the
connection-scoped protected frames, exact Route Profile, Isolation Context, and
authenticated reachability material needed to attach or replace a Route. It
returns a usable protected attachment or an explicit route failure.

The Route Module Interface does not expose or freeze:

- hop count or path directionality;
- Introduction, Rendezvous, tunnel-pool, or mix roles;
- multipath, padding, fixed packetization, delay, or cover traffic;
- concrete HTTP, WSS, TCP, UDP, QUIC, or other Carrier Channel Adapters;
- route retry schedules, peer selection, prepared state, or descriptor lookup.

A Route Profile is a small versioned product contract, not a universal routing
language and not a collection of user-tunable anonymity knobs. Its exact identity
and capability set are authenticated into the endpoint session. Unsupported
profiles fail explicitly; negotiation cannot silently select a weaker profile.
Service Descriptors and bootstrap state advertise extensible versioned
capabilities rather than encoding an implementation-specific topology.

All reusable route and session state is keyed and isolated by Route Profile and
Isolation Context. Every distinct Route Implementation and profile must earn its
own Route Qualification; the baseline cannot lend evidence to a stronger
profile. Adding that profile may require internal routing work, descriptor
capability extensions, protocol evolution, and new evidence, but not a rewrite
of Applications, Service Names, Service Targets, authority custody, or
publication workflows.

This Seam is justified now because the research already contains independent
Adapters with materially different shapes: split-leg rendezvous, paired tunnel
pools, and a possible delayed mixnet. The prototype must demonstrate at least
the first two through the same Route Module test harness. It must not build a
general routing virtual machine in anticipation of unknown designs.

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

The initial minimum hypothesis used the accepted endpoint-link claim to design a
smaller route rather than copy a reference network's full path:

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

The initial C-3 data-path views are deliberately narrow:

- User Entry knows User location and Rendezvous, but no Service destination;
- Service Entry knows Service location and Rendezvous, but no User origin;
- Rendezvous knows both entry Nodes and one-use handles, but no endpoint
  location, Service Name, Service Target, or plaintext;
- introduction delivery cannot reveal both an endpoint location and descriptor
  identity to one ordinary Node;
- route recovery replaces one or both legs, or the Rendezvous, beneath the same
  endpoint-authenticated Service Connection.

### Advantages

- it is the minimum endpoint-link separation and therefore a useful unqualified
  performance lower-bound control;
- one joined bidirectional route avoids the four unidirectional tunnel pools of
  Option B, though a qualifying position count may erase any advantage over
  Tor-shaped circuits;
- prepared endpoint legs and Service introduction state may optimize the warm
  path;
- Bridge entry replaces the User Entry transport without changing the security
  profile or exposing a direct path;
- rendezvous state is per-connection, short-lived, and horizontally distributable.

### Costs and risks

- this exact construction is not inherited from a deployed protocol and must earn
  every security claim through information-flow tests and a prototype;
- because the Rendezvous sees both Entry Nodes and itself, the three-position
  drawing gives one ordinary Node the identities of every carrier Node on the
  data Route. That contradicts the accepted no-full-Route disclosure boundary
  even though endpoint locations remain separated;
- the introduction path is the subtle part: it must not give User Entry or
  Rendezvous roles the Service Target, give any Introduction role an endpoint
  origin, or create a Target-to-origin mapping at one ordinary Node;
- the minimum C-3 shape makes unknown common control and poor role diversity more
  consequential than in longer routes;
- the Rendezvous is a per-connection availability and traffic-analysis point;
- same-connection recovery needs a new authenticated route-attachment mechanism
  that prevents replay, rollback, stream duplication, and cross-context linkage.

### Assessment

Endpoint-link fit: **strong**. Full-Route disclosure fit: **fails as drawn**.
Expected performance fit: **best lower-bound control**. The split-leg family may
advance only with enough interior separation that no ordinary Node knows every
carrier position. Retain Option B as the alternative and Option A as the mature
security reference.

### Split-leg selection and introduction refinement

The mature references constrain the design without selecting their complete
architectures:

- Tor has the client choose the Rendezvous Point, the Service choose several
  Introduction Points, and the Service build its own path to the client-chosen
  rendezvous. The sealed introduction contains the rendezvous location, one-use
  cookie, and the first endpoint-handshake material. The Service rejects replay,
  then completes an endpoint-authenticated handshake through the rendezvous.
- Tor uses a small persistent guard set because choosing a fresh endpoint-adjacent
  Node for every path eventually exposes endpoints to more malicious candidates.
- Tor Vanguards extends that principle to one or two rolling interior layers for
  onion-service activity because repeated attacker-triggered circuits can expose
  the endpoint guard. The layers deliberately rotate at different rates.
- I2P treats service reachability as sensitive state: LeaseSets are published and
  retrieved through tunnels, and the encrypted form blinds the Destination from
  storage Nodes. Its reusable tunnel pools remain the Option B alternative, not
  an implicit dependency of Option C.

Sources: [Tor protocol overview](https://spec.torproject.org/rend-spec/protocol-overview.html),
[Tor introduction protocol](https://spec.torproject.org/rend-spec/introduction-protocol.html),
[Tor rendezvous protocol](https://spec.torproject.org/rend-spec/rendezvous-protocol.html),
[Tor guard specification](https://spec.torproject.org/guard-spec/),
[Tor Vanguards specification](https://spec.torproject.org/vanguards-spec/),
[Tor network parameters](https://spec.torproject.org/param-spec), and
[I2P network database](https://i2p.net/en/docs/overview/network-database/).

#### P5-D5 — Endpoint-owned layered rotation

**Product Owner decision, accepted 2026-08-08:** each endpoint selects its own
leg. Entry uses a small long-lived set, Interior a small medium-lived rolling
set, and Rendezvous is fresh and scoped to one Service Connection. Introduction
roles rotate as a finite overlapping set. Numeric set sizes, durations, weights,
and replacement thresholds remain evidence-driven protocol parameters rather
than user-tunable anonymity controls.

| Position | Selector | Accepted lifetime | Required constraint |
|---|---|---|---|
| User Entry or Bridge | User endpoint | At most one small long-lived set for each activated Initiator Role Domain × ordinary/Bridge regime in an installation | Creating Applications, Services, Targets, generations, Isolation Contexts, destinations, or Bridge Invites cannot create another set or force fresh Entry sampling. Each Entry/Bridge key is eligible for one adjacent domain. Per-context channels, keys, Interiors, destinations, sessions, and recovery state remain separate. |
| User Interior | User endpoint | Small medium-lived rolling set in the same Isolation Context | Rotates more often than Entry but is not sampled afresh for every connection or retry. |
| Rendezvous | User endpoint | Fresh for a new Service Connection; retained only while that attempt or connection is alive | Bounded Introduction retry or one-leg repair may retain a live Rendezvous. A failed Rendezvous is replaced by another fresh candidate bound to the same connection; none is pooled across completed connections. |
| Introduction Node | Service endpoint advertises a finite set; User chooses one | Shorter-lived overlapping rotating set | Shared by many Services; holds only an opaque expiring introduction slot and bounded forwarding state. |
| Introduction-side Service Entry and Interior | Service endpoint | At most one long-lived Introduction-domain Entry Set per activated ordinary/Bridge regime plus medium-lived Interiors | Carries introduction control traffic without exposing the Service origin to the Introduction Node. It cannot reuse the Responder-domain Entry Set, including when client and Publisher roles share a host. |
| Data Service Entry and Interior | Service endpoint after it learns the proposed Rendezvous | At most one long-lived Responder-domain Entry Set per activated ordinary/Bridge regime plus medium-lived Interiors | The User never chooses or learns them; the Service checks only authenticated Candidate View eligibility, Role Domain, expiry, and replay before attaching. Services, Targets, and Instance generations do not multiply the set. |

An intermediate Node never chooses an endpoint-adjacent position for an endpoint.
The User validates every overlap it can see before sending an invitation. The
Service checks the proposed Rendezvous against the same authenticated Candidate
View and disjoint Role Domain rule, never against a hidden Entry or Introduction
set in a way that changes an observable result. R-011 must handle operator,
network, software, and jurisdiction overlap that different Node IDs cannot reveal.

One failure tries another already eligible member or returns a bounded failure;
it does not evict an Entry into an attacker-driven sequence of new candidates.
Entry replacement requires lifecycle expiry, authenticated ineligibility, or a
bounded sustained-unavailability rule. If the required distinct eligible roles
cannot be selected, the endpoint returns Route unavailable rather than reusing a
forbidden position, shortening the Route, or borrowing another Isolation
Context's state.

Fresh Rendezvous means new connection-bound state and an independent eligible
selection. It does not claim that random selection can never choose a Node ID
seen on an earlier completed connection by chance.

This contract intentionally adopts the ordering of lifetimes, not Tor's current
numbers. Tor's primary-guard, guard-lifetime, vanguard, and Introduction Point
parameters are reference inputs for experiments on a much larger deployed
network; Ardents must derive its numeric values from R-011 concentration evidence
and R-023 endpoint, latency, availability, and idle-cost budgets.

#### P5-D6 — Disjoint Role Domains

**Architecture closure decision, accepted 2026-08-08:** the five-position rule
cannot be enforced across independently hidden endpoint legs by ordinary
per-attempt comparison. A Service that rejects a User-proposed Rendezvous based
on overlap with its hidden Entry Set would also create a repeatable
Entry-discovery oracle.

Eligible Node identities are therefore partitioned for a stable assignment
lifetime into four non-overlapping exposure classes: Initiator Carrier,
Rendezvous, Responder Carrier, and Introduction. One Node Identity and one
honestly declared operator family occupy only one Role Domain during that
lifetime. The User selects two distinct Initiator-domain identities and one
Rendezvous-domain identity; the Service selects two distinct Responder-domain
identities. Both endpoints can enforce local repetition rules, while domain
separation makes cross-leg Node-ID collision impossible without revealing either
hidden leg.

Role assignment is deterministic from the authenticated Network Epoch,
precommitted Node/family material, and public randomness that neither one Node
nor one epoch signer can cheaply grind after seeing the outcome. It is not a
manual allowlist, a per-Route Node choice, or proof of independent ownership.
Hidden common control and Sybil identities remain R-011/R-010 limitations. If a
domain lacks sufficient eligible capacity, the Route is unavailable rather than
shortened or built through an overlap. The concrete randomness and resistance to
identity grinding remain R-013 evidence work.

Assignment is finite and cannot flip at an epoch boundary while old duty lives.
Before selecting or renewing an Entry, Introduction slot, Resolution role, or
other duty, the endpoint proves that the duty's maximum terminal lifetime plus
required drain fits before assignment `not-after`. Reassignment publishes a
finite stop-new-work boundary, then holds the identity and every known-family
identity in old-domain drain/quarantine until all prior duties terminate. Only
then may any become eligible in the new domain. Emergency may terminate old work
immediately with an honest unavailable result, but never permits overlap.

Destination-aware Name/Target/descriptor lookup and publication do not create a
fifth global domain. They use a Destination Resolution role restricted to
Rendezvous-domain identities, never Initiator, Responder, or Introduction. For
one exact destination and Isolation Context, the endpoint excludes every
resolution identity and known family it used from that connection's Rendezvous
selection. Resolution also uses a separately isolated private path that hides the
query from Entry and has bounded retries. Public concentration/capacity gates
apply to the subrole itself and to the remaining Rendezvous pool after exclusion.
This prevents one valid identity/family from receiving endpoint-adjacent origin
and destination-aware lookup in the same operation without fragmenting operators
across a fifth domain.

A globally advertised direct-origin bootstrap, materialization, time, or update
source identity and known family are likewise ineligible for every Route
position and Destination Resolution during the assignment. If an ordinary
candidate distributes bytes directly, the contacting Endpoint adds its
authenticated identity/known family to the bounded installation-wide Direct
Source Exposure Set and locally excludes it until the source exposure and every
derived state/work lifetime terminate. Pre-contact selection rejects overlap
with retained Entry/Interior/Introduction/prepared-role state or live work;
candidate sequences, retries, and exposure growth are finite, and exhaustion is
explicit unavailability.

All public domain-capacity gates use the **effective** pool after the maximum
union of own-family, Direct Source Exposure, exact resolver, drain/quarantine,
and other mandatory local exclusions. Beta/stable must retain three/five
families per domain or required subrole with no family above `40%`/`25%` of
effective capacity. If `x_d` is that maximum distinct excluded-family union for
domain `d`, the Route-family floor is at least `Σ_d(3+x_d)` for beta or
`Σ_d(5+x_d)` for stable, and capacity may require more. The older `12`/`20`
figures are only all-zero-exclusion four-domain Route floors. Three/five
authenticated source-only families are additional, so `15`/`25` are only the
all-zero-exclusion theoretical total infrastructure floors, not launch targets.

This follows the risk exposed by Tor's current path-restriction work: an onion
Service's observable refusal of particular Rendezvous candidates can reveal its
Guards over repeated attempts. See
[Tor Proposal 354](https://spec.torproject.org/proposals/354-relaxed-restrictions.html).

#### P5-D7 — Authenticated Candidate View and local selection

**Architecture closure decision, accepted 2026-08-08:** a Node publishes a
signed expiring Node Record into a precommitted permissionless publication
window and append-only transparency input, but publication alone grants no
eligibility, trust, or Route weight. The threshold-authenticated Network Epoch
commits one logical complete, canonically ordered Candidate View: root, length,
publication cutoff/input-log root, global eligible count/capacity/concentration
summaries, and the deterministic rules for eligibility, Role Domain, probation,
reachability, protocol compatibility, synthetic capacity evidence, and bounded
weighting. Every valid pre-cutoff record is included or receives a publicly
verifiable deterministic rejection/revocation reason. A captured threshold may
still deny or fork the whole input log; transparency detects accountable
omission rather than making governance capture impossible.

An ordinary endpoint need not download the complete View. It fetches
deterministic Candidate Materializations—indexed records/shards and inclusion
proofs—and verifies the requested material, committed selection indices, and
eligibility locally. It does not claim global completeness from a partial
sample. Independent full auditors recompute the View, input-log inclusion,
global summaries, and concentration gates.

Every endpoint selects its own positions locally. An epoch signer, distributor,
bootstrap peer, Service, Rendezvous, or other Node never chooses a User's
complete Route. If one selected record is withheld, the endpoint retries the
same committed index at another source or fails explicitly; it never resamples
silently. Fetches are batched independently of a destination and no distributor
learns the complete selected Route. Local observations may avoid a failed Node
only inside that endpoint's bounded state; Ardents uploads no User route history
or reputation graph. Exact log convergence, sampling, and audit mechanisms remain
R-013 feasibility work.

#### P5-D8 — Non-oracular diversity

**Architecture closure decision, accepted 2026-08-08:** Node identity, Role
Domain, and a known operator family are hard constraints. ASN, prefix, hosting
provider, jurisdiction, build provenance, and undeclared ownership are risk and
concentration evidence, not proof of independence. Unknown means uncertain, not
independent.

No externally observable acceptance result, error detail, retry count, or
deliberate timing distinction may depend on whether a proposed role intersects a
hidden Entry or Interior set. Each endpoint constructs only its own leg under
the disjoint-domain rule. When concentration or missing capacity prevents a
qualified path, the endpoint reports generic Route unavailable without rotating
Entry merely to satisfy the attempt.

This is protocol knowledge separation, not traffic-confirmation resistance. An
endpoint-adjacent malicious Node that also controls/observes an endpoint or
active probe source may correlate a known Target through chosen timing/volume.
That combined adversary is outside the Interactive Route claim and must be
characterized in R-001/R-023 evidence rather than described as another protected
single-Node case.

#### P5-D9 — Endpoint-only connection continuity

**Architecture closure decision, accepted 2026-08-08:** the initial
target-authenticated endpoint handshake creates a connection-only continuity
secret held solely by User and Service. A repaired leg or fresh Rendezvous uses
fresh random handles, route keys, and a monotonic route generation and proves
possession of that secret bound to the exact Target, Route Profile, Isolation
Context, and original handshake transcript.

Replayed, rolled-back, cross-target, cross-profile, and cross-context attachment
is rejected. Endpoints reconcile authenticated byte sequence and acknowledgement
ranges and may overlap old and new paths only for safe cutover; no Application
byte is presented twice. Relays receive no stable connection identifier. Timing
may still correlate repair with the old Route, which remains an explicit
low-latency limitation. Rendezvous loss uses a fresh sealed Introduction attempt
and shares the existing non-resetting recovery deadline rather than opening a
new Application-visible connection.

#### C0 — Service Entry is also the Introduction Node

```text
User -> User Entry -> Rendezvous -> Service Entry -> Service
```

The descriptor directs the invitation to the same Service Entry that reaches the
Service. This is the cheapest construction, but the Service-adjacent Node holds a
service-specific inbound handle while also observing the Service origin. Active
probing and timing can turn that into a Target-to-origin mapping. **Reject C0**:
the combined role is too close to the exact forbidden link.

#### C1 — Rendezvous-forwarded sealed introduction

Preparation:

```text
Service
  -> Introduction Entry
  -> Introduction Interior
  -> Introduction Node [opaque expiring slot]
```

Connection attempt:

```text
User -> User Entry -> User Interior -> Rendezvous -> Introduction Node
Introduction Node -> Introduction Interior -> Introduction Entry -> Service
Service -> Data Service Entry -> Service Interior -> Rendezvous
```

Joined data path:

```text
User
  -> User Entry
  -> User Interior
  -> Rendezvous
  -> Service Interior
  -> Data Service Entry
  -> Service
```

The User first establishes a fresh random join handle at its chosen Rendezvous.
It then sends a fixed-shape, randomized invitation through that Rendezvous to one
Introduction Node from the authenticated Service Descriptor. Among carrier
roles, only the Introduction Node uses the routable opaque slot; only the Service
can open the invitation. The sealed body binds:

- the exact Route Profile and compatible protocol capability set;
- the Rendezvous identity and reachability needed by the Service;
- a fresh single-use join handle and finite expiry;
- the User's ephemeral half of the endpoint handshake;
- transcript context that prevents substitution across Target, profile, network,
  or connection attempt.

Invitation and Route state are allocated inside the local Isolation Context and
cannot be reused by another context, but the Isolation Context itself is never
placed into the invitation or transmitted as a network identifier.

The Introduction Node forwards the opaque body over the Service's prepared
Introduction path. It cannot alter the proposed Rendezvous or handshake without
endpoint rejection. The Service checks expiry, replay, authenticated Candidate
View eligibility, and Role Domain, but no hidden local-set intersection changes
the externally observable result. It selects its own Data Service Entry and
attaches using a separate random join handle. The Rendezvous pairs the two
handles but receives no Service Name, Service Target, endpoint key, or plaintext.
The Application sees success only after the endpoints authenticate the exact
Service Target and Route Profile end to end.

The intended protocol-derived views are:

| Role | Receives | Must not receive from its role |
|---|---|---|
| User Entry | User origin, User Interior, timing, volume | Rendezvous, Introduction Node, Name, Target, Service origin |
| User Interior | User Entry, Rendezvous, opaque handles | User origin, Service side, Name, Target |
| Rendezvous | User Interior, Introduction Node during setup, Service Interior after attachment, random handles | Both Entries, either endpoint origin, descriptor, Name, Target, endpoint keys, plaintext |
| Introduction Node | Rendezvous, Introduction Interior, opaque slot, sealed invitation | Both Entries, either endpoint origin, plaintext invitation, Target-to-origin mapping |
| Introduction Interior | Introduction Node, Introduction Entry, opaque handles | Service origin, User side, invitation plaintext, Name, Target |
| Introduction Entry | Service origin, Introduction Interior, opaque channel state | User origin, Rendezvous proposal, Name, Target |
| Service Interior | Rendezvous, Data Service Entry, opaque handles | Service origin, User side, Name, Target |
| Data Service Entry | Service origin, Service Interior, random route handle | Rendezvous, User side, Name, Target, endpoint handshake plaintext |

C1 reuses the already prepared User-to-Rendezvous leg for introduction. Its
central risk is that the Rendezvous observes which shared Introduction Node
receives the invitation. That must remain a many-Service role and the randomized
invitation must not be comparable with descriptor bytes or another attempt.

P5-D4 rejects this role combination for the baseline: the selected Rendezvous
must not forward an invitation or receive the Introduction Node or slot from its
role. C1 remains only an explicitly unqualified setup-performance experiment. It
cannot be negotiated as, or silently substituted for, the Interactive Route.

#### C2 — Separate introduction route

```text
User -> User Entry -> User Interior -> Introduction Forwarder -> Introduction Node
Introduction Node -> Introduction Interior -> Introduction Entry -> Service
User -> User Entry -> User Interior -> Rendezvous
Service -> Data Service Entry -> Service Interior -> Rendezvous
```

**Product Owner decision P5-D4, accepted 2026-08-08:** C2 is the baseline
Introduction role architecture. The User delivers one sealed, expiring,
single-use invitation over a separate Introduction Path that does not traverse
the selected Rendezvous. The Service validates it, selects its own data leg, and
attaches to the proposed Rendezvous. Introduction carries no Application Data,
retained message, or offline-delivery semantics and ends for that attempt after
acknowledgement.

C2 prevents the Rendezvous from observing the selected Introduction Node and
keeps setup and data-path roles independent. The User may prepare both paths in
parallel and perform bounded retry through another advertised Introduction role
without rebuilding a still-valid Rendezvous leg. It costs another path, failure
domain, queue, and setup schedule, but that cost is confined to connection setup
rather than the joined data path. Exact path depth, preparation policy, overlap
rules beyond the accepted forbidden combinations, cryptography, and wire format
remain prototype questions.

#### C3 — Distributed invitation mailbox

The User stores a sealed invitation under a short-lived lookup token and the
Service polls anonymously. This removes a dedicated Introduction circuit and can
survive brief disconnection, but adds storage, polling traffic, lookup privacy,
Sybil/eclipse exposure, retention semantics, and new replay/DoS state. **Reject
C3 for the baseline**: it recreates a message-storage subsystem to establish a
live connection. It may return only if a later mechanism proves strictly cheaper
and retains no Application Data.

#### P5-D2 — Public Target knowledge at Introduction

An Introduction Node necessarily handles a service-specific opaque slot. Because
an Unlisted Service is public to anyone who knows or guesses its exact Service
Name, a malicious Node operator can independently act as a User, resolve that
name, and inspect the descriptor material available to every User. Tor's own
Introduction Points likewise hold a per-Service authentication key; encrypted
descriptors hide it from an uninformed directory, not from every party that knows
the service address.

**Product Owner decision, accepted 2026-08-08:** preserve the core guarantee that
one ordinary Node's role-local protocol view, without endpoint/second observation
or active probe control, receives no direct Service Name/Target-to-endpoint-origin
binding. Do not claim that an Introduction Node operator can never independently
learn which public Service its opaque slot serves, or that the operator cannot
actively confirm traffic when it also controls an endpoint/probe source.

Public Target knowledge alone is not an anonymity failure. The Introduction role
receives neither endpoint origin and cannot combine that state with a
Service-adjacent Entry view for the same Service or connection attempt. It gains
no additional route state, authority, or protocol privilege from external
knowledge. A Target-to-origin link remains a qualification failure.
Direct protocol-state disclosure inside the role-local claim remains a
qualification failure; timing/volume confirmation by the combined adversary is
the separately measured non-claim.

This closes the contradiction without making Services invite-only, treating
names as secrets, or selecting an unproven cryptographic construction. P5-D4
subsequently selects C2's separate role architecture for the baseline and keeps
C1 only as an unqualified performance experiment; neither decision selects a
production protocol.

### Data-path position comparison

The accepted P2-D3 contract protects both the Target-to-origin link and the
identities of all participating Nodes as one complete Route view. Position count
must satisfy both. Tor is a useful control: its specification says anonymous
connections should use at least three relays, and its onion-service rendezvous
joins independently built client and Service circuits at a client-selected
Rendezvous. I2P independently documents the tradeoff: more tunnel peers increase
latency and premature-failure probability, while fewer make internal traffic
analysis easier; most applications use two or three hops per tunnel.

Sources: [Tor circuit creation](https://spec.torproject.org/tor-spec/creating-circuits.html),
[Tor rendezvous protocol](https://spec.torproject.org/rend-spec/rendezvous-protocol.html),
[I2P tunnel routing](https://i2p.net/en/docs/overview/tunnel-routing/), and
[I2P performance](https://i2p.net/en/docs/overview/performance/).

#### C-3 — Three carrier positions

```text
User -> User Entry -> Rendezvous -> Service Entry -> Service
```

The shape meets the one-Node endpoint-location separation: Entry roles see only
their adjacent endpoint and Rendezvous, while the Rendezvous sees no endpoint
origin. It nevertheless fails the separate no-full-Route rule. The Rendezvous
sees User Entry, itself, and Service Entry: every carrier Node identity in this
Route. Accepting C-3 would require weakening P2-D3 rather than implementing it.

Retain C-3 only as an explicitly unqualified performance lower-bound control. It
cannot become the Interactive Route merely because it is faster.

#### C-4 — One asymmetric interior position

```text
User -> User Entry -> User Interior -> Rendezvous -> Service Entry -> Service
```

The added Interior prevents the Rendezvous from knowing every carrier identity,
so C-4 passes the literal full-Route boundary. But it gives the User and Service
different position exposure and different deterministic collusion thresholds.
Placing the Interior on a random side merely makes one endpoint weaker on each
connection and adds a route-shape distinction; it does not create a coherent
stronger claim. Fixed or adaptive asymmetry has no accepted Application reason.

Reject C-4 for the baseline. The product promises reciprocal endpoint-location
privacy and should not introduce an unexplained weaker leg only to save one Node.

#### C-5 — Symmetric interior separation

```text
User
  -> User Entry
  -> User Interior
  -> Rendezvous
  -> Service Interior
  -> Service Entry
  -> Service
```

Each endpoint selects its Entry and Interior leg independently; the User selects
the fresh Rendezvous. The Service validates that proposal against its own route
state before attaching. The protocol-derived data-path views become:

| Role | Immediate view | Deliberately absent |
|---|---|---|
| User Entry | User origin, User Interior | Rendezvous, Service side, Name, Target |
| User Interior | User Entry, Rendezvous | User origin, Service side, Name, Target |
| Rendezvous | User Interior, Service Interior | Both Entries, both endpoint origins, Name, Target |
| Service Interior | Rendezvous, Service Entry | Service origin, User side, Name, Target |
| Service Entry | Service Interior, Service origin | Rendezvous, User side, Name, Target |

No ordinary Node receives every carrier identity, either endpoint origin, or a
Target-to-origin link. The Rendezvous no longer learns the endpoints' sticky Entry
sets. C-5 therefore satisfies a security boundary that C-3 fails and avoids the
unexplained asymmetry of C-4.

The same separation must apply to introduction preparation:

```text
Service -> Introduction Entry -> Introduction Interior -> Introduction Node
```

and to the accepted P5-D4 attempt:

```text
User -> User Entry -> User Interior -> Introduction Forwarder -> Introduction Node
Introduction Node -> Introduction Interior -> Introduction Entry -> Service
User -> User Entry -> User Interior -> Rendezvous
Service -> Data Entry -> Service Interior -> Rendezvous
```

The normal joined path still carries no Introduction role. Introduction and data
Entries and Interiors are separately selected for the attempt unless a later
information-flow proof demonstrates that bounded reuse cannot combine forbidden
views.

#### Topological cost before measurement

| Shape | Carrier Nodes | Endpoint/inter-Node segments | Relay processing relative to C-3 | Segment transmissions relative to C-3 | Contract result |
|---|---:|---:|---:|---:|---|
| C-3 | 3 | 4 | `1.00x` | `1.00x` | Fails no-full-Route rule |
| C-4 | 4 | 5 | `1.33x` | `1.25x` | Asymmetric; reject |
| C-5 | 5 | 6 | `1.67x` | `1.50x` | Smallest symmetric candidate |

These ratios are topology counts for one carried unit, not performance results.
They exclude framing, acknowledgements, retransmission, introduction, prepared
state, cryptography, and recovery. C-5 adds one route extension per endpoint leg;
prepared Entry-to-Interior legs may reduce warm setup time, but their idle bytes,
memory, rotation, and failure cost remain inside the accepted budgets.

C-5 does **not** gain a claim against every colluding pair or a Broad Traffic
Observer. Low-latency timing correlation remains, and P2-D4 is unchanged. Its
measurable gain is narrower: no single carrier gets the complete Node sequence,
the Rendezvous does not directly learn either sticky Entry, and both endpoint
legs have the same knowledge structure.

#### P5-D3 — Symmetric five-position baseline

**Product Owner decision, accepted 2026-08-08:** the baseline Interactive Route
uses the symmetric C-5 logical data path:

`User -> User Entry -> User Interior -> Rendezvous -> Service Interior -> Service Entry -> Service`.

Each position is assigned to a different ordinary Node for that attempt. C-3
remains only an explicitly unqualified performance control because its
Rendezvous sees every carrier Node. C-4 is rejected because it protects the two
endpoint legs asymmetrically. A shorter path is not a transparent optimization;
it is an unqualified profile or downgrade and must never inherit the Interactive
Route claim.

Advance **C-5** as the smallest split-leg candidate that implements the already
accepted claim without asymmetry. Reject C-3 as privacy-qualified architecture
and C-4 as baseline shape. This is not a decision to copy Tor: Ardents derives
five positions from its own disclosure and symmetry contracts and still uses its
own Service Connection, Introduction, recovery, naming, bootstrap, and Route
Module boundaries.

C-5 nevertheless converges to the logical data-path shape of two anonymous
three-relay circuits sharing a Rendezvous. The split-leg hypothesis therefore no
longer has a proven data-path position or forwarding-cost advantage over the Tor
security reference. Its remaining possible advantages are C1's reused
Rendezvous-forwarded setup control, same-connection route replacement, explicit
Route Module boundaries, and replaceable Carrier Channel Adapters. P5-D4 rejects
the first advantage for the baseline; only the remaining differences may justify
implementation work.

Those differences do not justify a new routing protocol by themselves. R-013
must prefer a maintained mature implementation whenever it can satisfy the same
Interface, recovery, transport-agility, and qualification contract. A custom
Implementation remains justified only by measured security, performance, or
operability evidence that the adopted alternative cannot provide.

P5-D3 selects an information-flow shape, not a production mechanism. It does not
select Tor, onion construction, a library, cryptography, wire protocol, or
language, and it adds no claim against every colluding pair or a Broad Traffic
Observer. A bounded prototype must compare the five-position
shape with Option B under the same tracer; C-3 may appear only as a clearly
unqualified performance control. If no implementation can meet the accepted
security and performance contracts, the product contracts return to review; the
route is not silently shortened.

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
| One-Node knowledge separation | Strong reference | Strong if pool roles remain separate | C-3 fails full-Route rule; C-5 is the smallest symmetric fit | Stronger than required |
| Warm `p95 <= 1 s` hypothesis | Uncertain; circuits may be prepared | Promising with prepared pools | Uncertain; separate Introduction and Rendezvous paths may be prepared in parallel | Poor |
| `10 Mbit/s` and overhead hypothesis | Five-position data path; unmeasured | Medium to high scheduling cost | Same five-position lower bound as A; unmeasured | Poor |
| Same-connection `p95 <= 5 s` recovery | Requires added continuity | Natural path alternatives, complex ordering | Requires added leg attachment | Requires stream reconstruction |
| Endpoint state | Medium-high | Highest | Medium for C-5 | High |
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
3. run C-5 and Option B with the smallest viable tunnel pool on the same
   controlled topology through the same Route Module Interface and Service
   Connection tracer; C-3 may run only as an unqualified performance control;
4. measure cold and warm connection setup, one-stream goodput, `64`/`256`
   connection workloads, CPU, RSS, queues, and endpoint traffic overhead;
5. inject one leg, Rendezvous, Introduction, Carrier Channel, and descriptor-path
   failure and test the accepted `5 s`/`15 s` outcomes;
6. test replay, target substitution, route downgrade, timing and loss tagging,
   descriptor enumeration, forbidden pool reuse, Isolation Context crossover,
   repeated forced failure, Entry-churn bounds, medium-layer rotation, fresh
   Rendezvous selection, and overlapping Introduction rotation;
7. retain all failed runs as evidence and reject any candidate that passes only
   by reducing the Route, bypassing target authentication, or reusing forbidden
   state.

## Remaining implementation evidence

- whether the accepted Tor-shaped C-5 family passes hostile information-flow
  review and the accepted performance budgets with prepared symmetric legs;
- which maintained construction implements the separate Introduction Path
  without a stable target-to-entry mapping;
- what Entry, Interior, Introduction, and recovery set sizes, randomized
  lifetimes, and replacement thresholds fit the P5-D5 exposure boundary and the
  idle, latency, availability, and endpoint-resource budgets;
- which cryptographic and wire construction implements P5-D9 continuity without
  exposing a stable network identifier or accepting replay;
- what deterministic eligibility, Role Domain assignment, probation, evidence,
  and concentration parameters implement P5-D6 through P5-D8;
- which descriptor-distribution family can implement R-003 Private Resolution
  and the R-009 hostile bootstrap result.

## Recommendation

Select the **Tor-shaped pair of independently built endpoint circuits joined at
the User-selected Rendezvous** as the first Carrier Lab routing candidate to
evaluate.
Apply Ardents' C-5 positions, P5-D4 separate Introduction, P5-D5 lifetimes,
P5-D6 Role Domains, P5-D7 local selection, P5-D8 diversity boundary, and P5-D9
connection continuity. This selects a family, not Tor naming, `.onion`, exit
routing, a library, cryptography, wire protocol, or implementation language.

Option B tunnel pools may remain a bounded falsification prototype if the
Tor-shaped candidate cannot approach the recovery or warm-latency budgets, but
it is no longer an equal first candidate. C1 and C-3 are explicitly unqualified
performance controls, C-4 is rejected, and Option D is outside the current
Interactive Route research.

Option C is only the first candidate Route Adapter. Its internal position count
must not leak into the Application Interface, Service Connection contract,
Service Name, Service Target, or authority model.

The prototype recommendation is reversible. P5-D3 through P5-D9 define the
current candidate package, while the Product Core fixes its information-flow
claim. Evidence may return the package to explicit review, but a candidate
cannot silently weaken the claim in order to pass.

## Disposition

- State: `decided`; P5-D1 fixes the strengthening Seam, P5-D2 fixes the
  Introduction-role knowledge boundary, and P5-D3 fixes the baseline symmetric
  five-position logical data path. P5-D4 fixes a separate Introduction Path and
  forbids the selected Rendezvous from forwarding invitations. P5-D5 fixes
  endpoint-owned selection, long-lived Entry, medium-lived Interior,
  connection-scoped Rendezvous, and overlapping Introduction rotation. P5-D6
  through P5-D9 close hidden-leg distinctness, Candidate View authority,
  non-oracular diversity, and endpoint-only continuity. The first Carrier Lab
  candidate is Tor-shaped split circuits; no production family is selected. Its
  concrete implementation remains R-013 and its qualification remains
  R-001/R-023.
- P5-D6 also separates direct-origin distribution from Route/Resolution duty,
  applies the installation-wide source-exposure exclusion, and evaluates public
  family/capacity thresholds only after the profile's maximum exclusion union.
- Tor, I2P, Nym, Session, Lokinet, libp2p, and Waku are references or component
  sources, not selected dependencies.
- No concrete introduction protocol, library, cryptography, DHT, wire protocol,
  language, or production mechanism is selected.
- The Role Domain trade-off is recorded in ADR-0005. No production code.
