---
status: accepted
date: 2026-09-26
---

# ADR-0093 — Retire the Route v2 execution closure

## Context

The cite-or-die sweep (ADR-0091, Product Owner direction of 2026-09-26) and
ADR-0092 removed the Endpoint-side consumers of the former aggregate
Interactive User Route v2. The repository study established the removal
order (F-52): decide the historical wire obligations of
ADR-0026/0034/0035/0062/0072/0081 and the persisted Grant-spend data
treatment first, isolate the minimal historical readers and refusals, then
retire the uncalled files with their exact tests, allowances, and document
owners. The dependency audit at `53f02e64` confirmed the remaining v2
execution files form a closed dead cluster: no kept production file
references their symbols, `internal/node` uses only `route.Profile` for typed
refusals, and the only positive external caller of the sealed Introduction
grammar is the closed `cmd/ardents` service-instance test.

## Decision

1. Historical wire obligations:
   - The sealed Introduction v1 HPKE grammar is retained byte-exact: ADR-0034
     still writes `IntroductionPublic` keys into fresh signed publication
     data, and the retained command test opens sealed introductions. The v2
     envelope framing that grammar needs (`routeEnvelope`, `routeBody`, and
     their append/validate helpers) moves verbatim out of `wire_encoding.go`
     into `sealed_introduction.go`, which becomes its self-contained owner;
     canonical-vector tests prove byte exactness. Its final retirement,
     together with the orphaned publication v1 Introduction instruction
     codecs and the instance `OpenIntroduction`, awaits one superseding
     ADR-0035 record (F-42).
   - The Transit Grant v1 verifier closure is retired: the persisted duty
     root keeps only `{NodeID, GrantID, NotAfter}` digests, so no raw
     grant-bytes reader remains. ADR-0062 stays as the byte and
     signature-domain provenance record.
   - The reciprocal LegBinding v1 grammar (ADR-0072) and its canonical
     vectors are retired; it was wire-only, never persisted, and this record
     is its compatibility disposition.
   - The v2 EntryBinding, Attachment, EndpointTransitBinding,
     credential-relay, and Introduction slot/outcome grammars
     (ADR-0035/0081) are retired; ADR-0092 removed their last production
     consumers.
   - The `Profile` identity `ardents-interactive-route-v2` survives solely so
     `internal/node` refuses exact stale State records without side effects;
     no maintained composition accepts a v2 listener.
2. Retire the fifteen uncalled production files `credential_relay_io.go`,
   `credential_relay_setup.go`, `endpoint_transit_attachment.go`,
   `endpoint_transit_binding.go`, `entry_attachment.go`, `entry_binding.go`,
   `introduction_control_io.go`, `introduction_outcome.go`,
   `introduction_outcome_io.go`, `introduction_slot_registration.go`,
   `native_attachment.go`, `node_binding.go`, `route_binding_v1.go`,
   `transit_grant.go`, and `wire_encoding.go`, with their twelve adjacent
   test files; the new retirement guard asserts the exact absence inventory.
   Shared fixtures relocate to `closed_peer_fixture_test.go`. With
   `entry_attachment.go` gone, the `internal/route` → `internal/entry` import
   edge is removed from the package map.
3. Retire `duty.Store.SpendTransitGrant` (F-53). The persisted v1 receiving
   one-use Transit Grant spend-ledger schema, its validators, and `Replace`
   filtering are retained; their old-root migration-or-refusal decision is a
   separate data disposition.
4. Remove the dominated `nativeDuty` branch in `cmd/ardents-node/node_config.go`
   that assigned `AcceptedProfile = route.Profile` (F-46); the old Node duty
   reservation refuses those selections before it, and closed duties select
   `ClosedRouteProfile`.
5. Remove the `CarrierTCP`/`CarrierQUIC` v1 constants (F-59). The
   closed-opener rejection test keeps the exact literal profile values, and
   Network State keeps its historical signed-record readers.
6. Update the guards: `route_v2_closure_retirement_test.go` asserts the file
   absence inventory, the retained Node refusal comparisons, the retained
   sealed grammar, and the retired constants/operations; the Transit-issuer,
   Initiator-receiving, Node, User-Route, and authority-regression guards
   drop checks whose subject files are absent.
7. Restructure the deadcode allowlist: 75 symbols leave the common dead set;
   exact groups name the retained sealed Introduction grammar (21 symbols),
   the orphaned publication instruction codecs (2), the Node admission
   helpers (2), and the unwired shared Entry/Service Connection leaves (22).
8. `network-route-node.md` module rows and the package-map `internal/route`
   row record the retained grammar and the absent closure.

## Consequences

`internal/route` shrinks to the closed v3 Carrier/wire path, the sealed
Introduction v1 codec, the retained `Profile` refusal identity, and the
closed token issuer; it no longer imports `internal/entry`. No supported
composition changes behavior: every retired symbol had no non-test caller.
The remaining Route v2-era surface is exactly two bounded cards: the F-42
superseding ADR-0035 decision for the sealed grammar and publication
Introduction instruction, and the duty spend-ledger data disposition.
