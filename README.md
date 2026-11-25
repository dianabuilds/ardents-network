# Ardents Network

Ardents is a Go project building an application network for private,
location-independent Services in hostile environments. The selected closed
contract supports documents and bidirectional streams of arbitrary data on
Ubuntu, using Target Links and one common protection baseline. TCP/TLS is the
sole Carrier of the nearest closed release under
[ADR-0122](docs/adr/0122-select-tcp-only-closed-release.md). QUIC is deferred.
Ardents is a closed Alpha candidate. Component results establish no current anonymity, availability,
censorship-resistance, independent-operation or supported Node-host claim.

## Start here

[Current documentation](docs/README.md) is the entry point: product scope,
threat model, domain language and the exact technical owner for the operation.
The [domain map](docs/development/domain-map.md) names exclusive rules and
collaborations; the [package map](docs/development/package-map.md) owns actual
Go packages and imports. GitHub [#497](https://github.com/dianabuilds/ardents-network/issues/497)
owns the domain completion plan and acceptance criteria.

[Current ADRs](docs/adr/README.md) record surviving decisions.
Former editions, completed evidence, retired plans and unselected proposals
are retained in Git history with their current owners, recoverable via the
[pinned archive index](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/README.md).
Historical material is read for named provenance, not as an alternate
implementation plan. The former Go/Waku implementation on
[`old`](https://github.com/dianabuilds/ardents-network/tree/old) remains historical
evidence only.

## Working on the repository

Follow [CONTRIBUTING.md](CONTRIBUTING.md) and [repository instructions](AGENTS.md).
Maintained Go code uses thin command adapters and cohesive packages under
`internal/`, with responsibilities and imports registered in the package map.
Run `make quick-check` while changing code and `make check` before integration;
package and dependency registration are part of the architecture contract.
The Product Owner's current selection and the
[C0 milestone](https://github.com/dianabuilds/ardents-network/milestone/1) govern
which implementation work is active.

## License

[MIT](LICENSE)
