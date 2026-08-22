# Stage 7.2 task contracts — Update Transaction and custody preservation

Status: **the maintained S7.2 engineering slice was closed by the Product
Owner and Codex on 2026-08-22. S7.2-00a through S7.2-08 are implemented and
covered by the committed transaction tests; S7.2-09 is deliberately re-homed
to Stage 9 qualification.**
The MiniMax Interface/layout preflight found a missing authenticated-time
handoff and still-implicit private record formats. The jointly accepted S7.2-00a
amendment is in
[`stage-7-s7.2-transaction-format-proposal.md`](stage-7-s7.2-transaction-format-proposal.md).
Each task follows the independent implementation, worker-review, Codex-review,
and Owner-acceptance flow in
[the M3 collaboration workflow](m3-collaboration.md). Acceptance of this
contract authorizes only the named task briefs; it does not mark any S7.2
implementation or evidence cell accepted.

This document decomposes S7.2 into small vertical tracer slices. It is governed
by ADR-0015, the Stage 7 lifecycle specification, R-050, the Stage 7 development
plan, and the C/D evidence cells. If this document conflicts with those sources,
the higher-authority source wins and the affected task returns to `specified`.

## 1. Slice-wide contract

### Baseline and ownership

- The initial implementation baseline is S7.1 commit
  `7347a630fcb20241e8a0c09f1b33ca67b3691a49` plus the accepted contract-only
  commits that add this document and its vectors.
- Every later task starts from the last Owner-accepted S7.2 task commit, never
  from an unreviewed worker branch.
- Contract author: Product Owner + Codex.
- Implementer tag: `M3-autonomous` unless a task explicitly says otherwise.
- Independent worker-review tag: `M3-after-contract`, always in a fresh M3
  context and never permitted to fix its own findings.
- Windows installation, MSI repair, MSI removal, installer-owned registration,
  and destructive host lifecycle actions remain forbidden without a separate
  Product Owner command. S7.2 exercises only unprivileged filesystem/API logic,
  maintained commands, tests, and the already-authorized Docker surface.

### Fixed Module seam

S7.2 adds one `internal/updatetransaction` Module. Its Interface owns one whole
Installed transaction and restart recovery; it does not expose separate public
methods that let callers manufacture `staged`, `activated`, or `committed`
states. The Interface has at most:

- one `Apply` operation for a complete accepted transaction;
- one `Recover` operation for one existing transaction root;
- one bounded `Result` view;
- narrow work-control, self-test, schema, clock, and fault-free production
  dependencies required by those two operations.

`Apply` receives its work-control and self-test Adapters in `Request`; it never
constructs them. The work-control Interface consists exactly of
`StopNewWork(context.Context) error` and `Drain(context.Context) error`. The
self-test Interface is exactly
`Check(context.Context, CandidateIdentity) error`, where `CandidateIdentity`
contains only generation, target path, length, digest, platform, architecture,
environment, and network. It contains neither payload bytes nor Authority or
Vault material. `Recover` does not require either runtime Adapter.

Filesystem and durability Adapters are private to the Module. The Interface
must not expose `CommitStaged`, `SetCurrent`, raw journal writes, raw activation
writes, TUF metadata parsing, release signing, Vault access, package-manager
operations, or a generic filesystem abstraction.

`cmd/ardents-release` is the first non-test caller. Its existing
`offline-import` operation remains byte-for-byte compatible. The new explicit
operation is `apply-offline`; it reuses the existing release-input flags, adds
one required `-update-root`, and composes `releasedecision.Evaluate` with
`updatetransaction.Apply`. It must never stage a candidate whose decision is not
exactly `release-accepted`. The S7.2-01 caller uses a stopped-runtime Adapter:
successful exclusive ownership of the update root proves no competing Endpoint
transaction, while stop/drain remain explicit calls rather than being skipped.

### Frozen S7.2 H3 limits

Acceptance of this document freezes the following development limits. They are
not adjustable defaults and M3 may not replace them with more convenient
values.

| Concern | Frozen value |
|---|---|
| Candidate artifact | `1..67,108,864` bytes; length and SHA-256 must equal the accepted Release Decision |
| Payload retention | exactly one current payload, at most one verified rollback payload, and at most one staging payload |
| S7.2 payload shape | one artifact plus one authenticated immutable manifest; companions remain S7.3/S7.4 work |
| Update-owned root | fixed children only; unknown entries fail inspection or are preserved without mutation |
| Activation record | at most `16,384` bytes |
| Unix path | at most `512` bytes total and `64` bytes per component |
| Windows path | at most `240` UTF-16 code units and `64` bytes per component; no reparse component |
| Active transaction | at most one per state root under one exclusive state lock |
| Journal | at most `16` immutable state entries, each at most `4,096` bytes, plus one atomic current pointer |
| Restart recovery | one deterministic recovery pass; no retry loop or guessed commit |
| Activation retry | none inside a platform Adapter; busy is one explicit bounded result |
| Local drain ceiling | `15 s`, further reduced by every earlier authenticated or role deadline |
| Self-test ceiling | `15 s`, with network unavailability classified separately from payload failure |
| Copy-on-write fixture | at most `67,108,864` bytes and `4,096` entries in S7.2 |
| Cleanup deadline | `5 s` for staging, journal predecessor, process, handle, goroutine, and timer cleanup |

The byte ceiling applies only to the Update Transaction-owned subtree. It must
not be misapplied to the environment root, Vault, floors, Network State, or
another Module's owned state. Resource admission reserves the exact candidate,
rollback, copy-on-write, journal, and cleanup requirements before
`stop-new-work`; it does not pretend that three maximum payloads fit in one
`64 MiB` state-root budget.

### Common acceptance gates

Every implementation task must satisfy all of these in addition to its own
criteria:

1. The task changes at most `15` files and `1,500` production LOC, creates at
   most one new internal package and one new command, and ends in one commit.
2. Every Go file remains within the interim hard `500`-line ceiling. A
   production file above `250` lines records its cohesive responsibility,
   invariant-locality rationale, rejected obvious split, and behavior coverage
   for review; `250` is not an executable failure. No forbidden catch-all
   filename is introduced.
3. A new package arrives with `doc.go`, maintained Implementation, behavior
   tests, at least one non-test caller, exact imports, and factual
   `package-map.md` entries in the same change.
4. Tests cross the same Module Interface as the caller. Private fault seams may
   be used only to inject a declared interruption or platform result.
5. No test computes its expected classification using production validation,
   production encoding, production hashing of expected values, or the
   candidate's own summary.
6. A failed operation produces one lifecycle-spec outcome and leaves a complete
   inventory of current, rollback, staging, journal, schema, Vault/floor
   commitments, processes, handles, goroutines, and timers.
7. `make quick-check` and the task-targeted tests pass. Platform-specific code
   compiles for both `windows/amd64` and `linux/amd64`; runnable Windows and
   authorized Ubuntu Docker subsets pass where the task touches their Adapter.
8. The M3 handoff contains the required acceptance-criterion-to-test mapping
   and ends with the literal phrase `implemented, not accepted`.

### Frozen public vector V0

The byte-exact happy-path oracle is
[`c0-happy-path-v1.json`](testdata/s7.2/c0-happy-path-v1.json). It references:

- the already frozen S7.1 TUF set and `4,096`-byte candidate artifact in
  `internal/releasedecision/testdata/r049-public-vector-v1`;
- candidate SHA-256
  `a52b68413e0cd723547790c7ac161ece935d6459377442644b18031c3dc27d0a`;
- the byte-exact previous payload
  [`previous-payload-v1.txt`](testdata/s7.2/previous-payload-v1.txt), length
  `32`, SHA-256
  `8bdad9bde29bb6ee2a9d1d7005ec8ba2461b2bad3627372ee8458693c1fc08af`;
- an independently frozen `committed` result with the candidate current, the
  previous payload reserved for rollback, no staging residue, unchanged
  accepted release floors, and zero Authority mutation.

M3 may copy these bytes into a temporary owned state root during a test. It may
not rewrite the vector, regenerate expected values, replace it with a dynamic
fixture, or make `ReadActive` return the candidate before the `activated`
transition.

## 2. Task sequence

The first independent M3 contract review recommended splitting both S7.2-01
and the former S7.2-05. The self-test/network-unavailable work is split below as
S7.2-05a and the rollback/repair work as S7.2-05b. S7.2-01 remains one complete
C0 tracer: splitting it at `staged` would expose a temporary horizontal
Interface and violate the fixed Module seam. Its production/file budget and
`scope-blocked` stop rule are tightened instead.

### S7.2-00 — Accept the transaction contract and independent oracles

**Tag:** `Codex+Owner`; M3 must not implement this task.

**Blocked by:** S7.1 Owner acceptance.

**User story:** As the Product Owner, I can approve the complete S7.2 behavior,
limits, vectors, dependencies, and claim ceiling before a worker writes code.

**Allowed scope:** this document, its human-authored vector files, links from
the Stage 7 development plan, and the M3 collaboration workflow.

**Acceptance criteria:**

1. Every C0-C11 behavior and the S7.2 custody-preservation subset D0 maps to one
   implementation task and one independent evidence task.
2. The Module seam, runtime outcomes, numeric limits, Windows authorization
   boundary, and task dependency graph contain no unresolved implementation
   choice delegated to M3.
3. V0 hashes and lengths are recomputed with an independent host tool and match
   the saved values.
4. Each later task has explicit acceptance criteria, a frozen vector or fixed
   input table, allowed scope, exclusions, and tag.
5. Product Owner and Codex change this document's status to accepted together.

**Not in scope:** production Go code, generated packages, installer execution,
or declaring any S7.2 evidence cell passed.

### S7.2-00a — Freeze the transaction handoff and owned-state format

**Status:** contract and oracle amendment accepted by Product Owner + Codex on
2026-08-21; implemented in `a3b9ad02cfd765dbfa50907760c3dbdabd950fff`,
independently reviewed on both Standards and Spec axes, and accepted by the
Product Owner on 2026-08-21.

**Tag:** contract `Codex+Owner`; bounded implementation `M3-autonomous`;
review `Codex` followed by fresh `M3-after-contract` only if useful.

**Blocked by:** S7.2-00. **Unblocked:** S7.2-01.

**User story:** As the Update Transaction Module, I receive the authenticated
deadline facts already verified by Release Decision and one exact private
owned-state format, so neither my caller nor an implementation worker must
re-parse TUF, invent persistence, or guess recovery evidence.

**Contract source:**
[`stage-7-s7.2-transaction-format-proposal.md`](stage-7-s7.2-transaction-format-proposal.md).
The bounded worker instructions are accepted in
[`m3-s7.2-00a-brief.md`](m3-s7.2-00a-brief.md).

**Allowed implementation scope:** `internal/releasedecision/inputs.go`, the
minimum existing Decision-construction file, focused tests, and factual
documentation references. At most `5` changed files and `120` production LOC;
no new package, dependency, command output field, exported declaration, or
storage format is permitted.

**Frozen V0 deadline oracle:** reference time
`2030-01-02T03:04:05Z`, build no-new-work time
`2030-02-01T03:04:05Z`, build terminate time
`2030-07-01T03:04:05Z`, and no protocol transition deadline.

The S7.2-00a oracle amendment also separates the exact
`ardents-update-result-v1` command JSON under `expected.command_result` from
test-only floor, Authority, and Adapter evidence. It freezes `current_sha256`,
`staging_present`, safe notice, and custody notice instead of requiring a
command to emit the former `active_sha256`, nullable staging payload, or
test-probe counters.

**Acceptance criteria:**

1. `releasedecision.Decision` carries the four exact time facts named by the
   format proposal; the accepted V0 Decision equals the frozen oracle.
2. Values come only from the already decoded and verified target descriptor
   and fixed local reference time. No caller, Update Transaction code, notice
   parser, or second TUF parser derives them.
3. Accepted Decisions have non-zero ordered build bounds and a non-expired
   optional protocol transition deadline. Existing outcome and floor behavior
   is unchanged.
4. Existing `offline-import` JSON and the S7.1 public vector remain
   byte-compatible; the new fields are Go handoff facts, not new command
   output.
5. Focused tests cover V0, an accepted emergency deadline, and absence of
   partially trusted time facts on an identity-invalid rejection.
6. The amended V0 remains human-readable JSON, its previous/candidate hashes
   are independently recomputed, and command output is compared only with the
   exact `expected.command_result` object.
7. `make quick-check`, `make check`, race tests for `releasedecision`, and the
   existing Windows/Linux build gates pass.

**Not in scope:** Update Transaction production code, bootstrap creation,
changing signed metadata, adding a clock Interface, or accepting an S7.2
evidence cell.

### S7.2-01 — Complete compatible update tracer

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Status:** implemented in `840266e`, independently reviewed on both Standards
and Spec axes with no blocking findings, and not yet accepted by the Product
Owner.

**Blocked by:** accepted and implemented S7.2-00a.

**User story:** As an Endpoint Owner applying one accepted update while no work
is active, I get a complete immutable stage, rollback reserve, activation,
self-test, and commit without changing Authority or release floors.

**Evidence:** C0 happy path and D0 preservation tracer.

**Allowed scope:** one new `internal/updatetransaction` package;
`cmd/ardents-release`; their tests; the two factual `package-map.md` rows; no
other production package. At most `15` changed files and `10` production files
are permitted, including at most `8` production files in the Module. Production
LOC is at most `1,450` total, nominally at most `1,280` in the Module and `170`
in the caller. The eight-Module-file and total-LOC ceilings are conjunctive;
production files above the repository's `250`-line review threshold require
the common-gate evidence. If the complete tracer cannot fit, M3 stops with
`scope-blocked` without a commit and must not report `implemented`; it must not
expose an intermediate `staged` operation, omit a durable transition, or exceed
any cap.

The permitted Module production files are exactly:

| File | Sole responsibility |
|---|---|
| `doc.go` | Package contract and claim ceiling |
| `contract.go` | Module Interface, bounded request/result values, and frozen limits |
| `transaction.go` | Complete `Apply` orchestration and terminal `Recover` entry point |
| `journal.go` | Immutable state-entry representation, encoding, commitment chain, and inspection |
| `generation.go` | Canonical private manifest, current-selection, and stored-authorization record bodies and codecs |
| `store.go` | Shared owned-root, payload, staging, activation, locking, and cleanup Implementation |
| `durability_nonwindows.go` | Non-Windows atomic replacement and durability primitives only |
| `durability_windows.go` | Windows atomic replacement and durability primitives only |

The durability files must not duplicate journal, payload, inventory, locking,
or common filesystem orchestration. The private platform seam contains only the
operations that genuinely vary between the two maintained platforms.
`generation.go` is private Implementation detail and does not add a public
operation or widen the fixed Module Interface.

**Acceptance criteria:**

1. The command evaluates V0 through `releasedecision`; only its exact
   `release-accepted` Decision and exact artifact enter `Apply`.
2. The transaction durably traverses every normative happy-path state from
   `release-accepted` through `committed`. Journal entries are separate,
   immutable, and ordered. Every entry contains generation, selected release
   digest, Adapter result, monotonic observation, and deadline. Every entry
   after the first contains the SHA-256 commitment of the exact preceding entry
   bytes; the first entry commits the exact inspected predecessor state. No
   entry is overwritten by its successor.
3. Before activation, reads still select the previous payload. After commit,
   the candidate digest is current, the previous payload is the sole rollback,
   and staging is absent.
4. Candidate payload and manifest are byte-exact verified and immutable before
   `staged`; mutation through the Module Interface is impossible. External
   deletion or mutation is detected on inspection and fails closed; the Module
   does not claim to prevent an Owner or administrator from changing files.
5. `Request` carries the stopped-runtime work-control and self-test Adapters;
   `Apply` does not create or replace them. The stopped-runtime Adapter has zero
   active work. Its no-op
   `stop-new-work` and `drain` methods are nevertheless called exactly once so
   the complete tracer crosses the final Interface. It implements no deadline,
   active-work, or process-control logic; S7.2-04 adds that bounded behavior.
   The self-test receives the exact bounded candidate identity exactly once.
6. Release floors remain byte-identical to the accepted Decision. The test
   independently snapshots those floor bytes, precommits Vault and
   authority-watermark digests, and uses counter-based test probes to prove zero
   reads of secret material and zero Authority mutations. A notice string is
   not evidence for D0.
7. The command's bounded JSON equals the V0 expected terminal facts. It does not
   print filesystem internals, secrets, or a verifier verdict. Its schema is
   `ardents-update-result-v1` and includes only outcome, terminal state,
   transaction generation, current/rollback digests, staging presence, safe
   notice, and custody notice.
8. Re-running the exact committed request detects the matching committed
   release before generation allocation or staging, returns the same generation
   and terminal facts, and creates no fourth payload, second staging entry,
   journal entry, or additional live transaction. A conflicting request never
   reuses the committed generation.
9. Activation publishes the already verified staging directory without
   deleting the selected active payload and without recreating a manifest that
   already moved with staging. Atomic current-pointer replacement is the only
   operation that changes selection. The second identical `Apply` performs no
   activation replacement.
10. Every cleanup, close, sync, and atomic-publication error is observed and
    mapped to a bounded non-success result. Failure cleanup uses the fixed `5 s`
    ceiling; no `_ = os.Remove*`, errorless `Close`, or equivalent hidden error
    is permitted.
11. Interface-level tests reject an oversized candidate, a predecessor payload
    without its stored authorization, and an already occupied staging slot
    before `stop-new-work` or current-selection mutation. These are entry
    smoke-tests only; S7.2-03 still owns the complete resource/fault matrix and
    exact negative outcome taxonomy.
12. Tests include V0 happy path, V0 repeated-Apply idempotency, injected Adapter
    call counts, D0 probes, activation publication, cleanup failure, the three
    entry smoke-tests, and `apply-offline` JSON. Native Windows tests and race
    tests pass; Linux/amd64 cross-build and the authorized Ubuntu Docker package
    tests pass without a source edit between platforms.

**Frozen oracle:** V0. Expected values are read from the saved vector, not
computed through the Update Transaction Implementation.

**Not in scope:** real bounded-work drain or deadline behavior, live Contributor
duties, interruption injection, rollback on self-test failure, schema
migration, package installation, or evidence verdicts. Splitting staging into a
temporary public `CommitStaged`/`ReadActive` Interface is also forbidden.

#### S7.2-01 v2 remediation disposition

The first `M3-autonomous` attempt from contract commit `e3142ec` remains
`scope-blocked`, uncommitted, and not accepted. Its files are evidence only and
are not the v2 baseline. The v2 implementation is a fresh attempt from the
commit that accepts this amendment. It must not copy the duplicated platform
store shape, forbidden `types.go`, uninjectable runtime Adapter, mutable single
journal file, or unverified success notices from the abandoned worktree.

The later v3 locality review found that keeping three canonical generation
records plus storage orchestration in `store.go` would exceed the then-active
250-line hard cap even though the Module total remained within 1,280 LOC. The
Product Owner and Codex therefore accepted the eighth private Module file
`generation.go` on 2026-08-21. All other production, file-count, and public
Interface caps remain unchanged.

#### S7.2-01 final disposition

The Product Owner and Codex jointly accepted S7.2-01 on 2026-08-21. The
implementation is commit `4bc838601bb677db4bbe471fa495dbe09644d067`;
the separate gate-orchestration correction is
`8cad71a57ef999eedf9daa9446855c16d86ee0f2`. Native Windows tests, race,
Linux cross-build, the complete `make check`, and the authorized Ubuntu 26.04
clean-host package tests passed. S7.2-01 therefore unblocks S7.2-02; it does not
by itself close C1 or any later S7.2 evidence cell.

The accepted five-second cleanup ceiling is a continuation budget. No new
cleanup operation starts after it expires. An already-running non-cancellable
kernel operation may return later; the overrun is observed, no subsequent
operation starts, and the bounded result is `cleanup-incomplete`, never
`committed`.

### S7.2-02 — Exact restart recovery at every durable transition

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-01.

**User story:** As an Endpoint Owner restarting after interruption, I recover
one unambiguous previous or current state and never execute a guessed commit.

**Evidence:** C1.

**Allowed scope:** `internal/updatetransaction`, its tests, and fixture-only
edits to `cmd/ardents-release/main_test.go`; no caller production
change and no new package. The exact accepted controller brief is
`m3-s7.2-02-v2-brief.md`; it preserves the original accepted semantics in
`m3-s7.2-02-brief.md` and its Phase 0 evidence in
`m3-s7.2-02-phase0.md`.

**Fixed interruption table:** before and after each durable state entry
`release-accepted`, `artifact-verified`, `staged`, `rollback-reserved`,
`stop-new-work`, `draining`, `activated`, `self-testing`, and `committed`, plus
before/after atomic activation publication. Before durable publication selects
the predecessor; after acknowledged durability selects the complete successor;
an ambiguous, corrupt, missing-predecessor, reused-generation, or conflicting
record returns `transaction-invalid`.

**Accepted S7.2-02 amendment:** installer/portable bootstrap creates the direct,
regular, empty, single-link `.ardents-update-transaction-lock` as a permanent
stable root child. `Apply` and `Recover` each open it exactly once, acquire a
non-blocking exclusive OS lock, verify the locked handle still names the path,
and unlock/close without creating, repairing, replacing, retrying, or unlinking
it. Missing, changed, aliased, linked, or malformed lock evidence is
`transaction-invalid`; a live owner is `resource-denied/busy`.

The fixed recovery outcomes are:

| Observation | Outcome / state | Error | Result fields |
|---|---|---|---|
| coherent nonterminal prefix | `recovered` / exact last durable state, or `idle` | nil | interrupted generation; normalized current/rollback/staging; fixed `update interrupted`; custody from selected current manifest |
| coherent complete transaction | `committed/committed` | nil | exact committed V0-compatible Result |
| live owner | `resource-denied/busy` | non-nil | generation 0, zero digests, no staging/custody, fixed `update transaction busy` |
| cleanup failure/overrun | `cleanup-incomplete` / last coherent journal state | non-nil | verified transaction generation only; zero digests, no staging/custody, fixed `update cleanup incomplete` |
| corrupt, contradictory, aliased, or ambiguous evidence | `transaction-invalid/transaction-invalid` | non-nil | generation 0, zero digests, no staging/custody, fixed `update transaction invalid` |

The physical checkpoint oracle is frozen as follows:

| ID | Durable prefix and physical fact | Required recovery |
|---|---|---|
| R00 | no entry; predecessor current; empty transaction/journal directories | remove only those empty directories; `recovered/idle` |
| R01-R02 | through `release-accepted` or `artifact-verified`; predecessor current | preserve exact prefix and return that state |
| R03 | R02 plus complete unacknowledged staging candidate | remove that candidate and return normalized R02 |
| R04-R07 | through `staged`, `rollback-reserved`, `stop-new-work`, or `draining`; predecessor current and acknowledged staging | preserve and return exact state |
| R08 | R07; candidate published to generations; predecessor current | move exact candidate back to staging and return normalized R07 |
| R09 | R08 plus exact complete current temp | remove temp, move candidate back, return normalized R07 |
| R10-R11 | R07; complete successor current with exact predecessor rollback, before `activated` acknowledgement | verify predecessor commitment, restore predecessor current, move candidate back, return normalized R07 |
| R12-R13 | through `activated` or `self-testing`; coherent successor selection | preserve and return exact state |
| R14 | complete nine-entry `committed` chain and successor selection | return unchanged committed Result |

The independent table constructs literal checkpoint records and also exercises
real `Apply` with a private per-invocation crash sentinel. The sentinel bypasses
normal failure cleanup and Result construction, releases only the OS-lock
handle, and preserves checkpoint bytes. A second private per-invocation
operation seam may fail or delay only recovery remove/move/replace/sync calls;
public `Recover` always supplies native operations. Neither seam is exported,
package-global, context-carried, or allowed to replace validation or policy.

**Accepted S7.2-02 v2 caps and delivery:** the factual final amendment in
`m3-s7.2-02-gate-b-remediation.md` permits at most `26` changed files, `13`
changed production files, `1,850` net-new production LOC, `3,250` total Module
production LOC, and `500` lines per Go file. Production files above `250` lines
require the common-gate review evidence but are not rejected for size. The
responsibility map is frozen in `m3-s7.2-02-v2-brief.md`: recovery flow,
inventory, low-level bounded physical reads, journal validation, pure R00-R14
planning, cleanup execution, and
the two platform lock implementations remain separate. Existing durability and
`contract.go` files, caller production, dependencies, formats, and exported
declarations do not change. The implemented inventory is `26` changed files,
`12` changed production files, `1,691` net-new production lines, `2,961` total
Module production lines, and a maximum Go file size of `487` lines.

Delivery has two mandatory gates in the same isolated worktree. Gate A adds
only the independent test oracle, creates no commit, and must be observed red
by Codex for the specified missing behavior while R14 remains green. Gate B is
authorized only after that review; it preserves the Gate A oracle and creates
one implementation commit only after every accepted row, mutation, cap, and
verification gate is green.

**Acceptance criteria:**

1. One independent table-driven test injects every fixed interruption point and
   asserts exact active, rollback, staging, schema, and journal commitments.
2. `Recover` performs exactly one bounded pass and never infers commit from file
   presence, executable success, or the candidate summary.
3. A crash after atomic replacement but before the platform durability
   acknowledgement permits only complete predecessor or complete successor;
   the journal decides which is committed.
4. Corrupt or contradictory state returns `transaction-invalid`, performs no
   network work, and preserves repair/export reachability.
5. Recovery removes only the R00/R03/R08-R11 allowlist in deterministic plan
   order. It obeys the accepted `5 s` continuation budget, observes every
   failure/overrun, and starts no later step after expiry.
6. V0 record bytes, commitments, public Result, and command JSON still pass
   unchanged. Its physical bootstrap/committed root oracle now includes the
   permanent empty lock, which is not hashed into a record or Result.

**Not in scope:** disk-pressure admission, failed self-test rollback, live role
drain, or native power-loss qualification.

### S7.2-03 — Pre-stop staging and resource refusal

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-02.

**User story:** As an Endpoint Owner, invalid staging or insufficient reserve is
rejected before my current work or active generation changes.

**Evidence:** C2 and C3.

**Allowed scope:** `internal/updatetransaction` and its tests; caller output
mapping only if a required lifecycle outcome is not yet rendered.

**Fixed input table:** wrong candidate digest; short write; write/flush/rename
failure; insecure owner/mode/DACL; symlink/reparse/hardlink; unsupported or
cross-volume root; second staging payload; fourth retained payload; candidate
length `67,108,865`; insufficient exact byte/inode/entry reserve; unknown owned
root entry.

**Acceptance criteria:**

1. Digest, shape, path, ownership, and permission failures return
   `staging-failed` or `activation-unsupported` according to the lifecycle
   taxonomy; resource-envelope failures return `resource-denied`, never
   `release-conflict`.
2. Every table row occurs before `stop-new-work`; work-controller call counts
   remain zero and the old active bytes remain readable without change.
3. Failure leaves no partial generation, activation replacement, schema switch,
   or undeclared file. Declared incomplete staging is removed within `5 s`.
4. The Module accepts the S7.1 ceiling of `67,108,864` bytes and rejects the
   first excess byte before allocation or write outside the declared envelope.
5. At most one staging payload exists. There is no queue of four candidates.
6. External tamper is detected and reported; tests do not claim the Module can
   make Owner-controlled filesystem deletion physically impossible.

**Not in scope:** successful activation, rollback classification, or changing
the S7.1 Release Decision limits.

### S7.2-04 — Bounded work drain and atomic activation

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-03.

**User story:** As an Endpoint Owner updating during bounded work, new work
stops and existing work closes by the earliest deadline before activation.

**Evidence:** C4 and the drain/activation part of C0.

**Allowed scope:** `internal/updatetransaction`, its tests, and narrow existing
caller Interfaces; no Node/Route redesign.

**Fixed input table:** no active work; one work item completes before deadline;
one reaches local `15 s`; caller, Build Safety, or protocol-transition deadline
earlier than local; activation busy; unsupported storage; caller cancellation
during drain. Credential and role deadlines remain S7.2-07 inputs and do not
cross this Interface.

**Acceptance criteria:**

1. Every `StopNewWork` attempt is durably recorded before any `Drain`. A
   successful stop rejects later admission; a failed or expired stop never
   calls `Drain`.
2. Drain uses the earliest supplied deadline and never extends an authenticated
   or local deadline.
3. Deadline expiry returns `drain-expired`; the Application operation is not
   replayed and activation does not start.
4. Activation changes only the bounded activation record after the selected
   private platform Adapter verifies its supported filesystem/volume and
   ownership conditions.
5. The Adapter performs no hidden retry. Windows sharing denial is one bounded
   busy result with the old activation unchanged.
6. Concurrent readers observe only a complete old or complete new activation
   record, never missing or partial bytes.

**Not in scope:** Contributor rejoin, self-test failure rollback, or claiming
power-loss durability from process-exit tests.

### S7.2-05a — Bounded self-test and network-unavailable classification

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-04.

**User story:** As an Endpoint Owner, a bounded self-test distinguishes local
candidate failure from unavailable network checks without falsely marking
authenticated code bad.

**Evidence:** C8 plus the successful self-test branch retained from C0.

**Allowed scope:** `internal/updatetransaction`, its tests, and bounded command
result rendering; at most `8` changed files and `700` new production LOC. No
Release Decision parser or rollback implementation changes.

**Fixed outcome table:** complete offline success; complete online success;
network unavailable with all required offline checks passing; network
unavailable with a separately failing local predicate; self-test deadline; and
caller cancellation.

**Acceptance criteria:**

1. Self-test checks the exact candidate digest, environment/root, activation
   generation, schema readability, Vault non-mutation, release floors, and local
   IPC readiness within `15 s`.
2. Network unavailability alone never marks a payload bad and never mutates its
   retained safety status.
3. Offline-capable success may proceed to commit when every required offline
   predicate passes. A capability that normatively requires an online check
   returns its declared bounded unavailable result without inventing success.
4. A simultaneous local predicate failure remains a self-test failure even when
   the network is unavailable; the unavailable observation cannot hide it.
5. Deadline and cancellation do not extend the self-test window, replay work,
   lower floors, or start a hidden direct/network repair path.
6. The V0 complete offline-success result remains byte-exact unchanged.

**Fixed oracle:** the outcome table above, with candidate digest, local
predicate booleans, network observation, deadline observation, and expected
classification precommitted independently of the Implementation.

**Not in scope:** safe rollback, rollback refusal, `repair-required`, Authority
export, or online repair.

### S7.2-05b — Safe rollback, rollback refusal, and repair-required

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-05a.

**User story:** As an Endpoint Owner, a failed forward start executes only a
still-safe retained version or leaves networking stopped for repair/export.

**Evidence:** C5, C6, and C9.

**Allowed scope:** `internal/updatetransaction`, its tests, and bounded command
result rendering; at most `10` changed files and `900` new production LOC. No
Release Decision parser changes.

**Fixed outcome table:** payload digest failure; local IPC failure; rollback
valid; rollback revoked; rollback digest mismatch; rollback
schema-incompatible; rollback below a floor; rollback start failure; and forward
plus rollback start failure.

**Acceptance criteria:**

1. A non-network self-test failure durably enters `rollback-pending`; it cannot
   be reported as the C8 unavailable case.
2. Rollback executes only when every authenticated, digest, schema, revocation,
   protocol/build, and local-floor predicate independently passes.
3. A safe rollback returns `rolled-back`, reactivates code only, and leaves
   Authority, release, Network Epoch, Namespace, freshness, and generation
   commitments non-decreasing.
4. Any failed rollback predicate returns `rollback-refused` without executing
   retained bytes.
5. If neither direction starts safely, the terminal state is
   `repair-required`; normal networking remains stopped while bounded
   inspection, repair, Authority export, and diagnostics stay available.
6. Every rollback-pending and terminal transition extends the S7.2-02
   interruption table and exact recovery tests.
7. The S7.2-05a network-unavailable classifications remain unchanged and cannot
   be reinterpreted as authorization to roll back or repair online.

**Not in scope:** implementing Authority export, release metadata validation,
or hidden online repair.

### S7.2-06 — Copy-on-write schema commit with custody preservation

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-05b.

**User story:** As an Endpoint Owner, an update either commits a readable new
schema or leaves the prior schema and all custody/security state intact.

**Evidence:** C7 and D0.

**Allowed scope:** `internal/updatetransaction`, its tests, and one narrow schema
Adapter used by the existing non-test caller; no Authority Custody package.

**Fixed input table:** no-op schema; valid copy-on-write schema; write failure;
flush failure; validation failure; interruption before schema publication;
interruption after publication before transaction commit; forward rollback;
`67,108,864`-byte/`4,096`-entry boundary and first excess.

**Acceptance criteria:**

1. Schema work occurs only in a declared copy-on-write destination and is fully
   readable before activation/self-test can succeed.
2. The old schema remains current until transaction commit. A failed or
   interrupted migration leaves it readable and selected.
3. Commit atomically selects the new schema and records its predecessor in the
   transaction journal; recovery never guesses from directory presence.
4. Rollback before commit selects code compatible with the still-current old
   schema. Incompatible rollback returns `rollback-refused`.
5. Exact public commitments for Vault contents, authority signing watermarks,
   release floors, Network Epoch, Namespace, Grants, credentials, and Endpoint
   identity are identical before and after every fixed row.
6. No Runtime Interface is given Authority root material, Bundle secrets, or a
   method capable of lowering another Module's floor.

**Not in scope:** Bundle export/restore/reconciliation (D1-D4), general database
migration tooling, or unbounded user data.

### S7.2-07 — Contributor stop, drain, and fresh rejoin-or-withdraw

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-06.

**User story:** As a Contributor updating while role duties are active, no old
assignment survives and I rejoin only with fresh current authority.

**Evidence:** C11.

**Allowed scope:** `internal/updatetransaction`, its tests, and the minimum
narrow role-control Adapter at existing Contributor Interfaces. A required
change exceeding `15` files must be split and returned to contract review.

**Fixed role table:** zero roles; one role completes; two roles with different
lease deadlines; one partial drain; crash during role drain; stale assignment
after commit; fresh eligible assignment; fresh ineligible assignment;
withdrawal after failed rejoin.

**Acceptance criteria:**

1. New assignments stop for every active role before process replacement.
2. Each role drains by the earliest local, transition, credential, assignment,
   epoch, and Work Safety Lease deadline; one role cannot extend another.
3. Partial drain or crash cannot report commit while any old duty, process,
   handle, identity, or assignment survives.
4. Commit never revives old duties or clears their terminal/non-revival
   evidence.
5. Rejoin requires fresh current Release Safety, protocol/build eligibility,
   assignment, and role credentials. Otherwise the role remains stopped and
   may withdraw.
6. The fixed table produces one explicit per-role terminal inventory without
   exposing Node/Route topology in the public command result.

**Not in scope:** selecting a new Network Epoch, changing assignment policy, or
implementing a new Contributor role.

### S7.2-08 — Repeated pressure and complete bounded cleanup

**Tag:** implementation `M3-autonomous`; review `M3-after-contract`.

**Blocked by:** S7.2-02, S7.2-03, S7.2-04, S7.2-05a, S7.2-05b, S7.2-06,
and S7.2-07.

**User story:** As an Endpoint Owner, repeated updates, failures, rollbacks, and
restarts do not accumulate unbounded state or runtime resources.

**Evidence:** C10.

**Allowed scope:** `internal/updatetransaction`, its pressure tests, and factual
diagnostic counters already returned through its bounded Result; no new daemon
or telemetry system.

**Fixed pressure vector:** `100` deterministic episodes cycling success,
pre-stop failure, interruption/recovery, self-test rollback, rollback refusal,
repair-required inspection, and return to a fresh valid update. The vector
starts and ends with the V0 commitments.

**Acceptance criteria:**

1. Every episode respects one current, one rollback, one staging payload, one
   active transaction, at most `16` journal entries, and the fixed schema
   bounds.
2. After terminal cleanup there is no staging payload, incomplete schema,
   unowned temp, live child process, open owned handle, goroutine, timer, or
   undeclared log growth.
3. Cleanup completes within `5 s`; an incomplete cleanup returns
   `cleanup-incomplete` and cannot be reported as committed.
4. Disk bytes, entries, handles, goroutines, timers, and bounded result bytes do
   not trend upward across the final `20` equivalent episodes.
5. Pressure never lowers floors, mutates Vault commitments, admits new work
   after stop, retries activation internally, or creates a fourth payload.
6. The pressure test is deterministic, has no long sleep, and reports the first
   violating episode and resource parent.

**Not in scope:** native disk-full, inode-full, reboot, or hard-power evidence;
those remain environment-deferred until the specified clean-host surfaces.

### S7.2-09 — Independent C/D evidence verifier

**Tag:** implementation `M3-autonomous`; review `M3-after-contract` in a context
that has not implemented any candidate task.

**Disposition (Product Owner, 2026-08-22):** skipped as a Stage 7.2 delivery
gate. No producer, manifest format, verifier package, or verifier command is
authorized in this stage. The requirement is re-homed to the frozen-candidate
qualification work in Stage 9, specifically S9.5 in
`horizon-3-stage-9-brief.md`, where independent recomputation is used only for
claims that require it. This preserves ADR-0011's rule that ordinary Module
tests do not acquire a second verdict protocol.

The scope and acceptance criteria below are retained as historical provenance
for that later qualification design; they no longer block joint S7.2
acceptance.

**User story:** As the Product Owner, I receive a candidate-independent verdict
for the scheduled C0-C11 and D0 S7.2 evidence, not a trusted application log.

**Evidence:** complete scheduled S7.2 C0-C11 and D0 subset.

**Allowed scope:** one independent verifier package, one thin verifier command,
their frozen mutation corpus/tests, and their factual `package-map.md` rows. The
verifier must not import `internal/updatetransaction` or call its validators.

**Fixed mutation corpus:** one valid manifest plus mutations for wrong candidate
digest, missing transition, duplicate generation, broken predecessor, guessed
commit, post-stop staging failure, deadline extension, unsafe rollback, schema
switch before commit, floor decrease, Vault commitment change, surviving old
role, fourth payload, cleanup residue, and candidate-authored `pass`.

**Acceptance criteria:**

1. The verifier derives `pass|fail|invalid` solely from manifest-bound
   observations, hashes, clocks, transition order, expected runtime class, and
   complete resource inventories.
2. The valid V0/C0 evidence and each scheduled valid fixed table produce the
   independently expected verdict.
3. Every mutation is rejected for its semantic predicate even when candidate
   exit code, log, or summary says success.
4. Missing, contradictory, out-of-order, unbound, or contaminated evidence is
   `invalid`; observed contract violation is `fail`.
5. Expected runtime refusals such as `resource-denied`, `rollback-refused`, or
   `repair-required` can yield verifier `pass` only in their scheduled cells.
6. No secret values, raw topology, generated packages, caches, databases, or
   host mutation artifacts enter Git.
7. Codex performs separate Standards and Spec reviews; Product Owner alone
   changes the S7.2 slice disposition to accepted.

**Not in scope:** full Stage 7 acceptance, A/B/E/F/G cells, independent external
security audit, or native evidence that remains authorization-pending or
environment-deferred.

### Stage 7.2 closure boundary (Product Owner direction, 2026-08-22)

Stage 7.2 closes the maintained `internal/updatetransaction` engineering
slice: bounded on-disk update state, restart recovery, pre-stop admission,
work-control ordering, self-test/rollback terminals, bounded schema-selection
records, retained-generation rotation, and deterministic pressure coverage.
It is not an assertion that this repository already has a live schema migration
or Contributor runtime.

- **S7.2-06:** `no-op-v1` is the only current caller schema plan. The
  copy-on-write Adapter contract and recovery selection record are implemented
  and tested, but a real schema migration is deferred until Stage 8 chooses a
  maintained schema owner.
- **S7.2-07:** the transaction calls the complete bounded `WorkControl`
  lifecycle at the update boundary. No existing production Contributor role
  implementation is present to adapt, so per-role duty inventories and live
  rejoin behavior are deferred to Stage 8 runtime integration.
- **S7.2-08:** the deterministic pressure tests prove transaction-owned root
  bounds and terminal recovery behavior. Live-process, handle, goroutine,
  timer, and host-resource qualification remain Stage 9 evidence, not claims
  made by an in-process Module test.
- **S7.2-09:** independent verdict recomputation is deferred as stated above
  to Stage 9 S9.5.

These are deliberate scope boundaries, not silent acceptance of absent live
behavior. A Stage 8 implementation that introduces a schema or Contributor
runtime must return to the relevant boundary and add its concrete Adapter and
qualification tests before claiming those behaviors.

## 3. Dependency graph and acceptance rule

```text
S7.2-00 contract/oracles
  -> S7.2-01 complete C0/D0 tracer
     -> S7.2-02 restart recovery
        -> S7.2-03 pre-stop refusal
           -> S7.2-04 drain/activation
              -> S7.2-05a self-test/unavailable
                 -> S7.2-05b rollback/repair
                    -> S7.2-06 copy-on-write/D0
                       -> S7.2-07 Contributor lifecycle

S7.2-02 .. S7.2-04 + S7.2-05a + S7.2-05b + S7.2-06 .. S7.2-07
  -> S7.2-08 pressure/cleanup
     -> joint S7.2 acceptance

Stage 9 S9.5 performs any necessary independent qualification recomputation
after the Stage 8 freeze; it is not a Stage 7.2 code-delivery dependency.
```

No task closes an evidence cell merely because its candidate tests are green.
The cell is development-accepted only after implementation review, independent
worker review, Codex Standards + Spec review, and explicit Product Owner
acceptance. Independent verifier coverage belongs to Stage 9 only where the
frozen qualification claim requires it. Remediation findings become separate
narrow tasks against the same frozen contract; they never silently enlarge the
original scope.
