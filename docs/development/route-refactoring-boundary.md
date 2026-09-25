# Route package refactoring boundary

Status: working architecture analysis for the isolated refactoring branch. This
is not a new Route contract, a package-map change, or a C0 execution ledger.
The accepted Route and Carrier contracts continue to govern behavior.

## Current Linux owner graph

On the Linux amd64 candidate, `go list` selects 91 production files in
`internal/route`: 68 `closed_*.go` and 23 other files. It selects 84 test files,
70 of them `closed_*_test.go`. File prefixes show a likely cluster, but they do
not establish an independent package boundary.

The closed cluster still uses these declarations from files outside the cluster:

| Current declaration | Owner file | Closed consumers | Boundary consequence |
| --- | --- | --- | --- |
| `CarrierProfile`, `Carrier` | `node_carrier.go` | Carrier pool, Node Carrier, role/shared Carrier, bootstrap and recipient selection | The selected v2 profiles and byte lane need one deliberate shared contract; copying the type would break callers. |
| `quicNodeCarrier` | `node_carrier_quic.go` | Closed Node Carrier | The QUIC lane is a shared implementation, not a file-local name. |
| `exactPeer` | `tls_adapter.go` | Closed Node and role TLS | Peer verification is shared security logic. |
| `literalEndpoint` | `endpoint_literal.go` | Bootstrap, terminal selection, Node/role/shared Carrier | Literal-address validation must retain the same refusal rule. |
| `writeAll` | `wire_encoding.go` | Closed lane | The full-write rule must remain exact. |

The selected non-closed production files contain no reference to a `Closed*`
identifier; the existing `closedEntryOpener` in `entry_attachment.go` is a
separate local helper. This is a source-level observation, not proof that a
mass move compiles. The type-use audit confirmed the shared declarations above
but could not fully type-check external imports in isolation, so a package move
must still be checked by Linux and Windows builds and behavior tests.

`internal/route/credential` imports `internal/route` in seven production files.
It uses Closed Route types, so any split must migrate that consumer together
with Endpoint and Node imports. A temporary `route` wrapper that imports a new
closed package while that package imports `route` would create a cycle.

## Intended seam

`internal/route` remains the owner of the currently shared Carrier and native
Entry/TLS primitives until their caller-facing contract is settled. A closed
subpackage is justified only if it can own the selected closed Route lifecycle,
wire grammar and tests with a small API and one import direction. The split
must preserve exact persisted and wire identities, both selected Carriers,
cleanup order, and the existing `route/credential` consumer. No bulk file move
or `Closed*` rename is accepted from the prefix count alone.

Before a split, resolve each shared declaration above and map every public
`route.Closed*` caller to its prospective owner. Move behavior tests with the
owner, update the package map and deterministic profile in the same change,
and require compiling intermediate commits. The final candidate still needs
the full Ubuntu gate and both Carrier journeys before integration.
