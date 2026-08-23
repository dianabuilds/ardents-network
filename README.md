# Ardents Network

Ardents is a greenfield product and protocol research project pursuing a
private, location-private, decentralized application network designed for
hostile environments.

The project is defining network contracts and validating security assumptions.
This branch contains the maintained Go project foundation, the completed
Carrier Lab Route experiment, and the bounded Gate C Named Unlisted Site
laboratory tracer, but **not** production-ready networking software. The
previous implementation is preserved in the
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) branch as
evidence to learn from, not an architecture to continue by default.

The documents describe several delivery horizons. They are not one backlog.
The controlled Ubuntu **Gate C** terminal experiment defined in
[product scope](docs/product/scope.md) is complete with `advance`. The next
permitted scope is one bounded Horizon 3 Closed Test Network slice at a time;
within H3, Stage 1–4 local development is complete and Stage 5 Bridge + WebTunnel
development is complete, with the final qualification campaign deferred to
Stage 9.6. Stage 6 private naming implementation and bounded development evidence
are complete and were accepted by the Product Owner on 2026-08-20. Public naming,
permissionless public Bridge distribution, production
updates, Windows qualification, multiparty control, and the complete release
test matrix remain later promotion gates.

Stage 7 was stopped by the Product Owner on 2026-08-22. S7.1 Release Decision,
the maintained S7.2 Update Transaction engineering slice, and R-050 mechanism
evidence remain inputs; S7.3-S7.7 are cancelled, not deferred commitments or
accepted product delivery. Stage 8 productization is in progress from the
verified `1cf7100` entry identity. Its S8.0 current-system snapshot is
historical provenance, not current architecture: maintained owners and command
routes are registered in the [package map](docs/development/package-map.md) and
[Stage 8 target architecture](docs/development/stage-8-target-architecture.md).

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

## Current implementation state

The maintained Go tree contains the completed Carrier Lab and Named Site
laboratories plus the Horizon 3 product runtime. Stage 1–4 development is
complete with local Docker evidence. Stage 5 Bridge/WebTunnel material is C4
historical provenance under R-080; its runtime and live runners are retired and
cannot qualify the native profile. The Product Owner authorized
Stage 6 on 2026-08-20 after accepting R-042/R-044/R-045/R-055 and ADR-0017
through ADR-0019. Maintained S6.1-S6.6 implementation, journey trace, and
mutation coverage are complete; the bounded independent S6E1 command verdict is
`pass`, and the Product Owner recorded Stage 6 `complete` on 2026-08-20. Stage
8 is replacing the old one-command-per-tracer layout. `cmd/ardents` now owns
the retained offline State route and the Endpoint route
`ardents endpoint run <endpoint-plan.json>`; `cmd/ardents-node` and the
separate `cmd/ardents-custody` adapter retain their distinct current roles.
The remaining stage-era commands have no support promise and are being given
explicit C0/C2/C4 dispositions in the
[Stage 8 target architecture](docs/development/stage-8-target-architecture.md)
and [compatibility observer inventory](docs/development/stage-8-compatibility-observer-inventory.md).
Current maintained Module ownership is recorded in the
[package map](docs/development/package-map.md). Product work never grows inside
or imports the frozen `internal/lab` quarantine.

The first completed maintained vertical slice was an Ubuntu-to-Ubuntu
**Carrier Lab**. It
uses one deterministic byte stream, one active Service Instance, project-owned
test keys, a preconfigured Target/reachability fixture, and a fixed synthetic
topology to falsify the current five-position Route candidate. It implements no
Service Name, public Node discovery, Bridge, installer, updater, SDK, browser,
or public-network governance, and it makes no anonymity or decentralization
claim. Its exact disposable technology and evidence contract is
[R-013](docs/research/records/r-013-carrier-lab-technology-candidates.md).

Carrier Lab is not a second repository, Go module, product runtime, or future
top-level architecture. The repository has one root `go.mod`. `cmd/carrier-lab`
is only its executable adapter; `internal/lab/carrier`,
`internal/lab/tooling`, `internal/lab/preflight`, `internal/lab/directcontrol`, and
`internal/lab/nativecircuit` are laboratory Modules; and `lab/carrier/` contains
only four human-authored inputs needed to run them reproducibly: one Dockerfile,
one Compose topology, and two immutable supply locks. Product
Modules may later be promoted from accepted evidence, but never import the lab.

All maintained laboratory source is quarantined under `internal/lab/`; its
human-authored Docker and topology inputs are under `lab/`. The completed Gate C
implementation is factually named `internal/lab/namedsite`, with the thin
`cmd/named-site-lab` adapter. Future product Modules are created as cohesive
siblings under `internal/<responsibility>` and cannot import laboratory code.

The native C-5/C2 laboratory implementation and frozen R-013 comparison are
complete. Official Ubuntu run
[`31404126248`](https://github.com/dianabuilds/ardents-network/actions/runs/31404126248)
returned `advance` after Direct, C-3, C-5/C2, all seven negative cases, resource
and cleanup gates, and an isolated Tor/Chutney reference passed. This retains a
plausible shape for the next controlled slice; it is not a Route Qualification,
an anonymity claim, or a production networking decision.

Horizon 3 has since progressed past the Gate C tracer. R-029 through R-031
recorded the local Stage 1, Stage 2 (`95/95`), and Stage 3 (`27/27`) Docker
development campaigns. R-032 authorized the bounded S4.1–S4.3 recovery work,
and R-038 (`P3-D3b4`, accepted 2026-08-15) froze the four-role capacity
contract; Stage 4 development and local evidence are complete. R-033 through
R-036 are decided and authorized the Stage 5 Bridge + WebTunnel slice
(R-036 pins standalone WebTunnel `v0.0.6` at commit
`d729fde1f38357dcefa2a751eb4752e9ca78f910`); R-037 fixed the controlled
evidence contract and profile `h3-s5-b1-v1`. The Stage 5 implementation brief
was accepted on 2026-08-16, and the Product Owner advanced maintained Stage 5
development on 2026-08-19 while moving the complete `564`-cell candidate
campaign plus six evidence-integrity campaigns to S9.6. R-039 added the Stage 6
private-naming foundation. None of these local results close Ubuntu Stage 1
`short` / `churn-2h` / independent `unattended-24h`, applicable R-023
qualification, the deferred S9.6 final campaign, or any stronger external,
privacy, security, or release gate.

## First conditional Reference Application

Carrier Lab demonstrated a plausible Route candidate, and the completed first
product-shaped tracer is **Named Unlisted Site**:

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

Official Ubuntu workflow
[`31464163490`](https://github.com/dianabuilds/ardents-network/actions/runs/31464163490)
returned Gate C `advance` for run `gatec-31464163490-1`: 20/20
positive connections, 17/17 required failures, and 5/5 migration episodes
passed, with cleanup complete. The bounded result is bound to source SHA-256
`fc0b941cf28befacfa3ce76e0373eccfa3e08ec8c2f8c5075150741b8f570610`.
This closes the controlled Ubuntu tracer only; it is not Route Qualification,
a public privacy/anonymity claim, or permission to implement every Horizon 3
system at once.

## Start here

- [Testing model](docs/development/testing.md)
- [ADR-0011: independent unit, end-to-end, and live tests](docs/adr/0011-separate-unit-e2e-and-live-tests.md)
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
- [H3 technical design](docs/development/horizon-3-technical-design.md)
- [Stage 4 implementation brief](docs/development/horizon-3-stage-4-brief.md)
- [Stage 5 implementation brief](docs/development/horizon-3-stage-5-brief.md)
- [Stage 6 implementation brief](docs/development/horizon-3-stage-6-brief.md)
- [Stopped Stage 7 implementation brief](docs/development/horizon-3-stage-7-brief.md)
- [Stage 7 development-host campaign specification](docs/development/stage-7-host-campaign-spec.md)
- [Stage 7 joint review record](docs/development/stage-7-joint-review.md)
- [Stage 7 S7.0 start record](docs/development/stage-7-start-record.md)
- [Stage 7 stop record](docs/development/stage-7-stop-record.md)
- [Stage 8 productization and restructuring brief](docs/development/horizon-3-stage-8-brief.md)
- [Stage 8 start record](docs/development/stage-8-start-record.md)
- [Stage 9 frozen product qualification and closure brief](docs/development/horizon-3-stage-9-brief.md)
- [R-029 authenticated Node lifecycle (H3 Stage 1)](docs/research/records/r-029-h3-authenticated-node-lifecycle.md)
- [R-030 real multi-node route (H3 Stage 2)](docs/research/records/r-030-h3-real-multi-node-route.md)
- [R-031 Service Connection + Application Interface (H3 Stage 3)](docs/research/records/r-031-h3-service-connection-application-interface.md)
- [R-032 same-connection recovery (H3 Stage 4)](docs/research/records/r-032-h3-same-connection-recovery.md)
- [R-033 Stage 5 research map](docs/research/records/r-033-h3-stage-5-research-map.md)
- [R-034 Stage 4 Bridge capacity sequencing](docs/research/records/r-034-stage-4-bridge-capacity-sequencing.md)
- [R-035 Bridge state](docs/research/records/r-035-h3-bridge-state.md)
- [R-036 WebTunnel camouflage adapter](docs/research/records/r-036-h3-camouflage-adapter.md)
- [R-037 blocked-entry evidence contract](docs/research/records/r-037-h3-blocked-entry-evidence.md)
- [R-038 Stage 4 role capacity (`P3-D3b4`)](docs/research/records/r-038-h3-stage-4-role-capacity.md)
- [R-039 private naming lifecycle (H3 Stage 6)](docs/research/records/r-039-h3-private-naming-lifecycle.md)
- [R-048 Stage 7 contract and decision order](docs/research/records/r-048-h3-stage-7-contract.md)
- [R-058 reassessment, restructuring, and frozen qualification](docs/research/records/r-058-h3-reassessment-and-closure.md)

## Repository shape

The normative [repository layout and growth
rules](docs/development/repository-layout.md) separate the current factual tree
from conditional future Modules and delivery zones. The [package
map](docs/development/package-map.md) lists only Go packages that exist and their
permitted current imports. Run `make unit` while working, `make e2e` for
retained cross-process behavior, and `make check` before integration. The
native Route live profile is inactive until M8/M11 select and measure its
peer-facing runtime.

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
