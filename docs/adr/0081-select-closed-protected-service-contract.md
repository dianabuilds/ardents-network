---
status: accepted
date: 2026-09-07
---

# Select the closed protected text-Service contract

## Context

ADR-0078 chooses the split-circuit architecture but leaves its exact admission,
wire, Application and migration contract to design. The Product Owner selects
the complete first scheme in a closed network, text publication/read with
confinement on Ubuntu, and subsequently Target Link first. The delegated design
must settle mechanisms before implementation tasks.

[R-152's contract assessment](../research/records/r-152-closed-scheme-contract.md)
records source review, component evidence, rejected alternatives and the cost
model. It supports a bounded design selection, not implementation qualification.

## Decision

Adopt the [workload](../product/protected-service-workload.md),
[protected forwarding protocol](../technical/protected-route-protocol.md),
[private admission](../technical/private-admission.md) and
[Application confinement](../technical/application-confinement.md) as the
closed successor design contract. The [qualification owner](../development/privacy-qualification.md)
fixes its finite budgets, observer checks and migration.

- Preserve ADR-0048's TCP/TLS and QUIC v1 transport families behind the one
  Route owner. For this successor only, supersede its Carrier profile/binding
  clauses with `ardents-carrier-tcp-tls-v2`, `ardents-carrier-quic-v2` and the
  exact generation-3 HELLO binding. Preserve the old signed identities and
  LegBinding for current generation 2; no old session carries new work.
  Use authenticated forwarding generation 3, fresh inner TLS, recipient-only
  Introduction join material and separately bound logical Connection and
  Attachment contexts. Coalesce the initial Instance/Continuity authentication
  records while retaining all receiving checks before Application effects.
  No automatic generation or transport fallback.
- Select Go 1.26.8 and CIRCL v1.6.5 ordinary blind RSA
  SHA384PSSDeterministic with the exact RFC 9578 type-2 composition. Keep the
  existing reviewed HPKE suite and quic-go v0.62.0. This admits the new library
  use to design; each actual build still needs current dependency acceptance.
- Existing pinned closed State authority authenticates one finite profile per
  Epoch. A separate offline admission authority allocates context-scoped hourly
  permissions. An appointed issuer has finite durable reservations; receivers
  validate and spend offline with exclusive durable ledgers and local limits.
  Neither authority acquires public naming, Service, Release or voting powers.
  Custody retains the separate closed-admission signing purpose; credential
  prepares its sealed allocation request and never receives that private key.
- Select Ubuntu 24.04 LTS x86-64, systemd 255 and cgroup v2. Root-owned installed
  units confine fixed workers before any Grant or remote effect. Endpoint
  remains a separate unprivileged service; one Publisher worker owns one
  immutable snapshot and bounded concurrent streams.
- Select the Application Interface version-2 Connection boundary, fixed text
  Application module and thin ardents-text adapter described by the confinement
  owner. These are justified future package/command responsibilities; actual
  additions require their complete package-map, imports, callers and tests.
- First complete acceptance uses Target Link. Canonical Names follow a
  separately accepted real producer and proof composition, preserving
  ADR-0054; no temporary alias or simulated close is admitted.
- Install and validate the complete successor before atomic local adoption.
  Quiesce/drain old work, preserve all authority/conflict/resource floors, and
  reject incompatible peers. A retired executable cannot reopen old protection.

The Node-outer OPEN allocation is amended by
[ADR-0082](0082-bind-bootstrap-restriction-to-node-child.md), which binds a
mandatory issuer-bootstrap restriction and explicitly retires its former form.

## Consequences

This selects one engineering contract for implementation tasks. It does not
start implementation, change current C0 acceptance, deploy a public network,
establish operator independence or qualify anonymity. Current generation-2
facts remain documented until their explicit migration. Proposed public
State/update decisions under R-149 are not silently accepted by this ADR.

The first scheme retains participation visibility and both-end/active traffic
correlation. Protocol confidentiality, separation, confinement and bounded
failure are hard acceptance gates. No filler generator is selected. The cost
model has narrow warm-response headroom; a failing implemented gate returns
the affected design to review, with no weaker accepting path or hidden budget
relaxation. Public autonomy and a stronger correlation claim need their own
accepted research decisions.
