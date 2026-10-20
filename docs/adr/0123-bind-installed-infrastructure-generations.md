---
status: proposed
date: 2026-10-21
---

# ADR-0123 — Bind each installed infrastructure invocation to its own generation

## Gap and authority

ADR-0119 authenticates an Endpoint program and protected Endpoint resources.
`installation.ProgramTarget`, `GenerationTarget`, enrollment's closed inventory,
the fixed unit and private startup consumer all enforce that scope. A manifest
Node companion is neither a Node Release authorization nor a Source generation.
ADR-0120 authorizes its recorded qualification operation, not an unspecified
infrastructure target or another bootstrap acceptance. No accepted decision
currently supplies that extension. This proposal is **not accepted** and grants
no installation, signing, execution or migration permission.

## Recommended decision

1. Keep Release, Enrollment, Installation, Network, Source, Node and native
   process owners separate. Add a closed infrastructure generation contract,
   initially for one Direct-Origin Source; later admission of each of the five
   Node invocations requires its complete inventory and native evidence.
2. Use a distinct executable target `ardents/linux-amd64/node` and one descriptor
   target `ardents/linux-amd64/infrastructure/<invocation>`, where invocation is
   exactly `source`, `forwarding`, `issuer`, `descriptor`, `introduction` or
   `join`. Require two fresh coherent Release authorizations from the same
   frozen metadata/local/reference observation and retained floors. Do not
   reinterpret either Endpoint target, overload Release custom fields, or
   derive successor permission from the initial pin or Node companion.
3. Authenticate a canonical, finite descriptor and exact program/unit-template
   bytes as specified by the [technical proposal](../technical/network-route-node.md#proposed-installed-infrastructure).
   Local configuration, runtime credentials, State and other mutable histories
   are separately bound installation inputs, never signed release resources.
   Resource-only change requires a strictly forward generation; a changed
   command adapter also requires fresh exact artifact admission.
4. Keep existing enrollment-v3 and Endpoint inventories unchanged. An explicit
   infrastructure initial verifier must check an independently delivered pin,
   closed inventory and the actual running infrastructure executable before
   producing initial provenance. Define its separate closed inventory; never
   accept a new companion by relaxing v3 or its running-artifact gate. The
   exact initial format and its independent verification are part of the
   infrastructure artifact implementation, not an inferred existing capability.
5. Bind one non-root account, fixed unit and original process to one selected
   generation. Deny work before physical interruption; join original domain and
   native users before resource release. No automatic restart, update, rollback,
   adoption, root reset or key reload. Unknown original completion refuses a
   replacement and preserves histories and failures. Initially bound installed
   Node hosting to one active invocation per host/provider allowance, a common
   `ardents-node` account and one retained Hosting root across the five role
   units; Source has its separate account and no Hosting allowance. This smaller
   installed contract must not duplicate provider capacity or change Hosting policy.
6. Before any isolated installed execution, obtain a completed public bootstrap
   receipt explicitly covering these new targets, actual commands/artifacts and
   remaining qualification limits, accepted by the Product Owner. Preserve the
   independent participant first-pin and effective-family conditions. Existing
   ADR-0120 receipts cannot silently widen their scope.

## Impact and acceptance

This is a consequential persisted/artifact/authority extension, owned by Software
acceptance with the Product Owner deciding the scope. No new technology or
universal Identity, Control, PKI, repository or signing service is selected.
Existing Endpoint bindings, journals and floors are neither converted nor
reused. Native mechanism reuse requires explicit imports and new exact consumers;
the fixed Endpoint policy is never treated as a generic infrastructure policy.

Accept the extension only with explicit Product Owner decision; then verify
independent byte oracles, wrong-target/pin/resource/coherence refusals, retained
floors, initial provenance versus successor authority, exact native startup and
interruption/recovery obligations from the technical proposal. Accepting this
ADR would still neither select implementation nor qualify an installed system.
