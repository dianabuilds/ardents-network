# Reconstruction findings

Status: **finding evidence and unresolved dispositions** from the
[completed repository reconstruction](https://github.com/dianabuilds/ardents-network/blob/4764ae1c567e93180f3aa643b2544abfdb2a67dc/docs/development/repository-reconstruction.md).
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

## F-12: Completed experiment sources retired to Git history

**Disposition, 2026-09-27.** At the Product Owner's request, all five remaining
experiment directories (13 files) are removed from the working tree. Four held
executed R-149 analytical models; one held R-152 contract probes. None was a
product or selected test-profile dependency. Research records retain their
results and link to exact source revision `4764ae1c567e93180f3aa643b2544abfdb2a67dc`
for reproduction. An open research question does not require keeping every
completed experiment executable in the current checkout.

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

**Realization (ADR-0113).** The retained `internal/naming/alpha` parser and
floor surface named by this finding is now deleted outright: existing corpus
and floor bytes stay byte-for-byte as inert evidence with no read path, the
refusal evidence was rebuilt with test-local historic builders and synthetic
floor-shaped bytes, and the retained-compatibility deadcode groups were
retired with the packages.

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
`dutyHandle.Drain` closes both roots only after
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

## F-39: Node's common duty handle was named after the private probe

**Source fact.** `internal/node/probe_contract.go` defines `dutyHandle` with
`Done`, `Protect`, `Usage`, `Stop` and `Drain`. `duty_server.go` returns this
same type for all five closed duties and for the private role probe. The
issuer, forwarding, Resolution, Introduction and Data Join starters each
construct it around their own distinct listener and durable-root lifetime.
The current technical owner explicitly keeps probe implementation private to
Node; it does not define probe as the owner of those five duties.

**Resolution.** The common lifecycle handle is now `dutyHandle`. The old
`probeServer` name could imply shared probe transport or identical cleanup;
the resource matrix in
`repository-behavior-map.md` shows different close owners, including the
issuer's late-close gap after a Drain timeout.

**Disposition boundary.** Keep the five duty-specific servers and resource
owners separate. The rename did not change shutdown mechanics, package
boundaries or wire identity.

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

## F-46: Node's old Route duty parsing left dominated validation

**Source fact.** `cmd/ardents-node/node_config.go:readNodePlan` decodes five
former duty fields (`rendezvous`, `initiator`, `introduction`, `responder`,
`transit_issuer`) and calls `oldNodeDutyReservation` before reading operator
keys or constructing the State and Node roots. Any non-nil former field returns
`errOldNodeDutyRetired`. `node_duty_retirement_test.go` checks that rejection
precedes changes to the State and local-role roots, including a plan that
combines a former and a closed duty. The v2 `AcceptedProfile` assignment was
already removed. Later validation still included the five rejected fields in
`nativeDuty`, resource-profile and closed-duty checks. The live closed duty
branch chooses `route.ClosedRouteProfile`.

**Consequence.** The command does not currently support two Node duty
versions, but its parser carried former duty payload types through later
validation. This inflated the apparent supported surface. The
`internal/node` checks for `route.Profile` likewise refuse rather than start
an old receiver; they are a separate API-level refusal, not proof of a live
v2 Node command.

**Disposition boundary.** Keep one effect-free, typed refusal for each old
top-level duty key while those input keys remain a declared compatibility
obligation. Post-refusal duty and validation checks now cover only the five
closed reservations. The former payload structs remain for strict decoding
of retained operator input; replacing them needs an explicit decoder-behavior
decision. Audit
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

**Realization (ADR-0112).** Version acceptance is now decided once, at the
parser every command boundary shares: `ardents-closed-alpha-enrollment-v3` is
the sole accepted Network enrollment descriptor grammar, and a recognized
v1/v2 schema is refused with the typed `ErrLegacyEnrollmentDescriptor` after
the pin and descriptor-digest checks and before any companion inventory,
executable identity, or Release input construction. The v1/v2 parsing grammar
is deleted; `Verify` and `VerifyHeadless` differ only by companion inventory
scope. Unknown schemas, including Browser enrollment-v4, keep their generic
invalid refusal. ADR-0112 supersedes ADR-0042's clause preserving v1/v2
verification for non-acceptance uses, so the "superseding decision and exact
tests" condition is met: the unit fixtures verify the canonical v3 bundle and
the typed refusal over consistent pins, the cross-platform enrollment-check
e2e and the linux installed-package e2e now drive v3 bundles, and the former
v1 upgrade vector is refusal evidence. Existing legacy bundles stay on disk
byte-for-byte with no converter or compatibility reader. The retained
`enroll-installed` command and its per-user unit keep their separate
retirement outcome, still bound to the supported protected system-unit launch
(step 6).

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

**Disposition boundary at the source baseline.** One maintained inspection
format requires an explicit choice of catalog-floor ownership and retained
root treatment. The stateless ACA2 parser cannot replace the ACA1 reader by
switching a version constant. Inspection remains separate from Endpoint
authorization.

**Decision (ADR-0110).** ACA1 is the one maintained disclosure-inspection
format. The transition report needs its current Release, Network, and
Compatibility results and its separate catalog/Release/Network inspection
floors; those roots reopen unchanged. ACA2 exists only for the retired Alpha
Corpus diagnostic and owns no floor, so no ACA2 inspection migration exists.
The accepting `inspect-alpha-corpus` command and its production verifier are
to be retired together with a before-effect refusal. The separate retained
Alpha Corpus floor reader and enrollment v2/v3 grammar are different owners;
this decision does not delete or reinterpret their bytes. The source still
contains the ACA2 command until its bounded implementation slice lands.

**Realization (ADR-0110 slice).** The bounded removal has landed:
`inspect-alpha-corpus` is a fixed retired-command refusal before parsing
arguments or opening any file, root, or floor;
`inspection.VerifyACA2Corpus`/`VerifyCorpusComponent`,
`alphacontrol.VerifyV2`/`catalog_v2.go`, the `CatalogV2` type and the
`ComponentCorpus` class are deleted with their exclusive test fixtures; the
architecture guard now asserts both corpus-command refusals and the absence
of the three ACA2 files. ACA1 inspection, its tests, and the retained Alpha
Corpus floor reader (ADR-0088 compatibility) are unchanged.

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

**Realization (ADR-0111).** ADR-0111 selected AREP v3 as the sole new closed-Epoch
intake schema and the retained-root disposition. `internal/network/state/epoch_intake.go`
adds the closed-scoped gate: `requireClosedIntakeSchema` runs after decision
verification in offline `Accept` and at the `verifySourceBundle` tail (both Source
result forms, including the exact current/pending reuse branches), refusing v1/v2 with
the typed `ErrLegacyEpochIntake` before any commit, staging, or activation;
`classifyRetainedClosedSchema` returns a `RecoveryRequiredError` for retained v1/v2
current (`loadCurrent`), recovered active (`recoverDistributionActive`, before the
floor-repair persist), and pending (`recoverPendingState`) generations at Open, with
generations, pointers, and control floors preserved byte-intact. `loadGeneration`
keeps authenticating historical predecessors until the retained population is closed.
Both technical owners now record the sole schema. Evidence: `epoch_intake_test.go`
(refusal-before-commit, retained current, retained pending, authenticated v1→v2→v3
predecessor chain with a working v3 successor, and the Source choke point covering
both result forms) over the `epoch_intake_export_test.go` durable-root seams.

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

**Realization (ADR-0113).** The canonical `name encode` encoder and the whole
`internal/naming` tree this finding left standing are now retired: all three
name verbs refuse before effects, the frozen Stage 6 wire grammar died with
its final consumer, and the zero-effect oracle survives with the historic
bytes inlined in its fixture.

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

**Source fact, rechecked after the Forwarding fix.** `node.runDuty` receives a selected duty's
`dutyHandle` handle. Its `Done` channel reports the accept-loop result;
`Stop` interrupts admission; `Drain` is the final join and cleanup result.
This timing matters: Forwarding, Resolution, Introduction and Data JOIN send
`Done` before their accepted workers and owned roots have finished closing.
The Issuer delegates its accept loop and child join to
`route/credential.ClosedTokenListener`; Node then closes the spend and issuer
roots after that listener drains.

| Duty | Accepted-connection close | Final joined result |
| --- | --- | --- |
| Forwarding | `closeAcceptedCarrier` retains non-benign close results on interruption, child completion and capacity refusal. | `Drain` joins accepted-connection close failures after handlers finish, along with listener, outgoing-pool, session, receiving-root and host errors. |
| Issuer | Credential's direct and shared child handlers retain non-benign accepted-connection close results. | The listener's `Drain` joins those results with listener close. Node uses `Joined` to release spend and issuer roots after a completed join even when physical close failed; an incomplete join retains them. |
| Resolution | `closeCarrier` retains non-benign accepted-child, direct-refusal and capacity-refusal close results. | `drainErr` joins these results with listener, Reachability Store and spend-root close. |
| Introduction | `closeCarrier` retains non-benign accepted-child, direct-refusal and capacity-refusal close results. | `drainErr` joins these results with listener and spend-root close. |
| Data JOIN | `closeCarrier` joins non-benign accepted-connection close errors under a lock, including capacity refusals. | `drainErr` joins listener, child cleanup, spend root, monitor and host close. |

**Consequence.** The common `dutyHandle` contract already makes `Drain` the
correct place for a duty's final cleanup result; `Done` alone cannot prove
physical retirement. The accepted-connection result differs by duty even
though all five borrow the same Route listener and return a caller-owned
connection. Forwarding's outgoing pool has separate physical-close retention.
Its accepted connection now has injected close-error coverage for capacity
refusal and an admitted direct child. Resolution has corresponding coverage
for capacity and direct refusals plus an admitted Node child. Introduction now
has the same three injected close-error cases. Issuer now has injected failures
for Node and direct capacity refusals and an admitted Node child. This was an observed accounting gap, not proof that an actual
installed socket close failed or that a successful protocol exchange should
be reversed. The interruption close path still needs its own injected-error
case before a shared outer owner can claim uniform coverage.

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

## Continued static audit on dev, 2026-10-02

Source: `f3ec3d01ab075e480cc22678dd768209572856be`. Ordinary Windows `go vet ./...` and the maintained `make staticcheck` passed, including the newly integrated explicit standalone Linux script checks. Ten clean worktrees of confirmed merged fixes were removed after checking tracked, untracked and ignored state; their branches remain available. The unfinished platform-admission worktree is preserved. Current Product Owner instruction caps simultaneous active threads at five and requires completed merged worktree cleanup.

### F84 — Affected Fuzz seed checks omitted by PR selector

`parseDeclarations` in `scripts/select-pr-checks.go` recognizes only `Test*` executable checks. Affected `Fuzz*` declarations therefore never enter either the direct execution pattern or the job matrix. Ordinary deterministic seed checks are skipped by the explicit `-run` filter; this does not concern mutation fuzzing. Static fixture with changed `source.Value` and dependent `FuzzValue` produced `run="^$"` instead of the affected seed check. Evidence: `C:/Users/vitek/AppData/Local/Temp/ardents-static-audit-20261002-dev/fuzz-selection/selection.log` and `matrix.json`. The maintained Epoch `FuzzCanonicalParsers` is an actual repository consumer of this check category. Recorded as [#428](https://github.com/dianabuilds/ardents-network/issues/428) and delegated immediately for repair through merge into dev and managed worktree cleanup. No product runtime tests or security analysis were run for this finding.

### F85 — PR race selection depends on map iteration order

At the same dev source, `selectChecks` skips dependency processing once a declaration is marked changed. In a mixed dependency graph, selection through a changed plain dependency can occur before another changed concurrent dependency reaches its consumer; the skipped declaration never inherits that later race flag. Twelve static selector runs over identical commits selected `TestValue` with `race=true` nine times and `race=false` three times. A simple single-chain control returned true in all twelve runs. Evidence: `C:/Users/vitek/AppData/Local/Temp/ardents-static-audit-20261002-dev/race-selection/mixed-selection-N.log` and `mixed-matrix-N.json`; runs 3, 9 and 12 omit race. Recorded and immediately delegated as [#429](https://github.com/dianabuilds/ardents-network/issues/429), with monotone race/dependency closure and deterministic order-invariance regressions requested through merge into dev.

Additional receipt: explicit diagnostic source/test bundle passed Linux vet and staticcheck; special qualification build tags passed staticcheck. An initial default-cache permission failure was an invalid execution environment, not a code finding; the corrected run selected the writable temporary staticcheck cache and passed. No product runtime tests or cybersecurity audit were performed.

Static receipt, maintained script syntax and external-command calls: Python source compilation without executing modules passed for all seven current files under packaging/scripts/tests; `bash -n` passed for seventeen shell files; PowerShell AST parsing passed for nine scripts. Static inspection of the qualification installers found their selected subprocess operations use checked exit results; no new defect is asserted from these searches. Source remains `f3ec3d01ab075e480cc22678dd768209572856be`. The two selector repairs #428 and #429 were observed active through their specific thread handles; no additional implementation thread was started in this continuation.

Static profile inventory receipt at `f3ec3d01ab075e480cc22678dd768209572856be`: compared platform-specific `go list ./cmd/... ./internal/... ./tests/epochfixture/network` with the deterministic base and selected Linux inventory for Windows/amd64 and Linux/amd64. Both comparisons had zero missing and zero stale package entries. Enumerated sixteen current files selected by `text_worker_installed`; all belong to `internal/endpoint` or `tests/e2e/node`, both explicitly included in `INSTALLED_TAG_COMPILE_PACKAGES`. This confirms the current inventory only; it does not prove the registry will detect future omissions or qualify installed execution. No new issue was inferred from these passing static checks.

Static runner selection receipt at the same dev source: loaded the maintained `run-issue60-checks.go -list` inventory on the Windows host, then used Linux `go list -json` metadata and source declarations to check every job regexp without running product tests. Matching declarations by job: fixed-worker-protocol 18; worker-command 1; endpoint-stream-ownership 2; runner-evidence 15; worker-authority 7; hosting-ledger 15; route-bounds 64; node-bounds 7; joined-carriers 3; endpoint-real-service 4. None of these ten current job selections is empty. The initial attempt to execute a cross-built Linux inventory helper on Windows was invalid and did not inspect product behavior; corrected host execution supplied the inventory before cross-platform metadata loading. Static source review confirms the runner includes command wait, stdout/stderr read, reporter, sync and close errors in its final outcome. No new issue is claimed by this receipt.

Integration receipt for F85: PR #432 merged into dev at `b2dd34df4194462f3b1e2a71b84007d23d92199b`. The repair thread reported deterministic regressions, quick/full gates and CI passing. Its managed worktree was archived; the root checkout's worktree inventory independently confirms its removal. F84 remains under integration verification and is not reported complete.

### F86 — Diagnostic timeline silently drops wrong-typed recognized fields

Static source analysis at the same dev HEAD: `diagnosticString` in `internal/diagnostics/timeline/project.go` ignores `json.Unmarshal` errors. A recognized Node lifecycle JSON event with valid time but `state:42`, `assignment:false`, or object-valued `reason` is converted into an accepted row with absent categories/reason, because empty optional strings bypass validation. A recognized Endpoint failure with object-valued `failure` similarly loses its cause. Current `docs/reference/commands.md` and `timeline/doc.go` promise errors for malformed recognized categories; optional absence and invalid presence must differ. Existing tests cover malformed JSON and invalid string categories, not wrong JSON types. Recorded as [#430](https://github.com/dianabuilds/ardents-network/issues/430) and delegated for bounded typed-field validation through merge in dev and worktree cleanup. This is a static control/data-flow finding; no runtime reproduction or cybersecurity claim is made.

Static lifecycle review receipt: inspected textdocument worker, initialization, reader and Publisher attachment ownership against the current confinement owner. `ReadWorkerConnection` closes both its Service and worker attachments, joins transfer goroutines and the cancellation callback, and retains close errors. Publisher owns a once-retained attachment close result and joins owned stream/worker I/O. Ignored callback Close return values on those paths are followed by collection of the retained owner result; no independent lost-cleanup defect is established by the search alone. The inherited production worker attachment separately retains its first close result. No generic-interface-only hypothesis was filed as a confirmed product defect, and no product runtime tests were executed.

Static diagnostic-report review: inspected `assessRunRoot`, event/sample completeness checks, comparison and `reportCommand` against the current local-diagnostics owner. Capture incompleteness, terminal count mismatches, dropped/truncated records and interrupted/timed-out outcomes are explicitly represented; successful report construction does not change a retained command-failed outcome. Comparison retains both outcomes and leaves full workload/environment equivalence unknown rather than deriving a speedup claim. The finite sampler does not append its failed process-group samples as valid observations. No new confirmed defect is established on these inspected paths. No runtime report reproduction or cybersecurity checks were performed.

Static monitor bounds review: checked CLI timeout/sample freshness limits and `openLogStore` retention validation against the current log owner. Policy restricts retained bytes to 1 GiB and files to 2..128, with positive bounded ages; Append rechecks the active segment after pruning, preserves record writes and retains sink failures. No integer-bound or stale-active-segment defect was established by this review. Future extreme sequence exhaustion explicitly fails. This receipt does not claim live delivery, rotation execution or crash durability.

Integration receipt for F86: PR #433 merged into dev at `de36a4fc402adefdc6b2738fe4e14120359deff7`, confirmed by GitHub's merged state and the root checkout fast-forward. The repair thread reported quick/full gates and CI passing. Its managed worktree was archived and is absent from the root's active worktree inventory.

### F87 — AAI3 refused setup loses returned Stream cleanup failure

Static control-flow analysis at the same dev HEAD: `openAuthorizedAttachment` in `internal/application/connection/server_admission.go` discards `stream.Close()` errors when setup cancellation/refusal wins after Open returned a Stream, and on failed deadline reset. The helper returns no attachment, so `server.handle` exits before its ordinary deferred cleanup collector is installed. Neither local setup errors nor the joined `Server.Close` result retain this failure. The maintained Endpoint `readResult.Close` returns its retained Service/worker failure after joining work; a cancellation racing handoff therefore has a real producer for the lost result. Existing public server cleanup regression covers successful handoff only. This differs from #72/#75's Route/Node reservation release. Recorded as [#434](https://github.com/dianabuilds/ardents-network/issues/434) and immediately delegated to repair thread `01a0fb50-fda5-7e21-a98d-f81f2ac2e639` through merge into dev and worktree archive. The root audit used source reasoning only; runtime regression remains repair acceptance, not an already claimed reproduction.

Integration receipt for F84: GitHub confirms PR #431 merged into dev at `a2918a8c691522e903a2f4bb48917f355fb6554b`; the root checkout was fast-forwarded to that commit. The implementer reported quick/full gates, Ubuntu CI and merged-tree verification passing. Its managed worktree was archived and is absent from the active worktree inventory. The only newly dispatched repair in this continuation is #434; the audit plus that repair remain below the Product Owner's five-thread limit.

### F88 — Shared Hosting cache extends observation freshness and period validity

Static review at dev `a2918a8c691522e903a2f4bb48917f355fb6554b`: `internal/node/hosting/ledger.go` measures cache age from request `cachedAt`, not underlying `HostingSample.At`. Concrete `resource.Hosting.Sample` can reuse a committed observation already up to one second old. A cache miss at t0+900ms can therefore return At=t0, and a hit at t0+1800ms returns the same 1.8-second-old observation for a one-second request. Cached pressure also retains `Drain=false` across the provider period End until this outer TTL expires, whereas the resource owner reevaluates the period at the current time. Node, forwarding and JOIN are real one-second consumers of this adapter. Recorded as [#435](https://github.com/dianabuilds/ardents-network/issues/435) and immediately assigned to repair thread `01a0fb56-9874-7253-aa56-2d4763a140b7`. Required repair covers actual observation freshness, period crossing and each joined caller's age bound while retaining coalescing and reserve invalidation. Root and read-only reviewer performed source reasoning only; no runtime or cybersecurity reproduction is asserted.

Read-only refresh receipt: inspected actual Publisher start, verified first ACK, PairLifecycle switch, scheduler and withdrawal/retirement ownership. Pending replacement does not become accepting before verified ACK, exact retries preserve the first ACK and cannot extend predecessor retention, and cancellation retains cleanup ownership of both current and pending registration through withdrawal and publisher owner Close. No separate lost-registration or lost-refresh-cleanup finding was established.

Worktree cleanup receipt: verified that the completed monitoring worktree was clean including ignored and untracked paths, its checked commit `e272fd1ee0ca05cc075cbd528c5c09c5a4479ecd` is an ancestor of dev, and its earlier thread turn is terminal after PR #412 integration. Removed only that managed worktree; retained its branch/commits and all unrelated worktrees. The new AAI3 and Hosting repairs use separate worktrees and code owners.

### F89 — NET-32 proportional work reservation overflows

At dev `a2918a8c691522e903a2f4bb48917f355fb6554b`, `uint64(window)*1_000_000_000/uint64(24*time.Hour)+1` in the maintained idle runner overflows for its actual fixed ten-minute input. Independent exact BigInteger arithmetic yields 6,944,445 bytes per direction, whereas modulo-2^64 arithmetic yields 112,318. The real `net32-idle` path passes that underestimated work reservation to Hosting before admission; termination remains separately reserved. Registered as [#436](https://github.com/dianabuilds/ardents-network/issues/436), immediately assigned to thread `01a0fb5d-a826-7753-9f91-023ec6e8b4fc` for bounded arithmetic repair through merge and archive. Pure arithmetic/source evidence is not a runtime test claim.

### F90 — NET-32 projection denominator uses wall time

The same current idle runner computes MeasuredDuration from first/last durable UTC HostingSample.At, despite having a monotonic origin for event elapsed values. A forward wall step inflates the denominator: 7,000,000 aggregate bytes in actual 600 seconds should project to 1,008,000,000 bytes and fail, but a wall duration of 660 seconds yields 916,363,637 and passes. NET-14AI requires one host's monotonic elapsed boundaries, and the current qualification owner retains conservative short NET-32 projection. Registered as [#437](https://github.com/dianabuilds/ardents-network/issues/437), handed to thread `01a0fb5e-b67d-7bf3-bdde-92b4006e41ff`. Its first read-only preparation is complete; it has no worktree/branch/edits and explicitly awaits #436 integration before implementation to prevent simultaneous ownership of the same file. Runtime clock reproduction is not asserted by this static receipt.

### F91 — NET-14V verdict replaces directional p95 with whole-run mean

At the same dev source, evaluateNET14V calls relayDirectionalBitrateCriteria with raw link caps 20/100 Mbit/s for Reader and 100/100 for Publisher. The helper divides total attributable endpoint bytes by the whole run duration. The current NET-14V requires p95 one-second per-direction bitrate <= min(25 Mbit/s, 80% of declared usable budget), which is 16/25 and 25/25 under the current manifest. Constant 18 Mbit/s Reader tx therefore passes the implemented 20 Mbit/s mean test while violating the required 16 Mbit/s p95 test; sparse bursts can also be hidden by averaging. Real verify-net14v command dispatch reaches this calculator; one-second observations exist. Registered as [#438](https://github.com/dianabuilds/ardents-network/issues/438), immediately assigned to thread `01a0fb61-8d6c-7e73-9646-07eeb57e24df` for the actual verdict/observation repair. It owns NET-14V only, without simultaneous NET-32 file edits.

Static State receipt: read-only review found no new confirmed defect in inspected Refresh ownership, exclusion of Accept while refresh owns completion, exact pending reuse/conflict, completion-time bounds, or automatic/background failure collection. Earlier F19/F20/F21/F45/F50 are not reopened as new issues. This does not claim complete State correctness or runtime qualification.

Coordination receipt: #437 finished read-only preparation and is idle awaiting #436; no branch was created for it. Root plus active repairs #434, #435, #436 and #438 total five. Inspected current working diffs of #434 and #435 have no common file. Explicit messages separate #436's NET-32 documentation from #438's impairment/recovery documentation, and require current-dev overlap verification before integration. Both new read-only audits finished without edits before these repair slots were filled.

### Coordination verification — 2026-10-02, repairs #434–#438

Root verified live thread handles and current repository state; dev remains a2918a8c691522e903a2f4bb48917f355fb6554b. Four repair implementers are active; #437 is idle with no implementation/worktree and still depends on integrated #436. Root plus implementers occupies the authorized five active tasks.

GitHub checks: PR #440 has 40 successful checks and two skipped checks; PR #441 has five successful checks and two skipped checks. Both remain open and require their implementers' complete local gate before integration. PR #439 has 37 successful, two failed and two skipped checks. Its implementer is comparing TestTextIntroductionDeliversFourConcurrentReaders against unchanged base; failure is retained, not waived by a retry. No integration or worktree removal is claimed from green CI alone.

#438 requires two bounded read-only reviewers under its selected implementation workflow. Root instructed the implementer to finish gates and prepare the PR while waiting for capacity, then use reviewers sequentially after a repair completes. #437 remains prepared during that review scheduling window. Any independent baseline failure is to return with a precise trigger and owning source for a separate issue.

### Follow-up static trace — qualification clock ownership

Inspected current dev resource_verdict_linux.go: evaluateOwnerNetwork discards resourceObservation.at while building HostingSample series; directionalCarrierP95 uses HostingSample.At differences for rates. NET-14AI requires monotonic elapsed KPIs. Sent this trace to #438 implementer before filing any duplicate. Its current worktree net14v_bitrate_linux.go computes NET-14V bitrate independently from relay samples' monotonic Elapsed values, so the endpoint diagnostic percentile is not automatically the accepted NET-14V gate. Remaining source/node resource duration consumers require current caller/owner verification before a separate issue; no new confirmed defect is claimed here.

#438 caller verification: current repair worktree net14v_verdict_linux.go:133 and paired_evidence_linux.go:161 both call net14vDirectionalCriteria. paired_evidence_linux.go:120 independently checks report monotonic elapsed span and exact MeasuredDuration reconciliation. The NET-14V gate does not use the old Hosting diagnostic TxP95/RxP95. This is source inspection, not proof of completed checks or integration.

### Static candidate — first-participant-only resource verdict

Current main.go:169 computes whole-owner resourceCriteria from completed[0].measurements and completed[0].report only, whereas evaluateOwnerNetwork encloses all participant reports. The User plan requires four Readers (plan_linux.go). Each RunScenario owns an independent origin and sampling loop; scenario_linux.go stops that loop and takes its final sample before Measurements.Finish waits for all participants. Finish preserves workers until completion, but does not extend the first participant's sampling window. Therefore the existing completion barrier alone does not prove full-window RSS/CPU coverage. Verify the accepted combined-workload window and real Reader start/end synchronization before promoting this candidate to an issue. No runtime reproduction or accepted defect claim yet.

Resource-window candidate refinement: runReader has independent phase-shifted opening/setup, and RunConnections assigns Started locally after frameOpen emission. No cohort start barrier appears in the inspected caller path. However worker.go delays receiver EOF until its complete workload is ready and ten minutes elapse; this can synchronize completion indirectly. EvaluatePairedConditionWorkload validates per-reader work and peer counters but contains no explicit common-window resource check. Before issuing a repair, account for that EOF mechanism and demonstrate an admitted complete run where the first Reader's resource window differs materially from the required aggregate window. This prevents treating asynchronous setup alone as a proved failure.

### Verified integration — #434 and #436

GitHub PR #440 MERGED at 2026-10-02T07:08:22Z, merge 64e5caf39a6d462b837ddf248fe5f25109d8fc05; PR #441 MERGED at 07:08:48Z, merge d14436c23601cc8c464ec7f7d105ca0c90772f5b. Root fetched dev, verified both commits are ancestors of origin/dev and fast-forwarded its dev checkout to d14436c23. The local audit report was preserved. Attachment-close-errors worktree is absent from current git worktree inventory; NET32 worktree still present while its implementer completes cleanup. New unrelated untracked service-boundary-proposal.md was observed and preserved without edits. Reviewer slots are released only when the corresponding live repair turn becomes terminal, not solely on PR merge.

### Static progress-verdict inspection

Reviewed workload_verdict.go and bridge measurements at current dev d14436c23. Bridge records Tx/Rx last-event offsets and maximum gaps with local time.Now values retaining monotonic readings; verdict uses MeasuredDuration minus the matching direction's elapsed last-event offset. Actual source therefore does not substantiate a wall-clock gap defect in this path. Existing tests cover lost streams, duplicate IDs, short duration, stalled tail and counter truncation. No new issue from this inspection. #437 resumed after both preceding repair turns became terminal; one shared reviewer slot is reserved for sequential #438 reviews. #437 was explicitly told to coordinate its own review instead of competing for that slot.

### F92 — NET-14V per-episode endpoint overhead checked on failed segment only

Confirmed by static contract/caller trace at dev d14436c23. NET-14V limits combined endpoint additional carrier bytes separately for each recovery episode and forbids compensation by quiet episodes. evaluateNET14V checks endpoint bytes only at whole-set len(Failures)*8 MiB; relayEpisodeCriteria checks only failure.SegmentID per episode. Two episodes with +10 MiB endpoint traffic in the first, +0 in the second and zero extra on the failed internal segment pass those inequalities while violating the first episode's endpoint bound. evaluateFailedNET14V also uses only the failed segment. Registered as issue #442 https://github.com/dianabuilds/ardents-network/issues/442. Separate from #438 p95 gate; implementation awaits its integration because of shared net14v_verdict source ownership. Prepared brief outside Git: C:/Users/vitek/AppData/Local/Temp/ardents-net14v-episode-accounting-issue.md. Runtime regression belongs to assigned implementer, not this static receipt.

F92 counterexample correction: the actual recovery manifest requires at least three failures, so the admitted example uses three sequential episodes: +10 MiB, +0, +0 endpoint traffic, with zero extra on the first failed internal segment. Total 10 MiB <=24 MiB and segment-local checks still pass. The earlier two-episode algebra illustrated the inequality but was not an admitted manifest. Issue #442 and its external brief were corrected to the three-episode trigger.

Root independently verified PR #439 MERGED a3963544a4c305e14059c7b744b8e68cfbb41e5d, ancestor of fetched origin/dev, and fast-forwarded root dev to that commit. Hosting-freshness worktree is absent from git inventory; unrelated root report/proposal preserved.

### Static candidate — fault schedule is not bound to workload origin

At dev a3963544a, verifyRecoveryFaultEvidence compares ScheduledMillis-AtMillis origins only among fault records and checks positive origin, duration and <=1500ms schedule deviation. It receives no workload report/window. verifyNET14V invokes it without episode verdict binding, while relayEpisodeCriteria uses ReaderNetwork.Started+manifest offsets and ignores ActualMillis records. Shifting every scheduled and actual fault timestamp by one hour leaves that schedule validator's comparisons unchanged while byte-accounting windows stay fixed. recovery_faults.py accepts a positive started_millis argument; inspected script paths expose no additional validator binding. Asked #442 read-only preparer to check any remaining caller binding and report this separately rather than enlarge implementation silently. No runtime or complete CLI reproduction claimed; mathematical validator invariance is statically visible.

F92 assigned to prepared repair thread 01a0fb78-1dc8-73a2-839b-c170110933de. Read-only preparation permitted; no branch/worktree/implementation before #438 integration. Current active slots remain root, #437, #438, its single reviewer, and #442 preparation (maximum five).

### F93 — NET-14V fault schedule not bound to measured workload

Confirmed static validator invariance on successful verify-net14v path at dev a3963544a. verifyRecoveryFaultEvidence derives origin only from ScheduledMillis-AtMillis across records; no workload window input exists. Translating all scheduled/actual fault records +3,600,000 ms leaves every schedule criterion and all other successful verifier inputs unchanged. relayEpisodeCriteria ignores actual fault records, so faults outside the run can supply accepted recovery evidence. NET-14T requires the failures during the measured run; NET-14AH binds declared scenarios. Registered issue #444 https://github.com/dianabuilds/ardents-network/issues/444. Failed verifier has a different ActualMillis byte-window consumer and is not claimed to share this complete counterexample. Prepared external brief C:/Users/vitek/AppData/Local/Temp/ardents-net14v-schedule-binding-issue.md. Coordinate serial ownership with #442 and predecessor #438 PR443; no additional active implementer at capacity.

### Owner container cleanup instruction

Product Owner explicitly requires agents to delete containers they no longer use. Sent to active #437, #438 and prepared #442; applies to future repair prompts. Retain logs/results/source/exit status outside the container, then remove the exact owned container after its process finishes; use --rm for disposable runs when evidence survives externally. No global prune, shared image/volume/cache deletion or interruption of another active gate. Current authenticated Docker inventory contains only three Ardents containers, all running: ardents-437-gates, ardents-net14v-final-linux-check, ardents-net14v-linux-check. Stopped Ardents containers shown in the earlier screenshot are already absent; unrelated stopped swarm-zulip containers preserved. #438 asked to verify whether its older linux-check still has a live required gate before removal.

F93 assigned to read-only prepared thread 01a0fb7c-db38-7521-8231-5089c9970577. No implementation branch/worktree before explicit coordinator resume. Selected serial integration order: #438 directional p95/raw relay evidence -> #444 actual fault schedule/workload binding -> #442 per-episode combined endpoint byte accounting. This avoids concurrent edits of successful and failed recovery verifier owners. Both prepared agents retain separate acceptance boundaries and must remove their own unused containers after retaining evidence.

#438 implementer reported both bounded reviews terminal: Standards no findings; Spec cadence finding corrected and rechecked, no remaining findings. The superseded Linux container's logs and explicit incomplete cancellation evidence were retained before its removal; the final Linux gate still runs. Sequential reviewer slot handed to #437 (Standards then Spec, one concurrently). #438 final source-bound native/Linux gates and final CI remain pending; no merge completion claimed.

### First-participant resource candidate — narrowed by final verifier evidence

Follow-up at dev a3963544a found mandatory final whole-owner slice checks: verifyPair -> readNodeResults -> evaluateOwnerSlices(inputSet.OwnerSlices, owners). evaluateOwnerSliceWindow receives owner.Started/Stopped from evaluateOwnerNetwork, which encloses every participant's report. Final paired criteria require whole-owner slice RSS/CPU and the exact owner set, and issue60Evidence P8 includes those gates. Consequently the first-participant-only local resource sampling does not by itself establish an accepted final resource escape; the previously recorded candidate is not promoted to a bug. These external slice calculations still use serialized wall timestamps and need separate NET-14AI clock verification, not a claim that the full resource implementation is correct.

#444 read-only preparation is complete/idle. Its agent independently confirms successful verifier translation invariance, identifies missing host/run identity in start/stop producer records, and plans bounded producer/verifier repair after #438. No worktree/branch/runtime activity from the prepared thread.

### F94 — external owner CPU qualification uses UTC duration

Confirmed static trace: node_owner_samples.py and nodeOwnerSampleInput retain no monotonic elapsed position. evaluateOwnerSliceWindow and maintained Node/Source CPU window consumers divide cumulative CPUUsageNSec by serialized At duration, contrary to NET-14AI. 300e9 ns CPU over600 real seconds yields50%; wall span606s with601 samples spaced1.01s yields49.504950495%, while existing <=1.5s gap check passes. This is an incorrect KPI; no live quota escape or runtime reproduction is claimed. Brief retained outside Git as ardents-owner-cpu-monotonic-issue.md. Separate repair must cover real producer/collection and maintained consumers, without changing quotas or claiming memory percentile failure.

### F95 — whole-owner sampler refuses real orchestrator argument

At dev a3963544a, run-windows.ps1 Configure-OwnerSlices sets ardents-qualification-owner.slice and passes it to node_owner_samples.py. Its exact name validator accepts only the selected Node and Source service names, so this mandatory slice sampler exits before collecting counters. Final verifyPair requires complete both-host whole-owner slice evidence. Static caller/argument mismatch recorded as #446 https://github.com/dianabuilds/ardents-network/issues/446. Separate from #445 CPU arithmetic; serialize sampler admission repair after #445 integration. Brief outside Git ardents-owner-slice-sampler-issue.md. No live product run claimed.

#445/#446 dependency clarification: merge the checked monotonic component #445 first while keeping issue #445 open for complete owner-slice producer acceptance. Resume #446 only after that verified integration; it repairs exact slice-name admission. Verify the integrated real slice producer/collector/consumer path after #446, then close #445 if all acceptance is proven. No gate waiver, selector repair folded into #445, or circular requirement to close #445 before #446 can begin. Component integration and full issue acceptance remain distinct.

### Static sampler collection inspection

PowerShell Parser.ParseFile accepted current run-windows.ps1 with no syntax errors. Inspected Stop-OwnerSlices, Stop-StateSources and Stop-RouteNodes: collectors parse full JSON sample objects after field-presence filters; they do not reconstruct a fixed record that would automatically drop an added Elapsed field. Malformed-line omission alone is not filed as a bug: final consumers require bounded complete sample windows and reject insufficient/gapped evidence. Sampler terminal status handling needs requirement-specific proof before an additional finding. #446 preparation terminal/idle independently confirms the real slice-selector mismatch. Sequential reviewer slot released by #437 and allocated to #445 when its completed delta is ready; prepared #442/#444/#446 do not implement concurrently.

### Static candidate — Carrier retirement holds global pool mutex

Current carrier/closed_carrier_pool.go Release (last unused lease) and Invalidate delete the exact entry and call Carrier.Close while retaining pool.mu. AcquireContext first checks ctx then waits for the same mutex; an unrelated ready pair cannot proceed until physical retirement returns. Current protected-route-protocol owner explicitly preserves unrelated-pair progress during pair operations. Reap and openForPair already represent retirement through per-pair operations outside the global mutex. Next required proof: trace maintained caller to these two paths and concrete Close implementation delay/join before filing; bounded generic slow-close fixture alone is insufficient to establish product impact.

Carrier-pool candidate refinement: maintained forwarding/link.go acquires the pool lease and releases unused leases when session acquisition or attachment fails. Physical TCP retirement closes NetConn directly and does not perform TLS notification; that invalidates an assumption of a mandatory slow TLS close. QUIC CloseWithError in local quic-go v0.62.0 source signals local close and waits on its connection context; thus physical close can join asynchronous work, but a sustained product blockage remains unproven. Invalidate currently has no non-test caller. No new bug filed from global mutex placement alone.

#437 reported a superseded invalid-environment gate still running alongside the corrected gate. Root instructed it to retain original failure/incomplete-cancellation evidence and stop only its exact obsolete owned process/container, preserving the correct selected 15-minute Endpoint gate. This avoids consuming resources for a run already unable to establish acceptance.

#437 obsolete gate terminal receipt: agent reports session85036 exit1 (inner make2), exact owned obsolete test descendants terminated, failure/stdout and explicit superseded/incomplete-cancellation record retained. Correct private-cache gate still active under the unchanged selected 15-minute Endpoint timeout. Carrier bootstrap queue/control paths inspected; shared finite bootstrap quota alone is not filed as a separate reservation defect without a valid complete charging/termination trace. No speculative issue generated.

### Static outer-lane lifecycle inspection

Reviewed ClosedOuterBridgeLane Read/Write/CloseWithStatus/deadline updates and retiredFrame. Waiting for outbound credit releases lane.mu, waits on per-lane notification/deadline, and does not own shared writer; retirement signals outbound waiters and clears input. SetWriteDeadline releases deadlineMu before acquiring lane.mu, so the inspected Write lane.mu->deadlineMu sequence does not establish lock inversion. Late frames are restricted to valid BYTES/CREDIT/EOF/CLOSE of retained exact retired IDs. Existing source tests cover credit cancellation and sibling progress; root did not run runtime tests. No additional confirmed bug from this inspected lifecycle.

### Static outer writer serialization candidate — pending localization

Inspected internal/node/outer/writer.go write/drain/update and its real caller ClosedOuterBridgeLane.Write in internal/route/closed_outer_write.go. write enqueues then waits only on request.done. drain examines the live deadline only after acquiring physical serialization. update changes the physical deadline only for the active matching lane; SetWriteDeadline of a queued sibling signals its lane credit waiter but not request.done. Consequently, an already queued sibling whose deadline expires can remain blocked until the active different-lane physical frame completes. Existing TestClosedOuterExpiredQueuedWritePreservesSibling releases serialization manually before asserting timeout and does not test prompt return while a healthy sibling remains active. No runtime test was run by the static auditor. Before dispatch, locate the exact outer-lane deadline contract/current authority and distinguish the retained lower-writer requirement in protected-route-protocol from the outer writer, then check existing issues to avoid duplicating a prior repair. Preserve sibling framing and no-output cancellation in any proposed fix.

## F-96: Outer writer queue does not observe a waiting lane deadline

Static localization at dev a3963544a4c305e14059c7b744b8e68cfbb41e5d: node/outer.writer.write waits exclusively for request.done after enqueue; drain only checks current deadline after physical serialization, and update ignores non-active sibling requests. Real ClosedOuterBridgeLane.Write calls that writer with available credit, while SetWriteDeadline wakes only its credit notification and updates only active matching output. A short-deadline queued lane therefore does not return promptly while a different legitimately active lane retains its later deadline. The real lane is a net.Conn used by accepted inner TLS handlers; local Go net/net.go documents SetWriteDeadline applying to currently blocked Write. Existing expired-queued tests manually release serialization and cover absence of later physical emission, not prompt deadline completion while the sibling remains blocked. No runtime RED is claimed by root.

Recorded as [#448](https://github.com/dianabuilds/ardents-network/issues/448), C0 Closed Alpha. Reviewed #77 and #372; downstream-reader progress and selected CREDIT peer-close retirement do not implement this outer queued Write deadline. Proposed bounded repair must remove/refuse unemitted work atomically, preserve healthy sibling deadlines/framing, current body ownership/accounting, and retain partial-frame poisoning. Repair-agent runtime acceptance requires the actual admitted lane path. Preparation dispatch waits for the current #445 Standards reviewer to terminate; no sixth active task or new implementation branch was started. Earlier pending candidate is superseded by this localization.

#448 handed to prepared repair thread 01a0fb97-f388-7491-a07d-bebab2f08652 immediately after #445 Standards reviewer terminal released its slot. The new thread is read-only preparation only; no implementation branch/runtime checks authorized until a later selected slot. #445 Spec waits for preparation terminal. #438 reported corrected unprivileged full Docker make check terminal exit2: Endpoint cumulative900.065s timeout without preceding individual test failures; logs/stack/source/exit retained externally and owned container auto-removed. Existing full Ubuntu24.04 workflow dispatch on exactf97 source is authorized without changing selectors/timeouts/profile. #437 has retained a94.021s focused four-reader failure and is comparing exactd144 baseline in a fresh owned --rm runner; contention alone is not established. PR443/447 remain OPEN at f97/10db, no integration or gate waiver claimed.

### Static accepted Carrier cleanup follow-up

#448 preparation terminal confirmed independently from maintained source/callers, with no edits, branches or runtime tests. Fifth slot returned to #445 for one sequential Spec reviewer. Root inspected outer Serve, Resolution listener deferred close and accepted cleanup collection, JOIN/Forwarding classification, and actual shared QUIC Close producer. Some duty filters use errors.Is(err, net.ErrClosed), whereas probe has a recursive non-benign filter; that difference alone does not establish a current product bug. Actual closedRoleQUICCarrier retains errors.Join(stream.Close(), connection.CloseWithError()); inspected pinned quic-go SendStream.Close yields nil or its canceled-stream error, and no concrete mixed net.ErrClosed plus unexpected sibling close error was established. Resolution's early serveOuter refusals are covered by deferred server.closeCarrier at the real accepted worker; NewClosedOuterBridge rejection needs nil handshake/callbacks, not an admitted maintained call. No issue filed from these generic failure hypotheses.

Verified workflow receipt: GitHub run36980673783 is in_progress with full job active at exact f97b83d46e00745a94db5ff3e6df7b861e6c7f5d. Local quality.yml workflow_dispatch selects the full Ubuntu24.04 job, pinned Go/tools installation and make check with separately privileged system-scope Node process command; it does not substitute affected PR groups. Docker inventory shows only current owned #437 comparator and #445 quick runner, with #438 container absent. Unrelated stopped Zulip containers preserved.

## F-97: Expired queued outgoing Forwarding write retires shared Carrier

At dev a3963544a, outgoing forwarding.session.writeChildFrame takes writer.Lock without deadline-aware waiting. After acquisition it rejects nil deadlines but not elapsed deadlines, installs the expired deadline and attempts WriteFrame; any resulting failure closes the physical Carrier. Real forwardLink.startForwarding passes the child lifetime and aborts its parent on error. Two admitted links sharing one outgoing session therefore permit B's short deadline to expire behind A's valid longer physical write; B cannot return during its wait and after A completes it can retire the healthy shared Carrier despite its own frame being unemitted before expiry. Existing physical-blocked-write regression deliberately starts emission; #372 handles actual peer CLOSE rather than local deadline. Static reasoning only; runtime reproduction remains implementer acceptance.

Recorded independently as [#449](https://github.com/dianabuilds/ardents-network/issues/449) with external brief %TEMP%/ardents-forwarding-queued-deadline-issue.md. Distinct from #448 receiving outer writer; owner source is forwarding/session.go. Root informed #448 not to fold this different owner into its slice. New thread preparation awaits a free slot: #437/#438/#445/#448 plus root currently consume all five; no sixth active thread started. Repair must refuse unemitted expired output without physical poisoning while keeping real partial-frame failures terminal, and prove actual unrelated-prefix sibling survival on both Carriers.

#448 selected implementation resumed after both #445 reviewers terminal released the fifth slot; no added reviewers/subagents permitted while count is full. #437 comparator outcome clarified by implementer: candidate146.715s failure was fixture Node6 role-cleanup deadline after retained setup completed, not prior JOIN/setup failure; baseline126.688s pass and all candidate failures preserved separately. Full Ubuntu workflow36981137465 operates exact10db candidate; root has not yet verified its terminal result or integration.

### Static Forwarding reverse-queue retirement follow-up

Reviewed session.deliverReverse, frameQueue.push/next/close, forwardLink.copyReverse/stop/close, and ClosedForwardingChannel.QueueReverse/ReleaseReverse. Complete reverse frames reserve before enqueue, next clears consumed references, and release keeps control/data accounting separate. Child retirement makes late ReleaseReverse a no-op rather than debiting sibling capacity. Real openForwardingLink only publishes a link on success, so a hypothetical cancelLane unrelated error result with nonnil orphan link is not established by maintained producer. No additional queue leak filed.

Pending reverse-output deadline candidate: QueueReverse and ReverseRetired check child.deadline, but AccountOutput checks only channel.deadline and child existence. The real serialized parent writer in forwarding/listener.go calls AccountOutput after acquiring its mutex; already-queued reverse payload can wait past child expiry while the longer parent remains live. OPEN admits shorter child deadlines; inspected code has no per-child expiry removal timer. Before filing, verify current output/terminal expiry contract and actual caller transport deadline bounds, distinguish frames already physically started from queued work and terminal cleanup. Root source reasoning only, not a runtime reproduction; no backlog issue created yet from this candidate.

## F-98: Reverse output can start after a nested child expires

Confirmed statically at dev a3963544a: QueueReverse checks both deadlines, but AccountOutput checks only parent deadline and child presence. The maintained reverse copier calls the serialized parent write closure; parent TLS transport is assigned the parent lease deadline, and OPEN permits an earlier child deadline. Thus a complete reverse BYTES frame reserved before child expiry can wait behind another legitimate parent frame, then be charged/emitted after child expiry while the parent remains valid. There is no inspected per-child expiry removal timer. This differs from already-started physical output and terminal cleanup; no runtime reproduction claimed.

Recorded as [#450](https://github.com/dianabuilds/ardents-network/issues/450), with %TEMP%/ardents-reverse-output-expiry-issue.md. #449 and #450 must be implemented serially or explicitly coordinated because both own Forwarding paths; #448 owns receiving outer writer and is not expanded. New repair thread dispatch awaits a slot; current root+437+438+445+448 remains five. Required fix acceptance includes actual reverse copier/writer path, no post-expiry payload emission, joined accounting/retirement and healthy sibling continuation, without generic physical failure suppression or invented terminal policy.

### Static Forwarding terminal cleanup receipt

Inspected ClosedForwardingChannel.close/Cancel, duty.release and real forwardLink shutdown. Cancel releases both queue classes and duty.release atomically retires outstanding child-capacity counts, so absence of per-child releaseChildCapacity in the Cancel loop does not itself leak capacity. Some serveDirect teardown calls ignore link.close return, but the physical lease Release result is retained in pool.closeErr; pool.Close joins it into the outgoing owner result. No independently lost physical cleanup failure established from the ignored expression alone.

Corrected reachability detail for the earlier pool-retirement hypothesis: forwarding.session stores binding.Invalidate as method value and calls session.invalidate on failure; literal-call searches alone miss this maintained use. Thus Invalidate does have a real caller. Its close-under-global-mutex latency remains a candidate requiring evidence of actual concrete retirement duration, not proof supplied by an arbitrarily blocked fake Carrier. No duplicate issue filed from this correction.

#448 implementer reports actual admitted-lane runtime RED in both queued-expiry variants, preserved outside Git: B remains queued beyond deadline while A remains actively writing. Root has not rerun these tests and makes no GREEN/integration claim. Verified GitHub full jobs for runs36980673783 and36981137465 still in_progress; no observation timeout treated as terminal. #445 quick gate exit2 retained and owned runner removed; exact baseline/candidate comparison runs sequentially.

### Static Forwarding scheduling review

Reviewed NextAvailable, pending EOF, ready-data rotation, control dequeue, eventAvailable and drainForwarding. dataDue enforces a data turn between eligible controls; unavailable consumers keep their bounded queue/EOF accounting. Consumed body references are cleared by child.frames removal, control slice deletion releases corresponding reservation, and CLOSE decrements a pending EOF only in the matching stored case. Synthetic stale OPEN-after-CLOSE can be constructed against raw Route scheduling calls, but the maintained Node caller treats OPEN as always available and drains after each accepted frame, starting its opener before accepting a following CLOSE. No admitted maintained stale-OPEN trace established, so no separate bug filed from that API-only ordering.

Authoritative snapshot: PR451 is OPEN/draft at7f9d470f627010c4c36a02889a4ceb4bd34572fe. Full workflow runs36980673783/36981137465 remain in_progress, not terminal. #449/#450 dispatch still awaits capacity; their issues/briefs are preserved and the active count remains root+437+438+445+448. No gate or integration acceptance claimed.

### Static byte/queue accounting inspection

Inspected bootstrap forwarding transfer, queue/control rollback and actual compact forward-frame storage. reserveQueue/control reserve global capacity first and roll it back if bootstrap Queue refuses. transferred bootstrap lease invalidates its former handle without duplicating controller ownership. Nonempty ARDP BYTES is required by the real parser, so zero-body frames accumulating uncharged metadata through the public helper alone are not an admitted transport trace. Compact forward data stores payload and frame lengths; the explicit complete-frame traffic debit is performed independently in Accept/AccountOutput. The current contract explicitly charges complete control headers to control queues but describes lane queues as ciphertext; a header-only arithmetic mismatch alone is not filed as a separate data-queue defect without establishing the precise queue/heap owner requirement.

Root verified PR451 all selected affected CI/script parser/go jobs terminalSUCCESS at7f9d470f6; full job is intentionally skipped for PR event and does not establish full make check. #445 authorized existing fullworkflow on that exact reviewed candidate after sequential comparator terminal, without waiting for independent #437/#438 integration; any changed merged owner still requires appropriate integration verification. #445 remains open through #446 acceptance.

#449 read-only preparation terminal independently confirms outgoing session deadline defect and clarifies healthy sibling acceptance must use a different incoming prefix: abort B retires its own parent. Repair thread01a0fbac-d77b-7113-9808-ab673c9a2608 remains prepared/idle with no branch/runtime. Immediately assigned the temporary fifth slot to read-only #450 preparation thread01a0fbae-5ca5-7320-a3a4-3e995521b79b; no extra implementation started. #448 remains staged/locally focused-verified awaiting check window.

### Replenishment static follow-up

Accept debits the complete replenishment ADMIT before calling admit; success replaces byteLimit with usedBytes+32MiB and retains successful release callbacks until Cancel, preserving original deadline. Candidate cleanup path now localized for further audit: hosting.Replenisher returns errors.Join(spendErr, releaseErr) when Spend refuses after Hosting Reserve, but ClosedForwardingChannel.admit replaces that error with a generic replenishment error; no successful release callback is retained. HostingReservation.Release can return real context/persistence failure, and Hosting.Close only closes the root without reporting unresolved releases. Additionally the accepted Forwarding caller discards serveDirect return rather than collecting a separate cleanup result. Before filing, compare existing release findings/issues and trace final duty result to distinguish protocol refusal from a lost mandatory cleanup cause. No new issue or runtime reproduction claimed yet.

## F-99: Forwarding loses actual Hosting cleanup failures before final Drain

Static trace at dev a3963544a: Channel.Cancel returns initial/replenishment Release callback errors through serveDirect's named result, but accepted direct caller ignores that result and inner caller only chooses terminal status. finishShutdown collects physical/root/host Close, not Hosting reservation release failure. Refused replenishment similarly calls real hosting.Replenisher, which joins Spend refusal with failed rollback Release; channel.admit replaces that error with generic refusal and retains no rollback callback. HostingReservation.Release has concrete unresolved context/persistence outcomes, while Hosting.Close only closes its root and leaves unresolved durable reservations. Physical lease Release retention in pool.closeErr is separate and does not resolve this Hosting loss.

Recorded [#452](https://github.com/dianabuilds/ardents-network/issues/452); external brief %TEMP%/ardents-forwarding-hosting-cleanup-issue.md. Runtime failure injection remains implementer acceptance; root ran no runtime tests. #450 preparation terminal confirms reverse expiry gap and notes simple output guard does not alone prove joined child/accounting retirement. Used its released temporary slot to dispatch read-only #452 preparation; no additional implementation branch, gate waiver or Endpoint causation claim.

#452 read-only preparation terminal independently confirms both Hosting cleanup loss paths on current dev, with no runtime test/edits/branch. Returned temporary fifth slot to #448 to commit through normal hook and execute required quick/check in the correct selected environment. Root authenticated Docker inventory empty; #437/#438/#445 full profiles run remotely. No extra reviewer/subagent permitted while root+four implementers occupy five. Past canceled hook attempt remains explicit incomplete evidence; no bypass or deadline extension authorized.

Verified ongoing remote run handles36980673783/36981137465/36982727685 remain in_progress. For #438 actual full job step Run complete maintained gate began07:51:07Z; its live handle, not elapsed observation or conversation status, controls terminal interpretation. Source replenishment wrappers inspected for actual caller reachability; cancellation during physical control send remains an unfiled candidate until lifecycle context/retirement proof is traced.

### Source replenishment cancellation localization

Traced actual callers: maintained ReplenishPrefixes is reached through qualification.ReplenishStreams and qualifiedWorker; Source/Introduction/Responder wrappers otherwise only delegate. closedSourceChannels.replenish checks ctx before control.send and during receipt wait, but control.send/awaitWrite observes lane.writeEnd/owner.changed rather than that operation ctx. Existing awaitWrite already removes expired unemitted requests safely and interrupts active physical output at its lane deadline, so this is not a duplicate generic queued-lane deadline bug like #448. Need qualification setup/network lifecycle cancellation ownership and exact operation deadline contract before claiming late operation cancellation as a defect. Kept candidate unfiled; no product-wide replenishment cancellation claim made.

Root verified at08:22:47UTC that full CI run handles36980673783/36981137465/36982727685 remain in_progress; #438 full step started07:51:07UTC and remains within its original60min orchestration budget. #448 resumed active on selected checks window, no added reviewer. No slot freed by an observation timeout or intent alone.

### Qualification refill join observation

runQualifiedStreams starts one periodic ReplenishStreams goroutine under bounded context, then after RunConnections returns it cancels bounded and waits for that goroutine's stopped result. Source replenishment physical send precedes its ctx-select receipt wait; awaitWrite reacts to the retained lane deadline, not operation ctx. Prefix has an AfterFunc on its original creation context, so whether that independent lifetime closes on qualification cancellation is decisive; cannot assume absence of all cancellation ownership merely from replenish helper. Kept the candidate unfiled pending exact lifetime relation. Contrast with owner.open: it explicitly installs ctx AfterFunc to update the queued OPEN lane deadline before awaitWrite.

### Prefix creation lifetime trace

Resolved creation context: Endpoint openPrefix creates operationFlight from owner.lease.Context(), not the qualification refill operation context. During opening only, a caller ctx AfterFunc cancels that flight; it is stopped before successful completion. operation.complete transfers prefix and flight.cancel to Source.FinishOpeningLocked. The retained prefix's AfterFunc therefore follows duty lease/Source retirement after handoff, rather than every later bounded refill caller. This narrows the cancellation candidate: a later refill operation's ctx cannot be assumed to interrupt physical send through the original prefix callback. Still need exact qualification worker-attachment/job retirement effects and accepted prompt-cancellation requirement to finish localization; no runtime test or broad product claim made.

## F-100: Qualification refill cancellation does not interrupt pending ADMIT send

Resolved static lifetime trace: retained prefix is owned by Source/duty lease after successful opening; opening caller callback is stopped before handoff. Job cancellation cancels job, not duty lease or retained Source. Qualified worker attachment and supplied Service streams are the RunConnections cancellation targets. Replenishment send awaits owner.end/lane deadline before selecting operation ctx for ACCEPT; a later canceled qualification refill therefore cannot rely on prefix's original creation callback to interrupt that send. runQualifiedStreams cancels then joins its periodic refill worker, exposing the delayed completion at a real maintained caller. Recorded [#453](https://github.com/dianabuilds/ardents-network/issues/453) with %TEMP%/ardents-refill-send-cancellation-issue.md. Runtime RED remains implementer acceptance; no causation of current Endpoint failures claimed. New thread dispatch awaits a free slot; cap unchanged.

#437 implementer reports exact10db full Ubuntu job passed and aggregator finishing; root checks authoritative run/PR before claiming integration. Earlier Docker failures remain source-bound, not replaced by a claim that they never occurred.

### Integration checkpoint: NET14V percentile and NET32 monotonic measurement

Root fast-forwarded dev from a3963544a to db4014998d31a6936d7e7a5634ef117025b04980, preserving the local audit report and both unrelated untracked documents. #438 PR443 merge3d2321b68cd3da77cac234f22edaf9fa2370ebc8 is integrated; full Ubuntu gate succeeded, issue closed, own worktree archived and containers removed, implementer terminal. #437 PR447 merged db4014998; implementer reports integrated regressions passed and ancestry confirmed, cleanup/issue closure remains to verify. Prior failed attempts remain evidence, not erased by successful CI. Released #438 slot explicitly assigned to #444 schedule-binding implementation from fresh origin/dev; root plus #437 finishing, #445, #448 and #444 remains at five. #453 dispatch still awaits a verified free slot.

### Refill owner cross-check on integrated dev db4014998

Static reinspection confirms #453 persists after #437/#438 integration: closedSourceChannels.replenish still sends lane-zero ADMIT against owner.end before selecting refill ctx, and closedRoleChildStream.replenish still acquires its writer and emits before observing ctx. ClosedJoinedStream.Replenish additionally serializes with refillMu; the maintained periodic worker is sequential, so mutex contention alone is not claimed as another reachable bug. JOIN receiver replenishment is distinct: closed_join_replenishment.go returns the actual replenish error; closed_join_accept.go defers side.Close and joins side.cleanupErr into its result. Its existing retained-error test cannot be used to claim Forwarding's separate lost Hosting cleanup paths in #452 are already repaired. Root performed source reads only, no runtime RED or new issue from these cross-checks.

#437 terminal verified via thread poll: integrated regression passed, issue closed, worktree archived and containers removed. Assigned freed slot to #453 implementation thread01a0fbc5-f31a-78b0-bf2b-26e6ffb59d96 from fresh origin/dev, with real caller cancellation/framing acceptance and no extra reviewers until another slot is released. Root plus #444/#445/#448/#453 stays at five.

### JOIN Hosting cleanup propagation candidate

At integrated dev db4014998, ClosedJoinPairs.AcceptStream defers side.Close and joins side.cleanupErr into its returned outcome. However node/join serveInner only tests serveAdmitted result for terminal status, discarding that concrete error. node/outer.Serve's child callback returns no result and its deferred closeErr is only physicalCloseErr; JOIN final run collects server.cleanupErr/pairs.Close/spends.Close/host.Close. This is not a proof of end-to-end cleanup retention merely because the route test sees side.cleanupErr. Concrete Hosting failure production, current cleanup owner requirement and issue duplicate scope must still be checked before filing a distinct finding. Root source-only observation, no injected failure or causal claim about existing Endpoint failures.

## F-101: JOIN discards actual Hosting cleanup failures before final Drain

Confirmed source trace on dev db4014998: initial and successful replenishment releases enter ClosedJoinSide.releases; release retains callback causes in side.cleanupErr; AcceptStream returns them. node/join serveInner consumes that result only as status, node/outer.Serve void callback retains only physical close, final JOIN Drain cannot recover the side cause, and pairs.Close returns no result. Real Hosting.Release bounded-context/persistence failures stay unresolved; Hosting.Close does not report prior release errors. Distinct from #452 Forwarding ownership. Filed [#454](https://github.com/dianabuilds/ardents-network/issues/454) in accessible C0 Closed Alpha milestone1 with actual owner-boundary acceptance in external %TEMP%/ardents-join-hosting-cleanup-issue.md. Static finding only; runtime RED and full valid admitted path remain implementer acceptance. No implementation slot available, thread dispatch waits; root+#444+#445+#448+#453 remains five.

#454 acceptance expanded by a same-owner static follow-up: node/join.serveAdmitted defers lease.Release without collecting its returned error. Before transfer into a JOIN side, refusal at lane.Admit/deadline/ACCEPT/State/AcceptStream therefore loses the real initial Hosting release even before the void outer callback. After successful transfer this deferred Release is correctly a no-op: claim.transfer clears the old claim, preventing duplicate refund. Recorded issue comment with separate pre-handoff failure-injection acceptance, not another bug/thread. Pair.Close waits entries and timers outside owner.mu; Serve closes its own ioDone before waiting peer.ioDone, so no generic mutual-wait deadlock claimed from that ordering. CI36982727685 authoritatively remains live full in_progress; no retry or acceptance shortcut.

## F-102: JOIN refill contradicts current grammar and server omits client-required ACCEPT

At dev db4014998, current protected-route-protocol Kind2 row explicitly forbids later ADMIT outside forwarding parent; linked accepted ADR0085 selects forwarding replenishment only. Maintained qualified caller ReplenishStreams loops snapshot.Joins, ClosedJoinedStream creates its dedicated role-TLS channels owner, sends lane-zero ADMIT after real threshold and awaits status0/64KiB ACCEPT. Actual ClosedJoinSide.Serve accepts ADMIT through replenisher, updates remaining to32MiB and continues without emitting ACCEPT. Recorded [#456](https://github.com/dianabuilds/ardents-network/issues/456), external %TEMP%/ardents-join-refill-contract-issue.md, with explicit two contract-resolution options and actual paired caller acceptance. No runtime RED or accepted new wire semantics claimed. Not #355 incremental Forwarding reserve, #453 cancel boundary or #454 cleanup loss. Informed active #453 implementer of independent gap and forbidden scope expansion/fake ACK acceptance. New issue remains prepared, no free sixth thread/research slot assumed.

#456 Product Owner explicitly selects bounded JOIN refill in async response: original deadline/bindings, Hosting reserve before spend, remaining exactly32MiB and ACCEPT. Recorded the decision in issue comment; authoritative ADR/current grammar reconciliation and real paired caller acceptance still required, not asserted done by chat preference. Implementation awaits a slot; #453/#454 scopes remain separate. Root independently verified full CPU workflow36982727685 SUCCESS exact7f9d470f627010c4c36a02889a4ceb4bd34572fe with full+go successful; PR451 still draft/open at verification. #445 component merge/integration and #446 real producer acceptance remain outstanding.

### JOIN expiry cross-check

ClosedJoinPairs.expireLocked checks both authenticated clock deadlines and monotonic-backed setupWall/side.wallDeadline. Once paired, setup-only ten-second expiry is omitted while each original side deadline still retires the pair. stopLocked is idempotent, stops tracked timers and signals pair.done; wait rechecks context/expiry after ready. Thus paired data is not statically truncated to setup deadline, and generic wall-clock rollback extension is not claimed for this owner. #456 missing actual ACCEPT remains separate from timer behavior. #453 implementer informed of PO-selected refill semantics without authorizing scope expansion. #445 implementer explicitly checks combined latestdev db4014998 before merging reviewed CPU component; successful old-head gate alone does not establish final integration.

### Initial admission/spend failure cross-check

ClosedAdmissionChannel.acceptInitialAdmit refuses when Spend fails and joins failed Hosting release; it does not admit work or refund a spent token. Its generic protocol error does omit the raw Spend cause, but replay.Ledger retains first append/prune mutation failure in ledger.failure, bars subsequent Spend and joins failure into Close. Actual JOIN final run includes spends.Close, so loss of Spend cause from the immediate protocol result is not independently filed as lost final cleanup. Ledger validates the current hourly redemption window before spending; pruning writes a retained floor before forgetting earlier spends and rejects clock rollback below that floor. Existing failure/floor tests were read, not rerun or cited as fresh runtime proof. These mechanisms are separate from actual Hosting.Release loss in #454; do not equate journal-close retention with reservation-error retention.

### JOIN initial budget comparison on dev db4014998

Receiver Reserve seeds used with HELLO/ADMIT/ACCEPT including their three headers and the 4096-byte OPERATION/header; RESULT is then charged by side.writeFrame using fixed terminal.BodySize16384/header. Client newClosedJoinedStream seeds transferred with the same initial total including RESULT. Both account subsequent full frame headers/body before effects. No initial RESULT omission or double debit found in these actual owners; existing route budget test read only. #456 future matching ACCEPT must use the existing accounted physical output owner, not an uncharged direct write, and requires serialization with peer forwarding. Active #445 is synchronizing to integrated dev before final checks; original-head full success remains source-bound, not proof for the combined tree. #453 outer queued cancellation regression now reported green, inner and actual prefix acceptance still in progress; no completion claimed.

### Active-owner conflict check

Read current #444 worktree diff and #445 combinedhead efe2a946c0cfae83b0b89f5ea28a3ff02cf8ab8c through native Git. Common changed files are qualification_evidence_linux_test.go and stream-network-two-host/README.md; primary production files differ. Notified both implementers: #445 integration first, #444 reconcile fresh dev preserving CPU monotonic fixture fields and its own manifest/host/origin/actual-window behavior, no block replacement of shared README. No edits made to their worktrees or extra reviewers started. Native-read escalation used after sandbox foreign-worktree Git access failed; no destructive operation.

### Endpoint opening handoff static check

operationFlight.complete rechecks exact Source opening identity, caller cancellation and current authority under owner.mu before publishing prefix. A refused/current-owner-changed completion cancels its retained flight and closes the exact prefix; successful completion transfers prefix and flight.cancel into Source lifecycle and closes done only after completion. Lifecycle FinishOpeningLocked requires exact flight identity. operationGate persists across prefix replacement and checks both caller and independent lease after acquisition, so a ready slot does not allow a canceled caller. failDutyContexts snapshots owners under endpoint.dutyMu then releases leases after unlocking; it does not synchronously acquire each owner's mu at this callsite, so the suspected direct self-lock was not supported. Prefix cancellation callback invokes interrupt/retirement, not prefix.finish recursively. No new bug filed from these reads; runtime admission/cleanup acceptance still belongs to selected implementers.

### Issuance cancellation/retry ownership follow-up

tokens.Operation retains one exact permission/profile/batch and reserves its issuance slot before transport. Complete rechecks surviving owner, permission, profile and deadline; caller cancellation can discard only that operation's pending batch, while reserved grant counts remain consumed. Exchange failure preserves the original opaque pending batch/request identity for explicit retry, and ReserveBatch requires the same challenges/selection/prefix rather than refunding or re-sampling. finishLocked clears the operation's request copy and signals done after completion; Batch documents that network attempt owns copied request bytes. No replay/refund bug or new issue established by these static reads. #444 acknowledged preservation of #445 shared CPU fixtures/README; both remain active until their actual required checks and review/integration finish.

### #444 bounded review preparation

Pinned base db4014998 and exactcandidate ea6cd5164bf95b380e72d7e8bd9ccd07879cfc36 PR457: refs resolve, nonempty14file diff and one #444 commit. Retrieved original issue requirements and current standards sources. External review brief %TEMP%/ardents-444-review-checkpoint.md captures immutable commands/scope. Current GitHub ledger is explicit in repository/human instructions despite absence of optional docs/agents/issue-tracker.md; no setup flow or scope uncertainty inferred. #444 agrees to await terminal nativecheck then yield idle with remote handles; #445 requested same after combinedpush/dispatch. Two parallel read-only axes can start only after both verifiedidle, preserving root+448+453+two reviewers=5. No reviewer actually started yet; active gates not stopped or acceptance waived.

### Exact remote CI ownership receipt before review

Root native gh verifies #444 PR-run36986982630 SUCCESS exactea6cd516, fullworkflow36986920711 live in_progress. #445 PR-run36987025627 SUCCESS exactefe2a946; fullworkflow_dispatch36987021169 live fulljobin_progress. Native #444 makecheck exit0 and cleanworkspace reported by implementer; #445 local processes terminal and containers removed reported. Both still active in authoritative thread poll, so neither reviewer slot yet released merely by intent. Root retains these live run handles and does not restart them. Confirmed PendingClosedTokenBatch.Request defensively clones raw request bytes; operation.finish clear cannot corrupt the retained exact retry request.

### #444 read-only review dispatched within cap

Authoritative wait verified #445 idle completedturn cursor27 and #444 idle completedturn cursor15; all local processes terminal/receipts supplied, live remote runs remain rooted under coordinator. Spawned two independent read-only axes /root/review_444_standards and /root/review_444_spec on exactbase db4014998...ea6cd5164; no implementation/checkouts/tests/nested agents authorized. Root+448+453+these2=5 while444445 wait remotechecks/review. CPU integration first remains required; no returned slot yet. Refilling extra issue454456 implementation waits; this is bounded review, not another implementation slice or independent security validation.

#444 read-only axes terminal: Standards0 and Spec0 on db4014998...ea6cd516. Report external %TEMP%/ardents-444-review-result.md and issuecomment, no tests run by reviewers, fullgate/integration unresolved. Corrected functional-map source path docs/product/functional-map.md (checkpoint typo docs/reference); both axes used current owner. Reused released slots for parallel read-only #448 Standards/Spec reviews exactbase a3963544a...fe584813ebddb8ab5efb5e276dae24e598049dbf, verified4file diff. Root+448implementer+453implementer+2reviewers=5; 444445 remain explicitlyidle with liveCI under root. No additional implementation or scope expansion.

### #448 review result and next selected work

#448 Standards P2 fixture missing failure-path joins for HELLO reader/TLS handshake helper goroutines at queued_lane_test.go85/124: success-only channel receives bypassed by t.Fatal, pipe cleanup does not wait. Root verified exactfe584hunk and returned implementer slot for bounded fixture cleanup fix; Spec0 on samecandidate, valid scheduling fixture scope explicit, no gate claim. #448 localfull terminalfailed Node6 Endpoint cleanup; preserved no causal attribution, remotefull continues. Read-onlyreviewers terminal, so root+448+453 leaves two slots. Dispatched selected #456 thread01a0fbe1-1706-7b10-ba5c-19b513830c9b for PO-chosen bounded JOIN refill authoritative reconciliation then actual serverACK acceptance, currentowner/ADR discipline mandatory. Dispatched #454 thread01a0fbe1-b070-7a11-9456-f4d4b996e07c: read-only prep, no branch/worktree/runtime/implementation; future JOINcleanup after456integration or explicit sharedownership. Root+448+453+456+454prep remains five; 444445 idle with live remoteCI under root. No extra reviewers/implementation authorized.

#456 initial turn terminalfailed before actions due selected-model capacity; root retried same thread/settings once (no model override/new duplicate). Authoritative resumed thread nowactive cursor2, reading current owners; work did start. #454 read-only prep active cursor1. Full44436986920711 and full44536987021169 remain live exactpinnedheads. Forwarding refill cross-check: channel.admit updates budget and returns no event, but actual listener caller separately emits its ACCEPT, unlike JOIN Serve's continue-only path in456. Thus helper-level missingACK alone cannot be generalized across roles. No additional issue filed from Forwarding helper/no-event observation.

### F103 — conflicting maintained C0 execution limits (#458)

At dev db4014998, documentation.md:74-75 allows one implementation issue; AGENTS.md and agent-execution.md select two. The supplied PO policy permits explicitly authorized larger selections. Issue #458 records exact owners and acceptance; repair is queued, with read-only preparation only until an active-thread slot is released. No numeric session assignment belongs in permanent policy.

Coordination receipt: #454 read-only preparation is terminal; #449 implementation resumed in its separate forwarding-queued-deadline worktree. #448 setup-helper cleanup recheck at 2d7e8452b59d30aeccb8ab18e8cc5381a1a0ea75 reports Standards hard findings 0 / heuristics 0; previous Spec result 0 applies to the unchanged product delta. Full gate and integration remain unproven.

### F104 — Forwarding role reader retains unaccounted queued input (#459)

Static trace at db4014998: listener.go reads channel retains one complete frame and its producer another, before ClosedForwardingChannel.Accept reserves either against the duty queue. OuterBridge Read has already released ciphertext reservation at ownership transfer. Protocol 493-511 requires complete Node queues within the receiving-duty 64 MiB ceiling with separate control capacity. #459 contains exact owner path, required actual-handler RED and transfer/cancellation acceptance; no runtime RED or measured aggregate exceedance claimed. Repair thread awaits a free active-thread slot; not silently started beyond five.

#456 incorrectly stopped against stale local default-two policy. Explicit PO larger-count authorization and supplied current AGENTS text were reiterated; selected implementation resumed within five threads. #458 records the contradictory documents that caused this operational refusal.

Static follow-up to F104: accepted Forwarding data does not release its receiving-duty reservation merely on NextAvailable. nextReadyDataLocked transfers its body into the delivered count while child.queued remains reserved; Credit releases that reservation only after actual downstream CREDIT. Reverse frames likewise remain Route-reserved after frameQueue.next removes their local queue slot, until copyReverse calls ReleaseReverse after its output returns. No additional early-release bug recorded for these paths. Pre-Accept reader retention in #459 remains distinct.

Authoritative remote checks #445 run36987021169 and #444 run36986920711 remain in_progress specifically in Run the complete maintained gate. Neither component is claimed merged or fully accepted. #449 actual queued-expiry RED reported on both selected Carriers; #453 actual admitted-prefix fixture coverage is still being built, not a passing acceptance claim.

### F105 — expired Forwarding child loses terminal retirement (#460)

At db4014998, session.deliverReverse discards every frame when ReverseRetired detects child expiry, including authenticated CLOSE; frameQueue.next is not woken. forwardLink has no child expiry owner, so idle reverse copier, outgoing lease and Route child capacity remain until upstream CLOSE/parent cancellation. Listener Reap covers host/pool idle entries only, not an active child lease. #460 records actual-caller runtime RED requirements and preservation of healthy sibling/shared Carrier; no runtime RED yet. Implementation queued behind #449/#450 shared ownership and available active-thread slot.

### F106 — reserved Forwarding control lanes unreachable (#461)

At db4014998, ClosedForwardingChannel.open rejects all OPEN at children>=256 before control-purpose capacity admission. Source and closedDutyChannel correctly permit two reserved control lanes; tests bypass actual Accept. Downstream session.attach independently caps live+retired at256. #461 requires complete actual-caller control reserve acceptance on both Carriers, rejects extra work/third control and preserves duty ceiling; static evidence only so far. Queued behind #449 shared outgoing ownership and available thread slot.

#445 component integrated by root: PR451 MERGED 2026-10-02T09:32:58Z, dev merge63929a55951bd4dd9418e5c8c07056436da9c0b3, reviewed unchanged CPU delta, exact candidate efe2a946 full36987021169 SUCCESS and affected36987025627 SUCCESS. Issue445 remains OPEN pending446 actual owner sampler acceptance. Agent own-worktree archival awaits a freed active-thread slot; no claim of completed cleanup. #444 full36986920711 SUCCESS on ea6cd5164; fresh-dev reconciliation still needed after CPU component merge.

F106 follow-up: ClosedOuterHandshake.open uses duty.reserveChild() (control=false) at the downstream physical parent; #461 addendum requires authority-aware inspection and complete path acceptance, not blindly widening the last-256 retired-ID tombstone ring. Hosting period audit found no new period-reset defect: initialization refuses an existing root, storage checks immutable policy pin, normal Node composition enforces one exact root, and release affects the same retained ledger owner. Existing unresolved release-cause propagation remains #452/#454 rather than a duplicate issue.

### F107 — benign accepted-close cause hides real sibling failure (#462)

At63929a559, Resolution/Introduction closeCarrier and serveOuter plus JOIN/Forwarding accepted-close recording drop all errors matching net.ErrClosed. Joined/wrapped composite physical failures therefore disappear from final Drain. Issuer/probe already remove only benign sentinel leaves. #462 includes four real lifecycle owners, actual close seam RED and pure/wrapped/joined acceptance cases, coordinated against deferred Hosting-cleanup owners; no runtime RED claimed. New repair threads for459-462 await a freed slot rather than exceeding five.

Introduction static check at63929a559: serveRegistration binds ACK to exact pending even lane and fresh nonce, rejects duplicate/expired ACK, and deliver rechecks State/capsule expiry before closing with outcome. Slot admission reserves fixed OPERATION/RESULT/CLOSE allowance and separately withdrawal capacity; expiry preventing child protocol completion retires registration as the current protocol requires. No new late-ACK/refund bug recorded for these paths.

Workspace hygiene inventory: only live project containers ardents-453-gate and ardents-448-integrated-full are present; five stopped swarm-zulip containers belong to another workload and are not authorized task cleanup targets. Root has no managed attachments via list_artifacts, so it cannot use archive_worktree for445 from this chat; own agent cleanup must resume when a slot is free. Six selected repair worktrees remain (444/445/448/449/453/456); two non-selected user worktrees preserved. No deletion performed.

Coordination: #449 verified idle terminal turn, source4130691f2f1812f12f19ece894a45ba6094452be, PR463, full36991120839/affected36991173777 handles handed to root, no local live commands/own containers. #445 resumed cleanup-only in the freed slot, no new tests/implementation. Prepared repair dispatch inventory459-462 saved outside repo at Temp/ardents-static-audit-prepared-issues-459-462.md; active cap five preserved.

#445 cleanup independently observed: read_thread reports completed/idle, archived_worktree attachment01a0fc01-be01-71c1-908f-e8d589f326b8 and checkout absent, receipts retained. Root resumed444 integration in the freed slot; five active preserved. Root monitors449 remoteCI without reactivating449. Root obtained cleanup results by read_thread/receipt, so an agent's rejected unsolicited status message does not block authorized cleanup or coordination.

State wave static follow-up: attempts are committed before contact, changed Source exposure plan is bounded before its first attempt, durable terminal failure/backoff retains per-slot outcomes, current/pending transition rechecks trusted completion time. No new backoff bypass proved. Mixed-valid-result/caller cancellation remains an unconfirmed semantics question: the current owner says complete wave selects highest valid State and per-source cancellation classification has precedence; that does not alone prove all authenticated observations must be discarded. No bug filed without stronger current contract/caller evidence.

#456 implementer reports canceled-Serve vs delayed-policy-commit race RED, tracked in existing456 acceptance with exact caveat root not rerun. #453 cyclic test-import refusal not bypassed; requested maintained higher-owner/public API placement for actual qualification cancellation regression before concluding an external receipt replaces maintained coverage.

### F108 — Control Hosting reserve ends before child termination (#464)

Static exact ordering at63929a559: Resolution/Introduction serveAdmitted and credential ServeAdmittedAfterHello run deferred lease.Release before their outer serveInner performs TLS CloseWrite and Outer lane.CloseWithStatus. Admit only binds authority/deadline, with no retained Release transfer. The shared handle remains live but per-operation termination reservation is refunded while terminal output may remain blocked. #464 covers three actual control-role lifecycle owners, actual terminal-write gating and exact-once post-join release, normal checks/integration; no runtime RED yet. Distinct from462 filtering physical cleanup causes and452/454 lost Hosting release failures; queued within active cap.

### F109 — Outer local Close retains buffer after refunding queue (#465)

Static at63929a559: closeLocalOnce first handshake.Accept(CLOSE) refunds child.queued and releases child capacity; then waits terminal serialization, then closeInput clears unread buffer. Thus a post-innerHello unread ciphertext queue remains retained without receiving-duty reservation while terminal output waits. Existing active-CREDIT regression consumed its body before Close, and global bridge shutdown correctly holds reservations through join; neither disproves this per-lane gap. #465 requires actual queue/duty RED and transport caller acceptance without disturbing active-CREDIT serialization, distinct from459 role reader transfer and464 Hosting. No runtime RED claimed; queued behind448 ownership/cap.

### F110 — Resumed State wave loses interrupted BY_DIGEST outcome

Tracked in [#467](https://github.com/dianabuilds/ardents-network/issues/467).
Static trace on dev `63929a559`: recovery durably records `interrupted` for a
consumed BY_DIGEST slot; production Refresh assembles only its current results,
then `finishWaveState` replaces all four outcomes with that partial array. The
historical digest slot becomes zero (`not-attempted`) after terminal publication.
Its consumed attempt remains retained, so replay permission is not claimed.
The existing recovery test checks before wave completion and misses this loss.
Required repair verifies durable reopen/resume/completion/reopen, preserves
original slot cause and backoff, and keeps fresh-cycle reset and selector bounds.
Runtime RED has not yet been run. Separate implementation awaits a free selected
slot; the agent-ready evidence and acceptance brief is attached to the issue.

F110 dispatch: #467 selected repair thread `01a0fc1a-6b23-7062-80e3-5b16aaf6b307`
started after #444 completed its local work and yielded its active turn. Current
selected active chats: root, #453, #456, #458, #467. #449 exact-head full Ubuntu
CI 36991120839 independently reports success; its reviews and integration remain
pending, so neither issue closure nor thread archival is claimed.

Negative audit: State closed-profile runtime reads explicitly reject background
owner failure, while their current owner comments distinguish offline profile
acceptance from runtime use. Missing background-error checks on offline
acceptance alone do not establish a contract defect; no issue filed from that
observation. Reachability current-store review confirms conflict expiry retains
the longest observed signed Credential lifetime and enforces the no-overlap
successor boundary; no duplicate bug filed for the already covered path.

### F111 — Interrupted Reachability staging blocks real resolution restart

Tracked in [#470](https://github.com/dianabuilds/ardents-network/issues/470).
Static trace on dev `63929a559`: `replaceStoreFile` creates `.record-*` in the
canonical records directory, with removal deferred until orderly return.
A terminated process leaves its owned staging file; `restore` counts every
entry against 128 Targets and refuses names other than 64-character Target
records. Existing initialization has no staging recovery. The actual resolution
`Start` calls `OpenStore` before listener readiness, so intact previously
committed proofs become unavailable until unsupported raw cleanup.
No crash RED has yet run. Issue acceptance requires real production-write
subprocess interruption, bounded lease-held staging disposition preserving all
committed floors, 128-Target recovery, foreign/nonregular refusal and actual
resolution startup. An uncommitted staged update must not be acknowledged or
promoted. Repair awaits a free selected slot; the issue contains the full brief.

Negative boundary checks at dev63929a559: Publisher ACK rechecks current profile,
registration, exact resolution flight/Source, context, Instance binding and
recipient before local pair publication. `openRegistration`/`rotatePublication`
refuse a successor while a bounded predecessor remains, so merely seeing one
previousRegistration field is not evidence of an overwritten live predecessor.
PrivateRecipient.Close performs only owned key retirement and always returns nil;
its ignored return is not an actionable cleanup-cause loss.
Source readResponse refuses unknown status bytes, and production fetchFailure
and successful response closure check the operation context before returning
response evidence. No unknown-status or cancellation-precedence issue filed
from helper-only speculation.

### F112 — Forwarding CLOSE refunds retained outgoing payload

Tracked in [#472](https://github.com/dianabuilds/ardents-network/issues/472).
Static trace on dev 1cfe22e3a: Route close refunds child queued bytes and removes
the child before Node's selected BYTES writer has returned. A body waiting for
the shared outgoing writer remains owned by its goroutine; eventAvailable delays
CLOSE until that worker ends, but does not retain the refunded reservation.
The incoming reverse-output mutex does not join the outgoing writer. No runtime
RED or measured queue overshoot is claimed. Acceptance requires real admitted
child/governor reproduction on both Carriers, accounting until discard/join,
exact-once late completion, sibling progress and bounded terminal behavior.
Distinct from reader pre-Accept #459 and Outer unread-buffer #465. Prepared
repair awaits selected slot and Forwarding writer integration #449.

Integration receipt: #444 accepted and closed; PR457 merged dev
bd7036ab06147fc330ec11984c1cd4ba30ce0aae, own worktree archived and checkout
absent; coordinator archived its completed chat. #458 accepted/closed via PR469,
dev1cfe22e3a, own worktree and chat archived. #449 independent bounded Standards
and Spec reviews of 63929a559...4130691f2f both returned zero actionable findings;
agent resumed for fresh-dev integration, not yet recorded as accepted/merged.

Coordination: full Ubuntu36992523451 terminal SUCCESS exact448head1614fa30a;
full36992977496 terminal SUCCESS exact456head3af1abeb, authenticated root gh.
The failed local456 Endpoint attempts remain failures requiring distinct causal
handling, not relabeled by remote success. #448 resumed integration. #467
independent Standards0/Spec0 reviewed63929a559...9ceea9884; resumed own integration.
Current selected active threads root/#448/#449/#453/#467; cap five maintained.
Carrier-pool mutex candidate revisited: current production session.fail does call
lease.Invalidate, correcting an earlier no-caller observation. Its normal ordering
closes the same physical wrapper before Invalidate; that fact alone does not
prove sustained unrelated-pair blocking. Last-unused Release still holds pool.mu
through real Close; no new duplicate issue or runtime reproduction claim filed.

### F113 — Unemitted Source BYTES consume send credit permanently

Tracked in [#473](https://github.com/dianabuilds/ardents-network/issues/473).
Static trace on dev1cfe22e3a: closedSourceLane.Write subtracts lane.credit before
send/enqueue; queue refusal or inactive queued timeout returns without restoring
credit. removeQueuedWriteLocked refunds queue bytes only. Incoming peer CREDIT
is the only increment, but the peer never received those bodies. Repeated
unemitted failures can exhaust the live lane's 64 KiB window after its write
deadline is extended within admitted lifetime. No runtime RED claimed. Required
acceptance distinguishes queue refusal/timeout from actual physical emission,
checks exact-once credit return, retries and healthy siblings through actual
Source queue/caller seams. Implementation coordinates with #453 queue ownership
and waits for a selected active slot. Full external brief preserved.

#449 new combined ed2ce002 affected36996553302 FAILED: Endpoint
TestTextInitialPublicationLossBeforeAcknowledgement TCP caller_cancel reports
text Source handle unavailable after cancellation. Root notified implementer;
full36996546683 remains live. Failure not attributed to #453/#456 or waived.

F113/#473 refinement: source writeLoop dequeues and marks active, then may fail
an expired deadline before request.attempted. Completion refunds queue bytes but
not send credit; attempted=false keeps the parent alive. Issue473 addendum now
requires exact-once recovery at queue refusal, queued removal and selected-before-
physical-attempt failure, never after actual partial emission.
Negative comparison: receiving JOIN reserves two full maximum frames per side
through both I/O joins; any forward write error terminates its pair, so the
surviving-lane credit-loss claim does not apply to that implementation.

Publication read-only diagnostic follow-up: permissionProfileLocked first retires
ended role prefixes. Source RetireIdleLocked clears Handle.prefix only after
prefix.Done, invalidating exact retained ResolutionAcquisition. Its routePrefix
error matches #449's observed Source handle unavailable. Root passed the exact
call chain to the active #449 diagnostician; prefix.Done causal origin and
baseline remain unproven, no duplicate issue filed. finishResolution with nil
caller merely returns its existing outcome while retaining cleanup under owner
lifecycle; ignored return at lookup defer does not itself lose a new cleanup
cause. Acquisitions release their own reference, not the shared live prefix.

F113/#473 caller-boundary refinement: maintained joinedTransport is wrapped in
Service TLS; no current same-lane Service retry reachability is proved after its
write error. Root added this limitation to issue acceptance: reproduce a current
caller or identify the exact bounded Source/net.Conn obligation, never claim a
raw retry fixture establishes Service behavior. CloseWrite's unemitted EOF flag
candidate has only inspected test callers and is not filed as a new product bug.
#453's latest status turn ended idle with failed QUIC admission CI. Root resumed
its existing selected slot for diagnosis, preserving five active threads; no
new client-owner implementation was started concurrently.

#467 integration verified by authenticated root: PR471 MERGED at2026-10-02
10:51:17Z, dev369a9a760309034435747e39177732524ae01a5b; issue467 CLOSED10:52:22Z.
Implementer receipt records reviewed9ceea9884 and tested29b6d2bea ancestry, merge
same tree, unchanged patch identity, native full exit0 and composite CI36997108328
SUCCESS. Own managed checkout state-resumed-outcomes is absent (root Test-Path
False). Chat turn is still active in cleanup, so no new slot or chat archive yet.
Automatic review rejected extensive acceptance publication for nonpublic test,
review and advisory detail; exact payload/reason retained externally, not resent
through a workaround. Public merge/issue metadata and local receipt remain.

#467 final turn confirmed completed/idle cursorf4f3b2bd:9. Coordinator archived
its chat (toolarchivedtrue), worktree absent, own resources terminal. Selected
freed slot assigned to prepared #446 owner-slice sampler thread01a0fb83-a67e-
7370-887e-2cc25ace23d5; #445 predecessor merged and current dev369a9a760 still
contains the demonstrated real slice/parser mismatch. Active selected root,
448,449,453,446 (five). No concurrent sampler implementer or additional reviewer.
Root FF advanced dev1cfe22e3a to369a9a760, preserving audit modification and both
unrelated untracked user documents. Feature branches467 remain after automatic
review rejection; no unauthorized delete attempted by coordinator.

### F114 — Real successful relay collector emits unsupported Logs field

Tracked in [#474](https://github.com/dianabuilds/ardents-network/issues/474).
Static actual-caller proof on dev369a9a760: Stop-NetworkRelays emits Logs in each
successful relay record, writes relay-results.json and uploads it unchanged to
verify-pair. readRelayResults decodes []relayResultInput lacking Logs through
DisallowUnknownFields. Every nonempty successful real record therefore refuses
before evaluation. No runtime RED claimed. Failed capture's failedRelayResult
already contains Logs/Complete and is excluded from this allegation. Acceptance
requires actual PowerShell collector-to-production Go decoder/CLI regression,
strict unknown-field refusal, retained raw logs and existing terminal/counter/
sample semantics. Separate implementation awaits a free selected slot and #446
collector ownership coordination. Full brief retained externally.
Negative identity comparison: relay and Node binaries are distinct executable
artifacts from Endpoint binary; merely unequal SHA256 is not a candidate mismatch.
Current net14v comparison binds each respective identity across paired runs.

Follow-up schema audit on dev369a9a760: real Node/Source collector records match
nodeResultInput, Write-NodeAndSourceResults matches nodeResultsInput including
OwnerSlices, and installed cleanup record fields match cleanupOwnerInput. Owner
counter collector retains malformed observed fields for strict decoder/verdict
refusal rather than synthesizing missing monotonic evidence. No duplicate
unknown-field issue filed for these schemas. Relay successful Logs mismatch474
remains separate. #446 producer/collector and monotonic regressions reported PASS
while full required gates/integration remain pending. Its unsolicited coordinator
message was auto-review rejected for missing direct human messaging authorization;
root obtains progress by reading the task, so this does not block its work and
is not justification to send the rejected message via another route.

Token ownership static comparison on dev369a9a760: Permission consumes selected
stock before durable Journal.Mark; journal error clears outgoing bytes and fails
the owning context, post-mark authority/context checks happen before Route
presentation. Same-process pending blind batch retains exact challenges/selection;
discard/finalization burns its reserved allocation. Journal compaction persists
its new floor in the atomic replacement before later append; load derives the
maximum observed floor. Partial retained append refusal and no stock refund are
explicit current private-admission contract, not a newly inferred availability
bug. Recognized .attempts staging is cleaned after canonical journal load under
root lease, unlike Reachability470; no duplicate staging allegation filed.
Remote448 combined ad347065 full36996969312 confirmed in_progress; PR
36996973688 SUCCESS. #453 current first QUIC refusal is before refill after217-
218 exchanges; active owner is collecting transport/prefix terminal diagnostics
without changing accepted governor/Carrier/deadline limits.

Continuation audit: current dev 369a9a760. Rechecked actual successful collector cleanup-results.json against readCleanupResults: Role/ActiveState/MainPID/Result/ExecMainStatus/ActiveWorkers/Passed and Schema/Owners agree. Normal campaign checks real success and exit status before serialization; smoke follows a separate verdict branch. No additional schema defect established. Whole-owner receipt fields match current consumer; #446 retains the selected actual sampler argument defect and #474 remains the confirmed successful relay Logs producer/consumer mismatch. Remote full runs 36996969312 (#448 ad347) and 36996546683 (#449 ed2ce) were authoritatively in_progress on this continuation. #453 turn was terminal with unresolved QUIC cause; explicitly resumed diagnosis, not treated as an active waiter. Four repair threads plus root remain within five-thread cap.

Static continuation: verified campaign condition/manifest-cell and seed binding at the actual run-windows.ps1 preflight (175-177), shared participant profile/condition/seed at 168-173. verifyPair alone does not repeat all those preconditions, but a claim that the maintained orchestrator admits a mismatched seed/cell is contradicted by these current checks; no new bug filed from standalone-verifier omission. NET14V additionally requires normal/recovery conditions, cells and identical manifests after removing cell/failures, matching seed/profile and candidate identities. Further audit should distinguish a supported standalone verifier obligation from invented hostile evidence input.

Continuation terminal audit: evidenceJournal.write latches serialization/write/short-write failure, prevents later terminal success; finish encodes the joined runner outcome after participant results. main.go 149/180/190-209 joins workload criteria and owner-network criteria into participantErr and final outcome. readRunnerEvidence refuses nonempty terminal Failure for readCompletedEvidence, missing/duplicate participant results, truncated newline, checksum/count mismatch and records after terminal. verifyCompletedRun intentionally establishes completed byte-stream evidence rather than recalculating a qualification claim; real NET32 caller additionally requires installed Endpoint inactive/MainPID0/Resultsuccess/ExecMainStatus0 before consuming that journal. No supported-path false successful completion established in this slice. Failed NET14V intentionally uses allowFailure=true for bounded failed-byte accounting, not successful qualification; separate acceptance semantics are explicit.

#462 Service addendum: current initial Service protectedServiceTransport.Close suppresses entire composite result when errors.Is(net.ErrClosed), including any independent JOIN cleanup leaf. Actual endpoint admitted joinedTransport -> ClosedJoinedStream.Close aggregates multiple retirement owners. New caller coverage of existing F107/#462, not a separate issue. External brief ardents-462-service-close-addendum.md saved; GitHub publication attempted via gh issue comment --body-file and rejected by automatic approval review for detailed internal failure-path payload without exact external-publication authorization. No bypass/retry. Requested explicit PO choice for that precise file and issue; local evidence retained and audit continues independently. #446 candidate 723d833313778d31932939d49db0e278a0642ea6 reported ready for bounded review, corrected prerequisite quick/full still live; read-only reviewer needs freed slot, not extra concurrency.

Authoritative terminal CI update: full36996969312 exactad347 #448 completed SUCCESS, full36996546683 exacted2ce #449 completed SUCCESS. Both independently checked via gh run view. #448 instructed to complete integration/acceptance/owned cleanup; #449 instructed to retain affected FAIL2/2 and continue planned actual-CI causal probes, not waive failures with a full PASS.

New bounded candidate retained: failed replacement Service TLS setup discards raw Route Close result and disables openRecoveryAttachment deferred cleanup (ownedRaw=false), so no constructed Attachment can retain that result. Actual current recovery opener returns admitted joinedTransport; initial path has retained protectedServiceTransport and is not covered by this particular loss. Prepared ardents-service-recovery-tls-cleanup-candidate.md with exact owner/caller trace, independent distinction from #462/post-transfer F23, causal regression requirements and reachability caveat. Not yet tracker issue/assigned repair; no runtime proof claimed. Publication of #462's detailed addendum still awaits exact-payload authorization after actual auto-review rejection; do not use this candidate to bypass that refusal.

Integrated baseline update: root fetched dev and fast-forwarded 369a9a760 -> 63e3b4239d9a803ea0af3d74f07c3f24d9ed7303 (#448 PR455). Local audit findings and unrelated untracked contract map/proposal preserved; changed upstream files limited package-map/protocol and outer writer/test. #448 issue still OPEN while implementer completes integrated checks/cleanup; live turn cursor47 reports integrated outer/Route/State race PASS, quick-check live. Rechecked queued #465 at current closed_outer_bridge.go330-359: child CLOSE Accept and removal still precede serialized terminal write and closeInput. Existing #465 retained unread-buffer accounting defect persists after #448; no new duplicate and no claim #448 fixed it. #465 needs separate slot after cleanup; #446 completed bounded candidate also needs allocated independent review, capacity remains five including root.

Review preparation #446: exact723d83331 three-file delta inspected at freshdev369; producer adds only exact owner.slice alongside existing bounded Node/Source services. New regression extracts actual Configure-OwnerSlices/Read-OwnerCounterSamples/Stop-OwnerSlices AST functions, retains real producer argument and output shape, mocks only remote systemd/clock/SSH, evaluates both actual consumers, explicitly does not qualify installed campaign. Tests reject unsupported units before counter access. This root preparation is not either independent mandatory review verdict; reviewer allocation awaits a released slot. Updated external queue inventory with465470472473474 and caller/proof constraints to avoid losing prepared work while CI/integration occupy capacity.

Service workload audit dev63e3b423: actual installed worker launch selects DocumentWorkloadBounds or StreamQualificationWorkloadBounds before beginJob. Current owner explicitly requires 512-byte Reader request /4MiB+13 response or fixed64MiB per qualification direction, shared opposite directions for Publisher and logical counters preserved across recovery. WorkloadBounds validates nonzero ceilings and rejects unknown surface; no changed cap or current contract contradiction established. Resource observer keys/counters are internal high-water diagnostics, not additional authority. #448 integrated quick-check and outer/Route/State race now reported PASS while final acceptance/worktree archive still executing; slot remains occupied until terminal proof.

#448 final acceptance independently verified: issue CLOSED via authenticated gh, PR455 merged63e3b423, owning worktree path Test-Path=False. wait_threads cursor54 terminal completed/idle with final acceptance+all owned containers removed. Root archived completed chat tool archived=true. Freed fifth slot allocated read-only independent #446 Spec reviewer /root/review_446_spec exact base369a9a760 candidate723d83331; no implementation/reviewer overlap increase, Standards axis follows sequentially under PO cap. #449 temporary probe candidate20b8e2103 now in actual CI37000220260; source group numbering changed, implementer explicitly rejects falsely equating affected7 with original failing group. Full36996546683 PASS retained, original affected FAIL2/2 not waived; probes removed before final acceptance delta.

Publication restart negative audit currentdev63e: prepareRoot under exclusive root lease invokes cleanupStaging for recognized .stage-/.current- entries before restore. Normal publish withdraws/drains/removes prior persisted generation before successor writes; restore bounds generations to2, not a supported128 canonical record set. Therefore Reachability #470 full128+one staging counterexample does not transfer to Publication. Root cleanup scan128 alone cannot establish naturally accumulated129 staging because each reopen cleans prior stages. No duplicate staging bug filed. Full actual power-loss qualification remains explicitly unclaimed in current owner.
#446 independent Spec terminal0, Standards now independently active; exact review receipt external ardents-446-review-result.md. #449 controlled diagnostic plan acknowledges455 changed PR merge base confounds new PASS; additive dispatch-only probe must retain unchanged full gate and disappear from final delta.

#446 independent Standards terminal2P2 delivered to implementer: exact checked profile prerequisites pwsh/python3 absent, subprocess lifecycle unbounded without descendant cleanup/join/residue. Spec0 preserved separately; changed test/profile delta requires re-review, no gate waiver. Freed review slot selected #465 implementation thread01a0fc5e-37d4-7d22-bb5d-8418a3c81d74 title Outer Close: учёт непрочитанного буфера (projectc47ff0dc). Brief/current63e source/actualbuffer-governor acceptance, preserved448fix, full unchanged gates, devmerge/acceptance/owncontainer/worktree cleanup all handed off. Thread link emitted once. Root+449453446465=five; no active reviewers. Exact #462 payload authorization remains pending, not inferred from automatic continuation.

Qualification workload static audit dev63e: sendScheduledElapsed derives fixed offered workload from elapsed time (bounded600s), partitions target/remainder across declared active connections, uses available credit and one frame per turn; terminal requires delivered fixed offered load before EOF. Profile definitions16/64 active and10/40Mbit aggregate share exact47,343,750 offered bytes/active connection, below fixed64MiB Service bound, so aggregate750MB vs per-stream64MiB is not a contradiction. Actual worker validates deterministic offset-sensitive bytes; canaries challenge/echo match and pending completion, terminal cleanup joins reader. Current workload verifier rejects absent identity/active counts and failed completion; no new confirmed scheduling defect. Potential different recovery/canary timeout semantics need explicit current-owner requirement before filing; active workload recovery8s alone does not prove retained-canary2s policy is erroneous. No new issue from this ambiguity.

### F115 — Failed-verifier general usage omits mandatory journals

Confirmed static current dev63e3b423: main.go61 advertises verify-failed-net14v with5/6 arguments while failed_net14v_linux.go30-31 requires7/8 including reader-journal.jsonl and publisher-journal.jsonl after failed relay results. Actual maintained PowerShell caller and subcommand usage already pass both. Following general usage deterministically reaches argument-count refusal; no evidence or gate bypass needed. Registered GitHub #475 https://github.com/dianabuilds/ardents-network/issues/475 in C0 milestone. Bounded CLI text repair only; do not make journals optional or weaken failed evidence. External brief ardents-failed-verifier-usage-issue.md; no repair thread at current five-slot cap. Coordinate qualification main.go scope with queued442/474 or a later separate selected thread. Source-level documentation defect, no runtime/campaign qualification claim.

Owner-resource failure path audited currentdev63e: /proc/PID/statm error originates ownerResidentBytes invoked by MeasureOwnerCgroups, not sampleProcess pressure counters. It explicitly invalidates a disappearing-process sample; node.hostingPressure converts incomplete measurement to drain. Qualification Measurements serializes worker registration/retirement with sampleMu and explicitly rejects retired owner. Therefore saved #446 QUIC test /proc failure is not itself proof of incorrect ordinary worker teardown: actual PID identity/cgroup ownership and sample timing are needed. No production bug or baseline exemption filed; trace delivered implementer as diagnosis evidence, current #446 scope unchanged.

Resource ownership audit: actual Endpoint qualification Measurements sampleMu serializes sampling with add/retire worker; retirement clears cached sampledAt and marks owner invalid, so prior cached samples cannot silently outlive unregister. Sample clones interface slices. Resource owner measurement deduplicates process IDs across descendant inventories and excludes worker paths already inside prior cgroup. Actual accepted worker paths are sibling leaf units under fixed owner roots, so speculative ancestor reverse-order double-counting is not an established maintained caller bug. Qualification opening-slot channel serializes nextOpening updates through release and applies300ms after remote result; distinct setup concurrency slots do not bypass that pacing. No new confirmed numerical/accounting defect in this slice.

#446 repeat-review preparation: resolved exact candidate688be41d1d9a61138f7b646e2203f751c8f95df2 parent723d83331 base369a9a760. Correction changes only owner_sampler_linux_test.go, testing.md, profiles.json: explicit Linux python3/pwsh prerequisites developer/deterministic/race;30s CommandContext isolatedpgid, group SIGKILL cancellation,1sWaitDelay, joined command result,3s bounded residue probe and independent cleanup error join. Actual pwsh->Python cancellation fixture plus injected cleanup error path added. Root source preparation does not replace independent Standards/Spec re-review or gates. Original two findings remain open for reviewer confirmation on changed delta; current active threads root449453446465=five, no extra reviewer spawned.

#462 additional static caller coverage, current dev63e3b423

The maintained Node Issuer starts credential.StartClosedTokenListener with a real shared Carrier in internal/node/issuer/listener.go:88. Accepted Node and direct connections, capacity refusal, cancellation and terminal worker cleanup all call ClosedTokenListener.closeCarrier (closed_token_listener.go:173-266). That function drops the entire Close result whenever errors.Is(err, net.ErrClosed), although quicNodeCarrier.Close (node_carrier_quic.go:32-34) joins stream.Close and connection.CloseWithError. Consequently a composite benign-closed plus independent cleanup failure is not retained in listener.closeErr or returned through listener.Drain to closedIssuerServer.run/drain. This is another caller of existing #462, not a separate defect.

Existing TestClosedTokenListenerDrainRetainsAcceptedCarrierCloseFailure covers a standalone injected failure for Node/direct capacity refusal and admitted Node cleanup. It does not cover errors.Join(net.ErrClosed, independentFailure); mixed-error and benign-only coverage must be included in #462 acceptance, using the actual accepted-listener/Drain lifecycle and preserving completed join semantics. Static proof establishes the filter and real composition path; no new runtime RED or natural QUIC mixed-failure frequency is claimed.

Keep this addendum local: the prior exact #462 publication approval remains pending. Do not retry a rejected external comment or create a duplicate issue as a workaround.

Recovery TLS cleanup static follow-up, dev63e3b423: actual native Stream opener in internal/endpoint/service/stream.go:145 invokes openRecoveryAttachment; maintained serviceRouteRecoveryOpener returns openJoinedTransport result. joinedTransport.Close caches Conn.Close plus finish result and releases its exact acquisition. secureClient/securePublisher tls.go:55/65/72 discard raw.Close results on setup failure; openRecoveryAttachment attachment.go:129 disables its fallback ownership because TLS setup closed raw. No native Attachment is constructed, so its exactly-once retirement callback cannot retain this failure. This is pre-transfer cleanup, distinct from post-transfer F23 and #462 benign mixed-error filtering. The incomplete replacement path attachment.go:98 also discards Close, but maintained opener supplies a prepared nonzero digest, so natural reachability of that guard is not established.

Before filing: demonstrate the actual recovery Binding/opener path with independent joined Route cleanup failure and failed TLS setup, for both roles; check whether a later successful proposal can erase the setup cleanup failure because stream_recovery.go retains only last opener error. Do not claim ordinary TLS handshake refusal itself is a bug, or infer installed recovery qualification from an injected lower transport fixture.

Service recovery negative boundary dev63e: once a native Attachment exists, retireAttachment records its exactly-once callback failure in persistent stream.retirementErr; recovery success does not reset this field. Endpoint service.runNative consumes RetirementResult only after native Done, includes it in finishErr, and Stream.Close joins finishErr after the finished barrier. Existing retirement-result tests cover duplicate retirement, ordinary RunBounded completion, and terminal-tail teardown. Therefore do not broaden failed-TLS pre-Attachment candidate into a claim that successful recovery universally loses previous Attachment cleanup. The confirmed source-level gap remains before NewAttachment: TLS setup drops joinedTransport.Close result and no retirement callback exists for that failed proposal.
Recovery TLS candidate now has a bounded actual-caller acceptance plan in external ardents-service-recovery-tls-cleanup-candidate.md: both roles, actual Binding/opener/TLS failure, exact acquisition cleanup, failed-then-successful proposal, preserve ordinary eligible retry and existing F23. Runtime RED remains outstanding; no additional repair thread selected at five-slot cap.
#449 caller cancellation causal cross-check dev63e: regression explicitly allows bootstrap refusal on context revocation, but requires retained publication owner retry on caller cancellation. Nonzero nested CLOSE creates bootstrap refusal and prefix.observeChildTerminal forwards it to Source channels.fail; root sent exact boundary to implementer for Node reason/lifetime tracing. No evidence yet that Node refusal is incorrect, and neither retaining an ended Handle nor suppressing its terminal cause is an accepted fix.

Additional recovery-cleanup boundary dev63e: stream_recovery.go immediately after opener checks finishRecoveryIfCompleteLocked and returns nil after retiring a returned Attachment, regardless of opener err. When opener returns nil,error after post-TLS authority recheck, openRecoveryAttachment already joins replacement.Close into that error (attachment.go:133-135), but there is no Attachment for retireAttachment to record. A concurrent completed Terminal exchange can therefore discard an already-returned proposal cleanup error, independently of tls.go dropping cleanup at setup. Later successful proposal also replaces last opener error. Ordinary obsolete proposal/opening errors may legitimately be superseded; physical cleanup errors require separate retained ownership/classification rather than treating all opener errors as fatal.

Existing TestStreamRecoveryCancelsProposalAfterTerminalConfirmation deliberately covers concurrent Terminal completion with a successfully returned replacement and nil close result. It does not cover nil Attachment plus opener error carrying failed pre-transfer cleanup. Add this race to the same candidate acceptance matrix and prove actual Endpoint Binding authority/cancellation + joined transport cleanup before making a product bug claim. Do not duplicate #462 or F23; native successful Attachment retirement is correct.
Recovery limits static audit dev63e: episode deadline derives from lastProgress+15s with10ms publication reserve; each attempt uses the same computed deadline and tighter original NoNewRecoveryAfter. authorizationTime advances from original authorized time using monotonic elapsed duration. commitAttachment resets proposals/episodeEnd after authenticated replacement but does not reset original authorization, Work Safety or lastProgress; absent further Application progress, another recovery computes the same original progress bound. Existing deadline unit checks explicitly pin that origin. Therefore reset of proposal count alone is not proof of indefinitely extended work. Local accepted Application read and successfully delivered receive bytes update lastProgress; no current-owner contradiction established for that definition. Generation non-advance is refused at commit, and actual Endpoint opens fresh capsule for request generation; generic arbitrary opener generation mismatch is not established maintained-caller defect.
#446 revised full-check exact688 terminal exit2: Forwarding ParentReaderServesIndependentChildWith255QueuedLanes timed out before aWriteStarted; cmd qualification package passes are narrower evidence, not full acceptance. Root inspected helper: aOpen is after all255 actual downstream OPEN frames; aWriteStarted only after first16KiB BYTES. Timeout text differs from peerDone error branch. Sent implementer stage/deadline/Node terminal tracing guidance and required same-environment baseline comparison before baseline attribution; no timeout/gate waiver. Participant output negative audit: background error channel bounded/latches first failure; runInterfaces pendingFailure defer is registered before resource teardown defers, so it runs after teardown and retains late joined-owner delivery failure. Ordinary ready output failure returns directly; no new issue from cancellation-vs-channel select alone.
# Candidate: Node lifecycle event writer panics on ordinary Linux backpressure

Current dev63e3b423, internal/node/event_writer_other.go:28-29 adds syscall.Write count to written before classifying EAGAIN/EWOULDBLOCK/EINTR. Go1.26.8 syscall/zsyscall_linux_amd64.go:957 assigns n=int(r0) even when errno is nonzero; internal/runtime/syscall/linux/asm_linux_amd64.s:38-40 returns r1=-1 on error. Thus a full nonblocking pipe/journal socket yields written=-1 on its first failed write. If context remains live until next5ms tick, raw[written:] panics. On interrupted later write, position also regresses and can duplicate/truncate an event rather than preserve its offset. No malicious input or security condition required.

Maintained caller: cmd/ardents-node/node_mode.go installs nodeEventEmitter(boundedOutput,...); that adapter calls node.EventEmitter -> writeEvent. Node emitState/resource uses explicit bounded context. Selected stdout pipe or systemd journal socket backpressure is a normal output failure condition and should wait within deadline then return error, preserving lifecycle joined shutdown; it must not panic. Existing journal socket test checks only a writable socket, not saturation. F33 ownership audit does not cover this error arithmetic.

Acceptance: actual writeEvent on filled nonblocking pipe/socket with live short deadline must return deadline error without panic or negative count; a draining peer must receive exactly one complete bounded event with no duplicated prefix, partial offset correct; closed peer must retain its actual write error. Preserve selected platform behavior and deadline requirement. Add tests only through checked existing execution profile. Runtime actual-function reproduction remains outstanding; no thread at five-slot cap.

Runtime reproduction 2026-10-02: Go1.26.8 linux/amd64 in approved image0ecc, UID10001, own --rm container. Unchanged writeEvent body copied from source SHA256 B27D9C70C58E1E070E17C3764351036FA774029194548A8B0FA0EEE62D0713BB; only package changed to main and bounded filled-pipe harness appended outside repo. Harness SHA256 FC653A24D94BD3A6542DB556F7141B5C5441CA4507CED8ED860A21C62A042E5C. Actual output: REPRODUCED writeEvent panic: runtime error: slice bounds out of range [-1:]. Live100ms context, no reader, ordinary nonblocking pipe saturation. Confirms writer arithmetic defect, not full Node process/systemd qualification. First container attempt used default diagnostic entrypoint and failed unknown diagnostic operation (invalid invocation, not test result); corrected explicit /bin/sh invocation reproduced. Both containers auto-removed. Maintained runtime unmodified.
### F116 — Node event writer negative syscall count under backpressure. Actual Go1.26.8 Linux copied-body filled-pipe reproduction confirms panic slice bounds [-1:]. Registered GitHub #476 https://github.com/dianabuilds/ardents-network/issues/476 in C0 Closed Alpha after explicit Product Owner approval of the exact externally published defect payload. External complete brief ardents-node-event-negative-write-issue.md. No implementation thread admitted yet; active chat limit and implementation WIP selection/ledger are separate under updated AGENTS.md.
New candidate from #453 failed-fixture diagnosis: orphaned queued CREDIT after child CLOSE may exhaust Forwarding control queue around212-223 real issuer exchanges before refill. Root source check closed_forwarding_channel.close queues CLOSE, deletes child and releases queued data/reverse controls, but does not itself remove existing forward controls for that lane. Consumer reachability and actual retirement/available predicate remain required before confirming leak; implementer preparing deterministic RED and separate bounded receipt. Coordinator explicitly prevents silently adding shared Forwarding implementation scope to453 cancellation slice. No new tracker/thread yet.
#449 concrete candidate causal ordering independently source-checked: incoming CLOSE deletes Route child, late reverse CREDIT returns ErrClosedForwardingChildRetired, copyReverse deferred link.stop retires session lane/reverse queue, while drainForwarding still must emit the queued local CLOSE. Candidate449 acquireWriter treats reverse.closed without peer-CLOSE witness as cancellation; this can reject required terminal emission. Root relayed explicit local-vs-peer retirement distinction and retained parent/deadline/physical-failure acceptance. Runtime deterministic RED remains implementer work; symptom attribution still pending. Separate #453 orphaned-controls hypothesis shares retirement area but requires separate reservation/consumer proof.
# Confirmed queued defect: child CLOSE leaks unreachable Forwarding CREDIT controls

Current dev63e3b4239. Owner internal/route/closed_forwarding_channel.go close deletes child and refunds data/reverse controls, but leaves previously queued forward CREDIT controls. Node eventAvailable can no longer admit those controls after link retirement/deletion, so their controlBytes and shared governor reservations cannot be reclaimed by NextAvailable. Root inspected exact current close/queueControl/dequeueControl and actual Node availability code.

Evidence inspected: ardents-453/orphaned-controls-defect-receipt.md. Deterministic baseline RED retains16 orphan controls/320 bytes and no live child, then refuses a supported live OPEN below the unchanged16KiB capacity. Actual Node availability observation confirms retired CREDIT=false and sibling CREDIT=true. Failed real issuer Control exchange fixture reproduces queue-unavailable firstcause before client failure in3/4 controlled QUIC cases,212-223 exchanges. Exact source/test hashes and raw logs are in original receipt. No installed workload qualification claimed.

Bounded repair: release only retired child's superseded queued/unemitted CREDIT controls; preserve sibling work, undispatched OPEN cancellation/join responsibility, selected queue ceilings, actual physical writer accounting and terminal-close ownership. Acceptance: exact baseline RED->GREEN, correct governor refund once, sibling controls delivered, full248x66-byte OPEN boundary restored and249th refused; late/repeated retirement and cancellation, actual affected issuer exchange bothCarriers. Preserve previous gate failures; no timeout/queue increase.

Coordination: this is distinct from453 Source caller cancellation and449 writer acquisition/terminal ownership. Route channel owner also intersects queued472; sequence after449 final delta or explicitly verify disjoint actual changes. Diagnostic experimental patch was exported outsideGit and removed from453; no new implementation slice selected/admitted, no implementation thread created. External publication of this new payload has not been approved by automatic review; do not reuse rejected453-comment route as workaround.

Forwarding queued OPEN/CLOSE static negative boundary dev63e: OPEN event retains decoded destination/restriction independently of child map; drain starts tracked asynchronous opening, pending CLOSE is available via openings.pending, cancelLane cancels and collects its exact result before retiring that lane, while retaining preceding sibling successful results. Existing cancellation test covers earlier B result preceding canceled A result; physical cleanup errors remain retained. Thus deleting Route child before OPEN dispatch does not alone prove missing opening authority or a parent failure. Orphaned-CREDIT repair must preserve queued OPEN and its tracked cancellation/join path; simply deleting all lane controls at CLOSE would remove maintained opening ownership work. A natural opening failure racing local close can still need actual producer evidence before claiming independent additional bug.
Integration queue coordination:446/465/453 instructed to finish already necessary gates, supply compact exact-HEAD handoff, and end status-only working turns if solely awaiting review/integration/publication; no live gate canceled.465 exact6607ac563/base63e clean four-file delta independently pinned/prepared, not reviewed. Actual docker inventory confirms four owned active task containers449/446/465/453; root476 reproduction absent, five stopped foreign swarm-zulip containers preserved. No extra implementation admission or gate waiver.
465 pre-review source audit6607: exact ardp CLOSE validity gates incoming discard; local discard happens before reservation release; no transport deadline shortening in discardInput. Read drops lane mutex before handshake/physical CREDIT, avoiding newly introduced bridge/lane inverse-lock ordering. Original448 physical selected CREDIT vs terminal deadline remains governed by final closeInput after terminal write. No new defect established in examined hunk; independent axes/full gate outstanding.
