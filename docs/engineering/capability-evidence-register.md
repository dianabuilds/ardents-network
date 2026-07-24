# Capability and evidence register

## Scope

This register describes the product truth at the Wave 1 research baseline
`main@7c0965c` plus the uncommitted Wave 0/1 research changes.

It separates four maturity dimensions:

- `I` — implemented in production code;
- `R` — reachable through a supported caller interface;
- `O` — operable through status, diagnostics and recovery behavior;
- `Q` — qualified by the complete required evidence matrix for one exact
  clean commit.

`Q` is intentionally false for every capability until current-head Docker,
native, multi-node, security and release evidence is complete.

## Summary

| Vertical | I | R | O | Q | Research class |
|---|:---:|:---:|:---:|:---:|---|
| Node lifecycle and Operator control | yes | yes | yes | no | R3 |
| Principal identity and access | yes | yes | yes | no | R3 |
| Application identity and Content | yes | partial | yes | no | R1/R3 |
| Network foundation | yes | Operator | yes | no | R2/R3 |
| Discovery | yes | Operator only | yes | no | R1 |
| Content, transfer and replication | yes | mixed | yes | no | R1/R3 |
| Workloads and hosted services | yes | Operator only | yes | no | R2/R3 |
| Operations, deployment and release | yes | yes | yes | no | R3 |

## 1. Node lifecycle and Operator control

### User outcome

An enrolled Operator can start, stop and inspect one Node, stream events and
use CLI/TUI locally or through SSH stream-local forwarding.

### Current interface

- `NodeService`: start, stop, status, features, runtime and events;
- `ardentsctl node`, `shell`, and `tui`;
- protected Operator Unix socket;
- SSH forwarding to the same Unix socket.

### Implementation

- composition and lifecycle: `internal/daemon`;
- Operator handlers: `internal/localapi/node`;
- CLI: `internal/cli/node`, `internal/cli/tui`;
- identity-bound transport: `internal/cli/client`.

### Evidence

- default daemon, local API, CLI and session tests pass;
- critical daemon lifecycle tests pass locally with `-race`;
- historical integration/E2E reports cover restart, pending truth and terminal
  operation flows.

### Gaps

- current clean Linux/Docker E2E is missing;
- native systemd/SSH qualification is not current-head evidence;
- CLI documentation is not yet a machine-readable procedure/action/smoke map.

### Disposition

Existing feature, no new product design required. Complete R3 qualification and
an R1 Operator command smoke catalogue.

## 2. Principal identity and access

### User outcome

A Principal creates root/device custody, enrolls on a Node, authenticates a
short-lived session and receives finite grants or one-hop Delegation.

### Current interface

- Operator `IdentityService`;
- Application `IdentityService`;
- offline `ardentsctl identity principal` and `ardentsctl identity device`
  commands;
- online enrollment, grant, revocation, ticket and session commands;
- Go SDK session, enrollment, identity and delegation adapters.

### Implementation

- identity and capability model: `internal/identity`;
- sessions/grants/recovery: `internal/identity/access`;
- shared client state machine: `internal/identity/sessionclient`;
- Operator/Application interceptors and bindings.

### Evidence

- default identity, access, SDK and CLI tests pass;
- identity access passes locally with `-race`;
- recovery Credential, one-time ticket and session parity remediations have
  deterministic tests;
- Application process E2E exists in the tagged suite.

### Gaps

- full installation journey has not run against the Wave 1 commit;
- production Channel Grant authority is a separate unresolved research area;
- external/interoperability crypto vectors remain limited to repository-owned
  implementations.

### Disposition

Core identity is an R3 qualification target. Application installation is an R1
journey investigation. Channel authority remains R2.

## 3. Application identity and Content

### User outcome

An Application enrolls as its own Principal, authenticates to the dedicated
Application socket and puts/gets immutable Principal-owned content.

### Current interface

- Application `IdentityService`;
- Application `ContentService.Put/Get`;
- Go SDK `client`, `identity`, `content`, and typed errors.

No Application discovery, messaging or hosting interface exists.

### Implementation

- admission and binding: `internal/applicationapi`;
- owner-aware content adapter: `internal/applicationapi/content`;
- public client adapters: `sdk/go`.

### Evidence

- Application admission/content/SDK tests pass;
- dedicated Application process E2E and content fetch E2E exist;
- owner-qualified Object/Manifest and owner-content binding tests pass.

### Gaps

- the complete ticket -> enrollment -> session -> remote `Put/Get` journey
  needs one current-head acceptance scenario;
- unary payload limits and large-content ergonomics need a user-facing contract;
- SDK extraction/version/module-path decision is deferred.

### Disposition

Current slice is implemented but only partially qualified. The installation
journey is R1; release evidence is R3.

## 4. Network foundation

### User outcome

A Node joins a real Waku network, participates according to its profile and
publishes/receives private product traffic without exposing readable meaning.

### Current interface

- Operator Network status, peers, routes and presence;
- internal network contracts used by discovery, publication and transfer;
- no public Application network interface.

### Implementation

- product contracts: `internal/network`;
- Waku/libp2p adapter: `internal/network/waku`;
- Relay, Store, Filter and Lightpush roles;
- TCP/WSS supported profiles and explicit suppressed transports.

### Evidence

- Waku adapter tests pass locally with `-race`;
- persistent Store retention and quota remediations pass;
- historical network integration/E2E and multi-node reports exist.

### Gaps

- no current Docker/Linux multi-node evidence;
- real multi-host churn, partition, hostile peer, WSS and reachability matrix is
  not release-qualified;
- NAT and production topology policy need DR-04.

### Disposition

Implementation is substantial. Runtime qualification is R3; supported
multi-host topology remains R2.

## 5. Discovery

### User outcome

A Node publishes signed Node/service facts, evaluates remote trust and resolves
usable records, routes and service endpoints.

### Current interface

- Operator status, presence, records, resolve and route commands;
- internal publication/discovery interfaces;
- no Application discovery protocol or SDK package.

### Implementation

- records, merge, trust and resolution: `internal/discovery`;
- network delivery: `internal/discovery/private_network.go`;
- publication: `internal/publication`;
- Operator projection: `internal/localapi/network`.

### Evidence

- default discovery, publication and trust tests pass;
- discovery writer shutdown regression passes through daemon lifecycle evidence;
- tagged integration/E2E discovery scenarios exist.

### Gaps

- Applications cannot resolve a service without administrative authority;
- the Application admission interceptor and sealed-call channel must first be
  generalized from owner-required Content resources to a closed registry that
  also supports registered ownerless resources;
- current multi-node publish/resolve evidence is pending.

### Disposition

The Application Discovery R1 packet is complete:
`research/application-discovery.md`. It selects a bounded trusted
`NetworkPublished` service locator, exact
`application.discovery.resolve` / `service-type` authority, standard
Delegation intersection and uniform privacy-safe errors. AD-01 through AD-04
are ready for implementation in dependency order; AD-05 remains the
qualification gate.

## 6. Content, transfer and replication

### User outcome

An Operator manages Objects, Blobs and Manifests; an Application owns immutable
content; Nodes fetch and retain encrypted content and maintain replica
commitments.

### Current interface

- Operator Content, Retention and Transfer services;
- CLI data inventory, objects, blobs, manifests, sources and transfers;
- Application Content `Put/Get`;
- internal replication/repair interfaces.

### Implementation

- content/catalog/payload: `internal/content`;
- transfer protocol: `internal/transfer`;
- placement and commitments: `internal/replication`;
- private data envelopes: `internal/messaging`.

### Evidence

- content, messaging and transfer pass locally with `-race`;
- owner-qualified identity/migration and multi-provider fetch regressions pass;
- tagged content, transfer and replication integration/E2E scenarios exist.

### Gaps

- interruption/resume, partition and scale evidence is incomplete;
- Application exposes content, not replication policy or transfer diagnostics;
- large-object and streaming semantics are explicitly bounded/deferred;
- quota and repair UX require product-level validation.

### Disposition

Existing implementation is an R3 qualification target. User-journey and
large-content contract review is R1; scale/partition work is R2/R3.

## 7. Workloads and hosted services

### User outcome

An Operator registers and controls a workload, observes readiness and publishes
or withdraws an HTTP/HTTPS/TCP hosted-service endpoint backed by the running
generation.

### Current interface

- Operator Workload service and CLI;
- workload list/get/register/start/stop/restart;
- hosted service and publication status;
- no Application hosting interface.

### Implementation

- execution/registry/readiness: `internal/workload`;
- Docker adapter: `internal/workload/docker`;
- hosted-service truth: `internal/hosting`;
- publication: `internal/publication`;
- bounded ingress: `internal/ingressproxy`.

### Evidence

- workload and ingress packages pass locally with `-race`;
- reset, idle/fairness and hung-Docker regressions pass;
- historical workload/hosting integration and E2E reports cover publication
  withdrawal and restart.

### Gaps

- no current Docker daemon is available for Wave 1 qualification;
- Application hosting ownership, lease, readiness and drain are undefined;
- remote service authentication and client journey are undefined;
- soak, slow-client and production Docker failure evidence is incomplete.

### Disposition

Existing Operator/runtime path is R3. Application Hosting and direct service
interaction require separate R2 research.

## 8. Operations, deployment and release

### User outcome

An Operator diagnoses health, reloads configuration and performs supported
backup, restore, upgrade and rollback with attributable release artifacts.

### Current interface

- Diagnostics, Configuration and Node runtime services;
- CLI human/JSON/shell/TUI views;
- loopback observability;
- Compose lifecycle and native Linux installer;
- release build/verify scripts.

### Implementation

- `internal/diagnostics`, `internal/config`, `internal/observability`;
- `scripts/deploy`, `scripts/install`, `scripts/release`;
- `.github/workflows/ci.yml`.

### Evidence

- default tests, static policy gates and critical race packages pass locally;
- deployment remediation artifacts exist for transactional rollout;
- current-head Docker, native, multi-host and independent release builds have
  not run in this environment.

### Gaps

- five deployment/native audit rows remain `remediated_candidate`;
- formatting requires fresh-checkout validation of the new LF contract;
- current commit-bound vulnerability and release evidence is absent;
- tagged catalog previously returned an empty false pass and is corrected in
  Wave 1.

### Disposition

This is an R3 qualification program, not one implementation issue.

## Cross-vertical findings

### Reachability gap

The Operator surface reaches almost every implemented domain. The Application
surface reaches only identity and immutable content. Most apparent missing
product features are therefore interface/user-journey gaps, not absent domain
implementations.

### Qualification gap

All existing verticals have meaningful unit and historical tagged evidence.
None has the complete current-head clean release matrix. Documentation must not
collapse `implemented` into `qualified`.

### Deep-module opportunities

- Application Discovery can hide trust, route filtering and record projection
  behind a small read-only interface.
- Application Messaging must hide Waku, selectors, encryption, replay, Store
  queries and retry.
- Application Hosting should hide workload/hosting/publication orchestration
  behind one lifecycle interface.

## Wave 1 ready work

### Completed during research

- W1-001: static catalog now includes `integration,e2e` tags and rejects an
  empty result.
- W0-001 candidate: Go LF checkout policy added through `.gitattributes`.

### R0

1. Validate W0-001 in a fresh Windows checkout after commit.
2. Run the corrected static job and retain its 142-entry catalogue.
3. Produce a clean commit-bound gate snapshot.

### R1

1. Completed: Application Discovery research packet and implementation slices.
2. Application installation/content journey acceptance design.
3. Operator procedure/action/smoke catalogue.
4. Machine-readable capability/evidence catalogue derived from this register.

### R2/R3

Continue according to `global-feature-research-plan.md`; do not turn these
directions into implementation issues until their research/qualification exit
criteria are met.
