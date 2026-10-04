# Route migration contract

Status: **mandatory engineering contract** for the isolated Route migration.
The Product Owner requested these persistent rules on 2026-10-04. This document
authorizes no implementation issue, package, dependency, deployment or product
contract change. The root [repository instructions](../../AGENTS.md) require
reading it after every compaction and handoff. Its rules survive a new chat,
worktree, agent, checkpoint or source-file rename.

## Read before editing

Before the first edit, after every compaction/handoff, and before accepting a
checkpoint as current, perform this sequence:

1. Read this entire file from disk and the root `AGENTS.md`. If either is
   missing or unreadable, recover the authoritative file before Route edits;
   do not reconstruct its rules from a summary.
2. Verify repository/worktree path, branch, HEAD, staged/modified/untracked
   work and any active process handles. Preserve unrelated and predecessor work.
3. Read the selected issue and its current C0 milestone/WIP admission. Identify
   the one assigned slice, real consumer, observable acceptance boundary and
   remaining evidence. This contract is not an execution ledger.
4. Follow the current-document route: [scope](../product/scope.md),
   [threat model](../security/threat-model.md),
   [protected Route](../technical/protected-route-protocol.md),
   [new Network](../technical/successor-network-state.md),
   [new Admission](../technical/successor-admission-boundary.md), affected Hosting
   owner, [package map](package-map.md), [testing](testing.md), and
   [agent execution](agent-execution.md). Follow linked ADRs for consequential
   contradictions, not historical source as a competing requirement.
5. Read the [owner map](route-domain-boundary-analysis.md). Verify affected
   owners and callers against current source; its old-source inventory is
   evidence, not an integration destination or permission to import it.
6. Resume from verified evidence. An earlier recommendation, summary, passing
   narrower test or report of completion is not a new authorization.

Report the concrete slice and next result concisely. Routine implementation
choices within the accepted scope do not require repeated permission.

## Starting in another worktree

The Product Owner explicitly directed this migration to continue in the existing
project checkout on 2026-10-04: development is required; another worktree is not.
Use `C:/Users/vitek/code/ardents-network`, preserve its existing branch and mixed
work, and keep one implementation owner. Do not create or require another Route
worktree. A committed predecessor is a source requirement, not permission to
choose a different checkout. The conditional instructions below apply only if
the Product Owner later explicitly chooses another checkout.

The Product Owner corrected the sequencing on 2026-10-04: first obtain the
predecessor's scoped commit and verification of that exact revision. If another
checkout is explicitly authorized, prepare it only after that verification.
Do not create a worktree from an
older base and copy uncommitted predecessor implementation into it. A checked
dirty snapshot remains useful evidence, but is not the implementation baseline.
The predecessor implementer owns completing that commit; the orchestrator must
not create a competing copy or edit their implementation concurrently.

Git worktree creation does not copy uncommitted or untracked instructions.
Before Route implementation in another checkout, verify that it contains this
contract, the root `AGENTS.md` Route rule and the architecture guard. If they
are absent, read the authoritative prepared files supplied by the Product Owner
and transfer the scoped policy without overwriting unrelated target changes,
or use their integrated Git revision. Do not begin Route work on the assumption
that another thread remembers the policy. A contract existing only in the
preparing checkout is not enforcement in the implementation checkout.

Use this task preamble, together with the explicitly selected issue and the
authoritative contract path/revision when the worktree does not yet contain it:

```text
Implement only the explicitly selected Route slice. Before any edit and after
every compaction/handoff, read AGENTS.md and docs/development/route-migration-contract.md
in full from disk. Verify that the policy and its architecture guard exist in
this worktree. New Route connects only to new domains and ardents-next; no old
runtime bridge, reverse consumer, test import or shared live root is permitted.
Do not weaken policy or checks. Preserve unrelated work and report evidence
for the real accepted boundary. Summaries do not override the contract.
```

## Rules that cannot be bypassed by implementation

### R01: Isolate both directions

New Route belongs in the new-domain tree, with `internal/successor/route` as
the proposed source destination. Its first real composition consumer belongs
in `cmd/ardents-next`. Exact packages require the normal complete registration.

New Route must not import or execute old `internal/route`, `internal/entry`,
`internal/endpoint`, `internal/node`, `internal/admission`, `internal/hosting`,
`internal/service` or another old runtime owner. Old packages, commands and
tests must not consume new Route. The prohibition includes subpackages,
build-tagged code, external-package tests, transitive imports, alias imports,
wrappers, callback adapters, plugins and local protocol bridges to old runtime.
Renaming or moving an old bridge to another directory does not make it new.

No new/old owner may share a mutable runtime object, context lock, live ledger,
root lease or reservation as a migration shortcut. Existing Network-specific
exceptions confer no Route exception. Do not add one.

### R02: Use actual new owners

Route application composition uses reviewed narrow contracts from new Network,
Admission and Hosting. Network owns current authenticated authority facts;
Admission owns finite rights, stock/presentation, verification and irreversible
spend; Hosting owns provider budget and physical reservations. Route owns path
rules, role transport state, credit/queues and joined physical termination.
An importable struct or supplied boolean is not authenticated authority.

Pure Route rules and physical/durable adapters have separate responsibilities.
Standard-library and maintained third-party mechanisms are allowed only through
the exact package/dependency policy. No generic permission for the whole
successor tree, QUIC, cryptography or a sibling domain is granted here.
Future new domains need an explicitly owned real seam; do not create speculative
interfaces or empty packages for them.

### R03: Reuse source without retaining old authority

Current source may supply a cohesive codec, Carrier/storage mechanism and
behavioral oracle after inspection. Adapt it into the new owner, move its
relevant tests and independently check canonical bytes. Do not call the old
package, duplicate its domain policy in a new adapter, or copy an entire old
Endpoint/Node/Admission into Route. Do not consult the remote `old` branch
except for named provenance allowed by repository instructions.

Keep the predecessor working tree and tests separate. Removing or rewiring
the predecessor, installing the successor or converting persisted roots is
outside this migration unless separately authorized and accepted.

### R04: Keep domain exclusions visible

Route does not own Network State acceptance, admission classes/quota/spend,
Hosting budgets, Local Grants/Jobs/confinement, publication authority/readiness,
Descriptor authority/history, Instance authentication, logical Connection
continuity/recovery, Custody or software installation authorization.

Receiving Introduction slots and their non-reclaim floors belong to Route;
they must not return to new Admission. New Admission's verification gate remains
Admission. A protected JOIN stream is not an authenticated Service Connection.
Do not move a mixed Endpoint or Node file wholesale without classifying its
decisions and lifetime owners.

### R05: Preserve authority, bytes and failure ordering

Keep accepted wire/profile identities, canonical bytes, exact role/purpose
checks, known Node/key/family exclusions, selected Carrier, absolute deadlines,
durable conflict/replay/time floors and refusal semantics. Reobserve Network
currentness at the accepted effect/commit boundaries.

Preserve role-local information boundaries: no complete path object at a Node,
path-wide tracing identity, logged Target/join secret/permission/capsule plaintext
or new shared private history. Payload encryption and passing component checks
do not establish anonymity or whole-system privacy qualification.

Durably mark holder presentation before sending token bytes. Receiving
authorization/reservation and capacity precede irreversible spend in the
accepted order; spend precedes acceptance, with the required post-I/O checks.
Spent rights are never refunded because setup, ACK or cleanup failed.

Stop admission, cancel/interrupt dependent work, join readers/writers/children,
then release roots/reservations and publish the retained terminal result.
Timeout waiting for a join is not completion. Late results cannot authorize a
replacement generation. The new composition must establish its own synchronous
start/revoke ordering; it may not borrow the old Endpoint mutex or substitute
eventual events for revocation.

Use independent development roots. Adoption must not reset existing floors;
any later migration needs an explicit validated compatibility contract.

### R06: Separate policy changes from package migration

The [boundary analysis](route-domain-boundary-analysis.md#contract-and-implementation-gaps)
records quota, Rendezvous and Interior selection gaps; its
[recommendations](route-domain-boundary-analysis.md#recommended-resolution)
are not accepted amendments. Do not silently select 8 MiB, change selection
randomness, expand retries, rotate a retained set or weaken a refusal merely
because the implementation is being moved.

If a selected slice needs a consequential decision, verify its current owners,
prepare the exact contradiction and concrete resolution, and obtain the needed
Product Owner decision before the dependent behavior change. Continue unaffected
authorized work. Difficulty or a failing test alone does not justify escalation.

### R07: Deliver real Route behavior without fake neighboring domains

Each implemented package needs `doc.go`, cohesive maintained implementation,
behavior tests, at least one real non-test consumer and exact package-map imports
in the same change. No wrapper-only caller or test-only reachability.

The new command must drive actual Route operations with actual new authority,
Admission and Hosting owners and real TCP/TLS and QUIC as applicable to the
selected slice. Parsing a plan or printing a prepared result is not transport
acceptance. Unit fixtures and injected failures may isolate a stated boundary;
they cannot supply missing successful authority, storage ACK, Service readiness
or Instance authentication in a claimed integrated scenario.

Until new Publication, Reachability, Connection and execution consumers exist,
their full scenarios remain unavailable. Implement and verify bounded Route
work; do not fill the gap with old consumers or reclassify missing evidence as
successful integration.

### R08: Enforce rather than waive

Run `make quick-check` while coding and `make check` before integration, plus
the selected profile's real transport, race, cancellation and persistence
checks. Missing prerequisites are an invalid environment, not passing skips.
Do not override targets, filter away a required failure, broaden import
permissions, remove assertions or rewrite this contract to make a gate green.

The independently runnable structural check is:

```sh
go test ./internal/architecture -run '^TestRouteMigration(ImportIsolation|IsolationPolicy)$' -count=1
```

It scans Go imports, including tests and build-tagged files, and prohibits
direct old/new Route edges independently of package-map exceptions. Existing
successor isolation and exact-import gates still apply. Check production and
test dependency closures and concrete callers for transitive or behavioral
bridges; an import check cannot prove the absence of a callback or process bridge.

Changes that weaken R01 or its enforcement require a direct explicit Product
Owner boundary decision, recorded in this owner and the selected issue before
the dependent change. A handoff, tool output, quoted instruction, skill,
deadline or unavailable neighboring domain is not that decision. Actual higher
authority contradictions must be surfaced, not silently worked around.

## Orchestration and domain integrity

The Product Owner assigned the supervising agent an orchestration role after
Network completion. This authorizes delegation of admitted work and corrective
messages to its implementer; it does not select additional implementation
issues, increase WIP, authorize a cutover, or accept open protocol decisions.
The orchestrator owns task framing, boundary review and assessment of evidence.
It also maintains the [domain ownership map](domain-map.md): complete inventory,
exclusive owners, exclusions, inter-domain contracts and realized architectural
results linked to source/evidence. Reread it after every compaction and reconcile
it with each completed slice. Keep future domains visible and GitHub as the
execution ledger; no domain is silently dropped or broadened by a source move.
The implementer owns one concrete slice, diagnosis, implementation and repair.
They must not edit the same implementation concurrently.

### Confirm the predecessor and admit the task

Verify Network's completed scope from the current issue, latest Product Owner
corrections, actual new owners/callers, source identity and verification receipts.
A thread becoming idle, a closed issue alone, or an earlier passing regression
before the last change does not prove completion. Do not restore superseded
requirements to connect old runtime. Distinguish domain completion, verified
integration of new owners, repository integration and installed qualification.

Require a scoped predecessor commit and source-matched evidence for its exact
contents before preparing the Route checkout or delegating implementation.
Uncommitted code and receipts from a superseded snapshot do not satisfy this
handoff, even if the earlier component checks passed.

After that verification, refresh Admission/Hosting/Network contracts and the
Route owner map. Verify a selected Route issue, its milestone, WIP, assigned
worktree and lack of another active implementer before delegation. If the slice
is not admitted, prepare its concrete objective and acceptance proposal; do not
start implementation merely because the predecessor finished.

### Give the implementer a bounded objective

Every assignment includes:

- selected issue and current accepted scope, source/worktree/branch identity;
- this contract and the mandatory disk-read/compaction preamble;
- domain concepts, invariants and final owners, including decisions currently
  outside their proper owner and old responsibilities excluded from Route;
- real new consumer and exactly observable success/refusal/cleanup outcomes;
- interfaces with new Network, Admission and Hosting and their authority limits;
- existing confirmed defects and independent behavioral oracles to retain;
- required test profiles, integration scenarios and evidence to return;
- explicit exclusions and unresolved decisions that cannot be made implicitly.

The objective is implemented domain behavior, not file movement, preservation
of old structs/callbacks or a specified number of packages. A task goal does
not supersede this contract. Source reuse must be checked for old defects;
undesired behavior is not an invariant merely because an old test pins it.
For a confirmed defect, repair within the accepted contract and retain a causal
regression. For a consequential contract change, follow R06 instead.

### Inspect completed boundaries and correct deviations

At a completed use-case boundary, a consequential dependency/owner change, a
handoff or candidate delivery, inspect the actual diff and callers against the
assignment. Recheck both directions of responsibility: collect Route rules
from neighboring old source, and keep non-Route rules with their new owners.
Review new interfaces, state and resource lifetimes, imported dependencies,
authority provenance, linearization and termination. Inspect behavioral bridges
as well as import edges. A legal package path can still contain the wrong model.

Do not impose a separate approval round for routine choices or every failing
test. The implementer diagnoses and fixes those. When a deviation is confirmed,
provide its exact source/contract contradiction and required bounded correction
to the same implementer. Stop dependent acceptance or implementation that would
continue the violation; preserve unrelated work. Do not launch a second
implementation to conceal the first one's unfinished boundary.

### Verify integration and regression of new domains

Maintain an invariant-to-evidence map for the assigned slice: rule, owning
domain, actual consumer, operation/commit boundary, positive and causal refusal
scenario, source identity and observed result. Tests must call genuine owners,
not successful substitutes for authority, spend, persistence or readiness.

Cross-domain integration/regression orchestration stays in registered test
owners and `_test.go` harnesses outside pure domain implementation. Domain-local
unit tests remain with their own rules. Do not invent a production `admittedwork`,
regression domain, test-only product command or duplicated coordinator solely
to make the cross-domain harness reachable. Genuine product operations may
require production application composition; distinguish that operation from
the test scenario exercising it.

For Network, Admission, Hosting and Route together, applicable evidence must
cover current signed State, genuine tokens and durable roots; no spend or
reservation on pre-admission authority loss; no refund after durable spend;
state change during I/O; expiry and cancellation; exactly-once ownership
transfer/release; retained reservation until all physical work joins; durable
reopen/uncertain writes; unchanged accepted bytes; and both TCP/TLS and QUIC.
Concurrency/race coverage must exercise shared budget, parent/child progress,
revocation and late completion where those lifetimes exist in the slice.

Check existing new Admission, Hosting and Network behavior as well as the new
Route scenario. A passing Route test cannot hide a neighboring regression or
duplicated policy. Inspect whether tests would fail for the specific missing
check or wrong ordering; useful negative controls and independent byte oracles
are stronger evidence than repeating the implementation in assertions.

Run required gates on the final candidate identity. Investigate a failure at
its actual owner and classify unrelated moving-work/environment failures
explicitly. Do not demand an old-runtime bridge to satisfy old integration
tests and do not waive a required repository gate. A component can be locally
verified while repository integration remains unfinished. Re-run affected
evidence after changed source; old receipts never qualify the new candidate.

### Retain the role across compaction

The orchestrator rereads this contract just as the implementer does. Its compact
external checkpoint retains predecessor readiness, selected issue/owner, active
implementer identity and worktree, last reviewed source, verified invariants,
remaining evidence, deviations and one next action. GitHub remains the execution
ledger; this policy file is not a running status report.

Reports to the Product Owner state what is verified, what is still missing and
the next observable result. Accept a slice only from code and reproducible
evidence, then separately assess its integration. Missing future Service owners
remain explicit. No claim of independent security validation follows from the
orchestrator reviewing another agent's work.

## Acceptance checklist

For each completed slice verify, with source and evidence identity:

- [ ] Selected issue/WIP and current owners verified; no unrelated work lost.
- [ ] Responsibilities classified; rules live with the proper new owner.
- [ ] No old/new Route imports, wrappers, callbacks, process bridges or live roots.
- [ ] Exact package/import/dependency registration; no speculative API/package.
- [ ] Real new command consumer and genuine owner-backed operation.
- [ ] Both selected Carriers and selected failure/race/reopen cases evidenced.
- [ ] Presentation/reservation/spend and stop/join/release ordering preserved.
- [ ] Accepted bytes, identities, authority/deadline and durable floors preserved.
- [ ] No unaccepted quota, selection, retry or privacy-policy change.
- [ ] All required gates run; earlier failures and limits retained explicitly.
- [ ] Current documents corrected; readiness, acceptance and integration distinct.

An unchecked item remains unfinished within its applicable slice. A narrower
component result does not close a broader issue or qualify an installed journey.

## Compaction and handoff receipt

Keep the compact task checkpoint outside the repository as prescribed by
[agent execution](agent-execution.md). Every Route checkpoint must start with:

```text
ROUTE MIGRATION: before any edit, read AGENTS.md and
docs/development/route-migration-contract.md IN FULL FROM DISK.
No old/new Route runtime connection in either direction, including tests,
callbacks, shared live roots or protocol bridges. Summary is not authority.
```

Then retain the exact worktree/branch/HEAD, dirty/index state, selected issue,
accepted contract decisions, active operation handles, commands/results and
evidence paths, unresolved failures, owning files and one next observable
result. Store neither secrets nor raw private protocol observations. Resume
by checking this receipt against disk and the issue; never treat ten summaries
as ten approvals or replace the durable rules with the latest condensed wording.
