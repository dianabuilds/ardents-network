# Ardents Network

Ardents is a greenfield product and protocol research project pursuing a
private, location-private, decentralized application network designed for
hostile environments.

The project is defining network contracts and validating security assumptions.
This branch contains the maintained Go project foundation and the Carrier Lab
preflight, but **not** production-ready networking software. The
previous implementation is preserved in the
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) branch as
evidence to learn from, not an architecture to continue by default.

The documents describe several delivery horizons. They are not one backlog.
The only current implementation scope is the Ubuntu controlled **Carrier Lab**
defined in [product scope](docs/product/scope.md); public naming, Bridges,
production updates, Windows qualification, multiparty control, and the complete
release test matrix remain later promotion gates.

## Product hypothesis

Ardents lets an existing local Application publish or connect to an internal
Service through independently operated infrastructure without making either
endpoint's ordinary network location part of the application relationship.

Applications address a location-independent Service Target, optionally through
a human-readable Service Name. They exchange opaque bytes over a live protected
Service Connection. Infrastructure Node IDs are not User or application
addresses, and the network does not impose messenger, identity, storage, or
content semantics.

Public Beta is intended as endpoint software for ordinary Windows 11 and Ubuntu LTS `x86-64`
desktop/laptop devices: Users connect from them and Developers can publish local
Applications from them. The required infrastructure benchmark uses an Ubuntu
LTS `x86-64` `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS; macOS
and mobile remain later targets. Other Linux distributions and architectures
have no Public Beta compatibility or release claim.
Client and publisher capacity figures are minimum floors, not hard limits:
stronger endpoints may use larger finite local budgets without gaining a Node
role, trust, authority, route priority, or weaker security rules.

The durable Interactive Route contract is multi-hop Route Knowledge Separation:
one ordinary Node acting only from its role-local view is not directly given a
binding between an endpoint's ordinary location and a Service Name, Service
Target, or opposite endpoint. The first Carrier Lab candidate uses the
Tor-shaped family of two independently built endpoint circuits joined at a
User-selected Rendezvous, with five symmetric logical carrier positions and a
separate Introduction Path. This is a falsifiable candidate, not a selected
production route or a selection of Tor naming, exit routing, production
library, production cryptography, or production wire protocol. The disposable
lab choices are frozen separately in R-013 and carry no promotion by inertia.

If that data-path candidate survives, the current later-horizon enforcement
candidate has each endpoint select its own leg from authenticated Candidate
Materializations under one epoch-committed logical Candidate View. Public Beta
has ordinary and Bridge
regimes and at most one long-lived Entry Set for each activated adjacent Role
Domain and regime per installation. Applications, Services, contexts,
destinations, generations, and Bridge Invites cannot create more; every Bridge
key is eligible for only one adjacent domain. Channels, Interiors, destinations,
sessions, and recovery state remain context-separated. Disjoint stable Role
Domains make the two hidden legs assignable without a Service-side rejection
oracle. Name/Target/descriptor lookup uses a private Destination Resolution role
restricted to the non-adjacent Rendezvous Domain; a resolution identity/family
is excluded from that connection's Rendezvous. The User selects a fresh
Rendezvous for each new connection. Domain assignment is finite and cannot
overlap across reassignment: old duty stops, drains, and remains quarantined
before later-domain eligibility.

A source contacted before a private Route exists sees the requester origin.
Globally advertised Direct-Origin Sources are therefore source-only; ordinary
candidates contacted directly enter one bounded installation-wide exposure set
and are locally excluded from Route/Resolution work until every derived lifetime
ends. Public family thresholds are calculated after all mandatory exclusions,
and authenticated source-only family supply is counted separately.

The one-Node claim covers only that Node's role-local view. A Node operator that
also controls/observes an endpoint, active probe source, or direct-origin
contact may confirm a known
Target through distinctive low-latency timing/volume; arbitrary Correlated
Control has the same broader limitation. Those are explicit non-claims.
End-to-end Application Data confidentiality, Forward Secrecy for honestly
completed connections, and Service Target authentication remain separate
guarantees.

The product contract is Endpoint Location Privacy, not automatic anonymity
inside an Application. An intended Service reads its Application Data and can
recognize credentials, content, fingerprints, timing, or behavior that the
Application reveals; the network adds no global User identity or route
diagnostics. Ardents protects only bytes submitted to its local interface. A
claim-bearing private Application must run inside the tested Network-Isolated
Application Boundary, with no ordinary listener, DNS, direct socket/fetch,
WebRTC/QUIC, or callback/SSRF path. Generic adapters remain compatible but do
not inherit the stronger Application-level location claim.

The contract requires authentication and integrity to fail closed: modified,
injected, replayed, redirected, or downgraded protocol data is never accepted as
a valid Service Connection. A Node can still delay or drop traffic;
indistinguishable causes are reported honestly and bounded recovery never
replays an Application operation.

No implementation has yet earned Route Qualification. A Public Beta candidate
may present the Interactive Route claim only after reproducible edge-traffic, Node-state,
malicious-endpoint, Application Principal, Application-network isolation,
Direct-Origin Source, Role Domain transition, and active-attack tests pass.
Until then this
repository describes research toward an anonymous network, not a validated
anonymous network implementation.

The carrier is public so that private Services can draw from a broader anonymity
set. Naming, bootstrap, software releases, and governance remain explicit
Control Plane risks rather than being hidden behind the word “decentralized.”

## Current project slice

The first maintained vertical slice is an Ubuntu-to-Ubuntu **Carrier Lab**. It
uses one deterministic byte stream, one active Service Instance, project-owned
test keys, a preconfigured Target/reachability fixture, and a fixed synthetic
topology to falsify the current five-position Route candidate. It implements no
Service Name, public Node discovery, Bridge, installer, updater, SDK, browser,
or public-network governance, and it makes no anonymity or decentralization
claim. Its exact disposable technology and evidence contract is
[R-013](docs/research/records/r-013-carrier-lab-technology-candidates.md).

Carrier Lab is not a second repository, Go module, product runtime, or future
top-level architecture. The repository has one root `go.mod`. `cmd/carrier-lab`
is only its executable adapter; `internal/harness`,
`internal/harness/tooling`, `internal/preflight`, and `internal/directcontrol`
are laboratory Modules; and `carrier-lab/` contains only the three container
inputs needed to run them reproducibly. Product
Modules may later be promoted from accepted evidence, but never import the lab.

## First conditional Reference Application

After Carrier Lab demonstrates a plausible Route, the first product-shaped
tracer is **Named Unlisted Site**:

1. A Developer runs an ordinary local HTTP service.
2. Ardents exposes it under a Service Target without publishing a stable public
   origin to Users.
3. The test fixture supplies one pre-provisioned human-readable Service Name
   binding; permissionless claiming is not part of this slice.
4. A User who already knows the exact name resolves it and opens a protected
   live connection; Ardents supplies no directory or search.
5. HTTP remains application data. The tracer observes private name resolution,
   target authentication, route behavior, and explicit failure without claiming
   public Route Qualification.
6. A later ordinary-migration slice keeps one active Service Instance generation. The
   new host generates a new private Instance Key and receives a newly issued
   public bounded Instance Credential; neither the old runtime key nor durable
   Service Authority is moved.

Both controlled tracer Applications have no ordinary network path by harness
construction and communicate only through scoped local Ardents IPC/loopback;
this is not yet a reusable desktop sandbox product. The
first Reference Application has one active Service Instance; loss of its host means explicit
unavailability until Owner-driven migration or Target replacement.

The tracer does not require a replicated Site Bundle, bundled application
runtime, offline delivery, Inbox, or messenger. Those are separate optional
products or overlays if future evidence justifies them.

## Start here

- [Authoritative product scope and delivery horizons](docs/product/scope.md)
- [Carrier Lab technology and experiment contract](docs/research/records/r-013-carrier-lab-technology-candidates.md)
- [Language and runtime candidates](docs/research/records/r-014-language-runtime-candidates.md)
- [Product vision](docs/product/vision.md)
- [Accepted operating model and remaining bottlenecks](docs/product/operating-model.md)
- [Network functional map](docs/product/functional-map.md)
- [Network product journeys](docs/product/journeys.md)
- [Domain language](CONTEXT.md)
- [Threat model](docs/security/threat-model.md)
- [Network research queue](docs/research/questions.md)
- [Development entry gates](docs/development/entry-gates.md)
- [Go engineering rules](docs/development/go-engineering.md)
- [Repository layout and growth rules](docs/development/repository-layout.md)
- [Current package map](docs/development/package-map.md)
- [Carrier Lab preflight contract](docs/development/carrier-lab-preflight.md)
- [Contributor workflow](CONTRIBUTING.md)
- [Architecture decisions](docs/adr/README.md)

## Repository shape

The normative [repository layout and growth
rules](docs/development/repository-layout.md) separate the current factual tree
from conditional future Modules and delivery zones. The [package
map](docs/development/package-map.md) lists only Go packages that exist and their
permitted current imports. Run `make quick-check` while working and `make check`
before integration.

Carrier Lab preflight is run with:

```sh
bash ./scripts/preflight.sh --go-archive /absolute/path/go1.26.5.linux-amd64.tar.gz
```

The command requires the R-013-pinned Ubuntu image and Go archive to already be
present; it never substitutes mutable inputs or installs missing dependencies.

## Non-goals for the network core

- clearnet exit, VPN, or general anonymous Internet proxy;
- public Service directory, search, recommendation, or feed;
- mandatory wallet, blockchain, token, KYC, or proof of personhood;
- global User profile or universally linkable application identity;
- built-in messenger, Inbox, Contacts, conversation format, or offline history;
- multi-instance delegation or multihoming in the first public product;
- application persistence, arbitrary code execution, or decentralized compute
  by implication;
- an opaque cryptographic address as the ordinary human-facing Service Name;
- automatic anonymity for Application Data, credentials, fingerprints, or
  behavior;
- guaranteed indistinguishability from ordinary Internet traffic;
- Broad Traffic Observer resistance as an Interactive Route promise.

## License

[MIT](LICENSE)
