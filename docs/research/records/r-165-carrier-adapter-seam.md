---
id: R-165
title: Current Carrier adapter seam and import direction
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-165 — Where do current Carrier adapters live?

## Decision this unlocks

Issue [#237](https://github.com/dianabuilds/ardents-network/issues/237) asks for one package and Interface decision for current TCP/TLS and QUIC adapters before the later dual-listener/dial work in [#50](https://github.com/dianabuilds/ardents-network/issues/50). This record proposes retaining a private Carrier module inside internal/route, with Node and route/credential importing route. It does not create a package, change a Carrier, or authorize a future protocol cutover.

## Current contract and falsifiable options

The [Network/Route/Node owner](../../technical/network-route-node.md#adjacent-node-carrier-profiles) selects exactly one State-authenticated closed Carrier profile, TLS 1.3, fixed Route ALPN and pinned Ed25519 peer; TCP/TLS and QUIC have no fallback. The accepted [old-listener closure](../../technical/network-route-node.md#old-native-listener-closure) keeps the current shared/role listeners and closed Node dialer, while its separate #250 removal remains unfinished. The [package map](../../development/package-map.md) already permits Node and route/credential to import route. The [repository layout](../../development/repository-layout.md) and AGENTS.md require a cohesive new package with real Interface, Implementation, tests and non-test caller in one change.

- **H1, retain the in-package module:** the current three transport entrypoints and their private adapters already form a usable seam; Node owns duty/session/pool lifetime, Route owns physical Carrier authentication and cleanup, and no import direction changes.
- **H2, extract a transport package:** moving all physical TCP/TLS/QUIC mechanisms produces a smaller, cohesive external Interface without a route import cycle, compatibility aliases, duplicated validation or wire/admission leakage.
- **H0, postpone the architecture choice:** the affected current callers or accepted #235 disposition are not yet known.

Falsify H1 if an actual current consumer must import Route wire/admission solely to use a physical Carrier and a cohesive extraction can remove that dependency without exposing more Interface. Falsify H2 if the new package must import route's protocol types, or the required move leaves wrappers/aliases and a larger cross-package Interface than the current one. A file count is not a criterion.

## Evidence and exact caller graph

Primary source inspected 2026-09-23: dev@7e381928dd4bd906a1d623ca78a2dedf3738829c, the current owners above, and Go production callers. The local checkout is at another commit, so its affected source files were compared against this dev commit; only the two owning documents differ in the inspected paths. Reproduce with searches for OpenClosedNodeCarrier, ListenClosedSharedCarrier, ListenClosedRoleCarrier, OpenClosedRoleCarrier, ClosedRoleTLSExporter and AcceptClosedRoleTLS in internal/ with tests excluded, then inspect imports and Close/Join paths. These are sourced call-graph facts, not performance measurements.

| Caller or owner | Current Interface consumed | Retained responsibility |
|---|---|---|
| Node forwarding link and session | OpenClosedNodeCarrier, Carrier, ClosedCarrierPool/Lease | Node selects a current State peer and retains session/lease; Route opens one exact TCP/TLS or QUIC byte lane. Pool keys, useful-work retention and lane remapping stay outside physical adapters. |
| Five Node duty listeners and route/credential issuer listener | ListenClosedSharedCarrier, ClosedSharedCarrier/Kind, IsClosedSharedPeerFailure | Route binds one literal shared endpoint, bounds arriving handshakes, authenticates/classifies direct versus outer Node before ARDP; Node/credential admit and join their own handlers, spend roots and duty. |
| route/credential direct issuer | ListenClosedRoleCarrier | Route accepts one authenticated direct role channel; credential owns issuer Control and resource admission. |
| Route bootstrap/source | OpenClosedRoleCarrier | Route's current State selection opens one direct role channel; selected peer, deadlines and nested Route admission remain with Route. |
| Node and Route inner lanes | AcceptClosedRoleTLS, OpenClosedRoleTLS, ClosedRoleTLSExporter | One existing TLS/QUIC exporter fact is consumed by actual admission; the raw connection and transport state stay private to the adapter. |

**Sourced facts:** CarrierProfile and Carrier are defined in route/node_carrier.go, while closed profile IDs and Route ALPN are in route/closed_node_carrier.go. Shared/role adapters reuse private exactPeer, literalEndpoint, closedNodeQUICConfig and role TLS verification; those helpers also serve current Route planning/attachments. ClosedTLSExporter is a Route admission type. The shared listener's accepted classification is used by Node and route/credential together with Route HELLO and bootstrap types. Node's closed_forwarding_carrier.go is not just transport: it owns HELLO, keyed sessions, lanes, queues and retirement. Node's closed_forwarding_listener.go composes State duty, admission, spend, Drain and accepted handlers. Route's ClosedCarrierPool owns State-bound keys and exact useful-work leases, not a generic socket pool.

**Inference:** a new nested carrier package can avoid a Go cycle only by moving or duplicating protocol IDs, CarrierProfile, literal endpoint and TLS verification, or by introducing aliases and forwarding wrappers in route. Moving the role exporter would also force a conversion of Route's admission callback type. Moving Node's session/listener to make the package appear deep would cross the actual duty/admission owner. H2 is technically possible as one broad cutover, but it has no observed new capability or smaller caller Interface for this C0 slice. The present Node→route and credential→route direction is acyclic. No source finding alone proves that a future independent Carrier consumer will not change this calculus.

## Proposed H1 Interface and file disposition

Retain **internal/route** as the package. Its physical Carrier module offers only the already used exact open/listen operations and the byte-lane/accepted-channel values. The Interface includes: one selected profile and literal endpoint, fixed peer key or current-State verifier, certificate, original deadline/handshake limit, TLS/ALPN validation, no fallback, and owned Close result. Transport-specific socket, QUIC stream/connection, handshake reservation and failure classification remain private Implementation. Route callers may use these local functions directly. Node and route/credential import route; route must not import node or route/credential, and physical adapters must not read State roots, select a peer, spend a token, interpret ARDP lanes or decide duty drain.

**Keep with physical adapters in route:** closed_node_carrier.go, closed_shared_carrier.go, closed_role_carrier.go, closed_role_carrier_client_linux.go, node_carrier_quic.go, closed_role_tls*.go, tls_adapter.go, endpoint_literal.go, and their behavior tests. **Keep with Route admission/ownership:** closed_carrier_pool.go, closed_admission_channel.go, bootstrap/Source/lane grammar and exporter consumption. **Keep with Node duty/session ownership:** closed_forwarding_carrier.go, closed_forwarding_listener.go and other duty listeners. **Keep with issuer:** route/credential/closed_token_listener.go and its bounded issuer operation. This is an ownership map, not a mandate to rename every file.

The old ListenNodeCarrier and exclusive helpers are governed by #235/#250. #237 neither waits for a green #250 nor incorporates its deletion; a frozen or failed #250 run cannot be laundered through this docs decision. No new package, doc.go, import entry or code migration cohort is selected by H1. If a later actual consumer justifies H2, it needs a separate paired cutover moving the physical adapters, profile/ALPN and shared verification as one buildable cohort, removing old implementations and updating doc.go, package-map and caller tests in the same change. A partial package with route aliases or test-only callers is not a valid intermediate state.

## Verification, trade-off and disposition

The chosen H1 needs no runtime test to prepare this decision. After acceptance, update the Network/Route/Node owner and package map with this exact Interface/import rule; #237 can then close on an owner-backed contract. A later affected N14-D/E implementation must exercise the ordinary Node forwarding and issuer paths on **both** TCP/TLS and QUIC: exact State profile/key, invalid peer and profile refusal before effects, cancellation during handshake, arrival-bounded shared handshake slots, direct/outer classification, listener Close, in-flight handler Join, physical Carrier close errors and no fallback. Existing module tests are oracles for the unchanged paths, not proof of new dual-listener behavior or installed qualification.

Recommend H1 with high confidence in the present caller and import graph, moderate confidence in future reuse. The strongest objection is that Route remains a broad package; that alone does not justify replacing a working seam with a new external Interface. A future real consumer can reopen the choice with evidence. This draft changes research provenance only; it is **not** an accepted technical owner contract or implementation authorization.