# Ardents Network

Ardents is a greenfield product and protocol research project pursuing a
private, location-private, decentralized application network designed for
hostile environments.

The project is defining network contracts and validating security assumptions.
This branch does **not** contain production-ready networking software. The
previous implementation is preserved in the
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) branch as
evidence to learn from, not an architecture to continue by default.

## Product hypothesis

Ardents lets an existing local Application publish or connect to an internal
Service through independently operated infrastructure without making either
endpoint's ordinary network location part of the application relationship.

Applications address a location-independent Service Target, optionally through
a human-readable Service Name. They exchange opaque bytes over a live protected
Service Connection. Infrastructure Node IDs are not User or application
addresses, and the network does not impose messenger, identity, storage, or
content semantics.

V1 is endpoint software for ordinary Windows 11 and Ubuntu LTS `x86-64`
desktop/laptop devices: Users connect from them and Developers can publish local
Applications from them. The required infrastructure benchmark uses an Ubuntu
LTS `x86-64` `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` reference VPS; macOS
and mobile remain later targets. Other Linux distributions and architectures
have no V1 compatibility or release claim.
Client and publisher capacity figures are minimum floors, not hard limits:
stronger endpoints may use larger finite local budgets without gaining a Node
role, trust, authority, route priority, or weaker security rules.

The accepted Interactive Route contract is multi-hop for Route Knowledge
Separation: one ordinary Node cannot link an endpoint's ordinary location to a
Service Name, Service Target, or opposite endpoint. Its baseline data path fixes
five symmetric logical carrier positions without selecting Tor, a routing
algorithm, library, cryptography, or wire protocol.

The target anonymity claim covers one malicious ordinary Node, not arbitrary
Correlated Control of several roles. Control spanning both endpoint sides may
link the relationship through traffic metadata; end-to-end Application Data
confidentiality and Service Target authentication remain separate guarantees.

The product contract is Endpoint Location Privacy, not automatic anonymity
inside an Application. An intended Service reads its Application Data and can
recognize credentials, content, fingerprints, timing, or behavior that the
Application reveals; the network adds no global User identity or route
diagnostics.

The contract requires authentication and integrity to fail closed: modified,
injected, replayed, redirected, or downgraded protocol data is never accepted as
a valid Service Connection. A Node can still delay or drop traffic;
indistinguishable causes are reported honestly and bounded recovery never
replays an Application operation.

No implementation has yet earned Route Qualification. A candidate may present
the Interactive Route claim only after reproducible edge-traffic, Node-state,
malicious-endpoint, isolation, and active-attack tests pass. Until then this
repository describes research toward an anonymous network, not a validated
anonymous network implementation.

The carrier is public so that private Services can draw from a broader anonymity
set. Naming, bootstrap, software releases, and governance remain explicit
Control Plane risks rather than being hidden behind the word “decentralized.”

## First Reference Application

The first architecture tracer is **Named Unlisted Site**:

1. A Developer runs an ordinary local HTTP service.
2. Ardents exposes it under a Service Target without publishing a stable public
   origin to Users.
3. The Developer binds a human-readable Service Name.
4. A User who already knows the exact name resolves it and opens a protected
   live connection; Ardents supplies no directory or search.
5. HTTP remains application data. The tracer verifies name continuity, target
   authentication, endpoint-location claims, route failure, and blocked entry.
6. V1 uses one active Service Instance. An ordinary migration securely moves
   its Service Authority and preserves the target; compromise creates a new
   target while the Service Name remains.

The tracer does not require a replicated Site Bundle, bundled application
runtime, offline delivery, Inbox, or messenger. Those are separate optional
products or overlays if future evidence justifies them.

## Start here

- [Product vision](docs/product/vision.md)
- [Network functional map](docs/product/functional-map.md)
- [Network product journeys](docs/product/journeys.md)
- [Domain language](CONTEXT.md)
- [Threat model](docs/security/threat-model.md)
- [Network research queue](docs/research/questions.md)
- [Development entry gates](docs/development/entry-gates.md)
- [Architecture decisions](docs/adr/README.md)

## Repository shape

```text
docs/product/       Product promise, network boundary, functions, and journeys
docs/security/      Adversaries, assets, guarantees, and honest limitations
docs/research/      Open questions, evidence, and research templates
docs/adr/           Accepted durable decisions only
experiments/        Disposable code written to answer named questions
CONTEXT.md          Canonical network product vocabulary
```

No production source directory exists yet. It will be created only after the
relevant product, threat, protocol, and technology decisions pass the documented
entry gates.

## Non-goals for the network core

- clearnet exit, VPN, or general anonymous Internet proxy;
- public Service directory, search, recommendation, or feed;
- mandatory wallet, blockchain, token, KYC, or proof of personhood;
- global User profile or universally linkable application identity;
- built-in messenger, Inbox, Contacts, conversation format, or offline history;
- multi-instance delegation or multihoming in the first tracer;
- application persistence, arbitrary code execution, or decentralized compute
  by implication;
- an opaque cryptographic address as the ordinary human-facing Service Name;
- automatic anonymity for Application Data, credentials, fingerprints, or
  behavior;
- guaranteed indistinguishability from ordinary Internet traffic;
- Broad Traffic Observer resistance as an Interactive Route promise.

## License

[MIT](LICENSE)
