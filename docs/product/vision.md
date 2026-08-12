# Product vision

Status: **accepted long-term product contract; current horizon is bounded H3 research**

Last reviewed: 2026-08-11

The complete installation, readiness, operation, update, and withdrawal contract
is defined in [the product operating model](operating-model.md). What may be
built now is controlled separately by
[product scope and delivery horizons](scope.md).

## Vision

Ardents is an internal application network for reaching services without making
their ordinary network locations part of the application relationship. A User
runs an Ardents endpoint; a Developer connects an existing site or application
service; independently operated Nodes carry live Application Data between them.

The network is the product. A site, messenger, file exchange, community, or
identity system is an Application or optional Overlay Service built on it.
Ardents does not need to understand those application semantics in order to
route and protect their connections.

## Delivery horizon

The vision describes the eventual public product, not one immediate release.
Carrier Lab and the controlled Named Unlisted Site tracer have completed their
bounded gates. Horizon 3 now permits research for one project-controlled Closed
Test Network vertical slice at a time; no slice enters implementation without a
frozen record and explicit Product Owner promotion. Public contribution,
Bridges, permissionless naming, production updates, multiparty governance,
Windows qualification, and complete Route Qualification remain separate later
gates. A fixed long-term requirement is not automatically a current backlog
item, and the ambiguous planning label `V1` is no longer used for the next
implementation.

## What the product adjectives mean

- **Private** means Service Connections protect Application Data and expose only
  the metadata allowed by a declared Route Profile. It does not mean every
  Service is access-controlled.
- **Anonymous** is never a blanket claim. The Interactive Route aims to hide the
  User's ordinary location from the Service, the Service Instance's ordinary
  location from the User, and prevent one ordinary Node acting only from its
  role-local view from directly receiving an endpoint-origin-to-Name/Target
  binding. It does not resist timing/volume confirmation by a Node that also
  controls an endpoint/probe source, a Broad Traffic Observer, or identity
  disclosed by Application Data and behavior.
  An implementation may present this claim only after Route Qualification;
  Ardents research or an unqualified candidate is not an anonymous network.
- **Decentralized** means ordinary reachability and routing do not require one
  hosting or relay operator. Naming, bootstrap, releases, and emergency powers
  remain explicit Control Plane risks; their product boundaries are accepted,
  but public decentralization is not claimed until independent operators,
  custodians, auditors, and builders actually satisfy the launch gates.

## Security and performance

Security and performance are equal product constraints. Every privacy,
isolation, and abuse-control mechanism must state and measure its connection
latency, throughput, tail behavior, CPU, memory, and overload cost on declared
device and network classes. An optimization cannot bypass a security invariant,
and a security mechanism is not viable if it makes the accepted Application
journey impractical under honest load.

Performance floors are not hardware ceilings. A stronger endpoint may scale
finite hierarchical local budgets above the applicable qualified public floor, while the Endpoint Owner
retains a cap. Extra capacity grants no infrastructure role, trust, authority,
route priority, or privacy exception, and exact hardware capacity is not required
network metadata.

## Long-term fixed direction

The following product choices already constrain research:

1. Ardents is a public carrier for internal Services, not a membership-gated
   private network and not a general clearnet exit.
2. A Developer can expose a site or application through Ardents without
   publishing a stable origin address to its Users.
3. An Unlisted Service is not indexed, but anyone knowing its exact
   human-readable Service Name may attempt to open it.
4. The network carries opaque Application Data. It does not impose messenger,
   Inbox, Contact, Space, file, or command semantics.
5. Node identities are infrastructure details. They are not User identities,
   Service addresses, or an application-level `from ID` / `to ID` model.
6. Censorship, probing, Sybil participation, malicious infrastructure, seizure,
   traffic analysis, and governance capture are normal operating conditions.
7. The first public product supports one active Service Instance generation per Service Target. Each
   host generates a private Instance Key; the durable Service Authority may
   remain offline and authorizes its public key with a bounded public Instance
   Credential. Routine migration generates a new key and advances the Instance
   generation without exporting an old runtime secret; Service Authority loss or
   compromise replaces the target through the stable Service Name.
8. Endpoint Location Privacy is distinct from Application anonymity. Ardents
   does not inspect, sanitize, or promise to unlink credentials, content,
   fingerprints, timing, or behavior visible to an intended endpoint.
9. Canonical Service Names have no administrative owner or dispute court.
   Naming abuse is constrained by anonymous resource cost and explicit protocol
   rules, not identity, payment, IP reputation, or discretionary reassignment.

## Public product boundary, not current implementation scope

The core product must let a local Application:

- start an Ardents endpoint on a Windows 11 or Ubuntu LTS `x86-64`
  desktop/laptop and join the carrier without a central user account;
- create or securely import a Service Authority and obtain its
  location-independent Service Target;
- expose a local Service Instance behind that target;
- claim a root Name Lease or receive a delegated subordinate Name Lease without
  approval from a central administrator, account provider, or registrar;
- renew it through a visible finite Grace period or observe explicit Release,
  without allowing old records or descendants to revive after reclaim;
- rotate or transfer Name Authority and optionally precommit a delayed threshold
  Recovery Policy without giving a registrar administrative recovery power;
- use a distinct Name Authority to bind an optional human-readable Service Name
  to the target without making that authority part of ordinary publication;
- share that destination as an explicit `ardents://` Service Link without
  publishing or querying an ordinary DNS name;
- share a machine-verifiable Target Link and connect without naming when the
  canonical Namespace is unavailable or intentionally unused, without silently
  converting a failed Name into a Target destination;
- resolve an exact Service Name in one canonical network-wide Namespace without
  a public directory or resolver-selected alternate meaning, while preventing
  any one ordinary Node acting only from its permitted role-local view from
  linking User location to the queried name;
- perform naming operations under bounded Anonymous Cost and local admission,
  inspect the governing rule version and incompatible fork state, and distinguish
  an explicit local filter from a change to the canonical Name Record;
- establish a protected live Service Connection;
- exchange opaque Application Data within measurable performance and resource
  budgets and receive explicit failure and closure;
- keep unrelated application contexts from being linked by accidental route
  reuse;
- recover an entry path when ordinary bootstrap or transport is blocked.

Those functions have independent Capability Readiness. A running process is not
`ready` merely because its local socket exists, and unavailable naming cannot be
hidden inside an otherwise working Target connection.

The same Public Beta endpoint platforms must support publishing an ordinary local
Application. Ubuntu LTS is the sole Linux qualification baseline; other Linux
distributions and architectures receive no Public Beta compatibility or release claim.
The required Public Beta infrastructure reference class is an Ubuntu LTS `x86-64`
server or VPS with `2 vCPU`, `2 GiB RAM`, and a symmetric `100 Mbit/s` link.
Each selected role must be useful on that class; it is neither a capacity
ceiling nor additional trust. macOS and mobile are later targets rather than
current performance or release promises.

The core does not, by default, execute untrusted application code, retain
application payloads for offline recipients, replicate a site's content, define
User identity, or decide application authorization. Any of those may later be a
separate Ardents Application or Overlay Service after its own product contract
is justified.

## Product surfaces

These are responsibility boundaries, not selected binaries or APIs:

- **Local Endpoint** — joins the network, builds routes, resolves Services, and
  enforces connection isolation for local Applications.
- **Application Interface** — a local socket/proxy-style boundary that lets
  external software create/listen on Service Targets and open/accept Service
  Connections without a mandatory Ardents SDK or knowledge of network internals.
  SDKs are convenience wrappers and never separate network implementations.
- **Service Publisher** — binds a local Service Instance to a Service Target and
  publishes authenticated reachability metadata.
- **Naming Surface** — uses a distinct Name Authority to claim or receive,
  renew, delegate, resolve, rotate, recover, and inspect time-bounded Service
  Names in one canonical Namespace without making it part of Service
  publication or becoming a service directory.
- **Contributor Node** — provides explicit, bounded entry, relay, discovery,
  rendezvous, or Bridge roles.
- **Network Transparency** — exposes Control Plane roots, software provenance,
  concentration, failures, and the limits of privacy claims.

## Long-term core product promises

1. A Service Connection is confidential and authenticated end to end against
   carrier Nodes acting alone or together, while its endpoints, Service
   Authority, and accepted cryptography remain uncompromised.
2. A Service Target is independent of a Node identity and ordinary IP location.
3. A complete Service Name has one verifiable network-wide meaning under
   authority distinct from Service Authority and can remain stable when a
   compromised or lost Service Target must be replaced. Its Public Beta canonical form is
   a lowercase ASCII dot hierarchy and its explicit Service Link identifies
   Ardents rather than DNS. Resolution separates User location from the name
   against one ordinary Node, but does not make predictable names secret.
4. Root Service Name control begins with the first valid claim in deterministic
   shared Namespace order and remains a renewable lease, not permanent property,
   a User account, or human identity. Grace is visible, Release stops resolution,
   and reclaim creates a new generation that invalidates prior state. Recovery
   exists only through a precommitted scoped policy and fails closed while pending.
5. No administrator, project, registrar, legal claimant, trademark process, or
   manual dispute panel can seize, block, transfer, or reassign a canonical Name
   Lease. Only a finite versioned set of Protocol-reserved Names may exist for
   parsing, compatibility, or protocol safety. Local filters are non-canonical.
6. Naming capacity uses bounded Anonymous Cost without money, a global account,
   identity document, IP reputation, stable identity, wallet, or token. It does
   not prove one person, fair allocation, legitimate use, or rightful control.
   Rules and transition evidence are inspectable, one operator cannot change
   canonical state, and an incompatible fork is explicit rather than silent.
7. The User and Service do not learn each other's ordinary network location
   within the declared Interactive Route conditions.
8. Route Knowledge Separation prevents one ordinary Node's role-local view from
   receiving the full Route, Application Data, or a direct binding between an
   endpoint's ordinary location and a Service Name, Service Target, or opposite
   endpoint. An
   Introduction role may independently know a public Service Name or Target
   available to any User, but receives neither endpoint origin and must not turn
   that knowledge into a Target-to-origin link.
9. A failed path produces bounded recovery or an explicit failure; the network
   does not silently pretend that an offline Service received data.
10. Losing or blocking one ordinary path does not require one central operator
   to restore connectivity.
11. Every security, anonymity, availability, and decentralization claim states
   its adversary, conditions, measurement, and limitation.
12. Every accepted security mechanism also has a performance budget and overload
   test; no performance optimization may silently weaken that mechanism.
13. A Local Traffic Observer receives no Service destination or plaintext from
    the protocol. Ardents does not promise to hide its own use, but avoids one
    mandatory stable fingerprint and treats Transport Camouflage as a measurable
    censorship-resistance goal.
14. The one-Node claim covers only a malicious ordinary Node's carrier role-local
    view. A Node that also controls/observes an endpoint or active probe source
    or combines a direct-origin observation may confirm a known Target through
    distinctive low-latency timing/volume;
    arbitrary Correlated Control may do the same. Both are explicit non-claims
    and do not imply Application Data decryption.
15. A malicious Service receives no User origin, Route, Isolation Context, or
    network-generated stable User identifier from Ardents, while a malicious
    User receives no Service Instance origin, Route, or Service Authority. Each
    still sees the Application Data and behavior intended for it.
16. Target authentication, Route Profile binding, protocol freshness, and
    integrity fail closed. Detected modification, injection, replay, redirect,
    or downgrade never becomes an accepted connection or Application Data.
    Fresh authenticated ephemeral session/leg keys provide Forward Secrecy for
    honestly completed connections against later Service/Node long-term-key
    compromise; live endpoint compromise and memory/snapshot remnants remain
    explicit limits.
17. The Interactive Route claim is implementation-gated. Reproducible
    endpoint-edge, Node-role, malicious-endpoint, Application Principal,
    Application-network isolation, Direct-Origin Source, Role Domain transition,
    and active-attack tests must pass before a candidate earns Route
    Qualification; the claim's conditions and excluded adversaries remain
    visible to Users and Developers.
18. The Application Interface and logical Service Connection remain stable when
    Ardents strengthens routing. A versioned Route Profile may replace route
    shape, introduction, rendezvous, multipath, padding, mixing, cover traffic,
    or Carrier Channel Adapters below that boundary. An unsupported exact
    profile fails explicitly; it is never silently negotiated to a weaker one,
    and every Implementation earns Route Qualification independently.
19. The first Carrier Lab candidate has five symmetric logical carrier
    positions: User Entry, User Interior, Rendezvous, Service Interior, and
    Service Entry. A shorter path is only an explicitly unqualified comparison
    control. This fixes the experiment, not the production Route shape.
20. In that candidate, connection introduction uses a separate Introduction
    Path, not the selected Rendezvous. It carries only a sealed, expiring,
    single-use invitation that lets the Service attach its own data leg; it
    carries no Application Data and creates no offline-delivery promise.
21. If the data-path candidate survives, the current later-horizon design has
    each endpoint select its own leg using a small long-lived Entry Set and a
    small medium-lived Interior Set. Public Beta has ordinary and Bridge regimes and at
    most one Entry Set for each activated adjacent Role Domain and regime per
    installation: Initiator for client traffic, Responder for publication data,
    and Introduction for prepared introduction paths. Co-resident roles retain
    separate domain sets; Applications, Services, contexts, destinations,
    generations, and Bridge Invites cannot force unlimited Entry sampling. Every
    Bridge key is eligible for one adjacent domain only. Contexts share no
    channel, key, Interior, Route, session, target, query, or recovery state.
    Entry is not rotated after one failure; a fresh User-selected Rendezvous is
    scoped to one new Service Connection, and Introduction roles rotate gradually
    with overlap.
22. The corresponding later-horizon candidate places Initiator, Rendezvous,
    Responder, and Introduction Node identities in disjoint stable Role Domains.
    This makes the five-distinct-position rule
    enforceable across independently hidden endpoint legs without turning
    Service rejection into an Entry-discovery oracle. Name, Target, and
    descriptor lookup/publication uses a Destination Resolution role restricted
    to the non-adjacent Rendezvous Domain; identities/families used for one exact
    destination/context are excluded from that connection's Rendezvous. This
    keeps four capacity domains while preventing one valid identity/family from
    combining endpoint Entry and destination-aware lookup.
    Assignment is finite: duty must fit before `not-after`; reassignment stops
    new work, drains and quarantines the old identity/family, and only then
    permits another domain. Emergency closes rather than overlaps duty.
23. The current public Control Plane candidate separates network-state authority
    from distribution. An endpoint accepts one expiring threshold-authenticated
    Network Epoch, reports conflicting or
    stale state explicitly, and selects its own Route from deterministic
    Candidate Materializations under the common logical Candidate View. The
    epoch commits its canonical length, input-log cutoff/root, and global
    summaries. Shards or proofs bound client cost without pretending that one
    partial client proves global completeness; independent full auditors check
    inclusion, summaries, and concentration.
24. Official updates use separated threshold release roles, `3-of-5`
    authorization of every new public executable digest, authenticated version
    and rollback state, staged atomic replacement, separate protocol and build
    safety state machines, explicit protocol overlap, and no silent downgrade or
    privacy fallback. One-to-one project keys define only an unqualified
    provisional network.
25. A globally advertised Direct-Origin Source is never Route- or Resolution-
    eligible in the same assignment. An ordinary candidate contacted directly
    enters the bounded installation-wide Direct Source Exposure Set until all
    derived work expires. Public beta/stable requires three/five effective
    authenticated source-only families in addition to effective post-exclusion
    Route-domain supply.
26. Connection, per-Service administration, and Authority Custody are separate
    Local Grants bound to an OS-enforced or launcher-brokered Application
    Principal. A claim-bearing private Application additionally has a Network-
    Isolated Application Boundary; generic adapters protect only traffic they
    actually submit to Ardents.
27. The first public product has one active Service Instance. Host loss makes the Service unavailable
    until Owner-driven migration or Target replacement; automatic origin-loss
    survival would require a separately designed multi-instance Overlay.

Within the current candidate, P5-D3 chooses that five-position information-flow
shape because three positions give the Rendezvous the complete carrier sequence
and four positions make the endpoint legs asymmetric. R-004 selects the
Tor-shaped split-circuit family with a separate Introduction Path as the first
candidate. Disjoint Role Domains, local Candidate View selection, and endpoint-
only continuity are conditional promotion designs. A different production
candidate may replace this package under item 18 without changing Applications
or weakening the Product Core claim. R-013 compares maintained implementation
options only after the applicable gate, and R-023 qualifies performance only for
a promoted release candidate.

Route selection must reduce exposure to Correlated Control across operator,
network, software supply chain, and jurisdiction, but different Node IDs are not
proof of independence. The operating model fixes effective post-exclusion
concentration gates; R-013 must make their evidence and capacity calculation
implementable.

A malicious Node can always delay, drop, or block traffic. Ardents performs only
bounded safe route recovery and otherwise returns the narrowest supported
Connection Result, including indeterminate failure when attack and outage cannot
be distinguished. It never silently retries an Application operation.

The Interactive Route deliberately makes no Broad Traffic Observer resistance
claim. A delayed, padded, or cover-traffic-heavy profile enters the product only
if R-005 identifies a concrete Application job and measurable advantage. It
must reuse the same Application Interface and Service Connection contract; it
may require a new internal Route Adapter, descriptor capabilities, protocol
version, and its own Qualification Evidence Bundle.

## First Reference Application: Named Unlisted Site

Named Unlisted Site followed a successful Carrier Lab and completed its bounded
Gate C tracer. It is the first product-shaped tracer, not a production release.
Carrier Lab deliberately
uses a preconfigured Target/reachability fixture and no Service Name so that a
failed Route candidate stops the project before a second distributed system is
built.

The first Reference Application proves the network chain without defining a
general application platform:

1. A Developer runs a deterministic local HTTP service whose controlled process
   tree has no ordinary network path by harness construction and uses only
   scoped local Ardents IPC/loopback. This is not a general sandbox product.
2. Ardents creates a Service Target; the host generates a private Instance Key
   and receives a bounded public Instance Credential for that key. Ardents maps
   incoming connections to the local service and publishes reachability without
   an ordinary public origin.
3. The fixture supplies one pre-provisioned exact Service Name binding; public
   claiming, lease, delegation, and recovery are not part of this slice.
4. A User who already knows the exact name enters it in a small reference
   controlled client. The name resolves and the current Interactive Route
   candidate reaches the Service while remaining explicitly unqualified.
5. HTTP bytes cross a generic Service Connection; the network does not interpret
   pages, forms, sessions, or application identity.
6. The journey exposes offline and route failure honestly. A later vertical
   slice may preserve the Service Target during ordinary one-Instance migration;
   catastrophe Target replacement belongs to the public naming lifecycle.

The tracer does not require replicated Site Bundles, an Ardents application
runtime, offline storage, a built-in Inbox, or a permanent decentralized hosting
layer. Those are separate product hypotheses, not hidden assumptions inside the
network.

The full permissionless Namespace, Name recovery/governance, Bridges, public
Contributor operation, production updater, and R-023 qualification matrix are
not Reference Application prerequisites.

The same tracer must also open the exact Target Link. That path is not a weaker
Route: it removes only the optional naming operation and prevents an unfinished
global Namespace from blocking carrier, publication, and recovery research.

## Build versus adopt

Ardents should build only the network contracts and integrations that make its
promise distinct. Proven community components are preferred for cryptography,
transport primitives, secure local storage, serialization, sandboxing, and
protocol machinery when their threat and maintenance models fit.

No dependency is accepted because it is familiar, already present in `old`, or
popular. No component is rejected merely because it was not written here. The
first Route candidate is fixed for Carrier Lab and Go is the maintained project
foundation under ADR-0009. The production protocol family, concrete Route
Implementation, wire protocol, cryptographic constructions, and runtime library
set remain open until bounded prototypes compare them against the product,
security, and performance contracts.

## Explicit non-goals for the network core

- clearnet exit, VPN, or general anonymous Internet access;
- public Service index, search, recommendations, or discovery feed;
- built-in messenger, Inbox, Contacts, social graph, or conversation format;
- universal User identity, global profile, mandatory Persona system, or proof
  of personhood;
- offline delivery, application history, or content persistence by implication;
- multi-instance delegation or multihoming in the first Reference Application;
- bundled arbitrary application execution or decentralized compute;
- mandatory blockchain, wallet, token, or governance coin;
- a central registrar, paid name auction, trademark or legal dispute process,
  administrative name seizure, or mandatory global name blacklist;
- proof that anonymous naming produces one-person fairness, rightful ownership,
  or complete prevention of squatting and enumeration;
- opaque cryptographic addresses as the ordinary human naming experience;
- user-tunable anonymity knobs that create silent fingerprinting hazards;
- automatic anonymity from application credentials, content, client
  fingerprinting, timing, or behavior;
- guaranteed invisibility or indistinguishability from ordinary Internet
  traffic;
- Broad Traffic Observer resistance as an Interactive Route promise;
- compatibility with the architecture or wire contracts in `old`.

## What would falsify this direction

The direction must be reconsidered if evidence shows that at least one of the
following is unavoidable:

- the Application Interface cannot be useful without importing application
  identity, persistence, or protocol semantics into the network;
- a useful low-latency route cannot hide both endpoint locations from the
  accepted adversary;
- every useful candidate necessarily exposes information forbidden by the
  R-001 claim matrix or must accept target substitution, data modification,
  replay, redirect, or downgrade to meet its performance budget;
- human-readable naming necessarily creates an unacceptable control or query
  graph;
- safe isolation makes ordinary applications impractical to integrate;
- the accepted security contract cannot meet measured latency, throughput, and
  resource budgets on declared reference devices and networks;
- a diverse contributor population cannot plausibly avoid one operator,
  network, software, or jurisdiction becoming a de facto carrier;
- hostile bootstrap and software update recovery require a permanent trusted
  party whose power cannot be bounded or made visible.
