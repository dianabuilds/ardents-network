# S7.2 Update Transaction Interface and owned-state format

Status: **accepted by the Product Owner and Codex on 2026-08-21 after the
MiniMax M3 Interface/layout preflight, two bounded review passes, an amended V0
oracle, and a cap-feasibility spike.**

This proposal closes the private choices that remained implicit in the accepted
[S7.2 task contracts](stage-7-s7.2-task-contracts.md). It is subordinate to
accepted ADRs, the product contract, the threat model, and the
[Stage 7 lifecycle specification](stage-7-lifecycle-spec.md). If this proposal
conflicts with one of those sources, the higher authority wins and this
proposal must be corrected before implementation.

## 1. Decision and claim ceiling

`internal/updatetransaction` is one deep Module. Its external Interface owns a
whole update transaction and exposes exactly seven Go declarations:

```go
type WorkControl interface {
	StopNewWork(context.Context) error
	Drain(context.Context) error
}

type SelfTest interface {
	Check(context.Context, CandidateIdentity) error
}

type CandidateIdentity struct {
	Generation   uint64
	TargetPath   string
	Length       int64
	Digest       [32]byte
	Platform     string
	Architecture string
	Environment  string
	Network      string
}

type Request struct {
	UpdateRoot string
	Generation uint64
	ActiveWork uint64
	SchemaPlan string
	Decision releasedecision.Decision
	Artifact []byte
	Work WorkControl
	SelfTest SelfTest
}

type Result struct {
	Outcome string
	State string
	Generation uint64
	CurrentDigest [32]byte
	RollbackDigest [32]byte
	StagingPresent bool
	SafeNotice string
	CustodyNotice string
}

func Apply(context.Context, Request) (Result, error)
func Recover(context.Context, string) (Result, error)
```

The formatting above freezes field and method shape, not source layout. Go
formatting and comments remain Implementation work. `Apply` validates that the
accepted `Decision.Digest` is exactly 32 bytes and copies it into value-shaped
digest fields. No returned or Adapter-visible digest aliases caller or Module
storage.

All storage, codec, clock reading, journal, locking, cleanup, and platform
durability declarations are private Implementation. Tests and the command
cross the same external seam. Private fault injection may replace only the
specific failing durability operation under test and may not become another
external Interface.

`cmd/ardents-release apply-offline` constructs the stopped-runtime and
self-test Adapters in code and passes them in `Request`. It never reads or
constructs a private path, manifest, journal entry, current record, previous
artifact, or stored authorization.

For S7.2-01 the only accepted schema plan is the exact string `no-op-v1`.
Other strings remain representable so S7.2-06 can add a proved copy-on-write
plan without widening the Interface, but S7.2-01 rejects them before mutation.
`Result.SafeNotice` is the bounded Update Transaction outcome notice persisted
in the committed manifest; V0 uses `update committed`. `Result.CustodyNotice`
is the exact accepted `Decision.CustodyNotice` persisted in that manifest.
Idempotent Apply and Recover return those exact committed bytes.

## 2. Required upstream handoff correction

The current `releasedecision.Decision` omits three authenticated target times
that `releasedecision` already parses and evaluates plus its fixed local
reference time. Re-parsing TUF in the caller or Update Transaction would make
both Modules shallow and contradict the fixed seam. Before S7.2-01, a bounded
S7.2-00a correction must add these fields to the existing `Decision` value and
populate them from the already verified target descriptor and local reference
time:

```go
ReferenceTime               time.Time
BuildSafetyNoNewWorkAfter   time.Time
BuildSafetyTerminateAfter   time.Time
ProtocolTransitionDeadline time.Time
```

`ProtocolTransitionDeadline` is the authenticated emergency expiry when the
accepted protocol transition has one; it is zero otherwise. These are facts
already verified by `releasedecision`; adding them does not authorize Update
Transaction to parse release metadata or to mutate release floors.

The correction must prove:

1. every accepted Decision carries non-zero `ReferenceTime`,
   `BuildSafetyNoNewWorkAfter`, and `BuildSafetyTerminateAfter` with
   `ReferenceTime <= BuildSafetyNoNewWorkAfter < BuildSafetyTerminateAfter`;
2. a non-zero protocol deadline is not earlier than `ReferenceTime`;
3. a descriptor/identity-invalid Decision exposes zero time facts; a later
   lifecycle rejection may expose them only when the existing Decision already
   intentionally exposes that exact authenticated target identity;
4. the S7.1 public vector and existing command JSON remain byte-compatible;
5. no new exported declaration, metadata parser, clock Interface, or runtime
   dependency is added.

This gate was satisfied by the jointly accepted proposal and the reviewed
S7.2-00a implementation commit `a3b9ad02cfd765dbfa50907760c3dbdabd950fff`.

## 3. Owned root and bootstrap generation

The installer or portable bootstrap owns initial creation. S7.2-01 neither
creates an empty root nor exposes a bootstrap operation. It requires this exact
initialized shape:

```text
<updateRoot>/
  .ardents-update-transaction-v1
  current
  generations/
    0/
      artifact
      manifest.bin
  staging/
  transactions/
```

The marker contains the exact UTF-8 bytes
`ardents-update-transaction-v1\n`. `current`, `manifest.bin`, and later journal
entries use the private binary envelope in section 5. The generation-0
artifact is the exact previous artifact. Its manifest contains the exact
eight-field V0 stored authorization; stored authorization is part of the
authenticated manifest and is not a third payload companion.

The complete generation-0 manifest oracle is the exact
`initial.active_payload.manifest` object in
`testdata/s7.2/c0-happy-path-v1.json`, combined with the adjacent exact artifact
length, artifact digest, and stored-authorization object. Its
`bootstrap-preserved` strings are explicit installer/portable bootstrap facts,
not reconstructed Release Decision claims. Tests independently encode those
literal values; production S7.2-01 accepts no caller-supplied bootstrap
manifest and does not synthesize one.

These are the only stable root children. During one active operation the
following private names are additionally permitted:

- `.ardents-update-transaction-lock` — an exclusive create-new lock, absent
  when no operation owns the root;
- `.current.<16-lower-hex>.tmp` — one sibling current-record replacement temp;
- `staging/<generation>/` — at most one complete candidate directory;
- `transactions/<generation>/` — one transaction and its immutable journal.

There are no symlinks, junctions, reparse components, hard-link shortcuts, or
path aliases. Unknown entries, an idle leftover lock/temp, a second staging
generation, a reused generation, or contradictory initialized commitments fail
inspection before mutation or Adapter calls. External Owner deletion is not
prevented; it is detected by inspection and fails closed.

The lock and current temp are regular files created directly on the same
supported filesystem/volume as `current`; neither may resolve through a link or
reparse component. V0 tests, not S7.2-01 production code, copy the saved
`previous-payload-v1.txt` bytes and independently encoded manifest/current
fixtures into a temporary owned root. Production bootstrap source selection
remains installer/portable work outside S7.2-00a and S7.2-01.

Bootstrap is correct only when all of these independently agree:

- `current.current_generation == 0` and rollback is absent;
- the artifact length and SHA-256 equal generation-0 manifest fields;
- SHA-256 of exact `manifest.bin` bytes equals the current manifest commitment;
- the manifest authorization is the eight-field record from V0;
- the manifest platform, architecture, environment, and network equal that
  authorization;
- `staging/` and `transactions/` are empty.

The frozen V0 bootstrap artifact remains
`previous-payload-v1.txt`, length `32`, SHA-256
`8bdad9bde29bb6ee2a9d1d7005ec8ba2461b2bad3627372ee8458693c1fc08af`.

## 4. Stable and in-progress layout

One S7.2-01 transaction for generation 1 uses this exact layout:

```text
<updateRoot>/
  .ardents-update-transaction-v1
  .ardents-update-transaction-lock
  current
  generations/
    0/
      artifact
      manifest.bin
  staging/
    1/
      artifact
      manifest.bin
  transactions/
    1/
      journal/
        01-release-accepted.entry
        02-artifact-verified.entry
        03-staged.entry
        04-rollback-reserved.entry
        05-stop-new-work.entry
        06-draining.entry
        07-activated.entry
        08-self-testing.entry
        09-committed.entry
```

Activation publishes the already verified `staging/1` directory as
`generations/1`. It never recreates `manifest.bin` and never deletes
`generations/0`. The `current` record is the only replace-existing object.
Generation directories and journal entries are publish-new/no-overwrite.

After a successful S7.2-01 commit, the lock and current temp are absent,
`staging/` is empty, generation 1 is current, and generation 0 is the sole
rollback. Transaction 1 and all nine immutable entries remain as evidence.
There is no `result.json`, staging symlink, global journal, or `.current-prev`.

## 5. Canonical private binary envelope

All private records use one bounded big-endian envelope:

| Offset | Width | Field | Frozen value/rule |
|---:|---:|---|---|
| 0 | 8 | magic | ASCII `ARDUPD01` |
| 8 | 1 | kind | `1` manifest, `2` current, `3` predecessor inspection, `4` journal |
| 9 | 1 | version | `1` |
| 10 | 2 | flags | zero; any unknown bit rejects |
| 12 | 4 | body length | unsigned big-endian exact remaining byte count |
| 16 | variable | body | kind-specific fields below |

Integers are unsigned big-endian unless a field explicitly says signed.
Digests are exactly 32 raw bytes. Booleans are exactly one byte, `0` or `1`.
A string is a two-byte unsigned byte length followed by that many UTF-8 bytes;
it must be valid UTF-8, contain no NUL, and use the exact bytes supplied by the
owning trusted Module. No whitespace, case, or Unicode normalization is
performed. Optional UTC times are signed 64-bit Unix seconds, where zero means
absent. Trailing bytes, non-canonical booleans, unknown kinds/versions/flags,
duplicate semantic fields, or a decode followed by different canonical bytes
reject the record.

Bounds:

- any identity or plan string: at most `256` bytes;
- target path: at most `512` bytes and still subject to platform path limits;
- safe notice and custody notice: at most `512` bytes each;
- manifest: at most `16,384` bytes;
- current record: at most `16,384` bytes;
- predecessor inspection: at most `4,096` bytes;
- journal entry: at most `4,096` bytes.

The implementation uses this one codec vocabulary in `journal.go` and
`store.go`; platform durability files do not duplicate it.

## 6. Manifest v1 body

The manifest body contains these fields in this exact order:

1. generation `uint64`;
2. target path string;
3. artifact length `uint64`;
4. artifact SHA-256;
5. platform, architecture, environment, and network strings;
6. release identity string and release version `uint64`;
7. source revision, build-input commitment, build identity, dependency
   identity, SBOM identity, and attestation-policy strings;
8. qualification, build state, and protocol phase strings;
9. Build Safety and Protocol outcome strings;
10. eight sequential floor fields: signed 64-bit root version then root digest,
    timestamp version then timestamp digest, snapshot version then snapshot
    digest, and targets version then targets digest;
11. reference time, build no-new-work time, build terminate time, and optional
    protocol transition deadline as signed 64-bit Unix seconds;
12. schema-plan string;
13. safe notice and custody notice strings;
14. stored authorization fields from section 7.

All four floor versions must be positive and all four floor digests exactly 32
bytes for an accepted executable. Manifest fields must equal the typed accepted
Decision, Request generation/schema plan, and independently computed artifact
identity. There is no separately invented decision digest: SHA-256 of the exact
canonical manifest bytes is the manifest commitment.

## 7. Stored authorization projection

Stored authorization is embedded in the manifest in this exact order and has
no additional field:

1. classification string;
2. platform string;
3. architecture string;
4. environment string;
5. network string;
6. schema-compatible boolean;
7. revoked boolean;
8. above-local-floors boolean.

Generation 0 uses the exact logical V0 values. For a new S7.2-01 generation the
projection is deterministic:

- classification is exactly `string(Decision.Outcome)` and must be
  `release-accepted`;
- the four identity strings equal the Decision fields;
- schema-compatible is true only for the frozen `no-op-v1` plan in S7.2-01;
- revoked is false only when the overall, Build Safety, and Protocol Decisions
  are all `release-accepted`;
- above-local-floors is true only when every version/digest pair in
  `Decision.Floors` is complete and is the exact floor set already committed by
  `releasedecision`.

Generation-0 `revoked` is the stored V0 value and is not silently re-evaluated
by Update Transaction. A newly admitted generation uses the deterministic rule
above.

S7.2-01 rejects any other projection. S7.2-06 may add a new manifest version
for a proved copy-on-write schema plan; it may not reinterpret manifest v1.
There is no signature, payload digest, previous-payload digest, or signature
digest inside stored authorization. Artifact and authorization are bound by
their common immutable manifest.

## 8. Current selection v1 body

The current body contains, in order:

1. selected transaction generation `uint64`;
2. current generation `uint64`, artifact length `uint64`, artifact SHA-256,
   and manifest SHA-256;
3. rollback-present boolean;
4. when rollback is present: rollback generation `uint64`, artifact length
   `uint64`, artifact SHA-256, and manifest SHA-256.

Generation 0 has selected transaction generation `0` and no rollback.
Activation for V0 publishes transaction/current generation `1` and rollback
generation `0`. Because each manifest embeds stored authorization, binding the
exact manifest bytes also binds authorization without a third companion.

`current` does not claim that a terminal journal entry already exists. The
committed journal entry and coherent current record together prove the terminal
Result. Atomic replacement writes and durably closes one bounded sibling temp,
renames it over `current` on the same supported filesystem/volume, acknowledges
platform durability, and then removes no previous generation.

## 9. Predecessor inspection and journal v1

The predecessor-inspection body is reconstructed, not stored as a tenth entry.
It contains:

1. SHA-256 of the exact inspected `current` bytes;
2. current generation, artifact length, artifact SHA-256, and manifest
   SHA-256;
3. rollback-present and the rollback tuple when present;
4. SHA-256 of the exact current artifact bytes and exact manifest bytes after
   both have independently matched their recorded commitments.

The first journal entry's predecessor commitment is SHA-256 of this canonical
predecessor-inspection envelope. Each later entry's predecessor commitment is
SHA-256 of the exact preceding entry bytes.

Every journal body contains, in order:

1. state code `uint8`: `1` release-accepted, `2` artifact-verified, `3` staged,
   `4` rollback-reserved, `5` stop-new-work, `6` draining, `7` activated, `8`
   self-testing, `9` committed;
2. transaction generation `uint64`;
3. predecessor commitment `[32]byte`;
4. candidate artifact digest `[32]byte`;
5. candidate manifest commitment `[32]byte`; the deterministic manifest bytes
   are constructed in memory before the first journal entry and are written
   unchanged into staging;
6. Adapter-result code `uint8`: `0` not-called, `1` success, `2` busy, `3`
   unavailable, `4` failed;
7. observation sequence `uint8`, exactly equal to the state code in S7.2-01;
8. monotonic elapsed nanoseconds `uint64` since `Apply` captured its private
   monotonic start; values never decrease;
9. effective deadline as signed 64-bit UTC Unix seconds.

The deadline is finite. It is the earliest applicable value among the caller
context deadline, Build Safety terminate time, non-zero protocol transition
deadline, and any narrower state deadline already introduced by the accepted
task that owns that behavior. `stop-new-work` additionally cannot exceed the
authenticated no-new-work time. S7.2-04 and S7.2-05a add their frozen 15-second
local drain and self-test ceilings without changing the record schema.

`Apply` captures its private monotonic start and current context deadline at
function entry, before it acquires the root lock. Each entry deterministically
derives its deadline from those captured facts and the accepted Decision; it
does not resample or extend the caller deadline while writing. The first
predecessor inspection is built from the pre-activation `current` bytes and
generation selected at initial inspection, before any journal or staging
publication.

Elapsed time is obtained from Go's in-process monotonic clock by subtracting a
private start value. Only the elapsed integer is serialized. Recovery verifies
ordering evidence; it never treats a persisted elapsed value as a new process
clock or extends an absolute deadline. No public clock Adapter or package-level
mutable test clock is introduced.
Inspection additionally requires every journal elapsed value to be greater
than or equal to the preceding value; a decrease fails closed.

The Adapter-result code is the complete durable Adapter classification for this
record version. Dynamic error strings are not journal evidence. A bounded
public outcome and `SafeNotice` map from the code; secrets, raw OS errors, and
unbounded text are never persisted in the journal.

## 10. Publication and idempotency order

Under the exclusive private lock, S7.2-01 performs this fixed high-level order:

1. inspect the complete owned root, initialized current selection, previous
   artifact/manifest/authorization, budgets, and path rules;
2. before generation validation, allocation, staging, journal creation, or
   Adapter calls, detect an identical already committed request by matching the
   requested generation, independently hashed artifact, deterministic candidate
   manifest commitment, coherent `current`, and committed transaction entry;
   read the committed manifest/current records, return their reconstructed
   Result unchanged, and write no new journal entry;
3. publish immutable `release-accepted` and `artifact-verified` evidence, write
   and verify the complete candidate staging directory, then publish `staged`
   and `rollback-reserved`;
4. call `StopNewWork` and `Drain` exactly once and publish their corresponding
   entries; these calls precede activation, not all earlier durable evidence;
5. publish the staging directory as the new generation, atomically replace and
   durably acknowledge `current`, then publish `activated`;
6. call `SelfTest.Check` exactly once with the copied bounded identity, publish
   `self-testing`, then publish `committed`;
7. remove only declared unacknowledged temps/residue, observe every cleanup,
   close, sync, publication, and lock-release error, and leave the immutable
   transaction evidence intact.

Before the first journal entry, `Apply` requires
`len(Request.Artifact) == Decision.Length`, SHA-256 of the exact artifact bytes
equal to the 32-byte Decision digest, an exact `release-accepted` outcome, and
the frozen local identity/schema-plan predicates. A mismatch is bounded invalid
and changes no owned state. The deterministic candidate manifest is computed
from these Request/Decision facts before publication, so idempotency never
requires a caller-supplied manifest or private commitment.

For a state owned by an Adapter, the Adapter is called before its journal entry
is published. Success or bounded failure is then recorded in the immutable
entry. A crash after the call and before its acknowledgement remains an exact
S7.2-02 interruption point; S7.2-01 does not guess that the call succeeded.

An interruption between generation publication, current replacement, and
journal acknowledgement is not guessed by S7.2-01 `Recover`: it returns the
bounded invalid result without mutation or Adapter calls. S7.2-02 owns the
exact predecessor/successor recovery rule for each such interruption point.

## 11. Acceptance amendment

Product Owner and Codex jointly accepted the S7.2-00a amendment with all of the
following frozen:

1. the seven-declaration external Interface;
2. the upstream authenticated-time handoff correction;
   specifically `Decision.ReferenceTime`,
   `Decision.BuildSafetyNoNewWorkAfter`,
   `Decision.BuildSafetyTerminateAfter`, and
   `Decision.ProtocolTransitionDeadline`;
3. the two-file generation shape: artifact plus manifest containing stored
   authorization;
4. the exact owned-root names and absence of symlink/reparse shortcuts;
5. the canonical private envelope and record fields;
6. the nine-entry per-transaction journal and predecessor commitment rule;
7. the deadline/monotonic-observation representation;
8. the fixed bootstrap precondition and S7.2-02 interruption ownership.

Acceptance authorizes the small S7.2-00a production task and, after its separate
review and acceptance, a fresh S7.2-01 implementation task. It does not accept
either implementation or pass any C/D evidence cell.

## 12. S7.2-01 cap-planning guard

A throwaway responsibility-sizing spike compared the measured second M3
attempt (`1,550` Module LOC) with the frozen two-file generation format. It
found that the seven-file Module can fit the `1,280`-LOC total, but planning
`transaction.go` at `245` lines or `store.go` at `240` leaves less than five
percent per-file headroom and predictably recreates `scope-blocked`.

The replacement S7.2-01 brief therefore must require a pre-code function/
responsibility inventory and these planning targets:

| File | Planning target | Hard repository cap |
|---|---:|---:|
| `doc.go` | `25` | `250` |
| `contract.go` | `165` | `250` |
| `transaction.go` | `220` | `250` |
| `journal.go` | `205` | `250` |
| `store.go` | `215` | `250` |
| `durability_nonwindows.go` | `55` | `250` |
| `durability_windows.go` | `65` | `250` |
| Module total | `950` | effective `1,280` and per-file caps |
| `cmd/ardents-release` caller | `110` | `170` |

Planning targets are not permission to omit behavior, compress unreadable code,
or move a responsibility into another package. Crossing a planning target
triggers Codex review of locality and the existing private helper placement
before more code is added. Only the accepted hard caps determine
`scope-blocked`.
