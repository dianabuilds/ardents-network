# Ardents Network

Ardents is a greenfield Go project researching and building an application
network for private, location-independent Services in hostile environments. It
is not production-ready networking software and makes no current anonymity,
independent-operation, public deployment, availability, or supported Node-host
claim.

The maintained system contains bounded closed-alpha Route, Entry, Service
Connection, Namespace, State, Release, Update, Custody, Endpoint, and Node work.
These mechanisms and their historical qualification evidence do not yet
compose one current usable or qualified product candidate. Current work is
reconciling that candidate around a standalone headless Network product; public
operation, platform qualification, and independent-control claims remain
pending.

`H4` and its epic identifiers organize delivery planning only. New product,
release, package, executable, schema, and domain identities must use product
language instead. Historical immutable evidence names are preserved only as
provenance while their maintained consumers are explicitly generalized or
retired.

The former implementation remains on the remote
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) branch as
historical evidence only. It is not an architecture, dependency, or protocol
source for the maintained tree.

## What the project does and does not promise

Applications connect to a Service Target and may use a human-readable Service
Name. The network carries opaque Application bytes over a live Service
Connection; Node IDs are neither User nor Application identities. The current
native Route and private-resolution code are technical tracers, not Route
Qualification.

Ardents does not provide a public directory, messenger, global identity,
wallet/token, clearnet exit, VPN, offline delivery, replicated content, or a
general Application runtime. Encryption is not an anonymity claim, and a
generic local Broker does not provide a qualified isolation claim.

## Start here

- [Product scope](docs/product/scope.md) and [product vision](docs/product/vision.md)
- [Threat model](docs/security/threat-model.md)
- [Domain language](CONTEXT.md)
- Current technical contracts:
  [naming](docs/technical/naming.md),
  [network/route/node](docs/technical/network-route-node.md),
  [endpoint/service](docs/technical/endpoint-service-runtime.md), and
  [release/update/custody](docs/technical/release-update-custody.md), and
  [alpha-control transitions](docs/technical/alpha-control-transition.md)
- [Development documentation](docs/development/README.md) and the factual
  [package map](docs/development/package-map.md)
- [Architecture decisions](docs/adr/README.md)
- [Current command reference](docs/reference/commands.md)
- [Active research queue](docs/research/questions.md)
- [Contributor workflow](CONTRIBUTING.md)

## Working on the repository

Maintained Go code belongs in thin `cmd/<name>` adapters and cohesive
`internal/<domain>` Modules. Run `make quick-check` while making a change and
`make check` before integration. The package map and dependency register are
part of the architecture contract.

Stage briefs, closed research, experiments, and historical campaign material
are removed once their current facts are promoted; Git history preserves
provenance. Do not restore them as compatibility requirements.

## License

[MIT](LICENSE)
