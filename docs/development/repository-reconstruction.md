# Repository reconstruction study

Status: **active working analysis**. This is a factual source map and design
workspace, not a second product specification, a C0 issue ledger, or the formal
frozen-candidate [deep audit campaign](deep-audit.md). Work is based in the
architecture-refactor worktree; another agent's unfinished Endpoint changes
remain outside this study's ownership.

## Question and completion standard

If Ardents were composed today solely from the selected C0 outcomes and
accepted safety/compatibility obligations, which Modules, Interfaces, process
boundaries, durable roots, and tests would be necessary? Where does the current
implementation put each responsibility, and what must move, deepen, be replaced,
or be retired to reach that shape without changing accepted behavior?

The study is complete only when:

1. every tracked file has a physical and semantic disposition, with production
   Go callers, mutable state, resources, and tests traced for proposed moves or
   retirement;
2. each current product, security, technical, operational, reference, and
   engineering fact has one document owner, with ADR/research provenance and
   supersession conflicts identified;
3. every maintained command route and selected C0 behavior has normal,
   refusal, interruption, restart, resource, and terminal-outcome paths mapped
   to its implementation and evidence;
4. the existing and target package/import graphs, process trust boundaries,
   durable-root ownership, and stop/join order are explicit;
5. the target design names the necessary Modules and their small Interfaces,
   shows the Publisher/Reader journey end to end, and states which existing
   code is retained, deepened, replaced, or retired;
6. a dependency-ordered transition plan identifies independently reviewable
   slices, compatibility preservation, integration checkpoints with the other
   active task, and verification for the combined installed Ubuntu scenario.

## Source baseline and coverage

Initial tracked baseline: `50026274` on `codex/architecture-refactor`, with
concurrent uncommitted Endpoint edits. The initial Git inventory contains 1,933
tracked files: 848 production Go files under `cmd` and `internal`, 684 Go test
files, 38 Go fixture commands under `tests`, seven Go developer tools under
`scripts`, 201 tracked documentation files, and other assets. These numbers are
inventory counts, not reviewed or accepted architecture coverage. The live
worktree must be reconciled at each integration checkpoint.

At `e48d4c3c`, the next Endpoint commit changed ten existing Go files and no
tracked paths; all file counts above still match. The package graph below was
read from the current worktree at that revision, including Linux and Windows
build selections.

| Map | Current coverage | Limit |
| --- | --- | --- |
| [Repository file map](repository-file-map.csv) | All 1,933 tracked paths, physical group, artifact class, and review state; all 848 production files have a provisional responsibility; 58 production files have a direct behavior source trace | The final 170 assignments cover Application, Contributor, Qualification, Diagnostics, Architecture, and command adapters. The other assignments are inventory coverage, not proof of necessity: caller, mutable-state, resource, compatibility, and test evidence still need review for proposed moves and retirement. |
| [Package import graph](repository-package-graph.csv) | All 57 `cmd`/`internal` Go packages, production/test file counts, and first-party imports on Linux and Windows | Import edges show compile dependencies, not ownership or runtime call direction. |
| [Documentation map](repository-documentation-map.csv) | All 201 tracked `docs/` paths, title, declared status, role, and first authority classification | Current fact ownership, inbound links, duplicated claims, and correspondence to code require source-level review; a status label alone is not proof that a claim is still true. |
| [C0 component inventory](c0-component-inventory.csv) | All 386 production Go files in Node, Route, Endpoint, and Service | Proposed owner and treatment; mixed files and retirement candidates still need caller/resource proof. |
| [Behavior trace map](repository-behavior-map.md) | Initial selected C0 and maintained command journeys with contract and production entrypoints | Complete execution paths, refusals, lifetimes, and tests have not yet been traced. |
| [C0 Module composition](c0-component-reconstruction.md) | First-principles owner model for the installed Publisher/Reader scenario | Target Interfaces, import direction, and transition decisions remain open. |
| [Evidence-backed findings](repository-reconstruction-findings.md) | First confirmed uncomposed code, document drift, and platform graph observations | Disposition requires accepted contract and caller/resource review. |

The [package map](package-map.md), [command surface](command-surface.md), and
[documentation policy](documentation.md) are existing factual owners. This
study cross-checks them against code; it does not silently promote a research
record, proposed ADR, experiment, or historical compatibility artifact into a
current requirement. The `old` branch is outside the design input.

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
| Admission, Transit issuance, and reachability | [Private admission](../technical/private-admission.md), [Transit Grant acquisition](../technical/transit-grant-acquisition.md), [private reachability](../technical/private-reachability.md) | Separate token/issuer, grant, and Descriptor authority. |
| Endpoint, Service, Application confinement and common privacy | [Endpoint/Service runtime](../technical/endpoint-service-runtime.md), [Application confinement](../technical/application-confinement.md), [common privacy architecture](../technical/common-privacy-architecture.md) | Read the affected owner, not every Service section. |
| Release, Custody, enrollment, alpha control, and naming | [Release/update/custody](../technical/release-update-custody.md), [enrollment verification](../technical/enrollment-verification.md), [alpha control](../technical/alpha-control-transition.md), [naming](../technical/naming.md) | Each owns a distinct trust or compatibility transition. |
| Operator route and retirement procedure | [Command reference](../reference/commands.md), [Portable first-execution instruction](../reference/portable-enrollment.md), [Contributor profile](../reference/rendezvous-contributor.md) | Portable pre-execution verification has a direct route from the command reference; it does not qualify the separate protected text system-unit launch. |
| Engineering rules and factual maps | [Package map](package-map.md), [command surface](command-surface.md), [testing](testing.md), [dependencies](dependencies.md), [repository layout](repository-layout.md), [documentation policy](documentation.md), [risk exceptions](scoped-risk-exceptions.md), [agent execution](agent-execution.md), [deep audit](deep-audit.md) | [Development index](README.md) is a reader route; `ownership.json` is executable PR selection policy, not a prose contract. |
| Working designs and measurements | [Network-core proposal](network-core-transition.md) and [wire appendix](network-core-wire-proposal.md), [Endpoint design](endpoint-architecture-refactoring.md), [Route boundary](route-refactoring-boundary.md), [privacy workstream](privacy-anonymity-map.md), [qualification design](privacy-qualification.md), [test-cost measurement](endpoint-test-cost.md) | Read for a named design/evidence question. Proposed mechanics and measured old runtimes are not current implementation truth; execution state belongs to Issues. |

This routes all 41 current-location documents without asking one contributor
to read all 41. It is a responsibility index, not a duplicate specification;
the source document itself remains the canonical owner of its facts.

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

The first status extraction found 86 tracked ADR files: 76 declare `accepted`,
three `proposed`, one `withdrawn`, and six superseded or partially superseded.
In particular, ADR-0073, ADR-0076, and ADR-0080 are proposed. The next
documentation pass will follow references from current owners before using any
of those records as an architectural constraint.

The 57 observed production packages all have entries in the factual
`package-map.md`. Their Linux/Windows import union contains 139 first-party
edges. Eight packages have platform-dependent first-party imports; in
particular, Linux Endpoint includes its Application, qualification, resource,
wire, and durable helpers that the Windows import graph does not show. A
Windows-only dependency review would therefore miss important C0 seams.

## Investigation order

1. **Documentation and claim route:** identify selected C0 behavior and its
   current owner first, then check accepted ADRs and named research only for a
   disputed decision. Mark superseded/proposed decisions explicitly.
2. **Executable and behavior route:** enumerate every maintained command and
   packaging entrypoint. Follow each selected input through admission,
   authority checks, effects, failure, restart, and terminal cleanup. Link the
   corresponding direct and installed tests.
3. **Source ownership route:** review the 848 production Go files by actual
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
   integrate completed network and Endpoint work at bounded checkpoints, and
   qualify the combined installed C0 path with both selected Carriers.

Findings must distinguish contract fact, code fact, inference, and uncertainty.
No `retain` label means a file is permanently needed; no `retirement-review`
label authorizes deletion. A normal `go test` run cannot by itself establish an
installed C0 behavior claim, and a document title cannot establish a current
contract when its declared status or authority says otherwise.
