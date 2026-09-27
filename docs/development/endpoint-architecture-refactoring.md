# Endpoint architecture refactoring design

Status: **accepted working plan for the ongoing decomposition**. This is a
target ownership map, not an accepted runtime contract or a second delivery
ledger.
Baseline: `dev` at `51b38337` (2026-09-27). Coordination with the parallel
Node decomposition lives in `C:\Users\vitek\code\ardents-coordination`
(`assignments.md` plus one status file per implementer); `dev` is the single
integration point and the Node implementer performs merges into it.

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
| Endpoint | Local admission, component assembly, shared lifecycle | The composed local authority (`endpoint` value) | `textContext` and its private owners (jobs, publication, Source, permissions) | Context retirement in `text_context_retirement.go` order | Participant runtime operations only; no Source, token-stock, worker, or publication internals |
| Node | Running the selected network role, process resources, shutdown | The process entry through the dispatcher | Per-role runtime state, not the whole `Config` | Role shutdown with joined child completion | Start one selected role; stop; observe completion |
| Route | Protected channels, network operations, transport mechanics | Endpoint and Service composition | Carrier, ARDP, capsule, terminal, replay, forwarding state | Channel retirement witnessed by its owner | Dial, accept, forward, and retire operations over protected channels |
| Service | Identity, publication, logical service connection | Endpoint assembly | Instance, durable Publication, Connection, reachability | Publication withdrawal and Instance retirement | Publish, connect, and verify identity operations |
| Application | Application protocol and local application interfaces | Endpoint through the selected Interface | Workload state of the selected surface | The Service stream lifecycle that admitted it | Surface operations of the local versioned interface |

Dependency direction: Endpoint consumes Service, Route, Application, and
qualification; Route must not import Endpoint; the worker mechanism package
must not import Endpoint, broker, or qualification (see below). Node composes
roles and their resources without exposing one role's configuration to
another.

Endpoint target package tree:

```
internal/endpoint/             composition root: admission, Context coordination,
                               job identity, launch permission, Grant binding,
                               Service and Application assembly
internal/endpoint/worker/      installed worker mechanism: inventory, systemd
                               manager queries, unit property verification,
                               artifact verification, instance observation,
                               activation, cgroup pinning and verified stop,
                               socket attachment credentials, platform and
                               parent-service checks (stdlib-only leaf)
internal/endpoint/tokenjournal/ durable token-attempt journal (extracted)
internal/qualification/        per-invocation Run, Artifact, Attachment,
                               Measurements; later also the qualification
                               scenario orchestration (see slice order)
```

Worker boundary contract for the current slice: Endpoint issues the launch
permission — job reservation, the process-wide activation gate, and Grant
binding — and binds it to the current operation. The worker package owns the
observed process and its verified termination: it receives plain values
(inventory, role, accepted socket) and returns handles (Artifact, Instance,
Attachment, Cleanup); it never receives `textContext`, a job identity, or a
broker surface. The lifetime that joins job retirement, attachment close, and
cleanup stays in Endpoint because it consumes Context authority callbacks.
The existing check and refusal order is preserved exactly.

## Ownership that must remain distinct

| Owner | Responsibility | Target relationship |
| --- | --- | --- |
| `internal/endpoint` | Compose local authority, context cancellation, Route and Service owners, and Application Interfaces. | Remains the composition root. |
| `internal/network/source` | Direct-Origin Source transport for Network State. | Remains under Network; it is not an Endpoint Route prefix. |
| `internal/service/{instance,publication,connection,reachability,targetlink}` | Service identities, durable publication, logical connection, Descriptor, and Target Link contracts. | Remain Service modules with consumers beyond one text participant. |
| `internal/application/...` | Local versioned Application Interfaces and selected text-document workload. | Remains internal implementation of an externally usable local protocol. |
| `internal/route` | Carrier and protected Route mechanics. | Endpoint consumes Route; Route must not import Endpoint. |

The current `textResolutionSource` is implemented by Endpoint's exact Route
prefix acquisition in `text_source_lifecycle.go`. It is not implemented by
`internal/network/source`. Names and package moves must preserve this
distinction. The separate [Route boundary map](route-refactoring-boundary.md)
records why the closed file cluster cannot be moved by filename alone.

## Qualification boundary

The installed qualification command owns plans and verdicts, and
`internal/application/streamqualification` owns the fixed stream workload.
`internal/qualification` now owns the per-invocation `Run`, verified-worker
`Artifact`, `Attachment` bridge, and shared `Measurements`. The exact
`qualification.Run` remains bound to its Job because JOIN, Connection limits,
token refill and cleanup all recheck that invocation.

Endpoint still owns the authorized participant, worker and protected Service
operations being measured. Its remaining six `stream_qualification_*`
production files are not one self-contained package: runtime and preflight
construct or inspect the participant; connections and replenishment use private
Context, Job, permission, Source and worker state. Worker launch and
initialization also use the retained run; Endpoint worker paths create the
artifact from verified worker state and feed shared measurements. Moving the
remaining files by prefix would leave a reverse dependency or expose the
participant's internals. A worker-launch interface alone does not cover the
Connection and refill operations. Extract more runtime code only when a
bounded, authorized participant operation is available to a non-test caller.
The Context lock may still protect atomic cross-owner admission at that
boundary; splitting it into separate locks is not a prerequisite for package
extraction.

## Endpoint interior

`textContext` is the local admission, revocation, and join root. The current
implementation has private owners rather than one undifferentiated state bag:

| Responsibility | Current owner | Key coupling to remove or retain |
| --- | --- | --- |
| Source prefix | `textSourceLifecycle` | Exact handle identity, opening retirement, serialized operation gate, and retained Interior Set remain atomic with Context admission; the gate and set survive prefix replacement. |
| Publisher prefixes | `textIntroductionPrefixLifecycle`, `textResponderPrefixLifecycle` | Separate Route handles and opening lifetimes; borrowed Source is not closed by either. |
| Publication | `textPublicationPairLifecycle`, refresh lifecycle, Context coordinators | The Pair owner retains the active registration opening and withdrawal flight through install or cancellation. Context still coordinates their stop/join order; Instance and Publication ownership spans Context and Endpoint locks. |
| Permission and issuance | `textPermission`, `textIssuanceOperation` | `textPermission` owns holder request creation, signed approval acceptance, currentness and remaining-quota checks, exact retry matching and batch quota reservation, candidate-stock inspection, pending-batch cancellation, issued-token deposit, then burns and verifies the exact challenge under the Context admission lock. Context asks the permission owner whether approval, an exact request, or a pending batch exists instead of reading those fields. Context performs the durable token-attempt mark and surviving-owner check before presentation. Pending batch and exact Source reservation share that lock. |
| Token attempt storage | `tokenjournal.Journal` | Own mutex, replay/time floors, and durable attempts; consumes the shared `durableroot` access, lease, and sync API. |
| Permission file handover | `permissionfile` | Own canonical owner-private request/response paths, exact retry, and request durability; Context retains currentness and offline approval authority. |
| Transit Grant acquisition | retired per [ADR-0092](../adr/0092-retire-generic-publisher-transit-chain.md) | The acquisition journals, transit credential acquisition, and transit client certificates were removed; no maintained composition selects a transit acquisition root. |
| Descriptor history | `descriptorhistory.History` | Own per-Target verified publication/revision floors, conflict memory, capacity and context-retirement erasure; Context checks live authority before acceptance and before using a retained proof. |
| Resolution and JOIN | Context flights and narrow acquisitions | Exact current prefix must be checked again after network effects. |
| Introduction opening admission | `textIntroductionAdmission` | Own context-local four-per-second opening reservations and accepted delivery replay retention under the Context lock; shutdown clears both together. |
| Introduction delivery dispatch | `textIntroductionDispatch`, `textIntroductionRecoveryOwner` | Dispatch owns context-local waiter registration, one consumer gate, routing and waiter cleanup; recovery owns its exact generation, buffered capsule, deadline refusal and waiter/retirement handoff. Slot transitions use the Context lock; Context checks live job and publication authority and joins claimed Route deliveries. |
| Introduction exchange reservations | `textIntroductionExchangeSet` | Own active exchange membership, retention and shutdown cancellation under the Context lock; Context checks job authority and joins terminal completion. |
| Job and worker | `textJobIdentity`, `textWorkerLifetime`, `internal/endpoint/worker` | Context retains the job reservation and the launch permission; the worker package owns the installed mechanism — manager queries, properties, instance observation, artifact, activation socket, cgroup pinning and verified stop — serving both text and stream-qualification inventories. The lifetime stays in Endpoint because it consumes Context authority callbacks for retirement and joined cleanup. |
| Service TLS | `service_tls.go`, `protected_service_tls.go` | Shared Instance authentication, handshake, and exporter handoff have one Linux-only implementation. The selected protected path fixes X25519MLKEM768/X25519 groups and the authenticated Route retirement witness; the earlier generic Service path was retired per ADR-0092. |
| Protected Service Connection | `textServiceBinding`, `textServiceStream`, `protected_service_attachment.go` | Context admits the exact Job and publication; the binding rechecks immutable authority. The Attachment owner authenticates the first transport and each replacement, owns physical transport retirement, returns the Publisher lease to stream cleanup, and closes replacement transport on failure. The stream owns the logical Connection, Application half-close, and terminal join. These lifetimes still use Context and Publisher publication ownership, so a package split would expose those internals. |

The Context coordinates cancellation and join of Publication's opening and
withdrawal flights, refresh, resolution, and Introduction exchanges. These
lifetimes do not become separate modules merely because they have distinct
files. The shutdown dependency order is defined by
`text_context_retirement.go` and the maintained technical contract in
`docs/technical/endpoint-service-runtime.md`.

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
   cycle. A new package gets `doc.go`, behavior tests, and a package-map entry
   in the same change.
4. Keep the extracted `internal/endpoint/tokenjournal` as the durable
   attempt owner. Endpoint supplies only the selected token and its binding;
   the journal owns replay/time floors and persisted receipts through
   `durableroot`. Its integration tests read durable bytes independently.
5. Keep the text workload name on code that really depends on the selected
   text Application. Use responsibility names for mechanisms only after their
   ownership is clear. A bulk `text_` to `participant_` rename is not the
   architecture change. The protected Service TLS policy and Route-retirement
   adapter use `protected_service_tls.go`; the workload-bound stream composition
   remains in `text_service_stream.go`.

## Slice order

The decomposition proceeds in completed, locally verified slices; each slice
is one coherent commit series on the Endpoint work branch:

1. **Worker extraction** (current slice): move the installed mechanism into
   `internal/endpoint/worker` per the boundary contract above. Endpoint keeps
   the launch permission and the lifetime binding.
2. **Qualification separation**: move the qualification scenario
   orchestration — private context construction, job identity usage, token
   refill and qualification connection management — to the owner in
   `internal/qualification`. Endpoint retains the authorized participant
   operations behind permission checks. This runs **before** the Context
   decomposition so the private `textContext` surface shrinks first; no new
   `qualificationrun` package.
3. **Context decomposition**: move state together with its operations and its
   stop responsibility out of `textContext` into the corresponding owners, in
   sub-slices that are each separately committed: publication, Source and
   network prefixes, Introduction acceptance and exchanges, permission and
   token acquisition, current operation and completion. The Context remains
   the admission and shared-retirement coordinator. Mixed responsibilities
   are fixed inside their sub-slice — `text_source_state.go` currently
   computes eligible participants, opens the Entry, and closes the Entry with
   the token journal; participant computation and Entry lifecycle separate in
   the Source sub-slice. The `text_` prefix retires as substantive packages
   appear, not by bulk rename.

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

## Integration rule for this worktree

The Endpoint branch and the parallel Node branch (`codex/node-decomposition`
in the main checkout) both start from `dev` and integrate back into `dev`
through the Node implementer as single integrator. Rules:

- Integrate only completed, locally verified slices; never integrate half of
  a slice. Keep unfinished work in its own tree as temporary WIP commits —
  bare stashes are prohibited (the retained F-27/F-67 stash `bdf6490c` is a
  historical exception that must not be applied or deleted).
- Shared surfaces — Route, Credential, common commands, shared interfaces,
  repository-wide rules — have one assigned owner per bounded change; the
  other implementer states the required contract and proceeds with
  independent work.
- Both implementers read `ardents-coordination/assignments.md` and both
  status files before starting a slice, editing a shared interface, or
  integrating; each writes only its own status file.
- Per slice run the targeted checks for the touched packages; the combined
  gate (Windows suite plus the docker linux battery, the exact Ubuntu
  candidate, and both selected Carriers where affected) runs before merging
  into `dev`, not twice in parallel per implementer.
