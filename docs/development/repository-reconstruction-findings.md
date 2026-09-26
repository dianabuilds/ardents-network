# Reconstruction findings

Status: **working evidence ledger** for the [repository reconstruction](repository-reconstruction.md).
These are source facts and bounded inferences, not an instruction to delete or
move code. A finding is updated when the active Endpoint/network task changes
its evidence.

## F-01: Naming runtime retained without a production path

**Code fact.** The Linux/Windows package import graph has no path from any of
the seven `cmd` packages to `internal/naming/resolution` or the root
`internal/naming/namespace` package. Together those packages contain 17
non-test Go files and 24 package test files. `cmd/ardents` imports Resolution only in
`name_retirement_fixture_test.go`, where it serves a zero-effect refusal oracle.
The Namespace subpackages and local `internal/naming` encoder have separate
production callers and are not included in this count. `internal/architecture`
is the fourth package without a command path; it is a test gate, not a runtime
candidate. F-51 follows those indirect callers: the Namespace subpackages
and Resolution together contain 69 production files without a composed
command-level Namespace journey.

**Contract fact.** [ADR-0090](../adr/0090-retire-name-operator-network-adapters.md)
retires `name resolve` and `name control` at command dispatch while explicitly
retaining the uncomposed Resolution Module until an exact-consumer decision.
The current [Naming owner](../technical/naming.md) also states that no
production runtime composes a Gateway or Resolver, or the whole Namespace
transition sequence.
The former `internal/naming/alpha/private` package was retired under
ADR-0091; its lack of callers no longer inflates the maintained inventory.
The remaining Resolution Module is explicitly retained by ADR-0090 pending an
exact-consumer decision. Removing it solely from the import graph would
violate that current decision route.

**Reconstruction question.** Which exact compatibility behavior or future
selected Name behavior needs executable source in the maintained tree, as
opposed to tests, immutable evidence, and Git history? Resolve that against
ADR-0090 and the Naming owner before choosing retain, move to historical
evidence, or remove. Preserve the refusal oracle or replace it with equally
strong zero-effect evidence if the Module is retired.

## F-02: Dependency register describes a retired import path

**Code fact at `53f02e64`.** `cmd/ardents` production imports do not include
`internal/naming/resolution`; its `name resolve` and `name control` dispatch
returns a retirement refusal. The [dependency register](dependencies.md)
preserves dated 2026-09-22 and `e48d4c3c` projections with former Naming
and Transit/OHTTP paths. The current Linux/amd64 cgo-disabled
`go list -mod=readonly -deps` projection has 312 packages and no OHTTP
package. CIRCL remains reachable through the selected Endpoint and Service
owners; QUIC through Route. The retained Resolution package still imports
OHTTP, but none of these current commands composes it.

**Action.** Keep dated results as historical evidence and label the current
projection separately, as the register now does. Do not edit `go.mod` based
on an obsolete command path; Resolution's retained package obligation needs
its own disposition under ADR-0090.

## F-03: Platform selection changes the architecture graph

**Code fact at `53f02e64`.** All 55 current `cmd`/`internal` packages are registered in
`package-map.md`, but eight packages have different first-party imports on
Linux and Windows. Linux Endpoint adds Application Interface v2, the installed
text worker, qualification, resource, and protected-wire helpers that are
absent from its Windows import graph. A Windows-only graph omits much of the
selected C0 behavior.

**Design consequence.** The target import graph and ownership map must be
checked against Linux first, then across Windows for the supported compile
boundary. Compile edges alone do not prove resource ownership or a runtime
caller; the command/behavior trace supplies that evidence.

## F-04: Document count and reading cost have different causes

**Repository fact at `53f02e64`.** Of 212 tracked `docs/` files in the current map, 88 are ADR records and 66 are
research records. Seven research records declare an open, deferred, or
draft-not-ready state; they cannot supply selected decisions. The remaining
records provide decision evidence, while the current reader route starts with
product/security and affected technical and engineering owners. There are 42
current-location documents, plus indexes, receipts, and fixtures. Three ADRs
are only proposed, one withdrawn, and seven superseded or partially superseded.

**Inference to test.** The maintenance cost is not simply 211 files. It is
the number of current facts copied across owners, the length of the normal
reading route, and how often a code change requires editing several owners.
The documentation pass will trace selected claims and links to identify those
specific duplicates and stale descriptions before proposing consolidation.

## F-05: The deadcode gate explicitly carries a large uncomposed surface

**Repository fact at `53f02e64`.** `scripts/check-deadcode.go` compares the
production `deadcode` result with
`tests/profiles/deadcode-allowlist.json` for Linux and Windows. After the
ADR-0092 deletion, that allowlist names **487 common function symbols**:
338 in classifications beginning `unwired`, 105 beginning `retained`, and
44 in other explicit classes (four fixture-support, twelve shared Route
framing, and 28 Route v2 wire/Introduction closure). The Namespace-control
tracer alone accounts for 191 of the common `unwired` symbols; the retained
uncomposed Resolution class has 55, and the shared legacy
Route/Entry/Credential/Reachability/Service group has 58. Windows has 207
additional symbols, including 180 from the selected Linux-only protected
text runtime; Linux has one additional resource-acquisition symbol. These
are **allowlist symbol counts**, not file or line counts or a fresh deadcode
tool result. The Windows-only group must not be treated as unnecessary Linux
code.

**Interpretation.** A passing deadcode gate means the observed dead functions
match the reviewed allowlist, not that the maintained tree has no disconnected
implementation. The gate itself records a large amount of code whose current
consumer is absent or historical. The classification and retirement rationale
for each allowance provide a starting queue for exact-consumer review.

**Next proof.** Re-run the tool on the eventual frozen candidate, cross-check
each allowance against accepted decisions, production command reachability,
test-only evidence, and persisted/wire compatibility. Only then reduce the
allowlist together with the corresponding source. A platform projection or
static deadcode result alone cannot authorize deletion.

## F-06: A reachable dispatcher hides uncomposed Namespace behavior

**Code fact.** `cmd/ardents-custody/command.go` admits fixed Service and
admission Authority creation/issuance, plus envelope inspection, record
verification, recovery Bundle operations, and purge. Its production callers
construct `custody.Operation` for those routes. The shared
`internal/custody/Vault.Execute` switch also accepts
`OperationSignNamespaceTransition`, `OperationPrepareNamespaceSubmission`, and
`OperationActivateRecoveredAuthority`. A search of non-test Go callers found
no command or other package constructing those three operation kinds; the
references are their definitions, internal dispatch/receipts, and tests.
`verify-record` still accepts an `AuthorityName` binding, so historical Name
record inspection and recovery are separate live obligations.

**Inference.** Namespace signing/preparation is not currently an operator
behavior merely because Custody and its dispatcher are reachable from a
binary. A function-level deadcode pass sees the switch branches as reachable;
it cannot prove an admitted command supplies each discriminant. The current
`Operation` Interface also exposes Namespace, Service, and admission inputs
through one union-shaped request, increasing what every caller must understand.

**Exact operation matrix at `53f02e64`.** The switch accepts twelve kinds.
Eight are constructed by production command routes: Service Authority create
and credential issue, admission Authority create and permission issue, record
verify, Bundle export and restore, and record purge. `CreateVaultRecord` is
constructed only inside the two Authority-create implementations; it is an
internal shared primitive rather than a command. The remaining three kinds
are the Namespace sign, submission preparation, and recovered-authority
activation cases above. No non-test `cmd` or `internal` caller constructs
them. `inspect-envelope` is a separate public-header command and never opens
the Vault. This is a capability-composition issue, not twelve versions of one
Vault format: removing the three cases requires an explicit decision about
retained Name records and authority floors, while the eight command operations
must continue to share the single encrypted Vault root.

**Reconstruction question.** Keep the shared encrypted Vault and retained
record-format/recovery obligations, but decide whether the uncomposed Name
transitions belong behind a separate Namespace-facing Interface or only in
historical evidence until a current producer is selected. Trace persisted
record and floor compatibility before changing this seam.

## F-07: State-to-Node Interface is a 38-getter data transfer

**Code fact.** `node.DutyView` declares 38 `Duty*` getter methods in
`internal/node/contract.go`. `state.NodeDutyView` implements that projection
from a broad authenticated Snapshot in `internal/network/state/node_duty_view.go`;
Node then copies the getters into its private `dutyFacts` and candidate arrays
in `internal/node/lifecycle.go`. The view deliberately prevents Node from
opening State storage or seeing pending/source metadata, which is an important
trust boundary. Yet the Interface itself mostly mirrors fields one by one,
including thirteen candidate getters indexed separately. The candidates are
read by current closed forwarding, Resolution and issuer peers, so they are
not wholesale legacy. In contrast, `DutyAuthorityCount/ID/PublicKey` are
copied into `dutyFacts` but have no production read after that copy in the
inspected Node package. `state.NodeDutyView` also defines three
`DutyTransitIssuance*` methods that are not members of `node.DutyView` and have
no non-test call. These projection methods outlive the old Transit duty; this
does not by itself retire the signed Epoch fields they read.

The concrete `NodeDutyView` value contains a full copied `Snapshot`, including
Source attempts/outcomes, pending Epoch facts, control roots, and opaque old
issuer-profile bytes. Those fields are not exposed by its duty getters, so this
is not an observed Node authority leak. It does mean the apparent narrow view
is implemented as a broad copy and remains coupled to Snapshot's layout.
`currentFacts` checks the 64-candidate and 16-authority bounds and rejects
empty copied authority IDs/keys before Node uses the value. A search at
`53f02e64` found `DutyView` in 21 Node test files; changing the callback is a
bounded but nontrivial fixture migration, not a one-line rename.

**Design inference.** This is a shallow Interface at the State/Node seam: a
caller must learn many getters to obtain one immutable duty decision, and State
and Node maintain parallel field projections. The code is a plausible source
of change amplification whenever the accepted duty facts grow.

**Target to evaluate.** `cmd/ardents-node` is the only production caller of
`CurrentNodeDuty`, and Node already imports State, so a State-owned copied
`NodeDuty` value returned through `Config.Current` would add no reverse import
or new package. It can group the authenticated generation, local record,
assignment and bounded candidate values in one snapshot, without source,
retry, pending or storage fields. Node should validate bounds at receipt and
retain its local copy across the lifecycle; State still owns freshness and
conflict classification. Remove the unused authority projection from the C0
handoff only after checking its old Grant compatibility obligation separately.
The 38-method test fixture seam then changes with the same bounded slice;
preserve current admission, quarantine, restart and old-profile refusal tests.
Replacing getters must not weaken authentication or expose the full Snapshot.
The concrete callback to assess is `func() (state.NodeDuty, error)` with one
State-created value grouping current Epoch identity/freshness, the local signed
record and assignment, and a bounded candidate list. This value must contain
only copied duty facts, not a nested `Snapshot` or mutable State handle. Keep
Node's per-poll local copy and revalidate candidate bounds and complete
identities on receipt; settle the four-domain closed-profile role join (F-45)
before freezing its fields. The new State value remains a proposal until the
current technical contract and the active Node slice agree on that join.

## F-08: Entry has two distinct retained root lifecycles

**Code fact.** `internal/entry.Open` owns an Invite/recipient root, imports a
signed Invite, records adjacent contact attempts, and joins acquired
attachments on close. `cmd/ardents entry recipient/import` is its maintained
operator caller. Separately, Linux `entry.OpenClosedSets` owns a claimed
installation root with two State-selected members per adjacent Role Domain;
Endpoint's protected participant uses it for C0. It never issues an Invite or
owns a transport. The two paths share low-level root lease, permission, and
durable-file helpers inside one 29-file package.

**Design inference.** The package name describes a real common concept, but
its two roots have different authorities and terminal obligations. A future
split must place the shared filesystem primitives deliberately and avoid
accidentally allowing a legacy Invite root to reopen as a C0 Entry-set root.
The current `OpenClosedSets` contract explicitly refuses that substitution.
The Invite command is a separately dispatchable older operation, not a second
version selected by the protected C0 participant: its only production opener
is `cmd/ardents/entry_import.go`, while the participant calls `OpenClosedSets`
from `text_source_state.go`. Keeping both constructors in one package should
not be read as a requirement to support both operations indefinitely.

**Exact old-path closure at `53f02e64`.** The command calls only
`entry.Open(...).Import(...)` and `entry.RecipientPublicKey(...)`, with its
State-view adapter in `cmd/ardents/entry_import_plan.go`. The `entry import`
verb opens and mutates the Invite root; `entry recipient` only returns its
public key and has no writer effect. A non-test call search found no producer
for exported `entry.Issue`, no caller of the exported
Initiator-side `entry.Verify`, and no caller of `(*owner).Contact`.
`(*owner).Acquire` is referenced by `route.OpenEntryAttachment` through its
`EntryAcquirer` interface, but that Route opener itself has no selected
non-test caller after ADR-0092. `verification.go` is **mixed**: its exported
`Verify` wrapper is uncalled, while `validateInvite` is used by the live
`Import` and reopen paths through `owner.validate`; it cannot be deleted with
the old receiving adapter. `Issue` remains a valid-Invite fixture producer in
`internal/entry/entry_test.go`, not a maintained command operation. This
leaves the dispatchable import command as a writer without an in-tree
production issuance path or selected attachment consumer.

The two durable owners share `inspectRoot`, root permissions and lease,
generation/pointer/watermark operations, bounded reads and directory sync.
The closed-set root has a distinct marker and rejects the Invite root, but its
lock filename is still the shared historical `.ardents-entry-state-lock`.
Moving just the two `closed_set*.go` files into a new package would therefore
require a deliberate durable-root helper boundary or duplicated filesystem
logic; it is not a package-declaration edit. Existing closed-set tests cover
restart pointer reconciliation and explicit refusal of an old nonempty
Rendezvous-2 slot. They do not migrate an Invite root into a closed set.

**Disposition to decide.** The target C0 configuration has one Entry
admission path: the closed set used by Endpoint. The older Invite command
still writes a distinct durable root although its Route attachment no longer
has a selected production caller. The current command inventory and reference
still classify `entry recipient/import` as retained, and ADR-0081 keeps
generation-2 facts until explicit migration. Audit ADR-0072's accepted
operator contract and existing roots, then retire its writer and uncalled
execution path under an explicit decision. Keep only the minimum reader or
typed refusal needed to preserve an existing authority or rollback floor until its data
disposition is complete; remove that reader afterward. A package split should
follow the surviving ownership boundary, not preserve two Entry runtimes.

## F-09: Resource has two necessary but different lifetimes

**Code fact.** `resource.Guard` checks process placement and samples pressure;
Network State and Node construct it with `resource.New`. Separately,
`resource.Hosting` owns a durable shared provider-period root, charges
interface-counter deltas, and retains work/termination reservations until the
consumer joins and releases them. `ardents-node hosting initialize` creates
the period; Node opens it for forwarding and qualification reads it. Both are
used by selected C0 paths, but one is a volatile process governor and the
other a persistent multi-owner ledger.

**Exact boundary.** `Guard` exposes `Check` and `Observe`; it holds an
in-memory pressure monitor and has no `Close`. Measurement failure returns a
protect-and-drain observation plus the error. State owns its governor goroutine
and cancellation; Node combines its Guard result with Hosting pressure and
owns the duty drain. `Hosting` exposes `Sample`, `Observe`, `Reserve`, and
`Close`; `HostingReservation.Release` follows the consumer's work/termination
join. `Sample` may share a committed observation up to one second old, while
`Reserve` always uses a fresh durable transaction. `Close` releases only the
handle, not outstanding reservations. These are already the small observable
Interfaces; adding a Go interface solely for package aesthetics is unnecessary.
A package split is justified only if it removes actual cross-owner knowledge
or dependency, not by the 21-file count.

## F-10: Current architecture owners are hard to read and maintain

**Repository fact.** `docs/development/package-map.md` is only 83 lines, but
its Endpoint row is 6,756 characters long. It mixes package responsibility,
selected behavior, shutdown rules, platform caveats, and an import list in one
table cell. The working `network-core-transition.md` is 1,545 lines; the
current `endpoint-service-runtime.md` is 654 lines. These measurements do not
establish that their content is dispensable, but they do show that document
count alone understates the cost of locating and updating an architectural
fact.

**Concrete owner split and correction.** `docs/technical/private-reachability.md`
made its generation-3 Descriptor/Store contract current in the opening section,
but also carried 189 lines of generation-2 OHTTP Publisher/Gateway instructions
in the same current owner. ADR-0036/0037 preserve the former authority and
carrier decisions; ADR-0091 retired the unwired adapter, while F-32 and the
current Store source retain the exact v1 stored-root recovery/refusal question.
After checking those distinct obligations and the one inbound section link,
the historical block was reduced to a 20-line provenance and persisted-floor
note; the research link now points to the current Descriptor section. This
removes a misleading second operational narrative without deleting the old
decoder or treating an unresolved root migration as complete.

**Reconstruction question.** For each current fact, identify one short owner
and link its evidence. Keep the package map to responsibility and permitted
imports; put behavioral invariants in the affected technical owner; keep
transition chronology out of the current contract. Check each proposed move
against the existing documentation policy and source before editing owners.

## F-11: Portable enrollment instruction is outside the operator reading route

**Repository fact at baseline.** The former `docs/product/closed-alpha-enrollment.md` contained a
complete shell procedure for independently pinning and verifying a downloaded
bundle, rendering `endpoint user-unit`, and starting a user service. Its
`endpoint user-unit` route remains in the current command reference. A search
of tracked Markdown found no inbound link to this instruction, although
`docs/development/documentation.md` says operators should reach procedures
from the runbook and command/configuration reference. The file is in
`docs/product/`, whose declared role is product scope and promises.

**Disposition.** The procedure now lives at
[`docs/reference/portable-enrollment.md`](../reference/portable-enrollment.md)
and is linked from the matching `endpoint user-unit`/`enroll` route in the
command reference. Its independent Enrollment Pin and pre-execution check are
unchanged. The reference explicitly distinguishes Portable readiness from
the installed protected-text system-unit boundary (F-25/F-27); this move does
not claim a joined C0 launch.

## F-12: The remaining experiment tree is small and linked to active research

**Repository fact.** The tracked `experiments/` tree now has 13 files in five
question-scoped directories: four R-149 analytical experiments and one R-152
contract-probe directory. The R-149 and R-152 research records link directly to
these remaining sources. Earlier historical experiment trees are referenced
through pinned Git revisions, not present as current working-tree code.

**Disposition.** The old suggestion to delete nine large experiment trees no
longer describes this worktree. These 13 files are research evidence, not C0
runtime or test-suite dependencies. Review their retention when their exact
research questions are closed or superseded; deleting them now would break
named evidence links without materially simplifying the production code.

## F-13: Historical Alpha resolver was retired under ADR-0091

**Baseline code fact.** `internal/naming/alpha/private` had seven production files and
two package test files. No production package imports it; the architecture
test instead forbids Endpoint from importing it. The common deadcode allowlist
names 22 of its functions. `internal/naming/alpha` separately owns the retained
Alpha corpus parser and persistent read-only floor, and `ardents-control`
retains supplied-bytes inspection.

**Decision boundary.** [ADR-0088](../adr/0088-retire-alpha-service-links-and-corpus-intake.md)
retires all accepting Alpha Service Link paths and explicitly retains existing
floor bytes with their parser/reader until a data-retention decision. It does
not identify the OHTTP Client/Relay/Gateway implementation as necessary for
that compatibility obligation. At the inspected baseline the package map
still said to retain that historical exchange; ADR-0091 later superseded the
lower-authority retention and removed its package-map row.

**Separation proof.** The package owns OHTTP message/profile handling and
`Client`, `NewRelay`, and `NewGateway`; its `CorpusFloor` was only a supplied
interface, not the persisted floor implementation. The retained
`alpha.OpenCorpus` decoder and `alpha.OpenPersistentFloor` reader live in
`internal/naming/alpha`. `cmd/ardents-control inspect-alpha-corpus` reaches the
decoder through `internal/alphacontrol/inspection`. The exact retired-command
and Endpoint destination refusals live in `cmd/ardents-control/main.go` and
`internal/endpoint/target_link.go`; their process, architecture, and Endpoint
tests are outside `alpha/private`. The two tests inside `alpha/private` exercise
the retired exchange itself. No maintained non-test Go file imports that
package. This proves the selected parser, floor, inspection, and refusal code
is independent of the historical network adapter at source level; it does not
replace the required post-change verification.

**Current disposition.** Accepted [ADR-0091](../adr/0091-retire-uncomposed-legacy-artifacts.md)
retired the seven production and two test files, their package-map row and 22
deadcode allowances. `internal/naming/alpha` parser/floor and external
refusal/inspection remain. Git history holds the former source. OHTTP and
CIRCL have other consumers, so this deletion alone did not remove those
dependencies.

## F-14: Runtime diagnostics now have one local navigation path

**Code fact.** Node, Source, and the installed headless Endpoint emit bounded
JSON-line events with explicit schemas and occurrence times. Node writes
deadline-bounded events and optional atomic latest-state files; Source returns
failure if its ready/terminal event cannot be written. Endpoint serializes
events, timestamps occurrence, and retains a failed background write so its
runtime leaves READY and reports the error. `ardents diagnostics timeline`
streams a local safe projection of these three schemas from app lines or
`journalctl -o json`; its tests reject malformed categories and private fields.

**Limit.** The timeline deliberately omits Network IDs, destinations, tokens,
addresses and request commitments. It gives owner, process, role, Carrier,
event kind, state, and a bounded category, not a complete Route-attempt trace
or persisted history of its own. Its rows follow input order, not a verified
cross-process occurrence order (F-63). The old description of a totally blind
runtime or absent unified operator view is no longer accurate. The next
diagnostic change should start from an actual unresolved failure and name the
missing safe event or correlation field, rather than introduce generic logging
or a second event schema by default.

## F-15: Test feedback cost is concentrated in a few real network workloads

**Measured evidence.** The recorded Linux Endpoint development run at
`7bd68da2` took 548 seconds for 230 top-level tests. Its ten slowest tests
accounted for 313 seconds and its twenty slowest for 415 seconds. A later
Ubuntu WSL2 run at `9c17f772` took 689.430 seconds for the package. These
measurements are tied to their revisions and environments, not current
performance promises. The 256-Connection test and four-Reader Introduction
test deliberately retain 10-second and 5-second bootstrap spacing. Their
network fixture can also wait for the next Permission hour when less than two
minutes remain; source inspection confirms the wall-clock guard.

**Process consequence.** A four-line Endpoint edit should receive an affected
owner check first, then the repository's required `make quick-check` on the
coherent slice and `make check` on the integration candidate. The current
policy already allows this order; re-running the entire 548-689-second package
without changed evidence does not increase confidence. For a structural speed
improvement, trace one consistent clock seam through Endpoint, Custody, Route,
and transport deadlines before replacing the hour guard, and preserve the
real 4-by-64 workload in the final checked profile. Package extraction alone
cannot remove the measured network work.

## F-16: Node E2E repeats the canonical Network State fixture generator

**Code fact.** The five `tests/e2e/node/network_fixture_*_test.go` files for
contract, record, epoch, epoch encoding, and primitive encoding are identical
to the corresponding five `tests/epochfixture/network/*.go` files apart from
their package declarations. They total 356 lines in each location. Node E2E
uses the copied exported types and builders from
`closed_state_provisioning_test.go` and `dynamic_fixture_test.go`. The
canonical `tests/epochfixture/network` package already has a stability test,
is registered as a test-only builder in the package map, and is imported by
the qualification fixture command. Node E2E already imports the neighboring
`tests/epochfixture/assignment` package.

**Bounded reduction.** Replace those two Node E2E callers with qualified
imports of `tests/epochfixture/network`, then remove exactly the five copied
files. Check whether any unexported copied encoding helper has another Node
E2E caller before making the change; preserve the package-map's test-only
boundary and run the affected E2E/fixture checks once. This is a concrete
deduplication candidate, not a reason to merge the independent `state`
unit-test oracle or the older `tests/e2e/network-source` fixture, whose
contracts differ.

## F-17: Issuer timeout retains durable roots without a later cleanup owner

**Code fact.** `node.startClosedIssuer` opens the issuer key root and a
separate replay spend ledger before starting the credential listener. Its
`probeServer.Drain` closes both roots only after
`ClosedTokenListener.Drain` returns success. When the bounded drain times out,
it returns without closing either root. The listener's worker-join goroutine
can later close `drained`, but it does not own those roots or call their
`Close` methods. `node.Run` reports the cleanup failure and returns; there is
no later in-process issuer-root release in this path. This keeps the leases
held while an unjoined child may still use them, which is the safe immediate
choice, but also leaves no eventual owner after that child exits.

**Comparison.** Forwarding, Introduction, Resolution, and Data JOIN place
their durable-root cleanup after child `Wait` in the server's own completion
goroutine. Their `Drain` timeout means the caller does not know the final
result, while the server continues the close attempt. Issuer's close happens
in the Node adapter instead. This is a real lifetime seam, not a file-name
problem or a reason to merge all duties into one type.

**Terminal-result handoff.** `ClosedTokenListener.Done()` is a one-send channel
for the accept-loop result. `serve` sends it before its worker WaitGroup reaches
zero; `Drain` waits on `drained` but returns only the Stop result. `node.Run`
can consume `Done` as a failure signal, while an ordinary withdrawal can
select cancellation first and leave that result unread. A finishing owner
cannot independently receive the same one-shot channel after `node.Run` has
consumed it. The eventual cleanup result must therefore retain the listener
terminal cause and the Stop/worker/root-close outcomes in one shared owner,
without making `Done` consumption the worker-join barrier.

**Contract and evidence gap.** The current Node technical owner requires
`FAILED` rather than `WITHDRAWN` when cleanup is unproven and requires known
root-close errors to propagate. Existing issuer tests cover successful
successor drain and direct/Node-authenticated bootstrap, but the inspected
issuer/listener tests do not exercise a delayed accepted child past the drain
deadline. Before moving this code, select one owner that retains issuer and
spend roots until the last worker joins, including after caller timeout. A
Node-local finishing path can retain one result while the bounded `Drain`
waits on it; the listener alone cannot own roots it was not given. Test the
timeout, later close while the process remains alive, repeated-result
semantics, retry/restart exclusion, and root-close error path. A process that
exits after a timeout cannot promise an in-process late close; its terminal
cleanup result remains unproven.
The observed timeout is an unproven-cleanup outcome, not evidence that an
unauthorized second issuer can start or that a key is exposed.

## F-18: A proposed network-core design carries a second execution narrative

**Document fact.** `network-core-transition.md` is 1,545 lines and about
17,200 words. Its status says the design is proposed, not accepted. Sections
3–5 contain the candidate architecture, file mapping, and compatibility;
sections 6–11 (starting at line 748) include iteration ordering, issue links,
the next post-merge step, test matrix, gaps, handoff, historical cleanup, and
an update check. Its 484-line wire appendix is also explicitly unaccepted.
The former `docs/development/README.md` route linked this proposal beside
current technical owners. The index now links all eleven current technical
owners directly and places the proposal and wire candidate in a separate
working-design section. `documentation.md` assigns live C0 execution state
to GitHub Issues and unique current facts to current owners.

**Design consequence.** The proposal may contain still-useful decisions, so
length alone is not a deletion argument. The overlap of proposed design,
historical transition, and live execution narrative makes it costly to tell
which statements are obligations today. Reconcile each proposed mechanism
against the accepted ADR/current technical owner and the current issue; retain
only unresolved design questions in a short working proposal, promote any
accepted unique facts to their current owner, and retire obsolete execution
chronology to Git history with links repaired. The reader-route correction is
complete; proposal reconciliation and retirement remain open. The wire appendix
stays a candidate until its own decision, not a hidden C0 protocol requirement.

**Section-level disposition for that reconciliation.** This is an edit plan,
not a claim that the unique candidate rules have already been promoted:

| Transition proposal section | Destination of its unique content |
| --- | --- |
| §1 and §3 (proposed core and behavior) | Keep only still-unaccepted choices in a short design proposal; promote accepted facts to the affected current technical owner. |
| §2 and §4 (old baseline and source map) | Replace current-code claims with the revisioned file, package, behavior and command maps in this study; retain a historical source revision in Git for evidence. |
| §5 (dependencies and compatibility) | Carry each still-open migration/refusal rule to the one-version decision boundary; current behavior belongs to the technical owner, not the proposal. |
| §6–§7 (task order and next iteration) | Use the live GitHub Issues ledger; remove copied status and completed chronology after checking that no unique design rule is embedded in a task description. |
| §8 (verification matrix) | Keep unselected checks as candidate evidence; promote a selected profile to `testing.md` and the affected qualification owner with its actual executable entrypoint. |
| §9 (open blockers) | Retain only decision-relevant questions in the selected research/issue owner; a question is not a new C0 requirement. |
| §10–§11 (handoff, document procedure and post-merge history) | Use `documentation.md` and `agent-execution.md` for current procedure; preserve dated evidence in Git/Issues. |

The separate wire appendix remains an unaccepted Connection candidate and
should be decided as a whole against the accepted protected Route contract;
moving its numeric fields into a technical owner piecemeal would silently
select a second protocol.

## F-19: State close masks cleanup failures after a Source-server failure

**Code fact.** `internal/network/state/lifecycle.go:Close` cancels and joins
background work, then calls `storage.close()` and `releaseSourceServer()`.
When `serverErr` or `resourceErr` is non-nil and not `context.Canceled`, it
returns that prior error alone, dropping both close results. The close
attempts still occur, but the caller cannot know from the returned error
whether the durable-root lease or retained Source role also failed to release.
The command adapters generally join `State.Close` with their own result, so
this loss occurs inside State before the adapter can preserve it. No inspected
State test exercises the combined terminal-server and close-failure case.

**Contract implication.** C0 product scope requires joined cleanup, and the
current Node/Route technical owner requires known Node cleanup failures to
remain visible in terminal outcomes. Preserve the
expected cancellation treatment, but join a real server/resource failure with
storage and role-release failures. Cover the combined path with one owner test
using controlled close failures. This is separate from deciding whether State
should expose a new package or from any C0 protocol change.

## F-20: Offline State command still composes two durable acceptance steps

**Code fact.** `cmd/ardents/offline.go:runAcceptOffline` accepts and commits an
Epoch first, then optionally reads and accepts a signed closed profile. A
profile error returns after the Epoch commit and before the command emits its
success event. The separate `accept-closed-profile` route accepts the same
profile against an existing State root. Installed Node E2E tests exercise both
routes, including the combined command's rejected profile leaving no usable
closed Route and its successful combined event. The command reference had
listed only the separate route and omitted the combined flags/order; that
current owner has now been corrected.

**Disposition.** Both entrypoints have actual test consumers, so do not
remove one from a static search. The combined route's partial-success
semantics must remain explicit in the operator journey and recovery trace.
If later narrowed to one route, first establish the accepted compatibility
obligation and migrate the installed fixture/operator sequence in the same
bounded change.

## F-21: Source resume emits a false accepted-wave event

**Code and contract fact.** In `cmd/ardents/source_plan.go`, `--resume` skips
`State.Refresh` and obtains only `State.Current`. The command then
unconditionally emits an `ardents-source-event-v1` event with kind
`source-wave-accepted` and the current snapshot. The current command reference
says that event is emitted only after actual acceptance. The inspected command
tests cover `--once`/`--resume` mutual exclusion, but not the resumed event;
the E2E Source process test checks an actual wave only.

**Consequence and disposition.** A local diagnostic timeline can report an
accepted Source wave at process resume even when no network request happened.
Keep the accepted-wave event attached to the branch that actually calls
`Refresh`, and let resume enter the selected scheduler/Wait path without
inventing an acceptance. If operators need a resume marker, choose and
document a distinct bounded event; do not relabel it as network acceptance.
Verify both command branches with one focused test and retain the existing
real-wave E2E evidence.

## F-22: Reachability's uncomposed OHTTP facade was retired

**Baseline code fact.** The installed closed resolution operation uses
`Store.PublishPrivate` and `Store.LookupPrivate`; the Endpoint verifies and
issues v3 private Descriptors. A search of non-test `cmd`/`internal` Go callers
found no caller of the formerly exported `reachability.NewGateway`, `NewRelay`, or
`OpenClient` OHTTP facade. The baseline common deadcode allowlist named 35
`internal/service/reachability` symbols across its unwired reachability and
shared-legacy groups, including these constructors, their request/response
helpers, and legacy `Store.Publish`/`Store.Lookup`. This is an exact-symbol
finding, not proof that all 20 baseline production files in the package were removable.

**Compatibility fact.** `Store.PublishPrivate` and `LookupPrivate` share the
Store's durable generation/conflict machinery. On reopen,
`store_files.go` still decodes old Descriptor records and calls legacy
`Verify`, so absence of a new legacy publication caller does not make the
old read path disposable. The current [reachability owner](../technical/private-reachability.md)
first describes v3 and then keeps an approximately 180-line generation-2
section. The [package map](package-map.md) records both generations in one
package, while the current C0 scope keeps accepted persisted identities until
an explicit migration retires them.

**Current disposition.** ADR-0091 retired the uncomposed OHTTP
Client/Relay/Gateway adapter, signed GatewayProfile codec and their exact
tests/allowances. Fifteen production files remain in Reachability. The old
Descriptor decoder and durable floor reader stay until the persisted-root
contract is decided; the current technical owner distinguishes active private
v3 from retained generation-2 reading. Removing the adapter did not authorize
discarding old stored records.

## F-23: Replacement Service Attachment loses its Route close result

**Code fact.** `service/connection.Attachment` stores `close func()`;
`closeCarrier` calls it without a result, and the default callback discards
`carrier.Close()` errors. Endpoint's `securedAttachment.close` likewise calls
`_ = attachment.transport.Close()`. The initial protected text transport has a
separate `protectedServiceTransport` wrapper: it caches its first close error,
so the text stream's later `cleanup()` can return that result even if native
Service Connection already closed it. Recovery Attachments instead receive the
fresh `textJoinedTransport` directly in
`openProtectedServiceRecoveryAttachment`; after transfer, native Connection's
void callback is its close path. `textJoinedTransport.Close` computes and
returns the joined Route/peer-cleanup result, but the callback discards it.

**Bounded consequence.** An error such as `ErrClosedJoinPeerCleanupDeadline` on
a replacement's physical close can be absent from the native Stream result
and from `textServiceStream.Close`. The exchange completion side effect
terminalizes the Context for `ErrClosedSourceCleanup`, but it does not return
other Route close failures to the Service stream. The inspected recovery tests
verify join/cancellation and clean replacement; no inspected test injects a
distinct replacement-close error and asserts it reaches the final Endpoint
`Close()` result. This is a source-level propagation gap, not a claim that
every close fails or that first-Attachment cleanup is lost.

**Existing test ownership.** `service/connection/stream_recovery_cancellation_test.go`
checks that a proposed Attachment is closed and joined on cancellation;
`stream_terminal_tail_retirement_test.go` checks authenticated tail retirement
and one close call. Neither returns a distinct close error from the callback.
`endpoint/text_service_stream_close_test.go` checks directional EOF, grace,
tail retirement and cancellation using synthetic stream barriers, without a
native replacement Attachment. The network cancellation helper in
`text_read_result_network_linux_test.go` explicitly permits
`ErrClosedJoinPeerCleanupDeadline` as a cancellation-only cause; it cannot
serve as the oracle for preserving that error on a non-cancelled final close.
The required regression evidence is a real replacement callback returning a
distinct error, a blocked ordinary close proving `Done` stays open, and a
tail close proving the late error appears in final Endpoint `Close()` without
changing the already published Application outcome. These belong to the
deterministic/race package profiles, with the installed C0 journey remaining
a separate combined acceptance gate.

**Ordering limit.** `service/connection.Stream.RunBounded` may return while its
terminal-control tail still owns the current Attachment. `textServiceStream.runNative`
then publishes `Done()` before waiting for native `stream.Done()`, and only
after that wait calls Endpoint `cleanup()`. A physical close error first observed
in that tail cannot retroactively change the already-published Application
`Outcome`. With an owned result and a real completion barrier, it could still
be returned by the final `textServiceStream.Close()`.
An earlier replacement retirement during recovery can be known before the
Application outcome, but its error also currently disappears at the void
`Attachment` callback. There is also a barrier asymmetry: the ordinary
`RunBounded` defer closes native `stream.done` immediately *before*
`stream.close()`, whereas `runTerminalTail` closes `stream.done` *after* its
`stream.close()`. Thus waiting on native `Done()` alone does not prove physical
retirement has completed for the ordinary path. This contradicts the
completion promise in `service/connection/stream_completion_linux.go`.

**Architecture decision.** Retain one exactly-once close result for every
Route transport transferred into native Connection, including replacement
Attachments. The source call graph has one non-test `NewAttachment` caller,
`endpoint/newProtectedServiceAttachment`; its current `securedAttachment.close`
discards the `net.Conn.Close` result. Make the Attachment retirement callback
error-bearing and retain each result in native Stream without turning a
physical cleanup failure into a forged authenticated Terminal. Close `Done()`
only after the ordinary path's physical retirement, matching the tail path.
Expose one post-`Done()` retirement result to Endpoint, which joins it in
`textServiceStream.Close()`; an error known before Application outcome
publication may also classify that outcome, while a late tail error must not
retroactively rewrite it. The initial protected transport can retain its
existing exactly-once wrapper; replacement Route transports already cache
their own close result in `textJoinedTransport.Close`. This is one
ownership/result seam, not a second transport runtime. Verify separately an
injected replacement-close failure during recovery and one during the
terminal tail, with expected `Done()`/`Close()` ordering, before moving
Service/Route boundaries. Keep the signed Terminal exchange unchanged.

## F-24: Command documentation omitted live routes and misstated an event

**Code fact.** `cmd/ardents-custody/command.go` dispatches both admission
Authority routes; the installed closed-network test invokes them. The command
inventory and reference omitted both routes. The inventory also omitted
`ardents accept-closed-profile`, `ardents-node hosting initialize`, the text
Application routes, and the qualification-tool routes. The command reference
placed `ardents-text link` inside the `ardents` route table and listed only
`service|name` for `verify-record`, though its parser accepts `admission` too.
In addition, `refresh-sources --resume` reads `State.Current` and still emits
`source-wave-accepted`, contrary to the reference's former fresh-acceptance
claim (F-21).

**Consequence and disposition.** Seven binaries have distinct participant,
Custody, Application, and verification roles, but the previous command map
gave an incomplete view of their actual surface. The existing command
inventory and reference now reflect the dispatchers. This is a documentation
repair, not proof that all routes are needed. A reduction decision must first
match each route to its owning C0 behavior, compatibility obligation, or
verification profile; `refresh-sources` still needs the F-21 code fix.

## F-25: Enrolled Portable readiness and protected text readiness are separate

**Contract fact.** The C0 scope requires independently authenticated first
execution followed by one installed Publisher/Reader Service journey.
`docs/reference/portable-enrollment.md` supplies an external before-execution
Portable verification procedure; the in-process Enrollment verifier cannot
authenticate its own initial executable.

**Code fact.** `ardents endpoint enroll` and `enroll-installed` enter
`runEnrolledEndpoint`. They open `internal/endpoint/portable`'s per-user root,
lease, and `probe`/`ready` Unix attachment before verifying a first unbound
artifact. They then evaluate Release and record the exact current program
before emitting `endpoint-lifecycle ready`. A routine restart authenticates
the program against that durable replacement record without requiring the
original bundle. This lifecycle owns no Network State, Route, worker or Service
Connection. In contrast, `endpoint headless` parses the v2 text plan and
starts the protected participant without calling Enrollment, Release,
replacement, or Portable in its command path.
Inside that participant, `headless-runtime-ready` is emitted after the local
Connection and Administration sockets open, before any `PublishSnapshot`.
Descriptor publication has its own worker, Introduction registration and
resolution ACK transitions. Neither ready event proves a reachable Publisher
or a completed Reader exchange.

**Evidence boundary.** The enrolled-runtime Linux process test proves initial
pin/Release acceptance, no-ready refusal, and restart after deleting the first
bundle. The installed `text-command-network` profile runs both Carriers and
real text, Custody and Node commands, but its system unit calls `endpoint
headless` directly; it does not run either enrollment route. Its independent
binary/unit SHA-256 prerequisites establish test-artifact identity, not the
selected first-execution Enrollment/Release sequence. These are two useful
proofs with a missing composition proof, not evidence that the network test
secretly bypasses its own declared fixture contract.

**Target decision.** Make one installed launch path authenticate the program
and carry the accepted current-program identity into the protected text
runtime before Service readiness. Preserve the independent first-execution
pin, existing durable replacement/restart semantics, and separate Authority
roots. If two processes remain, specify and verify the exact trust transfer
between their owners. Use one installed scenario to exercise enrollment,
Release, protected Publisher/Reader over both Carriers, stop and restart; do
not infer this joined outcome from separate green profiles. Reassess whether
the generic Portable probe attachment still has a necessary operator role
once that path is designed.

## F-26: Portable profile creates roots with no maintained consumer

**Code fact.** `internal/endpoint/portable/roots.go` creates
`ConfigHome/grants`, `StateHome/vault`, `StateHome/diagnostics`, and `CacheHome`
as part of every `portable.Open`. Its live runtime uses `StateHome/live` for
the owner lock and `RuntimeHome` for the probe socket; the enrollment command
uses `StateHome/floors/release-decision` and `StateHome/replacement` through
their own Modules. A search of non-test Go callers found no use of the four
created directories above. The direct Portable test asserts their existence,
but exercises no grant, Vault, diagnostic, or cache behavior through them.
`portable.Run` is also test-only and listed with its `emit` helper in the
common deadcode allowlist; the production adapter calls `portable.Open`.

**Boundary consequence.** Directory creation gives the generic Portable
profile apparent ownership of grant, Vault, diagnostic and cache locations
even though the selected C0 Authority Vault, permissions, diagnostics and
protected text runtime have separate owners. This is a concrete source of
surface area and misleading structure, distinct from the necessary Release
floor, replacement ledger, process lease, and any explicitly retained
first-enrollment route.

**Reduction candidate.** In the integrated launch design from F-25, account
for each directory against a real command/resource consumer. Stop creating
unconsumed directories in the bounded Portable-profile change, preserving
existing on-disk bytes and the required owner-only validation for roots that
remain. Remove the test-only `Run` facade and two deadcode allowances if the
final production caller still uses `Open` directly. Update the direct test to
assert actual owner invariants rather than the now-removed scaffold. Verify
the existing Portable and installed restart paths after the change.

## F-27: The two Endpoint launch lanes have different supervisor identities

**Selected design.** The protected text workload and Application confinement
owner select Ubuntu's systemd **system manager**. The Endpoint is the active
`ardents-endpoint.service` MainPID under its own service account; fixed worker
units are root-owned system units with `BindsTo=ardents-endpoint.service`,
separate DynamicUser identities, and a verified exact manager/unit/cgroup
relationship. That relationship is part of the worker security boundary.

**Code and evidence fact.** Both `endpoint user-unit` and
`installed-user-unit` render `systemd --user` units whose `ExecStart` is
`endpoint enroll` or `enroll-installed`. `internal/endpoint/portable` derives
per-user XDG roots and owns a generic probe socket. The installed protected
text command test itself serializes an `ardents-headless-runtime-v2` JSON plan
from test-owned roots and credentials, then writes a root-owned **system** unit
with
`User=ardents-endpoint` and `ExecStart=... endpoint headless ...`; it checks
the active InvocationID. Its recursive `Chown` gives the service account
ownership of the fixture plan directory and the `0600` runtime JSON file;
the test does not establish a production authenticity rule for that plan.
The package E2E test verifies root-owned installed
program/static enrollment bytes and exercises `enroll-installed` under an
unprivileged UID, but it does not start the protected text system unit. In the
maintained command source, `endpoint headless` only decodes a supplied v2 plan;
no production constructor for that plan or protected-text system-unit renderer
was found. The system-unit installer under `packaging/stream-qualification-worker`
serves the separate stream qualification profile. The Ubuntu package procedure
instead tells the participant to create enrollment input and render the
per-user unit. The selected C0 scope disallows operator instructions that
require editing JSON or extracting test-fixture keys.

The artifact inventories stop at different boundaries. Current
`packaging/alpha-bundle/build.sh` inventories the four headless command
binaries and static enrollment files. Its enrollment-v3 companion inventory
does not include `ardents-text` or the fixed worker units. The installed
command profile's `build-candidate.sh` separately builds five command
binaries, copies `ardents-text` as the worker, and hashes that candidate set.
At runtime `endpoint.loadInstalledWorkerArtifact` authenticates a root-owned
local six-file manifest: worker executable, two service templates, two
sockets and the stop rule. This protects the local installed set against
substitution, but no current production composition binds its manifest
identity to the accepted Enrollment/Release program and protected Endpoint
system unit. This is an artifact-authority handoff to design, not evidence
that local worker verification is absent.

**Architecture consequence.** Joining the two lanes is more than adding a
`replacement.VerifyRunning` call to Headless. The per-user enrollment unit is
not the selected system-managed Endpoint MainPID and cannot satisfy the
worker-unit binding simply by sharing executable bytes. The target installed
C0 launch must authenticate its root-owned artifact and configuration, then
start the protected participant as the exact selected system-unit identity
before any worker Grant. The Portable user-unit procedure remains a distinct
accepted operator/compatibility lane until explicitly retired; do not treat
its `ready` event as evidence of the protected installed workload. Decide the
artifact-to-system-unit trust handoff and the fate of the generic Portable
profile in the same launch-boundary design, preserving its current restart and
replacement obligations meanwhile.
The supported operator route must supply and authenticate the protected v2
plan and system unit, bind the exact accepted program/Release to their active
system-manager MainPID, and preserve restart and replacement semantics without
requiring a participant to construct fixture-derived JSON. Its installed
acceptance must exercise that route, not a test-created substitute.
It must also identify the root-installed worker manifest and its six files as
part of the accepted candidate, or establish a separately authenticated and
explicit installation handoff. A local root-owned hash manifest alone does
not state which released worker set the operator accepted.

## F-28: Offline commands inherit Route's network dependency closure

**Build fact at `53f02e64`.** A Linux/amd64, cgo-disabled
`go list -mod=readonly -deps` returned 296 packages for
`cmd/ardents-control` and 229 for `cmd/ardents-custody`. Both closures
include CIRCL blind RSA and quic-go; neither contains an OHTTP import.
The earlier `e48d4c3c` measurement of 340/292 packages including OHTTP
predated ADR-0092. These are static imports, not evidence that either
offline command executes a network operation.

**Source fact.** Control's only production use of `route/credential` is
`DecodeClosedIssuerProfile` in its public, offline issuer inspection. Custody
uses `AllocationRole`, `DecodePermissionRequest`, `Permission`, and
`EncodePermission` for offline admission allocation. The latter permission
grammar uses only the standard library; issuer-profile decoding calls State's
SPKI validator. ADR-0092 removed the OHTTP Transit client. The current
`route/credential` package still imports parent `route` in three live issuer
files; Route imports QUIC. Its shared package boundary carries those network
adapters into the offline command closures.

**Graph counterfactual.** Traversing the recorded first-party import graph
from each offline command, then suppressing one edge at a time, gives the
following reachable-package counts (including the command itself):

| Command | Platform | Current | Without `terminal -> reachability` | Without `credential -> route` | Without both |
| --- | --- | ---: | ---: | ---: | ---: |
| Control | Linux | 20 | 18 | 16 | 14 |
| Control | Windows | 19 | 17 | 16 | 14 |
| Custody | Linux | 23 | 22 | 19 | 18 |
| Custody | Windows | 22 | 21 | 19 | 18 |

This is a hypothetical edge cut, not a buildable intermediate change or a
measurement of binary size. The two cuts have no first-party closure effect
for Linux `cmd/ardents` or `cmd/ardents-node`, which reach Route and
Reachability through other live edges. It prioritizes these seams as offline
dependency cleanup rather than the critical path to installed C0 behavior.

**Target boundary.** Preserve the exact signed profile and permission bytes,
validation and independent authority checks. Put their read/encode operations
behind a cohesive offline contract only when the real caller/import seam is
designed, so Control and Custody no longer import the networked issuer
listener. Prove the dependency reduction by repeating both command
projections after that boundary change; QUIC remains necessary for the live
Node/Endpoint Route path. There is no OHTTP Transit client left to split.

## F-29: Terminal Descriptor framing imports the whole Reachability owner for one bound

**Source and build fact.** The only production import of
`service/reachability` from `route/terminal` is in `descriptor.go` and
`descriptor_client_linux.go`; both use only
`reachability.MaximumPrivateDescriptorSize` (15,000 bytes). Reachability uses
the same bound for generation-3 signed-proof issue/decode and its persisted
Store record. At `53f02e64`, Linux/amd64 cgo-disabled
`go list -mod=readonly -deps` gives Terminal 112 packages and Reachability
111; Terminal's closure contains that entire Reachability closure. Neither
closure contains OHTTP after ADR-0091. The prior `9835e225` measurement was
244/243 before that retirement. This is a static coupling observation, not
evidence that terminal operations execute unrelated dependency code. The existing
`terminal/descriptor_test.go` already imports Reachability for a boundary
proof fixture.

**Boundary decision.** Keep Reachability authoritative for the signed-proof
and Store bound. A bounded Route change can define the same 15,000-byte wire
limit locally in `terminal`, remove the production Reachability import, and
make its existing cross-owner test assert equality with
`MaximumPrivateDescriptorSize` plus accept/refuse at the exact boundary for
publication and lookup-result framing. The copied value is safe only with
that explicit contract test; wire bytes, pre-Store refusal and persistence
limits must remain unchanged. Then compare `go list` closures on Linux and
Windows. Do not create a one-constant package or move the whole Descriptor
proof/legacy codec merely to remove this edge. A future proof Module should
be justified by a cohesive behavior boundary with actual callers and tests.

## F-30: Credential's three Route imports are live issuer adapters

**Source fact at `53f02e64`.** ADR-0092 removed the uncomposed OHTTP
`client.go`/`message.go`, its Endpoint acquisition chain and the obsolete
Transit declarations from `credential/contract.go`. The package now imports
parent `route` in exactly three production files, all live closed admission
work. This does not retire the separate Route Grant verifier or persisted
local-role spend field (F-52/F-53).

`closed_token_listener.go` owns the selected shared/role Carrier accept loop,
limits, Stop and Drain; `internal/node/closed_issuer_listener.go` calls its
constructor. `closed_token_bootstrap.go` implements `*ClosedTokenIssuer`
methods using Route's bootstrap controller and ARDP; the listener calls them.
`closed_token_admitted.go` implements the ordinary class-1 admission method
called from `internal/node/closed_outer_issuer.go`. These methods also call
the issuer's exact terminal operation, so moving the files wholesale would
require a real operation/ownership split, not only new imports.

**Decision boundary.** Keep the live closed issuer root, terminal operation,
listener and admitted exchange. If separating offline permission/profile
grammar from the network-facing issuer, put Route accept/bootstrap/admission
mechanics behind a narrow Node-owned adapter only after assigning accepted
child join and late issuer-root cleanup (F-17). Moving all of `credential`
under Route as one unit would create a parent-import cycle; there is no
remaining OHTTP client lifecycle inside this package to solve.

## F-31: The test framework exists, but installed C0 is a separate verdict

**Source fact.** `tests/profiles/profiles.json` registers 21 execution
profiles and four process-suite roots. `internal/architecture/test_profiles_test.go`
checks that every `cmd`/`internal` package is in the deterministic inventory,
every `tests/e2e` package is in the process inventory, entries are current,
suite roots have one profile, and active profiles name Make targets. The
ordinary `unit` and `e2e` targets run these explicit inventories serially;
the architecture test also asserts serial process execution because those
packages share loopback resources. This is an actual test framework, so the
earlier diagnosis that tests have no common base is incorrect.

`ownership.json` checks exactly one broad work owner for every maintained
file, with five owner names and six path rules; its `network` rule covers most
`cmd`, `internal`, `docs`, packaging and test paths. That gate is useful for PR
selection and artifact lanes, but it does not identify the Module or behavior
owned by each test. At this revision 598 of 659 Go test files are colocated
with `cmd`/`internal` packages; the other 61 are under `tests/e2e` (58),
`tests/epochfixture` (one) and `tests/qualification` (two). The reconstruction
must map tests for each proposed move or retirement to the exact source
behavior and checked execution profile, rather than treating the broad
registry owner or co-location alone as proof of preserved behavior.

`make check` runs quick checks, static analysis, vulnerability and dead-code
checks, process tests, Linux package E2E, then unit race tests. It does not
invoke `text-command-network-check` or the installed worker network, recovery,
policy, lifecycle, tree, or escape targets. Those are distinct registered
profiles with dedicated Ubuntu/systemd prerequisites and exact candidate
artifacts. `docs/development/testing.md` describes this separation; it is
not an accidental omission or a passing skip. The measured Endpoint package
cost and its deliberate network waits are recorded in
`endpoint-test-cost.md` (F-15).

**Process consequence.** Report three results separately: focused owner
feedback, the exact-candidate `make check` gate, and installed C0 journey
qualification with both Carriers. Do not infer installed readiness from a
green ordinary gate, and do not run the long installed profile on each local
architecture edit. The final integration checkpoint needs a candidate/host
inventory and raw outcome for the installed profile in addition to `make
check`. Consolidate the five byte-identical Node fixture copies (F-16)
without collapsing independent State test oracles.

## F-32: Legacy Descriptor issuance and persisted decoding have different fates

**Source fact.** `internal/service/reachability/descriptor.go` implements
Descriptor v1/v2 `Issue`, `Verify` and the old wire decoder. No non-test
production caller of `reachability.Issue` was found in the maintained command
closure after ADR-0092 removed the older Endpoint Publisher/Transit chain
(F-30). This does not make the decoder dead:
`store_files.decodeStored` dispatches stored record version 1 to `decode`
and `Verify`, while `store.lookup` and `verifyStored` also use `Verify` for
legacy records. `store.compareStored` explicitly refuses implicit adoption
between private generation-3 and older Descriptor formats. The current
[private reachability owner](../technical/private-reachability.md) retains
the v1/v2 stored-record floor and refusal contract.

**Restart consequence.** `OpenStore.restore` loads every old record into the
same Target map as v3 records. `publishVerified` counts all of them against the
128-Target bound even after expiry, and `compareStored` rejects a v3 proof for
an old Target with `reachability format change requires floor adoption`.
`LookupPrivate` re-verifies the old raw bytes as v3 and makes that Target
unavailable, but does not remove its floor. The package exposes no adoption
operation. An old root can therefore both block the same Target's v3
publication and consume capacity for new Targets; it is not merely a dormant
decoder or archival artifact. The current test suite proves the reverse
private-to-legacy refusal, but the old-record-to-private restart case needs an
exact test before a migration design is accepted.

**Complete package source/test pass at `53f02e64`.** All 15 production files
and seven test files in `internal/service/reachability` were inspected. The
14 tests cover old Descriptor v1/v2 issue/verify, private v3 signature and
profile binding, revision and publication conflicts across restart, expiry,
128-Target capacity, missing initialized records, failed commits, and the
private-to-legacy downgrade refusal. The old Descriptor tests still call
`Issue` and `Store.Publish`, so they cannot simply be deleted with those
public writer APIs: first retain immutable old input bytes and assert their
restore/refusal and floor behavior. No test constructs a persisted old record,
reopens the Store, then attempts `PublishPrivate` for the same Target; no test
checks an old record occupying one of the 128 slots after expiry. The source
and test inventory establishes these missing transition proofs, not a green
runtime verdict; no test command was run for this documentation pass.

**Disposition boundary.** Split the question by operation. The old issuance
entrypoint and the callable `Store.Publish` writer are retirement candidates
after the accepted Transit/compatibility audit; the former Endpoint composition
is already removed, and the selected Node calls only `PublishPrivate`. The old
decoder, verifier, signed record types and persisted floor comparisons remain
necessary while existing v1/v2 roots must be restored or rejected correctly. A whole-file or
whole-package deletion would erase that safety behavior. If the shared
`Descriptor` type is later separated from private generation-3 facts, first
preserve exact stored bytes, signature domains, no-adoption refusal and
restart tests for both record versions; choose the API after those callers
are mapped. A supported same-Target transition needs an explicit authenticated
floor-adoption operation or a documented typed refusal and new-Target policy;
it cannot silently discard the old record, reuse its slot, or bypass its
Credential generation/expiry/conflict floor. This resolves the two
Reachability `boundary-review` entries as a compatibility split, not as a new
package request.

## F-33: Node diagnostics and resources are mostly lifecycle-owned

**Caller and state fact.** All eight Node files initially tagged
`boundary-review` have a concrete owner in the current package.
`closed_route_receiver.go` rechecks the exact accepted State profile and
projects the local receiver for admission and all five closed duties; moving
it under Route would transfer Node's State-authority decision. The
`closed_forwarding_host.go` adapter opens a `resource.Hosting` root and
coalesces samples per root across Node duties; forwarding and Data JOIN
borrow its reservation interface, and the shared Node period owns the root.
`resource_pressure.go` combines that Hosting sample with the active duty's
`Usage` and `resource.Guard` to choose protect/drain in `node.Run`.

`event_writer.go` bounds Node's event schema before a platform-specific,
deadline-aware write; `cmd/ardents-node/node_event_output.go` adds serialized
stdout and diagnostic snapshot publication. The writer belongs with the Node
event contract, while the command owns its output destinations.
`terminal_cleanup.go` is named generically but its production calls are only
from `probe_server.go`: it collects accepted connection/listener close
failures for the private probe. It is not a shared cleanup system for the
five closed duties. Moving these seven files to a generic diagnostics or
resource package would increase the API surface without transferring a
cohesive lifetime.

`clock_observation.go` is started by
`cmd/ardents-node/node_mode.go` before State opens and joined after State and
Node close. It updates one installed Contributor clock-marker file and has
no Node duty call, but the current [package map](package-map.md) explicitly
assigns the joined installed-Contributor observation to `internal/node`.
The command owns when it starts and joins; Node supplies the bounded
observation implementation. This is current documented ownership, not an
unresolved package seam. Do not merge it with Node pressure sampling merely
because both mention time. The inventory reclassifies all eight rows as
retained or deepened Node responsibilities.

## F-34: Five shared Route rules have local owners without a new package

**Caller and state fact.** `endpoint_literal.go` enforces literal IP plus
valid port for selected TCP/TLS/QUIC Carrier paths and retained Route clients;
it keeps peer selection out of ambient DNS. `closed_purpose_assignment.go`
implements the generation-3 purpose/duty table consumed by Route admission,
Node receiver checks and Endpoint Source selection. The table is one shared
predicate, but it does not itself authenticate a current State assignment.
`closed_control_capacity.go` classifies the control purposes used by both
Source and forwarding child-capacity rules.

`ClosedDutyLimits` in `closed_duty_limits.go` owns one receiver duty's
verification, channel, child and queue counters. All five Node duty starts
construct one Route governor; the counters are released by the actual Route
channel lifetime. `closed_forwarding_budget.go` transfers a bootstrap lease
to the forwarding channel and couples its queue reservations to both duty
and bootstrap budgets, rolling back the first reservation if the second
fails. These are channel lifecycle mechanics, not Node's State or spend-root
authority.

**Disposition.** Keep the literal guard, assignment predicate and control
purpose rule as shared Route primitives. Deepen the duty governor and
forwarding budget with their admitted channel owner when a real package
boundary is extracted; do not move counters into every Node duty or copy the
assignment table. The existing `wire_encoding.go` remains under separate
review: its old v2 envelope and its still-called `writeAll` operation have
different reachability, so it cannot be classified by filename alone.

## F-35: Route's `wire_encoding.go` is a v2 closure, not the v3 lane writer

**Source fact.** `route/wire_encoding.go` defines the
`ardents-interactive-route-v2` profile, envelope reader/writer helpers and
`writeAll`. Direct production callers of that `writeAll` are v2 Node binding,
Entry/Transit attachment, credential relay and Introduction control/outcome
I/O. Its envelope helpers likewise feed v2 binding, sealed Introduction and
relay codecs. A source search found no direct call from the closed-v3 files;
`route/ardp/frame.go` defines a separate private `writeAll` for v3 frames.
The current `route-refactoring-boundary.md` table had called the older helper
a "Closed lane" dependency; that reading-route entry is corrected.

The common dead-code inventory classifies many of these v2 Route leaves as
unwired historical closure and permits the `wire_encoding.go` helpers to be
production-dead on Windows while retained Linux consumers remain. This is
platform and static-call evidence, not proof that every v2 caller can be
removed: Linux still compiles the closure, and accepted wire/persisted
compatibility must be checked at the exact entrypoint before deletion.

**Disposition boundary.** Audit the v2 caller group together with its
Endpoint Transit path (F-30), retained Entry/relay readers, command routes,
test oracles and accepted compatibility record. Keep the v3 ARDP writer with
v3 framing; there is no reason to move the v2 helper into `route/ardp` merely
because both perform complete writes. The file is now `retirement-review`
in the source inventory: its v2 closure has no accepting C0 root, but its
actual compatibility obligation still must be resolved before deletion.

## F-36: Version labels conceal different active contracts and old readers

**Source fact.** The selected Node path admits `route.ClosedRouteProfile`
and starts closed duties; `internal/node/admission.go` and `duty_server.go`
recognize `route.Profile` only to refuse an old interactive Route duty.
`cmd/ardents/endpoint_headless.go` similarly refuses the v1 plan marker and
accepts only `ardents-headless-runtime-v2`. The Node Resolution operation calls
`reachability.Store.PublishPrivate` and `LookupPrivate` for the v3 Descriptor,
while `service/reachability/store_files.go` still decodes and verifies stored
v1/v2 Descriptor records on restart. `compareStored` rejects a change between
old and private formats for one Target without floor adoption. The current
protected Endpoint also uses `service/connection`'s
`ardents-interactive-route-v2` record grammar inside the closed v3 Route;
`docs/technical/endpoint-service-runtime.md` explicitly retains those bytes.
Connection v2 and Administration v1 are distinct local Application contracts.

**Consequence.** A repository-wide "keep only v3" deletion or rename would
conflate unrelated version domains and could lose durable anti-rollback floors.
Conversely, a surviving old decoder is not evidence for a second accepting C0
runtime. The useful target is one active accepting path *per boundary*, with
older input handled only by the exact retained refusal or restart reader until
its own retirement decision is accepted.

**Disposition inventory.** The [version table](c0-component-reconstruction.md#one-supported-c0-configuration)
now names the old entrypoints and separates the Node duty refusal, Endpoint
v2 Entry Attachment caller, headless-plan refusal, Reachability restart reader,
State signed-record parser, and active Service Connection bytes. This disproves
the shortcut of deleting every `v1`/`v2` symbol together. The remaining proof
is the exact caller/retained-root audit of the old Route/Transit closure and a
floor-preserving disposition for Reachability's stored old Targets. Remove an
old live writer or reader only under its accepted owner; do not alter the Service
Connection record grammar or Administration identity as part of Route cleanup.

## F-37: The generic Publisher administration chain has no production root

**Status at `53f02e64`: resolved by ADR-0092.** The call graph below is the
pre-retirement evidence that justified deleting the generic Endpoint chain.
Its named Endpoint files, Credential OHTTP client and associated deadcode
allowances are no longer present. The remaining Route v2 and historical
Grant/local-role obligations are tracked separately in F-52/F-53.

**Source fact.** A non-test caller search finds no call to
`endpoint.OpenServiceAdministration` or `endpoint.OpenPublisherIntroduction`.
`serviceAdministration.Publish` calls `endpoint.StartPublisher`, which reaches
`publisher_start.go` and `publisher_introduction.go`; the latter calls the old
`route.OpenEndpointTransitAttachment`. The separate
`endpoint.configurePublisher` to `acquireTransitCredential` path calls
`route.OpenEntryAttachment`, v2 Credential Relay I/O and
`credential.OpenClient`, but `configurePublisher` also has no non-test caller.
The deadcode allowlist explicitly includes these Endpoint and Route entrypoints.
Node's old Route profile is recognized only for refusal, while current closed
duties start from `route.ClosedRouteProfile` (F-36).

The selected text path opens `textAdministration` and calls
`PublishSnapshot`, which starts `textContext.startTextPublisher`; its bodyless
`Publish` returns an explicit error. At the inspected baseline,
`docs/technical/endpoint-service-runtime.md`
said Administration Publish dispatched generic `StartPublisher`, and the
comment on `serviceAdministration` said it was shared by the headless CLI.
The technical owner has since been corrected in this architecture worktree;
the code comment still does not describe the production call graph.

The generic Service Connection adapter is in the same closure:
`endpoint.connectAuthorized` in `service_connection.go` has only a test caller,
and its generic `acceptAuthorized` path is reached from
`publisherIntroduction.acceptApplication` through `AcceptPublisher`. That
Publisher chain has no non-test construction root above. The selected text
path instead uses `textServiceBinding`/`textServiceStream` over the current
native `service/connection` owner. Its protected TLS wrapper calls
`secureClient`/`securePublisher` from `service_tls.go`, however, so that TLS
file is shared with the live path and cannot be removed with the generic
adapter.
The file boundary is mixed: `service_administration.go` has only this generic
entry and is now a retirement-review row, but `service_runtime.go` also holds
`newEndpoint`, `Close` and Broker/publication roots used by the selected text
participant. The generic methods there can be retired only by splitting their
exclusive symbols from the current owner; deleting the file would delete the
current participant foundation.
The source inventory now marks `publisher_start.go`,
`publisher_introduction.go`, and `service_publication.go` as exact-file
retirement reviews. The latter contains generic `unpublish` and
`decodePublication`, whose only production callers are generic
`service_runtime.go` and `service_connection.go`. This does **not** include
`service_credential.go`: its `validateCredential` also serves current
`text_descriptor_publication.go`. Nor does it include `target_link.go`, whose
`TargetFromLink` serves current `text_connection_linux.go`.

**Consequence.** The generic Publisher chain is a concrete retirement
candidate, not evidence that C0 must run two Publisher/Introduction versions.
Removing its source can also shrink the v2 Route/Transit and OHTTP dependency
closure. This is static reachability evidence, not permission to delete the
entire Route codec or stored compatibility readers: shared validators,
receiver-side codecs, test vectors and accepted refusal inputs still need
their own audit.

**Disposition boundary.** The technical owner now names the selected snapshot
path in this architecture worktree. Reconcile the code and contract with the
active Endpoint agent's completed slice. Then trace the generic chain by
exact symbol and file, separate exclusive helpers from shared closed-path
consumers, and update package-map/deadcode evidence in the bounded retirement
change. Keep the selected
`PublishSnapshot` and its authorization/withdrawal behavior intact.

## F-38: The Route technical owner called a retired profile selected

**Source fact.** At the inspected baseline,
`docs/technical/network-route-node.md` contained a `Native Route profile`
section that called `ardents-interactive-route-v2` selected. Its
own `Old start retirement` section and accepted ADR-0089 require refusal of
new old Node-role starts. `docs/technical/protected-route-protocol.md` selects
`ardents-route-v3` as the one closed Route construction, and the production
Node dispatch starts closed duties for `route.ClosedRouteProfile` while
returning unavailable for `route.Profile`. The v2 binding/relay/Introduction
call graph is mapped in `route-refactoring-boundary.md`.

**Consequence.** Before the correction, reading the technical owner without
its later retirement section made the old wire look like a second supported
product path. That could misdirect file classification and prolong dead code. The
accepted ADR and actual dispatch do not support restoring an old duty.

**Disposition boundary.** The technical owner now marks v2 as retained
grammar and points to selected closed v3 in the architecture worktree. Preserve
the exact v2 identity wherever typed refusal, owned-installation evidence or
retained canonical vectors require it; remove any claim that it is an
accepting C0 Route.

## F-39: Node's common duty handle is named after the private probe

**Source fact.** `internal/node/probe_contract.go` defines `probeServer` with
`Done`, `Protect`, `Usage`, `Stop` and `Drain`. `duty_server.go` returns this
same type for all five closed duties and for the private role probe. The
issuer, forwarding, Resolution, Introduction and Data Join starters each
construct it around their own distinct listener and durable-root lifetime.
The current technical owner explicitly keeps probe implementation private to
Node; it does not define probe as the owner of those five duties.

**Consequence.** The common lifecycle handle has a misleading name. A reader
of a closed duty sees `probeServer` at its boundary and may infer shared probe
transport or identical cleanup. The resource matrix in
`repository-behavior-map.md` shows different close owners, including the
issuer's late-close gap after a Drain timeout.

**Disposition boundary.** Name this private Node return type for its actual
role, such as `dutyHandle`, in a local Node change. Keep the five duty-specific
servers and resource owners separate. This is a naming correction, not a new
package, wire identity or permission to unify their shutdown mechanics.

## F-40: Credential's mixed declaration file was resolved; root diagnostics still lag

**Source fact at `53f02e64`.** ADR-0092 removed the old OHTTP `Profile`,
`Request`, `Result`, `ClientConfig` and `Exchange` declarations from
`credential/contract.go` with their client/profile implementation. That file
now declares only `ClosedIssuerRootConfig`, `ClosedIssuerKey`,
`ClosedIssuerProfile` and `ClosedIssuerRootReceipt` for the live issuer root.
The former file-level `split-candidate` is therefore resolved and classified
`retain` in the C0 inventory. The current issuer root and ledger still call
platform-specific `issuer_root_*` helpers whose error text says "transit grant
issuer" on Linux and Windows.

**Consequence and disposition.** Do not split or delete `contract.go` on the
obsolete premise that it mixes Transit and closed-issuer contracts. Keep the
live root lock, owner-only permissions, sync and reservation ledger with their
current owner. Rename the stale diagnostic text in a bounded code change
without changing accepted bytes, admission or terminal outcomes. Historical
Grant verification and the local-role spend field are separate decisions
(F-52/F-53), not declarations in this file.

## F-41: Service Connection has one record version but two execution modes

**Source fact.** `service/connection/record.go` accepts only the v2 record
version and profile. The selected protected Endpoint calls
`NewAuthenticatedStream` and `RunBounded`; its initial Instance and Continuity
proofs finish before Application I/O. `NewAuthenticatedStream` calls
`NewStream` to initialize shared state. ADR-0092 removed the uncomposed
generic Endpoint adapter, leaving no separate production `NewStream` caller
(F-37). A non-test `cmd`/`internal` search found no call to `Stream.Run`, the
exact-count entrypoint in `stream_lifecycle.go`. However that file also owns
`establishInitialAttachment`, failure and close methods used by `RunBounded`.
The current package map explicitly says exact declared workloads remain
supported. `initial_receipt.go` says its receipt is consumed once by `Run`,
although the selected `RunBounded` calls the same establishment method.

**Consequence.** This is not a v1/v2 wire negotiation problem: the record
grammar is already singular. It is an uncomposed execution mode and a
discoverability problem. Deleting `stream_lifecycle.go` or hiding
`NewStream` would break the current protected path. Retaining the old mode
without a named caller and acceptance boundary broadens the tested API.
Separately, `stream_contract.go:Attachment` accepts a void close callback,
which is the already recorded replacement-close loss (F-23).

**Disposition boundary.** Separate the exact-count `Run` entrypoint from
shared establishment/close methods within `connection`. Reconcile the
package-map support claim with the accepted Service Connection contract and
actual C0 callers; if no independent accepted use remains, retire the old
mode instead of maintaining two execution paths. Keep `NewStream`'s shared
initializer available to the coalesced constructor, the v2 bytes and the
bounded terminal tail. Correct the one-use receipt comment with that slice.

## F-42: Old SealedIntroduction methods outlive their generic Publisher caller

**Source fact.** Committed ADR-0092 removes the generic Endpoint
Publisher/Transit chain, including its `publisher_introduction.go` caller.
In the resulting working tree, a non-test `cmd`/`internal` call search finds no
caller of `service/publication`'s v1 `IntroductionInstruction` encoder,
decoder or validator, nor of `instance.Binding.OpenIntroduction`. The latter
still decrypts the old SealedIntroduction v1 grammar inside
`instance/lifecycle.go`, a file also containing current Accept, Binding,
CommitPublished and withdrawal behavior. The selected text publication calls
`Binding.NewPrivateRecipient`; capsule opening calls
`PrivateRecipient.OpenPrivateIntroduction` under the current private v3
grammar. The old `IntroductionPublic` key is still generated for every new
Instance in `instance/state.go`, persisted with its private key, included in
the `instance/request.go` commitment, and compared with the accepted
Credential's `IntroductionHPKEPublic` in `instance/response.go`. Accepted
ADR-0034 requires that separate public key in Credential v2 for the former
SealedIntroduction recipient. The selected
private publication instead issues a fresh `Binding.NewPrivateRecipient` key.
Thus the old decryptor has no production caller after that deletion,
but the old key is still emitted in new durable and signed data. Method
reachability does not prove that this field can be removed.

**Decision boundary.** Accepted ADR-0035 explicitly specifies the v1
`ServiceIntroductionInstruction` plaintext and locates its codec and live
publication comparison in `service/publication`. ADR-0081 selects the closed
successor and retains generation-2 facts until migration; the current
Endpoint technical owner calls SealedIntroduction v1 historical. Therefore
absence of a C0 caller proves that this is not a second current path, but
does not by itself supersede the accepted historical grammar or prove that
all generation-2 readers/evidence can be discarded.

**Consequence.** After ADR-0092 integration, preserving the old v1 execution
and plaintext grammar solely because other Instance or Publication methods
are live would maintain an unnecessary alternate implementation surface. A
whole-file deletion of `instance/lifecycle.go` would remove current lifecycle
behavior, and a blind key removal could break persisted Instance validation.

**Disposition boundary.** Retire the orphaned instruction implementation only
after recording how ADR-0035's historical grammar and any generation-2
evidence remain verifiable or are explicitly superseded; do not retain a live
Publisher handler for it. Split the old decryptor out of the current Instance
lifecycle. Audit the accepted Instance request and Credential contract plus
existing roots before stopping new writes of the old key; this requires a
superseding decision for ADR-0034 and a compatible Credential/Instance
transition, not a method deletion. Give existing roots an explicit migration
or typed refusal and an exit gate for the old reader. Keep the current private
recipient, publication readiness and v3 capsule path.

**Complete package source/test pass at `53f02e64`.** All 16 production and
four test files in `internal/service/instance` were inspected. Its sole
canonical request commits the old `IntroductionPublic`; Custody's response
and the signed Service Credential must match it, while the current private
recipient is a separate volatile key. No production caller invokes
`Binding.IntroductionPublic` or `Binding.OpenIntroduction` after ADR-0092;
Route's `OpenSealedIntroductionWith` still refers to the latter through an
interface but itself has no selected non-test caller. The State decoder has
an additional retained branch: a JSON root with the current `root-v1` schema
but no `phase` rederives both public keys and the request commitment from its
private keys, then treats it as pending. None of the four package test files
exercises that phase-less reopen or a transition away from the old key. A
one-version Instance migration must classify this persisted shape as well as
the explicit pending/accepted/consumed states; changing the request field
alone would invalidate existing roots and Authority responses.

## F-43: Descriptor floors exist at two distinct authority lifetimes

**Source fact.** Node Resolution calls `reachability.Store.PublishPrivate` and
`LookupPrivate` on its durable receiving Store. That Store verifies the current
State profile and retains publication and private revision conflicts across
restart. Endpoint `textContext` calls `descriptorhistory.History.Accept` after
a resolution flight, then `Matches` before introduction and recovery. History
re-verifies the raw proof, retains at most 128 Target floors in memory, and is
cleared by `text_context_retirement.go`. It belongs to one Context; Node's Store
belongs to a receiving duty. The two maps therefore remember different
observations under different principals and lifetimes.

**Consequence.** The similar generation and revision comparisons are not
automatically duplicate storage. Merging History into Store would require
Endpoint to trust Node's local observation as its own Context authority, or
make a remote Store own Context retirement. Removing either floor solely to
reduce code can reopen stale or conflicting Descriptor acceptance.

**Disposition boundary.** Keep both owners for the selected C0 journey. A
later shared pure comparison helper is justified only if it preserves each
owner's independent input verification, profile binding, conflict retention
and retirement boundary. Do not introduce a shared store or a second accepting
Descriptor version.

## F-44: Local Node duty class is derived from the broad State assignment

**Source fact.** `node/local_roles.go:retainLocalDuty` writes the current local
`network/duty` claim using `snapshot.Assignment`. Its switch maps `rendezvous`
and `introduction` to `route-rendezvous` and `route-introduction`; those two
domains are still active in the closed profile contract. It also maps
`transit-issuance` to the same-named class. The receiving duty itself is
selected by `closedRouteReceiver` from the accepted profile's numeric role
domain and subrole. `network/duty/store.go:validClass` accepts all these
persisted class strings and `ReadConflict` can read that root. This source
pass therefore does **not** prove the first two switch arms are obsolete.

**Consequence.** Local exposure labels and actual receiver duties come from
two representations of role. A broad State assignment can classify a local
claim differently from the selected closed-profile subrole if their join is
not checked (F-45). Renaming or dropping a class before auditing persisted
readers could change conflict behavior.

**Disposition boundary.** First resolve the State assignment to closed-profile
role join in F-45. Then trace which `network/duty` readers consume each class
and the existing persisted population. Retain currently used Role Domain
labels; retire only a proven unreachable class with an explicit data gate.

## F-45: Closed profile role is not joined to the accepted State assignment

**Contract.** The current
`docs/technical/protected-route-protocol.md` says a closed-profile Node entry
must match the accepted Node Record's identity, digest, **assignment** and
duty. It defines numeric Role Domains 1-4 and subroles 1-6; subroles refine
the Role Domain rather than creating another authority.

**Source fact.** `state/epoch_materialized_snapshot.go` derives
`Snapshot.Assignment` from the authenticated Epoch's `assignedDomain` and
record family. `state/epoch_envelope.go:decodeSummaries` checks role-domain
ordering and counts but does not restrict domain strings to the four closed
profile numeric roles. `state/transit_issuance.go` binds the special
`transit-issuance` domain only for the old interactive Route profile; the
current closed issuer is Rendezvous subrole 6. `state/closed_profile.go:parseClosedProfile` verifies the signed
profile and each node's internally valid numeric domain/subrole.
`state/closed_profile_accept.go:matchesClosedProfileCandidates` joins each
profile node to an accepted record by Node ID, generation, record digest and
closed Carrier. It does not compare the node's numeric Role Domain with the
record family's assigned domain. The same join is used by
`AcceptClosedProfile` and `currentClosedProfileLocked`.
`node/closed_route_receiver.go:closedRouteReceiver` then selects a profile
entry by Node ID and duty generation, checks that its subrole permits the
requested purpose, and returns that role. Its snapshot predicate checks
generation, Network, Epoch, digest and time, but not `snapshot.Assignment` or
`AssignmentDigest`. A source search found no later role/assignment join on
this accepting path. The current `state/closed_profile_store_test.go` fixture
constructs a partial accepted `Snapshot` with no `Assignment` at all and
successfully exercises both `AcceptClosedProfile` and `CurrentClosedRoute`.
That test does not model a full accepted Epoch, but it confirms the join is
not a precondition of these profile methods.

**Consequence.** The implementation appears to admit a correctly signed
profile that gives a selected Node a different Role Domain from its
authenticated Epoch assignment, then starts the profile-selected duty. That
contradicts the current technical contract. It also makes the local duty
claim in F-44, which uses the State assignment, disagree with the receiver
role. This is a contract gap inferred from source, not a demonstrated installed
exploit or a test result.

**Disposition boundary.** The accepted closed-profile contract already fixes
the name-to-number mapping for its four Role Domains:

| Epoch assignment | Profile Role Domain |
| --- | ---: |
| `initiator` | 1 |
| `rendezvous` | 2 |
| `responder` | 3 |
| `introduction` | 4 |

`transit-issuance` belongs to the former interactive Route issuer path in
`state/transit_issuance.go`; it has no closed-v3 Role Domain. The selected
closed issuer is `rendezvous` domain 2, issuance subrole 6. In the State owner,
derive each accepted record's assignment from the verified Epoch and compare
it with every same-Node profile entry before durable profile acceptance and
again on read-back. Reject an unknown or mismatched domain for a profile
entry; Node should consume the already joined view, not invent a second
mapping. Add a full-Epoch, valid-signature mismatch case that proves refusal
before listener start. Keep signed bytes and the single closed Route version
unchanged. Whether a closed Epoch may contain an unused additional domain is
a separate contract question; the per-entry join does not require resolving
that broader restriction first.

## F-46: Node's old Route duty parsing leaves an unreachable v2 selection branch

**Source fact.** `cmd/ardents-node/node_config.go:readNodePlan` decodes five
former duty fields (`rendezvous`, `initiator`, `introduction`, `responder`,
`transit_issuer`) and calls `oldNodeDutyReservation` before reading operator
keys or constructing the State and Node roots. Any non-nil former field returns
`errOldNodeDutyRetired`. `node_duty_retirement_test.go` checks that rejection
precedes changes to the State and local-role roots, including a plan that
combines a former and a closed duty. Later in the same parser, `nativeDuty`
still includes those five fields, and its non-closed branch assigns
`state.AcceptedProfile = route.Profile` (interactive Route v2). That branch
cannot be reached through `readNodePlan`: the old fields were already
rejected. A source search found no other production assignment of the v2
profile in this Node command. The live closed duty branch chooses
`route.ClosedRouteProfile`.

**Consequence.** The command does not currently support two Node duty
versions, but its parser still reads like a version selector and carries
former duty payload types through later validation. This inflates the
apparent supported surface and obscures the single closed C0 path. The
`internal/node` checks for `route.Profile` likewise refuse rather than start
an old receiver; they are a separate API-level refusal, not proof of a live
v2 Node command.

**Disposition boundary.** Keep one effect-free, typed refusal for each old
top-level duty key while those input keys remain a declared compatibility
obligation. Remove the dominated v2 `AcceptedProfile` assignment and simplify
post-refusal `nativeDuty`/validation to the five closed reservations. After
checking exact decoder behavior and tests, reduce former payload structs to
the smallest representation needed to detect and refuse the old keys. Audit
the separate `internal/node` v2 refusal against accepted State-input
compatibility before removing it; no v2 listener, fallback, or conversion is
needed for the selected C0 configuration.

## F-47: Enrollment verifier exposes three accepted descriptor grammars to command adapters

**Source fact.** `enrollment/descriptor.go:parseDescriptor` accepts Network
enrollment v1, v2 and v3 with different companion inventories. The selected
Portable first-run `cmd/ardents` composition passes `enrollment.VerifyHeadless`;
that method requires Node and Custody companions, which `enrollment.verify`
projects only from a v3 descriptor. This makes v3 a necessary condition for
that first-run enrollment, but does not itself start the protected runtime.
The still-dispatchable `ardents endpoint enroll-installed` path instead calls
`verifyInstalledEnrollment`, which uses general `enrollment.Verify` and can
accept v1/v2 as well as v3. The read-only alpha-control inspection also calls
general `Verify`; the legacy enrollment-check command uses it too. Accepted
ADR-0042 explicitly retained v1/v2 verification for existing non-acceptance
uses; ADR-0088 retired the corpus accepting command, not that enrollment
compatibility contract. In `runEnrolledEndpoint`, this bundle verification is
reached only for an unbound program or an allowed Installed rebind. An exact
`replacement.StateCurrent` restart bypasses the bundle parser and resumes the
per-user Portable profile from its retained program record. Neither branch
starts the separate protected text `endpoint headless` runtime (F-25/F-27).
`installedEnrollmentUserUnit` still renders `ExecStart=... endpoint
enroll-installed ...`, so this is also the command selected by the maintained
per-user Installed unit generator. That unit reaches Portable `ready`, not the
protected `endpoint headless` composition or its required system-managed
Endpoint identity.
`tests/e2e/endpoint/installed_package_process_unix_test.go` constructs its
upgrade bundle with an enrollment-v1 `RELEASE` descriptor and actually runs
`endpoint enroll-installed` as an unprivileged process. This is direct test
evidence for the older installed lane, not an installed v3 protected-Service
acceptance result. The test's `ardents-alpha-enrollment-input-v1` JSON is a
separate operator-input schema.

**Consequence.** There is a v3 Portable first-run enrollment gate but not yet
one accepted descriptor grammar across every dispatchable command route. The
common verifier is a real multi-version parser, not merely an old-record refusal
reader. Calling the repository "v3 only" would hide the installed and
inspection surfaces. Conversely, removing v1/v2 from the shared parser now
would change accepted compatibility behavior in those callers.

**Disposition boundary.** Keep the Portable first-run v3 gate explicit. Audit
the actual installed route and inspection obligations against the current C0
scope and existing bundle populations, then decide whether those routes need
only v3, a typed refusal for older bundles, or a finite compatibility reader.
Put version acceptance at each command boundary rather than allowing a
general-purpose verifier to silently widen a selected path. The retained
Installed command and its per-user unit need an explicit retirement outcome
when the protected system-unit launch becomes supported; otherwise a second
maintained startup path remains despite the v3-only Portable first-run gate.
Retire v1/v2 parsing only after the ADR-0042 compatibility obligation
has a superseding decision and exact tests for affected callers. This is
separate from the historical three-argument portable input bridge, which is
an operator-input schema rather than a Network enrollment descriptor version.

## F-48: Name Record v3 survives as signed state, not an accepting Target

**Source and contract fact.** Accepted ADR-0022 selects Record v4 and calls v3
decode-only migration input. `record.EncodeRecord` and `SignRecord` emit v4;
`record.DecodeRecord` and `VerifyRecord` authenticate both v3 and v4. The
Namespace Epoch Store verifies signed records already in its current snapshot
and pending journal, so v3 can remain part of retained state. A v3 record has
no signed `RecordNotAfter`; `epoch.effectiveRecordNotAfter` marks a Target with
no such boundary unavailable, and `epoch.VerifyBinding` rejects the same
case through `VerifyLegacy`. `epoch.Store.CommitLegacy` can publish a
caller-built corpus but has no non-test `cmd`/`internal` caller in the
inspected tree; production installation uses `EpochInstallation.Commit`.

**Consequence.** This is a narrower compatibility obligation than a second
supported Name Target version: old signed state can be interpreted without
making its unbounded Target resolvable. Removing the v3 decoder immediately
would instead make retained current or pending state fail verification. The
uncalled `CommitLegacy` API is a separate fixture/evidence seam, not a reason
to keep a caller-built production publisher.

**Disposition.** Record the exact re-publication or retirement treatment of
persisted v3 Names, including pending successors and continuity, then remove
the v3 reader when the old population is closed. Move test/evidence callers
of `CommitLegacy` to an explicitly non-production construction route before
retiring that API. Preserve v4 signing and the fail-closed Target rule.

## F-49: ACA1 and ACA2 are parallel inspection contracts, not Endpoint modes

**Source and contract fact.** `cmd/ardents-control inspect-bundle` and
`inspect-transitions` call `inspection.Inspect`, which verifies an ACA1
three-component catalog through `alphacontrol.Reader`. That reader claims a
distinct root, keeps a catalog floor, and commits it before reporting accepted
inspection. `inspect-alpha-corpus` calls `inspection.VerifyACA2Corpus`, which
verifies an ACA2 four-component catalog and exact signed corpus from supplied
bytes without opening the ACA1 reader or retaining its floor. No production
Endpoint caller of `OpenReader` or `VerifyACA2Corpus` appears in the inspected
non-test call graph. The current `alpha-control-transition.md` owner calls
`inspect-transitions` read-only. Accepted ADR-0041 deliberately retained ACA1
while adding ACA2; ADR-0088 later retired corpus acceptance but retained ACA2
supplied-bytes inspection.

**Consequence.** The repository exposes two supported diagnostic input
formats, even though neither is an accepting Endpoint runtime mode. The
stateless ACA2 command cannot replace ACA1's durable anti-rollback inspection
by merely switching a version constant. Keeping both indefinitely also
conflicts with the desired single-version simplification if both diagnostics
remain in the maintained operator surface.

**Disposition boundary.** Converge on one maintained disclosure-inspection
format. Decide whether the current transition report still needs ACA1 and
whether ACA2 inspection belongs to the C0 operator surface or retained
evidence. Choose the catalog-floor owner and treatment of existing ACA1 reader
roots, then supersede ADR-0041's explicit dual-format retention before
removing the old command, reader, and parser. Keep inspection separate from
Endpoint authorization throughout.

## F-50: Closed Route profile does not pin its Epoch envelope schema

**Source fact.** `state.parseEpoch` decodes AREP schema 1, 2, or 3.
`verifyEpoch` checks the configured profile with `matchProfile` and the signed
chain, but never pairs `ardents-route-v3` with one AREP schema. The common
`verifyEpochCandidate` path then evaluates inputs, Carrier eligibility,
commitments, and materializations without another Epoch-version gate. Its
closed Carrier rule excludes old Node Record v1 identities, but does not
exclude older Epoch envelopes. `AcceptClosedProfile` checks the authenticated
profile, generation, candidate records, and signed closed profile; it also
does not inspect the Epoch envelope version. The installed closed-State
provisioning fixture and canonical qualification fixture each build AREP v3.

**Consequence.** A correctly signed and otherwise valid AREP v1/v2 envelope
can take the new closed profile through the same State candidate path, despite
the current installed fixtures using v3. This is an alternate accepted input
grammar, not a second Node duty implementation. No evidence found here makes
that cross-version pair an intentional C0 contract. A parser-only cleanup is
unsafe: `loadGenerationChain` authenticates every persisted predecessor via
the same `verifyDecision` path, so globally deleting v1/v2 decoding could
make an existing root unreopenable. This is more than an archival-reader issue:
`Open` installs the recovered current decision before starting its Source
owner; `recoverDistributionActive` can repair the current pointer from the
authenticated control floor; `recoverPendingState` installs a pending decision
which a later Source wave can promote. An older closed-profile envelope already
in one of those positions can therefore remain active or become active after
restart under the present code.

**Documentation owner gap.** The current
`docs/technical/network-route-node.md` describes the State/Route boundary and
refuses old Route execution, while
`docs/technical/protected-route-protocol.md` requires verification of the
current Epoch before accepting the closed profile. Neither specifies an AREP
envelope schema for that Epoch. The latter document's `version u16=3` at its
authenticated-profile grammar is the `ARDCPR03` profile record, not the
`AREP` Epoch byte. Thus the code's three-version intake is neither selected nor
excluded by the current technical owner. Record the sole new closed-Epoch
schema there when the retained-root disposition is decided; do not infer it
from the Route profile label alone.

There is also a fast path outside the common candidate verifier:
`verifySourceBundle` parses a fetched Epoch, then returns an exact-byte
`currentDecision` or `pendingDecision` after checking only its requested
materialization. That is correct for an already admitted decision, but it
means a future version gate in `verifyDecision` alone would still allow an
older recovered current/pending decision through the Source wave. The
`completeSourceWave` selector can stage a future result or activate a current
one. Offline `Accept` and `loadGeneration` both use `verifyDecision`, but the
former commits a new decision and the latter authenticates retained chain
members. They require different acceptance policies.

**Disposition boundary.** Select and document the sole accepted AREP schema
for new closed C0 Epochs (v3 matches current installed fixtures). Apply the
new-candidate rule to both offline `Accept` and Source-wave intake before
staging or activation. Separately define how older persisted current,
pending, and predecessor generations are authenticated and either refused
with a recoverable operator outcome or replaced by an independently signed
current successor without discarding the chain/control floor. A signed Epoch
cannot be converted by rewriting its version byte. Recovery must not expose an
old current projection or later promote an old pending decision merely because
the historical verifier accepts it. Add exact cross-version intake, current
reopen, pending recovery/promotion, and predecessor-chain evidence before
removing old decoders.

**Change boundary.** First settle the accepted closed-profile rule for AREP
v3. Put a new-candidate check before `Accept` commits and before either
Source result is counted valid, including the exact current/pending reuse
branches. Keep the historical signature/chain verifier available to load
predecessors until the root disposition is decided; after loading, classify
current and pending separately before `startSource` or a later promotion can
make them usable. Preserve the control floor and return a typed recovery or
incompatibility outcome for an old current/pending root; do not silently
clear its pointer or select a lower generation. The direct evidence matrix is
offline v1/v2 refusal without a commit, both Source result forms, old current
reopen, old pending reopen/activation, and a v3 current with authenticated old
predecessors. This is an analysis boundary, not a selected migration policy.

## F-51: The retained Namespace closure has no current command composition

**Source and contract fact.** `internal/naming/namespace` has 54 production
files in seven packages; `internal/naming/resolution` has another 15. No
non-test command file imports either tree. The Namespace subpackages do have
production importers, chiefly the uncomposed Resolution Module and Custody's
Namespace-specific operation cases. A non-test call search found no opener
of `epoch.Store` and no command construction of Custody's
`OperationSignNamespaceTransition` or
`OperationPrepareNamespaceSubmission`. The retained `ardents name resolve` and
`name control` commands refuse at dispatch; `name encode` uses the separate
canonical Naming encoder. The current Naming owner explicitly says no
maintained runtime composes the full transition or a Resolver/Gateway.
Accepted ADR-0090 nevertheless retains local Namespace lifecycle/proofs,
Custody, and existing persisted evidence after retiring the old network
adapters.

**Consequence.** Package imports inside the 69-file Namespace/Resolution
closure do not establish a C0 product journey. The Custody Vault itself also
has selected Service/Admission uses, so dropping its package would conflate
active authority operations with uncomposed Namespace cases. Conversely,
moving the whole closure into the target C0 graph would preserve code for a
producer, global close, and network route that the product has not selected.

**Disposition boundary.** Keep the 69-file closure outside the proposed C0
runtime composition. Preserve the accepted local contracts and state until
their exact retention question is decided. For a smaller maintained tree,
choose separately whether Namespace code is needed to interpret or export
existing roots, whether its Custody operation cases have a non-test caller,
and whether future Service Names will reuse these exact contracts. A
retirement would supersede ADR-0090, define persisted-root treatment, and
retain the zero-effect old-command refusal. File count alone does not grant
deletion authority.

**Separate retirement decisions.** Resolution's 15 production files implement
HTTP/OHTTP Gateway, Relay, Resolver, selection and control exchanges. No
production `cmd` or `internal` file imports this package, and its production
files perform no filesystem I/O or own a durable root. The only command-side
composition is `name_retirement_fixture_test.go`: it builds an authenticated
State and Namespace, runs the old Gateway/Relay/Resolver as a valid fixture,
then checks that the retired command does nothing. This makes Resolution the
first candidate for a superseding ADR-0090 decision: preserve the exact
effect-free command refusal and its evidence with a bounded fixture, then
remove the uncomposed transport package, tests and package-map row together.
Do not keep an unused network implementation indefinitely as the refusal
oracle or claim that its removal selects a future Service Name protocol.

Namespace has a different obligation. `epoch.Open` acquires an exclusive
filesystem-root lease and can read/write generations and a pending journal;
there is no non-test command opener, but existing roots and signed records
remain explicitly retained by ADR-0090. Decide whether those roots exist in
the supported population and choose authenticated read-only export, a bounded
migration, or typed incompatibility before deleting its Store or old Record
reader. A source search cannot establish that no user has such a root. The
active Custody Vault imports Namespace authority/record/epoch contracts for
two operation kinds, but no production command constructs those operations.
After the root/authority decision, remove only the uncalled Namespace cases
and their imports from Custody; keep the Vault's independently used Service
and Admission operations. Canonical `name encode` and the command refusal
remain regardless of these two decisions. This sequence reduces the
maintained tree toward one selected Name surface without erasing historical
authority or creating a second Name runtime.

## F-52: Route v2 execution has no caller, but shares readers with retained state

**Source fact at `53f02e64`.** The twelve Route `retirement-review` files
implement the old Entry/Transit attachment, credential relay and Introduction
I/O, plus their v2 envelope. A non-test `cmd`/`internal` search found no
external call to their exported openers, encoders, decoders or I/O methods
after ADR-0092. The remaining `route.Profile` references in Node are only the
old-duty refusal branches. The separate `route_binding_v1.go` and
`node_binding.go` are also uncalled compatibility files, but depend on
`wire_encoding.go` for v2 framing, bounds and `writeAll`. The still-retained
`route/transit_grant.go` verifies historical signed Grant v1 bytes and uses
`wireReader` from that same v2 envelope file. Its `VerifyTransitGrant` has no
non-test caller. `network/duty.SpendTransitGrant` likewise has no non-test
caller, yet `TransitGrantSpends` remains in the current durable local-role
state: Node and State still open that root, and `Replace` carries forward
unexpired spends.

**Contract fact.** ADR-0062 retains the exact Transit Grant v1 bytes and
historical State-authority-signed evidence; ADR-0092 explicitly leaves the
Route-side verifier/decoders pending a bounded closure audit. The current
Network/Node owner retains LegBinding canonical vectors pending a separate
compatibility decision. The protected closed Route uses ARDP and its own
Carrier/ALPN rather than these v2 envelopes.

**Consequence.** Old execution is unreachable, but `wire_encoding.go` is not
an isolated removable file. Deleting it would also remove the shared Grant
reader, old LegBinding framing and Node's typed old-profile refusal. Deleting
the unused Grant-spend method or JSON field without inspecting retained
local-role roots could change restart validation or erase irreversible spend
evidence. None of these historical readers authorizes a v2 accepting Route.

**Test dependency.** The live `closed_node_carrier_test.go` exercises TCP/TLS
and QUIC handshake, wrong peer, cancellation and old-profile refusal. It calls
`entryBindingCertificate` and `identifierFromKey` from the retired
EntryBinding test file, plus `identifier` from the retired LegBinding test
file. Deleting those old tests together with their implementation would break
the live Carrier test at compile time. Move these three fixture functions into
a narrowly named current Carrier test fixture file before removing the old
tests; keep the Carrier behavior checks and the exact old-profile refusal.
The remaining Route v2 tests cover old attachment/relay execution and canonical
wire vectors. Their disposition follows the corresponding historical-evidence
decision; a green run of them does not establish a current C0 Route caller.

**Disposition boundary.** First decide the retention form for historical
Grant and LegBinding evidence and the treatment of existing local-role spend
records. Then retire the unreachable attachment/relay/Introduction execution
closure with its exact tests and deadcode allowances. If Grant decoding must
remain, move only its byte cursor into the Grant owner before deleting the
v2 envelope; keep the old Node-profile refusal without an accepting v2
listener. Review `service/publication`'s orphaned v1 instruction separately.

## F-53: Grant spends are an unused operation in a live version-1 duty root

**Source fact.** `network/duty.(*store).SpendTransitGrant` and
`route.VerifyTransitGrant` have no non-test callers in current `cmd` or
`internal`. By contrast, `node/local_roles.go`, `network/state/local_roles.go`
and Endpoint qualification preflight still open the duty root. Its
`durableState` version is exactly 1 and contains `TransitGrantSpends` beside
current conflict Duties. `loadGeneration` strictly validates the whole JSON
object and content-addressed generation; `Replace` filters expired spends
into a new generation, while opening alone retains them. Watermark/current
selection can recover a retained generation after interruption.
The package-map row and technical owner already distinguish the retained field
from selected C0 behavior. `network/duty/doc.go` now states the same
distinction. No non-test admission caller remains.

**Contract and consequence.** ADR-0062 preserves the Grant v1 byte grammar
and historical signed evidence, but that does not itself require a new
receiving admission path or perpetual writes to the Node-local spend ledger.
Removing the uncalled spending operation is a code-closure decision; changing
the persisted root schema is a separate data decision. Dropping the field
from the decoder or rejecting an old generation can make a live Node/State
root fail to open even though its conflict Duties remain relevant. Ignoring
the watermark/previous generation during conversion could weaken the root's
rollback protection.

**Disposition boundary.** Keep a single current duty writer and one current
root format. Specify a bounded old-root conversion or explicit typed refusal
for existing generation chains before changing that format. Preserve duty
conflict records and generation continuity in any conversion. A historical
Grant reader, if retained for evidence, must not imply a second receiving
runtime. Expired spends can be pruned by normal `Replace`, but their possible
presence in a previously committed generation needs an explicit reader rule.

## F-54: Current documents cited accepted material outside this worktree

**Source and Git fact at `53f02e64`.** A relative Markdown-link scan of the
maintained tree found five missing local file targets. Two current owners,
`technical/private-admission.md` and `development/privacy-qualification.md`,
linked ADR-0086, which exists at accepted commit `e6168f16` on
`codex/r155-r156-design` but is not in this branch's HEAD. The research queue
linked draft R-157, present at `e0dc1e38` on the separate
`codex/issue-78-credit-record` branch. Historical R-144 linked two Entry
admission files removed by #202; both exist at `83cf491a`, before that
retirement. These were navigation gaps, not evidence that the accepted ADR,
draft research or historical source had been integrated into this worktree.

**Disposition.** The four documents now link the exact GitHub commit objects
for their out-of-tree evidence. Repeating the same local-link scan finds zero
missing file targets, and all 42 top-level product, security, technical and
development Markdown documents have at least one inbound inline Markdown
link. This limited scan does not validate heading anchors, reference-style
links, external availability, or the claims inside those documents. ADR-0086
and R-157 still need normal branch integration and current-owner reconciliation
before this worktree can treat their files as local maintained authorities;
the permalinks are provenance and navigation, not an implicit merge.

## F-55: Qualification support enters the product binary through Endpoint

**Source and build fact at `53f02e64`.** Linux `cmd/ardents` imports Endpoint,
whose production files import `internal/qualification` and
`internal/application/streamqualification`. The former has six production
files and the latter twelve. `text_job_lifecycle.go` retains an optional
`*qualification.Run`; `text_worker_launch_linux.go:launchTextWorker` calls the
shared installed-worker launcher with `run == nil`, while
`launchStreamQualificationWorker` passes the actual run from the separately
invoked qualification path. `cmd/ardents-qualification` is a real production
caller of Endpoint's exported qualification runner. The
[Endpoint design](endpoint-architecture-refactoring.md#qualification-boundary)
already records why the remaining qualification methods use private Context,
Job, worker and token state rather than forming a self-contained package.

**Consequence and disposition.** Compile-time inclusion does not make the
NET-14 workload a normal C0 Application action, and moving the six
`stream_qualification_*` files by prefix would export or duplicate Endpoint
authority. Keep one measured Endpoint execution path for qualification and
normal text work. If the product artifact must exclude qualification code,
make that an explicit artifact/build boundary with a real command caller and
the installed evidence gate; do not infer it from a package move or delete
the verification path to reduce a static dependency count.

## F-56: Contributor is an active retirement owner, not a second Node runtime

**Source fact at `53f02e64`.** `cmd/ardents-node` still calls
`contributor.Open` and `Profile.Control` for diagnose, drain, withdraw and
confirmed remove. The recognized old apply/restart command shapes refuse
before the host/systemd boundary. `Control` holds an exclusive root lease,
authenticates the installation and reconciles interrupted updates before the
selected action. Its supervisor adapter offers only Reload, Stop, Disable
and Status. Recovery may Stop an authenticated predecessor but does not
execute either generation. Existing installation records accept one
historical profile spelling and normalize reports to the canonical identity.

**Documentation gap corrected.** The Module table in
`technical/network-route-node.md` still described a pinned-bundle lifecycle
and omitted the current closed Entry set from the adjacent Entry row. Both
rows now name their actual owners and limits. The command inventory, package
map and retirement runbook already describe the narrower Contributor surface.

**Disposition.** Retain the no-start retirement owner while an already owned
installation may need authenticated drain, recovery or removal. Confirm the
actual installation population and its final retirement evidence before
removing the command, package, runbook and historical decoder together. This
is not grounds to restore Apply/Restart or to place Contributor in the new
closed Node duty composition.

## F-57: Old Portable Endpoint arguments are adapters to one runtime

**Source fact at `53f02e64`.** `cmd/ardents/endpoint.go` routes the retained
one-argument `endpoint enroll` through `runEnrolledEndpoint`, exactly as the
manifest-pinned form does, changing only the enrollment verifier callback.
`endpoint user-unit <old-input>` reads the retained JSON and renders the
current two-argument unit. The old `enrollment-check` verifies the old input
without starting an Endpoint. `docs/reference/commands.md` retains these
forms for already installed Portable user units, not as a new C0 enrollment
contract. A process test exercises restart through the old one-argument unit
after removing its original bundle; another tests the old check. Targeted
test search found pinned renderer coverage but no exact legacy renderer
command test. The rollback process test exercises the exact predecessor
program after a failed candidate; recovery classification is covered in the
replacement module, with no exact recovery command test found in that search.

**Disposition.** Maintain the one pinned argument shape for newly rendered
units and one Portable runtime. Inventory any installed v1 user units and
their migration to the pin-argument form before deleting the old parser and
dispatch routes. Their temporary acceptance is a bounded installation
compatibility obligation, not a second supported runtime version. If the
product chooses to stop accepting them sooner, specify the exact refusal and
operator recovery path in the current command owner first.

## F-58: Custody record verification has a filesystem creation side effect

**Source fact at `53f02e64`.** `ardents-custody verify-record` calls
`custody.Open` before verifying the selected record. `Open` uses `MkdirAll`
for the Vault, records and quarantine directories and
`prepareVaultLock` creates `.ardents-custody-vault.lock` when absent. The
subsequent verification authenticates a password, exact Authority binding
and current floor without advancing the floor or returning root material.
By contrast, `inspect-envelope` reads the public envelope without opening a
Vault. The command route map now distinguishes these two inspection effects.

**Consequence and disposition.** A mistyped absent `--vault-root` can leave
an empty Vault structure even though verification fails. This is a narrow
ownership and operator-clarity issue, not evidence of a second Custody
version. When the command boundary is revised, require an existing Vault
for verification or document an intentional initialization step; verify
that refusal does not create a root while preserving the current exact-record
and floor checks. Do not change all `Open` callers merely to fix this one
read-oriented command.

## F-59: Old Carrier symbols outlive their production callers

**Source fact at `53f02e64`.** `internal/route/node_carrier.go` declares the
v1 `CarrierTCP` and `CarrierQUIC` constants next to the live `CarrierProfile`
and `Carrier` types. A repository-wide Go symbol search finds no non-test
reference to either constant; only `CarrierTCP` is used by
`closed_node_carrier_test.go` to prove the current opener refuses an old
profile before dialing. `network/state/epoch_record.go` separately contains
literal v1 Carrier identities for signed-record interpretation. Removing the
Route constants would not remove that State reader or its persisted obligation.

**Disposition.** During the Carrier boundary slice, keep the live byte-lane
type and current v2 Carrier identities together. Express the negative test
with the exact old profile value and retire the two unused exported v1
symbols. Keep the State historical interpretation until its own accepted
record migration/refusal rule is resolved. This removes misleading version
surface without changing the one current dial path or old-data safety.

## F-60: Two working Node designs prescribed incompatible package outcomes

**Document and source fact at `53f02e64`.** The earlier
`node-architecture-refactoring.md` named `internal/node/forwarding` and
`internal/node/probe` as planned packages and made forwarding extraction a
completion point. The later cross-system source map found no demonstrated
independent probe boundary: four implementation files use the common duty
lifecycle. Forwarding's server owns a real cohesive lifetime, but moving it
now would transfer private `runtimeConfig`, `dutyFacts`, receiving-resource
close and common running-duty contracts. The two working designs therefore
gave a reader incompatible instructions despite neither being an accepted
runtime contract.

**Correction.** The Node plan now requires in-package navigation and
source-backed authority/outer seams first. Forwarding and probe remain
Node-owned unless a small acyclic caller API and complete resource transfer
are proved. This keeps the refactoring goal while removing package creation
as a success metric. The documentation map and current reading route point to
that reconciled status; GitHub Issues still own execution state.

## F-61: Accepted Node Carrier close results differ across five duties

**Source fact at `53f02e64`.** `node.runDuty` receives a selected duty's
`probeServer` handle. Its `Done` channel reports the accept-loop result;
`Stop` interrupts admission; `Drain` is the final join and cleanup result.
This timing matters: Forwarding, Resolution, Introduction and Data JOIN send
`Done` before their accepted workers and owned roots have finished closing.
The Issuer delegates its accept loop and child join to
`route/credential.ClosedTokenListener`; Node then closes the spend and issuer
roots after that listener drains.

| Duty | Accepted-connection close | Final joined result |
| --- | --- | --- |
| Forwarding | `serveAccepted` closes on interruption and again in its finalizer, discarding both results; a capacity refusal also discards its close result. | `Drain` retains listener, outgoing-pool, session, receiving-root and host errors, but no accepted-connection close error. |
| Issuer | Credential's direct and shared child handlers discard connection-close results. | Its `Drain` retains listener close; Node's adapter then closes spend and issuer roots. |
| Resolution | Accepted child and refusal paths discard connection-close results. | `drainErr` joins listener, Reachability Store and spend-root close. |
| Introduction | Accepted child and refusal paths discard connection-close results. | `drainErr` joins listener and spend-root close. |
| Data JOIN | `closeCarrier` joins non-benign accepted-connection close errors under a lock, including capacity refusals. | `drainErr` joins listener, child cleanup, spend root, monitor and host close. |

**Consequence.** The common `probeServer` contract already makes `Drain` the
correct place for a duty's final cleanup result; `Done` alone cannot prove
physical retirement. The accepted-connection result differs by duty even
though all five borrow the same Route listener and return a caller-owned
connection. Forwarding's outgoing pool has separate, tested physical-close
retention; those tests do not cover its accepted child. A targeted test search
found no injected accepted-connection close-error oracle for the five duty
adapters. This is an observed accounting gap, not proof that an actual installed
socket close failed or that a successful protocol exchange should be reversed.
The separate `codex/issue-285-route-opening-diagnostic` diff changes
Forwarding's outer/inner diagnostic calls but not the inspected
`serveAccepted` close sites; recheck this row when that branch is integrated.

**Disposition boundary.** Before moving the listener/outer seam, specify
which non-benign accepted-connection close failures must enter each duty's
terminal cleanup result. Keep `Done` as accept-loop observation and `Drain` as
the post-join result; do not release a spend root on a timed-out join. Use the
Data JOIN close classification as the existing local precedent, and preserve
each duty's protocol acknowledgement independently of a later cleanup error.
Cover interruption, capacity refusal and ordinary child completion with an
injected close result before treating all five ownership transfers as uniform.

## F-62: Control admissions lose Hosting release results and can outlive their shared handle

**Source fact at `53f02e64`.** `closedControlTokenVerifier` reserves shared
Hosting work and termination capacity for class-1 and class-3 admission through
`reserveClosedForwarding(config.host, ...)`. The resulting callback returns the
durable `HostingReservation.Release` result through `ClosedAdmission.Release`.
`ClosedOuterBridgeLane.Admit` checks the bound admission but does not transfer
its claim. Resolution's and Introduction's `serveAdmitted`, and Credential's
`ClosedTokenIssuer.ServeAdmittedAfterHello` used by the Node issuer, all use
`defer lease.Release()` without observing that result. Their successful
operation/reply can therefore be followed by a failed or ambiguous Hosting
release that is absent from the handler and duty `drainErr` results. This is
distinct from the accepted-socket close result in F-61. Data JOIN can transfer
its admission into `ClosedJoinPairs`, which collects release errors in its own
cleanup result; that path must not be conflated with these control admissions.

`node.Run` defers `config.host.Close()` after `runDuty` returns. Resolution and
Introduction `Drain` adapters stop waiting after their configured timeout,
while the server goroutine continues joining accepted workers and closing its
spend/Descriptor roots. The issuer listener likewise may continue joining
children after its adapter returns a timeout. Those three control-admission
callbacks use `config.host`, so a later child release after the Node returns
will meet a closed Hosting handle. The source comment in `closed_hosting.go`
that the shared period is retained until *every* child joins is therefore only
true when bounded drain completes. The resource ledger deliberately leaves
unreleased reservations charged across reopen; its restart test proves this
fail-closed accounting, not an eventual refund or terminal diagnostic.

**Disposition boundary.** Make the owner of each control-admission release
result explicit in the joined duty outcome and diagnostics. Preserve protocol
acknowledgement semantics separately from cleanup. Retain the shared Hosting
handle through a late child join where the process remains alive, or specify
an explicit fail-closed terminal/recovery outcome when that cannot be proved;
never refund an ambiguous release by retrying it blindly. Verify successful
join, injected release failure, and drain-timeout/late-child order before
changing owner lifetimes or moving packages. This is an architecture finding,
not evidence that an installed host actually exhausted its period.

## F-63: The diagnostic timeline is an input-order stream, not an event-time merger

**Source fact at `53f02e64`.** `timeline.Project` scans one input line and
writes its projected row immediately. `diagnosticTimelineRow` chooses the
event's `at` timestamp when present and falls back to the journal timestamp,
but `Project` never buffers or sorts rows. The cross-owner projection test
supplies its Node, Endpoint and Source examples in increasing timestamp order,
so it establishes schema projection and redaction, not reordering. The command
reference's `journalctl -f -o json` example provides journal input order; an
Endpoint background callback, Node process or Source process may emit after a
later-stamped event from another process has already reached that stream.

**Consequence and disposition.** The table is useful for local navigation,
but adjacent rows do not prove a happens-before relation between owners. The
current command reference already states that input order is preserved; the
working behavior map and interpretation of the cross-owner test overstated
what was proved. This is not evidence of missing events or an installed
failure. If a future
diagnosis needs event-time sorting, first choose a finite reorder window and
late-event policy; unbounded buffering would defeat the current live stream.
No new event schema or central collector follows from this finding alone.

## F-64: Alpha-control inspection commits private floors but discards three close results

**Source fact at `53f02e64`.** Both `ardents-control inspect-bundle` and
`inspect-transitions` call `inspection.Inspect` with one distinct inspection
root. It prepares `catalog`, `release` and `network` children. The ACA1
`Reader.Inspect` durably publishes an accepted catalog floor;
`verifyRelease` calls `release.Verifier.Evaluate`, which can commit verified
root/metadata floors; `verifyNetwork` can call `state.Accept` and commit an
Epoch to its separate State root. These are inspection-owned durable effects,
not Endpoint acceptance or a mutation of the live Endpoint roots.

`inspection.inspect` uses `defer reader.Close()` without checking its error;
`verifyRelease` similarly defers `verifier.Close()`, and `verifyNetwork`
defers `opened.Close()`. Each Close has a real error result from lease/root
release (State also joins background and role cleanup). A release, Network or
catalog component can therefore be reported accepted even when its close
failed. The ordinary `ardents endpoint enroll` and replacement adapters
explicitly check their Release verifier's `Close` result, so the two command
paths have different terminal-evidence standards. A focused test of
`verifyRelease` checks accepted/invalid evidence, but does not inject a
Close failure or prove the inspection terminal result.

**Contract and disposition boundary.** The technical alpha-control owner
called the declaration and transition report read-only without distinguishing
the inspector's private floor mutations. Correct that reader route. Before a
package split or ACA1/ACA2 convergence, make all three close outcomes part of
the inspection result, with an exact policy for a report whose verification
committed but whose lease release failed. Keep catalog, Release and Network
floors distinct and never imply that an inspection grants Endpoint authority.
This is source-level failure accounting, not an observed installed close error.

## F-65: Source server drops individual handler errors from observation

**Source fact at `53f02e64`.** `internal/network/source/transport.go` owns
one TLS listener and at most eight concurrent handlers. A handler returns an
error when it cannot set a deadline or write its terminal response. The server
goroutine deliberately ignores that return so a peer-controlled connection
cannot stop the shared listener. `transport_test.go` proves the handler returns
a final response-write error, but does not prove the serving owner observes
it. `network/state/server.go` supplies a resolver and active-handler count;
the command reports readiness and whole-server failure only.

**Consequence and disposition.** A failed Source response can be invisible on
the server side while the client's Source wave records its own failure. This
is a source-level observability limit, not evidence of an installed outage or
a reason to terminate the listener for one peer. If diagnosis requires it,
add a bounded, redacted outcome callback or counter at the State-owned server
boundary; record a failure class without peer address, payload or key material.
Keep the eight-handler limit and joined shutdown. Do not broaden the Source
package into candidate selection or durable State ownership.

## F-66: AAI3 package documentation still describes a removed AAI2 owner

**Source and current-owner fact at `53f02e64`.**
`internal/application/interfacev2/connection/doc.go` says AAI2 retains an
independently versioned compatibility owner and prescribes regression changes
in both owners. There is no `internal/application/interfacev1/connection`
package in the maintained tree. The current product scope and Endpoint runtime
owner say the AAI2 codec, server, client and exclusive Endpoint adapter were
removed. The AAI3 decoder and server still recognize old magic only to refuse
it before `Interface.Open`; `transport_test.go` names that effect boundary.
The separate `interfacev1/administration` package remains active, but it is
a different contract and does not own AAI2 Connection compatibility.

**Consequence and disposition.** This stale package comment made a retired
second Connection implementation look mandatory and confused the one-version
inventory. `doc.go` now describes the sole AAI3 accepting owner and its bounded
old-magic refusal. Its regression instruction covers AAI3 admission,
cancellation, half-close and terminal behavior. This correction changes no
wire, command, runtime or package behavior.

## F-67: Headless plan syntax is checked, but its installation authority is not bound

**Source fact at `53f02e64`.** `cmd/ardents/endpoint.go` dispatches `endpoint
headless <path>` directly to `runHeadlessRuntime`. `loadHeadlessRuntimePlan`
uses `decodeOperatorInput`, whose `readOperatorInput` calls `os.Open` and checks
only a byte bound, read and close results. It does not inspect plan-file
ownership, permissions, symlink status or an independently authenticated
digest. `validateHeadlessTextFields` checks referenced path strings for exact
distinctness, absolute form and canonical spelling and rejects retired v1
fields; it does not establish who supplied those strings. With a Source plan,
`headlessNetworkConfig` compares duplicate State trust anchors and clock/role
paths, but neither plan has an installation-authority proof. Without a Source
plan, the runtime takes the headless JSON trust anchors directly. No
`replacement.VerifyRunning` or enrollment/Release call occurs on this
headless dispatch path before `RunTextParticipant` opens State and Instance.

The installed command test creates `runtime.json` itself with mode 0600 and
recursively changes its directory and file to the `ardents-endpoint` UID/GID;
it then writes a temporary root-owned system unit pointing at that path. The
test proves that this consumer can run under the selected account and both
Carriers. It does not prove that a maintained installer generated an immutable
plan or joined the plan, active unit, running executable and six-file worker
manifest to one accepted release. The current Ubuntu package procedure
installs only `ardents` and static enrollment facts and renders a per-user unit;
it does not supply this protected plan/system-unit pair (F-27).

**Consequence and design boundary.** A root-owned unit file alone cannot
authenticate mutable JSON named by its `ExecStart`. The selected C0 operator
route needs one explicit installation receipt or equivalent authenticated
handoff covering exact plan bytes and Source plan, trust anchors, unit bytes
and manager identity, executing program, and worker manifest. The producer
must keep Authority Vault, Service Instance, State, token and publication
roots separately owned; it may name them, not absorb or reset them. Endpoint
must refuse a substituted, stale or mismatched input before network or worker
effects, and repeat the binding after restart or authorized replacement.
The accepted enrollment/Release contract must decide whether worker/unit
inventory is added to one release candidate or separately pinned and bound
to it; the current verifier's v3 Node/Custody companion rule does not itself
select that format. A user-editable JSON field or a second accepting startup
version is not an equivalent handoff. This is a source-level design gap, not
evidence of a completed installed attack.

## F-68: Replacement writer drops the state-root lease release result

**Source fact at `53f02e64`.** `internal/endpoint/replacement/store.go:openStore`
acquires an exclusive writer lock. Its `close` returns `releaseLock`, and the
Linux implementation joins the `flock(LOCK_UN)` and file-close results.
`Prepare` and `CommitPrepared` in `contract.go`, plus `Replace` and `Rollback`
in `operation.go`, all defer `store.close()` without joining its result to
their returned error. Those four operations can report a successful prepare,
commit or transaction even when the lease release reported failure. `close`
also clears its lock field after the attempt, so the public caller cannot
recover that result by calling it again. By contrast, `openStore` joins a
failed validation with its close result. The read-only `openReadStore` paths
also defer `close`, but they have no writer lock and its result is nil; they
are not the affected boundary.

**Contract consequence.** This does not prove a second writer acquired the
root or that a durable record was lost. It does make the reported terminal
result weaker than the replacement resource lifetime actually observed.
The foreground replacement may already have stopped/started its user unit or
committed the current pointer, so a release failure must be joined to its
operation result without rewriting the journal or implying rollback. Keep
the exact returned `Result.State`/record and attach the cleanup error; the
command must then surface the combined outcome. A focused injected
`releaseLock` failure on each writer return class, including a successful
commit and a prior operation error, should establish this before the
protected system-unit replacement handoff is built. The installed C0 path
still needs F-27/F-67 independently.

## F-69: Two frozen test-vector corpora are copied byte for byte

**Measured file fact at `53f02e64`.** A SHA-256 pass over the tracked
`testdata` paths found two exact cross-owner copies. The ten frozen Network
State hex files (`epoch`, one materialization and eight inputs) under
`cmd/ardents/testdata` equal their same-named files under
`internal/network/state/testdata`. `cmd/ardents/main_test.go` feeds the copy
to `accept-offline`; `state/golden_test.go` feeds the canonical bytes directly
to `state.Accept`. The command's expected `event.jsonl` is separate and must
stay a command result oracle. The seven files of the frozen R-049 public
Release vector under `internal/endpoint/replacement/testdata` equal those
under `internal/release/testdata`, including the explanatory README.
`release/public_vector_test.go` checks the Release Decision, while
`replacement/custody_nonmutation_test.go` uses the same vector to obtain a
real opaque authorization before checking Vault/Release nonmutation.

**Disposition.** Keep State's frozen input bytes and Release's R-049 vector
as the two canonical fixture owners. Make the command and replacement tests
read those same bytes through fixed repository-relative test paths, while
retaining their independent expected outcomes and assertions. Remove only
the 17 redundant copied paths after the affected tests and the selected
source-extraction/package profiles confirm both canonical testdata trees are
present. No new Go fixture package or regenerated expected result is needed.
This is distinct from F-16's copied Go fixture builder and does not collapse
State's independent unit oracle into a generated qualification fixture.

## F-70: Three fixture tests are outside the full ordinary gate

**Profile-source fact at `53f02e64`.** Of 659 tracked Go test files, 598 are
under the `cmd`/`internal` packages listed by
`tests/profiles/deterministic-packages.txt`; another 58 are under the seven
`tests/e2e` packages in `process-packages.txt`. The remaining three are
`tests/epochfixture/network/network_test.go` and the Linux-only
`tests/qualification/stream-network-two-host/fixturecommand/qualification-network/{main,selection}_test.go`.
`Makefile`'s `check` calls the deterministic and process inventories, but does
not call either fixture package. `internal/architecture/test_profiles_test.go`
enforces complete membership for `./cmd/...`, `./internal/...` and
`./tests/e2e/...`; it does not enumerate the two other `tests` packages.

The canonical Network fixture test verifies repeatable signed Record/Epoch
bytes and rejection of incomplete input. The qualification fixture command
tests its bounded 23-owner topology, root/plan inventory, recovery schedule
and retained Route selection. `ownership.json` maps changes under the
two-host qualification tree to both packages for selective PR checks, so
there is a real change-triggered execution route. That mapping does not make
their tests part of the full `make check` on an arbitrary integrated commit.
This is a coverage distinction, not evidence that the tests currently fail.

**Disposition.** Register the portable canonical fixture package in the
ordinary deterministic inventory. Give the Linux-only qualification fixture
command one explicit checked Linux profile or include it in a bounded Linux
fixture target invoked by the integration gate; keep non-Linux builds from
reporting a passing skip. Extend the architecture profile-membership check to
cover these selected test-only owners and their Make wiring. Retain selective
PR checks as an earlier feedback path. The exact integrated C0 verdict still
requires its separately selected installed command and worker profiles; this
fixture correction cannot substitute for them (F-31).

## F-71: Reachability restore masks an exclusive-root release failure

**Code fact at `53f02e64`.** `reachability.OpenStore` acquires the exclusive
Store lease, initializes the root, and restores all retained Descriptor
records before returning a Store. Initialization failure joins
`lease.release()` with the original error. Restoration failure instead calls
`_ = store.lease.release()` and returns only the restore error. On Linux and
other non-Windows platforms release joins `flock(LOCK_UN)` and file `Close`;
on Windows it returns `CloseHandle`. Both can fail. `Store.Close` already
returns this result on the successful-open path.

**Consequence and repair boundary.** A malformed or incompatible retained
record can therefore make startup fail while hiding whether exclusive-root
ownership was released cleanly. This matters when the one-version policy
requires an old root to be refused or migrated: the operator must see both
the exact restore refusal and a failed lease cleanup. Join the two errors on
the restoration-failure path, preserving the original reason. A focused
injected-release failure around a failing restore is sufficient behavior
evidence; this repair does not select a Descriptor migration policy or
authorize deleting its old reader (F-32).

## F-72: Instance startup failures hide exclusive-root cleanup results

**Source fact at `53f02e64`.** `instance.openPrepared` acquires the root lock,
then reads and validates durable state. A read or validation failure calls
`_ = lock.release()` and returns only the original error. After a successful
open, `Initialize` discards `root.Close()` on mismatched configuration, key
generation failure and state-write failure; `Open` does the same when the
root has no initialized state. Normal `Root.Close` returns the lock-release
result. On non-Windows platforms that result joins unlock and file-close
errors; on Windows it reports `CloseHandle` failure. The four package test
files exercise ordinary open/close and ambiguous-root refusal, but no
combined startup/cleanup failure.

**Boundary.** Preserve the exact primary refusal and join the exclusive-root
release failure on each post-acquisition startup error. This is separate from
deciding whether an old Instance root is migrated or refused (F-42), but it
must remain observable during that decision. A focused injected-release
failure on a bad retained state and one post-open error is enough to pin down
the terminal result; no package split or new runtime is needed.
