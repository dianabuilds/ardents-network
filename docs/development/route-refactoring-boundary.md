# Route package refactoring boundary

Status: working architecture analysis for the isolated refactoring branch. This
is not a new Route contract, a package-map change, or a C0 execution ledger.
The accepted Route and Carrier contracts continue to govern behavior.

## Current Linux owner graph

On the Linux amd64 candidate after the capsule extraction, `go list` selects
86 production files in `internal/route`: 63 `closed_*.go` and 23 other
files. It selects 83 test files, 69 of them `closed_*_test.go`. The child
`internal/route/capsule` has six production files and one behavior test. File prefixes show a likely
cluster, but they do not establish an independent package boundary.

The fixed Introduction capsule codec, HPKE transcript, and its canonical-byte
test now belong to `internal/route/capsule`. Route, Endpoint, and Node consume
its small API directly; literal-byte tests still pin the fixed 4-KiB
operation and plaintext. The remaining closed cluster uses these
declarations from files outside the cluster:

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

## Candidate terminal-operation owner

The next bounded extraction candidate is the fixed terminal body grammar,
tentatively `internal/route/terminal`. Its production cohort is exactly
`closed_issuance_operation.go`, `closed_issuance_client_operation_linux.go`,
`closed_descriptor_operation.go`, `closed_descriptor_client_operation_linux.go`,
`closed_join_operation.go`, `closed_join_client_operation_linux.go`,
`closed_registration_operation.go`, and
`closed_registration_encoding_linux.go`. Their four matching operation test
files contain the canonical size, nonce, padding, expiry, and byte-offset
oracles.
This owner would import only `internal/service/reachability` and the standard
library; it has no reason to import `internal/route`.

The owner would expose the four request types, the issuance result type, and
their existing encode/decode operations without the redundant `Closed` prefix.
The 4-KiB and 16-KiB body sizes must have one definition shared with Route
lane validation; merely copying the two constants would create independent
wire contracts. The zero-padding rule belongs to this codec owner and must
remain identical for Descriptor, JOIN, and registration bodies.

The current Route tests for Descriptor and JOIN also assert outer lane
framing. Keep those cross-layer assertions in `internal/route` while moving
the pure body tests with the codec. Migrate actual Route, Node, Endpoint, and
`route/credential` callers in the same compiling change; no delegating
`route.Closed*` wrappers or test-only package may remain. Route may import
the terminal codec, but the codec must not import Route. Register exact imports
and command ownership in `package-map.md`, then verify Linux and Windows
builds, affected behavior tests, and the full gate.

This is a source-level candidate, not an accepted package split. The caller
cohort includes `closed_introduction_client.go`,
`closed_introduction_delivery.go`, and Node Introduction registration and
delivery. Those owners overlap the live network opening work. Recheck their
names and behavior after its completed changes are integrated before editing
the cohort; the existing capsule extraction does not prove this split.

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
