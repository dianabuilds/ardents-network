# DR-04: Select the first-release multi-host topology

Status: ready-for-agent
State: open
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

- [ ] Current network and deployment behavior is evidenced from the frozen baseline.
- [ ] At least two materially different deployment/reachability topologies are compared.
- [ ] The selected topology has explicit bootstrap, endpoint, certificate, NAT/firewall, churn, partition, Store, and recovery contracts.
- [ ] Deployment ownership, upgrade order, backup/restore, diagnostics, and support boundaries are explicit.
- [ ] Compatibility with DR-03 authority assumptions is reviewed before acceptance.
- [ ] Kubernetes and suppressed transports remain explicitly out of scope.
- [ ] A proposed ADR decision, qualification matrix, and vertical implementation slices are ready for review.

## Blocked by

- W3-00

## Comments

None.
