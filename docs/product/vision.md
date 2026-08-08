# Product vision

Status: **proposed network product contract**

Last reviewed: 2026-08-07

## Vision

Ardents is an internal application network for reaching services without making
their ordinary network locations part of the application relationship. A User
runs an Ardents endpoint; a Developer connects an existing site or application
service; independently operated Nodes carry live Application Data between them.

The network is the product. A site, messenger, file exchange, community, or
identity system is an Application or optional Overlay Service built on it.
Ardents does not need to understand those application semantics in order to
route and protect their connections.

## What the product adjectives mean

- **Private** means Service Connections protect Application Data and expose only
  the metadata allowed by a declared Route Profile. It does not mean every
  Service is access-controlled.
- **Anonymous** is never a blanket claim. The Interactive Route aims to hide the
  User's ordinary location from the Service, the Service Instance's ordinary
  location from the User, and prevent any one ordinary Node from linking an
  endpoint location to a Service Name, Service Target, or opposite endpoint. It
  does not promise resistance to timing-and-volume correlation by a Broad
  Traffic Observer or hide identity disclosed by Application Data and behavior.
  An implementation may present this claim only after Route Qualification;
  Ardents research or an unqualified candidate is not an anonymous network.
- **Decentralized** means ordinary reachability and routing do not require one
  hosting or relay operator. Naming, bootstrap, releases, and emergency powers
  remain explicit Control Plane risks until their own designs are accepted.

## Security and performance

Security and performance are equal product constraints. Every privacy,
isolation, and abuse-control mechanism must state and measure its connection
latency, throughput, tail behavior, CPU, memory, and overload cost on declared
device and network classes. An optimization cannot bypass a security invariant,
and a security mechanism is not viable if it makes the accepted Application
journey impractical under honest load.

Performance floors are not hardware ceilings. A stronger endpoint may scale
finite hierarchical local budgets above the V1 floor, while the Endpoint Owner
retains a cap. Extra capacity grants no infrastructure role, trust, authority,
route priority, or privacy exception, and exact hardware capacity is not required
network metadata.

## Fixed direction

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
7. V1 supports one active Service Instance per Service Target. Routine migration
   preserves the target through an encrypted Service Authority export/import;
   loss or compromise replaces the target through the stable Service Name.
8. Endpoint Location Privacy is distinct from Application anonymity. Ardents
   does not inspect, sanitize, or promise to unlink credentials, content,
   fingerprints, timing, or behavior visible to an intended endpoint.

## Network product boundary

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
- resolve an exact Service Name in one canonical network-wide Namespace without
  a public directory or resolver-selected alternate meaning;
- establish a protected live Service Connection;
- exchange opaque Application Data within measurable performance and resource
  budgets and receive explicit failure and closure;
- keep unrelated application contexts from being linked by accidental route
  reuse;
- recover an entry path when ordinary bootstrap or transport is blocked.

The same V1 endpoint platforms must support publishing an ordinary local
Application. Ubuntu LTS is the sole Linux qualification baseline; other Linux
distributions and architectures receive no V1 compatibility or release claim.
The required V1 infrastructure reference class is an Ubuntu LTS `x86-64`
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

## Core product promises

1. A Service Connection is confidential and authenticated end to end against
   carrier Nodes acting alone or together, while its endpoints, Service
   Authority, and accepted cryptography remain uncompromised.
2. A Service Target is independent of a Node identity and ordinary IP location.
3. A complete Service Name has one verifiable network-wide meaning under
   authority distinct from Service Authority and can remain stable when a
   compromised or lost Service Target must be replaced. Its V1 canonical form is
   a lowercase ASCII dot hierarchy and its explicit Service Link identifies
   Ardents rather than DNS.
4. Root Service Name control begins with the first valid claim in deterministic
   shared Namespace order and remains a renewable lease, not permanent property,
   a User account, or human identity. Grace is visible, Release stops resolution,
   and reclaim creates a new generation that invalidates prior state. Recovery
   exists only through a precommitted scoped policy and fails closed while pending.
5. The User and Service do not learn each other's ordinary network location
   within the declared Interactive Route conditions.
6. Route Knowledge Separation prevents any one ordinary Node from learning the
   full Route, reading Application Data, or linking an endpoint's ordinary
   location to a Service Name, Service Target, or opposite endpoint.
7. A failed path produces bounded recovery or an explicit failure; the network
   does not silently pretend that an offline Service received data.
8. Losing or blocking one ordinary path does not require one central operator
   to restore connectivity.
9. Every security, anonymity, availability, and decentralization claim states
   its adversary, conditions, measurement, and limitation.
10. Every accepted security mechanism also has a performance budget and overload
   test; no performance optimization may silently weaken that mechanism.
11. A Local Traffic Observer receives no Service destination or plaintext from
    the protocol. Ardents does not promise to hide its own use, but avoids one
    mandatory stable fingerprint and treats Transport Camouflage as a measurable
    censorship-resistance goal.
12. The Interactive Route anonymity claim covers one malicious ordinary Node,
    not arbitrary collusion. Correlated Control spanning both endpoint sides may
    link a User and Service through traffic metadata without exposing
    Application Data.
13. A malicious Service receives no User origin, Route, Isolation Context, or
    network-generated stable User identifier from Ardents, while a malicious
    User receives no Service Instance origin, Route, or Service Authority. Each
    still sees the Application Data and behavior intended for it.
14. Target authentication, Route Profile binding, protocol freshness, and
    integrity fail closed. Detected modification, injection, replay, redirect,
    or downgrade never becomes an accepted connection or Application Data.
15. The Interactive Route claim is implementation-gated. Reproducible
    endpoint-edge, Node-role, malicious-endpoint, isolation, and active-attack
    tests must pass before a candidate earns Route Qualification; the claim's
    conditions and excluded adversaries remain visible to Users and Developers.

The Interactive Route is therefore multi-hop, but the product contract does not
select Tor, onion routing, a fixed path shape, or a fixed number of hops. R-004
must find the least costly routing family that preserves the accepted knowledge
separation, and R-023 must bound its performance cost.

Route selection must reduce exposure to Correlated Control across operator,
network, software supply chain, and jurisdiction, but different Node IDs are not
proof of independence. R-011 must make that uncertainty measurable.

A malicious Node can always delay, drop, or block traffic. Ardents performs only
bounded safe route recovery and otherwise returns the narrowest supported
Connection Result, including indeterminate failure when attack and outage cannot
be distinguished. It never silently retries an Application operation.

The Interactive Route deliberately makes no Broad Traffic Observer resistance
claim. A delayed, padded, or cover-traffic-heavy profile enters the product only
if R-005 identifies a concrete Application job and measurable advantage.

## First tracer: Named Unlisted Site

The first Reference Application proves the network chain without defining a
general application platform:

1. A Developer runs an ordinary local HTTP service.
2. Ardents creates a Service Target, maps incoming connections to that local
   service, and publishes reachability without an ordinary public origin.
3. The Developer binds a recoverable Service Name to the target.
4. A User who already knows the exact name enters it in a small reference
   client. The name resolves and an Interactive Route with current Route
   Qualification reaches the Service; a simulated or unqualified route is
   labeled as such.
5. HTTP bytes cross a generic Service Connection; the network does not interpret
   pages, forms, sessions, or application identity.
6. The journey exposes route failure honestly, rebuilds an alternate path when
   possible, preserves the Service Target during an ordinary host migration,
   and preserves only the Service Name after simulated target compromise.

The tracer does not require replicated Site Bundles, an Ardents application
runtime, offline storage, a built-in Inbox, or a permanent decentralized hosting
layer. Those are separate product hypotheses, not hidden assumptions inside the
network.

## Build versus adopt

Ardents should build only the network contracts and integrations that make its
promise distinct. Proven community components are preferred for cryptography,
transport primitives, secure local storage, serialization, sandboxing, and
protocol machinery when their threat and maintenance models fit.

No dependency is accepted because it is familiar, already present in `old`, or
popular. No component is rejected merely because it was not written here. The
production language, routing family, wire protocol, and library set remain open
until the product, security, and performance contracts can compare them fairly.

## Explicit non-goals for the network core

- clearnet exit, VPN, or general anonymous Internet access;
- public Service index, search, recommendations, or discovery feed;
- built-in messenger, Inbox, Contacts, social graph, or conversation format;
- universal User identity, global profile, mandatory Persona system, or proof
  of personhood;
- offline delivery, application history, or content persistence by implication;
- multi-instance delegation or multihoming in the first tracer;
- bundled arbitrary application execution or decentralized compute;
- mandatory blockchain, wallet, token, or governance coin;
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
