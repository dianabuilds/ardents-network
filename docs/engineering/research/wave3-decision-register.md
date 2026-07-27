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
| W3-D001 | Production realm and Channel Grant authority | DR-03 | accepted | One non-federated realm has one deployment-owned Realm Authority Principal on a designated authority Node; Operator-only HPKE bundles and approved-host attestations drive fresh-generation cutover, suspect members are fenced, and same-realm recovery depends on an independent monotonic checkpoint repository | ADR-0011 | DR-01, DR-04 |
| W3-D002 | Application Messaging addressing and delivery model | DR-01 | proposed | Opaque authority-backed conversations with Operator-owned membership, five bounded Application operations, durable local send acceptance, recipient-local cursor order, at-least-once transport, Node receipts distinct from Application consumption, and Content References for large immutable data | Proposed ADR-0015 | Messaging implementation |
| W3-D003 | Application hosting ownership and lifecycle | DR-02 | proposed | One owner-qualified leased Hosted Service over an immutable Operator-approved managed-workload profile; the profile pins service type, Hosting owns durable intent/profile revisions, and workload/readiness/ingress/publication remain derived | Proposed ADR-0012 | DR-05, Hosting implementation |
| W3-D004 | First claimed multi-host release topology and reachability | DR-04 | proposed | Exactly three operator-owned Linux amd64 hosts with `private_lan` and `public_direct` variants, at least two bootstrap/Store Nodes, one designated authority slot, and bounded workstation-side topology operations including monotonic fence/rejoin | Proposed ADR-0013 | Future multi-host DR-06 scope; excluded from independent stabilization release |
| W3-D005 | Direct service boundary and authentication | DR-05 | proposed | Ardents v1 ends after authenticated, authorized, privacy-filtered Discovery resolution; the Application/service own dialing, TLS, service credentials, authorization, protocol limits, retry, and errors, and no Direct Service adapter or Access-Grant translation is added | Proposed ADR-0014 | Discovery implementation and handoff qualification; Direct Service adapter rejected for v1 |
| W3-D006 | New Application features in first-release scope | Wave 3 synthesis | proposed | No new Wave 3 Application, authority, Direct Service, or real multi-host capability enters the independent stabilization release; its DR-06 scope is the fifteen existing capabilities enumerated in the synthesis, while Wave 3 designs proceed on post-stabilization tracks | No new ADR; scope decision in Wave 3 synthesis | DR-06 |

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

## Resolved integration questions

| ID | Question | Resolution | Blocks |
|---|---|---|---|
| W3-Q001 | Does the first release include Application Messaging? | No; post-stabilization after ADR-0011/0015, authority implementation, recovery, and matching-commit evidence | Nothing in stabilization DR-06 |
| W3-Q002 | Does the first release include Application Hosting or Direct Interaction? | No; Hosting is post-stabilization, while Direct Service adapter is rejected and only the independent Discovery handoff may be implemented later | Nothing in stabilization DR-06 |
| W3-Q003 | Is the first claimed multi-host production topology private-LAN, public-direct, or both? | DR-04: both, as separately qualified variants of the same exact three-host shape | Future multi-host qualification only |
| W3-Q004 | Is federation supported, explicitly deferred, or rejected for v1? | Unsupported for v1; future federation or MLS requires new R2 research | Nothing in Wave 3 |
