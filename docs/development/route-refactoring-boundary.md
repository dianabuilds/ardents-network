# Route package refactoring boundary

Status: working architecture analysis for the isolated refactoring branch. The
capsule, terminal body, ARDP framing, and replay extractions below are implemented and registered
in the package map; the remaining Route split is still analysis, not a new
Route contract or C0 execution ledger. The accepted Route and Carrier contracts
govern behavior.

## Current Linux owner graph

On the Linux amd64 candidate after the replay extraction, `go list` selects
73 production and 77 test files in `internal/route`. Its capsule, ardp,
terminal, and replay children have respectively 6/1, 3/1, 9/4, and 5/3
production/test files. File prefixes show a likely cluster, but they do not
establish an independent package boundary.

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

## Terminal-operation owner

The fixed terminal body grammar is now owned by `internal/route/terminal`. Its
production cohort came from
`closed_issuance_operation.go`, `closed_issuance_client_operation_linux.go`,
`closed_descriptor_operation.go`, `closed_descriptor_client_operation_linux.go`,
`closed_join_operation.go`, `closed_join_client_operation_linux.go`,
`closed_registration_operation.go`, and
`closed_registration_encoding_linux.go`. Their four matching operation test
files contain the canonical size, nonce, padding, expiry, and byte-offset
oracles.
This owner imports only `internal/service/reachability` and the standard
library; it does not import `internal/route`.

The owner exposes the four request types, the issuance result type, and their
encode/decode operations without the redundant `Closed` prefix. Route lane
validation uses `terminal.BodySize` and `terminal.SmallBodySize`; these sizes
have one definition. The zero-padding rule belongs to this codec owner and
remains identical for Descriptor, JOIN, and registration bodies.

The pure body tests moved with the codec. Outer Descriptor and JOIN lane
assertions remain in `internal/route`. Actual Route, Node, Endpoint, and
`route/credential` callers use the new package directly, with no delegating
`route.Closed*` wrappers. The exact imports and absence of command ownership
are recorded in `package-map.md`. Recheck overlapping caller names and behavior
when completed network-opening work is integrated; the network task retains
its own uncommitted implementation.

## ARDP framing owner

`internal/route/ardp` owns the generation-3 lane header, frame-kind and body
size checks, HELLO and bootstrap bodies, and fixed ACCEPT acknowledgement. Its
canonical-byte tests moved with the codec. The purpose-to-duty assignment
table remains in Route because it evaluates current role and duty facts, not
wire syntax. JOIN's separate byte-accounted reader asks ARDP to validate the
header before reserving its body; Route still owns accounting and physical
stream lifetime. ARDP imports only `terminal` and the standard library, so
Route, Node, Endpoint, and credential can consume it without an import cycle.
They use the new types directly, with no retained `route.ClosedLaneFrame` or
`route.ClosedHello` aliases.

## Receiving replay owner

`internal/route/replay` owns the receiving duty's durable token-spend journal
and Introduction-slot floor under one exclusive lease. It imports only the
standard library. Route asks it to burn a token before admitted work; Node
opens and joins the exact receiving root. Both callers use `replay.Open`,
`replay.Ledger`, `replay.Binding`, and `replay.IntroductionSlots` directly,
without retained `route.ClosedSpend*` wrappers. The fixed persisted file names,
headers, crash-tail recovery, and slot time floor remain unchanged. Its owner
tests move with the files; Route and Node retain admission and listener tests.

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
