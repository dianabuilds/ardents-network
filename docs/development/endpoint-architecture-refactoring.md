# Endpoint architecture refactoring design

Status: **accepted working plan for the ongoing decomposition**. This is a
target ownership map, not an accepted runtime contract or a delivery ledger.
GitHub Issues in the C0 milestone own delivery state; commit history records
implementation evidence. Original planning baseline: `dev` at `c849226f`
(2026-09-28). `dev` remains the integration point; assigned owners follow
[agent execution and handoff](agent-execution.md).

## Objective

Make the owner of an Endpoint behavior easy to locate while preserving the
accepted text-Service contract, exact ownership checks, cleanup order, and
existing wire and persisted identities. Package boundaries should remove
coupling; directory depth and filename prefixes are not goals by themselves.

## Target composition

Five owners divide the runtime. Each boundary states who creates the object,
who owns its state, who closes it, and what the caller may do with it:

| Owner | Responsibility | Created by | State owner | Closed by | Caller-visible operations |
| --- | --- | --- | --- | --- | --- |
| Endpoint | Local admission, component assembly, shared lifecycle | The composed local authority (`endpoint` value) | `dutyContext` and its private owners (jobs, publication, Source, permissions) | Context retirement in `duty_context_retirement.go` order | Participant runtime operations only; no Source, token-stock, worker, or publication internals |
| Node | Running the selected network role, process resources, shutdown | The process entry through the dispatcher | Per-role runtime state, not the whole `Config` | Role shutdown with joined child completion | Start one selected role; stop; observe completion |
| Route | Protected channels, network operations, transport mechanics | Endpoint and Service composition | Carrier, ARDP, capsule, terminal, replay, forwarding state | Channel retirement witnessed by its owner | Dial, accept, forward, and retire operations over protected channels |
| Service | Identity, publication, logical service connection | Endpoint assembly | Instance, durable Publication, Connection, reachability | Publication withdrawal and Instance retirement | Publish, connect, and verify identity operations |
| Application | Application protocol and local application interfaces | Endpoint through the selected Interface | Workload state of the selected surface | The Service stream lifecycle that admitted it | Surface operations of the local versioned interface |

Dependency direction: Endpoint consumes Service, Route, Application, and
qualification; Route must not import Endpoint; the extracted Endpoint
subpackages (`worker`, `tokens`, `publication`, `source`, `introduction`,
`service`) must not import the Endpoint root — each defines a consumer-side
seam that the root implements. Node composes roles and their resources
without exposing one role's configuration to another.

Endpoint package tree (current; implementation files of the extracted
mechanisms are Linux-tagged. `worker/doc.go` is untagged, so that package is
Windows-visible and listed in `tests/profiles/deterministic-packages.txt`.
`tokens`, `introduction`, and `service` have Linux-tagged `doc.go` files;
`publication` and `source` currently place their package comments in
implementation files and still need dedicated Linux-tagged `doc.go` files
to satisfy the repository package rule):

```
internal/endpoint/               composition root: admission, dutyContext
                                 coordination, job identity, launch permission,
                                 Grant binding, Route recovery orchestration,
                                 Service and Application assembly
internal/endpoint/worker/        installed worker mechanism ("mechanism, not
                                 authority"): inventory, systemd manager
                                 queries, unit property verification, artifact
                                 verification, instance observation,
                                 activation, cgroup pinning and verified stop,
                                 socket attachment credentials (stdlib-only
                                 leaf)
internal/endpoint/tokens/        pure token authority: Permission (holder
                                 request creation, approval acceptance,
                                 currentness/quota checks, exact retry
                                 matching, batch reservation), Batch, Stock,
                                 Operation (issuance slot), Owner over the
                                 shared duty mutex; seam tokens.Host
                                 implemented by root dutyTokenHost
internal/endpoint/publication/   pure refresh-scheduler mechanism:
                                 RefreshLifecycle/Refresh/RefreshRetirement,
                                 failure classifiers; root injects the
                                 rotation callback (no Host seam)
internal/endpoint/source/        Source prefix two-slot opening state machine:
                                 Lifecycle/Handle/Retirement, acquisition
                                 joins, failure classifiers; testsupport seams
                                 TerminateRoute/RouteDone/TransplantLive
internal/endpoint/introduction/  Introduction ...Locked mechanism: Admission,
                                 Dispatch/DeliveryKey, ExchangeSet,
                                 PairLifecycle+Registration, RecoveryOwner,
                                 Refusal; seams introduction.Host and
                                 RecoveryBinding implemented by root
internal/endpoint/service/       protected Service stream mechanism: checked
                                 directional workload contract, TLS
                                 attachment establishment with the exported
                                 continuity commitment, in-process Application
                                 half-close pair, resource ledger, bounded
                                 native Connection lifecycle; seam
                                 service.Binding (14 methods) implemented by
                                 root *serviceBinding
internal/endpoint/tokenjournal/  durable token-attempt journal
internal/endpoint/descriptorhistory/ per-Target verified publication floors
internal/endpoint/permissionfile/    canonical permission handover files
internal/endpoint/durableroot/       shared durable-root access/lease/sync
internal/endpoint/portable/, replacement/  portable-run and replacement leaves
internal/qualification/          per-invocation Run, Artifact, Attachment,
                                 Measurements, and the retained-run
                                 qualification scenario orchestration driven
                                 through Endpoint's authorized Session
                                 boundary
```

Worker boundary contract: Endpoint issues the launch permission — job
reservation, the process-wide activation gate, and Grant binding — and binds
it to the current operation. The worker package owns the observed process and
its verified termination: it receives plain values (inventory, role, accepted
socket) and returns handles (Artifact, Instance, Attachment, Cleanup); it
never receives `dutyContext`, a job identity, or a broker surface. The
lifetime that joins job retirement, attachment close, and cleanup stays in
Endpoint because it consumes Context authority callbacks. The existing check
and refusal order is preserved exactly. The `text_worker_installed` tag
surface (dedicated Ubuntu hosts) compiles in every quick-check through the
`installed-tag-compile-check` gate.

## Ownership that must remain distinct

| Owner | Responsibility | Target relationship |
| --- | --- | --- |
| `internal/endpoint` | Compose local authority, context cancellation, Route and Service owners, and Application Interfaces. | Remains the composition root. |
| `internal/network/source` | Direct-Origin Source transport for Network State. | Remains under Network; it is not an Endpoint Route prefix. |
| `internal/service/{instance,publication,connection,reachability,targetlink}` | Service identities, durable publication, logical connection, Descriptor, and Target Link contracts. | Remain Service modules with consumers beyond one text participant. |
| `internal/application/...` | Local versioned Application Interfaces and selected text-document workload. | Remains internal implementation of an externally usable local protocol. |
| `internal/route` | Carrier and protected Route mechanics. | Endpoint consumes Route; Route must not import Endpoint. |

The Endpoint resolution Source is implemented by Endpoint's exact Route
prefix acquisition in the root `source_*` files through
`internal/endpoint/source.Lifecycle`. It is not implemented by
`internal/network/source`. Names and package moves must preserve this
distinction. The separate [Route boundary map](route-refactoring-boundary.md)
records why the closed file cluster cannot be moved by filename alone.

## Qualification boundary

The installed qualification command owns plans and verdicts, and
`internal/application/streamqualification` owns the fixed stream workload.
`internal/qualification` owns the per-invocation `Run`, verified-worker
`Artifact`, `Attachment` bridge, shared `Measurements`, and the retained-run
scenario orchestration: hosting reservation and sampling cadence, Reader
setup pacing with its token-reserve ordering, Publisher set serving with
workload binding, and the replenishment boundary sequence. The exact
`qualification.Run` remains bound to its Job because JOIN, Connection limits,
token refill and cleanup all recheck that invocation.

The boundary is dependency inversion: qualification cannot import Endpoint, so
it defines the consumer-side `Session` (and its `PublisherWorker` subset),
which Endpoint implements in `stream_qualification_session_linux.go` as
bounded, authorized participant operations behind permission checks. Every
operation rechecks its own authority under the Context lock internally;
private Context, Job, permission, Source, and worker state never crosses —
only plain values, the retained `Run`, and the opaque `Preparation` and
`Publication` handles. The Context lock still protects atomic cross-owner
admission inside those operations; it was never split for the extraction.

Endpoint retains the participant composition: the thin
`RunStreamQualification` entry validates and composes the participant, binds
launch authority, and hands one `Session` to `qualification.RunScenario`;
preflight constructs or inspects the participant; the idle scenario observes
an ordinary User; the three retained token operations
(`ensureQualificationTokenReserve`, `ensureQualificationIssuerReserve`,
`presentQualifiedRefill`) and the stream-limit inspections stay as Context
methods because they consume private permission and prefix state. Moving the
remaining files by prefix would expose the participant's internals; further
runtime code crosses only when another bounded, authorized participant
operation is available to a non-test caller.

## Endpoint interior

`dutyContext` is the local admission, revocation, and join root: one admitted
local Reader/Publisher duty per instance. The implementation has private
owners rather than one undifferentiated state bag; after the extraction
series, `dutyContextState` composes: `publication publicationOwner`,
`introduction introductionOwner`, `descriptorHistory`, `responder
responderPrefixLifecycle`, `resolution resolutionLifecycle`, `source
source.Lifecycle`, `sourceSet *interiorSet`, `tokens tokens.Owner`,
plus job identity
(`job`/`lastJob`/`verifiedJob *jobIdentity`), lease/principal/surface
binding, and the shared `mu`:

| Responsibility | Current owner | Key coupling to remove or retain |
| --- | --- | --- |
| Source prefix | `source.Lifecycle` (extracted) + root `sourceSet` | Source owns exact handle identity, opening retirement, and the serialized operation reservation across prefix replacement. Root Context admission validates live authority and captures the revocable lease before Source reserves opening or issuance; the retained Interior Set remains at root for role selection. |
| Publisher prefixes | `introductionPrefixLifecycle`, `responderPrefixLifecycle` over shared `rolePrefixCore` (root) | Separate Route handles and opening lifetimes; borrowed Source is not closed by either. |
| Publication | `publicationOwner` (root) + `publication.RefreshLifecycle` (extracted scheduler) + `introduction.PairLifecycle`/`Registration` (extracted pair mechanism) | The pair owner retains the active registration opening and withdrawal flight through install or cancellation. Context still coordinates their stop/join order; Instance and Publication ownership spans Context and Endpoint locks. |
| Permission and issuance | `tokens.Permission`, `tokens.Operation`, `tokens.Owner` (extracted; fields exported, error strings byte-identical) | The permission owner holds holder request creation, signed approval acceptance, currentness and remaining-quota checks, exact retry matching and batch quota reservation, candidate-stock inspection, pending-batch cancellation, issued-token deposit, then burns and verifies the exact challenge under the Context admission lock via `tokens.Owner` over the shared duty `mu`. Orchestration (`issueTokens*`, `prepareIssuerStock`, `provisionPermission`) stays at root. Context performs the durable token-attempt mark and surviving-owner check before presentation. |
| Token attempt storage | `tokenjournal.Journal` | Own mutex, replay/time floors, and durable attempts; consumes the shared `durableroot` access, lease, and sync API. |
| Permission file handover | `permissionfile` | Own canonical owner-private request/response paths, exact retry, and request durability; Context retains currentness and offline approval authority. |
| Transit Grant acquisition | retired per [ADR-0092](../adr/0092-retire-generic-publisher-transit-chain.md) | The acquisition journals, transit credential acquisition, and transit client certificates were removed; no maintained composition selects a transit acquisition root. |
| Descriptor history | `descriptorhistory.History` | Own per-Target verified publication/revision floors, conflict memory, capacity and context-retirement erasure; Context checks live authority before acceptance and before using a retained proof. |
| Resolution and JOIN | Private `resolutionLifecycle` for the one Descriptor lookup/publication flight; Source and role owners retain their exact acquisitions | The flight owns cancellation, caller join, admitted Source release, and completion under the Context lock. Root rechecks Permission, Source, Publisher/Instance and Descriptor authority after network effects; retirement revokes before the ordered join. |
| Introduction opening admission | `introduction.Admission` (extracted) | Own context-local four-per-second opening reservations and accepted delivery replay retention under the Context lock through `introduction.Host`; shutdown clears both together. |
| Introduction delivery dispatch | `introduction.Dispatch`, `introduction.RecoveryOwner` (extracted) | Dispatch owns context-local waiter registration, one consumer gate, routing and waiter cleanup; recovery owns its exact generation, buffered capsule, deadline refusal and waiter/retirement handoff through `RecoveryBinding`. Slot transitions use the Context lock; Context checks live job and publication authority and joins claimed Route deliveries. |
| Introduction exchange reservations | `introduction.ExchangeSet` (extracted) | Own active exchange membership, retention and shutdown cancellation under the Context lock; Context checks job authority and joins terminal completion. |
| Job and worker | `jobIdentity`, `workerLifetime` (root), `internal/endpoint/worker` | Context retains the job reservation and the launch permission; the worker package owns the installed mechanism — manager queries, properties, instance observation, artifact, activation socket, cgroup pinning and verified stop — serving both text and stream-qualification inventories. The lifetime stays in Endpoint because it consumes Context authority callbacks for retirement and joined cleanup. |
| Service TLS | `internal/endpoint/service` (`tls.go`, `attachment_tls.go`) | Shared Instance authentication, handshake, and exporter handoff have one Linux-only implementation. The selected protected path fixes X25519MLKEM768/X25519 groups and the authenticated Route retirement witness; the earlier generic Service path was retired per ADR-0092. |
| Protected Service Connection | root `serviceBinding` (implements `service.Binding`, 14 methods) + `service.Stream` (extracted mechanism) | Context admits the exact Job and publication; the binding rechecks immutable authority and owns Route recovery orchestration (`service_route_recovery.go`). The Attachment owner authenticates the first transport and each replacement, owns physical transport retirement, returns the Publisher lease to stream cleanup, and closes replacement transport on failure. The stream owns the logical Connection, Application half-close, and terminal join. Binding authority, role orchestration, and job identity remain in the root by design; the subpackage cannot establish reachability or reinterpret a workload on its own. |

The Context coordinates cancellation and join of Publication's opening and
withdrawal flights, refresh, resolution, and Introduction exchanges. These
lifetimes do not become separate modules merely because they have distinct
files. The shutdown dependency order is defined by
`duty_context_retirement.go` — `stopDutyContextChildrenLocked` revokes every
child under `dutyContext.mu`, then `join()` runs outside the lock in the
fixed order: Source opening → Introduction opening → Responder opening →
registration opening → refresh → publication → Introduction prefix close →
Responder prefix close → Source prefix close → issuance → resolution →
withdrawal → exchanges → Job last (so its root reservation and first cleanup
error survive until every child is terminal). The maintained technical
contract stays in `docs/technical/endpoint-service-runtime.md`.

## Target code shape

1. Keep `internal/endpoint` as the local composition root. Its package-level
   interface should expose participant runtime operations, not the state of
   Source, token stock, worker, or publication internals.
2. Deepen existing private owners before extracting them. A transition should
   be performed by its owner; the Context coordinates admission, cancellation,
   and cross-owner ordering. This reduces direct field access without adding
   package imports or speculative interfaces.
3. Extract a package only when it has one cohesive resource or lifecycle, a
   small caller-facing contract, a non-test caller, and an import graph with no
   cycle. A new package gets `doc.go` (with `//go:build linux` when the whole
   package is Linux-only; an untagged `doc.go` makes it Windows-visible and
   requires the profile registry to list it, as for `worker`), behavior tests,
   and a package-map entry in the same change.
4. Keep the extracted `internal/endpoint/tokenjournal` as the durable
   attempt owner. Endpoint supplies only the selected token and its binding;
   the journal owns replay/time floors and persisted receipts through
   `durableroot`. Its integration tests read durable bytes independently.
5. Keep the text workload name on code that really depends on the selected
   text Application. Production identifiers in the Endpoint root carry
   domain names after the in-package rename series; the remaining legitimate
   `text` surface is the workload-bound Application protocol
   (`internal/application/textdocument`), error-message strings, the
   `text_worker_installed` build tag, the `text` CLI surface, and
   `plan.TextTokenRoot` — each a product decision, not a rename target.
   The workload-bound stream composition now lives in
   `internal/endpoint/service/stream.go`; the protected Service TLS policy
   and Route-retirement adapter live beside it in the same package.

## Completed slices

Each slice was locally verified and integrated into `dev`. GitHub Issues and
commit history contain the delivery evidence; this plan does not duplicate
hashes:

1. **Worker extraction**: installed mechanism in `internal/endpoint/worker`
   per the boundary contract above; Endpoint kept launch permission, the
   activation gate, the readiness exchange, and the lifetime/Grant binding.
2. **Qualification separation**: scenario orchestration in
   `internal/qualification` behind the consumer-side `Session` seam.
3. **dutyContext decomposition (sub-slices 3a–3e)**: publication, Source and
   network prefixes, Introduction acceptance and exchanges, permission and
   token acquisition, and operation/completion state each moved together with
   their operations into named private owners (`publicationOwner`,
   `introductionOwner`, `tokens.Owner`, unified `operationFlight`,
   `rolePrefixCore`).
4. **In-package rename series (A1–A11)**: production `text_*` identifiers
   renamed to domain names; `textContext` → `dutyContext`; files git-mv'd.
5. **Subpackage extraction series**: `tokens/` → `publication/` → `source/`
   → `introduction/` → `service/`, each behind a consumer-side seam
   (`tokens.Host`, injected rotation callback, `source` acquisition joins,
   `introduction.Host`+`RecoveryBinding`, `service.Binding`), error strings
   byte-identical, package-map rows added in the same commits.
6. **Installed-tag cure + compile gate**: the `text_worker_installed` test
   surface repaired after the worker extraction and cross-compiled in every
   quick-check/check via `installed-tag-compile-check` (target-specific
   `export GOOS/GOARCH` — inline recipe env prefixes are not portable across
   Windows make shells).
7. **Descriptor resolution flight (#345)**: private `resolutionLifecycle`
   owns the one lookup-or-publication flight, caller cancellation join,
   admitted Source release, and once-only completion under the Context lock.
   Root retains Permission, current Source, Publisher/Instance, and Descriptor
   authority checks and terminal cleanup failure.

## Remaining architecture review

Evaluate each further slice by state owner, interface, shutdown order, affected
tests, and integration gate — not by file counts or unmeasured percentages.
GitHub Issues own selection and delivery state. Structural acceptance of this
Endpoint decomposition is distinct from the #309/#311 bug fixes, protocol
choices, and installed systemd/cgroup qualification.

### L1 — dutyContext boundary review

- **State owner**: `dutyContextState` still holds publication, introduction
  (root aggregate over the extracted mechanisms), responder, resolution,
  Source lifecycle and retained Interior Set, tokens owner, and job identity
  under the shared `mu`. Root `*dutyContext` methods span several operation
  families; each proposed seam is evaluated by state, invariant, callers, and
  close owner, not by method or file counts.
- **Interface**: an extracted subpackage uses a consumer-side interface
  implemented by the root (`tokens.Host`, `introduction.Host`,
  `service.Binding` precedents). A private in-package owner uses direct
  methods and needs no interface. Either seam must have a non-test caller
  and hide an invariant or lifecycle, not merely reduce root method count.
- **Shutdown order**: pinned by `duty_context_retirement.go` `join()` (exact
  order quoted above); any dissolution preserves it step-for-step.
- **Affected tests**: root fixture family (`duty_context_test.go`
  `beginTestJob`/`admittedDutyContext`/`dutyContextEndpoint`), the composition
  helpers (`worker_composition_linux_test.go`), and every family consuming
  `liveCapsuleJob` (~20 files); behavior tests stay beside their production
  owner as they move.
- **Next seam**: none selected in this plan; a bounded issue must identify its
  exact state owner, lifetime invariant, callers, and retirement tests.
- **Integration gate**: per slice — commit-hook quick-check (includes the
  installed-tag compile gate), targeted Linux Docker battery in a claimed
  host-wide serialized window, then the integration owner's combined full
  `make check` before `dev` fast-forward.

### L2 — test-audit consolidation (mechanical)

- **State owner**: none (test-tree only). Mechanical consolidation within the
  accepted contract needs no separate PO checkpoint; behavior or coverage
  changes (e.g. tagging `heapdump_parser_test.go`) need explicit review.
- **Interface**: n/a. **Shutdown order**: n/a.
- **Affected tests**: Source failure-stage wrappers are checked in
  `internal/endpoint/source/failure_test.go`; the root Source-operation,
  nested role-failure, and exact-handle retirement tests stay with their
  Context orchestration. Further audited candidates include recovery fixtures,
  worker and Introduction helpers, publication ACK, admission-rate, and refresh
  classifier tests. Merge or move a test only with its actual behavior owner,
  preserving assertions, setup, and independently useful fixtures. The
  retained-setup `!race`/`race` pair changes together; package-visible helpers
  with external consumers survive every rename.

- **Integration gate**: same as L1 (commits wait for the shared window to be
  free so commit-hook quick-check does not contend with other owners'
  timing-sensitive gates).

### L3 — seam-surface review (Binding depth, fixture retirement)

- **State owner**: n/a (design review). Scope: `service.Binding` (14 methods)
  depth/locality — a one-implementation interface is not forbidden by itself,
  but each method must justify crossing the seam; and the remaining fixture-only
  exports (`service/testsupport.go` worker-ownership witness,
  `introduction/testsupport.go` 20 allowlist symbols, `source/testsupport.go` 3)
  with their recorded retirement condition: the test-audit slice reworks
  whitebox fixtures onto production seams, then the allowlist entries are removed.
- **Current audit**: the remaining Introduction and Source fixture exports and
  the Service worker-ownership witness have active behavior checks. Having no
  production caller alone does not justify removal. A replacement fixture must
  preserve each negative or retirement oracle through a real owner transition.
- **Integration gate**: review notes go to the owning GitHub issue; code changes
  (if any) ride bounded implementation slices.

### L4 — #309/#311 (separate bug fixes, not structural acceptance)

- Terminal-receipt recovery and Administration snapshot withdrawal belong to
  their confirmed Service Connection and Administration owners. Their
  reproduction, repair, checks, and integration receipts live in GitHub
  [#309](https://github.com/dianabuilds/ardents-network/issues/309) and
  [#311](https://github.com/dianabuilds/ardents-network/issues/311), not in
  this architecture plan. Neither defect's acceptance proves a structural
  Endpoint seam, and a structural slice cannot waive either defect gate.
- Any recurrence needs an owner-boundary reproduction and a deterministic
  regression. Do not add retry, skip, or quarantine; suppress EOF or integrity
  failures; or change deadlines before establishing the cause.

### L5 — protocol review (separate contract decisions)

- Any structural cleanup with unchanged wire needs one bounded owner and a
  concrete caller/acceptance seam. Semantic or compatibility changes in shared
  Route, Service Connection, or Application protocol packages require a
  decision against their current contracts and one assigned implementer;
  the closed v3 grammar is already selected, while public wire remains
  unselected. Existing C0 defect issues retain their own acceptance gates.
  Protected surface (PO product decisions): CLI, `plan.TextTokenRoot`, error
  strings, `text_worker_installed` tag.

## Tests and diagnostics

Tests beside an owner may use a temporary disk root or real loopback network
when persistence or transport is the invariant under test.
Endpoint integration tests inspect token-attempt receipts through an independent
persisted-file oracle; journal implementation tests retain access to private
state to verify poisoning, pruning, and crash boundaries. Shared fixtures
must not replace independent canonical-vector builders. A fixture move must
remove actual duplication or setup cost, rather than gather unrelated helpers
in one file.

`internal/diagnostics/timeline` normalizes time, owner, event class, and safe
reason from bounded Node, Source, and Endpoint runtime schemas. The command
adapter only supplies input and output.
Owner-specific event fields and privacy limits stay typed. Background delivery failures terminate the participant and return to its caller;
a private observation owner serializes output and retains the first such failure.

## Integration rule for Endpoint slices

Completed Endpoint slices integrate into `dev` through the assigned
integration owner. Rules:

- Integrate only completed, locally verified slices; never integrate half of
  a slice. Keep unfinished work in its own tree as temporary WIP commits —
  bare stashes are prohibited (the retained F-27/F-67 stash `bdf6490c` is a
  historical exception that must not be applied or deleted).
- Shared surfaces — Route, Credential, common commands, shared interfaces,
  repository-wide rules — have one assigned owner per bounded change; the
  other implementer states the required contract and proceeds with
  independent work.
- The assigned owner reads the current issue, affected current owners, and
  [agent execution and handoff](agent-execution.md) before a slice or shared
  interface change; progress goes to the issue and active conversation.
- One heavy host/Docker window at a time, host-wide; claims and releases are
  communicated to the integration owner. Per slice run targeted checks for the
  touched packages; the combined gate (Windows suite plus the docker linux
  battery, the exact candidate, and both selected Carriers where affected)
  runs before merging into `dev`, not twice in parallel per implementer.
