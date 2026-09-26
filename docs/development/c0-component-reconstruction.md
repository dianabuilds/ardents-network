# C0 component reconstruction

Status: **review-ready architecture proposal** for rebuilding the maintained code
around the selected journey. This is not a new product contract, package
registry, delivery ledger, or authorization to replace persisted formats.
Initial file inventory: `codex/architecture-refactor` at `50026274`
(2026-09-25). The package graph and file inventory are now reconciled to
`53f02e64`, which includes the adjacent task's accepted ADR-0092 generic
Publisher/Transit retirement. Selected behavior traces retain their inspected
revision labels. The remaining Route v2 and Service plaintext retirement
questions below are separate from that completed Endpoint change.

## Architecture decision checkpoint

The repository-wide inventory is sufficient to choose a direction without
reading every source and test file in sequence. The selected journey needs the
fourteen ownership components below, but does not require fourteen new Go
packages or a rewrite from an empty tree. Keep each existing deep module
whose authority and lifetime are coherent; split the crowded Endpoint, Node,
and Route roots only at a proven caller/ownership seam. One supported runtime
version per contract is the target; old bytes remain only under an explicit,
bounded migration or refusal rule (F-08/F-32/F-42/F-50).

| Decision now | Evidence and unresolved gate |
| --- | --- |
| Preserve State, Instance, Publication, Reachability and Custody as single authority/floor owners. | Their durable roots and transitions are cohesive; package size alone does not justify splitting them. Repair identified close-result and old-root handling gaps before moving their files (F-19/F-32/F-68/F-71/F-72). |
| Rebuild Endpoint, Node and Route around admission, duty, Carrier, wire, client path and resource lifetime. | The focused 360-file inventory assigns a concrete responsibility to every file; a package move still needs the caller and import direction, especially Route/Credential and Endpoint qualification. The adjacent Endpoint/network slices must be integrated first. |
| Stop alternate writers/execution, then close each retained-root obligation. | The old Route v2, Invite Entry, Reachability Descriptor, Instance Introduction key and multi-version Epoch intake have different last callers and persisted floors. Their retirement gates are listed in the one-version table below; removing parsers wholesale would lose authority history. |
| Treat the installed Ubuntu Publisher-to-Reader path as the acceptance boundary. | Individual component tests and `make check` do not establish artifact-to-system-unit identity, restart, and both Carriers on one candidate (F-27/F-31/F-67). This is an implementation/integration gate, not a reason to postpone the architecture decision. |

Further source review is decision-driven: inspect an exact caller, root, test,
or current document only when a proposed move, retirement, interface or
compatibility rule depends on it. Raw counts of source-traced files and tests
measure evidence coverage, not progress toward a desired percentage. The
subsequent contract and implementation work is to close the specific gates
in the table, reconcile finished parallel changes, and record each selected
boundary in its current owner and package map.

### Short execution route from this review

This is the order of **decisions and bounded changes**, not a second issue
ledger. There is one active C0 implementation issue; the ongoing network and
Endpoint work retains its current owner. A later architecture slice starts
only after that owner's completed delta is reconciled with this map.
At the inspected snapshot, the main committed HEAD is an ancestor of the
architecture branch, while the sampled #253, #277, #279, #281, #282 and #285
issue branches have separate commits based on an earlier common point.
Therefore the integration boundary is each completed issue delta, not a blind
copy of either worktree or an assumption that a green result on one branch
qualifies their combined candidate.

| Order | Decision and change | Evidence that closes the slice |
| --- | --- | --- |
| 1. Preserve and reconcile | Keep the accepted State, Custody, Instance, Publication, Reachability, Service Connection and local Application owners. Compare completed network/Endpoint changes against the 360-file owner inventory before touching their source. | One integrated revision, exact changed-owner list, unchanged authority and durable roots; no duplicated fix from another worktree. |
| 2. Correct authority before moving code | Join each signed Epoch role to the closed Node profile; pin one new closed AREP intake version while defining old current/pending/predecessor recovery; retain terminal close errors at the actual owner. | Refusal before effects for mismatched role/old new-intake bytes, restart with retained floors, and observed final cleanup results. These are separate implementation slices under their existing owners. |
| 3. End alternate execution | For old Route v2, Invite Entry, old Reachability writer, Instance Introduction key and legacy Installed startup: stop the last writer or opener, decide each persisted population's migration or typed refusal, then retire its reader when its obligation ends. | Last production caller and stored floor named for every removal; one active C0 version per contract, no historical runtime fallback. The older Installed command waits for the proved protected successor. |
| 4. Make ownership visible | Deepen Endpoint participant/qualification, Node duties, Route client path/Carrier/protected wire/receiving spend in their current packages first. Extract only a cohesive boundary with a real non-test caller and acyclic import graph. | Resource transfer and close owner, smallest caller Interface, behavior tests, `doc.go` and package-map edge in each actual extraction. Filenames follow the final owner rather than a bulk `text_` or `closed_` rename. |
| 5. Reduce verification and diagnostic friction | Consolidate only byte-identical fixture copies; register the missing generator checks in the selected profile; keep Node/Endpoint security-specific event envelopes and expose bounded causes and late cleanup errors. | Existing expected results remain independent, checked profile covers each canonical fixture, and a failed installed journey identifies its state transition and owner without a diagnostic rebuild. |
| 6. Prove the supported installation | Bind accepted artifact/Release, root-owned plan and system unit, service account, MainPID and fixed worker identity into one operator route. Then run installed Publisher-to-Reader and restart on TCP/TLS and QUIC. | No manual JSON or fixture key, real Service readiness and terminal outcome, retained floors, both Carriers, `make check` and recorded installed evidence on the same candidate. |

The architecture study can select these owners and order without running the
future installed candidate. Contract changes in rows 2, 3 and 6 still require
their own accepted owner decisions before implementation; this table does not
silently select a migration or weaken an existing refusal.

## Starting from the required result

The selected [C0 readiness profile](../product/scope.md#c0-closed-alpha-readiness-profile)
and [protected text workload](../product/protected-service-workload.md)
require one installed Ubuntu Publisher to expose an immutable text document,
and a second Endpoint to open an explicit Target Link and read it. The candidate
uses both TCP/TLS and QUIC Carriers. Stop/restart must retain required durable
state and return an honest terminal outcome.

The journey has four phases:

1. Verify the artifact and enrollment, establish current Network State and
   separately held authority, permission, and Service Instance material.
2. Admit the Publisher, confine its worker, prepare its network prefixes,
   register Introduction, publish the signed Descriptor, and return a Target
   Link only after readiness.
3. Admit a Reader, validate its Target Link, resolve the current Descriptor,
   establish Introduction and JOIN, authenticate the Service, exchange one
   bounded text request/response, and verify the terminal result.
4. Revoke or withdraw new work, cancel and join accepted work in dependency
   order, close physical transports, and report cleanup failure when present.

Every component below is required by one of those phases or by an accepted
authority, safety, or persistence obligation. A component is an ownership
boundary; one component need not equal one Go package or process.

## Minimum component set

| Component | Owns and returns | Current implementation to assess |
| --- | --- | --- |
| Artifact and enrollment trust | Authenticate first-execution executable/static inputs, retain the current-program record across restart, and gate actual protected-Service readiness on that identity under the selected system-manager unit/account. | `internal/enrollment`, `internal/release`, `internal/endpoint/replacement`, `internal/endpoint/portable`, enrollment and headless adapters under `cmd/ardents`. The per-user enrollment and installed protected-text system-unit lanes are not yet composed (F-25/F-27). |
| Network State and time | Publish an authenticated current view and refuse stale, conflicting, or absent authority. Join each closed-profile numeric role to the accepted Epoch assignment before exporting an immutable duty projection; own the State root. | `internal/network/state`, `internal/network/source`. Node's installed Contributor clock marker is joined operational evidence, not State authority (F-33). The role join is currently unproven in code (F-45). |
| Service authority and Instance | Keep Authority signing material separate; issue the public Credential; own the host-local Instance key and generation. | `internal/custody`, `internal/service/instance`, `cmd/ardents-custody`. |
| Permission and token issuance | Issue bounded permission and tokens under separate authority; retain issuer roots and exact retry state. | `internal/route/credential`, Endpoint permission and token-attempt journals, Node issuer duty. These are several trust owners, not one package. |
| Receiving spend | Verify admission at the selected recipient, spend once durably, enforce class and resource limits. | `internal/route/replay`, Route admission channels, Node role-specific spend roots. |
| Carrier | Open/listen on the State-selected TCP/TLS or QUIC profile and own physical connection retirement. Expose authenticated peer/transport evidence. | Carrier implementations and listeners currently inside `internal/route`. |
| Protected wire and channels | Encode/decode bounded frames and operations, multiplex lanes, enforce credit/deadlines, and terminate channels. | `internal/route/ardp`, `internal/route/terminal`, and channel owners in `internal/route`. |
| Client path | Select and retain the exact Source/Introduction/Responder handles from current State, present tokens, perform resolution and JOIN, and join its Route transport. | Endpoint's prefix owners together with client operations in `internal/route`. |
| Node duties | Start one State-authorized receiver from an already joined duty view, accept its work, retain listener/host/session/spend lifetimes, react to pressure, drain, and return terminal cleanup. | `internal/node` plus Route receiving operations and its joined installed-Contributor clock observation. Issuer, forwarding, resolution, Introduction, and Data Join have distinct lifetimes. |
| Service publication and discovery | Own Instance publication, private Descriptor proof/revision, and exact Target Link interpretation. | `internal/service/publication`, `internal/service/reachability`, `internal/service/targetlink`, Endpoint publication coordination. |
| Service Connection | Authenticate the Service Instance and own one ordered logical byte stream, attachment replacement, continuity, and final authenticated terminal state. | `internal/service/connection`; Endpoint supplies authenticated attachments and local lifetime. |
| Endpoint and local admission | Admit Client/Publisher capabilities, bind the current State, permission, publication, route, worker and Application Interface to one local lifetime, and revoke/join children. | `internal/endpoint`, `internal/application/broker`. |
| Confined Application and local interface | Verify worker confinement before network effects; exchange one bounded text request/response through separately authorized local Connection and Administration surfaces. | `internal/application/interfacev2/connection`, `interfacev1/administration`, `textdocument`, Endpoint installed-worker code, `cmd/ardents-text`. `textdocument` is one cohesive fixed-grammar package used by trusted UI, Endpoint and confined worker roles; their process/descriptor authority is enforced by installed launch, not by moving its files to role-named packages. |
| Resource and diagnostic evidence | Measure actual limits, choose protect/drain, emit bounded safe causes and retain cleanup failures. Each owner supplies its own event facts. | `internal/resource`, Node/Endpoint event owners, `internal/diagnostics/timeline`. |

Qualification tools, fixtures, and architecture gates verify this system; they
are not production data-path owners. Compatibility refusals for retired plans
and retained persisted floors are obligations at existing command/storage
boundaries, not second accepting runtime implementations.

### Where the component boundaries meet today's packages

The [focused file inventory](c0-component-inventory.csv) assigns 360 production
files to 17 responsibility tags. These tags describe ownership for the audit;
they are **not** a proposal for 17 new Go packages. The
[package disposition map](repository-package-disposition.csv) separately
classifies all 55 physical packages, including provisioning, verification and
retained uncomposed code outside this focused set. The large root packages
combine several tags, while some existing child packages already have a useful
cohesive boundary:

| Physical package at `53f02e64` | Focused production files by responsibility | Package decision |
| --- | --- | --- |
| `internal/route` (74) | Client path 25; protected wire 13; Carrier 13; old Route v2 12; receiving spend 6; compatibility 4; Node duty bridge 1. | Keep the existing `route/ardp`, `route/terminal`, `route/capsule`, and `route/replay` owners. Extract the Carrier only after its shared-helper closure and Credential issuer import direction are resolved; retire the uncalled v2 closure only with its persisted and refusal obligations. A single root rename would preserve the present mix. |
| `internal/endpoint` (95) | Publication/discovery 20; client path 18; confined Application 15; local admission 15; Service Connection bridge 11; permission issuance 8; qualification 6; diagnostics 2. | Keep Endpoint as the admission and lifetime composer while the adjacent implementation task finishes its selected slice. Then reconcile its actual file moves with this map. Neither a blanket `text_` rename nor one participant child package defines the eight different owners. |
| `internal/node` (45) | Node duty 44; diagnostics 1. | Node remains the process and duty lifetime owner. A receiver child package depends on a real accepted-child close interface, resource-pressure ownership and issuer separation; the current file count alone does not supply that seam. |
| `internal/route/credential` (28) | Permission issuance 28. | Retain the issuer authority boundary. Three live issuer files import Route for wire/channel operations; resolve that direction before extracting Route's Carrier or claiming Credential is independent. |
| `internal/service/connection` (20), `service/instance` (16), `service/reachability` (15), `service/publication` (9) | Service Connection, identity and publication/discovery already occupy distinct packages. | Preserve their present authority and lifetime boundaries; repair the identified attachment replacement and close-result defects inside the owning package. |

Other focused files are in existing narrow packages such as
`endpoint/durableroot`, `endpoint/tokenjournal`, `endpoint/permissionfile`,
`service/targetlink`, and `route/replay`. The inventory's physical file counts
are a navigation aid, not a measure of architectural quality. Any future
package extraction must name a real caller and small interface, preserve one
active accepting version of each contract, and satisfy the package-map and
behavior-evidence requirements in `AGENTS.md`.

**Decision for each of the 17 focused responsibility cohorts.** Counts below
are an exact grouping of the 360-row inventory, not a count of future
packages. “Move” names a resource/interface boundary to implement only when
its stated gate is satisfied.

| Cohort (files) | Architectural treatment | Gate before a move or retirement |
| --- | --- | --- |
| Artifact/enrollment (17) | Retain trust owners; compose their result into installed launch. | Exact Release/plan/system-unit identity (F-27/F-67). |
| Carrier (13) | Move toward one physical Carrier owner; retain Node/Endpoint selection outside it. | Shared TLS/helper closure and Credential import direction (F-30). |
| Client path (43) | Deepen Route prefix/operation lifetime and Endpoint coordination; move only the cohesive lower operation. | Keep Source/Introduction/Responder reservations and JOIN lease/close transfer explicit. |
| Compatibility (4) | Retain minimum readers/refusals temporarily. | Each persisted root or wire identity gets a migration/refusal exit gate (F-52/F-53). |
| Confined Application (15) | Deepen Endpoint worker/admission handoff in place. | Installed unit/account/MainPID and worker close authority (F-27). |
| Durable storage (7) | Retain existing narrow filesystem owners. | No generic filesystem package without a real shared authority boundary. |
| Endpoint admission (15) | Keep Endpoint as local capability and context lifetime owner; split mixed files by responsibility. | Reconcile the adjacent Endpoint slice before choosing any child package. |
| Legacy Route v2 (12) | Retire uncalled execution after extracting shared historical readers/refusals. | Grant ledger and old wire obligations decided separately (F-52/F-53). |
| Node duty (45) | Deepen Node's five duty lifetimes in place; rename the common handle for its actual role. | Accepted-child close and issuer late-root owner (F-17/F-39/F-61). |
| Permission issuance (41) | Retain issuer authority and Endpoint attempt owners; later separate offline grammar from Route-facing listener. | No parent Route import cycle; keep exact signed bytes (F-28/F-30). |
| Protected wire (31) | Retain ARDP/terminal/capsule owners; narrow shared Route root by operation. | Preserve frame bounds, accepted-channel and physical Carrier close ownership (F-29). |
| Publication/discovery (48) | Retain Publication, Reachability and Target Link floors; deepen Endpoint publication coordinator. | Old Descriptor root policy and obsolete instruction retirement (F-32/F-42). |
| Qualification (6) | Retain Endpoint's stream-qualification adapter for verification; do not treat its fixture path as ordinary Publisher/Reader behavior. | Its installed verdict remains separate from `make check` and the ordinary text-command verdict (F-31). |
| Receiving spend (13) | Retain Replay ledger and Node duty admission as distinct owners. | No spend-root close before accepted children join. |
| Resource/diagnostics (3) | Retain Resource and event owners; route results into bounded diagnostics. | Keep observation failure and stop/join result visible to the process owner. |
| Service Connection (31) | Retain native logical stream owner and Endpoint physical adapter. | Error-bearing replacement Attachment and completion barrier (F-23). |
| Service identity (16) | Retain one Instance root; isolate and retire old decryptor under its contract. | Existing request/Credential and phase-less root disposition (F-42). |

### Network State: one authority root with distinct internal work

`internal/network/state` has 57 production Go files at `53f02e64`;
`networkState` has 43 methods across 19 files. The count alone does not
justify a package split. `Config` is validated as one trust/source/clock
input, and `Snapshot` is copied as one verified generation. The owner's
current/pending/conflict relation is shared by offline `Accept`, Source
refresh, pending activation and `AcceptClosedProfile`; splitting those
mutations into independent roots would introduce a second authority.

| Internal responsibility | Current source and owned lifetime | Boundary decision |
| --- | --- | --- |
| Signed Epoch and closed-profile grammar | `epoch_*`, `closed_profile*.go` verify supplied bytes and project bounded results. | Pure parsing can be made more navigable within State. A new package is warranted only if it has a small real caller interface and does not let callers bypass the accepted current/pending/conflict decision. |
| Durable admission and conflict floors | `durable_*`, `control_state.go`, `distribution_journal.go`, `selection*.go`, `offline_accept.go` commit one root under the `networkState` lock. | Keep one State decision and recovery owner. Fix the exact role/profile join (F-45) and select the closed Epoch-envelope rule with old-root treatment (F-50) before moving grammar. |
| Source and runtime observation | `refresh.go`, `server.go`, `scheduler.go`, `resources.go`, `clock_observation.go` run children of the State owner. | Source transport and `resource.Guard` remain separate lower modules; State owns their admission, cancellation and terminal errors. `Close` currently masks root/source-release failures after a server/resource failure (F-19). |
| Borrowed current views | `snapshot_access.go`, `node_duty_view.go`, `resolution_view.go`, `closed_profile_accept.go` return checked, immutable projections. | Keep State's currentness check at the boundary. Narrow the 38-getter Node projection only after preserving authenticated currentness and the Node caller's actual fields (F-07). |

The next State work is a bounded correctness repair and internal navigation,
not an automatic `state/epoch` extraction. This source pass covers the
owner's public transitions and close order; the file map still marks 32 of
its 57 production files at package-contract depth pending exact behavior
review where a move or retirement depends on them.

### Release: retain one decision and floor owner

`internal/release` has 29 production files, but its public composition is
`Open` -> `Verifier.Evaluate` -> checked `Close`. The caller supplies exact
offline metadata, artifact bytes and local binding; the Verifier alone owns
the exclusive `release-decision` root. `buildVerifiedSet` verifies a bounded
consecutive TUF root chain and durably publishes each newly trusted root
before using it. `evaluate` then verifies the target identity and
build/protocol state and commits executable metadata floors only for an
accepted or no-update decision that advances them. One `Decision` supplies
opaque replacement authorization only after acceptance. Root rotation,
artifact identity and floor publication are distinct internal steps of the
same authority decision; splitting them into separately openable packages
would expose an intermediate trusted root or duplicate the rollback floor.

The command enrollment and replacement paths both open the same floor root
and check `Close` before using authorization. The protected `endpoint
headless` path does not join that decision to its plan, system unit or worker
manifest (F-27/F-67). The architectural repair is the installed provisioning
handoff, not a second Release implementation or a package split. Keep the
historical metadata identity bytes until their own researched migration;
their version labels do not select an alternate executable runtime.

## Necessary runtime versus retained surroundings

The component table is the **selected C0 runtime composition**, not a demand
to import every currently maintained package into one process. The package
graph contains 55 `cmd`/`internal` packages. Command import closures include
non-C0 branches and shared packages, so compile reachability is not a count
of required C0 Modules. `ardents-text` is a separate installed
Application process; the two qualification commands are verification tools.

The 815 production Go files classify by their **physical package role** as
follows (joined from the file and package-disposition maps at `53f02e64`):

| Current package role | Production files | Architectural implication |
| --- | ---: | --- |
| Runtime or mixed runtime | 535 | Includes selected C0 paths and old operations sharing their packages; this is not a 535-file C0 execution closure. |
| Provisioning, operator and retirement commands | 157 | Distinct authority and retained-root obligations; only the required installation handoff composes into the selected journey. |
| Verification and architecture gate | 45 | Evidence producers, not product runtime owners. |
| Retained uncomposed or evidence-only packages | 78 | Includes 69 Namespace/Resolution files without a selected C0 composition; decide their exact retained obligations before removal. |

The total is 815, with no unclassified production row. This explains why
deleting everything outside the text journey would be unsound while also
identifying the 69-file uncomposed Name cluster as a concrete reduction target.

| Surrounding code | Target relationship to the C0 composition | Current decision limit |
| --- | --- | --- |
| Retained operator and compatibility work: Portable user-unit enrollment/replacement, Invite Entry, Contributor retirement, local Name encoding, historical floors and refusals | Keep exact command/storage owners and acceptance evidence, but do not make them alternate C0 paths or pull their authority into the text Service. The selected protected worker boundary requires a system-managed Endpoint account and root-owned units (F-27). | Remove only after an explicit retirement or migration decision for each persisted or operator obligation. |
| Installed qualification commands, stream-test Application, architecture gates, and test fixtures | Exercise the real C0 path and report evidence; keep their plans/verdicts outside product runtime ownership. | A test-only callback is not proof of installed behavior. |
| Canonical Namespace and private Resolution without a selected production composition | Keep their 54 and 15 production files, respectively, outside C0 runtime composition. Resolution has no production importer or durable root; its remaining command composition is a retirement-test fixture. Namespace Epoch owns leased generations and a pending journal, while no command opens it or constructs the retained Custody Namespace operations (F-51). | Supersede ADR-0090 to retire the uncomposed Resolution transport with equivalent command-refusal evidence first. Decide existing Namespace roots and authority material separately before removing its Store, record readers or the Vault's Namespace cases. Keep local `name encode` and the old-command refusal. |
| Historical Alpha private OHTTP resolver | Already retired from maintained source under ADR-0091; its floor parser and supplied-bytes inspection remain in separate owners. | Git history retains the old exchange evidence; do not reintroduce it as a C0 component. |
| Question-scoped R-149/R-152 experiments and research drafts | Evidence and open design, not C0 runtime foundations. | Existing research links must remain usable if a spike is retired. |

This separation makes removal measurable: a source file leaves the maintained
tree only after its live consumer, persisted/wire identity, refusal oracle,
test evidence, and documentation owner are accounted for. It also prevents a
large uncomposed research package from influencing the necessary C0 package
graph merely because it exists in the same repository.

## Current reduction decisions and limits

| Material | Source-backed disposition for this study |
| --- | --- |
| Five duplicate Network State fixture files in `tests/e2e/node` | Consolidate their two caller sites onto `tests/epochfixture/network`; the 356 copied lines differ only in package declarations. Keep the distinct older network-source and State unit-test oracles. |
| Historical `internal/naming/alpha/private` | Retired from the working tree under accepted ADR-0091, including its package-map row and deadcode allowances. `internal/naming/alpha` still owns the corpus parser/floor; external refusal and supplied-bytes inspection remain. The former exchange is Git-history evidence. |
| Fifteen production files in `internal/naming/resolution` | Not a C0 runtime component. F-51 identifies no production importer or owned durable root; ADR-0090 currently retains the Module. Retire it only with a superseding decision and replacement for the command refusal's live-fixture oracle. |
| Sixteen production files in `internal/service/instance` | Retain as one deep host-key/root Module. Initialization, exact response acceptance, durable phase, non-exporting binding and restart refusal share one authority invariant; the traced command has a real non-test call path. |
| Uncomposed OHTTP facade in `internal/service/reachability` | Retired under ADR-0091 with its exact allowances and tests. The shared Store, legacy Descriptor reader and private v3 publication/lookup remain; persisted generation/conflict floors still require a separate migration decision. |
| Portable profile scaffold (`ConfigHome/grants`, `StateHome/vault`, `StateHome/diagnostics`, `CacheHome`, test-only `portable.Run`) | No non-test Go consumer uses these created directories or the `Run` facade. Review external operator assumptions, then contract the profile around the actual first-enrollment and current-program owners without deleting existing state (F-26). |
| Offline Control/Custody imports of `route/credential` | Preserve the public issuer-profile verification and signed permission grammar, but separate their offline contract from the live Route-facing issuer listener when a real cohesive seam is ready. At `53f02e64` both offline command closures still include CIRCL and QUIC, but no OHTTP; the Transit client was retired under ADR-0092 (F-28/F-30). Recheck the closures after a real package boundary change; keep Node/Endpoint network dependencies. |
| Legacy Endpoint Transit acquisition and generic Publisher setup | Removed under ADR-0092 with their exact deadcode allowances. The protected text participant retains its permission and token-attempt journals. ADR-0062's historical Grant bytes and the separate durable local-role spend field still need their own compatibility disposition (F-52/F-53); do not conflate those with the live closed-token issuer or receiving spend. |
| Proposed `network-core-transition.md` and wire appendix | Keep as unaccepted design while decisions are open; promote selected facts to current owners and retire superseded execution chronology after reconciling issue state and links. Do not implement candidate wire grammar by document proximity. |

This ledger intentionally mixes removals with explicit retentions. It avoids
using raw file count as a proxy for necessary work and will expand only when a
caller/resource/contract trace supports a disposition.

### Former v2 execution closure after ADR-0092

ADR-0092 removed the thirteen former Endpoint `retirement-review` files and
`route/credential/profile.go`. The current C0 inventory has thirteen remaining
`retirement-review` rows: twelve Route and one Service Publication. Do not carry the Route
closure forward merely because it still compiles; do not delete a persisted
reader or current issuer by treating this count as a package boundary.

| Closure | Exact files or mixed owner | Current reachability and removal gate |
| --- | --- | --- |
| Generic Endpoint Publisher/Transit | Thirteen former Endpoint `retirement-review` rows, including the six-file `endpoint/transit` package | Removed by committed ADR-0092. The protected text participant and durable token journal remain. |
| Old Route Entry/Transit attachment | `entry_attachment.go`, `entry_binding.go`, `endpoint_transit_attachment.go`, `endpoint_transit_binding.go` | Both exported attachment openers and the Transit receiver have no non-test external caller after ADR-0092. Their codecs are called only inside this uncomposed closure. Audit exact Entry/Transit wire refusal and retained Invite/Grant evidence before deleting it; do not remove `internal/entry`'s separate durable Invite owner. |
| Old Route relay and Introduction I/O | `credential_relay_io.go`, `credential_relay_setup.go`, `introduction_control_io.go`, `introduction_outcome_io.go`, `introduction_outcome.go`, `introduction_slot_registration.go`, `sealed_introduction.go` | No exported relay or Introduction I/O entrypoint has a non-test caller outside the old closure. Its private codecs call one another, not the selected closed ARDP operations. Resolve accepted vectors and historical verification needs before retiring it together; the current private capsule lives in `route/capsule`. |
| Old Route profile and shared v2 wire | `route/credential/profile.go` was removed; `route/wire_encoding.go` remains | The old OHTTP profile verifier and client are gone under ADR-0092. The v2 envelope has no selected execution caller, but `wire_encoding.go` also supplies `wireReader` to the historical Transit Grant v1 verifier and the uncalled LegBinding decoder, `writeAll` to uncalled Node binding, and the old profile spelling used for typed Node refusal. Split historical readers/refusal from the execution codec before deleting the latter (F-52). |
| Old Service Introduction plaintext | `service/publication/introduction_instruction.go` | Its encoder, decoder and `Current` validator have no non-test caller after the generic Publisher deletion. Accepted ADR-0035 still specifies the generation-2 plaintext; ADR-0081 makes the private v3 capsule the current successor. Decide historical verification or explicit supersession before deleting the implementation, without restoring a live v1 Publisher path (F-42). |
| Mixed or shared-package prerequisites | `route/transit_grant.go`, `network/duty/store.go`, and `service/instance/lifecycle.go` remain; the old `route/credential/client.go` is removed | Credential's `contract.go` now contains only live closed-issuer declarations (F-40). The retained Grant verifier has a separate historical wire obligation. The uncalled Grant spend still shares a live version-1 duty root with current conflict Duties, so code removal and persisted-schema change require separate decisions (F-53). Instance's old decryptor shares `lifecycle.go` with the current durable owner. |

The current caller audit finds no selected external production caller for the
eleven pure Route attachment/relay/Introduction files or the Service v1
instruction. Their own local helper calls do not constitute a supported
runtime. The twelfth Route file, `wire_encoding.go`, is mixed: the old v2
envelope/IO uses it, but historical Grant and LegBinding readers and old
Node-profile refusal still need an explicit disposition. The target remains
one closed Route accepting path, with no v2 fallback.

**Removal order for this closure:**

1. Decide the historical wire/vector obligations under ADR-0035, ADR-0062,
   ADR-0072 and the generation-2 migration clause of ADR-0081. Name the
   persisted Grant-spend treatment separately; it shares the live local-role
   root (F-53). This step records which old bytes remain verifiable or typed
   refused and the eventual condition for removing their readers.
2. If needed, isolate the minimal historical Grant cursor, LegBinding reader
   and old-profile refusal from the v2 execution envelope. Keep the selected
   closed ARDP writer and current Node duties unchanged. A reader retained
   for migration never starts old admission or wire execution.
3. Retire the eleven uncalled Route files, the old Service plaintext file,
   and then the obsolete part or entirety of `wire_encoding.go` with their
   exact tests, architecture allowances and package-map/document owners.
   Check the full `cmd`/`internal` caller graph and the selected installed
   C0 path on the same integrated candidate before closing that slice.

## Transition dependencies visible so far

This is a dependency order for review, not a second issue ledger or a new C0
implementation slice. Each step must be compared with the active network and
Endpoint owners before execution; accepted ADRs, current contracts and the
single C0 work-in-progress rule continue to control scope.

| Order | Bounded outcome | Evidence required before the next step |
| --- | --- | --- |
| 1. Reconcile the source baseline | Freeze one reviewed architecture-worktree revision, classify the other agent's completed Endpoint and network changes by touched owner, and update this map only where behavior actually changed. | Exact HEAD/diff, no lost staged or untracked changes, current issue owner, and affected current technical owner. |
| 2. Repair observed authority and outcome gaps | Enforce the accepted four-domain Epoch-to-closed-profile mapping in the State owner and refuse a signed mismatch before durable profile acceptance and listener start (F-45). Keep State root/role close failures in the terminal result, stop emitting an accepted Source-wave event for `--resume`, and give Issuer roots a later close owner after bounded drain timeout. Account for control-admission Hosting release results and the shared handle's late-child lifetime (F-62). Retain Catalog, Release and Network inspection close outcomes (F-64), join Endpoint replacement writer-lock release failures to their public result (F-68), and join Reachability and Instance startup failures with their exclusive-lease cleanup failures (F-71/F-72). | A full-Epoch valid-signature mismatch refusal case plus focused combined-error, resume/actual-wave, release-failure, inspection-close, replacement-lock-close, Reachability/Instance startup lease-release, and delayed-child/late-close tests; unchanged signed bytes and one accepting closed Route version. These corrections precede package moves. |
| 3. Close alternate accepting C0 versions | Select the sole AREP envelope schema for new closed Epoch intake, then separate offline and Source-wave candidate acceptance from authentication of retained State generations (F-50). Define explicit current, pending and predecessor restart outcomes before treating an old parser as inert. Classify the Installed enrollment command and general verifier separately from protected first-run v3 (F-47); decide their successor and ADR-0042 obligations here, but retire the old Installed startup only with the supported system-unit launch in step 6. Stop other old-format writers only after deciding their roots and accepted operator obligations. | Exact old-envelope/closed-profile refusal before staging or activation; reopen and pending-promotion evidence with chain/control floors intact; version decisions in the owning contract, with no new writer or alternate runtime. An accepted old operator path is not silently relabeled C0-ready. This precedes broad decoder removal or package moves. |
| 4. Remove verified duplication and retire only decided dead paths | Consolidate the five exact Node E2E fixture copies (F-16) and the 17 copied frozen-vector paths while keeping each test's own expected result (F-69). Register the canonical fixture and Linux qualification-generator tests in an explicit checked profile (F-70). Simplify Node's dominated v2 plan-selection branch while preserving early old-key refusal (F-46). ADR-0091 has already retired Alpha private, Browser compatibility source and the unwired Reachability OHTTP adapter. Resolve the retained Resolution Module against ADR-0090 and promote current document facts before retiring old transition chronology. | Caller and compatibility/refusal evidence, affected fixture/command tests and source-extraction profile, package-map/deadcode changes where applicable, checked-profile wiring, and unchanged black-box fixture verification. An open ADR decision blocks only its own deletion candidate. |
| 5. Deepen the real network owners | Use the [Node plan](node-architecture-refactoring.md) and observed Route/Credential/Terminal edges to make duty, Carrier, wire, spend, and logical Connection ownership navigable. Rename Node's private `probeServer` handle for its actual common duty role (F-39); keep duty-specific servers separate. Extract a package only with a real caller, small Interface, tests and no parent import cycle. | One coherent passing slice at a time, exact stop/join/error owner, targeted Linux/Windows build as affected, `make quick-check` at the required boundary. |
| 6. Integrate Endpoint and qualify the combined journey | Reconcile completed Endpoint refactoring with Node/Route; design the root-owned artifact/Release-to-system-unit handoff and supported v2 plan/unit provisioning, keeping the exact service account/MainPID and worker-binding invariant (F-25/F-27). With its installed successor proved and accepted obligations resolved, retire the older `enroll-installed`/per-user-unit startup rather than leaving two maintained readiness routes; any retained old descriptor reader is for explicitly bounded evidence/recovery only. Then exercise installed Ubuntu Publisher/Reader with both TCP/TLS and QUIC and final repository gates. | One installed operator path proves authenticated artifact, plan and unit identity, restart, system-manager identity and actual Service readiness together without manual JSON or fixture keys; no test-only reachability, preserved enrollment/Custody/State roots, real Application terminal outcome, `make check`, and separately recorded installed evidence. |

Further source traces may reorder or split these steps. In particular, a
network bug already owned by the other active task stays there; this study
does not duplicate its implementation merely because it appears in a map.

### One supported C0 configuration

**Product Owner direction:** maintain one current version of each C0 contract.
The reconstruction is incomplete while an older implementation can still be
selected, writes new data, or remains supported as an alternate operating
path. For each older version, name its last production caller and decide its
retirement in the owning contract; do not leave compatibility open-ended.

The target is one supported C0 configuration, with one writer and one accepting
path for each contract in that configuration. This includes maintained operator
commands and diagnostic formats: they must not leave two supported versions of
the same contract on the normal C0 surface. There is no runtime version
selection, negotiation with an older implementation, or fallback to it. Retire
an older execution path once its exact callers and accepted obligations are
resolved; a lack of callers is a reason to finish that decision, not to maintain
the path indefinitely.

Old persisted records are a separate, temporary migration or recovery problem,
not another supported version. For each old record population, record the owner,
retained authority or rollback floor, chosen migration or typed refusal, and
the condition for removing its reader. The migration or refusal must have a
bounded completion criterion; it is not a second long-lived product mode. An
old record must not start an old runtime. Simply ignoring it may erase durable
authority or a rollback floor.
The intended end state has no old reader once its recorded obligation is closed.

Version numbers belong to different contracts, not one project-wide release:
closed Route v3 currently carries the active Service Connection v2 record
grammar, and the separate Administration interface is v1. These are each the
single current version of their own contract. Renaming their bytes to v3
would change accepted contracts without removing a second implementation.

**Decision order for removing mixed versions.** First close the actual
acceptance defect: State currently admits signed AREP v1/v2 on the closed
C0 candidate path despite the v3 fixture and Route profile (F-50). Select
v3 as the sole *new closed-C0 intake* in State's technical owner, then make
offline and Source admission enforce it before staging; old current,
pending and predecessor roots need their own authenticated restart rule.
Second close old-write surfaces: the dispatchable Invite `entry import`
and current old-key Instance initialization, plus the uncalled public
Reachability `Store.Publish` and Name `CommitLegacy` APIs. Their accepted
operator and persisted contracts require individual decisions before removal;
this order does not authorize deletion now. Third remove uncalled execution
closures and temporary readers only after each old population has a
migration or typed-refusal exit gate. Finally converge legacy Installed and
ACA1 operator routes on proved successors; a new unit or diagnostic format
must preserve their current authority floors. This order prioritizes a
currently accepting wrong version over an unreachable old codec.

| Contract and current C0 path | Older entrypoint and direction | Current production reachability | Disposition gate |
| --- | --- | --- | --- |
| Closed Route v3 duty | `cmd/ardents-node/readNodePlan` refuses old reservation keys before effects; later parser code still contains an unreachable v2 profile assignment (F-46). `node/admission.go` and `node/duty_server.go` separately read old `route.Profile`. | Refusal only; old profile starts no Node duty. | Remove the dominated command-side version selection while retaining effect-free old-key refusal; audit the separate State/Node input refusal before deleting it. Do not restore a v2 duty. |
| Closed Route v3 wire | `route/wire_encoding.go` reads/writes v2 envelopes; `route/entry_attachment.go` dials with v2 ALPN | The former non-test caller `endpoint/transit_credential_acquisition.go` was removed under ADR-0092; no selected Endpoint path uses this older wire. `wireReader` in the same file serves the retained Transit Grant v1 verifier. The current duty root stores Grant spends beside live conflict Duties (F-52/F-53). | Retire the uncalled v2 execution closure; first isolate the exact historical Grant reader and typed refusal obligations. Decide the spend ledger's persisted-schema treatment separately from removing its uncalled writer. Do not carry v2 envelopes into v3 ARDP or treat retained data as a second supported Route. |
| Closed Entry set | `cmd/ardents entry recipient/import` opens the older Invite root and can still write it; `route.OpenEntryAttachment` remains but has no selected production caller. Exported `entry.Issue`, `Verify` and `Contact` have no non-test caller; only internal Invite validation remains in the import/reopen path (F-08). | Endpoint opens `entry.OpenClosedSets` for its current State-selected members. The Invite command does not feed this root or the protected participant. Both roots share filesystem helpers but reject each other's marker. | Choose the existing Invite roots' recovery/refusal and accepted operator-contract disposition, then stop the old writer and retire its uncalled attachment. Keep the internal verifier required to read retained Invite records until their data disposition is closed. A temporary reader is not another supported runtime or a reason to move two files into a premature subpackage. |
| Headless runtime plan v2 | `cmd/ardents/endpoint_headless.go:loadHeadlessRuntimePlan` reads the v1 schema marker | Effect-free v1 refusal before v2 startup; no v1 runtime dispatch. | Keep refusal evidence; remove only unreachable v1 execution, without a converter. |
| Network enrollment descriptor v3 | `enrollment.Verify` parses v1/v2/v3; `ardents endpoint enroll-installed`, legacy enrollment-check, and read-only control inspection call it. The maintained Installed user-unit renderer still starts `enroll-installed`. | The selected first-run portable route uses `VerifyHeadless`, whose required Node/Custody companions can be projected only from v3. The installed package process test actually upgrades through an enrollment-v1 descriptor; that route reports Portable readiness and does not start the protected text Endpoint (F-47). | Decide accepted versions at each command boundary; do not widen a selected route by sharing a general verifier. Retire the old Installed startup with a proved protected system-unit successor and disposition of its accepted obligations. Retire v1/v2 parsing only after ADR-0042's retained compatibility and existing bundles have an explicit disposition. |
| Private Reachability Descriptor v3 | `service/reachability/store_files.go:restore` reads stored-record v1 and verifies Descriptor v1/v2; `Store.Publish` can still write an old descriptor through `encodeStored`, while `Lookup` exposes the old read API | Node Resolution calls only `PublishPrivate`/`LookupPrivate` for v3; a non-test `cmd`/`internal` call search found no old `Publish`/`Lookup` caller. Restored old Targets occupy the same 128-entry map, block same-Target v3 publication through `compareStored`, and are unavailable to `LookupPrivate` even after expiry. | Retire the unused old writer API separately from its compatibility reader. Decide authenticated floor adoption for an old Target or an explicit refusal/new-Target policy, preserving generation, expiry and conflict floors; no adoption API exists yet (F-22/F-32). ADR-0091 already retired OHTTP. |
| Closed State and Carrier v2 | `network/state/epoch_record.go:parseRecord` reads signed Node Record v1/v2 and v1/v2 Carrier identities | For a closed Route epoch, `validCarrierForEpoch` accepts only v2 Carrier identities; an old v1 Record supplies the v1 TCP identity and cannot become a closed Carrier. | Preserve the signed-record interpretation and refusal until State's own compatibility/migration contract retires those inputs; do not expose v1 Carrier dialing as a fallback. |
| Closed C0 Epoch envelope | `state.parseEpoch` accepts AREP v1/v2/v3; `verifyEpoch` matches only the `ardents-route-v3` profile, not the envelope version. Older versions can pass the closed candidate path if otherwise signed and valid (F-50). `verifySourceBundle` also reuses exact current/pending decisions without the general candidate verifier. | Installed closed-State and qualification fixtures build AREP v3. The current admission rule does not pin that version; retained chain recovery reuses the same verifier and can restore an old current or pending decision. Neither current State/Route technical owner states the AREP byte; the closed-profile `version u16=3` belongs to a different signed record. | Product Owner's one-version direction makes AREP v3 the proposed sole closed C0 intake grammar; select it explicitly in the technical owner with the retained-root policy. Gate offline and both Source result forms before staging or activation, classify recovered current and pending before use, and separately authenticate older predecessors without discarding chain/control floors. Define typed refusal or signed-successor migration for old roots, then remove old decoders when their population is closed. |
| Name Record v4 | `naming/namespace/record.DecodeRecord` and `VerifyRecord` accept v3/v4; `epoch.Store` can retain signed v3 records from current or pending state. `CommitLegacy` remains exported but has no non-test caller. | `SignRecord` emits v4 only. A v3 Target has no signed `RecordNotAfter`, so materialization marks it unavailable and proof verification rejects it (F-48). | Keep v3 solely for the ADR-0022 re-publication/migration obligation. Make the old-record disposition explicit before removing the decoder; retire the uncalled caller-built `CommitLegacy` writer after its fixture/evidence users are relocated, without making v3 Target resolution available. |
| Alpha-control disclosure ACA2 | `ardents-control inspect-bundle` and `inspect-transitions` use an ACA1 `Reader` with a durable catalog floor; `inspect-alpha-corpus` separately verifies ACA2 without a reader floor (F-49). | Both report diagnostics and authorize no Endpoint action. ACA1 inspection advances separate Catalog, Release and Network floors, while ACA2 has no reader floor. Their different ownership prevents treating one parser as a drop-in replacement; ACA1 currently discards three close results (F-64). | Converge on one maintained disclosure-inspection format. Choose its catalog-floor owner and treatment of existing ACA1 roots, then supersede ADR-0041's explicit ACA1 retention and the current transition-inspection contract before removing the old command, reader, and parser. |
| Protected Service Connection v2 | `service/connection/record.go` writes/reads v2 records with retained `ardents-interactive-route-v2` profile bytes | `endpoint/text_service_binding.go` uses the v2 core in the selected protected path; this is its one active grammar, despite the historical name. | Keep accepted bytes and shared TLS core; the unwired generic Endpoint adapter was removed under ADR-0092 (F-37). |
| Private Introduction capsule v3 | `instance.Binding.OpenIntroduction` and `publication.IntroductionInstruction` retain the old SealedIntroduction v1 operation | The current text path uses `PrivateRecipient.OpenPrivateIntroduction`; after committed ADR-0092 deletion, no non-test call to the old decryptor or instruction grammar was found. New Instance roots still generate and persist the old key and bind its public half into the request and Credential. The `root-v1` decoder also accepts phase-less retained JSON and reconstructs the old public request from those keys (F-42). | Retire old execution after confirming the committed call graph. Stop new writes of the old key only through a superseding decision for ADR-0034, an explicit Instance request/Credential contract change, and a disposition for both phase-less and current stored roots. |
| Local Application Connection v2 and Administration v1 | `application/interfacev2/connection` and `interfacev1/administration` accept distinct local contracts | Both serve the selected protected journey; the former Connection v1/AAI2 implementation is removed. | Do not rename Administration v1 or revive Connection v1 to make version numbers uniform. |

This first source-backed pass over selected C0 version boundaries does not
discard persisted floors or alter accepted wire identities. The remaining exact-caller
audit must distinguish old write operations from startup/refusal readers and
identify the accepted owner for each retirement slice.
It must end with a removal sequence: stop writing the old format, migrate or
retire old data under an accepted decision, remove its reader and execution
code, then remove obsolete refusal and test scaffolding that has no remaining
contract. Coordinate ownership with the network agent before editing its code.

**AREP retirement needs separate intake and recovery policies at the State owner.** The proposed
v3-only closed intake cannot be implemented as a global `parseEpoch` change:
the same decoder authenticates retained generations. The source trace at
`53f02e64` identifies the exact acceptance and recovery boundaries:

| Boundary | Current path | Required result for the proposed single version |
| --- | --- | --- |
| New offline decision | `networkState.Accept` -> `verifyDecision` -> `verifyEpochCandidate` -> `verifyEpoch` -> `parseEpoch`, then `commitActiveDecision` | Refuse an older closed-profile envelope before any generation or control-floor commit. |
| New Source decision | `verifySourceBundle` falls through to `verifyDecision`; `commitPendingSourceWave` can stage it and `commitActiveSourceWave` can activate it | Refuse an older new candidate before either staging or activation, including both LATEST and BY_DIGEST results. |
| Exact current/pending Source result | `verifySourceBundle` returns an already loaded decision after byte/input and materialization checks | Apply the recovered-current/pending policy here too; a gate only in `verifyDecision` leaves this path open. |
| Retained predecessor | `loadGenerationChain` -> `loadGeneration` -> `verifyDecision` authenticates each prior signed generation | Keep historical decoding solely to prove the chain and retained floors; do not expose an old predecessor as the active C0 runtime. |
| Recovered current or pending | `loadCurrent`, `recoverDistributionActive`, and `recoverPendingState` install authenticated decisions; the Source wave can later promote pending | Decide an explicit recoverable refusal or independently signed successor for an older active/pending envelope before opening its C0 duties. Preserve the control floor and old bytes until that decision is complete. |

This is the boundary for the owning contract and later implementation tests,
not a claim that v3-only intake or old-root migration already exists. The
tests must separately cover offline intake, both Source result forms, restart
from old current and pending roots, and a v3 successor with old predecessors.
The existing `state.RecoveryRequiredError` is a suitable typed *refusal
result* for an authenticated old active/pending root: keep its bytes and
chain/control floors, expose no C0 duty, and require a separately authorized
State repair or signed successor. It is not itself that repair operation.
`parseEpoch` must continue to authenticate retained predecessors while the
new-candidate and runtime-use gates enforce the single current version.
The accepted State owner still needs to specify how a v3 successor can be
admitted when an old pending slot already exists; silently clearing the slot
would violate the current/pending/conflict invariant. This is the precise
remaining contract decision, rather than a reason to keep v1/v2 as live C0
envelopes.

## One read through the proposed boundaries

```text
Reader Application -> local Connection Interface -> Reader Endpoint
    -> current State + local permission + Target Link / Descriptor
    -> client path -> protected channels -> selected Carrier
    -> forwarding / Introduction / resolution / JOIN Node duties
    -> Publisher Endpoint -> authenticated Service Connection
    -> confined Publisher Application
```

The Publisher setup prepares its Instance, worker, prefixes and Introduction
registration, then commits its Descriptor and returns the Target Link. During a
read, Endpoint first admits the local operation; the client path obtains the
selected recipient and physical attachment; Service Connection authenticates
the final Instance and owns ordered Application bytes. A network JOIN result
alone is not a completed Service read.

## Import direction and the first exact seams

The observed Linux package graph has `endpoint -> route`, `node -> route`,
`route/credential -> route`, and `route/terminal -> service/reachability`.
Endpoint imports 24 first-party packages and Node imports ten. These are
compile-time edges, not evidence that the imported owner is needed on every
runtime path. The target should first assign each concrete decision and
resource to an owner, then let the top-level process composition call that
owner. It should not turn a 74-file Route root into a generic transport API.

| Current edge or mixed responsibility | Source-level reason | Target decision before moving files |
| --- | --- | --- |
| State `CurrentNodeDuty` `->` Node `DutyView` | One command callback passes State's full copied Snapshot inside an opaque view through a 38-getter Interface; Node reconstructs a second `dutyFacts` value. The view does not expose Source/pending getters, but still carries those fields internally. Current closed duties use candidate values, while copied old authority fields are not read by Node (F-07). | Keep State's authenticated currentness and Node's private lifecycle copy. Assess `Config.Current func() (state.NodeDuty, error)` returning a copied value with Epoch status, local record/assignment and bounded candidates, not a nested Snapshot or State handle. Validate bounds and complete identities on receipt; settle the role join (F-45) before fixing fields, and retire unused authority/Transit projections only after their compatibility audit. This changes 21 Node test fixtures but needs no new package or reverse import. |
| `route/terminal -> service/reachability` | Two terminal production files import Reachability solely for `MaximumPrivateDescriptorSize`; its proof and Store enforce the same bound. Terminal currently inherits Reachability's full 111-package static closure (F-29). | Keep Reachability authoritative for the signed proof. Give terminal its exact wire bound and a cross-owner test that checks equality and both 15,000/15,001-byte framing outcomes; then remove only the production import. Avoid a new package containing one constant or a wholesale Descriptor move. |
| `route/credential -> route` | After ADR-0092, only `closed_token_listener.go`, `closed_token_bootstrap.go` and `closed_token_admitted.go` import parent Route; all serve the live Node issuer (F-30). Credential's issuer root/reservation ledger and Replay's receiving spend ledger have different owners. | Preserve the live issuer core and Node-owned listener/admission lifetime; extract a narrow Route-facing adapter only after assigning accepted-child join and late issuer-root cleanup. A broad Carrier interface does not resolve either ownership question. |
| Offline Control/Custody `-> route/credential` | Control calls only the signed public issuer-profile decoder; Custody calls only permission/request grammar. The package also contains the live Route-facing issuer listener, so static dependency closure still includes QUIC and CIRCL in these offline commands, but no OHTTP. | Give the signed offline grammar a cohesive owner with a non-test caller and exact validation tests. Keep the live networked issuer adapter separate without changing bytes or moving admission authority into Route. |
| Node forwarding `->` shared Carrier listener | Route already supplies `ClosedSharedCarrierListener.Accept/Close` and a direct-role or authenticated outer-Node result with caller-owned `net.Conn`. Node owns the accept loop, work limit, accepted children and drain. Listener `Close` reaches `Drain`; admitted connection `Close` errors are currently discarded by the Node handler. | Keep the narrow listener seam and explicit ownership transfer. Place TCP/TLS/QUIC mechanics behind it, retain Node's admission and stop/join authority, and decide the required per-connection close-error outcome before a package move. No speculative broad Carrier interface is needed. |
| Endpoint Descriptor publication `->` Route Control lane `->` Node Reachability Store | Endpoint owns the signed bytes and exact retry; Route sends one nonce-bound operation; Node checks current Introduction and calls `PublishPrivate`. Status 0 follows `StoreAccepted` or `StoreAlreadyCurrent`, after the Store's file and directory sync for a new record. | Keep durable revision/conflict floors with Reachability, State/duty admission with Node, and caller lifetime with Endpoint. The result Interface must distinguish a committed or already-current ACK from refusal and uncertain transport loss; it must not turn a lost reply into a second publication identity. |
| `endpoint -> route` and `node -> route` | Both processes borrow transport/channel operations, while Endpoint also coordinates publication and Node owns receiving duties. | Keep process admission, lifecycle, and terminal error ownership at Endpoint/Node. The shared Route boundary should expose only the operations and leases each caller actually consumes. |
| `service/connection` has no first-party imports | It owns authenticated logical byte ordering and attachment lifetime without importing Endpoint or Route. | Preserve this deep boundary; adapt physical attachment at its caller, not inside Service Connection. |
| Replacement Service Attachment close | Native `Attachment` accepts a `func()` close callback; Endpoint's replacement callback discards the joined Route close result, while its initial transport separately caches that result. `NewAttachment` has one non-test caller: Endpoint's protected Service adapter. | Keep Service Connection independent of Route. Make the callback return `error`, retain retirement results in native Stream, and close its `Done()` only after physical retirement on both ordinary and tail paths. Endpoint joins the post-`Done()` result in final `textServiceStream.Close()`. Preserve exactly-once retirement, the already-published Application outcome, and authenticated terminal ordering (F-23). |

The target import rule is therefore directional: command adapters compose
Endpoint, Node, State, Service, and trust owners; Endpoint/Node borrow the
smallest Route operations required for their exact duty; wire and Carrier
mechanics do not own Service publication, Network authority, or process
lifetime. This is a constraint for the source-level design, not a package
creation list. The Endpoint agent's accepted boundary and the concurrent
network changes must be read before finalizing exact exported Interfaces.

### Observed Interfaces and the smallest target handoffs

**Decision on the first three seams at `53f02e64`.** The checked State-to-Node
handoff should become one State-created immutable duty value, copied and
bounded again by Node, in the existing packages. `cmd/ardents-node` is its
only production caller; a new interface package would add no authority. Fix
the role/profile join (F-45) before freezing that value's fields. This is a
navigation and change-amplification improvement, not a prerequisite for
retiring the old Route or making the installed C0 path work.

Keep the live `route/credential` issuer and its three Route-facing adapter
files together during the current C0 correctness work. Their listener and
admitted child lifetime need the F-17 close owner first; relocating their
methods now would either export issuer internals or create a parent import
cycle. After that owner is settled, put Route accept/bootstrap/ARDP exchange
in the Node-facing adapter and leave permission/profile grammar with the
offline issuer contract. Control and Custody can then depend on that grammar
without importing the live network listener. This is a dependency cleanup
slice, not an additional accepting credential version (F-28/F-30).

Repair Service Connection's Attachment seam before a package move:
`NewAttachment` currently takes `close func()` and loses the replacement
Route close result. The selected result is an exactly-once `func() error`
owned by the Attachment; native Stream retains that result and completes
`Done()` only after ordinary and terminal-tail physical retirement. Endpoint
may publish its Application outcome earlier but must return the late cleanup
error from final `Close()` (F-23). No new package or broader transport
interface is needed. These three decisions set the target direction while
keeping implementation ownership and tests in separate bounded slices.

These are source-observed call boundaries at `9835e225`, not proposed exported
Go declarations. A caller-local interface can stay private even when its
implementation later moves. The final signatures depend on the active Endpoint
slice and the selected package direction.

| Caller and operation | Existing seam | Target handoff and owner |
| --- | --- | --- |
| Node accepts a selected Carrier | `route.ClosedSharedCarrierListener.Accept(ctx, timeout)/Close` returns a classified `ClosedSharedCarrier` with caller-owned `net.Conn`. | Route owns TLS/QUIC authentication and physical listener mechanics. Node owns State freshness, admission limit, accepted handlers and drain. Preserve listener-close errors in Node's terminal result; settle the separate accepted-connection close policy. |
| Endpoint retains a Source/Introduction/Responder path | `route.OpenClosed*Prefix` returns a `ClosedSourcePrefix`; its `Close` stops channels, joins children and returns physical retirement failure. Endpoint owns its source acquisition and release around that value. | Keep prefix lifetime with the exact Endpoint owner and channel mechanics with Route. The result must remain closeable with an error; do not export Endpoint's `textContext` or its handle locks to make a new package. |
| Endpoint publishes or looks up a Descriptor | The private `textResolutionSource` supplies currentness, recipient and `exchangeDescriptor`; Route's `ClosedSourcePrefix.ExchangeDescriptor` opens one Control lane and returns status/proof after nonce checking. Endpoint validates the signed proof or commits its publication only after ACK. | Keep `textResolutionSource` as a caller-local coordination seam: its `currentLocked(*textContext)` cannot be an independent Route API. A cross-package operation should use only current-State-bound recipient selection, a token presenter and exact operation bytes, return a bounded result or transport uncertainty, and leave proof/history and publication authority with Endpoint/Reachability. |
| Endpoint pairs a Data JOIN | The private `textJoinAcquisition` chooses a recipient and calls `ClosedSourcePrefix.Join`; the returned `ClosedJoinedStream.Close` joins channel and peer-close results. Endpoint's `textJoinedTransport.Close` additionally releases Job qualification and its acquisition lease. | Route owns channel/JOIN mechanics; Endpoint owns pairing intent, simultaneous capsule submission, Job, token attempt and lease transfer to Service Connection. Preserve a returned close error and one owner for the joined stream. Private methods taking `*textContext` or returning `*textSourceHandle` remain local coordination, not a package contract. |
| Service Connection replaces an Attachment | Native `connection.AttachmentOpener` returns an authenticated Attachment under an immutable Recovery context. Endpoint's opener obtains and authenticates the new Route transport. Native `Attachment` currently takes a `func()` cleanup callback. | Service Connection keeps logical identity, offsets, terminal and recovery rules; Endpoint retains Route selection and physical close outcome. Resolve the callback's lost replacement-close error before calling the boundary complete (F-23). |

This inventory already rules out one tempting split: moving the Endpoint-local
`textResolutionSource` or `textJoinAcquisition` declarations into Route would
force Route to know Endpoint's private context and locks. The package seam is
the narrow operation plus its owned result, while those coordination checks
remain with the Endpoint owner.

### Carrier source cluster: one responsibility, three lifetimes

The ten Carrier rows in the [file inventory](c0-component-inventory.csv) now
have concrete callers and resource owners. They do not move as ten independent
files:

| Cohort | Production caller and lifetime | Package-boundary dependency |
| --- | --- | --- |
| Node physical dial and byte lane: `closed_node_carrier.go`, `closed_tcp_transport.go`, `node_carrier_quic.go`, `node_carrier.go`, `tls_adapter.go` | Node forwarding dials one State-selected adjacent peer through `OpenClosedNodeCarrier`; the returned `Carrier` belongs to its lease/session. TCP closes the physical socket once and retains its result; QUIC owns stream and connection close. | Dial validates literal endpoint, v3 profile, exact peer TLS and shared `Carrier` interface. The v1 symbolic constants in `node_carrier.go` have no non-test Go caller; the negative test can keep its old-profile refusal without keeping those exported names. State's v1 signed-record reader is a separate obligation. |
| Direct role and shared listeners: `closed_role_carrier_client_linux.go`, `closed_role_carrier.go`, `closed_shared_carrier.go` | Closed Source/bootstrap opens a direct role Carrier; Credential's direct issuer and all five Node duties accept through narrow `Accept/Close` listeners. The accepted connection transfers to the caller; Node or Credential joins its handlers. | The role files call `closed_role_tls*.go`, while `ClosedRoleTLSExporter` returns a type declared in `closed_admission_channel.go`. A Carrier package move must place TLS authentication/exporter consistently or it will import Route back. |
| Retained outgoing Carrier: `closed_carrier_pool.go`, `closed_carrier_retirement.go` | Node forwarding supplies exact State validation and an open callback. The pool retains only a used Carrier, gives each borrower a lease, invalidates one incarnation, and shares one physical close result. | Pool policy is Route's bounded physical reuse; Node keeps peer selection, session reader and shutdown order. A package seam must return the lease's real close result to Node. |

`literalEndpoint`, `exactPeer`, `ClosedRouteProfile` and role TLS validation are
shared security rules, not convenience imports. The next package decision is
whether these concrete files and their TLS dependencies can form one acyclic
owner with Node and Credential as non-test callers. The existing listener and
lease Interfaces already provide its small external seam; adding a broader
transport abstraction would obscure the transfer of accepted connections.

### Client-path source cluster: four ownership scales

The 25 Route-root rows grouped under `client-path` do not have one
resource lifetime. Their source-specific dispositions are in the
[file inventory](c0-component-inventory.csv):

| Source group | Actual owner and boundary implication |
| --- | --- |
| `closed_bootstrap_exchange.go`, `closed_bootstrap_plan.go`, `closed_recipient_inspection.go` | Endpoint calls the one-shot issuance exchange and recipient inspection. The exchange closes its own Entry/Interior transport before returning; inspection opens none. The plan validates the live State selection for both this exchange and retained prefixes. Keep its State contract shared without pretending the operation owns a retained Source. |
| `closed_bootstrap.go` | Credential's `closed_token_listener.go` constructs `ClosedBootstrapController` for receiver-side queue/byte admission. It is a duty resource controller, not a client Source object; the inventory now assigns it to `node-duty`. |
| `closed_source_prefix.go`, `closed_source_channels.go`, `closed_source_capacity.go`, `closed_source_queue.go`, `closed_source_lifetime.go`, `closed_source_lane*.go`, `closed_role_child_stream.go`, `closed_terminal_priority_linux.go` | Route owns an admitted parent Carrier, multiplexed lanes, credit, child joins and physical-close result. The child-stream and terminal-priority helpers are called only by outgoing bootstrap/prefix owners. Endpoint retains separate Source, Introduction and Responder prefix handles selected from current State. `ClosedSourcePrefix.Close` joins channels and child before returning failure to that owner. |
| `closed_source_control.go`, `closed_source_issuance.go`, `closed_source_resolution.go`, `closed_source_submission.go`, `closed_source_replenishment.go`, `closed_introduction_client.go`, `closed_join_client.go`, `closed_join_client_stream.go` | These operations borrow a retained prefix, but registration and joined stream each acquire their own child lifetime. Endpoint owns publication, token attempt, pairing and Service Connection transfer; Route owns operation framing and the child transport cleanup. |
| `closed_introduction_delivery.go`, `closed_terminal_recipient.go` | Delivery state is owned by one retained Introduction registration and claimed by Endpoint; completion joins its result and terminal close. Terminal-recipient selection opens no channel and rechecks the current State and unique role before Endpoint uses the result. Both belong to the client path, with different lifetimes. |

The next package decision must preserve the shared bootstrap plan and Route's
parent/child ownership while giving Endpoint closeable results. Moving all 22
files into one package by their `closed_` prefix would conceal these different
lifetimes and move the receiver-side admission controller with client code.

### Endpoint client path: separate reservations and close owners

The 20 Endpoint Source/Introduction/Responder/JOIN rows now have source-specific
reasons in the [file inventory](c0-component-inventory.csv). They do not form
one `text_` lifecycle:

| Endpoint owner | Observed reservation and terminal boundary |
| --- | --- |
| Source | `textSourceLifecycle` retains the Route prefix and lends JOIN/Resolution acquisitions; `textSourceOperationGate` serializes actual opening and issuance across prefix replacement. `textPrefixOpeningOperation` owns one stock-to-open attempt and closes a prefix that loses admission. |
| Introduction | `textIntroductionExchangeSet` bounds flights reserved before State reads or I/O. Worker cancellation ends the attempt, while an accepted Service transport may retain its own cleanup lifetime. Publisher registration has a separate `textIntroductionPrefixLifecycle`. |
| Responder | Accepted Introduction prepares a Domain-3 prefix under the Publisher job; `textResponderPrefixLifecycle` retains that prefix and lends JOIN acquisition. Source issuance readiness is checked separately from Descriptor publication ACK. |
| JOIN to Service | `textJoinAcquisition` borrows Reader Source or Publisher Responder. `textJoinedTransport.Close` joins the Route transport and releases the Qualification join and acquisition; the Service binding receives that exact closeable transport. |
| Endpoint roots | `text_source_state.go` selects live State/Entry members but also opens and closes Endpoint Entry and token-journal roots; `text_source_prefix.go` opens a Source but also lazily opens the token journal. Those root methods have a different lifetime from one client operation. |

This supports two in-package file splits and a Route-to-Service boundary review,
not an immediate package move. Preserve separate Source, Introduction and
Responder shutdown: `text_context_retirement.go` cancels their openings,
joins them, then closes each retained prefix and joins exchange flights.

### Node forwarding and role probe stay under the Node lifecycle

`internal/node` already owns the current forwarding duty from
`startClosedForwarding` through admission, accepted handlers and drain. Its
receiving resource group opens replay spend, duty limits and bootstrap
allocation together; `finishShutdown` joins accepted producers and outgoing
sessions before closing the host and spend root. The forwarding Carrier
session and link files hold child lifetimes under that same server. Moving
these files into a new package would first require transferring private
`runtimeConfig`, `dutyFacts`, the spend-root close owner and `probeServer`
return contract. There is no demonstrated boundary improvement from that
transfer, so the inventory now assigns these files `deepen`, not a package
move.

The private role probe is an active Node duty selected only after
`assessAdmission` checks `h3-role-probe-v1`; `duty_server.go` starts it through
the same `probeServer` lifecycle. The [current technical owner](../technical/network-route-node.md#node-and-resource-lifecycle)
explicitly excludes a standalone probe runtime. Its four implementation
files therefore remain Node-owned. `contract.go` contains the DutyView
projection, public Config, Event/Result and runtimeConfig in 332 lines;
separating those file responsibilities within `node` is the next local
readability action, without inventing a new package. The shared
`probeServer` return type also names all five current closed duties after
one older probe mode; rename that private handle for its common lifecycle
role (F-39) while preserving distinct duty resources.

The six `stream_qualification_*.go` Endpoint files are the concrete
qualification adapter: they open actual participant roots, Service streams,
worker artifacts and refill tokens, then pass bounded streams and observations
to `internal/qualification` and `internal/streamqualification`. The latter
owners hold workload and verdict; Endpoint must keep its worker, Job and
transport resources. Their callers and ownership now support retaining these
six files in Endpoint, with local file cleanup if needed. A second
qualification runtime package would duplicate this composition.

### Installed worker: verification and lifetime are one Endpoint handoff

The 15 worker-related Endpoint files now have source-specific reasons in the
inventory. `workerInventory` chooses exact installed roots, units, sockets and
accounts; the artifact, manager, property and cgroup readers verify what is
installed. `launchInstalledWorker` reserves one Context Job, verifies the
parent service and immutable artifact, observes one new manager invocation,
then transfers the local attachment to `textWorkerLifetime` before INIT and
Grant. The lifetime interrupts the attachment, joins Application I/O, asks
`textWorkerCleanup` to stop only that pinned invocation, and releases the Job.
The separate workload bounds limit what can cross the worker attachment.

This is a real confinement and resource owner, but moving its files wholesale
would require exporting `textContext` and `textJobIdentity` or moving Context
admission with them. First keep the launch-to-lifetime handoff explicit in
Endpoint; extract only an independently owned installed-artifact or manager
reader if it has a real caller and a smaller Interface. These readers verify
existing units; they do not install the root-owned Endpoint and worker units.
The missing installation/provisioning handoff remains F-25/F-27.

### Resource and diagnostic handoff

`resource.Guard` is a process-local monitor. State constructs one, checks
placement before accepting current authority, and owns its periodic governor
and cancellation. Node constructs another for its duty and combines its
`Observe` result with the shared host-period observation before deciding
NORMAL/PROTECT/DRAIN. The Guard has no close operation or durable root.

`resource.Hosting` is a separate, durable period opened from one operator
initialized root. Its relevant Interface is `Sample`/`Observe`, `Reserve`,
`HostingReservation.Release`, and `Close`. A reservation commits work **and**
termination capacity before effects and remains charged until the borrowing
Node or qualification owner joins the work and releases it. `Close` releases
the root handle; it cannot forgive an outstanding reservation. Node owns
the listener/session response to pressure and its bounded event facts;
`cmd/ardents-node` owns stdout and diagnostic-file destinations. The local
timeline only projects those already emitted facts. No generic logging owner
should acquire the State, Node or Hosting lifetime to make events uniform.

### Installed Endpoint launch: exact provisioning boundary

`cmd/ardents/endpoint_headless.go` decodes one supplied v2 JSON plan and
`endpoint_text_linux.go` starts the participant. The installed command test
`tests/e2e/node/closed_text_command_installed_linux_test.go` constructs that
plan, grants its service account access to fixture roots, writes a temporary
root-owned **system** unit, and responds to permission requests with a test
Authority. This proves the runtime can consume those inputs, not that a
maintained operator path can produce and authenticate them (F-25/F-27).

| Plan or unit input | Current test producer and actual owner | Required production handoff |
| --- | --- | --- |
| Executable, static Enrollment and Release facts | Installed package and separate enrollment test verify these bytes; the protected text command test builds its binary independently. | Independently pinned first execution and retained current-program acceptance must bind the exact system-unit MainPID before protected readiness. |
| Network root, direct-Source plan, Network ID, authorities, threshold, closed-profile signer, clock marker and local-role root | Test supplies `state.Config`, reads the accepted closed profile, writes a Source plan and clock observer, then repeats trust anchors in runtime JSON. `headlessNetworkConfig` checks equality only when a Source plan is present. The plan parser checks syntax and referenced path spelling, not the origin of the plan file (F-67). | One authenticated provisioning owner supplies the trust anchors and paths; State keeps the exclusive root and rejects a mismatch before participant effects. |
| Entry, token and Publication roots, Service Instance root | Test creates or acquires each fixture path; Instance itself is accepted through separate command and Authority operations. | Name the owner and permissions of each path; preserve Instance/Custody separation and the token and Publication durable floors. Do not package mutable roots with the executable. |
| Reader/Publisher permission request and response paths with maxima | Test constructs both path pairs and answers them after the unit starts. | A separate Custody operator produces exact responses to the public requests; plan provisioning fixes only allowed paths and bounds, never embeds Authority private material. |
| Broker ID, Connection and Administration principals, local socket paths | Test invents three fixed identities and two sockets under its fixture directory. | Define how the installation pins these identities, socket locations and authorized local peers; Endpoint must not derive their authority from arbitrary service-account-edited JSON. |
| Text Application and confined-worker artifact | The alpha bundle inventories four headless binaries and static enrollment inputs. The installed command candidate separately builds `ardents-text`, copies it as the worker and hashes it with the test binary. Endpoint verifies a separate root-owned manifest for the worker executable, two service units, two sockets and the stop rule. | Bind that exact six-file worker inventory to the accepted candidate and active Endpoint unit, or specify a separately authenticated installation handoff. Keep Endpoint's local re-verification before and after worker activation; a test-stage checksum is not a maintained operator installation procedure (F-27). |
| Endpoint system unit and worker relationship | Test overwrites `/run/systemd/system/ardents-endpoint.service` with `User=ardents-endpoint` and `ExecStart=... endpoint headless <plan>`; worker units are a separate installed fixture. | A maintained root-owned system-manager unit producer binds the accepted artifact, plan, service account, MainPID and fixed worker units. The existing per-user enrollment unit cannot stand in for it. |

The design decision is the **producer and authenticity of the plan/unit
handoff**, not a new runtime version. The selected consumer stays the sole v2
headless plan. Provisioning must keep its paths and trust anchors stable across
restart or make an explicit, authenticated replacement transition. The
installed acceptance boundary is one launched Publisher and Reader on both
Carriers, after the same enrollment/Release sequence, with the actual
system-manager and worker identities observed. No fixture-created plan or
temporary unit may substitute for that operator route in the final verdict.
The selected handoff must cover the bytes of the headless and Source plans,
not just the root-owned unit pointing to them: current `decodeOperatorInput`
does not authenticate plan-file ownership or a release binding (F-67).

**Proposed production handoff for this missing component.** Add one bounded
installation operation under the existing `ardents endpoint` command owner;
its exact verb and wire-free file schema belong to the implementation brief,
not to a new runtime package. It consumes an independently verified artifact
and accepted Release/enrollment record, the selected State/Source and local
root declarations, public Custody/Instance facts, and the fixed worker-unit
inventory. It must not mint Authority keys or silently accept a test-issued
permission response. Before starting the Endpoint, it atomically publishes
the v2 headless plan, Source plan, worker manifest and system-unit inputs
under a root-controlled directory whose files are readable but not writable
by the Endpoint service account. One manifest binds their exact digests to
the accepted artifact, account, unit names and MainPID verification rule.

The existing `endpoint headless` consumer then verifies that binding and
file ownership before opening State, Instance or worker resources; the
system manager's observed unit/account/InvocationID/MainPID must match it.
Restart re-verifies the same durable plan and floors. Manual replacement
uses the accepted Release decision and explicit predecessor recovery without
lowering its floor. A partial write, mismatched plan,
service-account edit, wrong unit or wrong executable fails before participant
effects and leaves the prior accepted bundle recoverable. This is a proposed
authority boundary, not an implemented installer or an accepted change to
the v2 plan grammar. It needs the affected product/security owners and an
independently reviewable implementation slice before C0 qualification.

### Verification evidence for the combined candidate

The [testing owner](testing.md#current-profiles) defines the checked profiles;
this table records what their current code can establish for the reconstruction.
Run results must name one exact integrated artifact and host. A green profile
on a different branch or a fixture-produced installation cannot be promoted
to installed C0 acceptance.

| Required fact | Existing profile and source evidence | Remaining combined-candidate proof |
| --- | --- | --- |
| Repository code and artifact gates | `make quick-check` covers formatting, architecture, vet, deterministic packages and command builds; `make check` adds process/race/static/vulnerability and Linux package evidence. `make headless-check` checks the headless artifact lane. | Re-run required gates on the exact integration candidate after Endpoint/network reconciliation. None alone exercises the installed ordinary text command journey. |
| Ordinary Publisher/Reader exchange on both Carriers | `make text-command-network-check` pins five command binaries, the worker, test binary and temporary root-owned unit; `TestInstalledClosedTextCommandsThroughNodeProcesses` loops TCP/TLS and QUIC and exercises publish, Link, read, refresh, withdraw and refusal. The testing owner names empty, 64 KiB and 4 MiB cases. | Replace test-written runtime plan, authority responses and temporary unit with the maintained authenticated provisioning path while retaining the same real-command and terminal observations. |
| Worker confinement and lifecycle | `text-worker-network`, `text-worker-lifecycle`, `text-worker-policy`, and `text-worker-escape` each have a separately selected installed profile. They check bounded network composition, effective unit/cgroup lifetime, policy and hostile-worker access attempts. | Bind their exact worker artifact and unit policy to the accepted Release/enrollment candidate; preserve the Endpoint MainPID, service account, socket and six-file manifest identity on restart. These profiles do not by themselves qualify the whole host. |
| Interrupted work and durable recovery | `text-worker-recovery` interrupts one accepted request over each Carrier. Module/process tests cover token, publication, State and replacement journals; the replacement qualification concerns the existing user-unit adapter. | Exercise selected installed restart, retained floors, explicit rollback/refusal and final cleanup under the supported system-unit composition. Do not count a module recovery oracle as that installed result. |
| Authority and version refusals | Deterministic/architecture tests cover bounded old-input refusal and retained parsers; `text-command-network` uses current fixture-issued permissions. | Select sole AREP closed intake and old-root policy (F-50), retire alternate operator startup only after its successor, and verify old inputs cannot enter the new installed accepting path without losing retained floors. |

`make check` does **not** invoke the separately selected
`text-command-network-check`; both results are necessary on the same candidate.
The final verdict also needs a record of candidate digests, Ubuntu/systemd
environment, exact commands, Publisher/Reader outcomes for both Carriers,
restart and withdrawal outcomes, and any failed cleanup result. This is the
evidence boundary for the design; the implementation owner still decides the
specific test cases and profile registration with its bounded slice.

The permission files in Endpoint are the holder side of this handoff, not the
Node issuer. Offline provisioning first prepares an installed worker, exports
one public request, and accepts only a matching Custody response. The
`textPermission` stock and `textIssuanceOperation` retain one exact blinded
batch across an uncertain issuer exchange; `text_token_transfer.go` marks a
token attempt in the durable journal before its bytes reach Route. Node's
issuer root and receiver-side Replay spend remain different authorities. A
package split must preserve this order and the Context's cancellation/join
ownership rather than grouping every `token` file under one owner.

### Receiving wire: physical and virtual close are separate

The same `serveClosedOuter` in Node serves issuer, forwarding, resolution,
Introduction and Data Join. It owns the accepted physical connection,
interrupts it, closes Route's `ClosedOuterBridge`, then joins every inner
handler. Node's `closedOuterWriter` serializes terminal, control and payload
frames on that connection. These two files belong to the Node duty lifecycle;
moving them to a wire package would transfer the accepted-handler join owner.

Route's `ClosedOuterHandshake` and `ClosedOuterBridge` validate and multiplex
virtual inner lanes. `ClosedOuterBridge.Close` wakes blocked lane readers; it
does not join the Node handlers or retire the physical Carrier by itself.
`ClosedAdmissionChannel` binds an admitted lane to the role TLS exporter,
spends its token in the durable Replay ledger and transfers a bounded
reservation to the duty handler. Data Join's `ClosedJoinPairs` owns the
ephemeral matched pair and its queue/timer; Node owns its listener, spend root
and final shutdown. The boundary between these owners must preserve the exact
order of interruption, child join and spend-root close.

The source pass also reassigns `closed_role_tls*.go` to Carrier identity,
`closed_terminal_recipient.go` and `closed_introduction_delivery.go` to the
client path, and JOIN receiver files to receiving spend. Their filename prefix
was insufficient evidence for a single protected-wire package.

### Target dependency order to validate

This is a component import direction, not a registry of new Go package names.
It is derived from the observed Linux/Windows [package graph](repository-package-graph.csv)
and the exact operation seams above. A package extraction must implement one
row with a real caller and tests; the whole graph cannot be created as empty
scaffolding.

```text
command composition
  -> Endpoint / Node / offline Custody and Control
Endpoint -> checked State, Service owners, client path, local Application owners
Node     -> checked State, Reachability Store, receiving spend, receiver duties
client path -> checked State for per-operation freshness and peer selection
client path / receiver duties -> protected channels -> physical Carrier
client path / receiver duties -> network token adapter -> offline token grammar
protected channels -> ARDP and bounded operation grammar
Reachability Store -> Service Publication
Service Connection -> standard library only (current boundary)
offline Custody / Control -> offline token and issuer-profile grammar
```

The Carrier takes an already selected profile, certificate and current-peer
verifier from its caller; it does not decide State authority. Protected
channels enforce framed operations and own child/lane retirement; they do not
commit a Descriptor, spend a permission, or declare a Node duty current. Node
and Endpoint compose those decisions and own their process terminal results.

Five observed edges determine the next Go import-graph changes:

1. `route/credential -> route` now comes only from three live issuer adapters
   (F-30); ADR-0092 removed the uncomposed OHTTP Transit client. Keep issuer
   reservation and offline permission/profile grammar below the live network
   listener. Put Route-facing bootstrap, admission channel and Carrier calls
   behind a Node-owned operation adapter only after assigning accepted-child
   join and issuer-root late-close ownership (F-17). Moving the current
   Credential package inward as one unit creates a parent import cycle.
2. `route/terminal -> service/reachability` exists only for the 15,000-byte
   Descriptor bound in `descriptor.go` and `descriptor_client_linux.go`.
   Reachability remains the proof/Store authority; a terminal-local copy with
   an exact cross-owner boundary test cuts the production import without
   changing refusal behavior or adding a one-constant package (F-29).
3. Offline Control and Custody import all of `route/credential` for the
   profile decoder and permission grammar. They should reach the same verified
   bytes through a cohesive offline owner, not through the live Route/QUIC
   issuer adapter. Compare their `go list` closures after the real split;
   their current 296- and 229-package Linux closures include QUIC and CIRCL
   but no OHTTP. This is import coupling, not proof of runtime network use
   (F-28).
4. `route -> network/state` is a live Linux edge in
   `closed_bootstrap_plan.go` and `closed_source_prefix.go`: the client path
   builds a bounded plan from the current view and rechecks it while opening
   the prefix. Keep a checked-State dependency in that operation boundary;
   replacing it with one stale Endpoint snapshot would lose the existing
   freshness check. It can use a smaller caller-local view only if that view
   preserves each pre-effect recheck.
5. `route -> entry` comes from `entry_attachment.go`, the uncalled old v2
   attachment path after ADR-0092. Retire that edge with the v2 execution
   closure after its refusal and retained-root obligations are settled
   (F-08/F-52), not as a prerequisite for the live client path.

**Package placement decision for the first reconstruction slices.** The
physical package map remains authoritative until each extraction lands with
its caller and tests. These are concrete target placements, with the blocking
source edge stated where the package cannot yet be created:

| Placement | Current decision and exact gate |
| --- | --- |
| `internal/network/state`, `internal/service/{instance,publication,reachability,connection}`, `internal/route/{ardp,capsule,replay,terminal}` | Keep these existing authority, wire or lifetime owners. Repair the State role join, Service Connection completion barrier and terminal-to-Reachability size-bound import in their own slices; none requires a replacement catch-all package. |
| Candidate `internal/route/carrier` | Move the 13-file physical transport/authentication cohort only after moving the one `CarrierProfile`/`Carrier` byte-lane contract, shared TLS peer and literal-address rules, role TLS helpers, and the exporter function type out of Route-root declarations. Node and Route's client/receiver paths are real callers; Carrier must import neither parent `route` nor receiving spend/Service authority. The old v1 symbolic constants are not an accepting Carrier API. |
| Candidate `internal/route/client` | The 25 Route-root outgoing bootstrap, Source, Introduction, resolution and JOIN files may follow Carrier after their transitive shared declarations and per-effect State rechecks are preserved. The Endpoint-local Source handle, token attempt, Job and publication coordination stay in `internal/endpoint`; moving them would import its private context back into Route. |
| `internal/route/credential` with a Node-facing issuer adapter | Keep the live issuer together until its late root-close owner exists. Then separate the three Route-importing listener/bootstrap/admitted files from the offline profile and permission grammar. Do not make `route` import `credential` while `credential` still imports parent `route`. |
| `internal/node` and `internal/endpoint` | Keep them as process/local composition roots while deepening private duty and participant owners. A possible `internal/node/outer` has only the two accepted-outer lifetime files as its source closure; Endpoint qualification already owns workload/verdict separately but still borrows Endpoint runtime. Neither root gets a speculative subpackage just to hide filenames. |

The remaining receiving Route cluster stays in its current package until its
Node accepted-child and spend transfer is expressed as one error-bearing
operation. An inventory responsibility tag is not, by itself, a suitable
package name or Interface. The [Route boundary analysis](route-refactoring-boundary.md)
records the Carrier type-use blockers, and the [focused inventory](c0-component-inventory.csv)
records the exact candidate files. This placement preserves an acyclic
direction without requiring every component to become a package at once.

The first-party graph counterfactual in F-28 shows that these two edge cuts
primarily simplify offline Control/Custody. They do not shrink the Linux
Endpoint or Node first-party closure because those commands reach the same
owners through other live imports. Keep the offline split after the State
role-join, resource-transfer and installed-path boundaries that affect C0
correctness; measure real Go dependency and artifact changes only after a
buildable split.

The existing `service/connection` package is already below Endpoint and
Route: it has no first-party imports and receives authenticated Attachment
bytes through an opener. Preserve that direction while fixing the lost
replacement-close result. The ordinary native `Done()` currently closes before
physical `stream.close()`, while the terminal tail closes it afterward; align
that completion barrier and keep the Endpoint-owned close result observable at
final `Close()` without changing a previously published Application outcome
(F-23). Revalidate the graph against the active Endpoint
agent's completed revision before assigning exact package paths.

## State and close ownership

| Resource or decision | Owner at the boundary | Terminal obligation |
| --- | --- | --- |
| Current Network State | State runtime; Node and Endpoint borrow checked views. | Close the State root after borrowers stop; successor/loss refuses new effects. |
| Endpoint replacement writer lease | `endpoint/replacement` claims it for Prepare, CommitPrepared, Replace and Rollback. | Join the writer-lock release result to the public transaction result after the last journal/program effect. Four writer paths currently discard it (F-68); a release failure cannot undo an already committed pointer or imply rollback. |
| Local grant and participant job | Endpoint/Broker and the exact participant context. | Revoke new admission, cancel all children, then join operations and worker. |
| Instance key and publication generation | Instance and Publication owners, borrowed by Endpoint. | Withdraw publication, join leases, erase/close the Instance binding in the required order. |
| Private Descriptor revision/conflict floor | Reachability Store under one Node resolution duty. | Return success only after durable write; refuse new operations after ambiguous write failure, restore the highest valid floor on reopen, and recheck duty currentness before ACK. |
| Source/Introduction/Responder prefix | Exact client-path owner. | Retire admission and join child channels before releasing the parent transport. |
| Node listener and accepted sessions | The selected Node duty. | Stop acceptance, join accepted producers/readers, then release spend and host resources. `Drain` retains listener/root errors; only Data JOIN currently retains non-benign accepted-connection close errors (F-61). Decide the common cleanup rule before moving this seam. |
| Physical Carrier | Carrier lease/connection owner. | Close once; report physical close failure to its borrowing duty or path. |
| Logical Service Connection | Service Connection owner with Endpoint's attachment opener. | Verify terminal outcome and join physical attachment cleanup before reporting completion. |
| Confined worker | Endpoint's installed-worker owner. | Stop and join its process tree/cgroup before releasing its job authority. |

### Durable roots and their first lifetime owner

This is the selected journey's root registry, not a directory layout proposal.
The operator plans supply paths; each named Module owns the bytes and recovery
rule. The Endpoint plan checks its supplied root/socket/file paths for exact
distinctness, absolute form and canonical spelling in
`cmd/ardents/endpoint_text_plan.go`. A Node plan selects exactly one closed
receiver duty; the role-specific rows below are alternatives across Nodes.

| Root supplied or opened | First durable owner and current opener | Retained fact and boundary on redesign |
| --- | --- | --- |
| Portable artifact, Release floor, Endpoint replacement | `endpoint/portable` verifies the first artifact; `release.Open` owns the decision floor; `endpoint/replacement` owns current/prepared/journal and retained predecessor. | Preserve first-run pin and accepted Release floor separately from executable activation. Replacement's predecessor is for explicit authorized recovery, never a second running version. Its writer-lock release result is currently dropped by four public operations (F-68). The `systemctl --user` adapter is not yet the protected system-unit composition (F-27). |
| `network_state_root` | `network/state.Open` in both Endpoint and Node commands. | State alone commits accepted Epoch, current/pending and conflict floors; borrowers receive checked views and close before its root. Historical Epoch intake needs the F-50 decision before decoder retirement. |
| `local_role_state_root` and `entry_state_root` | `network/duty` owns local role/conflict records; `entry.OpenClosedSets` owns the Endpoint's closed Entry set. | Keep the role and Entry roots distinct from each other and from State. The old Invite command uses a different retained Entry root; it does not supply the selected closed set (F-08/F-53). |
| `service_instance_root` and `publication_root` | `service/instance.Open` supplies the binding; `service/publication.Open` is constructed by Endpoint's `newEndpoint`. | Keep signing key/generation separate from Descriptor publication/revision. Endpoint joins publication and Instance close after local work; old Instance key material has a separate disposition (F-42). |
| `text_token_root` | `endpoint/tokenjournal.Open` on Source-prefix admission; Endpoint holds the opened journal until its source roots close. | Retain retry/presentation state across restart; it is not Node's receiving spend ledger. Its close result joins the Endpoint terminal outcome. |
| Closed Issuer `root` and `admission_root` | `route/credential.OpenClosedTokenIssuer` and `route/replay.Open`, composed by `node/startClosedIssuer`. | Issuer key/issuance and receiving spend have separate roots. On a bounded Drain timeout the current adapter lacks a later root-close owner (F-17); the target finalizer must join workers before closing both. |
| Closed forwarding `root` and `hosting_root` | `route/replay.Open` via Node receiving resources; `resource.OpenHosting` via Node's shared host adapter. | Spend once and host-period accounting are different authorities. A shared host sampler can serve concurrent duties, but its lease and terminal result remain with Node. |
| Closed Resolution `root` and `admission_root` | `service/reachability.OpenStore` and `route/replay.Open` in `node/startClosedResolution`. | Descriptor revision/conflict and receiving token spends must retain distinct floors; a publication ACK follows durable Store acceptance, and shutdown joins both roots. |
| Closed Introduction and Data JOIN `admission_root` | Each Node listener opens its own `route/replay` receiving ledger. | No timeout may release a spend root while an accepted child still owns a commit or reply. The duty owns the final join, not the Carrier package. |

This registry covers the selected C0 data path and the directly required
first-run/replacement path. Other maintained roots, including Custody Vault,
Namespace and legacy operator state, remain in the broader file and behavior
maps until their accepted retention or retirement decision is made. They must
not be silently folded into one of these C0 roots.

The reduction decision for those other roots is bounded by their actual
openers, not by a package name:

| Retained or absent root | Current owner and consequence for reconstruction |
| --- | --- |
| Custody Vault and its offline allocation journal | `cmd/ardents-custody` and the permission handover open `internal/custody` authority; the Vault commits its own floor before the Endpoint token attempt or Node receiving spend. Keep this offline trust root separate from all Route/Endpoint journals. Its verification command closes the Vault and reports close failure. |
| Older Invite Entry root | `ardents entry import` can still write this root; the closed Endpoint opens a different `entry.OpenClosedSets` root whose marker rejects Invite data. Stop the old writer only after the accepted operator and stored-contact disposition; never import Invite bytes as closed members by filename coincidence (F-08). |
| ACA1 reader and transition floors | `ardents-control inspect-bundle` and `inspect-transitions` open the ACA1 reader/catalog root and separately advance Release and Network inspection floors. ACA2 corpus inspection has no corresponding reader root. One-format convergence must decide these floors before deleting ACA1 (F-49/F-64). |
| Retained Contributor installation root | The Contributor command can diagnose, drain, withdraw and remove an already owned installation; its recovery handles current/predecessor generations but does not start new installation. Keep its exclusive root and cleanup outcomes until the accepted retirement transition ends. |
| Namespace composition | The maintained command graph opens no Namespace runtime root or Gateway/Resolver. Resolution remains retained by ADR-0090 pending its exact-consumer decision; it is executable code without a current C0 process owner, not an extra root to merge into State or Reachability (F-01/F-51). |

The [behavior traces](repository-behavior-map.md#source-trace-separate-service-authority-issuance)
and [one-version disposition](#one-supported-c0-configuration) supply the
caller and migration/refusal questions for these rows. This table records
ownership; it does not decide to erase a retained root.

These relationships are checked against the current
[Node/Route contract](../technical/network-route-node.md) and
[Endpoint/Service contract](../technical/endpoint-service-runtime.md). The
[first per-file ownership pass](c0-component-inventory.csv) is recorded.
The [Node receiver resource matrix](repository-behavior-map.md#node-receiver-resource-matrix)
now distinguishes all five duty roots and late cleanup after a caller timeout;
the Issuer alone lacks that later root-close owner (F-17). The installed text
Endpoint's local socket servers and adapters have a source-backed handler
join before Endpoint, Instance and State close. Nested worker and Route
lifetimes, physical connection-close error policy and combined installed
evidence still require targeted review before moving package boundaries. The
[JOIN-to-Service handoff trace](repository-behavior-map.md#source-trace-join-transport-transfer-into-service-connection)
now resolves the initial Route stream's opening-failure and successful-transfer
close owner; replacement-close result and native completion barrier remain
open under F-23.

### Issuer's late close owner

`startClosedIssuer` transfers an opened key root and receiving spend ledger
to a Node-local listener adapter. `ClosedTokenListener.Done()` reports its
accept-loop result; `ClosedTokenListener.Drain(ctx)` stops acceptance and
waits for its workers, but deliberately does not close either root. Today the
adapter closes both roots only when that bounded Drain succeeds. If it times
out, the listener may finish later without a caller that releases the roots.

The target Node duty owner needs one retained finalization result, distinct
from the caller's bounded wait. Its first Stop starts exactly one finishing
path: stop the listener, wait for its workers without the caller's short
deadline, then close the spend ledger and issuer root in that order, retaining
the listener terminal cause, Stop, spend-close and issuer-close results.
`Done()` is a one-send accept-loop signal that `node.Run` may already consume;
the finalizer must share the recorded cause instead of receiving that channel
a second time. The Node adapter can be its sole consumer, forward a buffered
failure signal to `node.Run`, and retain the cause for finalization.
`Drain(ctx)` waits for the final result up to the configured
deadline and reports unproven cleanup on timeout; it does not claim success or
close roots while a worker still borrows them. A later wait on the same owner
must return the same recorded result without closing twice. This requires no
new Route/Credential package and does not make the issuer key or spend ledger
one authority. If the process exits after a timeout, a late in-process close
cannot be promised; the terminal result remains failed/unproven and restart
must prove durable-root recovery separately. A delayed accepted-child test
must check lock exclusion before join, eventual close while the process stays
alive, repeated wait results, and a distinct root-close error (F-17).

## First source inventory

The inventory covers all **360 tracked, non-test Go files** under
`internal/node`, `internal/route`, `internal/endpoint`, and
`internal/service` at committed `53f02e64`: Node 45, Route 127, Endpoint 126,
Service 62. It includes platform files and package documentation. It excludes
commands, other `internal` packages and tests. No committed file
is omitted or assigned twice. The CSV assigns a proposed primary
component to each file; a mixed file is explicitly marked for a boundary or
split review. This is an ownership hypothesis, not a deletion list or a
package-move instruction.

| Treatment | Files | Meaning |
| --- | ---: | --- |
| `retain` | 165 | The existing package or primitive appears to have a coherent owner; preserve its contract while composing the new shape. |
| `deepen` | 96 | Local ownership is visible but buried in a broad package; make the owner and lifetime explicit before deciding on a new package. |
| `move-candidate` | 32 | The file's current package mixes different component responsibilities; decide the destination with the caller and close graph first. |
| `boundary-review` | 40 | The file bridges components or its exact owner is unclear from the current API. Eight Node and five Route rows now have source-backed owners (F-33/F-34/F-44); Service Connection's `Done` promise and ordinary close order still disagree (F-23). |
| `split-candidate` | 10 | One file contains distinct responsibilities; determine whether a local file split or a package boundary is justified. |
| `compatibility-review` | 4 | A codec or Attachment appears to have no maintained production caller; confirm the accepted compatibility evidence before disposition. |
| `retirement-review` | 13 | The remaining twelve Route v2 files and one Service plaintext instruction need an exact caller, persisted-state and accepted-contract audit before removal (F-30/F-37/F-42). The generic Endpoint chain was retired by ADR-0092. |

The strongest positive boundary is already in `service/connection`,
`service/instance`, `route/replay`, and Endpoint's small durable packages.
`route/credential` has a cohesive token and ledger core, but three production
files import the parent `route` package for its listener, admission channel
and bootstrap controller. The largest
unresolved boundary is the Route root package:
its 74 production files mix Carrier, protected wire, client Source/JOIN,
receiver admission, and compatibility evidence. The Endpoint root has 95
production files with participant lifetime, publication, worker, and client
path mixed together; recheck this inventory after any later accepted slice.

Several source checks changed the classification:

- `reachability.OpenStore` is called by Node resolution, so Reachability's
  `store*.go` files are active storage, not old debris. `portable` and
  `replacement` are called by `cmd/ardents`; they remain in the artifact and
  enrollment path.
- `node/closed_forwarding_admission.go` holds both forwarding reservation and
  a role-token verifier used by control duties. `node/closed_hosting.go` holds
  the shared host period, control admission, and pressure response. Moving
  either whole file by its prefix would put shared authority under one duty.
- `route/closed_node_open.go` combines the Node OPEN grammar with an accessor
  on the bridge lane. `route/terminal/descriptor.go` imports
  `service/reachability` for the private Descriptor bound. These are concrete
  cases where a desired package move needs an API/dependency decision first.
- Admission is a process-spanning operation, not one storage owner:
  Custody's encrypted allocation journal, Credential's issuer reservation
  ledger, Endpoint's potential-presentation journal and Replay's receiving
  spend ledger have separate retry, restart and close rules. The installed
  text command test invokes real Custody permission issuance for both roles;
  these boundaries are current C0 behavior, not candidate file names.
- The current Go import graph has `node -> route`, `endpoint -> route`,
  `route/credential -> route`, and `route/terminal -> service/reachability ->
  service/publication`. The issuer listener in `route/credential` therefore
  cannot simply be pulled into a Route-root caller, and a new low-level wire
  package must not inherit all of Reachability's store dependencies
  just for its Descriptor size limit.
- The v1 `LegBinding` codec and older Reachability Gateway/Relay/Client path
  have no external production caller found in the targeted symbol pass. They
  are review candidates only; this pass did not establish that their accepted
  compatibility or test obligations can be retired.

### Ten mixed files: first split within existing packages

The ten `split-candidate` rows have source-level first cuts.
These cuts name implementation responsibilities, not new package names or
permission to change admission behavior.

| Current file | Distinct responsibilities and callers | First cut |
| --- | --- | --- |
| `node/closed_forwarding_admission.go` | `closedForwardingAdmissionVerifier` and `closedForwardingReplenisher` reserve the forwarding host allowance and spend class-2 tokens; `closedRoleTokenVerifier` verifies selected-profile class keys for forwarding **and** control duties. | Keep forwarding allowance/replenishment together. Give the shared role-token verifier its own responsibility file in `node`; both forwarding and control continue to call the same verifier. Preserve reserve-before-spend and release-on-spend-failure ordering. |
| `node/closed_hosting.go` | `openClosedHosting` opens one period before duty activation; `hostingPressure` samples that period for `resource_pressure.go`; `closedControlTokenVerifier` wraps shared token verification with class-1/3 host reservation for issuer, Introduction, Resolution and JOIN callers. | Keep opening and pressure under the host-period owner; place control admission beside shared token verification or in its own control-admission file. Keep the same host handle and reservation lifetime. |
| `node/contract.go` | `DutyView`/`dutyFacts` copy authenticated State input; `Config` and role profiles select local roots/keys; `Event`/`Result` report lifecycle; `runtimeConfig` holds mutable host, pressure and clock state. | Split by State projection, local configuration, and lifecycle observation/runtime state inside `node`. Apply the source-backed State-owned copied `NodeDuty` handoff from F-07 in its own bounded slice before removing the duplicate getter facade; a file rename alone does not change the seam. |
| `route/closed_node_open.go` | `EncodeClosedNodeOpen`/`DecodeClosedNodeOpen` enforce the mandatory 50-byte Node OPEN and restriction grammar; `(*ClosedOuterBridgeLane).Restriction` reads an already-authenticated bridge child constraint. The decoder is called by the handshake and bridge; Node consumes the encoder/accessor. | Keep wire grammar together; move the accessor beside `ClosedOuterBridgeLane` implementation within `route` before considering package extraction. Preserve refusal of the retired 49-byte form and the non-ordinary nil-lane result. |
| `endpoint/text_source_state.go` | `closedTextRoleMembers` binds current State to candidate members; `textEntrySets` opens the retained Entry root; `closeTextSourceRoots` closes Entry and token-journal roots. | Keep the State/Entry projection in an admission responsibility file and place the root open/close methods with the Endpoint root owner. Preserve the same lock and close-error path. |
| `endpoint/text_source_prefix.go` | `openTextPrefix` reserves a Source opening and Route prefix under Context; `endpoint.textTokenJournal` lazily opens the root-owned durable journal. | Keep Source opening with its operation identity and move the token-journal root accessor beside Endpoint root lifetime. Do not transfer journal ownership to a transient prefix. |
| `endpoint/text_introduction_registration.go` | `openTextRegistration` and `finishTextRegistration` own one registered Route channel and token spend; the latter half of the 401-line file projects committed Descriptor proof, refresh time, recipient and predecessor state. | Keep the registration type and its lock/ACK invariant; put registration opening/withdrawal and committed proof/refresh methods in responsibility-named files inside `endpoint`. Do not treat registration as publication readiness before the Descriptor ACK. |
| `service/connection/stream_lifecycle.go` | `Run` implements exact-count work and has no non-test `cmd`/`internal` caller, while `establishInitialAttachment`, failure and close methods also serve current `RunBounded`. The package map still says exact workloads are supported. | Move the `Run` entrypoint into an exact-workload responsibility file inside `connection`; retain shared lifecycle methods with the bounded path. Decide whether to retire exact-workload support from the accepted owner before deleting the method. |
| `service/connection/stream_support.go` | `acquireResource`, `erase` and `writeAll` serve timer observation, continuity-byte clearing and Application writes respectively. | Put each small helper beside the lifecycle or receive operation it supports and remove the catch-all file. Preserve timer release and full-write behavior. |
| `service/instance/lifecycle.go` | Current durable Instance acceptance, Binding signing, publication commit and withdrawal coexist with the old `OpenIntroduction` SealedIntroduction v1 decryptor, which has no non-test caller after committed generic Publisher deletion (F-42). | Extract the old decryptor before retirement; preserve the current Binding methods. Audit persisted Introduction key fields separately from method reachability. |

Each cut can be reviewed with existing direct tests and a source diff before
any package move. It does not shorten the required final gate, and it should
be scheduled after the active Endpoint/network slice rather than run as a
second implementation stream.

The table is intentionally file-level, not a claim that every Go file needs a
different module. Before a move, inspect its non-test callers, imports,
mutable state, resource ownership, and terminal cleanup. Then choose one
interface and import direction for the component, updating `package-map.md`
only when the package is actually introduced or renamed.

## Where current packages obscure the model

1. `internal/route` currently combines Carrier opening/listening, ARDP and
   terminal grammar, admitted channels, client Source/JOIN operations, and
   receiver-side primitives. The word *Route* does not identify which of those
   owns physical transport, protected wire, or a client path.
2. `internal/node` composes a process and five receiver duties in one package.
   Forwarding also retains outgoing Carrier sessions and links; Route owns
   some of their physical and admitted channel behavior. Their stop/join
   relationship is real and must be explicit in the new shape.
3. `internal/endpoint` composes local capability admission, worker, token stock,
   publication, Route prefixes, authenticated Service attachment and Application
   exchange. Its context coordinates necessary cross-owner ordering, while
   private owner implementations remain difficult to navigate in one directory.
4. `internal/service` already has distinct Instance, Publication, Reachability,
   Target Link and Connection packages. The distinction between Service's
   logical Connection and Route/Carrier physical connections needs to appear
   in interfaces and naming at their handoff. The Route terminal Descriptor
   codec currently imports `service/reachability` for a size bound, a concrete
   dependency to resolve when placing wire grammar.
5. `closed_*` and `text_*` filenames encode the implementation campaign across
   many responsibilities. A package or owner name should supply that context
   after the actual boundary is moved; a bulk rename alone cannot establish it.

## Reconstruction method and next decisions

1. Carry the three selected handoffs into bounded owner changes:
   State-to-Node's checked duty value (F-07/F-45), Route/Credential's live
   issuer adapter versus offline grammar (F-17/F-28/F-30), and the
   Service Connection attachment's physical-close result (F-23). The table
   of observed handoffs above already gives callers, input, result and close
   owners. State's value fields await the accepted role join; Credential's
   package move awaits the late issuer-root close owner. Service Connection's
   error-bearing callback and completion barrier can be corrected in its
   existing package. Refine exported signatures only in the selected slice.
2. Decide one-version retirement in accepted contract order: old Route v2
   execution, Invite Entry writer, Reachability old writer/root, Instance old
   key/root and AREP intake. For each, record last caller, persisted floor,
   migration or typed refusal, and reader-removal criterion. Do not keep a
   second writer simply because its decoder must temporarily remain.
3. Assign each focused source cohort **retain**, **move/deepen**, or
   **retire after decision** from the existing 360-file inventory. Reopen an
   individual file only when those three decisions still conflict with its
   caller or resource owner. This avoids treating 815 direct source traces as
   a prerequisite for the architecture verdict.
4. After the active Endpoint/network slices finish, freeze one integrated
   candidate. Diff their actual production and test owners against this map;
   preserve their changes and revise only conclusions invalidated by the diff.
5. Hand bounded, accepted slices to the one active C0 implementation owner
   under the repository work-in-progress limit. A new Go package, if needed,
   arrives with its real caller, implementation, tests, `doc.go` and
   package-map edge in one buildable slice. The combined installed Ubuntu
   Publisher/Reader journey on TCP/TLS and QUIC, plus `make check`, is the
   later implementation acceptance gate, not a condition for finishing the
   architecture analysis.

The existing [Node refactoring plan](node-architecture-refactoring.md) and
[Endpoint ownership map](endpoint-architecture-refactoring.md) are inputs to
steps 1-2. Their proposed packages are not fixed until the cross-system
dependency and close-ownership map is complete.
