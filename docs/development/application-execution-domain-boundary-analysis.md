# Local Application Execution boundary

This is the orchestrator's source-backed domain design for the delegated DDD
transition. It defines a proposed new owner, not an implemented package, installed
qualification receipt or amendment to the product/protocol contract. Execution
selection, implementation and delivery remain in the GitHub ledger. The
[domain map](domain-map.md) owns the complete architectural inventory.

## Responsibility and DDD posture

**May this exact local Application invocation perform this operation now, and
what must finish before its authority and cleanup capacity can be released?**

This is a supporting subdomain with a rich authority/lifecycle model. It warrants
DDD because revocation, one-use rights, late completion, physical descendants
and finite cleanup capacity have different consistency and termination rules.
It is a brownfield modular Go implementation, not a new service, database,
general process launcher or framework. Linux mechanisms remain adapters.

Authoritative owners are [scope](../product/scope.md),
[threat model](../security/threat-model.md),
[confinement](../technical/application-confinement.md),
[local admission and Service lifecycle](../technical/endpoint-service-runtime.md#local-admission)
and [execution workflow](agent-execution.md). [CONTEXT.md](../../CONTEXT.md)
remains the glossary. Source paths below refer to maintained predecessor code,
not the remote `old` branch and not a permitted new-runtime dependency.

## What belongs here

| Responsibility | Exclusive Execution ownership |
|---|---|
| Local permission | Match an exact Application Principal, Local Grant, allowed surface and generation; admit and consume one-use capabilities; revoke/drain within existing finite bounds |
| Local admission lifetime | Retain cancelable sessions and separate finite cleanup reservations; reject new work synchronously after revoke or a terminal generation failure |
| Job | Fresh invocation nonce, exact session membership, one worker-lifetime handoff, one operation, currentness and completed-result checks; first immutable joined cleanup result |
| Qualified local binding | Create a fresh private Principal and byte-stream Grant only after verified artifact, physical invocation, attachment credentials and readiness recheck |
| Worker lifecycle | Reserve before activation; claim cleanup before INIT; interrupt attachment; join initialization, operation, cancellation callback and all physical descendants |
| Physical confinement | Fixed installed worker inventory, effective systemd properties, exact invocation/cgroup, descriptor audit and no-FD-transfer attachment; fixed bounded stop and original-scope observation |
| Failed cleanup | Retain the failure; close all text-Job admission for that runtime generation, including idle sessions and late completions; no reset through a new session |

Execution owns local permission, not the network Admission domain. Local Grant,
network token, software authorization and confinement evidence are distinct facts.

## What stays outside

| Neighbor / supporting owner | Its own rules; Execution must not acquire them | Narrow seam |
|---|---|---|
| Network | Accepted signed State, membership/duty, profile conflicts and trusted network time | Network operation composition obtains genuine current authority separately |
| Admission | Issuance, permission, stock/presentation, receiving verification and irreversible spend | A local operation permit is a prerequisite, never a token or spend approval |
| Hosting | Durable provider-period Tx/Rx budget and reservations | Physical work keeps Hosting reservations until its own joined completion; no duplicate local budget |
| Route | Entry/Interior selection, role-purpose transport, lanes/credit, Introduction slots, JOIN and physical Carrier termination | Composition checks exact local operation before effects and handoff; Route closes its own borrowers |
| Service Publication | Authorized Instance, generation/revision, registration pair, readiness ACK, refresh, withdrawal and private material | Exact live publisher operation; revoke stops acquisitions; Publication joins and retires itself |
| Reachability | Exact-Target Descriptor verification, Store conflicts, history and cache | Lookup remains a separately authorized operation; no Descriptor authority from a Job |
| Service Connection | Destination/provenance, Instance authentication, logical bytes, Attachment generation, recovery deadline and terminal result | Retain exact Job lifetime; recovery never creates a new Job, Grant or Application operation |
| Software acceptance | Enrollment/Release verification and floors, selected artifact authorization, installation/replacement journal and predecessor transition | Runtime consumes exact authorized artifact/binding; verifying installed bytes does not authorize a Release |
| Custody | Purpose-specific authority storage and approved signing | No private keys, general signing interface or authority export in Execution |
| Text Application | Snapshot import, UTF-8/document grammar, stream multiplexing/credit and escaped presentation | Fixed initialization protocol and bounded byte-use lifetime; document semantics remain with Text |
| Application IPC | AAI3/Administration wire grammar, socket setup/peer cancellation and response classification | Consume local authorization and return/close actual operation handles; IPC cannot attest confinement |
| Process resources | Whole-process pressure, placement, measurements and NORMAL/PROTECT/DRAIN policy | Worker confinement limits are physical enforcement; global pressure remains a different owner |
| Qualification | Run identity, workload schedule/canaries, observers, sampling and reports | A separately selected fixed worker may use the lifecycle mechanism; qualification is not ordinary Job policy |
| Command composition | Parsing, construction and ordered supervision of genuine domain owners | Thin adapters; no permission decision, cleanup registry or shadow domain model |

An Isolation Context also owns private network separation. Execution receives
only its local admission/session part. It must not absorb the whole current
`dutyContextState`, Endpoint or Node. A canceled `context.Context` is a lifetime
signal, not a substitute for an exact local permit or physical completion.

## Tactical model

The [class model](application-execution-domain-model.puml) is conceptual, not a
promise of exported Go structs or one package per class.

| Consistency owner | State and invariants | Commands / observable transitions |
|---|---|---|
| **Local authority generation** (aggregate root) | Volatile grant set, pending capabilities, active session admission, permitted drain; exact surface budgets and synchronous invalidation | Admit, consume, activate, revoke, bounded drain, close |
| **Execution session** (aggregate root) | Exact admitted session, local cleanup reservation, current and last Job identity, closed flag; serialize Job start, handoff, retirement and completion | Begin Job, claim lifetime, accept verified binding, acquire one operation, retire, complete cleanup, check completed-current, close/join |
| **Job** (entity and lifetime owner within that session's consistency boundary) | Nonce, claimed handoff, worker binding, operation use, retired/finished state and one cleanup result | No mutation from neighbors; a replacement cannot receive its binding, result or released capacity |
| **Execution supervisor** (generation consistency owner) | Retained sessions/cleanup capacity, admission-open state and first cleanup failure | Reserve/release exact session; close admission; revoke all before joining any; fail the generation synchronously |
| **Worker lifetime** (physical resource owner) | Original attachment, pinned cleanup scope, INIT/operation/callback joins and one final result | Interrupt, join, return terminal result once; not an aggregate repository or qualified permission by itself |

The session replaces the *local* role of the current Context mutex. Job is not
made an independently transactional aggregate merely because it has its own
identity. Global admission and a Job's local transitions still need explicit
synchronous ordering; an eventual event cannot implement revocation. Do not
hold one owner's lock while waiting for I/O or call back into a locked owner.

Entities include Local Grant, admitted session and Job. Immutable value objects
include the closed surface, capability admission window, original operation
bound and observed physical invocation tuple. These identities stay separate:

- Broker generation and consumed receipt commitment;
- fresh Job nonce and exact current/last Job reference;
- systemd InvocationID with unit/PID/UID/cgroup and pinned kernel descriptor;
- fresh qualified worker Principal and its local Grant;
- Service Connection identity and Attachment generation, owned by Connection.

An opaque verified launch binding is a retained resource/proof handle, not a
copyable value object, supplied boolean or naked PID. It cannot be reconstructed
from wire identifiers. A joined successful preparation Job may leave a distinct
private **launch provenance** for permission preparation under a still-live
session; that provenance never authorizes bytes from a dead worker. Current
source retains this in `verifiedJob`; the new model must make the distinction
explicit without inventing a persisted permission or silently breaking bootstrap.

Factories create nonzero fresh identities and acquire reservations before
effects. A separate launch application service orders the physical checks and
publishes a binding only to the exact live claimed Job. Domain services are
needed only for rules that do not belong to those state owners. No mandatory
repository, generic CRUD service, event store, CQRS split, message bus or event
on every mutation is justified. Runtime authority is volatile. Durable Release,
Network, Admission and Hosting roots remain with their existing owners.
Notifications may report Job started/retired/cleanup completed or generation
closed after the synchronous transition; they cannot grant rights or make a
late replacement current. An exported event API requires a real consumer.

## Required lifecycle and linearization

1. Consume the exact Principal/surface capability and retain local cleanup
   capacity before activation or destination-dependent effects. Pending
   capability TTL is not an active-session or Job deadline.
2. Serialize activation against the shared installed worker inventory. The
   current process-wide gate cannot become a per-Job lock that permits two
   baselines to consume each other's candidate.
3. Observe authorized fixed artifact/inventory, active Endpoint parent and one
   exact candidate. Retain ambiguous/queued-start cleanup obligations even if
   dial fails. Do not select an arbitrary executable or replacement candidate.
4. Claim the lifetime once and pin original cgroup cleanup before INIT. Validate
   attachment credentials on each read, reject/close SCM_RIGHTS and audit
   inherited descriptors. READY alone is not confinement proof.
5. Recheck artifact and exact live invocation after readiness. Under the local
   ordering boundary accept a fresh worker Grant only for the current Job.
   Reject and close a late handoff against its original owner.
6. Acquire the single worker operation. Recheck local currentness at network
   effects and final handoffs independently of Network authority. Publisher
   administration is never delegated to the worker's byte Grant.
7. On revoke/close: deny new admission, invalidate local rights and interrupt
   every dependent owner before waiting for any. Each neighbor stops and joins
   its own state/resources; composition supplies the dependency order.
8. Join operation, initialization, callback and descendants using the original
   pinned scope and a finite cleanup context independent of canceled Job context.
   Never stop a replacement invocation. Path absence, MainPID zero, failed stop
   or timeout alone is not proof of join. Original removal is established by
   the accepted pinned-core-file semantics, not reopening a pathname.
9. Retain the first joined error, release only the exact reservation, and reject
   a result unless its successful joined Job is still the session's last Job.
   A replacement that has already finished still invalidates the old result.
   Cleanup failure closes the generation; repeat Close cannot make it green.

Local cleanup capacity and Broker active count are distinct. Releasing a revoked
lease does not prove joined descendants or create new cleanup capacity. Network
token spend remains irreversible; local release cannot refund it.

## Source inventory: collect, split and leave

These are inspected predecessor responsibilities to classify during migration.
The same filename or similar cancel/done shape is not proof of common ownership.

| Current source | Action | Required destination / retained neighbor |
|---|---|---|
| `internal/application/broker/{broker,contract}.go` | Collect local matching, capability/window, active leases, revoke/drain and independent budgets | Local authority generation; keep generic/unqualified mechanics distinct from qualified launch |
| `internal/endpoint/{duty_context,duty_context_shutdown,job_lifecycle}.go` | Split local session, exact Job and supervisor capacity/failure rules out of mixed Context | Execution; leave token stock, Source, Publication, Introduction, Descriptor history and private routing with their owners |
| `internal/endpoint/{worker_grant,worker_lifetime,worker_initialization,worker_launch}_linux.go` | Collect exact handoff, one-use, completed-current, launch ordering and cleanup transfer | Execution application/lifetime; text INIT/workload and qualification selection remain composition |
| `internal/endpoint/worker/*_linux.go` | Adapt fixed activation, invocation/properties/artifact inspection, credentials, cgroup pin and cleanup | Linux Execution adapter; artifact authorization and privileged installation remain Software acceptance |
| `internal/endpoint/duty_context_retirement.go` | Split Job retirement from multi-domain stop/join composition | Execution joins its Job; Publication/Route/Source owners detach and join themselves; no generic callback registry |
| `internal/endpoint/{service_binding,join_service,introduction_prepare}.go`, `introduction_exchange_lifetime.go`, `service/{stream,attachment}.go` | Replace scattered local currentness checks with exact new operation/binding checks; preserve effect and final handoff checks | Execution supplies local permission; Service/Route retain all auth, Target, bytes, recovery and transport policy |
| `internal/endpoint/{publisher_start,administration,service_reader,service_publisher}_linux.go` | Split local Job acquisition, cleanup and final current-result acceptance from workload/publication | Execution consumes one use; publication claim/readiness and text exchange remain neighbors |
| `internal/endpoint/{permission,permission_provision_linux}.go` | Split retained verified-launch provenance from State/permission/stock decisions | Execution supplies non-live launch provenance under live session; new Network/Admission composition owns authority and permission |
| `internal/endpoint/operation_flight.go` | Leave Source-specific reservation/prefix completion out | Source/application lifecycle; do not turn it into universal Execution flight |
| `internal/application/connection/server_admission.go`, `transport.go` (with `server_lifecycle_test.go` and `server_setup_cleanup_test.go` oracles) | Preserve transport-local setup cancellation and joined refused-stream cleanup | IPC adapter, not a Job authority or confinement owner |
| `internal/application/textdocument/worker_process_linux.go` | Extract/adapt inherited descriptor and physical interruption mechanism only where genuine consumers justify it | Execution physical boundary; text dispatch/grammar/multiplexer stay Text |
| `internal/application/textdocument/{worker,worker_scheduling,worker_initialization,worker_attachment_initialization}.go`, snapshot/presentation files | Retain text semantics and protocol-local joins | Text Application; no destination, key or publication authority passed to worker |
| `internal/endpoint/installation/{process_binding,start,predecessor_join,release_authentication,installation_lease,generation_ownership}*` | Leave authorization/journals/replacement rules; inspect shared mechanism without moving their authority | Software acceptance; predecessor shutdown transaction is not ordinary Job cleanup |
| `internal/resource/*`, `internal/qualification/*`, `internal/application/streamqualification/*` | Leave global pressure, Run/report/observation and qualification workload; identify lifecycle mechanism consumers explicitly | Separate supporting and test owners, not a wider Execution aggregate |
| `packaging/text-worker/*`, `cmd/ardents-text`, `cmd/ardents-stream-qualification` | Adapt verified fixed-worker entrypoints and installed configuration with their owning change | No runtime unit/polkit editing, arbitrary exec, extra descriptors or caller-selected privileged command |
| `cmd/ardents`, Endpoint participant/Connection/Publisher composition | Replace local policy with genuine new owners in the new composition | Old command cannot consume new Route/Execution through callbacks, wrappers or local process bridges |

Real predecessor callers are `cmd/ardents` -> `RunClosedParticipant` -> admitted
Context -> reader `launchWorker` / publisher `startPublisher`; fixed text worker
entrypoints are `cmd/ardents-text` -> `RunInheritedWorker`. Qualification has
its own runner and fixed Stream worker. Their existence supplies source evidence,
not reachability of a new Execution owner.

## Proposed implementation shape and genuine consumer

Use `internal/successor/execution` for local authority/session rules;
`execution/runtime` for actual launch/lifecycle composition and supervisor;
`execution/worker` for Linux installed-worker mechanisms **only if each boundary
earns its package through actual implementation and callers**. The names are
proposals, not package registration or permission to create skeletons.

Rules import standard library only. Linux worker adapters do not import domain
policy or old Endpoint. Runtime imports only these rules/mechanisms and exact
new owner contracts needed by its real use case. Thin `cmd/ardents-next`
participant composition constructs the owners. Its real operation must activate
a selected installed worker, establish an exact local binding, execute/retire
the bounded use and join descendants; when it acquires a protected path it uses
genuine new Route, Network, Admission and Hosting. No fake Service ACK or logical
Connection is added before those new owners exist.

The owning implementation registers exact imports, commands, doc.go, behavior
tests and non-test callers together. Reuse of independent text grammar must be
checked against both dependency closures and owned language; it is not permission
to import a mixed predecessor consumer. Installation adoption and persisted-root
conversion require their own compatibility contract. No live old root, mutex,
lease, reverse consumer or process bridge is a migration shortcut.

## Contract discrepancies and resolution direction

| Evidence | Resolution to carry into the owning change |
|---|---|
| Resolved documentation discrepancy: the previous Administration description omitted retained leases, but `beginDutyContext` activates one; `surfaceAdmissionLoadLocked` counts pending plus active, and shutdown tests retain six Administration Contexts | The accompanying Service owner and package map now name the retained Context lease separately from receipt-only authorization. Preserve the existing combined six-slot admission bound and separate cleanup reservation; do not create another budget or weaken revoke |
| Confinement/current qualification owner select Ubuntu 24.04/systemd 255; worker mechanisms and installation separately accept Ubuntu 22.04/systemd 249 | New ordinary Execution admission must enforce its selected 24.04/255 profile. Mechanism compatibility or an installation lane is not an ordinary-product support/qualification claim; preserve any separately accepted installation contract until its owner is reconciled |
| Fixed Stream qualification command reads inherited stdin/stdout directly; current qualification owner requires the same descriptor boundary, while Text has an inherited AF_UNIX/null/extra-FD audit | The physical launch boundary must cover every selected worker consumer. Carry a causal injected-FD control and blocked-I/O join check; no static-review claim of an exploit or reproduced hang |
| `permissionProfileLocked` uses retained `verifiedJob.workerGrant` after joined preparation rather than requiring a live worker | Model launch provenance separately from live operation permission; preserve bootstrap ordering and inspect all consumers before changing retained state |

These are precise source/document discrepancies, not new product options. Repair
documentation and defects against accepted owners and independent oracles;
do not freeze accidental old behavior as an invariant.

## Verification and release boundary

Every row needs a positive case, a causal refusal/control, source identity and
terminal owner observation. Existing tests below are oracle candidates, not
proof that the new architecture works or that the selected installed host passed.

| Invariant | Required scenario / independent observation |
|---|---|
| Exact local permission | Wrong Principal/surface/generation, replay, expired pending capability, revoke during activation and permitted non-extendable drain; no worker/network effects on refusal |
| Distinct admission and cleanup capacity | Exhaust each existing surface independently; revoked lease plus pending descendants still blocks capacity; cleanup failure denies idle/fresh sessions synchronously |
| Exact Job and one handoff/use | Late launch after revoke, foreign Job, double lifetime claim/use, late Grant closed, old result after a replacement starts and completes |
| Physical identity | Unit/PID/UID/InvocationID/artifact changed before and after READY; queued activation ambiguity, extra candidate, original vs replacement cgroup; replacement never stopped |
| Descriptor confinement | Credential change on later read, SCM_RIGHTS injection with FD closure, inherited extra FD/null/AF_UNIX audit for Text and Stream; worker cannot access Endpoint authority |
| Joined termination | Cancel during activation/INIT/read/write; child/grandchild survival control; original removal vs missing path; unavailable manager; repeat Close preserves failure; revoke every session before any join |
| Result ownership | Physical writer/attachment/cgroup failures reach final command/session result; successful bytes withheld after revoke or non-current Job; cancellation cause preserved |
| New-domain integration | Genuine signed State, new tokens and durable roots, Job loss around presentation/reservation/spend and I/O; no refund; exact retained Hosting/Route reservation; TCP/TLS and QUIC; State/expiry/late handoff/races/reopen |
| Provenance versus permission | Joined preparation can support the intended permission bootstrap; the same proof cannot acquire a live worker operation or authorize a replacement |

Inspect the existing Broker, `duty_context*`, `worker_grant*`, `worker_lifetime*`,
`worker/cleanup*`, `worker/attachment*`, `worker/instance*`, Service cancellation
and recovery-authority tests, permission-preparation and IPC setup-cleanup tests.
Keep cross-domain scenarios in registered test composition, not a product domain
or test-only command. Mocks may isolate failures, never manufacture successful
authority, physical qualification, spent rights or Service readiness.

The executor runs full regression of all affected new domains, `make quick-check`
and `make check` on the final source; installed lifecycle/tree/escape/network/
recovery lanes use their checked privileged profile with genuine artifacts.
Missing prerequisites invalidate that lane, never pass by skip. Record causal
failures and changed-source rechecks. Separate component readiness, issue
acceptance, dev integration and installed/privacy qualification. Release includes
scoped commits, verified dev integration/push, corrected owners/map and exact
receipts under the standing delegated authorization; no repeat human approval.

## Inspection provenance and limits

Read-only inventory on 2026-10-04 started at
`d08dbc18b2a8542a103c5655fd27e75fe6cf8a42` and was reconciled against
`edbcfb1c420a652c55c5860cb8595908f9b6c1fb` on local dev. Intervening changes
were Route/prefix and the delegated workflow/map documents; inventoried
Execution implementation and its current technical owners had no diff before
the accompanying correction to the local admission description.
Two bounded read-only inventories informed this design. The orchestrator also
inspected Broker loading/activation, Job and supervisor cleanup, qualified
handoff/result checks, permission provenance, worker lifetime/cleanup, platform
checks and the fixed Stream entrypoint. No tests were executed for this design.
Static descriptor absence is not a demonstrated escape or hang. Current tests
and reported predecessor qualification do not establish new Execution delivery.
