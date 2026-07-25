# DR-02: Application Hosting

## Metadata

- Status: accepted research recommendation; ADR-0012 remains Proposed
- Research class: R2 deep research
- Decision owner: Application Interface / Workload and Hosting / Security
- Research owner: Wave 3 DR-02
- Date: 2026-07-25
- Frozen baseline commit:
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`
- Parent program: `.scratch/wave3-deep-research/PRD.md`
- Blocking research: none; the accepted Application Discovery locator boundary
  is an input, but its implementation remains separate
- Downstream consumers: DR-05 Direct Service Interaction, Application Hosting
  implementation, DR-06 qualification

## Answer first

Add one Application-facing Hosted Service lifecycle, not Application access to
the existing workload, hosting, publication, or ingress controls. An
Application Principal acquires a finite lease on one Operator-approved hosting
profile. The Node persists the owner-qualified Hosted Service as authoritative
intent and internally derives one managed workload, readiness observation,
ingress binding, and optional discovery publication.

For v1, reject registration of an already-running process or arbitrary endpoint.
It has no Node-owned crash/restart truth, cannot prove generation-bound
readiness, and would make withdrawal race an external process. The selected
model costs a new persisted aggregate, an Application wire service, and an
Operator profile/revocation surface, but preserves one authority and recovery
boundary.

Write Proposed ADR-0012 before implementation. DR-05 remains responsible for
client connection, Principal authentication to the hosted service, service
authorization, TLS identity, pinning, and application-protocol errors.

## User outcome

An authorized local Application can start an Operator-approved service, keep it
alive by renewing a bounded lease, observe one coherent status, and drain it.
The Node publishes it only while the lease, current workload generation,
readiness, ingress, network reachability, and policy all permit publication.

## Scope

### In scope

- ownership among the Application Principal, Hosted Service, internal workload,
  and Node-signed publication;
- one leased, managed-workload lifecycle per Hosted Service;
- Operator-defined immutable hosting profiles;
- exactly one canonical service type pinned by each immutable profile revision;
- acquire/ensure, renew, get/list, recover, and drain semantics;
- local-only and network-published exposure;
- `http` and `tcp` v1 service protocols;
- readiness, bounded restart, ingress admission, publication, withdrawal,
  revocation, restart recovery, quotas, audit, and migration;
- additive local Application wire and Go SDK contracts.

### Out of scope

- arbitrary endpoint or existing-process registration;
- caller-supplied image, command, environment, port, probe, policy reference,
  runtime, certificate, secret, mount, device, or Docker option;
- exposing independent workload, hosting, publication, ingress, Docker, or
  discovery APIs;
- Application-selected `https` until DR-05 defines TLS identity and secret
  delivery;
- returning or opening a service endpoint;
- client authentication, service authorization, retries, or application
  protocol semantics (DR-05);
- remote Application transport, non-Go SDKs, Kubernetes, schedulers, QUIC,
  WebTransport, and WebRTC;
- release qualification or promotion of capability `Q`.

## Current product truth

### Supported interfaces

| Boundary | Supported surface at the frozen baseline | Disposition |
|---|---|---|
| Operator | `WorkloadService` on the protected Operator Interface: register/start/stop/restart/status/list plus hosted-service and publication status | Supported and reachable |
| Application | protected local `IdentityService` and `ContentService`; Go `client.Client` exposes Content and Session | No Hosting surface |
| Internal | `workload.Runtime`, `execution.Service`, readiness registry/controller, `hosting.Service`, `publication.Manager`, Docker/trusted-process executors, ingress proxy | Implemented internal collaborators |
| Deployment | Node configuration supplies workload/service specs, executor, policy, registries, runtimes, ingress image/bind/hosts | Operator/deployment-owned |
| True external | Docker Engine, selected OCI runtime, workload image/process, client of the published service | Not Application Interface dependencies |

ADR-0001 prohibits wrapping the Operator interface for Applications. The
Application Discovery packet selects a bounded read-only locator and explicitly
leaves connection and authentication to DR-05. It does not implement Hosting.

### Reachable journey

The current supported caller-to-domain path is:

```text
Operator Principal
  -> protected Operator listener
  -> identity/access admission and exact workload action/resource
  -> localapi/workload handler
  -> workload.Runtime mutation
  -> persisted execution.Service desired/observed state
  -> executor prepare/start/inspect/stop
  -> generation-bound readiness.Controller
  -> publication.Manager policy + network gate
  -> local signed discovery record / network publication
```

`RegisterWorkload` accepts the complete administrative `WorkloadSpecSnapshot`,
including config, policy reference, restart policy, service endpoints, probes,
and desired state. The Application journey ends after session admission and
Content `Put/Get`; there is no Application protocol, SDK service, action
catalogue entry, owner-qualified Hosting resource, lease, or Application
recovery path.

### Implementation and evidence

| Claim | Source or contract | Evidence | Baseline disposition |
|---|---|---|---|
| Operator can register and control workloads and inspect hosting/publication | `api/ardents/v1/workload.proto`; `internal/localapi/workload`; `internal/daemon/assembly.go` | local API authorization/mapping tests and workload control-surface integration tests | I=yes, R=yes, O=yes, Q=no |
| Workload desired/observed state and generations persist | `internal/workload/execution/reconciler*.go`; storage key `workload/snapshot` | persistence/recovery unit tests and `tests/integration/workload/runtime_recovery_test.go` | implemented, reachable to Operator, not release-qualified |
| Workload Control, not Docker, owns restart truth | `internal/workload/execution/reconciler_run.go`; `docs/operations/workload-execution-platform.md` | restart-budget, crash, orphan, and Docker integration tests | implemented, operable, Q=no |
| Publication is derived from current generation readiness, network reachability, endpoint validation, and policy | `internal/workload/readiness`; `internal/publication/plan.go`; `internal/policy/evaluation_publication.go` | readiness/publication unit and integration tests | implemented, Operator-reachable, Q=no |
| Workload mutation and publication attempt rollback toward one prior snapshot | `internal/workload/runtime.go`; `internal/publication/rollback.go` | mutation/compensation tests referenced by remediation OPS-002/OPS-003 | remediated candidate/local evidence, not qualified |
| Crash or failed observation withdraws effective publication; restart can republish only after readiness | `internal/workload/execution`; `internal/publication`; `tests/e2e/workload/lifecycle_test.go` | tagged workload E2E and hosting readiness tests | implemented, environment-specific evidence, Q=no |
| Docker ingress is a bounded generation-owned proxy, not workload-owned host-port authority | `internal/workload/docker/docker_ingress.go`; `internal/ingressproxy`; workload security policy | Docker security/ingress integration tests | implemented for Docker path, Q=no |
| Application Hosting exists | Application protocol, `sdk/go/client`, capability catalogue | catalogue says I=no, R=no, O=no, Q=no at frozen commit | absent |
| Application Discovery locator is an accepted boundary but not implementation truth | `docs/engineering/research/application-discovery.md`; capability catalogue | no Application Discovery protocol or SDK at frozen commit | design input only |

The current tagged and local tests do not qualify Application Hosting because
the public Application lifecycle and its authority/persistence model do not
exist. Historical audit and remediation rows are hypothesis/evidence pointers,
not current release evidence.

### External runtime facts

Only official primary documentation is used:

| Source | Version/scope | Fact used | Accessed |
|---|---|---|---|
| [Docker restart policy](https://docs.docker.com/engine/containers/start-containers-automatically/) | current Docker Engine documentation; unversioned page | Docker can own automatic container restart, including daemon-restart behavior, so Ardents must deliberately keep Engine restart disabled to avoid two lifecycle owners | 2026-07-25 |
| [Docker resource constraints](https://docs.docker.com/engine/containers/resource_constraints/) | current Docker Engine documentation; unversioned page | containers have no default CPU/memory bounds and enforcement depends on host capabilities | 2026-07-25 |
| [Docker rootless resource limits](https://docs.docker.com/engine/security/rootless/tips/#limiting-resources) | current Docker Engine documentation; notes behavior since Engine 23.0 | rootless cgroup limits require cgroup v2/systemd and otherwise may be ignored | 2026-07-25 |
| [gVisor Docker Quick Start](https://gvisor.dev/docs/user_guide/quick_start/docker/) | current gVisor guide; Docker >=17.09 prerequisite | `runsc` is installed as a Docker runtime and explicitly selected per container | 2026-07-25 |

These sources corroborate adapter constraints only. Ardents code and normative
repository contracts remain the source for the selected product semantics.

## Actors, assets, and trust boundaries

| Actor | Identity | Authority | Protected assets | Trust boundary |
|---|---|---|---|---|
| Application | Actor Principal authenticated by finite Application Credential and Node/interface-bound Session | exact Hosting Access Grants; no Operator authority | its Hosted Service declarations, lease, status | protected local Application Interface |
| Node Hosting Controller | local Node subsystem; not a Principal | enforces grants, profile policy, leases, state machine and recovery | authoritative Hosting store and mutation journal | in-process trusted control plane |
| Operator | Operator Principal for this Node | defines profiles/quotas; inspects, suspends, revokes, force-drains | Docker authority, profiles, policy, capacity, diagnostics | protected Operator Interface |
| Workload | no Principal identity by default | none; receives only closed profile material | bounded runtime allocation | hostile execution boundary |
| Ingress proxy | generation-labelled Node runtime component, not a Principal | fixed target/ports and bounded forwarding only | listener capacity | Docker/network boundary |
| Docker Engine / OCI runtime | transport/runtime identity, not Ardents Principal | local execution authority delegated by Node configuration | containers, networks, resource enforcement | true external privileged control plane |
| Publishing Node | Node Principal signs discovery facts | discovery-publish trust purpose and network carrier permission | service record and withdrawal | Node-to-network boundary |
| Service client | identity and protocol deferred to DR-05 | no Hosting authority | endpoint connection and application data | outside this decision |

Application Principal, workload identity, Node Principal, Waku Peer ID, Docker
container labels, ingress transport identity, Credential, Access Grant,
Delegation, and any future service TLS identity remain distinct.

## Invariants

- Only an authenticated Application call admitted for the exact action and
  resource enters Hosting; no request is persisted or executed first.
- A Hosted Service is owned by the Actor Application Principal. Hosting
  mutations reject Delegation in v1; `Actor == Effective` is required.
- A workload has no Principal. Its internal owner metadata points to the Hosted
  Service aggregate and never grants authority.
- The Node Principal signs publication; the published record does not become
  proof that the workload or Application owns the Node key.
- The persisted Hosted Service aggregate is authoritative desired truth.
  Workload, readiness, ingress, and discovery state are derived/recoverable
  projections.
- An active lease permits reconciliation; it never implies ready or published.
- Network publication requires an active lease, current workload generation,
  composite service readiness, admitted ingress, network reachability, profile
  policy, and successful signed publication.
- Every signed service record expires no later than the Hosted Service lease.
  Renewal refreshes publication only after the renewed aggregate is durable.
- Loss of any publication prerequisite withdraws or replaces the local
  publication truth before a later republish.
- Normal drain closes admission for new ingress and withdraws discovery before
  stopping the workload. Revocation/expiry use immediate withdrawal and
  connection termination.
- No Application field selects image, runtime, resource ceiling, environment,
  host, port, probe, service type, certificate, policy reference, or discovery
  endpoint.
- IDs, requests, responses, profiles, services, restart attempts, mutation
  retries, leases, connections, and diagnostics all have explicit bounds.
- Application Get/List reads the maintained aggregate projection and never
  starts a synchronous Docker observation, readiness probe, network fetch, or
  publication attempt.
- Unknown enum/field/resource/profile, malformed ID, unsupported protocol,
  invalid transition, stale mutation revision, and persistence corruption fail
  closed.
- Status and errors never disclose another Principal, Docker/container
  identity, raw config, endpoint, trust/policy detail, or topology.

## Dependency classification

| Dependency | Classification | Owner | Failure ownership | Substitutable locally? |
|---|---|---|---|---|
| Application admission/procedure registry | in-process | Application Interface / Identity | rejects before Hosting | yes |
| Hosting aggregate store and journal | in-process | Hosting | Node fails Hosting readiness; corrupt truth blocks reconciliation | in-memory test store only |
| Operator hosting-profile catalogue and immutable revision archive | in-process | Policy / Hosting | unknown/disabled current profile denies Ensure; missing/corrupt pinned revision blocks all execution/publication | fixture yes |
| Workload runtime and execution journal | in-process | Workload Control | Hosting reports degraded/failed and reconciles from aggregate | fake executor yes |
| Readiness controller | in-process | Workload/Hosting | never publish without current ready observation | fake prober yes |
| Publication/discovery store | in-process | Publication / Discovery | local withdrawal remains authoritative; network convergence is degraded | fake carrier/store yes |
| Application Connect handler and Go SDK | local-substitutable | Application Interface | typed local transport failure | Unix/in-memory contract fixture |
| Ingress proxy | local-substitutable runtime component | Workload Docker adapter | generation not exposure-eligible; recover or fail | fake executor for unit, real image for acceptance |
| Docker Engine | true-external | deployment Operator | workload cannot start/inspect/stop; Node owns diagnosis and retry | fake executor locally; real Linux Docker required |
| `runsc`/`runc` and host cgroups/LSM | true-external | deployment Operator | preflight/admission fails closed | no for security acceptance |
| Published-service clients | true-external | DR-05/service owner | not a Hosting call dependency | not applicable |
| Waku publication carrier / remote Nodes | remote-owned Ardents dependency | Network / remote Node operators | withdrawal/refresh convergence reported; local truth remains authoritative | test carrier yes |

## Alternative designs

### Alternative A: register an existing endpoint

- External interface: `RegisterEndpoint(service type, endpoint, readiness URL,
  exposure, ttl)` plus renew/unregister.
- Internal seam: a registration catalogue feeding readiness and publication;
  no Node-owned workload.
- State ownership: Application/external supervisor owns process and restart;
  Node owns only registration lease and publication.
- Authority model: Application can introduce an address into Node discovery,
  requiring endpoint-scope and SSRF/topology controls.
- Failure and recovery: Node can probe and withdraw, but cannot distinguish
  process crash, replacement, or endpoint reuse and cannot restore the process.
- Compatibility and migration: less new workload integration, but creates a
  permanent second hosting lifecycle beside managed workloads.
- Operational cost: two supervisors, two readiness truths, and external
  recovery runbooks.

### Alternative B: leased Operator-profile managed Hosted Service

- External interface: acquire/ensure, renew, get/list, recover, and drain one
  Hosted Service; no workload or endpoint fields.
- Internal seam: `Hosting.Controller` owns an aggregate and projects it through
  profile resolution to Workload Control, readiness, ingress, and publication.
- State ownership: persisted Hosted Service desired state/lease is
  authoritative; execution/publication are derived.
- Authority model: exact owner-qualified actions plus profile scope and a
  separate network-publication action.
- Failure and recovery: Node owns generation, restart budget, withdrawal, and
  restart reconciliation.
- Compatibility and migration: additive Application wire/store; current
  Operator workloads remain operator-owned and are not adopted.
- Operational cost: profile administration and a transactional recovery
  journal, but one lifecycle owner.

### Alternative C: expose Application workload and publication controls

- External interface: separate create workload, probe, publish, withdraw, and
  ingress methods.
- Internal seam: thin adapters over current packages.
- State ownership: caller coordinates several persisted/observed truths.
- Authority model: broad execution and topology actions.
- Failure and recovery: partial success leaks into the SDK and every caller
  implements compensation.
- Compatibility and migration: fastest initial code path, largest permanent
  public surface.
- Operational cost: high caller complexity and security review burden.

### Decision matrix

Scores are 1 (poor) to 5 (strong); weighted total is score multiplied by
weight.

| Criterion | Weight | A: endpoint registration | B: managed aggregate | C: exposed controls | Evidence or reasoning |
|---|---:|---:|---:|---:|---|
| Module depth | 5 | 2 | 5 | 1 | B hides four subsystems behind one lifecycle |
| Caller leverage | 4 | 2 | 5 | 1 | B supplies restart/readiness/withdrawal, not just storage |
| Change locality | 3 | 3 | 4 | 2 | B adds one controller and adapters; A adds a parallel lifecycle |
| Trust-model fit | 5 | 2 | 5 | 1 | B preserves Operator profiles and exact grants |
| Failure clarity | 5 | 2 | 5 | 1 | B has one persisted intent and explicit projections |
| Migration cost | 2 | 4 | 3 | 4 | A/C are initially cheaper but create long-term compatibility |
| Operability | 4 | 2 | 5 | 2 | only B owns crash/restart/drain recovery end to end |
| **Weighted total** | **28** | **65** | **132** | **43** | B selected |

## Selected design

### External interface sketch

The SDK owns domain types; it does not alias generated protobuf:

```go
package hosting

type Protocol string
const (
    ProtocolHTTP Protocol = "http"
    ProtocolTCP  Protocol = "tcp"
)

type Exposure string
const (
    LocalOnly        Exposure = "local_only"
    NetworkPublished Exposure = "network_published"
)

type Declaration struct {
    Name        string
    Profile     string
    Protocol    Protocol
    Exposure    Exposure
    Lease       time.Duration
    RequestID   string
}

type Status struct {
    ID             string
    Name           string
    Type           string
    Protocol       Protocol
    Exposure       Exposure
    State          string
    Ready          bool
    Published      bool
    LeaseExpiresAt time.Time
    Revision       uint64
    Reason         string
}

type Service interface {
    Ensure(context.Context, Declaration) (Status, error)
    Renew(context.Context, string, uint64, string) (Status, error)
    Get(context.Context, string) (Status, error)
    List(context.Context) ([]Status, error)
    Recover(context.Context, string, uint64, string) (Status, error)
    Drain(context.Context, string, uint64, string) (Status, error)
}
```

`Ensure` is create-or-read for the tuple `(Actor Principal, Name)`. Repeating
the same immutable declaration and `RequestID` returns the same result.
Changing profile, protocol, or exposure conflicts; v1 uses drain then a new
Hosted Service ID for replacement. `Renew`, `Recover`, and `Drain` require the
current revision and a bounded idempotency `RequestID`.

Mutations acknowledge durable intent; they do not wait for Docker start,
readiness, drain completion, or remote publication convergence. `Ensure` may
return PENDING/STARTING, `Recover` STARTING, and `Drain` WITHDRAWING. The caller
uses Get with bounded polling chosen by the SDK consumer. If transport timeout
loses a response after commit, retrying the same RequestID returns the durable
result without repeating the transition.

The Node, not the caller, assigns the opaque Hosted Service ID and the
advertised endpoint. The immutable profile revision pins exactly one canonical
service type; `Status.Type`/wire `service_type` projects that value so Discovery
and DR-05 can refer to it. `Declaration` has no service-type field and an
Application cannot select or override the type. Status does not contain workload
ID, image, port, probe, endpoint, Node Principal, container, runtime, trust, or
policy detail.

Wire operations mirror these six methods under a new Application
`HostingService`. Unknown protobuf fields are rejected before admission.
Requests are unary and use existing Application transport/body limits.

```proto
service HostingService {
  rpc Ensure(EnsureHostedServiceRequest) returns (HostedServiceStatus);
  rpc Renew(MutateHostedServiceRequest) returns (HostedServiceStatus);
  rpc Get(GetHostedServiceRequest) returns (HostedServiceStatus);
  rpc List(ListHostedServicesRequest) returns (ListHostedServicesResponse);
  rpc Recover(MutateHostedServiceRequest) returns (HostedServiceStatus);
  rpc Drain(MutateHostedServiceRequest) returns (HostedServiceStatus);
}

message EnsureHostedServiceRequest {
  string name = 1;
  string profile = 2;
  string protocol = 3;
  string exposure = 4;
  int64 lease_seconds = 5;
  string request_id = 6;
}

message MutateHostedServiceRequest {
  string service_id = 1;
  uint64 expected_revision = 2;
  string request_id = 3;
}

message GetHostedServiceRequest {
  string service_id = 1;
}

message ListHostedServicesRequest {}

message HostedServiceStatus {
  string service_id = 1;
  string name = 2;
  string service_type = 3;
  string protocol = 4;
  string exposure = 5;
  string state = 6;
  bool ready = 7;
  bool published = 8;
  google.protobuf.Timestamp lease_expires_at = 9;
  uint64 revision = 10;
  string reason = 11;
}

message ListHostedServicesResponse {
  repeated HostedServiceStatus services = 1;
}
```

The protocol file imports `google/protobuf/timestamp.proto`. Strings are used
for closed, validated protocol/exposure/state/reason domains so adding a value
is an explicit server/SDK compatibility decision rather than an unknown-enum
default. Empty values are invalid.

### Profile contract

An Operator profile has a logical profile ID whose current pointer names one
immutable, content-addressed revision. Each revision pins exactly one canonical
service type plus the closed workload template, allowed protocols/exposures,
resource limits, readiness policy, restart budget, drain deadline, and
publication/ingress policy. An Application supplies only the logical profile ID
and allowed protocol/exposure choices. Ensure resolves the active current
revision, validates its canonical type, durably archives that exact revision,
and then persists the aggregate reference `(profile_id, profile_digest)`.

Profile defaults and bounds:

- name/profile/request ID: canonical printable IDs, each at most 128 bytes;
- one Hosted Service maps to exactly one workload and one service;
- at most 256 logical profiles have active current revisions on one Node;
- at most 1,024 immutable current-or-archived profile revisions are retained on
  one Node; creating another revision is rejected until safe garbage collection
  reduces the count;
- lease request: default 15 minutes, minimum 1 minute, maximum 60 minutes;
- renewal is accepted only before expiry and resets expiry to
  `now + admitted duration`; it cannot resurrect an expired service;
- a network record's absolute expiry is capped by the durable lease expiry, so
  delayed withdrawal cannot make a remote record fresh beyond the lease;
- normal drain deadline: profile-selected up to 30 seconds;
- at most eight live Hosted Services per Application Principal and at most the
  existing Node workload/policy ceiling globally;
- at most 32 retained terminal records per Principal and 1,024 per Node;
  reaching either bound rejects new Ensure until retention compacts or an
  Operator resolves the pressure;
- per-Principal mutation admission uses a one-per-second token bucket with
  burst eight; the Node-wide bucket is 32 per second with burst 64;
- list returns at most eight owner-visible services, active first and then
  newest terminal records; Get remains available by owned ID;
- terminal records are retained for 24 hours for audit/idempotency, then
  compacted; opaque Hosted Service IDs are never reused;
- one outstanding mutation per service; stale revisions conflict;
- mutation request IDs are retained with the aggregate's latest result and are
  bounded to the latest eight entries.

`http` and `tcp` are the v1 Application-selectable protocols.
`NetworkPublished` additionally requires the exact network-publish action and
an Operator profile that admits it. `https` is rejected until DR-05 defines
TLS identity/rotation and a secret-delivery mechanism; this packet does not
smuggle certificates through workload config.

### Durable profile revisions

The Hosting store owns both the aggregate/journal and a durable immutable
profile-revision archive. A digest alone is not sufficient state: the archived
canonical revision bytes are required to reconstruct the resolved Plan after
restart or restore.

- Updating a logical profile writes a new canonical revision and digest, then
  atomically advances its current pointer. Existing aggregates keep their old
  `(profile_id, profile_digest)` and never silently change service type or Plan.
- Disabling a logical profile removes it from new Ensure admission. Existing
  pinned aggregates may Renew, Recover, reconcile, and drain against their
  archived revision until expiry or explicit Operator revocation.
- Operator "delete" is logical: it disables the profile and removes its current
  pointer. It does not physically remove any pinned revision.
- A revision is GC-eligible only when no current pointer, aggregate (including
  its 24-hour terminal tombstone), or in-flight journal references it. It is
  retained for an additional 24 hours after the last reference disappears.
  GC is deterministic oldest-unreferenced-first and never breaks a reference.
- The archive cap is fail-closed. It never evicts a referenced revision to make
  room. The Operator must let retention elapse or revoke/drain and compact old
  resources before creating more revisions.
- Every archive read recomputes the canonical digest. A missing revision,
  mismatch, unknown revision schema, or invalid canonical service type makes
  Hosting failed, withdraws local publication truth, and blocks execution,
  recovery, renewal, and republish until a matching backup is restored.

Profile disable is therefore an admission control, not revocation. Explicit
Hosted Service/profile-revision revocation is the only Operator action that
terminates already admitted aggregates.

### Internal seam and state machine

```go
type Controller interface {
    Ensure(context.Context, AdmittedDeclaration) (Snapshot, error)
    Renew(context.Context, OwnedMutation) (Snapshot, error)
    Recover(context.Context, OwnedMutation) (Snapshot, error)
    Drain(context.Context, OwnedMutation) (Snapshot, error)
    Get(OwnedID) (Snapshot, error)
    List(OwnerPrincipal) ([]Snapshot, error)
    Reconcile(context.Context) error
}

type ProfileResolver interface {
    Resolve(profileID, profileDigest string, protocol Protocol, exposure Exposure) (Plan, error)
}

type RuntimePort interface {
    Apply(context.Context, DerivedWorkload) (RuntimeSnapshot, error)
    Drain(context.Context, DerivedWorkloadID, time.Duration) error
    Remove(context.Context, DerivedWorkloadID) error
}
```

The controller owns the aggregate, journal, lease clock checks, profile
resolution, public status projection, and orchestration. Workload Control owns
execution and generation observations. Publication owns signed discovery truth.
Ingress owns connection admission/drain. None becomes a public Application
subservice.

```text
ABSENT
  -- Ensure/persist --> PENDING --> STARTING
                                  |      |
                                  |      +-- ready + local ----------> READY_LOCAL
                                  |      +-- ready + publish success -> PUBLISHED
                                  |      +-- bounded failure --------> DEGRADED
                                  |      +-- budget exhausted -------> FAILED
                                  |
READY_LOCAL/PUBLISHED/DEGRADED
  -- Renew ----------> same lifecycle state, later absolute expiry
  -- observed crash -> WITHDRAWING -> RESTARTING -> ready/published
  -- Recover failed -> STARTING with a new bounded generation budget
  -- Drain ----------> WITHDRAWING -> DRAINING -> RELEASED
  -- expiry/revoke --> WITHDRAWING -> TERMINATING -> EXPIRED/REVOKED

Any state -- corrupt truth/pinned revision missing or invalid/
clock rollback beyond 30 s --> WITHDRAWING and operator-required FAILED;
never publish
```

Each mutation first persists aggregate intent and a journal phase, then applies
external effects. Completion persists observed projection and clears the
journal. On interruption, replay is idempotent. Publication withdrawal precedes
normal stop; a failed withdrawal records local withdrawn truth, blocks
republish, and reports remote convergence degraded.

`Published` means the local Node durably admitted the current publication and
the carrier accepted its bounded send. It does not promise that every remote
Node has converged; Application Discovery freshness and record expiry remain
the reader-side boundary.

### Authority and audit semantics

Exact Application actions:

```text
application.hosting.ensure
application.hosting.renew
application.hosting.read
application.hosting.recover
application.hosting.drain
application.hosting.publish.network
```

Resources:

```text
hosting-profile:<profile-id>          # ownerless, exact at Ensure
hosted-service:<owner>/<service-id>   # owner-required thereafter
```

`Ensure(NetworkPublished)` requires both `ensure` on the exact profile and
`publish.network` on that profile. Later mutations require their exact action on
the owner-qualified Hosted Service. Node-wide scope may be issued deliberately;
exact scope is preferred. The profile resource authorizes selection of the
Operator-controlled current revision, including its one pinned service type;
there is no separate caller-selected service-type resource or authority.

Hosting rejects presented Delegation in v1. The Actor Application Principal is
the durable owner and must equal Effective. This avoids a local Application
creating durable execution and publication owned by a delegating Principal.
A future delegated-hosting design would change durable ownership and requires a
new decision.

Every accepted mutation audits Actor, owner, action, Hosted Service ID, profile
ID, exposure, old/new stable state, revision, request ID, and outcome. Denials
audit the same fields available after safe parsing. Audit excludes raw workload
config, image, endpoints, container IDs, secrets, and detailed policy errors.
Read success is metric-visible but not a successful-mutation audit event.

Operator profile disable prevents new Ensure without stopping existing leases
or making their archived Plan unrecoverable. Existing pinned aggregates may
Renew and Recover. Operator revocation of a Hosted Service or exact profile
revision immediately advances matching aggregates to REVOKED,
withdraws/terminates them, and blocks renewal. Revoking an Access Grant blocks
subsequent calls but does not itself destroy a running resource; Operator
revocation of the Hosted Service/profile revision or lease expiry owns that
lifecycle transition.

## Delivery and data semantics

| Concern | Contract |
|---|---|
| Ordering | per Hosted Service revision is strictly increasing; independent services have no order |
| Acknowledgement | a mutation is acknowledged only after authoritative intent/result is durable; `Ready`/`Published` are observed states, not mutation acknowledgement |
| Deduplication | current revision plus bounded RequestID ledger makes mutation retries idempotent |
| Expiry | absolute lease expiry is persisted; no renewal after expiry |
| Limits | request/body, IDs, profiles, services, lease, drain, restart, result list, journal and idempotency history are bounded above |
| Backpressure | one mutation in flight per service; Node/global executor saturation returns `ResourceExhausted` or retryable `Unavailable`, never an unbounded queue |
| Large payload references | not applicable; Hosting accepts only IDs/enums/durations, never payloads or Content References |
| Terminal outcomes | RELEASED, EXPIRED, and REVOKED are terminal; FAILED is recoverable only through admitted `Recover` before lease expiry |

## Failure, restart, recovery, and migration

| Event | Caller outcome | Persisted truth | Retry rule | Operator action |
|---|---|---|---|---|
| malformed request/unknown field/unsupported protocol | `InvalidArgument` | none | no | none |
| invalid session | `Unauthenticated` | none | existing one session refresh only | repair enrollment/session |
| grant/profile/network action mismatch or Delegation | `Forbidden` | none | no | issue correct finite grant/profile |
| logical profile disabled/deleted before Ensure | uniform `NotFound` | none | no | enable or publish a new current revision |
| another owner's or absent ID | uniform `NotFound` | unchanged | no automatic retry | inspect Operator Hosting status |
| duplicate own name with different declaration | `Conflict` | original aggregate | drain or use new name | none |
| stale revision | `Conflict` with current safe status | unchanged | read then deliberate retry | none |
| store/journal write fails before effects | `Unavailable` | prior truth | bounded retry with same RequestID | restore storage |
| executor unavailable/start failure | DEGRADED or FAILED status | active lease and desired running; failure budget | controller retry within budget; then explicit Recover | repair Docker/runtime/profile |
| workload crash | publication withdrawn; RESTARTING/FAILED | active lease, new generation/restart count | automatic bounded restart; no SDK loop | inspect crash/resource evidence |
| readiness lost/stale | Ready/Published false, DEGRADED | active lease remains | probe/reconcile cadence only | repair service/probe |
| ingress capacity/config failure | not published, DEGRADED/FAILED | active lease remains | bounded internal retry | repair ingress configuration |
| network/policy publication loss | local withdrawal; DEGRADED | active lease and local withdrawn truth | bounded refresh; no caller-driven publish loop | repair reachability/policy |
| normal Drain | immediate withdrawal, then DRAINING/RELEASED | terminal desired state and journal | retry same RequestID | force-drain if deadline fails |
| lease expiry | EXPIRED; immediate withdrawal/termination | terminal expired truth | cannot renew/recover | acquire a new service |
| Operator revoke | REVOKED; immediate withdrawal/termination | terminal revoked truth | cannot retry | new explicit authorization/profile |
| Node restart with active unexpired lease | temporary PENDING/STARTING; never optimistic published | aggregate loaded first; derived workload reconciled | internal recovery | inspect if recovery fails |
| Node restart after expiry | EXPIRED, never restarted/published | expiry wins over prior execution snapshot | none | clean failed runtime if needed |
| wall clock moves backward >30 s | FAILED and withdrawn | last evaluated time retained | no automatic extension | repair clock, then explicit Recover if lease is still valid by trusted time |
| persistence corruption/mixed unsupported schema | Hosting unavailable, no Application-owned publication | fail-closed store/journal error | no | restore matching backup or supported migration |
| pinned profile revision missing/digest or projected-type mismatch | Hosting failed; affected service withdrawn and not executed | aggregate reference retained; corrupt archive is never substituted with current profile | no | restore the exact revision from a consistent backup |
| profile disabled/deleted after Ensure | existing service behavior unchanged; new Ensure is `NotFound` | aggregate keeps archived revision | existing service may Renew/Recover | revoke explicitly to terminate |

Startup order for Application-owned resources is: load Hosting truth and
journal; mark expired/revoked resources withdrawn; reconcile labelled runtime
inventory; resume/compensate journal; start active desired resources; establish
current-generation readiness; then publish. Existing startup discovery must not
republish stale Application-owned service records before this sequence.

Lease evaluation runs in the same bounded maintained reconciliation loop used
for Hosting projections and is also checked synchronously before every
publication/refresh. The controller does not maintain an unbounded mutation or
expiry queue.

### Compatibility and migration

- Wire: additive Application `HostingService`; existing Identity, Content, and
  future Discovery clients remain valid. Unknown fields continue to fail.
- Persistence: introduce one versioned `application-hosting/v1` consistency
  group containing aggregates, mutation journal, logical-profile current/
  disabled pointers, and immutable canonical profile revisions. Each aggregate
  includes owner Principal, opaque ID, `(profile_id, profile_digest)`, the
  profile-pinned canonical service type projected for safe reads,
  protocol/exposure, lease timestamps, desired state, revision, bounded
  idempotency history, derived workload identity, and safe outcome. On load,
  the projected type must equal the archived revision; mismatch fails closed.
- Existing workload snapshot remains execution evidence, not desired authority
  for Application-owned IDs. A reserved ownership marker prevents direct
  Operator WorkloadService mutation; Operator uses Hosting suspend/revoke/
  force-drain actions.
- Existing configured/Operator-created workloads and static services remain
  Operator-owned with no lease and are never automatically adopted or exposed
  to Applications.
- Backup/restore uses one consistency group containing Hosting aggregates,
  journal, current/disabled profile pointers, every referenced immutable profile
  revision, workload execution snapshot, local discovery/publication truth,
  identity/access state, and diagnostics metadata. Backup fails rather than
  omit a referenced revision. A backup is self-contained: later live-node
  archive GC does not affect it.
- Restore validates the complete group, archive digests/schema/service types,
  and every reference before mutating live state. It then re-evaluates absolute
  expiry and revocation before workload reconciliation or publication. A
  logically deleted/disabled profile does not invalidate a still-referenced
  archived revision; the restored aggregate continues pinned behavior. A
  missing or corrupt revision blocks restore and leaves the prior live
  consistency group authoritative.
- Rollout is two-phase: readers understand the new store before any writer is
  enabled; then Application Hosting/profile issuance is enabled. Old binaries
  must refuse downgrade while any Hosting consistency-group marker, aggregate,
  journal, profile pointer, or archived revision exists.
- Mixed generation: old Nodes ignore no new publication semantic because
  records remain current service records; only upgraded hosting Nodes create
  them. Application SDK feature negotiation must report Hosting unavailable on
  old Nodes. New readers support `application-hosting/v1` profile revisions;
  an unknown future revision schema blocks affected Hosting reconciliation and
  publication rather than interpreting it. Any future schema migration writes
  a new consistency group transactionally and retains the previous group until
  validation succeeds.
- Changing a profile creates a new archived revision/digest and advances only
  the current pointer. Disable/delete affects new Ensure only. Existing active
  aggregates remain pinned to reconstructable archived revisions until
  expiry/drain/revocation and terminal retention; silent template or service
  type replacement is forbidden.

## Security, privacy, and abuse analysis

- The primary threat is an authorized Application turning a Node into an
  unbounded executor or ingress publisher. Profiles, exact grants, per-owner
  and global quotas, resource ceilings, lease expiry, restart budgets, and
  connection limits bound that power.
- Caller-controlled runtime material is deliberately absent. The closed
  Operator profile preserves immutable image digest, registry allowlist,
  non-root/resource/isolation settings, no Docker socket, and runtime selection.
- Local-only means no discovery record and no externally bound ingress.
  Network-published requires a separate action and current publication gates.
- Another owner's missing service, wrong owner, expired tombstone, and
  unauthorized service are all `NotFound`. List is owner-filtered and capped.
- Status reasons are stable categories such as `starting`, `not_ready`,
  `runtime_unavailable`, `publication_unavailable`, `lease_expired`, and
  `operator_action_required`; internal endpoint/policy/container details remain
  Operator-only.
- Request replay cannot extend a lease without a current revision; bounded
  RequestID history returns the prior result. Session replay protections remain
  owned by Identity/Application admission.
- A malicious workload is contained by the existing Docker/gVisor/resource and
  isolated ingress contract. Hosting does not weaken those controls.
- Lease evaluation is independent of Application process liveness. Credential
  rotation does not change owner identity. Grant revocation blocks calls;
  explicit Hosted Service revocation or expiry terminates execution.
- Metrics never label Principal, service ID/name, profile, endpoint, image,
  port, container, or RequestID.

## Observability

Application status is intentionally small. Operator diagnostics correlate the
Hosted Service aggregate, derived workload generation, readiness, ingress,
publication record, and last journal phase.

Bounded metrics:

- gauge counts by stable state and exposure only;
- mutation totals by operation and stable result;
- lease expiry/drain/revoke totals;
- recovery/restart totals by stable reason;
- publication transition totals by stable outcome;
- orchestration duration histograms by operation;
- no high-cardinality resource labels.

Health:

- one failed service does not fail Node readiness unless capacity/control-plane
  invariants are compromised;
- Hosting subsystem is degraded when reconciliation, observation, withdrawal,
  or profile resolution is impaired;
- corrupt authoritative state, failed fail-closed withdrawal, or unreconciled
  ownership conflict makes Hosting failed and blocks new mutations/publication.

Operator procedure:

1. inspect owner-safe Hosted Service status and journal phase;
2. inspect pinned profile digest and current policy;
3. inspect derived workload/generation and Docker runtime;
4. inspect readiness and ingress;
5. inspect local publication, network convergence, and discovery withdrawal;
6. repair the owning dependency;
7. reconcile, or explicitly Recover/revoke/force-drain;
8. never edit Docker objects or persisted JSON to simulate recovery.

## Compatibility consequences

The design adds Application wire, SDK, access actions/resources, Operator
profile/admin contract, a persisted aggregate/journal, backup material, and a
reserved derived-workload identity. It does not change the current discovery
record shape or expose a direct client adapter.

The main irreversible compatibility choice is making the owner-qualified Hosted
Service, not a workload or endpoint registration, the public durable resource.
That is why ADR-0012 is required before implementation.

## Acceptance matrix

| Level | Required evidence | Environment | Commit-bound artifact |
|---|---|---|---|
| Unit | aggregate transitions; lease/clock bounds; revision/idempotency; profile canonicalization/digest/type pinning; archive reference/24-hour GC/cap; disable/delete/revoke distinctions; owner IDs; quotas; startup ordering; withdrawal-before-stop; terminal tombstone compaction | Go unit, deterministic fake clock/store/ports | JSON/JUnit tied to exact commit |
| Contract | Ensure has no service-type input; Status type equals the exact profile revision; profile resource authorizes that type; unknown fields and malformed bounds fail before admission; actions/resources; Actor==Effective; network extra action; owner-filtered privacy; typed errors; no internal fields | real Application handler/admission + SDK | protocol/SDK contract report |
| Integration | aggregate-to-archived-revision-to-workload/readiness/ingress/publication tracer; restart after current-profile change/delete uses the old Plan/type; missing/corrupt/mismatched revision fails closed; local/network modes; crash/restart budget; readiness loss; policy/network withdrawal; storage failure and journal resume | local substitutable runtime plus Linux integration | tagged JSON/JUnit |
| E2E | Application enrolls, acquires approved profile, observes its pinned service type, reaches ready/published, renews, survives Node restart and profile replacement/disable, drains; second Principal cannot observe/control; expired service never restarts | Linux container, protected Unix socket, real Node and Docker | scenario IDs and logs bound to commit |
| Security | arbitrary config/image/endpoint/port/cert/Delegation denied; quotas and rate pressure; malicious workload isolation; error/audit/metric redaction; replay/stale revision; revocation; clock rollback | Linux Docker + `runsc` where untrusted path claimed | security report and retained evidence |
| Deployment | profile revision/change/disable/logical-delete/revoke; archive cap and GC; Docker/ingress failure; atomic backup contains all referenced revisions; restore with active, disabled, deleted, expired and in-flight journal cases; missing-revision restore leaves live truth unchanged; mixed-schema rollout and downgrade refusal | supported private-LAN/public-direct deployment, no Kubernetes | deployment evidence bundle |
| Release | static, fast, race where supported, tagged, security, deployment, multinode as applicable, release candidate all pass once on one clean source commit; no retry hides failure | canonical release matrix | accepted qualification snapshot |

Capability `application.hosting` remains I=no/R=no/O=no/Q=no at the frozen
baseline. Implementation may advance I/R/O only with matching evidence. `Q`
stays `no` until DR-06 and the release gate accept one exact commit.

## Open questions

None that changes the selected external interface, trust root, persistence
authority, or migration contract.

DR-05 must decide how a client connects and authenticates, and whether/when
`https` becomes an admitted Hosting protocol. That downstream decision may add
a profile capability and client adapter, but it must not expose workload or
endpoint registration or change Hosted Service ownership/lease semantics.

## Decision-register proposals

| Type | Proposed row | Rationale |
|---|---|---|
| Decision | Application Hosting v1 is a finite leased, owner-qualified Hosted Service over an Operator-approved managed-workload profile; arbitrary endpoint registration is rejected | keeps one lifecycle/recovery authority |
| Decision | Each immutable profile revision pins exactly one canonical service type; Ensure cannot select it and Status projects it | keeps discovery identity under Operator-approved configuration |
| Decision | Actor Application Principal owns the Hosted Service; workload has no Principal; Node Principal signs publication; Hosting Delegation is rejected in v1 | prevents durable delegated execution ambiguity |
| Decision | Hosting consistency group archives canonical profile revisions and aggregate references; aggregate/journal is authoritative desired truth while workload, readiness, ingress and publication are derived projections | deterministic restart/restore after profile change or deletion |
| Decision | v1 Application protocols are HTTP/TCP; HTTPS waits for DR-05 TLS identity and secret delivery | avoids implicit certificate plumbing |
| Question for DR-05 | Define client connection/auth/TLS behavior without changing Hosting ownership, lease, profile, or endpoint-hiding boundary | explicit cross-stage handoff |

## Recommendation

Write ADR before implementation: propose
`docs/adr/0012-lease-application-hosting-through-approved-profiles.md`.

After approval, implement the following dependency-ordered vertical slices.
Do not publish issue files until the maintainer approves this granularity.

## Vertically sliced implementation issues

### AH-01 — Persist an owner-qualified Hosted Service aggregate

- User story: an authorized Application can Ensure a local-only approved
  service and read the same durable status after Node restart.
- Complete behavior: add logical profile catalogue plus immutable revision
  archive, canonical service-type pinning, aggregate/journal,
  ID/revision/RequestID/lease bounds, exact actions/resources, Application
  wire/SDK Ensure/Get/List, and one local-only fake-runtime tracer. Reject
  Delegation, caller-selected service type, and all arbitrary runtime material.
- Acceptance: unit/contract matrices for ownership, profile scope/type,
  revision digest/reference/GC/cap, disable/delete/revoke, privacy,
  idempotency, expiry, corruption, backup schema, and old-Node feature absence.
- Blocked by: ADR-0012 acceptance; Application protected-procedure registry
  prerequisite selected by Application Discovery.
- Research class after packet: R1 bounded implementation.

### AH-02 — Drive one managed workload to readiness

- User story: an Application sees its leased service become ready without
  learning workload details.
- Complete behavior: reconstruct the resolved Plan only from the aggregate's
  archived profile revision, derive reserved workload identity/spec, integrate
  Apply/Recover, surface the pinned service type and stable status, enforce
  per-owner/global quotas, and reconcile active/expired resources in safe
  startup order.
- Acceptance: real domain integration for start, failure budget, crash,
  restart, Node restart after profile replacement/disable/logical deletion,
  missing/corrupt revision, stale observation, clock rollback, and no direct
  Operator workload mutation of derived IDs.
- Blocked by: AH-01.
- Research class after packet: R1.

### AH-03 — Publish and withdraw through the same lifecycle

- User story: an authorized network-published service appears only when ready
  and disappears before drain, crash recovery, expiry, or revocation.
- Complete behavior: require network action/profile permission; integrate
  ingress/readiness/publication; implement connection drain; add Renew and
  Drain; preserve local withdrawn truth during carrier failure.
- Acceptance: local/network modes, HTTP/TCP policy, generation-bound ingress,
  readiness loss, policy/reachability change, partial publication compensation,
  drain deadline, expiry/revoke immediate termination, privacy and load bounds.
- Blocked by: AH-02; accepted DR-04 topology/support assumptions for deployment
  qualification, but not for local implementation.
- Research class after packet: R1/R2 at deployment boundary.

### AH-04 — Close Operator recovery and migration

- User story: an Operator can configure, inspect, revoke, recover, back up,
  restore, upgrade, and safely refuse downgrade for Application-owned services.
- Complete behavior: Operator profile/status/revoke/force-drain surface;
  diagnostics/metrics/audit; archive retention/GC; transactional consistency-
  group backup; active/disabled/deleted/expired/in-flight restore; two-phase
  enablement; profile revision pinning; mixed-schema and downgrade refusal.
- Acceptance: Operator authorization/privacy, journal interruption matrix,
  self-contained backup/restore, missing-revision negative restore,
  profile change/disable/delete/revoke, archive cap/GC, mixed-generation/schema
  rollout, bounded observability, and runbook tests.
- Blocked by: AH-03.
- Research class after packet: R2 migration/deployment implementation.

### AH-05 — Qualify the supported Hosting journey

- User story: release reviewers can prove the exact supported Hosting lifecycle
  on one clean commit.
- Complete behavior: protected-socket Application E2E, second-Principal
  negatives, real Docker/ingress crash and drain, Node/Docker restart,
  private-LAN/public-direct publication as selected by DR-04, security
  isolation, and retained release artifacts.
- Acceptance: every row of this packet's acceptance matrix is linked to one
  matching commit; capability evidence is updated without premature claims.
- Blocked by: AH-04, DR-04 accepted support topology, DR-06 gate definition.
- Research class after packet: R3 qualification.

## Cross-stage dependencies

- Upstream: ADR-0001; accepted Application Discovery locator boundary;
  Application admission registry extraction before the second protected product
  service; existing Workload/Hosting/Publication/Ingress contracts.
- Parallel cross-check: DR-04 supplies supported reachability topology and
  deployment ownership for AH-03/AH-05.
- Downstream hard dependency: DR-05 accepts Hosted Service ID, protocol,
  exposure, profile, readiness/publication, and endpoint-hiding semantics and
  designs only connection/authentication above them.
- Qualification: DR-06 owns matching-commit evidence and is the only stage that
  may support `Q=yes`.
