# Repository reconstruction study

Status: **completed architecture analysis; proposed design awaiting owner decisions**. This is a factual source map and design
workspace, not a second product specification, a C0 issue ledger, or the formal
frozen-candidate [deep audit campaign](deep-audit.md). Work is based in the
architecture-refactor worktree; other active network/Endpoint implementation
branches remain outside this study's ownership until integration.

At the 2026-09-26 integration check, the main checkout's `6134408f` is an
ancestor of this worktree's `53f02e64` (114 commits ahead here, zero commits
unique to that main HEAD). This establishes a fast-forward relationship for
those committed revisions, **not** readiness to copy or merge the working
trees: main has separate uncommitted documentation/AGENTS changes, this
worktree has uncommitted architecture changes, and several network/Endpoint
issue worktrees are separate. Preserve those changes and compare each
completed issue delta against the owner map before a combined candidate.

## Question and completion standard

If Ardents were composed today solely from the selected C0 outcomes and
accepted safety/compatibility obligations, which Modules, Interfaces, process
boundaries, durable roots, and tests would be necessary? Where does the current
implementation put each responsibility, and what must move, deepen, be replaced,
or be retired to reach that shape without changing accepted behavior?

The study is complete only when:

1. every tracked path is inventoried; each production Go file has a semantic
   responsibility, with callers, mutable state, resources and tests traced
   where a move, replacement or retirement depends on them;
2. each current product, security, technical, operational, reference, and
   engineering fact has one document owner, with ADR/research provenance and
   supersession conflicts identified;
3. every maintained command route has its dispatch, first effect and owner
   mapped; selected C0 behaviors have observed normal, refusal,
   interruption, restart, resource and terminal-outcome paths mapped to
   implementation and evidence. Missing paths are explicit implementation
   or qualification gaps, never presented as successful runs;
4. the existing and target package/import graphs, process trust boundaries,
   durable-root ownership, and stop/join order are explicit;
5. the target design names the necessary Modules and their small Interfaces,
   shows the Publisher/Reader journey end to end, and states which existing
   code is retained, deepened, replaced, or retired;
6. a dependency-ordered transition plan identifies independently reviewable
   slices, compatibility preservation, integration checkpoints with the other
   active task, and the exact verification required for the combined installed
   Ubuntu scenario. Implementation and C0 qualification follow this study.

## Source baseline and coverage

Initial tracked baseline: `50026274` on `codex/architecture-refactor`, with
concurrent uncommitted Endpoint edits. The initial Git inventory contains 1,933
tracked files: 848 production Go files under `cmd` and `internal`, 684 Go test
files, 38 Go fixture commands under `tests`, seven Go developer tools under
`scripts`, 201 tracked files in `docs/`, and other assets. At `9835e225`, nine
tracked working-analysis files have been added under `docs/development`; the
tracked total at that revision was 1,942, with unchanged Go counts. ADR-0091
and its four completed retirement commits moved the worktree to `8cd1575f`:
44 tracked paths were deleted and one ADR added. The adjacent task then
committed ADR-0092 and the generic Publisher/Transit deletion at `53f02e64`.
The current committed inventory has 1,861 tracked paths, including 815
production Go files under `cmd`/`internal`, 659 Go tests, 212 `docs/` paths,
and 30 Markdown files elsewhere. The focused Node/Route/Endpoint/Service
production set is 360 files; `internal/endpoint/transit` is gone. These are
inventory counts, not reviewed or accepted architecture coverage. The maps
below have been reconciled to this commit without modifying the adjacent
agent's implementation.

Earlier behavior traces name the commit inspected when they were written.
Package/file membership below is reconciled to `53f02e64`. An offline
`go list -mod=readonly` projection with explicit Linux and Windows `GOOS`
found 55 packages on each platform. Their first-party import union has 133
edges; the retired Endpoint Transit package and its imports are absent.

| Map | Baseline coverage and reviewed overlay | Limit |
| --- | --- | --- |
| [Repository file map](repository-file-map.csv) | All 1,861 paths committed at `53f02e64` plus the two untracked map artifacts created by this study, physical group, artifact class, and review state; all 815 `cmd`/`internal` production files have a provisional responsibility; 257 have a direct behavior source trace and 68 have a caller-only source trace; 51 Go tests have a direct behavior trace | The other assignments are inventory coverage, not proof of necessity: caller, mutable-state, resource, compatibility, and test evidence still need review for proposed moves and retirement. |
| [Package import graph](repository-package-graph.csv) | All 55 `cmd`/`internal` Go packages at `53f02e64`, their production/test file counts, and 133 first-party edges in the Linux/Windows union | Import edges show compile dependencies, not ownership or runtime call direction. |
| [Package disposition map](repository-package-disposition.csv) | Exactly the same 55 packages, each classified by its relationship to the selected C0 execution, provisioning, verification, operator contract, or retained uncomposed code | The treatment is a review direction, not an authorization to delete a retained package or a substitute for its exact caller, root and ADR decision. |
| [Command route map](repository-command-route-map.csv) | All 69 dispatchable/current, compatibility, refusal, internal and qualification routes in the seven maintained binaries at `53f02e64`; each points to an existing function and primary domain owner, with dispatch and first effect traced | `source-traced` does not claim a complete refusal/restart matrix or installed acceptance. Plan-schema refusals are within their route, not separate commands. |
| [Documentation map](repository-documentation-map.csv) | All 212 `docs/` paths and 30 Markdown files elsewhere committed at `53f02e64`, plus the two worktree-only CSV maps; title or purpose, status, role, and first authority classification | Current fact ownership, inbound links, duplicated claims, and correspondence to code still require review. |
| [C0 component inventory](c0-component-inventory.csv) | All 360 production Go files in Node, Route, Endpoint, and Service at `53f02e64`; ADR-0092 deletion rows are gone. All 99 decision-sensitive rows have direct behavior or caller/local closure traces | Proposed treatment still needs an accepted owner decision before a move or retirement; five Endpoint rows need recheck after adjacent implementation integration. |
| [Behavior trace map](repository-behavior-map.md) | Selected C0 and maintained command journeys with contract, entrypoint, authority, normal/refusal/restart and resource traces; all 69 command routes have dispatch and first-effect source traces | Explicitly named combined failure, timeout and installed-evidence gaps remain; a source trace is not an installed qualification verdict. |
| [C0 Module composition](c0-component-reconstruction.md) | Fourteen necessary ownership components, observed small handoffs, Publisher/Reader journey, target import direction, initial exact package placements and dependency-ordered transition | Carrier/client extraction, Credential adapter split, accepted old-root policy, Endpoint integration and installed operator handoff require bounded decisions and implementation evidence. |
| [Evidence-backed findings](repository-reconstruction-findings.md) | Confirmed uncomposed code, duplication, document drift, platform graph, authority gaps and resource-lifetime findings with source-specific actions | A finding is evidence for a decision; it does not itself supersede an accepted contract or qualify C0. |

**Map reconciliation at `53f02e64` plus the working overlay.** The file-map
path set contains all 1,861 committed tree paths with no missing row and
exactly two additional untracked CSV maps: command routes and package
disposition. The documentation map has 212 committed `docs/` paths, 30
Markdown paths elsewhere, and those two working CSV maps. The 360 C0 inventory rows
represent current production Go files under Node, Route, Endpoint and Service.
The package graph has 55 physical packages and its production/package-test
counts sum to 815/598; the remaining 61 Go tests live outside `cmd`/`internal`.
The package disposition map contains each of the 55 graph packages exactly
once, with no missing or extra package. The command route map's 69 source
locations and named functions all resolve in this tree. Recheck these sets if
worktree HEAD changes.

A CSV completeness check at this revision found 815 production Go rows with
nonempty physical owner, proposed responsibility, and audit state; all 360
focused C0 rows have a proposed component and source-specific reason; all 55
package-disposition rows have a treatment and decision boundary. This proves
the maps have no empty classification fields. It does not prove that every
provisional responsibility is the correct future owner: the 257 behavior
traces, 68 caller traces, 159 first-source passes, 68 command passes, and 263
package-contract passes carry different levels of evidence.

The file map now distinguishes **document indexing** from behavior review:
all 41 `current-document` rows have a path in the documentation map and a
nonempty authority review (40 `document-owner-pass`, one stronger
`owner-route-pass`). All 88 ADR and 66 research-record paths have mapped
status and authority treatment; their declared status was rechecked against
the first 10 or 18 lines of each source with zero mismatches. The 152 records
previously marked `unreviewed` are therefore `provenance-status-pass`; two
ADRs already had stronger accepted-decision review. Neither state means that
every claim, inbound link or code correspondence in those documents has been
audited. Their exact owner and supersession checks remain part of this study.

`caller-source-traced` in the file map means that current non-test callers and
the local helper closure were checked against source, but the file's accepted
historical or persisted-data disposition is still open. It does not claim a
normal/refusal/restart behavior trace or authorize deletion. The twelve
remaining Route/Service retirement candidates with that state are distinct
from the fully traced mixed `route/wire_encoding.go` (F-52).
F-66 records the AAI2 package-comment drift found against the maintained owner
and tree. Its correction is now at `package-contract-pass`, not a behavior trace.
`test-behavior-traced` means the named test file was read against one specific
source behavior, with both its proven oracle and missing case recorded in the
behavior/findings map. It does not claim that every test in that file was
audited or that the installed scenario passed. Four Service Connection/Endpoint
test files have this status for F-23; three Route tests have it for the live
Carrier fixture dependency in F-52.

**Parallel-branch reconciliation snapshot, 2026-09-26 (local refs only).**
`origin/dev` is an ancestor of this worktree by 104 commits, and
`origin/codex/architecture-refactor` is an ancestor by twelve commits. The
completed ADR-0092 deletion at `53f02e64` is therefore already in this
analysis baseline. The still-separate local refs below illustrate the next
integration surface; their diffs are measured from each merge base, not
assumed to be final PR or acceptance state. `codex/issue-277-publisher-drain`
currently points at the shared `c10d59dc` base and contributes no branch-only
commit:

| Separate ref | Branch-only commits relative to this worktree | Changed paths from merge base | Overlap requiring fresh trace after integration |
| --- | ---: | ---: | --- |
| `codex/issue-253-role-stop-capture` | 2 | 5, including two production `resource` files | Cgroup owner/process sampling and the Node resource-failure interpretation. |
| `codex/issue-276-cgroup-slice` | 2 | 6, including one Endpoint production file and two system-unit files | Installed worker cgroup properties, system-unit authority and F-27/F-67. |
| `codex/issue-277-installed-worker-drain` | 2 | 3, including one Endpoint production file | Worker cleanup/late drain and its installed lifetime owner. |
| `codex/issue-279-permission-oracle` | 4 | 9, all tests, test-profile wiring and documentation | Installed Permission-expiry boundary evidence. Its new profile does not add the two missing fixture packages to `make check` (F-70). |
| `codex/issue-281-initial-publication-cancel` | 1 | 5 test files | Endpoint initial publication cancellation, Source set and Route close-boundary assertions. |
| `codex/issue-282-quic-snapshot` | 1 | 1 Endpoint test file | Installed QUIC Publisher snapshot evidence. |
| `codex/issue-285-route-opening-diagnostic` | 21 | 57, including 27 production `cmd`/`internal` files | Endpoint Source/Context, Route Carrier/prefix, Node opening/lifecycle, resource sampling, and diagnostic event output. |

Neither divergent ref is merged into `53f02e64`; overlapping findings are
current only for this worktree. Rebuild the file/package maps and recheck
caller, ownership and close-result traces after a completed integration
candidate is selected. Do not copy isolated fixes into this study or count
green checks on the divergent refs as combined C0 acceptance.

The test-infrastructure pass found a real framework rather than an absent
base: 21 registered execution profiles, explicit deterministic and process
package inventories, four assigned process-suite roots, and architecture
checks for completeness and Make wiring. Its important verdict boundary is
that `make check` does not run the dedicated installed text-command/worker
profiles (F-31). A green ordinary gate and a successful installed C0 journey
must be recorded separately on the same integration candidate. F-16 identifies
five exact fixture copies; the independent State oracle is not part of that
deduplication. The separate `ownership.json` gate assigns broad work owners,
not behavioral test ownership: 598 Go tests are colocated under `cmd`/`internal`
and 61 live in external test trees. For each proposed move or retirement, map
its exact tests and checked profile before treating that evidence as portable
to a new boundary (F-31).
F-69 identifies two additional byte-identical frozen-vector corpora (17
redundant paths) whose command/replacement consumers can share canonical
inputs while retaining independent assertions and checked-profile evidence.
The profile-to-file crosswalk covers 598 deterministic and 58 process test
files; the other three test files are a portable canonical Network fixture
test and two Linux-only qualification-fixture command tests. They are selected
by targeted PR ownership rules when relevant paths change, but are absent
from `make check`'s full package lists (F-70). This is the current test-profile
coverage gap, not a claim that 609 otherwise unreviewed test bodies have
received behavior review.

The [package map](package-map.md), [command surface](command-surface.md), and
[documentation policy](documentation.md) are existing factual owners. This
study cross-checks them against code; it does not silently promote a research
record, proposed ADR, experiment, or historical compatibility artifact into a
current requirement. The `old` branch is outside the design input.

### Current fact owners for the selected C0 reading route

The documentation CSV classifies every path, including historical evidence.
For a current C0 question, use the following owner before following an ADR or
research link. A row identifies the fact to maintain in that document; it does
not turn a selected design or a test profile into an implemented result.

| Fact to locate or change | Current owner | Boundary against nearby documents |
| --- | --- | --- |
| Applicable C0 product and installation outcome | [Product scope](../product/scope.md) | The [functional map](../product/functional-map.md) owns requirement IDs and budgets; [protected workload](../product/protected-service-workload.md) owns exact text-Service user behavior. |
| Observable user journeys and operating thresholds | [Journeys](../product/journeys.md) and [operating model](../product/operating-model.md), respectively | The long-term [vision](../product/vision.md) and [future catalog](../product/future-product-catalog.md) do not add C0 acceptance paths. |
| Adversary, protected information and claim condition | [Threat model](../security/threat-model.md) | A green test or protocol description does not widen the claim. |
| Domain meaning of Person, Service, Credential and Capability | [Glossary](../../CONTEXT.md) | Package or stage labels do not create product terms. |
| Selected whole-system privacy composition | [Common privacy architecture](../technical/common-privacy-architecture.md) | This page routes to owners; detailed wire bytes and numeric limits stay with the linked contracts. |
| Protected Route grammar, Carrier identity and channel binding | [Protected forwarding protocol](../technical/protected-route-protocol.md) | [Network/Route/Node](../technical/network-route-node.md) owns currently implemented State, Entry and Node operations and retained old grammar/refusal, not a second accepting Route. |
| Issuance, permission, spend and retry authority | [Private admission](../technical/private-admission.md) | Credential/Replay code and local journals implement separate floors; Route's wire owner does not select entitlement. |
| Private Descriptor, Target proof and Store conflict floor | [Private reachability](../technical/private-reachability.md) | The protected protocol owns its transport framing; this owner keeps stored old-Target disposition explicit. |
| Endpoint, Broker, Service and logical Connection lifetime | [Endpoint/Service runtime](../technical/endpoint-service-runtime.md) | [Application confinement](../technical/application-confinement.md) owns systemd, worker and process isolation, including installed verification. |
| First-artifact inventory verification | [Enrollment verification](../technical/enrollment-verification.md) | [Release/Update/Custody](../technical/release-update-custody.md) owns release-floor decision, replacement and Authority material; enrollment grants none of them. |
| Canonical Name and retained Namespace/Resolution behavior | [Naming](../technical/naming.md) | `name resolve/control` refuse; the uncomposed Resolution transport is not the selected Service Name journey (F-51). |
| Project alpha transition and non-authorizing inspection | [Alpha control transition](../technical/alpha-control-transition.md) | Catalog, Release and State keep distinct durable floors. |
| Operator syntax and result | [Command reference](../reference/commands.md) | [Command surface](command-surface.md) owns the engineering inventory; package tests are evidence, not operator instructions. |
| Current Ubuntu package steps and Portable enrollment instructions | [Ubuntu package procedure](../../packaging/ubuntu-deb/README.md) and [Portable reference](../reference/portable-enrollment.md), respectively | Neither supplies the protected system-unit/plan producer required by the selected C0 journey (F-27/F-67). |
| Checked profiles and what a green gate proves | [Testing](testing.md) | [Privacy qualification](privacy-qualification.md) owns the selected protected acceptance design; each `tests/qualification/*/README.md` owns its runnable profile and exact prerequisites. |
| Maintained packages and permitted imports | [Package map](package-map.md) | The reconstruction maps are working evidence and cannot create a package contract by themselves. |

The 1,545-line [network-core transition](network-core-transition.md), its wire
appendix and the Endpoint refactoring draft are proposals, not additional
current owners. F-18 records the transition document's duplicated execution
chronology. The [historical Transit Grant acquisition](../technical/transit-grant-acquisition.md)
is not the current Endpoint intake after ADR-0092. Preserve their named
evidence while promoting any unique selected fact to the owner above before
retiring the overlapping text. Line-by-line consolidation of that proposal
is a subsequent documentation change; this study identifies the duplicate
chronology and its canonical fact owners without granting the proposal C0
authority.

### Completion audit of this study

The architecture-study deliverables above are met at the recorded source
baseline with the two working CSV artifacts mapped separately. Only 257 of
815 production files have a direct behavior-source trace: that count is not a
claim that every file received line-by-line review. A direct or caller/local
closure trace was required where a proposed move, retirement, compatibility
decision or command behavior depends on it; a retained, cohesive file may
remain at package-contract review once its role and dependencies are verified.
In the focused 360-file Node/Route/Endpoint/Service
inventory, no row remains at the generic package-and-symbol reason level.
All 360 rows now state a source-specific responsibility or next question,
including the 32 `move-candidate`, 40 `boundary-review`, ten
`split-candidate`, 13 `retirement-review`, and 96 `deepen` rows. These
reasons do not approve a package move or prove behavior unchanged.
An exact join of the focused inventory to the file-map audit states finds 99
decision-sensitive rows (`move`, `boundary`, `split`, `retirement`, or
`compatibility`): 33 have a direct behavior trace and 66 have a caller/local
closure trace. No decision-sensitive row remains at first-source-pass. The
cluster traces identify present owners and narrow proposed seams; they do not
authorize the moves or prove behavior unchanged. Recheck the five Endpoint
rows after the adjacent task's completed implementation is integrated.
The remaining 13 retirement rows comprise twelve Route files and one
Service Publication file. ADR-0092 has already removed the thirteen Endpoint
rows and one Route profile row from this set; the [closure inventory](c0-component-reconstruction.md#former-v2-execution-closure-after-adr-0092)
records their former dependencies and mixed-file prerequisites.

| Completion requirement | Architecture-study evidence | Subsequent contract or implementation gate |
| --- | --- | --- |
| File and package disposition | 1,861 unique tracked paths at HEAD plus two mapped worktree artifacts, 815 production Go responsibilities, 55 observed packages with an explicit C0 relationship, and a 360-file focused owner inventory. All 99 decision-sensitive focused rows have a direct behavior or caller/local closure trace. | Approve and implement each proposed move, split, retirement or compatibility treatment in a bounded owner change; recheck affected callers and tests against the integrated revision. |
| Current documentation and authority | 244 unique document/data paths classified by location/status, including two working CSV maps; current C0 fact-owner route and ADR/research precedence mapped. Specific active-version conflicts were corrected in their owners; duplicated transition chronology and supersession are identified. | Consolidate the duplicated proposal text after any unique accepted fact is promoted; publish the missing installed operator procedure with its implementation. The map is not a second specification. |
| Actual behavior and evidence | All 69 maintained routes have dispatch/first-effect source traces. Selected enrollment, State, Node, Publisher, Reader, Reachability, local Application, refusal, restart and shutdown paths have source/resource traces; missing combined failure and installed evidence are named rather than counted as passing. | Repair the identified failure/timeout paths and run installed Publisher/Reader acceptance on one integrated candidate; a source map is not a qualification verdict. |
| Resource and dependency graph | Linux/Windows first-party graph and static command closures, five Node receiver lifetimes, local Endpoint handler join, JOIN-to-Service transfer, selected C0 durable roots and other retained root openers are mapped. Target import direction and first exact extraction candidates are stated. | Settle Invite, ACA1 and Namespace retention in their accepted owners; fix replacement physical-close propagation and Issuer late close; validate new Go package edges in buildable slices after adjacent changes integrate. |
| Target composition and transition | Fourteen necessary components, observed small handoffs, Publisher/Reader read-through, concrete candidate package placements and six dependency-ordered steps cover the selected journey and preserve one-version and root obligations. | Accept consequential contract decisions such as old pending AREP recovery and the installed artifact-to-system-unit producer; implement slices under the single C0 issue limit and qualify both Carriers. |

The full focused inventory has source-specific reasons and all 99
decision-sensitive rows have a behavior or caller/local closure trace. Further
whole-tree file enumeration would not change the proposed architecture. The
remaining items above belong to accepted contract decisions, bounded
implementation, integration and qualification. Reopen a source trace only
when one of those changes invalidates its recorded baseline.
Old v2 Route closure, Carrier, client-path,
Node forwarding/probe, Endpoint qualification, Service Connection and receiving
wire clusters, and two current-doc corrections have source-specific evidence;
the Endpoint implementation task remains the owner of its active code changes.

## Current fact owners and reading routes

The [document CSV](repository-documentation-map.csv) enumerates every tracked
document and declared status. The table below identifies the **fact to edit**
when an implementation changes. A link to an ADR or research record explains
the decision; it does not replace the owner of current behavior. The listed
owners may be accepted target designs with a narrower C0 subset selected by
scope, so status and applicability still need checking at the exact section.

| Fact or reader task | Current owner | Boundary |
| --- | --- | --- |
| Selected C0 surface and support claims | [Product scope](../product/scope.md) | Selects the C0 subset; public target documents do not add a C0 command. |
| Product requirement IDs, public lifecycle, and observable journeys | [Functional map](../product/functional-map.md), [operating model](../product/operating-model.md), [journeys](../product/journeys.md) | Requirement numbers, lifecycle thresholds, and journey narration have separate owners; [vision](../product/vision.md) and [future catalog](../product/future-product-catalog.md) describe longer-term direction. |
| Publisher/Reader text outcome | [Protected workload](../product/protected-service-workload.md) | Selected bounded design; installed qualification is still separate evidence. |
| Adversaries and protection claims | [Threat model](../security/threat-model.md) | Accepted claim registry, not a passing security verdict. |
| State, Entry, Route, Node, and protected wire | [Network/Route/Node](../technical/network-route-node.md), [protected Route protocol](../technical/protected-route-protocol.md) | Current component boundary and selected closed wire contract; proposed network-core wire cannot override them. |
| Closed admission and private reachability | [Private admission](../technical/private-admission.md), [private reachability](../technical/private-reachability.md) | Current token/issuer and private Descriptor authority. The [Transit Grant acquisition](../technical/transit-grant-acquisition.md) document is historical ADR-0062/0092 provenance, outside the normal implementation route. |
| Endpoint, Service, Application confinement and common privacy | [Endpoint/Service runtime](../technical/endpoint-service-runtime.md), [Application confinement](../technical/application-confinement.md), [common privacy architecture](../technical/common-privacy-architecture.md) | Read the affected owner, not every Service section. |
| Release, Custody, enrollment, alpha control, and naming | [Release/update/custody](../technical/release-update-custody.md), [enrollment verification](../technical/enrollment-verification.md), [alpha control](../technical/alpha-control-transition.md), [naming](../technical/naming.md) | Each owns a distinct trust or compatibility transition. |
| Operator route and retirement procedure | [Command reference](../reference/commands.md), [Portable first-execution instruction](../reference/portable-enrollment.md), [Contributor profile](../reference/rendezvous-contributor.md) | Portable pre-execution verification has a direct route from the command reference; it does not qualify the separate protected text system-unit launch. |
| Engineering rules and factual maps | [Package map](package-map.md), [command surface](command-surface.md), [testing](testing.md), [dependencies](dependencies.md), [repository layout](repository-layout.md), [documentation policy](documentation.md), [risk exceptions](scoped-risk-exceptions.md), [agent execution](agent-execution.md), [deep audit](deep-audit.md) | [Development index](README.md) is a reader route; `ownership.json` is executable PR selection policy, not a prose contract. |
| Working designs and measurements | [Network-core proposal](network-core-transition.md) and [wire appendix](network-core-wire-proposal.md), [Endpoint design](endpoint-architecture-refactoring.md), [Node plan](node-architecture-refactoring.md), [Route boundary](route-refactoring-boundary.md), [privacy workstream](privacy-anonymity-map.md), [qualification design](privacy-qualification.md), [test-cost measurement](endpoint-test-cost.md) | Read for a named design/evidence question. Proposed mechanics and measured old runtimes are not current implementation truth; execution state belongs to Issues. The Node plan's former fixed forwarding/probe package sequence was reconciled to the later source map; those packages remain conditional. |

This routes the current-location documents without asking one contributor
to read all 42 top-level files. It is a responsibility index, not a duplicate specification;
the source document itself remains the canonical owner of its facts.

A source-link scan at `53f02e64` found inbound inline Markdown links for all
42 top-level product, security, technical and development documents. Five
missing local file targets were repaired with immutable commit links: two to
unmerged accepted ADR-0086, one to the separate R-157 draft, and two to
retired Entry source in historical R-144 (F-54). No missing local file target
remains in that syntax scan; anchors and external availability were not
checked. Cross-branch links do not integrate those decisions into this tree.

### Claim audit: installed protected Endpoint launch

This first cross-document pass checks the actual C0 launch claim against
current owners and operator-facing material at `9835e225`. The documents are
largely consistent about their *separate* routes; the missing item is their
supported composition, not a contradictory instruction to run both user and
system units as one Endpoint.

| Claim | Owning source and observed status | Code/evidence boundary |
| --- | --- | --- |
| One technical operator can verify an installed Ubuntu artifact and complete Publisher/Reader C0 without editing JSON or extracting fixture keys | [Product scope](../product/scope.md#c0-closed-alpha-readiness-profile): selected readiness criterion | The command accepts a v2 JSON plan; the installed text test constructs it from fixtures. No maintained protected-text plan producer was found in production command or packaging sources. Readiness remains unproven. |
| First execution needs an independent pin, then enrolled program/Release acceptance; a current-program restart uses its retained record | [Enrollment verification](../technical/enrollment-verification.md), [Release/update/custody](../technical/release-update-custody.md), and the [Portable procedure](../reference/portable-enrollment.md): selected trust owners and one matching operator route | `endpoint enroll`/`enroll-installed` exercise this trust path and generic per-user profile. Neither starts `RunTextParticipant`. Their `ready` is not protected Service readiness. |
| Protected worker admission requires a system-managed Endpoint MainPID and root-owned fixed worker units | [Application confinement](../technical/application-confinement.md): selected Ubuntu security boundary | Installed text tests create a system unit around `endpoint headless`. The shipped `.deb` instructions render a separate `systemd --user` enrollment unit. The qualification installer under `packaging/stream-qualification-worker` serves another profile. |
| A v2 plan selects the protected runtime and is refused if required permissions or roots are absent | [Endpoint/Service runtime](../technical/endpoint-service-runtime.md) and [command reference](../reference/commands.md): current runtime contract and CLI facts | `cmd/ardents/endpoint_headless.go` decodes the supplied plan; the reference lists fields but does not provide an operator route that supplies and authenticates them. This is accurate command documentation with an unresolved provisioning step. |
| The installed package contains verified static payload, not participant state or a system unit | `packaging/ubuntu-deb/README.md`: package procedure outside `docs/` | Its explicit per-user enrollment directions match `installed-user-unit` behavior. It does not claim to qualify the protected system-managed text runtime. |

The target documentation change follows the implementation decision: the
operator procedure should link from the command reference, name the exact
plan/unit producer and verification steps, and distinguish program acceptance,
local-socket readiness, Descriptor ACK, and completed Reader presentation.
Until that route exists, duplicating a speculative command sequence in product
scope or a second runbook would obscure the missing handoff (F-25/F-27).

### Claim audit: one active runtime version

Several current reader routes described a former accepting path or retained
present-tense implementation at the inspected baseline. The architecture
worktree corrects their current owners and flags the remaining archival
wording for ADR-0092 integration without adding another technical owner:

| Claim a reader encounters | Source and current evidence | Owning correction |
| --- | --- | --- |
| `ardents-interactive-route-v2` was called the selected native Route profile | Accepted ADR-0089, the protected Route v3 contract, and Node dispatch refuse an old duty (F-38). | `network-route-node.md` now marks v2 as retained grammar and directs C0 readers to protected v3. Exact old identity/readers still require retirement or persisted-data disposition. |
| Route's module table implied that the old User attachment surface itself was absent | `route.OpenEntryAttachment` and `OpenEndpointTransitAttachment` remain in production source; after committed ADR-0092 deletion, the inspected non-test caller search finds no selected Endpoint root (F-35/F-37). | `network-route-node.md` now distinguishes the removed aggregate User Route runtime from the still-present v2 attachment/codec closure awaiting a separate retirement decision. |
| Node lifecycle prose described each new Entry or Transit Grant admission as the active duty | The old Node engines and plan routes refuse before startup; closed Route receiver admission is the selected path. Its role-to-Epoch join is still missing in code (F-45). | `network-route-node.md` now states the closed admission contract and locates the role join with State instead of presenting the retired admission path as live. |
| Administration `Publish` was said to start generic `endpoint.StartPublisher` | The installed text participant opens `textAdministration`; its bodyless `Publish` refuses and `PublishSnapshot` starts the selected text Publisher. Generic `OpenServiceAdministration` was removed under ADR-0092 (F-37). | `endpoint-service-runtime.md` now describes the selected snapshot path and retired `route.Route` composition. Separate Administration authorization stays current. |
| Endpoint/Service said the maintained Publisher uses SealedIntroduction v1 as its Instance recipient | `text_descriptor_publication.go` creates `instance.PrivateRecipient`; `text_introduction_registration.go` opens the current `route/capsule` through it. The old `Binding.OpenIntroduction` has no non-test caller after committed generic Publisher deletion (F-42). | `endpoint-service-runtime.md` now describes the volatile private recipient and marks SealedIntroduction v1 historical. The old method and persisted key fields still need separate disposition. |
| Private reachability opened as though the generation-2 Gateway/OHTTP path were still current | ADR-0091 removed its unwired Relay/Gateway/Client adapter; Node Resolution calls `Store.PublishPrivate`/`LookupPrivate` for the current generation-3 path, while old stored records still hold floors (F-22/F-33). | `private-reachability.md` now leads with the current v3 owner and labels the generation-2 section historical. It retains the old-format floor question without describing OHTTP as an active alternative. |
| Transit Grant acquisition gained a historical status while its protocol body retained present-tense wording | Committed ADR-0092 removed the Endpoint acquisition chain; the Node-side issuer was already retired. Route's Grant verifier remains uncalled and the local-role root still stores historical spends (F-52/F-53). | `transit-grant-acquisition.md` now frames every present-tense section as historical design, routes current readers to private admission, and identifies the separate Grant/local-role disposition. The documentation map marks it as provenance, not a current runtime contract. |
| The root Start-here route and threat invariants still presented Transit acquisition as current | README linked the historical Transit document alongside live technical owners; the threat model described its signer and Endpoint acquisition in present tense. ADR-0092 retires both runtime owners but retains ADR-0062 as provenance. | README now routes ordinary C0 readers to private admission; the threat model marks the exact old signer/acquisition constraints historical rather than an available C0 operation. The historical document remains linked only for named provenance. |
| Product scope's command-role summary still assigned Transit issuance to Node and a Transit root to Endpoint | The old Node issuer command refuses before startup; committed ADR-0092 removed Endpoint Transit acquisition. The selected headless plan uses `text_token_root`, and closed Node issuer owns the current token duty. | `docs/product/scope.md` now names the closed-token issuer and Endpoint text-token root, preserving the separate Authority, State, Entry and Publication owners. Recheck after any later Endpoint contract change. |
| The protected Route contract and Network owner still described generation 3 as only future while generation 2 was called current | The maintained Node dispatch starts closed v3 duties, Route exposes its v3 Carrier/ARDP owners, and protected Endpoint calls `NewAuthenticatedStream`; the old sequential `NewStream` has no independent non-test production caller (F-41). Full installed C0 qualification is still pending. | `protected-route-protocol.md` now distinguishes selected and partially implemented v3 from unqualified installed composition; `network-route-node.md` identifies retained v2 grammar as migration/refusal input. Neither claims that code presence proves the installed journey. |
| Enrollment v3 sounded like the only accepted bundle descriptor across all commands | `enrollment.VerifyHeadless` requires v3-only Node/Custody companions, but the installed Endpoint and standalone control inspection routes call general `Verify`, which still accepts v1/v2/v3 (F-47). Inspection advances only its own Catalog, Release and Network floors (F-64). | `enrollment-verification.md` accurately states v1–v3 parser behavior. The route-specific acceptance decision remains open under ADR-0042's retained compatibility; do not call every command v3-only or delete the old parser by inference. |

None of these corrections selects a new wire format. The Service Connection v2 record
grammar and Administration v1 local interface remain current *within their own
boundaries*, even though the Network Route is v3 (F-36). The source inventory
is reconciled to committed ADR-0092; later Endpoint changes still require
their own integration check.
The [version disposition table](c0-component-reconstruction.md#one-supported-c0-configuration)
now identifies, for the selected C0 boundaries, which old entrypoints only
refuse input, which still have a production call chain, and which read
persisted authority on restart. The immediate unresolved retirement gates in
these rows are the old Route/Transit exact-caller closure and Reachability's
stored v1/v2 Target floors; neither is a reason to run a second C0
implementation.

## Reduction rule

The repository is larger than the C0 runtime. Keep three different questions
separate while reducing it: what an installed C0 process executes; what an
accepted compatibility or safety obligation retains; and what only records the
reason for a past decision. A historical ADR or research record can remain
searchable without being part of the normal implementation reading route.

For each proposed removal, record the production caller or its absence,
persisted/wire identity, refusal and restart behavior, test oracle, and current
document owner. Remove exact copies when a canonical fixture already supplies
the same behavior. Reconcile uncomposed code against its explicit retention
decision. For current documents, promote one unique current fact to its owner
before retiring a duplicate or obsolete transition narrative. File count alone
is neither a retention rule nor a deletion rule.

The current status extraction finds 87 tracked ADR files: 76 declare `accepted`,
three `proposed`, one `withdrawn`, and seven superseded or partially superseded.
In particular, ADR-0073, ADR-0076, and ADR-0080 are proposed. The next
documentation pass will follow references from current owners before using any
of those records as an architectural constraint.

The 55 observed production packages all have entries in the factual
`package-map.md`. Their Linux/Windows import union contains 133 first-party
edges. Eight packages have platform-dependent first-party imports; in
particular, Linux Endpoint includes its Application, qualification, resource,
wire, and durable helpers that the Windows import graph does not show. A
Windows-only dependency review would therefore miss important C0 seams.

### Command reachability is narrower than package retention

At `53f02e64`, a Linux/amd64 cgo-disabled `go list -mod=readonly -deps` pass
over all seven maintained commands reaches 45 of the 48 `internal` packages.
The three absent packages are `internal/architecture` (test-only owner),
`internal/naming/namespace` (root composition with no command opener), and
`internal/naming/resolution` (retained but uncomposed under ADR-0090). This
is compile reachability, not proof that every included operation runs in C0.

| Inclusion | Actual path and boundary |
| --- | --- |
| Namespace subpackages in `ardents-custody` | Custody imports Authority, Epoch and Record for retained Namespace `Vault.Execute` cases. No current command constructs those cases or opens the Namespace Epoch root (F-51); inclusion does not make Name control an installed C0 path. |
| `application/streamqualification` and `qualification` in `ardents` | Linux Endpoint imports both. The ordinary text worker calls `launchInstalledWorker` with a nil qualification run; the installed qualification command calls the separate `RunStreamQualification` adapter. Their 18 production files enter the product binary's static closure, but the command's normal headless path does not select that workload (F-55). |
| Alpha control and Contributor | `alphacontrol` enters only `ardents-control`; `contributor` only `ardents-node`. These are retained operator/hosting owners, not a second Endpoint or protected Route accepting path. |

The command import result prevents declaring Namespace or qualification code
necessary to the C0 data path merely because a broad package compiles into a
binary. The reverse is also true: uncomposed Resolution still has an accepted
retention decision. A removal or package move needs its caller, resource,
persisted identity and owner contract, not a `go list` count alone.

## Investigation order

1. **Documentation and claim route:** identify selected C0 behavior and its
   current owner first, then check accepted ADRs and named research only for a
   disputed decision. Mark superseded/proposed decisions explicitly.
2. **Executable and behavior route:** enumerate every maintained command and
   packaging entrypoint. Follow each selected input through admission,
   authority checks, effects, failure, restart, and terminal cleanup. Link the
   corresponding direct and installed tests.
3. **Source ownership route:** review the 815 production Go files by actual
   package and caller graph. First resolve the Route/Node/Endpoint/Service
   mixed cluster, then cover Network, Entry, Custody, Release, Enrollment,
   Application, Resource, Diagnostics, Naming, Contributor, and command
   adapters. Include tests and fixture owners after each runtime cluster.
4. **Resource route:** draw a graph of keys, roots, listeners, Carriers, logical
   Connections, worker processes, and observation sinks. For each edge, name
   who opens, borrows, revokes, joins, closes, and reports a cleanup failure.
5. **Reconstruction:** derive Module seams and import direction from the
   behavior/resource graph. Apply the deletion test to shallow pass-through
   packages and verify that each proposed new package has real implementation,
   a small Interface, tests, a production caller, and a package-map entry.
6. **Transition:** order changes around one observable Publisher/Reader path,
   identify bounded integration checkpoints for completed network and Endpoint
   work, and define how a future combined installed C0 candidate will be
   qualified with both selected Carriers.

Findings must distinguish contract fact, code fact, inference, and uncertainty.
No `retain` label means a file is permanently needed; no `retirement-review`
label authorizes deletion. A normal `go test` run cannot by itself establish an
installed C0 behavior claim, and a document title cannot establish a current
contract when its declared status or authority says otherwise.
