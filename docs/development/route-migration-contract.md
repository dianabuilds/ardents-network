# Route migration contract

Status: **mandatory engineering contract** for the isolated Route migration.
The Product Owner requested these persistent rules on 2026-10-04. This document
authorizes no implementation issue, package, dependency, deployment or product
contract change. The root [repository instructions](../../AGENTS.md) require
reading it after every compaction and handoff. Its rules survive a new chat,
worktree, agent, checkpoint or source-file rename.

## Current selection takes precedence

The Product Owner directly changed the nearest closed release to TCP/TLS alone
on 2026-10-19, recorded by [ADR-0122](../adr/0122-select-tcp-only-closed-release.md).
For this release only, references below to both selected Carriers mean positive
TCP/TLS coverage plus explicit pre-effect QUIC refusal and no fallback. This is
a product-scope amendment, not a waiver of a failing gate. Preserve R01–R08,
all other Route behavior, authority/bytes/lifetime rules and mandatory checks.
Historical both-Carrier receipts remain evidence of their original artifacts.

The [execution owner's current-selection rule](agent-execution.md#current-selection-takes-precedence)
and [#497](https://github.com/dianabuilds/ardents-network/issues/497) govern work
admission. This contract cannot restore an earlier automatic next-domain
assignment; a committed prerequisite or documentation correction selects no
implementation.

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
5. Read the [domain ownership map](domain-map.md). Verify affected owners and
   callers against current source. Archived predecessor inventories are evidence,
   not an integration destination or permission to import old runtime.
6. Resume from verified evidence. An earlier recommendation, summary, passing
   narrower test or report of completion is not a new authorization.

Report the concrete slice and next result concisely. Routine implementation
choices within the accepted scope do not require repeated permission.

## Workspace and sequential development

The Product Owner explicitly directed this migration to continue in the existing
project checkout on 2026-10-04: development is required; another worktree is not.
Use `C:/Users/vitek/code/ardents-network`, preserve its existing branch and mixed
work, and keep one implementation owner. Do not create or require another Route
worktree. A needed Network component and its dependent Route operation form
a sequential chain: commit and verify that exact prerequisite before selected
dependent implementation in this checkout. They cannot be parallel
implementations. Component readiness does not require full acceptance of every
Network responsibility. #497 owns full-domain transition gates. A committed
prerequisite is a source requirement, not permission to start another domain or
create another checkout.

This is not a general ban on worktrees. Separately selected, genuinely independent
tasks may use branches or worktrees under the repository's existing ownership,
WIP and workspace rules. Available WIP capacity does not make dependent work
independent. The conditional instructions below apply to an authorized change
of execution arrangement, not to routine continuation of this domain chain.

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

New Route belongs in the new-domain tree at `internal/successor/route`.
Its development composition consumer belongs in `cmd/ardents-next`. On
2026-10-27 the Product Owner directly authorized the narrow canonical installed
Forwarding exception, recorded in [#542](https://github.com/dianabuilds/ardents-network/issues/542#issuecomment-6436276753):
only `cmd/ardents-node/forwarding_installed_linux.go` may directly compose new
Installation/Network/Node owners and `internal/successor/route/role` for this
installed invocation. Its exact five permitted imports are
`internal/successor/installation/forwarding`, `internal/successor/network`,
`internal/successor/network/state`, `internal/successor/node` and
`internal/successor/route/role`. No other old-command or test consumer, role,
wrapper, callback, process bridge or shared live root is admitted. All other
R01 restrictions remain. Bootstrap acceptance remains separately required by
ADR-0123 Decision 6. Exact packages require normal complete registration.

New Route must not import or execute old `internal/route`, `internal/entry`,
`internal/endpoint`, `internal/node`, `internal/admission`, `internal/hosting`,
`internal/service` or another old runtime owner. Old packages, commands and
tests must not consume new Route, apart from the exact canonical installed
Forwarding file explicitly admitted above. The prohibition includes subpackages,
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

The [private admission contract](../technical/private-admission.md),
[protected Route protocol](../technical/protected-route-protocol.md) and
[domain map](domain-map.md) own quota and selection rules. Historical boundary
recommendations are not accepted amendments. Do not silently select 8 MiB, change selection
randomness, expand retries, rotate a retained set or weaken a refusal merely
because the implementation is being moved.

If a selected slice exposes a consequential gap, the orchestrator verifies its
current owners and resolves the design from accepted contracts, named evidence
and independent oracles before assigning dependent behavior. Record the exact
contradiction, resolution and affected obligations with their owning design.
Do not ask the Product Owner to perform this delegated design work, stop the
development thread, or silently turn a migration into an unrelated product
policy change. Preserve accepted protection and safety bounds, repair defects
and continue useful authorized work. Difficulty or a failing test alone is not
a reason to return the engineering assignment to the human.

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

[Agent execution](agent-execution.md#delegated-project-wide-domain-transition)
owns the common design, visible ownership, correction, evidence, scoped Git
delivery and handoff workflow. Verify the current selected executor and
arrangement in #497 and the selected child issue; historical Route assignments
are not a second live ledger. The [domain map](domain-map.md) owns exclusive
responsibilities and collaborations. Reread it after compaction and reconcile
both neighbors of any accepted ownership transfer without dropping future
domains. Execution arrangement changes no rule in R01–R08.

### Confirm the predecessor and admit the task

Verify the exact needed Network/Admission/Hosting components from their current
owners, actual callers, scoped commits and source-matched receipts. Idle, issue
closure or a dirty passing snapshot alone is insufficient. Component readiness
is distinct from full domain acceptance, repository integration and installed
qualification. Do not restore old-runtime integration requirements.

Refresh those contracts and the Route owner map. Before dependent implementation
verify the selected issue, milestone/WIP, current human selection, existing
checkout, visible assigned executor and exclusive ownership. A finished
predecessor cannot replace design, a real acceptance boundary or permission
under the current hold. Follow #497's exact full-domain handoff conditions.

### Give the implementer a bounded objective

Every selected Route assignment includes this contract and its mandatory
disk-read preamble; source/worktree/branch identity; the verified Route question,
invariants, final owners and exclusions; a real new consumer with observable
success/refusal/cleanup; narrow Network/Admission/Hosting seams; confirmed
defects and independent byte/behavioral oracles; selected test profiles and
integration evidence; and unresolved decisions that cannot be made implicitly.

The objective is implemented domain behavior, not file movement, old structs,
callbacks or package counts. Check reused source for defects. Repair confirmed
defects within the accepted contract with causal regression; use R06 for a
consequential contract change. Neither a goal nor a passing test supersedes
this contract.

### Inspect completed boundaries and verify integration

At a completed use-case boundary, consequential owner/dependency change, handoff
or delivery, inspect actual code and callers in both directions. Route rules
belong to Route; neighboring authority, state and termination stay with their
named owners. Inspect behavioral/process bridges as well as imports, exact
authority provenance, linearization and resource lifetimes. A legal package
path can still contain the wrong owner.

Cross-domain regression composition belongs in registered test owners and
`_test.go` harnesses outside pure implementation. Do not invent a production
regression domain, test-only command or duplicated coordinator merely to make
a harness reachable. Genuine product composition and the scenario exercising
it have different responsibilities.

For Network, Admission, Hosting and Route together, applicable evidence covers
signed current State, genuine tokens and durable roots; no spend/reservation
on pre-admission authority loss; no refund after durable spend; state change
during I/O; expiry/cancellation; exactly-once ownership transfer/release;
retained reservations until physical join; durable reopen/uncertain writes;
accepted canonical bytes; and both TCP/TLS and QUIC. Race checks exercise shared
budget, parent/child progress, revocation and late completion where applicable.
Check neighboring behavior too; a Route pass cannot hide a regression or
duplicated policy. Use causal negative controls and independent byte oracles.

Run the required gates on the final source identity. Classify unrelated
moving-work/environment failures explicitly without waiving a gate or adding
an old-runtime bridge. Changed source invalidates affected receipts. Missing
genuine Service consumers and installed/privacy qualification remain explicit.

## Acceptance checklist

This checklist owns Route engineering obligations for a selected slice;
#497 and the selected child issue own full-domain acceptance and transition.
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
