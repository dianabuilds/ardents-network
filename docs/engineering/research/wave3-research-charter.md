# Wave 3 deep-research charter

## Purpose

Wave 3 resolves decisions that are too expensive or security-sensitive to
discover during implementation. It covers Application Messaging, Application
Hosting, Production Channel Grant authority, multi-host reachability, and
direct service interaction.

The stage produces research packets, proposed ADRs, and approved vertical issue
breakdowns. It does not implement the selected product designs and does not
qualify a release.

## Baseline admission

The frozen Wave 3 product baseline is
`main@8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`, the final post-R1 product
commit.

The Wave 3 documentation/governance preparation revision is
`main@e1e5299bf3b83cb534605811c032eeb2fe1bdd0c`.

W3-00 reconciles R1 tracker bookkeeping, canonical capability truth, the global
plan, and this research program in a later documentation/governance commit.
Research packets bind product claims to the frozen product commit rather than
to that later preparation revision. This avoids a self-referential source SHA
while keeping the assessed implementation immutable.

An agent may start only when:

- the preparation revision is present on `main` and `main` equals
  `origin/main`;
- the worktree is clean;
- the R1 parent is completed and its AIJ, OCS, and FEC tasks are tracked;
- the canonical capability catalogue reports the frozen product commit;
- capability, documentation, architecture, and tooling gates pass;
- no capability is promoted to `Q=yes`.

## Normative vocabulary and decisions

All packets use the language in `CONTEXT.md`. The following accepted decisions
are fixed inputs unless a new ADR explicitly supersedes them:

- ADR-0001: separate Application and Operator interfaces;
- ADR-0002: Principal-centered identity, exact Access Grants, and one-hop
  Delegation;
- ADR-0003: authorize private messages before durable replay admission;
- ADR-0004: owner-qualified Object and Manifest identity;
- ADR-0005: recoverable one-time ticket handoff;
- ADR-0006: transactional Compose rollout journal;
- ADR-0007: multi-provider fetch terminal semantics;
- ADR-0008: composite rollout readiness;
- ADR-0009: immutable and verifiable release materials.

## Shared research contract

Every DR-01 through DR-05 packet must:

- state one user outcome and explicit non-goals;
- reconstruct the current supported-interface journey;
- distinguish implementation, reachability, operability, and qualification;
- classify all dependencies and failure ownership;
- compare at least two materially different designs;
- select a small caller interface and a deep internal seam;
- specify authority, state ownership, privacy, abuse bounds, restart, recovery,
  compatibility, migration, and operator behavior;
- include an acceptance matrix and vertically sliced issues;
- recommend implement, prototype, ADR, defer, or reject;
- keep `Q=no` until DR-06 obtains complete matching-commit evidence.

## Research lanes

### DR-03 — Production Channel Grant authority

- Output: `channel-grant-authority.md`
- First-wave blocker: none.
- Downstream: DR-01 and private multi-host assumptions.
- Required inputs: ADR-0002, ADR-0003, ADR-0005, private discovery/data
  implementation, backup/restore contracts.
- Boundary: authority lifecycle only; no Application Messaging API.
- Required decision: trust root, realm model, issuance/delivery/
  acknowledgement/recovery, membership and generation lifecycle, channel-class
  separation, audit, backup/restore, federation and migration position.

### DR-01 — Application Messaging

- Output: `application-messaging.md`
- Blocked by: accepted DR-03 authority result.
- Downstream: messaging implementation and large-message Content integration.
- Required inputs: ADR-0002, ADR-0003, ADR-0004, ADR-0007, DR-03, current Waku
  and private messaging implementation.
- Boundary: no arbitrary Waku publishing and no leakage of selectors,
  encryption, replay, Store queries, or retry into the SDK.
- Required decision: addressing, conversation/membership lifecycle, delivery,
  acknowledgement, deduplication, ordering, expiry, receive model,
  backpressure, quotas, Content References, revocation and audit.

### DR-02 — Application Hosting

- Output: `application-hosting.md`
- First-wave blocker: none; use the accepted Application Discovery locator
  boundary as an input.
- Downstream: DR-05 and hosting implementation.
- Required inputs: ADR-0001, workload/hosting/publication implementations,
  ingress policy, Application Discovery packet.
- Boundary: one orchestration interface above workload, hosting, publication,
  and ingress; no exposure of their independent internals.
- Required decision: registration versus owned workload, ownership, lease,
  readiness, renewal, drain, crash/restart, protocols, local/network modes,
  and withdrawal.

### DR-05 — Direct service interaction

- Output: `direct-service-interaction.md`
- Blocked by: accepted DR-02 hosting result.
- Downstream: direct client adapter or explicit discovery-only scope.
- Required inputs: ADR-0001, ADR-0002, Application Discovery packet, DR-02,
  endpoint publication and TLS configuration.
- Boundary: define where Ardents ends; do not create a generic service mesh.
- Required decision: discovery-only versus adapter, Principal authentication,
  service authorization, TLS identity/rotation/pinning, limits, retry/errors,
  and application-protocol ownership.

### DR-04 — Multi-host reachability

- Output: `multi-host-reachability.md`
- First-wave blocker: none.
- Cross-check before acceptance: DR-03 authority result.
- Downstream: production support matrix and DR-06.
- Required inputs: ADR-0006, ADR-0008, ADR-0009, network foundation, deployment,
  DNS/bootstrap, WSS, backup/upgrade/rollback.
- Boundary: private-LAN/public-direct first-release topologies; no Kubernetes
  and no suppressed transports.
- Required decision: topology, availability, NAT/firewall, advertised
  endpoints, certificates, churn/partition/Store recovery, ownership, upgrade
  order, observability and minimum support matrix.

## Parallel execution

The safe first parallel wave is DR-03, DR-02, and DR-04. DR-01 starts only
after DR-03 acceptance. DR-05 starts only after DR-02 acceptance.

Parallel agents do not edit `wave3-decision-register.md`. Each packet proposes
decision rows, and the integrator updates the shared register after review.

## Synthesis

After all five packets are accepted, one integrator:

- reconciles overlapping identity, authority, endpoint, and topology choices;
- updates the decision register;
- identifies required ADRs and their order;
- proposes which new Application features, if any, enter the first release;
- preserves an independent release path for existing stabilization scope;
- produces the final dependency-ordered implementation and DR-06 input.

## Exit gate

Wave 3 is not complete while any open question can change a selected external
interface, trust root, persisted state, wire compatibility, migration, or
first-release support topology.
