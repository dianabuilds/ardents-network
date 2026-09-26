---
status: accepted
date: 2026-09-26
---

# ADR-0091 — Retire uncomposed legacy artifacts from the working tree

## Context

The Product Owner directed an architecture-refactoring campaign on the
`refactor/architecture` branch and decided on 2026-09-26 that retained legacy
dies. The affected artifacts have no maintained production caller, exist only
as in-tree evidence, and remain fully available in Git history and accepted
research records. This record executes the private Alpha resolver allowance's
own retirement clause, the bounded OHTTP-adapter reduction boundary of finding
F-22, and the explicit supersede-or-retirement decision that ADR-0061 required
for the non-executable Browser compatibility tree.

## Decision

1. Retire `tests/compatibility/browser-endpoint-v4`, superseding ADR-0061's
   retention. R-117, the accepted Browser ADRs, and the immutable audit
   receipts remain as non-executable evidence.
2. Retire `internal/naming/alpha/private` — the historical fixed-size OHTTP
   Client/Relay/Gateway exchange — together with its package-map row and its
   22 common deadcode allowances, reconciling the lower-authority package-map
   retention statement with ADR-0088. The `internal/naming/alpha` parser and
   persistent floor, `ardents-control inspect-alpha-corpus`, and every Alpha
   destination refusal are retained.
3. Retire the unwired OHTTP Relay/Gateway/Client adapter and signed
   GatewayProfile codec inside `internal/service/reachability`
   (`private_client.go`, `private_gateway.go`, `private_relay.go`,
   `private_wire.go`, `gateway_profile.go`, and their tests) with their exact
   deadcode allowances. The legacy generation-2 `Store.Publish`/`Lookup` API,
   `Issue`, `verifyStored`, the old-format Descriptor decode on reopen, and
   the entire v3 private path are retained until the persisted-root contract
   decision.
4. The #240–#242 experiment trees (R-138 long-running harness, multi-node
   pilot, R-139 multi-agent harness) are already retired on this branch. The
   R-149/R-152 research trees are research evidence and outside this
   decision.

## Consequences

Provenance becomes history-only: no build, package inventory, current
qualification, or normal reading route references the retired trees.
`internal/service/reachability` no longer imports OHTTP; CIRCL keeps its live
v3 consumer. ADR-0061 becomes superseded. No acceptance obligation is
weakened: none of the retired artifacts had a maintained production caller.
