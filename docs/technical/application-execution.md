# Local Application Execution

Status: **current implemented boundary and lifecycle contract** for new local
Execution. `internal/successor/execution` owns authority/session/Job rules;
`execution/runtime` owns launch and supervision; `execution/worker` owns fixed
native mechanisms. Genuine permission preparation, Route holder, Publication
and bounded installed bootstrap consume this boundary. Exact imports remain
in the [package map](../development/package-map.md).

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
[confinement](application-confinement.md),
[local admission and Service lifecycle](../technical/endpoint-service-runtime.md#local-admission)
and [execution workflow](../development/agent-execution.md). [CONTEXT.md](../../CONTEXT.md)
remains the glossary. Predecessor source inventories are retained in the
archive; they grant no new-runtime import or shared authority.

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

The consistency model below names rule and lifetime owners, not one exported Go struct or package per concept.

| Consistency owner | State and invariants | Commands / observable transitions |
|---|---|---|
| **Local authority generation** (aggregate root) | Volatile grant set, pending capabilities, active session admission, permitted drain; exact surface budgets and synchronous invalidation | Admit, consume, activate, revoke, bounded drain, close |
| **Execution session** (aggregate root) | Exact admitted session, local cleanup reservation, current and last Job identity, closed flag; serialize Job start, handoff, retirement and completion | Begin Job, claim lifetime, accept verified binding, acquire one operation, retire, complete cleanup, check completed-current, close/join |
| **Job** (entity and lifetime owner within that session's consistency boundary) | Nonce, claimed handoff, worker binding, operation use, retired/finished state and one cleanup result | No mutation from neighbors; a replacement cannot receive its binding, result or released capacity |
| **Execution supervisor** (generation consistency owner) | Retained sessions/cleanup capacity, admission-open state and first cleanup failure | Reserve/release exact session; close admission; revoke all before joining any; fail the generation synchronously |
| **Worker lifetime** (physical resource owner) | Original attachment, pinned cleanup scope, INIT/operation/callback joins and one final result | Interrupt, join, return terminal result once; not an aggregate repository or qualified permission by itself |

The session owns local Job serialization. Job is not
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
session; that provenance never authorizes bytes from a dead worker. This preparation provenance remains distinct from a live operation permit; it never creates persisted permission or authority for a dead worker.

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

### Original Text operation and retained Reader result

The runtime also admits fresh local Administration requests for Link and
Withdraw through the same original authority and cleanup supervisor. Each
request consumes its exact Principal/surface capability and joins its original
session. It creates no Job or qualified worker and supplies no Publication or
Connection authority. The installed Administration consumer additionally checks
the original startup and actual retained Publication. Snapshot publication
continues to require its separately qualified Publisher invocation and operation.

The ordinary Reader and selected-snapshot Publisher each consume their original
qualified invocation's one operation and a trusted directional byte contract.
Text retains the sole initialized protocol reader; Execution does not run the
setup-only EOF reader concurrently with it. One stream transfer is one local
Application request. Connection replacement changes an Attachment, never the
operation, Job, byte Grant or selected snapshot.

The Reader's local authenticated completion may retain Connection's bounded
control tail. Its bounded private result remains pending while the original
stream and operation retain that tail and composition joins the original Source.
Closing the successful operation releases and joins
the Text reader before Job retirement, then the invocation joins its original
descendants. Only that exact successful completed-current Job may expose the
result once. Revocation, failed protocol use or a later Job invalidates it;
pending bytes supply neither live worker authority nor physical completion.

Successful Publisher Text completion retains the original Service stream and
worker cleanup borrow until that stream's original joined notification or
original operation cancellation. Application Done does not trigger full Close
while authenticated Terminal controls remain in flight. Cancellation and refusal
interrupt the wait and join the same resources immediately; neither completion
nor the notification renews authority, bounds or the Application request.

Borrowed owners report only actual joined physical cleanup failures to the
operation's cleanup accounting. Their full exchange results retain transport
loss, refusal and authenticated logical failure separately. A failed old
Attachment with clean physical retirement does not terminate the Job, while
an actual write/close/join failure still closes generation admission. No error
classification, retry or new operation may erase that physical failure.
The Text Publisher retains each Service's full Close result separately and,
after Close joins that same Service, consumes its `JoinedCleanup` receipt for
worker cleanup. A logical control-tail cancellation cannot poison the snapshot
Job. Actual physical failures from every joined Service remain retained.
The Reader Connection Source closes its original Prefix and retains that full
terminal result separately. Cleanup accounting consumes Prefix's exact joined
physical receipt, not its retired-readiness notification or complete authority
refusal. A missing original join still fails; a nil physical receipt cannot
authorize another Job or restore the canceled Reader.

## Implementation and evidence limits

[Execution's source entry point](../../internal/successor/execution/README.md)
and [testing profiles](../development/testing.md) own real consumers and selected
environments. Native hostile-tree, escape and Endpoint-death components retain
their own artifact and observer contracts. Local worker restart does not prove
logical Service recovery. Portable compilation does not qualify native
confinement, a complete installed Service, whole-host protection or anonymity.
The [domain map](../development/domain-map.md) records collaborations; GitHub
owns delivery/acceptance. The [original design and source inventory](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/development/application-execution-domain-boundary-analysis.md)
remain historical provenance, not an alternate new-runtime destination.
