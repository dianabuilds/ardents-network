# Stage 8 refactoring and retirement plan

Status: **S8.4 accepted by the Product Owner on 2026-08-22.** This temporary control
document instantiates the accepted S8.3
[target architecture](stage-8-target-architecture.md). It is not a second
technical specification and is deleted in M14 after its retained facts have
current owners. The human Product Owner and Codex are the only execution team;
each wave therefore has one Codex integration owner and no presumed external
validation staff.

## Wave contract

Before a wave starts, Codex records its exact `HEAD`, relevant Go/toolchain and
input identities, prerequisite decision IDs, and affected observers. A wave
may create a target package only with real behavior, behavior/failure tests, a
non-test caller, `doc.go`, and a factual `package-map.md` row in the same
change. The old writer and target writer never operate simultaneously.

Every completed wave supplies: characterization at the old observer; target
normal/failure/restart/race tests; real process/platform coverage when the
target requires it; caller cutover; cleanup/residue assertion; format
reader/writer disposition; current technical/operations documentation; and
deletion of the listed old directory, command, fixtures, and tests. A blocked
decision, unsafe dual writer, unknown observer, failed cutover, or untestable
repair returns the wave to its frozen input.

Format modes are deliberately limited:

- **C0 coordinated break** — no named observer; remove old reader/writer in
  the cutover.
- **C1 atomic local migration** — target reads validated old local state once,
  writes only target state, atomically switches pointer, and forward-repairs;
  it never reactivates the old writer.
- **C2 bounded adapter** — a named observer uses a target-owned adapter with
  an explicit expiry and removal test.
- **C3 protocol phase** — versioned peer transition with downgrade, mixed
  version, retirement, and Qualification rules.
- **C4 provenance only** — immutable historical evidence remains readable by
  its independent verifier; runtime code is removed.

`DA-*` means the stop condition in the accepted
[decision-authority register](stage-8-decision-authority-register.md). A
Product Owner acceptance of this plan admits the ordered plan, not a bypass of
one of those conditions.

## Ordered wave graph

| Wave | Frozen input, owner, and objective | Authority/format/cutover | Required evidence and deletion outcome |
|---|---|---|---|
| **M0 — governance** | Input: S8.3 accepted map and S8.2 policy. Owner: Codex. Objective: make policy, profile, architecture, format, disposition, and retirement control one checked system. | C0 for superseded numeric gates; no runtime writer changes. Current owners are `AGENTS.md`, development policy, profile manifests, package map, target architecture, and this plan. | `quick-check`; architecture/profile/document gates. Delete only obsolete command/export caps and negative profile filters already replaced in S8.2. M0 ends when all later paths below are owned by a wave. |
| **M1 — Release trust** | Input: `internal/releasedecision`, `cmd/ardents-release`. Owner: `release`. Objective: own verified metadata, root/floor/archive, lease, and opaque update authorization. | DA-01 is mandatory. D06 uses C1 security-forward migration: roots/floors never decrease; no staging/store export survives. | Metadata/adversary/rotation/floor/archive/restart tests and update-authorization integration. Cut all callers to `release`; delete `internal/releasedecision/` and the Release command path or retain only M13-approved adapter. |
| **M2 — Update lifecycle** | Input: `internal/updatetransaction`, Release interface, `cmd/ardents-release`. Owner: `update`. Objective: own the bounded H3 technical-tracer stage, stop/drain, activation, self-test, journal, recovery, rollback, and cleanup. | DA-01, R-064, and R-088 are mandatory. D07 retains its current forward-repair/safe-rollback behavior without selecting a supported lifecycle. | Checkpoint/fault/recovery/idempotence/rollback/residue, C0 V2 cutover, and C4 V0 verifier tests. Real supported filesystem/activator evidence is explicitly out of scope. Delete `internal/updatetransaction/` and caller-authored transaction choreography. |
| **M3 — authenticated Network State** | Input: `internal/network/{epoch,epoch/assignment,epoch/merkle,framing,source,state,store}`, `internal/namestore`, `cmd/ardents`, e2e network-source. Owner: `network/state`; `network/source` is its acquisition port. | DA-02 and DA-05 are closed by R-059/R-060/R-061. **R-061 prerequisite:** first transfer only the currently used Namespace root/pointer and proof mechanics to its existing `namestore` owner, preserve its bytes and one writer, and remove both Network imports. D01 is C1 with one current/pending/control writer; W01 is C3 or C0 only after its observer decision. | Namespace durable/reopen/tamper/stale/partial-write and proof-mutation characterization plus a no-Network-import assertion precede the Network cutover. Then supply decision-time validity, source-wave, corruption, physical-root, restart, source cleanup, and process tests. Delete all listed old epoch/framing/store directories and concrete Source reverse dependency after caller cutover. |
| **M4 — Duty and Resource** | Input: `internal/localroles`, `internal/resource`, callers in State/Node/Route/Bridge. Owners: `network/duty`, then `resource`. Objective: one monotonic duty writer and one finite resource coordinator. | D02 is C1 with no generation/watermark reset. Resource platform scope must be explicit; unsupported platform fails closed. | Duty conflict/expiry/restart/physical-root tests; resource reservation/hysteresis/oversubscription/counter-reset and native adapter tests. Delete `internal/localroles/`, all displaced per-Module guards, and old duty readers. |
| **M5 — Namespace** | Input: `internal/{nameadmission,nameauthority,nameclaim,namelease,namerecovery,namestore,naming}`, including the R-061-owned local persistence/proof mechanics, name command, Stage 6 fixtures. Owner: `naming/namespace`. | DA-03/DA-04/DA-07 are closed for this wave by R-065/R-066/R-067; DA-05 is closed by R-060/R-061. Retain only R-067 profile facts. The target is the R-066 127-record technical tracer, not a product scale selection; D04/W03 internal tracer bytes are C0 unless a later observer decision says otherwise. | Authenticated create/renew/control/recovery/claim/materialization, durable reopen, tamper, and codec/property tests. Delete the six former state-package directories; retain `internal/naming` only as the cohesive R-041 canonical vocabulary package. Delete duplicate validators/field bags, stage fixtures, and command-only control wiring after cutover. |
| **M6 — private Resolution** | Input: retained private-resolution behavior, current name command and e2e role fixtures. Owner: `naming/resolution`. | DA-03, DA-04, and DA-07 are mandatory. C3 applies to retained OHTTP/wire behavior; no plaintext fallback or shared implementation view. | Opaque Namespace/State port tests, replay/admission/failure/scale tests, and selected real resolution process coverage. Delete `internal/nameresolution/` and old plan/command imports. |
| **M7 — Entry and Carrier** | Input: `internal/bridge`, `internal/camouflage`, bridge/route commands. Owner: `entry`. | DA-06 is closed by R-076/ADR-0024; R-077/ADR-0025 select the sole Entry Invite reader/writer, and R-079/ADR-0027 bind each Invite to a fresh TLS attempt key without creating a User identity. R-080 closes DA-11 for this Stage-5 runtime subset: records/readers are C4 provenance, but live runners and runtime are retired. D03 is C1 only with revision-safe replay/attempt recovery. W04/WebTunnel is C0 retired; no `route/webtunnel` package is created. | Invite/replay/replacement/durable-fault tests; EntryBinding key/substitution/replay tests; TCP/TLS adjacent-entry failure and cleanup tests. Delete `internal/bridge/` and `internal/camouflage/`. |
| **M8 — Route** | Input: `internal/route`, `internal/routeplan`, route command/e2e/live tests. Owner: `route`. | R-076/ADR-0024 select the native Profile and R-078/ADR-0026 plus R-079/ADR-0027 select its closed wire. R-081 permits native preannouncement behavior only behind an explicit resource-admission port; M11 selects any real Node profile. W02 is C0 retirement of H3 bytes; before any v1 peer announcement, M8 must deliver canonical codecs/vectors and mixed-generation/downgrade/retirement tests. Route consumes opaque View/Duty/Resource/Entry ports. | Select/open/carry/recover/close, role-local knowledge, attachment capacity, cancellation, pressure, cleanup, and selected impairment tests. Delete `internal/routeplan/`, actor/evidence unions, stage workload orchestration, and obsolete route tracer command path. |
| **M9 — Publication and Connection** | Input: `internal/serviceconn`, Service endpoint composition, service command/e2e and stream workload. Owners: `service/publication`, `service/connection`. | R-076/ADR-0024 and R-083/ADR-0028 provide native Profile and endpoint wire; R-082 closes DA-10 only for the unobserved M9 H3 local/connection bytes as C0. R-084 selects D05 C1 immutable publication/floor migration and drain. A newly found external observer reopens its own support rule. | Publish/acquire/unpublish/supersede/drain/crash tests; stream/replay/cutover/terminal property and fuzz tests; one real Broker/Route process journey. Delete `internal/serviceconn/`, action unions, static authority bags, fixed batch/byte semantics, and unowned workload code. |
| **M10 — Isolation, Broker, Endpoint** | Input: `internal/applicationipc`, `internal/serviceendpoint`, app tracer commands, local sockets/plans. Owners: `application/isolation`, `application/broker`, `endpoint`. | R-085 closes DA-08 only for a generic/unqualified Broker; DA-10 remains mandatory. A01-A03 are C0 unless a named observer requires bounded C2. No generic adapter earns an isolation claim. | Principal/Grant/revocation/drain race tests; real platform IPC/process/escape/substitution tests only for a later selected profile; Endpoint readiness/signal/join/residue process tests. Delete `internal/applicationipc/`, `internal/serviceendpoint/`, old sockets/plans, and tracer commands after cutover. |

| **M11 — Node** | Input: `internal/node`, Node command/e2e. Owner: `node`. | Depends on M3/M4 target view/duty/resource ports and selected M8 Route facts. D02 reader is target-owned only. | Duty→quarantine→ready→pressure/drain/withdraw, listener/handler join, signal/restart, and selected platform process tests. The private probe is implementation, not a package; delete Snapshot translation glue. |
| **M12 — Custody** | Input: ADR-0021, D08, remaining foreign custody limitation fields. Owner: `custody`. | DA-08 and DA-09 are mandatory. D08 format/secret storage is C1/C3 only after accepted custody/platform design; no foreign secret writer. | Unlock/export/restore/reconcile/revocation/corruption/non-mutation tests; accepted platform storage/restore evidence. Delete foreign custody fields and limitation-string substitutes only after target behavior exists. |
| **M13 — command and operator consolidation** | Input: all retained product commands, plans/config/results, README/current operator routes. Owners: thin `cmd/ardents`, `cmd/ardents-node`, conditional bootstrap. | DA-10 governs every observed command/config/result. Each gets C0, C1, or C2 with a named observer and expiry; plans never become authority. | Command descriptor/process tests, migration/error/redaction checks, documentation route review. Delete `cmd/ardents-name`, `cmd/ardents-bridge`, `cmd/ardents-route`, `cmd/ardents-service`, `cmd/ardents-release`, `cmd/ardents-publish-app`, and `cmd/ardents-stream-app` unless an accepted bounded adapter remains. |

**M13 progress, 2026-08-23.** The sole repository-visible observer for
`cmd/ardents-service` was the unversioned root README statement. Under the
Product Owner's standing delegation it is a C0 command break: its exact
Endpoint route is now `ardents endpoint run <endpoint-plan.json>`, exercised by
the existing process readiness/cleanup and recovery journey. The plan remains
an explicitly temporary Endpoint-owned input, not a new general command
configuration contract. The former executable, its deterministic profile
entry, and its package-map row are deleted together.

The `ardents-bridge import` tracer is likewise C0: its only retained journey is
now `ardents entry import <entry-import-plan.json>`, with the authenticated
Network State, local-duty, replay, and classified-receipt behavior covered by
the moved command test. Its plan is an explicitly temporary Entry-owned input,
not a new general operator configuration contract. The former executable, its
deterministic profile entry, and its package-map row are deleted together.

The `ardents-name` tracer is C0 as a standalone command: its selected canonical
encoding, private resolution, and private control operations now sit under
`ardents name encode|resolve|control`. Their typed inputs and bounded receipts
remain unchanged; none becomes a general command configuration contract. The
former executable, its deterministic profile entry, and its package-map row
are deleted together.

The former `ardents-stream-app` and `ardents-publish-app` commands are C0
product tracers. Their only current caller is the Endpoint recovery process
test, so their implementations and unit suites now live as explicit
`tests/e2e/service/fixturecommand` fixtures in the e2e process profile, built
only by that test. This preserves
the Application-stream and separately granted publication-boundary evidence
without shipping test workloads as operator commands.

**M13 progress, 2026-08-23.** `internal/planfile` is C0 deleted. Its former
generic bounded read, JSON, exact-field, credential, and freshness helpers had
made historical plans an implicit cross-command Interface. `ardents`,
`ardents-node`, `endpoint`, and Node identity now each own only their bounded
input boundary; no maintained package imports a generic plan abstraction.
| **M14 — retirement and current truth** | Input: every remaining lab, verifier, e2e/live matrix, stage document, fixture, and this plan. Owner: Codex under Product Owner final disposition. | DA-11 chooses C4 provenance/reproduction or C0 deletion; Qualification requires an accepted claim and active profile, never a historical receipt. | Full profile/reader-route audit, claim/evidence identity check, package-map/import graph, artifact/residue scan. Delete obsolete `internal/lab/`, six lab commands, obsolete e2e/live suites, stage materials whose current facts are promoted, target/disposition ledgers, and this plan. |

**M10 progress, 2026-08-23.** `internal/applicationipc` and
`internal/serviceendpoint` now have one target owner at `internal/endpoint`;
the old package paths are deleted and both service and stream tracer callers
import that owner. The raw byte stream, single bounded terminal result, control
frame, plan decoding, socket cleanup, and Route Attachment acceptance moved as
one composition boundary. The sole Endpoint v1 Application handshake declares
its sideband result channel before opaque bytes; raw-tail and timing-based
fallback are C0 retired, with delayed, mismatch, and partial-handshake tests.
It does not create an isolation claim. The temporary `serviceconn` package,
its public `Do(Request)` action union, and its test callers are deleted;
Endpoint now exposes role-specific `Admit`, `Publish`, `Withdraw`, `Connect`,
and `Accept` operations. M10 is structurally complete under R-085: its generic
Broker remains explicitly `unqualified`, and no placeholder platform-isolation
package is created. The remaining deepening cutover has no new forwarding
Interface: Endpoint directly acquires a Publication lease, performs the
role-local TLS carrier and exact-Instance exchange, and supplies the resulting
opaque Attachment plus Application stream to `service/connection`. Broker
remains only the Local Grant/receipt owner; Publication retains private
Instance material and `service/connection` retains stream/recovery state. The
remaining connection result now retains only evidence with a current caller
(terminal outcome, stream/recovery facts, broker receipt, and process-bound
IPC/attachment counters); dead publication/session and sampled process-metric
projection is deleted. The result no longer carries transient TLS canaries,
Introduction bytes, or unobserved IPC high-water state.

**M10 progress, 2026-08-23.** `ardents` converts `os.Interrupt` into Endpoint
context cancellation. Endpoint then closes only its owned application, result,
and Route listeners, joins the blocked accept loop, and removes all three
socket paths before returning; it does not wait for the ordinary deadline or
leave a listener owned by a cancelled process. The direct lifecycle test covers
readiness, cancellation, join, and residue with a real local socket set.

**M11 progress, 2026-08-23.** `internal/node/probe` is deleted. Its
authenticated role-probe TLS, framing, replay, listener capacity, pressure,
and joined-drain implementation now remains private to the single `node`
Module that owns its full duty lifecycle. `cmd/ardents-node` configures
`node.ProbeConfig` only as part of `node.Config`; it has no separate probe
runtime to compose or import. The Node process and lifecycle behavior tests
continue to exercise the same public Node seam. State now supplies a narrow
opaque Node-duty view, so the command no longer copies a broad `Snapshot` into
Node facts. The resulting immutable facts are private Node implementation;
the only cross-Module contract is the consumer-owned `DutyView`.

## Completed wave records

### M0 — governance

**Complete, 2026-08-22.** The frozen source/toolchain identity is the accepted
[Stage 8 start record](stage-8-start-record.md):
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`, Go `go1.26.6 windows/amd64`,
and the recorded module, sum, and Makefile digests. M0 has no `DA-*`
prerequisite, no runtime writer mutation, and no new platform or Qualification
claim. Its affected observers are maintainers and test runners using current
policy, profile, package, and documentation routes.

S8.2 promoted those rules into `AGENTS.md`, the development guides,
`tests/profiles/`, the factual package map, and checked architecture/profile
gates. The entry record binds the successful `make check` and its
architecture/profile/document gates to the frozen source identity above;
later waves establish their own affected-gate evidence. The gate enforces
ownership facts rather than a package-export count or historical shape receipt;
its retained 500-line limit is the
explicit interim safety rule; cohesion is reviewed by responsibility and
invariant locality, not a soft line-count threshold. The [surface inventory](stage-8-current-system-surface-inventory.md),
[preservation ledger](stage-8-preservation-ledger.md), target architecture,
and this plan are the respective current inventory, disposition, ownership,
and retirement ledgers. Historical Stage 7 material remains only the
enumerated provenance/transitional material subject to M14's named outcome.

### M1 — Release trust

**Complete, 2026-08-23.** Starting from `7c1c991469d7fef0f47e6d2cf86596c347ad6b35`
on Go `go1.26.6 windows/amd64`, M1 used the closed DA-01 semantics of accepted
[R-063](../research/records/r-063-release-root-transaction-boundary.md).
`internal/release` now owns the D06 root, lease, archive, floors, and private
persistence seam behind `Open`, `Verifier.Evaluate`, and `Close`; the former
`internal/releasedecision` path, public `Store`, and public store-open entry
point are gone. Its accepted Decision yields an opaque Authorization retaining
a private verified snapshot. Update receives that authorization for both a new
activation and a rollback-pending predecessor check, not a caller-constructible
Decision; its package-private synthetic-decision seams are limited to
same-package behavior tests. The external authorization tests prove that a
caller-constructed accepted-looking Decision cannot invoke Update and that
Request exposes neither an initial nor rollback raw Decision; the command's
frozen V0 test exercises the real Release-to-Update handoff.

With the Product Owner's 2026-08-22 standing Stage 8 delegation, this is the
explicit delegated M13 disposition for the existing `cmd/ardents-release`:
it is retained only as a C2 target-owned tracer adapter. Its named bounded
observer is the frozen V0 offline command contract in
`TestApplyOfflineV0IsExact`, and it expires in M13. It makes no new installer,
activation, Custody, or compatibility claim; DA-09 and DA-10 still govern that
final product-command disposition. The package map and selected deterministic
profile now name `release`, and active test-data references move with the
package.

`internal/updatetransaction/transaction.go` remains one cohesive Apply
orchestrator (495 lines): the local invariant is that opaque authorization is
resolved before validation, owned-root inspection, lock acquisition, storage,
or Adapter action. Splitting that ordered lifecycle merely to reduce lines
would force checkpoint/choreography state across files; the authorization,
pre-admission refusal, interruption, recovery, and frozen V0 handoff tests
cover the retained responsibility. Targeted Release, Update, command, and
architecture tests passed before integration. The first final `make check`
attempt exposed a concurrent Route Unix-socket collision: global time-derived
test paths could bind the same address. `introduction_setup_test.go` now uses
a short unique `asi-*` directory per test (not `t.TempDir`, whose test-name
path exceeds the Windows AF_UNIX limit). The former 16-process × 100-run stress
reproducer passes after the change, and the subsequent complete `make check`
(format/architecture, mod, build, vet, staticcheck, vuln, unit, e2e, and race)
passes. The integrating commit records this checked profile.

### M2 — Update lifecycle

**Complete, 2026-08-23.** With DA-01
closed by R-063 and the bounded H3 technical-tracer scope accepted by
[R-064](../research/records/r-064-h3-update-tracer-scope.md), M2 moved the
complete transaction/recovery owner from
`internal/updatetransaction` to `internal/update`; the old package path and
all deterministic-profile references are gone. `update.Request` now accepts
the opaque Release authorization, candidate bytes, and declared runtime/schema
Adapters only. It derives the successor or exact idempotent-replay generation
from its owned root, chooses schema transition mode internally, and exposes no
caller-controlled generation, active-work count, schema-plan string, or raw
Release Decision. The external boundary test covers those omissions. The
former V0 command/result is not a C2 observer. [R-088](../research/records/r-088-update-test-root-format-cutover.md)
and ADR-0030 completed the C0 V2 fixture cutover, independent C4 verifier,
and command deletion without adding a root selector or lifecycle surface.

The `Apply` orchestrator remains cohesive at the interim 500-line maximum: it
orders authorization, root admission, storage reservation, journal checkpoints,
Adapter calls, activation, and terminal classification. Root admission and
exact transaction-directory inventory now live in the storage owner rather
than inflating the orchestrator. The full Update recovery matrix, the V0
characterization test, and `make quick-check` passed. This does not claim a
supported installer, native activation, or Custody lifecycle; any such work
reopens DA-09.

### M3 — authenticated Network State

**Complete, 2026-08-22 through `d7137a3`.** Namespace first received its
compatible durable/pointer/proof mechanics; State now owns the retained
current/pending/control writer, Epoch/View verification, commitments,
materialization, canonical cursor, and durable root. The obsolete
`network/store`, `network/epoch` (including subpackages), and
`network/framing` directories are deleted. `network/source` remains the
one-way acquisition port and no longer has a concrete State reverse dependency;
external fixtures use independently implemented test-only canonical builders.

### M4 — Duty and Resource

**Complete, 2026-08-22 through `66ed927`.** D02 completed through `394f3fc`
with the compatible `.ardents-local-roles-v1` durable root, unchanged generation
and watermark semantics, and no `internal/localroles` reader or writer.
R-062 accepts H1: `internal/resource` is the one shared coordinator for Linux
cgroup-v2/rlimit profiles; `!linux` adapters refuse readiness and default
observation fails protected and drained. The caller audit finds State, Node,
and Route using this one owner and no displaced production resource guard or
non-test resource-adapter override.

### M5 — Namespace

**Complete, 2026-08-23.** The R-061 prerequisite is complete: Namespace
owns its compatible current/pending root and proof mechanics without a Network
State persistence or commitment import. R-065 through R-073 now bound the
retained tracer's decision time, 127-record envelope, byte profiles, record
proof envelope, `submitted` semantics, durable exact-successor journal, typed
claim winner, and typed Epoch installation. `OpenControl` reconstructs only
verified current plus signed pending successors; `EpochInstallation` alone
selects a durable prefix or materializes an opaque `ClaimWinner` into an exact
signed successor before the existing threshold statement publishes current
state. A locally consumed R-045 root-claim proof now produces an opaque,
canonical 64-byte Epoch input; its threshold leaf binds the epoch-assigned
ordinal, commitment, and admission digest. The winner is restricted to the
same Network and Epoch installation that authenticated its close.

The focused Namespace, Resolution, and independent Stage-6 evidence suites
pass, as does the full `make check` profile (format, architecture, build, vet,
module, staticcheck, vuln, unit, e2e, and race) after this slice. M5 is
complete within its accepted local scope: R-074 records that no global-close
owner is selected in Stage 8. Namespace must therefore not claim root-claim
current behavior from its local input verifier; a future selected Network
Epoch protocol must commit the opaque inputs and issue the complete threshold
close.
The public `Record`,
`Op`, `ApplyLegacy`/`ApplyAtLegacy`, `VerifyLegacy`, `ResolveBindingLegacy`, raw `Store.CommitLegacy`, and historical
Stage-6 fixtures remain C4 compatibility surfaces. The caller audit finds no
runtime consumer of their caller-built field bags: production Resolution
consumes `Binding` through sealed Namespace views. M14's C4 disposition, not a
cosmetic M5 rename, removes the remaining historical verifier seam without
losing required provenance.

Under the Product Owner's standing Stage 8 delegation, M5 has also removed the
unobserved C0 `ardents-name validate-record` adapter. It was the sole
non-laboratory command caller of the raw Record codec; canonical Service Name
encoding and the still-required private resolution/control tracer journeys
remain. This narrows the runtime raw-Record surface without declaring the
remaining command, C4 evidence codec, or Namespace compatibility API retired.
The volatile detailed-control constructor is now explicitly
`NewEvidenceControl`/`ApplyEvidence`; its sole caller is the Stage 6 evidence
runner, while the durable Gateway path remains `OpenControl`/`Submit`.
Likewise, raw caller-built corpus publication is explicitly `Store.CommitLegacy`;
only fixtures and the Stage 6 evidence runner call it, while runtime publication
is `EpochInstallation.Commit`.
Raw caller-built lifecycle transitions are likewise explicitly
`ApplyLegacy`/`ApplyAtLegacy`; authorized Namespace paths call their private
transition core only after validating the owned proof and decision time.
`EpochInstallation` now passes its signer only a sealed `RecordSigningRequest`
containing the exact Record transcript and Authority public key, then verifies
the returned signature and builds the container itself; no callback receives a
caller-mutable lifecycle Record.

Private Resolution now reaches the durable Store, threshold policy, epoch
digest, and one-use admission gate only through Namespace-owned
`ResolutionGateway` and `ResolutionVerifier` views. The server view admits
only the exact `resolution` proof for its configured Gateway Node and returns
only a verified immutable Binding with its compact proof; the client view
re-verifies that proof without exposing a lifecycle Record. The temporary
`state.Snapshot`-to-verifier construction was retired in M6: the paired
opaque State view now reaches target `internal/naming/resolution` directly.

### M6 — private Resolution

**Complete, 2026-08-23.** Network State
owns `ResolutionView`, which admits only a fresh bounded selection window and
returns one immutable Epoch trust fact or one authenticated candidate valid
throughout that window. Target `internal/naming/resolution` accepts only that
view for runtime `Open` and `OpenControl`; `cmd/ardents-name` acquires it
through `CurrentResolution`. Explicit `OpenEvidence` adapters are retained
only for Stage-6 evidence and test fixtures. The old
`internal/nameresolution` directory and every Go import are deleted.
The retained replay/admission/failure suites cover fixed OHTTP envelopes,
finite replay capacity, response tampering, unavailable selected relays, and
role separation. `process_test.go` starts the Gateway and Relay as distinct
processes and completes an admitted private resolution through their OHTTP
boundary. No plaintext fallback or shared implementation view remains.

## Active wave review notes

### M7 — Entry lifecycle

`internal/entry/attempt.go` (265 lines) is one bounded Entry-attempt state
machine: it persists a State-derived candidate before exposure, permits at
most four ordered contacts over two slots, records cleanup before retry, and
terminally settles a draining replacement. Its local invariant is that a
replacement cannot become active while its predecessor can still have live
carrier work, and that incomplete cleanup never advances to another contact.
Splitting contact selection, terminal settlement, or cleanup handling merely
to shorten the file would distribute one durable state transition across
choreographing helpers; persistence, State validation, and guarded connection
behavior are already separate responsibilities. `TestAcquireRetriesOneCleanFailureAndRecordsTerminalCleanup`,
`TestAcquireFailsClosedWhenOpenerCannotProveCleanup`,
`TestReplacementDrainsUntilLiveAttemptSettles`,
`TestOpenTerminalizesInterruptedAttemptAndSettlesReplacement`, and
`TestAcquiredCarrierStopsAfterTimeConfidenceLoss` cover its normal, failure,
replacement, restart, and time-confidence paths.
`TestAcquirePassesStateCandidateToMutualTLSOpener` separately proves that the
opener receives the literal State endpoint and authenticates the State Ed25519
pin through a mutually authenticated TLS 1.3 handshake. The file remains below
the interim 500-line hard maximum.

`internal/entry/admission.go` is the separate Initiator-side durable replay
owner for R-079. It verifies the opaque Invite again and commits the finite
`(invite-id, attachment-id, client-key-digest)` tuple under one Entry lock;
the tuple is retained only through its expiry and survives reopening. Route's
native accept path supplies a narrow adapter, so it receives only the admitted
non-secret facts and cannot inspect an Entry root or User identity.
`TestAdmitterPersistsExactReplayTupleAcrossReopen`,
`TestAdmitEntryBindingRejectsSubstitutionAndConsumesOneTuple`, and
`TestAcceptEntryAttachmentVerifiesAndConsumesBeforeReturning` cover durable
replay refusal, TLS-key substitution, and the no-allocation-before-admission
order. The previous `net.Pipe` cleanup test now holds its peer through setup;
its 20-run reproduction passes rather than depending on close scheduling.

### M8 — Route

**In progress, 2026-08-23.** `route` now exposes the deep native lifecycle
`Open`, `Attach`, and `Close`, rather than a caller-visible complete Route
plan or stage actor. `Attach` reads one atomic authenticated View, creates an
unexposed selection and attachment identifier, reserves disjoint live
candidate identities/families, limits its deadline to current View/candidate/
duty facts, and obtains a caller-owned resource reservation before it invokes
Entry. Attachment close releases the Entry attempt, resource reservation, and
Route selection exactly once; Route close cancels and joins pending Attach
calls before it releases active attachments. `TestRouteAttachRefusesBeforeEntryAcquisition`,
`TestRouteAttachmentCapacityReleasesOnlyAfterClose`,
`TestRouteConcurrentAttachmentsUseDistinctCandidates`,
`TestRouteCloseCancelsPendingAttachmentAndReleasesReservation`, and
`TestRouteCloseReleasesEveryActiveAttachment` cover that interface.

The C0 H3 Route runtime, plan package, command path, live runners, and State
H3 Route-profile reader are retired. The R-078/R-079 v1 records have canonical
EntryBinding, Node LegBinding, and Sealed Introduction vectors; the latter now
includes a fixed HPKE known-answer and each visible/AAD/HPKE substitution
fails closed. M8 intentionally does not announce or run a peer-facing Node
profile: R-081 assigns measured Node admission, pressure, drain, and listener
integration to M11. The remaining M8 work is the selected native role-carriage
and impairment path that can be integrated only with that later Node work.

### M9 — Publication and Connection

**Complete, 2026-08-23.** Under R-084, `service/publication` is now the
single C1 owner of the publication root. It creates an exclusive owned root,
reads the old numeric floor only when that root is empty, writes only its own
monotonic floor, stages one immutable public record under its 16-hex
generation, and atomically publishes `current`. A restarted record is public
evidence only: without the volatile Instance signer it cannot be acquired.
`Publish`, `Unpublish`, supersession, and `Close` withdraw before draining,
then erase the signer and remove the withdrawn generation. The adapter no
longer writes or reads the former `GenerationStateFile`; its only temporary
M9 input is the one-time `LegacyGenerationFloor` read by Publication.

`internal/service/publication/publication.go` (408 lines) keeps one cohesive
publication lifecycle: admission of one already-validated generation,
immutable-record creation, withdrawal/drain, and the lease that confines the
volatile Instance signer. Splitting record commit from the state transition
would either expose a record before its floor/pointer is durable or force the
drain/erase invariant across coordinating objects. `root.go` (352 lines)
keeps the separate physical-root invariant: marker/lease identity, strict
inventory, migration floor, bounded reads, staging cleanup, and pointer/floor
replacement. `TestPublishAcquireDrainAndUnpublish`,
`TestPersistedPublicationIsNotLiveAfterRestartAndFloorSurvives`, and
`TestOpenRejectsSurplusOrTamperedPublicationState` cover publication,
draining, restart, floor, surplus, and tamper paths; the retained Service
Connection and endpoint process tests exercise the caller cutover.

The former `serviceconn` caller delegated publication persistence, generation,
acquisition and Instance signing to the opaque Publication lease, so it could
not copy or persist the private key. Its action union is now deleted;
Introduction acknowledgement tracer, H3 connection records, and obsolete
result/evidence bag are C0 retired. ADR-0028's native connection grammar,
vectors, and focused caller replaced them; no M9 compatibility adapter remains.

R-083's native wire now owns immutable Name-origin and finite recovery
contracts, plus all six records in `service/connection`: exact
ConnectionContext, Instance Challenge/Proof, deterministic exporter-bound
Continuity, Data, Acknowledgement, and Terminal. It also owns the role-ordered
Continuity exchange and record verification, plus the entire ordered/replayed
logical-stream, Attachment-replacement, and native-terminal lifecycle. The
temporary adapter supplies only the selected TLS carrier, retained local
admission, and product result projection. Each record is one exact
`ardents-service-connection-v1` envelope with the fixed native Profile,
closed kind, whole-body parser, and a 16 KiB Data bound. The temporary adapter
uses that codec for the live TLS proof, every fresh attachment, and stream
records, so the old `ASCF`, `ASAT`, `ASCH`, `ASPR`, associated H3 exporter
labels, and connection-binding tags are gone. The native codec's fixed context
vector and mutation tests, together with the retained Service Connection
recovery suite, cover profile/kind/length, proof/context, continuity MAC, and
offset/terminal refusal. This completes the R-083 grammar and stream-state
transfer. M10 subsequently removed the temporary `serviceconn` local
action/result adapter, so M9's Publication/Connection ownership transfer is
complete. `go test ./internal/service/publication ./internal/service/connection
./internal/endpoint ./tests/e2e/service -count=1` covers the direct owners and
their retained process journey.

**M12 progress, 2026-08-23.** `internal/custody` is now a real owner rather
than a Release/Update limitation string. Its first vertical slice admits the
accepted canonical outer envelope and fixed KDF/AEAD profile before derivation,
creates an independent encrypted Authority Vault record after double explicit
secret entry, and verifies an expected Authority binding after authenticated
unlock without returning root material. Wrong passwords and authenticated
ciphertext changes both return the same bounded unlock failure. The separate
`cmd/ardents-custody` caller exposes public canonical header facts and can
verify an active record through a no-echo interactive terminal secret boundary,
returning no root material. The same owner now exports a Bundle
only to a new owner-selected destination after a distinct double-entered
password, reopens it, and isolatedly test-restores its expected binding. This
is intentionally not M12 completion: confirmed replacement, restore
quarantine/reconciliation, revocation, foreign-format migration, and platform
qualification remain required before Release and Update can lose their custody
compatibility text. A restored Bundle now enters a separately encrypted
`authority-locked` quarantine record in a previously empty Vault and is
export-only; it cannot pass the active-record verification path. A recovered
Name Authority now activates only through a fresh opaque witness from an
already verified current Namespace materialization: the witness identifies one
active Authority record and must be strictly above the recovered generation and
revision. Custody advances every local watermark, durably writes the new active
encrypted record and exact floor before allowing a signature, and creates no
runtime Instance Key or Local Grant. Ambiguous, stale, equal, inactive, or
wrong-Authority witnesses leave the record locked and export-only. An active
Name record can sign one exact sealed Namespace transition request after
checking its local floor, the hash commitment of its Ed25519 public key, and
the request's exact predecessor generation/revision; the callback cannot return
an unrelated or stale proof, and root material never leaves custody.
The same owner now writes a canonical non-decreasing local floor only after a
new encrypted active record is published, reopened byte-for-byte, and admitted
again as a canonical envelope; equal/lower records fail before secret input and
a superseded record fails active verification. The missing piece is
the real authenticated Network/Namespace successor source, not a caller-supplied
claim of freshness.

Namespace transition signing has the same sealed-request shape as Record
signing: `TransitionSigningRequest` contains only Namespace's exact canonical
predecessor generation/revision, operation transcript, and expected Authority
key, and refuses a substituted signature or a request for an older active
record.

The durable Authority now has that inner caller: `Prepare` accepts only a
canonical unsigned existing-Name Intent, derives its exact transition and
successor Record while retaining the current chain, and invokes custody for the
two ordered sealed requests. Custody accepts only the active Authority key and
its exact predecessor generation/revision, then requires the successor Record
to advance precisely one revision before returning the opaque prepared
Submission. The static Intent digest still binds anonymous admission and
`Submit` remains the sole durable pending-journal writer. This does not yet
migrate the command/Gateway source: it still consumes the retained complete
signed wire, which is a compatibility path rather than another custody signing
interface.

Record and floor publication now flush the encrypted temporary file, use the
selected same-directory platform replacement primitive, flush the parent where
the platform permits it, and reopen before reporting success. This is an
implementation precondition, not a Windows/Ubuntu crash or power-loss
qualification result.

M12's D0 Update behavior test now creates a real encrypted custody Vault and
floor through the test fixture, commits its whole-root digest, then runs the
black-box Update transaction. The production Update Module never receives that
root or a custody input; the test fails if any Vault/floor bytes or paths
change.

Bundle export now permits replacement only after a separate explicit Owner
confirmation. It seals and test-restores a new encrypted temporary Bundle
first, copies the prior encrypted bytes to a same-directory backup while
leaving its destination in place, atomically publishes and reopens the new
Bundle through the same flushed platform primitive, then restores the previous
bytes on any ordinary publication or final test-restore failure. A failed
rollback leaves the encrypted backup for explicit repair rather than reporting
a false success. Crash/interruption recovery and platform durability
qualification remain separate M12 obligations.

The old `custody_notice` bytes now have an explicit C2 disposition. They are
the fixed H3 evidence limitation in the retained V0 release-result,
update-result, and update-manifest formats, not a Custody status channel.
`release.Decision` and `update.Result` project them as `EvidenceNotice`; only
the former R-064 `cmd/ardents-release` tracer rendered the old key. R-088
completed the M13 C0 V2 fixture cutover, C4 V0 verifier, and command deletion;
no second compatibility writer was introduced.
R-044 already revokes a former Name Authority at the authenticated Namespace
Record: the former key cannot sign its recovered successor. The active Vault
cannot discover that event safely because D08 has no Name-scoped replacement
commitment. R-086 accepts Namespace-level effective revocation for the selected
custody scope: no local Vault demotion is added without a new Name-scoped
predecessor-to-successor proof and format decision. Broker Grant revocation is
intentionally a separate local-admission transition.

## Dependency and retirement rules

M1 precedes M2. The accepted R-061 Namespace-first prerequisite occurs before
M3 source deletion but is not M5: it changes neither Namespace format nor its
target-module cutover. M3 precedes M4 and M11; M4 precedes M8/M11; M5 precedes M6;
M7 precedes M8; M8 and M9 precede M10; M10 and M11 precede M12; M13 follows
all retained runtime composition; M14 follows every selected retirement. A
change in this order requires a new owner/format/rollback argument in this
plan and Product Owner acceptance.

The following current paths may not be silently retained after their named
wave: `internal/releasedecision`, `internal/updatetransaction`,
`internal/localroles`, the seven Namespace source packages,
`internal/nameresolution`, the Network State source subpackages,
`internal/bridge`, `internal/camouflage`, `internal/routeplan`,
`internal/serviceconn`, `internal/applicationipc`, `internal/serviceendpoint`,
and `internal/node/probe`. Their test directories, fixtures, command callers,
exports, package-map rows, and documentation are part of the same deletion
outcome, not deferred cleanup.

## S8.4 acceptance

The Product Owner accepted this plan on 2026-08-22. Codex starts S8.5 with the
earliest wave whose DA and input gates are satisfied; it does not create
placeholder target packages for blocked waves.

**M14 progress, 2026-08-23.** R-089 closes DA-11 for the self-referential
S6E1 runner subset. The repository retained no immutable campaign input and
the evidence/verify commands now fail before a verdict, so their two commands,
two lab packages, and historical-reproduction profile entries are deleted as
C0. R-055 and the Stage 6 documents remain C4 decision provenance. This does
not dispose of the remaining Carrier, Named Site, or blocked-entry lab
families; each still needs its own M14 record-or-removal outcome.

**M14 progress, 2026-08-23.** R-090 completes the deferred whole-record audit
for `blocked-entry-verify-lab` and `blockedverify`. The tracked Stage 5 files
are only frozen preparation inputs, not an immutable H3 campaign bundle, and
the accepted native-v1 Route profile makes an H3 S9.6 result ineligible for a
future claim. The unbound reader, its command, package-map rows, and active
historical-profile entries are therefore C0 deleted. R-080, the Stage 5
records, and `tests/live/stage5-final` remain C4 provenance; Carrier and Named
Site are the remaining M14 laboratory families.

**M14 progress, 2026-08-23.** R-091 closes the remaining Carrier and Named
Site laboratory family. R-013/R-017 retain the historical result identities,
but their manual workflows rebuild current code and upload artifacts for only
30 days; they cannot reproduce the source-bound accepted runs and have no
native-v1 claim duty. Commands, shared lab Modules, workflows, supply inputs,
and the now-empty historical-reproduction profile are C0 deleted. The records
remain C4 provenance. M14's laboratory-corpus audit is complete.

**M14 progress, 2026-08-23.** `internal/streamworkload` had no maintained
product caller: the only consumers were the named Endpoint recovery process
test and its `stream-app` fixture command. The bounded opaque workload and
direct baseline therefore move to that sole evidence owner and the internal
package, package-map row, and deterministic-package entry are C0 deleted.

**M14 progress, 2026-08-23.** The four frozen S8.0 source-entry diagnostics
(`stage-8-current-system-report`, `stage-8-current-system-trace`,
`stage-8-current-system-surface-inventory`, and
`stage-8-current-test-portfolio`) are now explicitly marked historical. They
record the pre-cutover tree at `1cf7100`, not current ownership; the package map
and this plan remain current. They are C4 provenance until their retained facts
have canonical owners, after which the planned S8.6 deletion remains required.
