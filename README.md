# Ardents Network

Ardents is a Go project building an application network for private,
location-independent Services in hostile environments. The selected closed
contract is a bounded Ubuntu text-Service using Target Links, one common
protection baseline and TCP/TLS as the sole Carrier of the nearest closed release
under [ADR-0122](docs/adr/0122-select-tcp-only-closed-release.md); QUIC is deferred.
It is not a qualified public
product. Component results establish no current anonymity, availability,
censorship-resistance, independent-operation or supported Node-host claim.

## Start here

[Current documentation](docs/README.md) is the entry point: product scope,
threat model, domain language and the exact technical owner for the operation.
The [domain map](docs/development/domain-map.md) names exclusive rules and
collaborations; the [package map](docs/development/package-map.md) owns actual
Go packages and imports. GitHub [#497](https://github.com/dianabuilds/ardents-network/issues/497)
owns the domain completion plan and acceptance criteria.

[Current ADRs](docs/adr/README.md) record surviving decisions.
[Archived documentation](docs/archive/README.md) preserves former editions,
completed evidence, retired plans and unselected proposals with their current
owners. Historical material is read for named provenance, not as an alternate
implementation plan. The former Go/Waku implementation on
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) remains historical
evidence only.

## Working on the repository

Follow [CONTRIBUTING.md](CONTRIBUTING.md) and [repository instructions](AGENTS.md).
Maintained Go code belongs in thin command adapters and cohesive domain Modules.
Run `make quick-check` while changing code and `make check` before integration;
package and dependency registration are part of the architecture contract.
The current human instruction keeps Connection working and holds new
implementations; documentation cleanup selects no additional slice.

## License

[MIT](LICENSE)
