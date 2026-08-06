# Ardents Network

Ardents is a greenfield product and protocol research project for a private,
anonymous, decentralized application network designed for hostile environments.

The project is currently defining product contracts and validating security
assumptions. This branch does **not** contain production-ready networking
software. The previous implementation is preserved in the
[`old`](https://github.com/dianabuilds/ardents-network/tree/old)
branch and is evidence to learn from, not an architecture to continue by default.

## Product hypothesis

Ardents lets people and developers create private relationships, spaces, sites,
and applications without depending on one provider, exposing a universal
identity, or revealing the network location of a publisher or visitor within a
declared privacy profile.

The network is a public, people-powered carrier. Services and collaboration
spaces can be private and capability-gated. This distinction is deliberate: a
small membership-gated transport would also produce a small anonymity set.

## First tracer product

The first accepted architecture tracer is **Named Unlisted Site**:

1. A developer obtains a human-readable Service Name.
2. The developer builds and signs a reproducible site or client application.
3. Independent Replicas make the release available.
4. A person resolves the name and opens the verified application without either
   endpoint learning the other's network location.
5. The name survives service-key rotation, updates, one unavailable Replica, and
   one blocked relay path.

Ardents is expected to expose protected transport for Application Data, but it
does not ship a mandatory Inbox or messenger. Applications define chat, files,
commands, collaboration, or other semantics themselves. R-019 will decide the
exact destination, online/offline, reliability, ordering, and retention boundary
between the network and an application.

## Start here

- [Product vision](docs/product/vision.md)
- [Functional map](docs/product/functional-map.md)
- [User journeys](docs/product/journeys.md)
- [Domain language](CONTEXT.md)
- [Threat model](docs/security/threat-model.md)
- [Research queue](docs/research/questions.md)
- [Development entry gates](docs/development/entry-gates.md)
- [Architecture decisions](docs/adr/README.md)

## Repository shape

```text
docs/product/       Product promise, scope, functions, and journeys
docs/security/      Adversaries, assets, guarantees, and honest limitations
docs/research/      Open questions, evidence, and research templates
docs/adr/           Accepted durable decisions only
experiments/        Disposable code written to answer named questions
CONTEXT.md          Canonical product vocabulary
```

No production source directory exists yet. It will be created only after the
relevant product, threat, protocol, and technology decisions have passed the
documented entry gates.

## Non-goals for the first product

- a clearnet exit, VPN, or general anonymous Internet proxy;
- a mandatory wallet, blockchain, token, KYC, or global proof of personhood;
- an opaque address as the normal human-facing service identity;
- a global user profile or universally linkable account;
- generic decentralized compute;
- a built-in universal Inbox, messenger, contact model, or conversation format;
- large public social communities;
- an unqualified claim of protection from a global traffic observer.

## License

[MIT](LICENSE)
