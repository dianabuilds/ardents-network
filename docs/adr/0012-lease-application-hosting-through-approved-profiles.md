# ADR 0012: Lease Application Hosting Through Approved Profiles

- Status: Proposed
- Date: 2026-07-25
- Decision owners: Application Interface, Workload and Hosting, Security
- Research: `docs/engineering/research/application-hosting.md`

## Context

Ardents already lets an Operator register and reconcile a workload and derives
hosted-service readiness, ingress, and signed discovery publication from its
current generation. Applications have no Hosting surface. Exposing the Operator
workload API would violate ADR-0001 and make Applications coordinate workload,
readiness, ingress, and publication state.

Registering an arbitrary existing endpoint is also insufficient. The
Application or another supervisor would own process crash/restart while the Node
owned probes, lease, and publication. After restart, the Node could withdraw an
unready address but could neither prove endpoint generation nor recover the
service. This creates two lifecycle authorities.

The new public resource changes Application wire, authorization, persistence,
backup/restore, and downgrade behavior, so it requires an architectural
decision before implementation.

## Decision

Application Hosting v1 exposes one owner-qualified Hosted Service aggregate.
The authenticated Actor Application Principal is its owner; Hosting mutations
require `Actor == Effective` and reject Delegation in v1.

An Application Ensures a Hosted Service by selecting an immutable
Operator-approved hosting profile, `http` or `tcp`, local-only or
network-published exposure, and a bounded lease. It cannot submit an image,
command, environment, runtime, resource limit, endpoint, port, probe, service
type, certificate, secret, policy reference, mount, device, or Docker option.
Each immutable profile revision pins exactly one canonical service type.
Hosting Status projects that type for Discovery and DR-05, but Ensure has no
service-type input and the Application cannot override it. Arbitrary
existing-endpoint registration is not supported.

The exact Ensure Access Grant targets `hosting-profile:<profile-id>` and
authorizes selection of that logical profile's current approved revision,
including its pinned service type. Network exposure additionally requires the
exact network-publication action on the same profile. Service type is not a
separate caller-selected resource.

The Node persists one versioned Hosting consistency group as authoritative
desired truth. It contains Hosted Service aggregates, the mutation journal,
logical-profile current/disabled pointers, and the canonical bytes of immutable
content-addressed profile revisions. Each aggregate references
`(profile_id, profile_digest)`; a digest without the archived revision is not
sufficient state. The Node reconstructs the resolved Plan from that exact
revision and derives one internal managed workload, generation-bound readiness,
ingress, and optional signed discovery publication. The workload has no
Principal. The Node Principal signs publication and does not lend its authority
to the Application or workload.

Changing a logical profile writes a new immutable revision and atomically
advances its current pointer. Existing aggregates remain pinned to their old
revision and service type. Disabling or logically deleting a profile prevents
new Ensure calls but does not prevent existing aggregates from Renew, Recover,
reconcile, or drain. Explicit Hosted Service/profile-revision revocation, not
disable/delete, terminates existing aggregates.

A profile revision cannot be physically collected while referenced by a
current pointer, aggregate (including its 24-hour terminal tombstone), or
in-flight journal. It remains retained for another 24 hours after its last
reference disappears. The Node retains at most 1,024 current-or-archived
revisions and rejects creation at the cap; it never evicts a referenced
revision.

The Application surface is one lifecycle: Ensure, Renew, Get, List, Recover,
and Drain. It returns stable Hosted Service status without workload, endpoint,
container, runtime, topology, trust, or policy details. Network exposure
requires a separate exact Access Grant action.

An active lease permits reconciliation but never implies readiness or
publication. Publication additionally requires the current workload generation,
readiness, ingress, network reachability, and policy. Drain withdraws discovery
and stops new ingress before bounded connection drain and workload stop.
Expiry or Operator revocation withdraws and terminates immediately.
Every signed service record expires no later than the durable lease, so delayed
network withdrawal cannot extend remote freshness past Hosting authority.

On restart, Hosting truth and journal load before any Application-owned service
can be published. Expired/revoked aggregates win over derived workload state.
Current Operator-created workloads remain Operator-owned, unleased, and are
never adopted. A missing revision, digest/type mismatch, or unknown revision
schema fails closed before execution or publication.

Backup/restore captures the complete Hosting consistency group, every referenced
profile revision, workload snapshot, local discovery/publication truth,
identity/access state, and diagnostics metadata atomically. Backup fails rather
than omit a referenced revision. Restore validates the entire group before
replacing live truth and re-evaluates expiry/revocation before execution.
Disabled/logically deleted profiles remain reconstructable from their archived
revisions. A missing or corrupt revision rejects restore and leaves prior live
truth unchanged.

Rollout enables writers only after readers understand the archive schema.
Unknown future revision schemas fail closed. Old binaries refuse downgrade
while any Hosting consistency-group marker, aggregate, journal, profile pointer,
or archived revision exists.

`https` is not Application-selectable until DR-05 decides service TLS identity,
rotation, and secret delivery. DR-05 also owns direct client connection,
Principal authentication to the service, authorization, retry, and application
protocol semantics. It must not change this Hosting ownership or lease boundary.

## Consequences

- Applications receive a small durable lifecycle without Operator authority or
  orchestration internals.
- Workload Control remains the only restart/generation owner; Docker automatic
  restart remains disabled for managed workloads.
- Operator profiles and exact grants bound an authorized malicious
  Application's execution and ingress authority.
- Service type is an Operator-owned immutable profile property, not caller
  input, while the safe value remains visible in Hosting Status.
- Hosting must add a versioned consistency group, immutable profile-revision
  archive and GC, logical profile catalogue, Operator revoke/recovery controls,
  atomic backup/restore integration, and additive Application protocol/SDK.
- Derived workload IDs need an ownership marker and cannot be mutated through
  ordinary Operator workload commands; emergency actions use Hosting
  administration.
- Endpoint registration and delegated Hosting remain rejected rather than
  becoming compatibility commitments.
- Capability qualification remains a separate DR-06 release decision.
