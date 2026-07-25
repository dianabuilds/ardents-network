# Wave 3 decision register

## Ownership

This is an integrator-owned projection of accepted cross-packet decisions.
Parallel research agents propose rows in their own packets and do not edit this
file directly.

Status vocabulary:

- `pending`: a named decision has no accepted recommendation;
- `proposed`: a reviewed packet recommends one option;
- `accepted`: the decision is recorded by an accepted ADR or explicit
  maintainer approval;
- `rejected`: the direction is intentionally not part of the product;
- `deferred`: the direction is outside the selected release scope.

## Decisions

| ID | Decision | Owner packet | Status | Selected direction | ADR | Downstream |
|---|---|---|---|---|---|---|
| W3-D001 | Production realm and Channel Grant authority | DR-03 | pending | — | — | DR-01, DR-04 |
| W3-D002 | Application Messaging addressing and delivery model | DR-01 | pending | — | — | Messaging implementation |
| W3-D003 | Application hosting ownership and lifecycle | DR-02 | pending | — | — | DR-05, Hosting implementation |
| W3-D004 | First-release multi-host topology and reachability | DR-04 | pending | — | — | DR-06 |
| W3-D005 | Direct service boundary and authentication | DR-05 | pending | — | — | Direct interaction implementation or defer |
| W3-D006 | New Application features in first-release scope | Wave 3 synthesis | pending | — | — | DR-06 |

## Cross-packet constraints

| Constraint | Source | Consumers | Status |
|---|---|---|---|
| Application and Operator interfaces remain separate | ADR-0001 | DR-01, DR-02, DR-05 | accepted |
| Principal identity is separate from Channel Grant and Waku Peer ID | ADR-0002 | DR-01, DR-03, DR-04, DR-05 | accepted |
| Authorization precedes durable replay admission | ADR-0003 | DR-01, DR-03 | accepted |
| Large immutable payloads use Content References | ADR-0004 | DR-01 | accepted |
| Multi-provider failure is per-candidate and bounded | ADR-0007 | DR-01, DR-04 | accepted |
| Rollout readiness is the protected composite runtime contract | ADR-0008 | DR-02, DR-04 | accepted |
| Release materials and source are immutable | ADR-0009 | DR-04, synthesis | accepted |

## Open integration questions

| ID | Question | Must be answered by | Blocks |
|---|---|---|---|
| W3-Q001 | Does the first release include Application Messaging? | Wave 3 synthesis | DR-06 scope only |
| W3-Q002 | Does the first release include Application Hosting or Direct Interaction? | Wave 3 synthesis | DR-06 scope only |
| W3-Q003 | Is the first production topology private-LAN, public-direct, or both? | DR-04 | DR-06 |
| W3-Q004 | Is federation supported, explicitly deferred, or rejected for v1? | DR-03 | DR-01, DR-04 |
