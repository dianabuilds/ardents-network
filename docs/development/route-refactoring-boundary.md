# Route package refactoring boundary

Status: working architecture analysis for the isolated refactoring branch. The
capsule, terminal body, ARDP framing, replay, Carrier, and client-path
extractions below are implemented and registered in the package map; the
former v2 execution closure is retired by the ADR-0093/ADR-0094 chain and
pinned absent by the architecture suite, so this document is a realized
boundary record, not a new Route contract or C0 execution ledger. The
accepted Route, Carrier and client contracts govern behavior.

## Current Linux owner graph

On the Linux amd64 candidate after the replay extraction, completed #252
v1 Node Carrier listener retirement, and ADR-0092 Endpoint/credential cleanup,
the reconciled package graph at `53f02e64` records 74 production and 76
test files in `internal/route`. Its capsule, ardp,
terminal, and replay children have respectively 6/1, 3/1, 9/4, and 7/3
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
| `writeAll` | `wire_encoding.go` | Retained v2 Route binding/relay I/O; no direct closed-v3 caller in the current source | Keep its full-write rule with the audited v2 closure. `route/ardp` has its own `writeAll` for v3 frames. |

The selected non-closed production files contain no reference to a `Closed*`
identifier; the existing `closedEntryOpener` in `entry_attachment.go` is a
separate local helper. This is a source-level observation, not proof that a
mass move compiles. The type-use audit confirmed the shared declarations above
but could not fully type-check external imports in isolation, so a package move
must still be checked by Linux and Windows builds and behavior tests.

### Exact Carrier extraction blockers at `53f02e64`

The proposed Carrier owner is a **physical transport and authentication**
boundary. It needs `ClosedNodeCarrierRequest`, direct-role and shared
listeners, TCP/QUIC connection close, `literalEndpoint`, `exactPeer`, the
role TLS helpers, and the physical pool/lease. These functions use one
`CarrierProfile` and the current `ClosedRouteProfile` ALPN. The candidate is
not an atomic prefix move:

| Mixed declaration | Current use | Required import direction |
| --- | --- | --- |
| `node_carrier.go` declares `CarrierProfile` and `Carrier` beside the retired v1 `CarrierTCP`/`CarrierQUIC` constants. | Current Node/Route/Credential code uses the byte-lane type. At this HEAD, neither v1 constant has a non-test Go caller; only `CarrierTCP` appears in one old-profile rejection test. The separate State parser still reads literal v1 identities. | Put the current physical lane/profile with Carrier. Preserve the negative test using an explicit old profile value, and retire the unused symbolic v1 constants with the move; this does not delete State's historical signed-record reader. Do not make Carrier expose a v1 dial merely to move this file intact. |
| `closed_role_carrier.go:ClosedRoleTLSExporter` returns `ClosedTLSExporter`, currently declared in `closed_admission_channel.go`. | The exporter is obtained from authenticated TLS/QUIC and then borrowed by receiving admission. | The exporter function signature belongs with the authenticated Carrier; receiving admission may depend inward on that function type. Carrier must not import the admission/spend owner. A local function signature is also possible if it keeps one exact byte contract. |
| `closed_role_tls*.go` uses `ClosedRouteProfile` and `exactPeer`; `closed_role_carrier_client_linux.go` uses the same role TLS config for both TCP and QUIC. | Both direct role and shared listeners enforce the same TLS identity/refusal rule. | Move TLS identity, ALPN and physical transport together or first expose one narrow lower-level owner. Leaving role TLS in `route` while Carrier imports it reverses the desired dependency. |
| `endpoint_literal.go` validates both current physical dial/listen endpoints and client selection inputs. | It prevents DNS resolution and malformed port fallback before network effects. | One lower-level literal-address rule may be called by Route's client selection; copying it into both owners would create divergent refusal behavior. |

Realized: the thirteen production files of this cohort now form
`internal/route/carrier` (package `carrier`), a leaf whose only non-standard
dependency is `github.com/quic-go/quic-go`. Every blocker above was resolved
as proposed: `ClosedTLSExporter` moved to the Carrier side and Route qualifies
it inward; role TLS, the `ClosedRouteProfile` ALPN, and `exactPeer` moved with
the physical transport; `literalEndpoint` was exported as
`carrier.LiteralEndpoint` so Route's client selection calls the one
literal-address rule; the retired v2 `Profile` refusal identity stayed in
Route (`closed_route_profile.go`) beside its Node typed refusal. Route, Node,
Endpoint, Credential, and both node commands import the leaf directly without
delegating wrappers, and the pure Carrier behavior tests moved with their
owner. The old Route v2 execution closure is retired; only its typed refusal
identity survives, pinned by
`internal/architecture/route_v2_closure_retirement_test.go`.

`internal/route/credential` imports parent `internal/route` in three current
production files: `closed_token_listener.go`, `closed_token_bootstrap.go`, and
`closed_token_admitted.go`. They serve the live closed issuer's listener,
bootstrap and admitted exchange. ADR-0092 removed the old client and message
imports. Any split must move these live consumers or provide a lower-level
acyclic Carrier/channel contract. A temporary `route` wrapper importing a
child that imports `route` would create a cycle (F-30).

## Retired v2 source closure versus the selected v3 path

The protected Route owner selects `ardents-interactive-route-v3` for C0. The
old `ardents-interactive-route-v2` identity survives only as the typed
retired `route.Profile` constant in `closed_route_profile.go`:
`node/admission.go` and `node/duty_server.go` refuse it before starting a
duty, and State keeps its own signed-record projection separate. ADR-0089's
no-new-old-start boundary is intact; nothing authorizes another accepting C0
Route.

The former v2 source closure is retired, not retained. ADR-0093 deleted the
four old Entry/Transit attachment files, the relay/Introduction I/O closure,
`transit_grant.go`, `node_binding.go`, `route_binding_v1.go`,
`native_attachment.go`, and `wire_encoding.go` together with the historical
Grant verifier and LegBinding decoder; `sealed_introduction.go` survived it
byte-exact and was retired by ADR-0094 with the old Service Introduction
plaintext after their superseding record. The historical wire/vector
obligations were decided in those ADRs (F-52), the persisted Grant-spend
treatment by ADR-0107's bounded version-2 duty-root conversion (F-53), and
the Invite subsystem by ADR-0106 (F-08).
`internal/architecture/route_v2_closure_retirement_test.go` pins the exact
absence of every closure file and the survival of the refusal identity and
the persisted-data contracts. The current
`docs/technical/network-route-node.md` contract records
`ardents-interactive-route-v2` as the former native Route grammar behind
effect-free typed refusals and points C0 readers to closed v3. The
[current closure inventory](c0-component-reconstruction.md#former-v2-execution-closure-after-adr-0092)
records the row-by-row disposition and supersedes the pre-ADR-0092 Endpoint
caller narrative in earlier revisions of this document.

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

Realized for the client path: the outgoing cohort — the retained Source prefix
with its multiplexed lanes, credit accounting and child lifetimes, the
one-attempt issuer-bootstrap exchange and its plan validation, the read-only
recipient inspection, and the client-side Introduction/JOIN/resolution/
submission operations — now forms `internal/route/client` (package `client`),
a leaf importing Route, live State, and the ardp, capsule, carrier and
terminal leaves; Route never imports it. The seam kept one import direction by
exporting the eight shared Route facts (`ClosedAdmissionFrameBytes`,
`ClosedClassLifetime`, `ClosedControlPurpose`, `ClosedDutyAdjacent`,
`ClosedForwardChildren`, `ClosedRoleDomainInitiator`,
`ClosedRoleDomainIntroduction`, `ClosedOuterLaneCredit`), each keeping its
single definition in Route, and by moving the terminal-priority switch to the
client side with a new `ClosedOuterBridgeLane.BeginTerminalWrite` method for
the outer-bridge case. The receiver-side `ClosedBootstrapController`
(`closed_bootstrap.go`) stayed in Route as a node duty, exactly as the
ownership-scale table required. Endpoint, Qualification and Node behavior
tests import the leaf directly without delegating wrappers, and the 25
behavior test files moved with their owner. What remains of the Route split
is the v2 execution-closure retirement, which stays analysis.

Before a split, resolve each shared declaration above and map every public
`route.Closed*` caller to its prospective owner. Move behavior tests with the
owner, update the package map and deterministic profile in the same change,
and require compiling intermediate commits. The final candidate still needs
the full Ubuntu gate and both Carrier journeys before integration.
