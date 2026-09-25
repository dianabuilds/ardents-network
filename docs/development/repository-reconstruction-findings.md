# Reconstruction findings

Status: **working evidence ledger** for the [repository reconstruction](repository-reconstruction.md).
These are source facts and bounded inferences, not an instruction to delete or
move code. A finding is updated when the active Endpoint/network task changes
its evidence.

## F-01: Naming runtime retained without a production path

**Code fact.** The Linux/Windows package import graph has no path from any of
the seven `cmd` packages to `internal/naming/resolution`,
`internal/naming/alpha/private`, or the root `internal/naming/namespace`
package. Together those packages contain 24 non-test Go files and 2,142 source
lines, plus 26 package test files. `cmd/ardents` imports Resolution only in
`name_retirement_fixture_test.go`, where it serves a zero-effect refusal oracle.
The Namespace subpackages and local `internal/naming` encoder have separate
production callers and are not included in this count. `internal/architecture`
is the fourth package without a command path; it is a test gate, not a runtime
candidate.

**Contract fact.** [ADR-0090](../adr/0090-retire-name-operator-network-adapters.md)
retires `name resolve` and `name control` at command dispatch while explicitly
retaining the uncomposed Resolution Module until an exact-consumer decision.
The current [Naming owner](../technical/naming.md) also states that no
production runtime composes a Gateway or Resolver, or the whole Namespace
transition sequence.
The [package map](package-map.md) explicitly calls `alpha/private` historical
and says no production package imports it. Thus these files have a known
retention reason or unresolved decision, even though they have no C0 command
path. Removing them solely from the import graph would violate the current
decision route.

**Reconstruction question.** Which exact compatibility behavior or future
selected Name behavior needs executable source in the maintained tree, as
opposed to tests, immutable evidence, and Git history? Resolve that against
ADR-0090 and the Naming owner before choosing retain, move to historical
evidence, or remove. Preserve the refusal oracle or replace it with equally
strong zero-effect evidence if the Module is retired.

## F-02: Dependency register describes a retired import path

**Code fact.** Current `cmd/ardents` production imports do not include
`internal/naming/resolution`. Its `name resolve` and `name control` dispatch
returns a retirement refusal. The [dependency register](dependencies.md)
records a dated 2026-09-22 projection that names
`cmd/ardents -> internal/naming/resolution -> circl/ohttp`; this is not a
current import path. At `e48d4c3c`, an offline Linux/amd64, cgo-disabled
`go list -deps -e` projection returned 357 packages with no package errors.
CIRCL remains reachable through `endpoint -> service/reachability`; OHTTP
through `endpoint -> route/credential`; QUIC through `route`.

**Action.** Keep the dated result as historical evidence and add a complete
current projection to the dependency register after classifying all modules.
Do not edit `go.mod` based on the obsolete Naming path.

## F-03: Platform selection changes the architecture graph

**Code fact.** All 57 current `cmd`/`internal` packages are registered in
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

**Repository fact.** Of 201 tracked `docs/` files, 86 are ADRs and 66 are
research records. Seven research records declare an open, deferred, or
draft-not-ready state; they cannot supply selected decisions. The remaining
records provide decision evidence, while the current reader route starts with
product/security and affected technical and engineering owners. There are 41
current-location documents, plus indexes, receipts, and fixtures. Three ADRs
are only proposed, one withdrawn, and six superseded or partially superseded.

**Inference to test.** The maintenance cost is not simply 201 files. It is
the number of current facts copied across owners, the length of the normal
reading route, and how often a code change requires editing several owners.
The documentation pass will trace selected claims and links to identify those
specific duplicates and stale descriptions before proposing consolidation.

## F-05: The deadcode gate explicitly carries a large uncomposed surface

**Repository fact.** `scripts/check-deadcode.go` compares the production
`deadcode` result with `tests/profiles/deadcode-allowlist.json` for both Linux
and Windows. The current allowlist names 529 common function symbols: 393 are
classified `unwired`, 132 `retained`, and four test-fixture support. Its
largest common class is 191 symbols in the unwired Namespace-control tracer;
another 79 are shared legacy Route/Entry/Credential/Reachability/Service
leaves. Windows adds 356 platform-specific symbols, including 315 from the
selected Linux-only text runtime. These are **symbol counts**, not file or
line counts, and the Windows-only group must not be treated as unnecessary
Linux code.

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
including thirteen candidate getters indexed separately.

**Design inference.** This is a shallow Interface at the State/Node seam: a
caller must learn many getters to obtain one immutable duty decision, and State
and Node maintain parallel field projections. The code is a plausible source
of change amplification whenever the accepted duty facts grow.

**Target to evaluate.** Return one copied, validated, immutable duty-fact value
at this seam, with a small acquisition Interface and exact status/error modes.
Preserve State's ownership of freshness/conflict checks, Node's local copy,
bounded candidate/authority counts, and the absence of mutable State internals.
Confirm all production/test callers and the Go import direction before making
that concrete; replacing the getters is a refactor, not a permission to weaken
authentication or expose the full Snapshot.

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

**Disposition to decide.** Keep both behaviors until the retained Invite
command and compatibility obligations are traced. Then choose whether one
deep Entry Module with two explicit constructors remains clearest, or whether
the independently owned C0 set earns its own package and exact import rule.
File count alone does not decide this seam.

## F-09: Resource has two necessary but different lifetimes

**Code fact.** `resource.Guard` checks process placement and samples pressure;
Network State and Node construct it with `resource.New`. Separately,
`resource.Hosting` owns a durable shared provider-period root, charges
interface-counter deltas, and retains work/termination reservations until the
consumer joins and releases them. `ardents-node hosting initialize` creates
the period; Node opens it for forwarding and qualification reads it. Both are
used by selected C0 paths, but one is a volatile process governor and the
other a persistent multi-owner ledger.

**Design inference.** The shared package name `resource` is accurate at a
topic level but does not tell a caller which lifetime it owns. A target design
should specify separate Guard and Hosting Interfaces and their close/error
contracts. A package split is useful only if it removes actual cross-owner
knowledge or dependency; it is not required by the 21-file count itself.

## F-10: Current architecture owners are hard to read and maintain

**Repository fact.** `docs/development/package-map.md` is only 83 lines, but
its Endpoint row is 6,756 characters long. It mixes package responsibility,
selected behavior, shutdown rules, platform caveats, and an import list in one
table cell. The working `network-core-transition.md` is 1,545 lines; the
current `endpoint-service-runtime.md` is 654 lines. These measurements do not
establish that their content is dispensable, but they do show that document
count alone understates the cost of locating and updating an architectural
fact.

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

## F-13: Historical Alpha resolver is a bounded retirement candidate

**Code fact.** `internal/naming/alpha/private` has seven production files and
two package test files. No production package imports it; the architecture
test instead forbids Endpoint from importing it. The common deadcode allowlist
names 22 of its functions. `internal/naming/alpha` separately owns the retained
Alpha corpus parser and persistent read-only floor, and `ardents-control`
retains supplied-bytes inspection.

**Decision boundary.** [ADR-0088](../adr/0088-retire-alpha-service-links-and-corpus-intake.md)
retires all accepting Alpha Service Link paths and explicitly retains existing
floor bytes with their parser/reader until a data-retention decision. It does
not identify the OHTTP Client/Relay/Gateway implementation as necessary for
that compatibility obligation. The package map still says to retain that
historical exchange, so its rationale must be reconciled with the higher
authority ADR before removing source or tests.

**Separation proof.** The package owns OHTTP message/profile handling and
`Client`, `NewRelay`, and `NewGateway`; its `CorpusFloor` is only a supplied
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

**Proposed disposition.** Retire the seven production and two test files of
`alpha/private` in one bounded architecture change, together with its package
map row and 22 common deadcode allowances. Preserve the `internal/naming/alpha`
parser/floor and the external refusal/inspection tests; verify their Linux and
Windows behavior after removal. Pin the last source revision as historical
evidence. The package-map statement to retain the exchange is lower authority
than ADR-0088's retired accepting path and should be reconciled in that change.
OHTTP and CIRCL have other consumers, so this removal alone does not remove
those dependencies.

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
or persisted history of its own. The old description of a totally blind
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

**Contract and evidence gap.** The current Node technical owner requires
`FAILED` rather than `WITHDRAWN` when cleanup is unproven and requires known
root-close errors to propagate. Existing issuer tests cover successful
successor drain and direct/Node-authenticated bootstrap, but the inspected
issuer/listener tests do not exercise a delayed accepted child past the drain
deadline. Before moving this code, select one owner that retains issuer and
spend roots until the last worker joins, including after caller timeout; test
the timeout, eventual close, retry/restart exclusion, and result/error path.
The observed timeout is an unproven-cleanup outcome, not evidence that an
unauthorized second issuer can start or that a key is exposed.

## F-18: A proposed network-core design carries a second execution narrative

**Document fact.** `network-core-transition.md` is 1,545 lines and about
17,200 words. Its status says the design is proposed, not accepted. Sections
3–5 contain the candidate architecture, file mapping, and compatibility;
sections 6–11 (starting at line 748) include iteration ordering, issue links,
the next post-merge step, test matrix, gaps, handoff, historical cleanup, and
an update check. Its 484-line wire appendix is also explicitly unaccepted.
`docs/development/README.md` links this proposal as the development reading
route, while `documentation.md` assigns live C0 execution state to GitHub
Issues and unique current facts to current owners.

**Design consequence.** The proposal may contain still-useful decisions, so
length alone is not a deletion argument. The overlap of proposed design,
historical transition, and live execution narrative makes it costly to tell
which statements are obligations today. Reconcile each proposed mechanism
against the accepted ADR/current technical owner and the current issue; retain
only unresolved design questions in a short working proposal, promote any
accepted unique facts to their current owner, and retire obsolete execution
chronology to Git history with links repaired. The wire appendix stays a
candidate until its own decision, not a hidden C0 protocol requirement.

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

## F-22: Reachability retains an uncomposed OHTTP facade beside the v3 path

**Code fact.** The installed closed resolution operation uses
`Store.PublishPrivate` and `Store.LookupPrivate`; the Endpoint verifies and
issues v3 private Descriptors. A search of non-test `cmd`/`internal` Go callers
found no caller of the exported `reachability.NewGateway`, `NewRelay`, or
`OpenClient` OHTTP facade. The common deadcode allowlist names 35
`internal/service/reachability` symbols across its unwired reachability and
shared-legacy groups, including these constructors, their request/response
helpers, and legacy `Store.Publish`/`Store.Lookup`. This is an exact-symbol
finding, not proof that all 20 production files in the package are removable.

**Compatibility fact.** `Store.PublishPrivate` and `LookupPrivate` share the
Store's durable generation/conflict machinery. On reopen,
`store_files.go` still decodes old Descriptor records and calls legacy
`Verify`, so absence of a new legacy publication caller does not make the
old read path disposable. The current [reachability owner](../technical/private-reachability.md)
first describes v3 and then keeps an approximately 180-line generation-2
section. The [package map](package-map.md) records both generations in one
package, while the current C0 scope keeps accepted persisted identities until
an explicit migration retires them.

**Reduction boundary.** Separate the uncomposed OHTTP Client/Relay/Gateway
adapter from the old Descriptor decoder and durable floor reader in the
consumer/compatibility map. A bounded retirement can remove the adapter and
its deadcode allowances only after checking exact test/refusal evidence and
the accepted migration decision. Keep old-format read and refusal behavior
until the persisted-root contract is decided. Make the technical owner's
generation headings an explicit installed-v3 versus retained-generation-2
reading route so an implementer need not infer the active transport from
historical text.

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
distinct replacement-close error and asserts it reaches the final Application
outcome. This is a source-level propagation gap, not a claim that every close
fails or that first-Attachment cleanup is lost.

**Architecture decision.** Make the replacement transport's close result
observable by its Endpoint owner after native Connection joins it, or give the
native Attachment close boundary an error-bearing result. Preserve exactly-once
physical retirement and the authenticated terminal ordering. Verify an
injected replacement-close failure through the Endpoint terminal outcome and
the retained Context error before moving Service/Route boundaries.

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
text command test instead writes a root-owned **system** unit with
`User=ardents-endpoint` and `ExecStart=... endpoint headless ...`; it checks
the active InvocationID. The package E2E test verifies root-owned installed
program/static enrollment bytes and exercises `enroll-installed` under an
unprivileged UID, but it does not start the protected text system unit.

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

## F-28: Offline commands inherit Route's network dependency closure

**Build fact.** At `e48d4c3c`, an offline Linux/amd64, cgo-disabled
`go list -mod=readonly -deps -e` returned zero package errors for both
`cmd/ardents-control` (340 packages, 22 external modules) and
`cmd/ardents-custody` (292 packages, 13 external modules). Both closures
include CIRCL, OHTTP and QUIC. These are static import paths, not evidence
that either command executes network operations.

**Source fact.** Control's only production use of `route/credential` is
`DecodeClosedIssuerProfile` in its public, offline issuer inspection. Custody
uses `AllocationRole`, `DecodePermissionRequest`, `Permission`, and
`EncodePermission` for offline admission allocation. The latter permission
grammar uses only the standard library; issuer-profile decoding calls State's
SPKI validator. The same `route/credential` package also owns the OHTTP
Transit client and imports the parent `route` package in five production files;
Route imports QUIC. One broad package import therefore carries unrelated
network code into these command closures.

**Target boundary.** Preserve the exact signed profile and permission bytes,
validation and independent authority checks. Put their read/encode operations
behind a cohesive offline contract so Control and Custody do not import the
networked issuer/Transit adapter. Prove the dependency reduction by repeating
both command projections after that boundary change; do not remove OHTTP or
QUIC from the Node/Endpoint path on this evidence.
