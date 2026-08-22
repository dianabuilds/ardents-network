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
| **M2 — Update lifecycle** | Input: `internal/updatetransaction`, Release interface, `cmd/ardents-release`. Owner: `update`. Objective: own stage, stop/drain, activation, self-test, journal, recovery, rollback, and cleanup. | DA-01 and DA-09 are mandatory. D07 is C1 only after every post-activation state has a forward repair or safe rollback. | Checkpoint/fault/recovery/idempotence/rollback/residue tests plus real supported filesystem/activator evidence. Delete `internal/updatetransaction/` and caller-authored transaction choreography. |
| **M3 — authenticated Network State** | Input: `internal/network/{epoch,epoch/assignment,epoch/merkle,framing,source,state,store}`, `internal/namestore`, `cmd/ardents`, e2e network-source. Owner: `network/state`; `network/source` is its acquisition port. | DA-02 and DA-05 are closed by R-059/R-060/R-061. **R-061 prerequisite:** first transfer only the currently used Namespace root/pointer and proof mechanics to its existing `namestore` owner, preserve its bytes and one writer, and remove both Network imports. D01 is C1 with one current/pending/control writer; W01 is C3 or C0 only after its observer decision. | Namespace durable/reopen/tamper/stale/partial-write and proof-mutation characterization plus a no-Network-import assertion precede the Network cutover. Then supply decision-time validity, source-wave, corruption, physical-root, restart, source cleanup, and process tests. Delete all listed old epoch/framing/store directories and concrete Source reverse dependency after caller cutover. |

**M3 execution record (2026-08-22):** complete through `d7137a3`. Namespace
first received its compatible durable/pointer/proof mechanics; State now owns
the retained current/pending/control writer, Epoch/View verification,
commitments, materialization, canonical cursor, and durable root. The obsolete
`network/store`, `network/epoch` (including subpackages), and
`network/framing` directories are deleted. `network/source` remains the
one-way acquisition port and no longer has a concrete State reverse dependency;
external fixtures use independently implemented test-only canonical builders.
| **M4 — Duty and Resource** | Input: `internal/localroles`, `internal/resource`, callers in State/Node/Route/Bridge. Owners: `network/duty`, then `resource`. Objective: one monotonic duty writer and one finite resource coordinator. | D02 is C1 with no generation/watermark reset. Resource platform scope must be explicit; unsupported platform fails closed. | Duty conflict/expiry/restart/physical-root tests; resource reservation/hysteresis/oversubscription/counter-reset and native adapter tests. Delete `internal/localroles/`, all displaced per-Module guards, and old duty readers. |

**M4 execution record (complete, 2026-08-22):** D02 completed through `394f3fc` with
the compatible `.ardents-local-roles-v1` durable root, unchanged generation
and watermark semantics, and no `internal/localroles` reader or writer.
R-062 accepts H1: `internal/resource` is the one shared coordinator for Linux
cgroup-v2/rlimit profiles; `!linux` adapters refuse readiness and default
observation fails protected and drained. The caller audit finds State, Node,
and Route using this one owner and no displaced production resource guard or
non-test resource-adapter override.
| **M5 — Namespace** | Input: `internal/{nameadmission,nameauthority,nameclaim,namelease,namerecovery,namestore,naming}`, including the R-061-owned local persistence/proof mechanics, name command, Stage 6 fixtures. Owner: `naming/namespace`. | DA-03, DA-04, and DA-07 remain mandatory; DA-05 is closed by R-060/R-061. D04/W03 uses C1/C3 only after transcript, scale, and proof authority selects it. | Authenticated create/renew/control/recovery/claim/materialization, durable reopen, tamper, and codec/property tests. Delete all seven source directories, their duplicate validators/field bags, stage fixtures, and command-only control wiring after cutover. |
| **M6 — private Resolution** | Input: `internal/nameresolution`, current name command and e2e role fixtures. Owner: `naming/resolution`. | DA-03, DA-04, and DA-07 are mandatory. C3 applies to retained OHTTP/wire behavior; no plaintext fallback or shared implementation view. | Opaque Namespace/State port tests, replay/admission/failure/scale tests, and selected real resolution process coverage. Delete `internal/nameresolution/` and old plan/command imports. |
| **M7 — Entry and Carrier** | Input: `internal/bridge`, `internal/camouflage`, bridge/route commands. Owners: `entry`, conditional `route/webtunnel`. | DA-06 is mandatory. D03 is C1 only with revision-safe replay/attempt recovery; W04 is C3 if retained, otherwise C0. | Invite/replay/replacement/durable-fault tests; selected child/front death, process-tree, port/path, and cleanup tests. Delete `internal/bridge/`; delete `internal/camouflage/` unless the selected Adapter replaces it. |
| **M8 — Route** | Input: `internal/route`, `internal/routeplan`, route command/e2e/live tests. Owner: `route`. | DA-06 is mandatory. W02 is C3 or C0 only with mixed-version/downgrade/retirement rule. Route consumes opaque View/Duty/Resource/Entry ports. | Select/open/carry/recover/close, role-local knowledge, attachment capacity, cancellation, pressure, cleanup, and selected impairment tests. Delete `internal/routeplan/`, actor/evidence unions, stage workload orchestration, and obsolete route tracer command path. |
| **M9 — Publication and Connection** | Input: `internal/serviceconn`, Service endpoint composition, service command/e2e and stream workload. Owners: `service/publication`, `service/connection`. | DA-06 and DA-10 are mandatory. D05 is C1 for generation/drain; A01-A03/W02 use C2/C3 only for named observer/protocol support. | Publish/acquire/unpublish/supersede/drain/crash tests; stream/replay/cutover/terminal property and fuzz tests; one real Broker/Route process journey. Delete `internal/serviceconn/`, action unions, static authority bags, fixed batch/byte semantics, and unowned workload code. |
| **M10 — Isolation, Broker, Endpoint** | Input: `internal/applicationipc`, `internal/serviceendpoint`, app tracer commands, local sockets/plans. Owners: `application/isolation`, `application/broker`, `endpoint`. | DA-08 and DA-10 are mandatory. A01-A03 are C0 unless a named observer requires bounded C2. No generic adapter earns an isolation claim. | Principal/Grant/revocation/drain race tests; real platform IPC/process/escape/substitution tests only for selected profile; Endpoint readiness/signal/join/residue process tests. Delete `internal/applicationipc/`, `internal/serviceendpoint/`, old sockets/plans, and tracer commands after cutover. |
| **M11 — Node** | Input: `internal/node`, `internal/node/probe`, Node command/e2e. Owner: `node`. | Depends on M3/M4 target view/duty/resource ports and selected M8 Route facts. D02 reader is target-owned only. | Duty→quarantine→ready→pressure/drain/withdraw, listener/handler join, signal/restart, and selected platform process tests. Delete `internal/node/probe/` and Snapshot translation glue. |
| **M12 — Custody** | Input: ADR-0021, D08, remaining foreign custody limitation fields. Owner: `custody`. | DA-08 and DA-09 are mandatory. D08 format/secret storage is C1/C3 only after accepted custody/platform design; no foreign secret writer. | Unlock/export/restore/reconcile/revocation/corruption/non-mutation tests; accepted platform storage/restore evidence. Delete foreign custody fields and limitation-string substitutes only after target behavior exists. |
| **M13 — command and operator consolidation** | Input: all retained product commands, plans/config/results, README/current operator routes. Owners: thin `cmd/ardents`, `cmd/ardents-node`, conditional bootstrap. | DA-10 governs every observed command/config/result. Each gets C0, C1, or C2 with a named observer and expiry; plans never become authority. | Command descriptor/process tests, migration/error/redaction checks, documentation route review. Delete `cmd/ardents-name`, `cmd/ardents-bridge`, `cmd/ardents-route`, `cmd/ardents-service`, `cmd/ardents-release`, `cmd/ardents-publish-app`, and `cmd/ardents-stream-app` unless an accepted bounded adapter remains. |
| **M14 — retirement and current truth** | Input: every remaining lab, verifier, e2e/live matrix, stage document, fixture, and this plan. Owner: Codex under Product Owner final disposition. | DA-11 chooses C4 provenance/reproduction or C0 deletion; Qualification requires an accepted claim and active profile, never a historical receipt. | Full profile/reader-route audit, claim/evidence identity check, package-map/import graph, artifact/residue scan. Delete obsolete `internal/lab/`, six lab commands, obsolete e2e/live suites, stage materials whose current facts are promoted, target/disposition ledgers, and this plan. |

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
