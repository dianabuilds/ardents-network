# DR-04: Select the first-release multi-host topology

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R2

## Parent

`../PRD.md`

## What to build

Choose the minimum supportable private-LAN, public-direct, or combined
multi-host topology for the first release. Define bootstrap and DNS
availability, NAT/firewall and advertised endpoint contracts, WSS certificate
ownership, churn and partition behavior, Store availability and recovery,
deployment ownership, upgrade ordering, observability, and operator recovery.

The result must be a finite support matrix that DR-06 can qualify. It must
remain compatible with the accepted Channel Grant authority model without
inventing that authority inside deployment research.

## Acceptance criteria

- [x] Current network and deployment behavior is evidenced from the frozen baseline.
- [x] At least two materially different deployment/reachability topologies are compared.
- [x] The selected topology has explicit bootstrap, endpoint, certificate, NAT/firewall, churn, partition, Store, and recovery contracts.
- [x] Deployment ownership, upgrade order, backup/restore, diagnostics, and support boundaries are explicit.
- [x] Compatibility with DR-03 authority assumptions is reviewed before acceptance.
- [x] Kubernetes and suppressed transports remain explicitly out of scope.
- [x] A proposed ADR decision, qualification matrix, and vertical implementation slices are ready for review.

## Blocked by

- W3-00

## Comments

Accepted 2026-07-25 after integrator review and one revision cycle.

Evidence:

- `docs/engineering/research/multi-host-reachability.md`
- Proposed `docs/adr/0013-bounded-multi-host-reachability.md`
- canonical capability remains `partial/no/no/no`

Selected exactly three Linux amd64 hosts with separately qualified
`private_lan` and `public_direct` variants, at least two bootstrap/Store Nodes,
one designated DR-03 authority slot, and five bounded workstation-side
topology operations. Rejected manual-only orchestration and a long-running
cluster controller. The revision added monotonic fencing/rejoin, the frozen
workstation-side `ardentsctl --ssh` signer/session model, separate authority
and checkpoint failure domains, clock bounds, and explicit upgrade ordering.

Validation:

- Wave 3 packet and ADR contract review
- compatibility review against accepted DR-03 research recommendation
- documentation, architecture, and capability-catalogue tooling gates
