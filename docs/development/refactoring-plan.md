# Stage 8 refactoring and retirement plan

Status: **active.** This is a short-lived engineering control, not a technical
specification or project history. Current behavior belongs to the technical,
reference, package-map, ADR, and research documents named below. This plan is
deleted when M12 and M14 are either completed or explicitly transferred to a
later accepted scope.

## Current truth

- [Package map](package-map.md) is the checked source of Go package ownership
  and permitted imports.
- [Private naming and Namespace](../technical/naming.md),
  [Endpoint and Service runtime](../technical/endpoint-service-runtime.md),
  [Network State, Entry, Route, and Node](../technical/network-route-node.md),
  and [Release, Update, and Authority Custody](../technical/release-update-custody.md)
  own current implementation facts and limitations.
- [Command reference](../reference/commands.md) owns the maintained command
  surface.
- Accepted ADRs and the one open R-092 research record remain decision and
  evidence provenance. Historical Stage material is recoverable from Git, not
  a second specification.

## Completed Stage 8 waves

| Wave | Current disposition |
|---|---|
| M0 governance | Complete. Engineering policy, testing profiles, package map, dependency register, and architecture gate are the current controls. |
| M1 release; M2 update | Complete within the bounded technical-tracer scope. No supported installer or automatic update lifecycle is claimed. |
| M3 State; M4 Duty/Resource | Complete. State and Duty own their durable roots; Resource remains Linux-only and fails closed elsewhere. |
| M5 Namespace; M6 Resolution | Complete. Namespace has cohesive nested modules and root Resolution views only; Resolution consumes those opaque views. |
| M7 Entry; M8 Route; M9 Publication/Connection | Complete within the selected closed-test-network scope. A peer-facing Route runtime, public deployment, and capacity qualification remain future work. |
| M10 Broker/Endpoint | Complete for the explicit `generic/unqualified` Broker contract. No platform-isolation claim is made. |
| M11 Node | Complete within the closed-test-network scope. R-092 remains a future operating-profile measurement, not a Stage 8 blocker. |
| M13 commands | Complete. `ardents`, `ardents-custody`, and `ardents-node` are the maintained command adapters; unselected Custody operations remain unexposed. |

The completed scoped Route evidence is functional only: it proves a mixed
closed network carried bytes end-to-end; it does not qualify State/Entry,
privacy, independent operation, public deployment, or a Node host profile.

## M12 — Custody: active boundary

`internal/custody` is a real encrypted Authority-Vault owner. It owns the
accepted envelope, bounded secret use, Bundle export/test restore, encrypted
quarantine/reconciliation, exact local floors, and sealed Namespace signing.
Release, Update, Endpoint, and diagnostics never receive the root material.

M12 is **not** a supported platform or operator lifecycle. The remaining work
requires a new accepted scope before implementation because it would select
hard-to-reverse product and platform behavior:

- a supported Windows/Ubuntu storage, permissions, crash, and power-loss
  profile;
- a complete supported Custody lifecycle and command surface;
- any Name-scoped predecessor-to-successor proof that would allow local Vault
  demotion after Authority replacement.

Until then, the current technical contract and its explicit limitations are in
[Release, Update, and Authority Custody](../technical/release-update-custody.md).
DA-08 and DA-09 in the [decision-authority register](stage-8-decision-authority-register.md)
remain the required route for any expansion.

## M14 — retirement and current truth: active boundary

M14 has retired the generic `tests/live/` tree, obsolete laboratory packages
and commands, closed disposable experiments, historical source diagnostics,
the broad Namespace compatibility façade, and redundant Stage 8 entry/
compatibility records. The only active experiment is R-092.

Remaining M14 work is deliberately narrow:

1. Keep reducing Stage-only documentation after every unique current fact has
   a named technical/reference/ADR owner; do not delete the M12 stop
   conditions while they remain active.
2. Perform a final repository audit after the active M12 boundary is settled:
   package/import graph, tracked-artifact residue, current-reader routes,
   documentation links, and selected test profiles.
3. Delete this plan, the target architecture, and the decision register only
   when no active migration or stop condition relies on them.

DA-11 remains the authority for any newly discovered evidence or laboratory
artifact with a claimed reproduction or Qualification duty.

## Verification

Use `make quick-check` while changing code or current documentation and
`make check` before integration. On 2026-08-24, after Namespace façade removal
and current-document retirement, the static check, `make e2e`, and the full
`make test-race` profile passed. The race profile ran through the repository
Makefile, which sets `GOENV=off`; this avoids an unrelated user-global stale
Waku/RLN linker flag and does not add that dependency to the project.

No test result upgrades the scoped technical tracer into a supported platform,
privacy, public-network, independent-operator, or product-lifecycle claim.
