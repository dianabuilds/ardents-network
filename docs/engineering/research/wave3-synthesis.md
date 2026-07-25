# Wave 3 synthesis: compatible model and release scope

## Status and recommendation

- Status: accepted Wave 3 synthesis recommendation
- Date: 2026-07-25
- Frozen product baseline:
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Inputs: accepted DR-01 through DR-05 research recommendations
- Decision state: ADR-0011 through ADR-0015 remain Proposed

Wave 3 is research-complete. The five selected designs form one compatible
model, but none is implemented or qualified at the frozen product baseline.
The immediate release path therefore remains the existing stabilization scope.
Application Discovery, Application Messaging, Application Hosting, production
Channel Grant authority, Direct Service handoff qualification, and real
multi-host deployment do not enter that release.

This is not a rejection of the selected designs. It prevents unfinished
features from expanding DR-06 implicitly. The first release that later claims a
Wave 3 capability must use the bounded v1/first-release contract in that
capability's packet. In particular, DR-04's “first release” means the first
release that claims `deployment.multi-host`, not the independent stabilization
release.

## Reconciled product model

```text
one Realm Authority Principal
  -> distinct discovery/data/application/control Channel Grant generations
  -> one designated authority operations slot + independent checkpoint root

Application Principal + exact Access Grant
  -> Application Discovery.Resolve -> bounded locator handoff
  -> Hosted Service lease          -> Node-owned managed workload projection
  -> Application conversation      -> Messaging-owned inbox/outbox projection

three-host multi-host contract
  -> host-local Nodes remain state owners
  -> workstation-side topology operations coordinate rollout/fence/recovery
```

The model has five non-overlapping ownership boundaries:

1. DR-03 alone owns realm membership, Channel Grant issuance, generation
   activation, revocation, fencing acceptance, and anti-rollback checkpoint
   truth.
2. DR-01 owns conversation addressing, message/inbox/outbox/subscription state,
   delivery projection, and Application Messaging semantics. It consumes, but
   never issues, DR-03 Application-channel grants.
3. DR-02 owns one owner-qualified leased Hosted Service intent and immutable
   profile-revision archive. Workload, readiness, ingress, and publication are
   derived projections, not separate Application controls.
4. Application Discovery owns authenticated, authorized, privacy-filtered
   locator disclosure. DR-05 ends Ardents direct-service responsibility at
   that handoff; the service owns TLS client use, credentials, authorization,
   protocol, retry, and errors.
5. DR-04 owns bounded deployment topology intent, rollout/fence journals, and
   workstation-side coordination. Nodes keep their state; the Realm Authority
   remains a separate consistency group and trust root.

Principal, Credential, Session, Access Grant, Delegation, Channel Grant,
Conversation ID, Hosted Service ID, Node Principal, Waku Peer ID, TLS identity,
SSH host identity, and discovery record signer remain distinct.

## Cross-packet compatibility results

| Concern | Reconciled choice | Conflict disposition |
|---|---|---|
| Application/Operator authority | Applications send/use leases/resolve; Operators own realm, conversation membership, profiles, hosts and fencing | preserves ADR-0001 |
| Realm and channels | one non-federated authority; independent discovery/data/application/control generations | DR-01 and DR-04 consume DR-03 |
| Membership removal | fresh generation plus survivor activation or explicit deployment fencing | DR-04 supplies evidence; DR-03 decides |
| Restore | each Node and Hosted Service projection restores its full group; authority restore also matches independent monotonic head | stale/partial restore fails closed |
| Endpoint truth | Hosting derives publication; Discovery discloses eligible locator; Direct Service does not authenticate the process | no signature/TLS/authorization conflation |
| Large data | Messaging carries immutable Content References, never large inline bytes or new fetch authority | preserves ADR-0004 |
| Retry | Messaging owns bounded post-accept delivery retry; direct-service retry remains application-protocol-owned | no generic retry layer |
| Topology | exactly three hosts only for the first claimed multi-host release; current stabilization qualification keeps its existing environment contract | no implicit new deployment claim |
| Federation | unsupported for v1; any future federation/MLS work is new R2 scope | no hidden multi-authority merge |

No remaining question changes an external interface, trust root, persisted state
owner, wire boundary, migration, or support topology.

## ADR dependency order

The required order is a dependency graph, not an arbitrary serial queue:

```text
ADR-0011 Channel Grant authority
  +-> ADR-0013 bounded multi-host reachability
  +-> ADR-0015 authority-backed Application conversations

ADR-0012 leased Application Hosting
  +-> ADR-0014 end Direct Service interaction at Discovery

ADR-0001..0010 remain accepted fixed inputs.
```

ADR-0011 and ADR-0012 may be reviewed in parallel. ADR-0013 and ADR-0015 may be
accepted only after reviewers accept their ADR-0011 dependency. ADR-0014 may
be accepted only after ADR-0012 and the existing Application Discovery
boundary remain compatible. ADR acceptance authorizes a design, not
implementation or qualification.

## Release-scope decision

### Independent stabilization release

The immediate DR-06 candidate contains only these fifteen existing capability
claims from the canonical catalogue:

1. `node.lifecycle`
2. `operator.command-interface`
3. `identity.principal-access`
4. `application.installation-content`
5. `network.waku-foundation`
6. `discovery.operator-resolution`
7. `content.operator-lifecycle`
8. `transfer.replication`
9. `workload.lifecycle`
10. `hosting.operator-publication`
11. `operations.diagnostics`
12. `operations.configuration-reload`
13. `operations.backup-upgrade-rollback`
14. `operations.native-installation`
15. `release.artifacts-provenance`

The catalogue remains the authority for each claim's exact I/R/O status and
required evidence. This synthesis does not change any `Q` value.

Explicitly excluded from this release are:

- `application.discovery`;
- `application.messaging`;
- `application.hosting`;
- `service.direct-interaction`;
- `realm.channel-grant-authority`;
- `deployment.multi-host`;
- Kubernetes, QUIC, WebTransport, WebRTC, remote Application transport, and
  non-Go SDKs.

### Post-stabilization capability releases

The first expansion may implement Application Discovery independently because
its AD-01 through AD-04 stream does not require the Wave 3 authority or Hosting
designs. Direct-service scope remains discovery-only even then.

The authority/multi-host/messaging expansion requires ADR-0011 first and must
not claim production private-realm messaging until the authority, fencing,
checkpoint, restore, and three-host evidence align on one commit.

Application Hosting may proceed after ADR-0012 independently of Messaging.
Its network-published qualification consumes the supported DR-04 topology;
local Hosting implementation does not. Application-owned HTTPS Hosting remains
future R2 scope.

## Dependency-ordered implementation backlog

The following are proposals, not newly published tracker issues. Each item is a
vertical slice whose detailed acceptance criteria live in its source packet.

### Tranche 0 — decisions and independent release

1. Review/accept ADR-0011 and ADR-0012 in parallel.
2. Review ADR-0013 and ADR-0015 after ADR-0011; review ADR-0014 after ADR-0012.
3. Run the independent stabilization DR-06 matrix below without waiting for
   any Wave 3 implementation.

### Tranche 1 — independent Application locator

1. AD-01 deepens protected Application admission without changing Content
   behavior.
2. AD-02 resolves one trusted service end to end.
3. AD-03 proves privacy, unsafe-target filtering, bounds and no side effects.
4. AD-04 proves exact grants, one-hop Delegation and Actor/Effective
   attribution.
5. AD-05 proves lifecycle convergence and matching-commit qualification.

### Tranche 2A — production authority foundation

1. CGA-01 creates and inspects one production Realm Authority.
2. CGA-02 delivers and acknowledges one recipient-bound initial generation.
3. CGA-03 rotates a channel and attests activation across hosts.
4. CGA-04 adds/removes membership with revocation and fencing.
5. CGA-05 renews grants and preserves strict channel-class separation.
6. CGA-06 restores/migrates against the independent anti-rollback root.
7. CGA-07 qualifies the authority lifecycle.

### Tranche 2B — local Hosted Service foundation

1. AH-01 persists an owner-qualified Hosted Service aggregate and immutable
   profile revision.
2. AH-02 drives one managed workload to readiness from archived profile truth.
3. AH-03 publishes and withdraws through the same leased lifecycle.
4. AH-04 closes Operator recovery, revocation and migration.
5. AH-05 qualifies the supported Hosting journey after topology evidence.

Tranches 2A and 2B may run in parallel after their ADRs.

### Tranche 3A — bounded multi-host operations

1. MR-01 compiles the exact three-host manifest.
2. MR-02 inspects Nodes through pinned workstation-side `ardentsctl --ssh`.
3. MR-03 places/recovers authority and checkpoint truth.
4. MR-04 fences/rejoins one Node monotonically.
5. MR-05 forms/recovers the private-LAN topology.
6. MR-06 admits verified public-direct endpoints.
7. MR-07 journals cross-host rollout/recovery.
8. MR-08 qualifies both topology variants.

MR-03 onward consumes the implemented CGA contracts. Public-direct work follows
private-LAN recovery rather than running as an unrelated demo.

### Tranche 3B — bounded Application conversations

1. AM-01 freezes Messaging contracts and authority bindings.
2. AM-02 creates/rotates conversations through the production authority.
3. AM-03 durably sends and projects delivery.
4. AM-04 admits/receives/acknowledges with durable cursors.
5. AM-05 recovers/migrates against authority/topology truth.
6. AM-06 qualifies bounded conversations.

AM-02 consumes CGA activation; AM-05 consumes DR-04 restore/fencing behavior.

### Tranche 3C — discovery-only service handoff

1. DSI-01 freezes the discovery-only documentation/contract.
2. DSI-02 proves a bounded locator-to-Application HTTP/TCP tracer after
   AD-01–AD-04.
3. DSI-03 proves existing Operator-published HTTPS locator semantics after the
   supported topology exists.
4. DSI-04 qualifies the exact handoff, not a Direct Service adapter.

Hosted-Service lifecycle scenarios consume AH-03; DR-05 adds no Node data-plane
implementation.

## Finite DR-06 input for the stabilization release

DR-06 evaluates exactly the fifteen listed capabilities and these environments:

| Gate | Required environment | Scope |
|---|---|---|
| `static` | clean Ubuntu runner | format, vet, architecture, docs, catalogue and traceability |
| `fast` | clean Windows runner | canonical fast/unit suite |
| `tagged` | Linux container | retained tagged integration scenarios |
| `application-process` | Linux container | existing Application Identity/Content process journey |
| `network-foundation` | local canonical network fixture | current Waku foundation contract |
| `workload-integration` | local canonical workload fixture | existing workload/publication lifecycle |
| `security` | clean Ubuntu hostile-input runner | current selected capability security matrix |
| `deployment` | clean Linux container | Compose install/upgrade/rollback/recovery |
| `native-install` | clean Linux systemd host | native lifecycle candidate |
| `multinode` | canonical Linux multinode QA environment | existing network/transfer claims, not DR-04 real-host support |
| `release-builds` | immutable release matrix | source/toolchain/base provenance and artifact verification |
| `release-candidate` | canonical Linux release environment | one no-hidden-retry aggregate decision |

Evidence must bind one clean source commit, toolchain/base identities, exact
commands, start/end times, outcomes, retained artifact hashes, and any
unavailable environment. A retry invalidates the run unless both attempts are
retained and the flake is resolved before acceptance. DR-06 may promote only a
capability whose complete declared gates pass on that exact commit.

The following are not substitutes for the matrix: this research worktree,
local Windows tooling tests, old stabilization evidence, Docker multinode QA
for real three-host qualification, or existence of a test without retained
matching-commit results.

## Deferred and rejected directions

- Rejected for v1: topic-centric Messaging, recipient-mailbox group fan-out,
  exactly-once/read-receipt claims, Direct Service adapter/proxy/credential
  translation, arbitrary endpoint registration, a long-running cluster
  controller, automatic NAT traversal, TOFU TLS, federation, and MLS.
- Deferred to a new R2 decision: Application-owned HTTPS Hosting, uniform
  authenticated service protocol, federation/MLS, more than three supported
  multi-host Nodes, additional transports, Kubernetes, remote Application
  transport, and non-Go SDKs.

## Exit state

Wave 3 is ready for maintainer ADR review and implementation planning. It is
not evidence that any new capability is implemented, reachable, operable, or
qualified. The independent stabilization scope is ready to enter DR-06; every
Wave 3 feature remains on a post-stabilization implementation/qualification
track until its own gates are satisfied.
