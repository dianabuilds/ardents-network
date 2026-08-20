# Stage 6 development plan

Status: **S6.1 and S6.2 implementation were authorized by the Product Owner on
2026-08-20. The general Stage 6 gate remains closed while R-042, R-044, R-045
and evidence serialization are open. ADR-0013 is withdrawn; ADR-0014 is accepted.**

This plan maps accepted R-003/R-039 semantics and the Stage 6 brief into
sequential implementation slices without selecting undecided foundations in code.

## 0. Entry and S6.0 decision freeze

Before Stage 6 production implementation starts:

1. Stage 5 maintained development advanced. **Satisfied 2026-08-19; final
   R-037 qualification remains S9.6.**
2. R-041 freezes canonical textual/wire name behavior. **Satisfied.**
3. R-043 freezes the semantic persistence boundary. **Satisfied; the future
   interface and adapter still require conformance tests.**
4. R-046 supplies the complete field-level role matrix. **Satisfied.**
5. R-044 selects a maintained suite matching the distinct Recovery Authority
   trust model, and a replacement ADR is accepted. **Open; ADR-0013 withdrawn.**
6. R-045 measures and freezes Anonymous Cost/admission. **Open.**
7. R-042 proves eligible-set inclusion and deterministic ordering across all
   hostile cases. **Open.**
8. The manifest/evidence/verdict serialization and mutation vectors freeze only
   after items 4-7. **Open.**
9. Product Owner accepts the corrected documents and records coding start.
   **Open.**

Exception: the Product Owner explicitly authorized Slice A (S6.1) on 2026-08-20
so the decided R-039/R-041 lifecycle and encoding core could replace earlier
feasibility scaffolding. On 2026-08-20 the Product Owner separately authorized
Slice B after accepting R-046, R-047, and ADR-0014. Neither exception satisfies
item 9 or permits code to choose a mechanism still open in S6.0.

S6.0 produces decisions and schemas, not a disguised implementation spike. A
decision that selects storage, consensus, wire protocol, cryptography, or another
lock-in follows the repository research and ADR rules.

## 1. Sequential implementation slices

### Slice A - S6.1 encoding and lifecycle core

Owner: Codex

- Implement one canonical Service Name and Service Link parser from R-041.
- Represent Name Lease, Name Generation, Name Record, revision, parent binding,
  and the three independent state dimensions.
- Implement deterministic encoding and strict rejection of non-canonical forms.
- Implement Active/Grace/Released, renewal, reclaim, and parent/child invariants.
- Keep Conflict/Fork orthogonal to Lease state; neither can release a Lease.

Definition of Done:

- every legal and illegal parser/state edge has a maintained behavior test;
- old generation, revision, parent-generation, and Record-schema replay fails;
- signature replay remains blocked on R-044 and cache rollback belongs to the
  R-043 persistence/resolver adapter, not this in-memory lifecycle core;
- no command or persistence behavior is added outside the accepted profile.

Authorization: **implementation open since 2026-08-20; independent Stage 6
evidence remains a later completion gate.**

### Slice B - S6.2 role separation

Owner: Codex

- Implement explicit role-local request and observation types from the accepted field
  matrix; do not pass a superset object and hide fields by convention.
- Separate endpoint-adjacent, naming/Rendezvous, local resolver,
  authority-operation, and bounded observer responsibilities.
- Enforce query hiding, Rendezvous Role Domain and known-family exclusions, and
  Isolation Context unlinkability.
- Freeze control-operation field separation now, but implement only the complete
  resolution vertical slice; later owning slices add concrete proof codecs after
  R-042/R-044/R-045 decide them.
- Return classified failure when the private path or role evidence is absent,
  stale, conflicting, forked, or invalid.

Definition of Done:

- adversarial tests prove that no ordinary role obtains the forbidden combined
  view and no stable query identifier crosses Isolation Contexts;
- DNS, HTTP, search, local alias, alternate Namespace, and less-private fallback
  are absent and represented as explicit failure.

Authorization: **open since 2026-08-20 after Product Owner acceptance of R-046
O1, R-047 O1, and ADR-0014. R-044 threshold recovery remains a separate S6.4
gate.**

### Slice C - S6.3 target continuity

Owner: Codex

- Bind resolution to exact Name generation/revision and Service Target evidence.
- Preserve naming state during same-Target Service Instance migration.
- Require a fresh monotonic Name Record for replacement Target publication.
- Keep direct Target connections pinned.
- Enforce the finite close/no-new-recovery behavior of name-origin connections
  after Recovery Pending, Release, or different-Target rebind.

Definition of Done:

- maintained tests cover same-Target migration, replacement Target, stale cache,
  old generation, name-origin stream closure, and direct-Target pinning;
- naming changes never mutate Service Target Authority or retarget a live stream.

### Slice D - S6.4 authority, delegation, and recovery

Owner: Codex

- Implement claim, renew, authenticated release, rotation, and transfer gates.
- Implement subordinate delegation as a separate bounded operation with parent
  generation/lifecycle enforcement.
- Implement delayed Recovery Policy add/replace/disable transitions while the
  preceding policy remains effective.
- Implement policy-authorized Recovery Pending, precommitted contest/cancellation,
  bounded outcome, successor installation, and fresh-record requirement.
- Reject stale predecessor, admin, registrar, manual, rollback, and implicit
  recovery paths.

Definition of Done:

- old authority and old policy material cannot regain power;
- parent Release disables every descendant without reviving it after reclaim;
- no authority can silently erase recovery or extend pending state indefinitely.

### Slice E - S6.5 concurrency, fork, and abuse cells

Owner: Codex

- Implement the accepted deterministic ordering rule without using process race or
  local arrival time as authority.
- Distinguish ordered collision from unresolved Conflict, partition, and rule Fork.
- Implement fail-closed resolver behavior for every unresolved state.
- Apply the measured and accepted Anonymous Cost/admission profile to claim, renewal,
  resolution, policy, and recovery surfaces.
- Cover front-running, observation copying, withholding, flooding, rollback,
  equivocation, and incompatible rule versions.

Definition of Done:

- deterministic loser behavior exists only when accepted evidence proves order;
- unresolved branches remain visible and cannot release a valid Lease;
- resource bounds and honest-client limitations are measured, not inferred.

### Slice F - S6.6 independent evidence and verifier

Owner: Codex

- Implement the accepted immutable manifest and evidence schemas.
- Keep the runner unable to author expected verdict fields or mutate verifier
  outputs.
- Build a separately owned verifier that recomputes every required predicate and
  emits only `pass`, `fail`, or `invalid` plus bounded diagnostics.
- Run the A0-D6 matrix and J02/J03/J05 maintained-path trace.

Definition of Done:

- valid expected-success and expected-runtime-failure cells can produce verifier
  `pass`; contract breaches produce `fail`; malformed or contaminated artifact
  sets produce `invalid`;
- Product Owner receives the independent report and records the Stage 6 gate.

## 2. Current production boundaries

The following are current factual package-map entries, not future proposals:

- `internal/naming`: canonical Service Name and Service Link parsing/encoding;
  standard library only.
- `internal/namelease`: lease, generation, record, and monotonic state contracts;
  standard library only.
- S6.2 uses `internal/nameauthority` and `internal/nameresolution` only for the
  accepted R-046/R-047 authenticated private-resolution vertical;
  they land only with accepted field views, maintained callers, and tests.

These entries record existing boundaries, not Stage 6 completion. No verifier or
lab package name is reserved here. If one is created, its real implementation,
tests, non-test caller, `doc.go`, exact imports, and package-map row land together.
Production packages cannot import `internal/lab/*`.

## 3. Command and integration impact

S6.1 adds `cmd/ardents-name` only as a bounded canonical encoding validator. It
cannot claim, update, renew, release, resolve, publish, or recover a Name and
therefore does not select an R-046 role view or an R-042/R-044 mechanism.

Stage 6 command impact is limited to maintained paths that:

- claim/update/renew/release/transfer/recover an exact Service Name;
- resolve an exact Service Name for User connect;
- publish or replace the bound Service Target; and
- expose bounded evidence without authoring a verdict.

No Stage 1-5 command is rewired outside the naming path. No public route, DNS,
search, release/update/installer, Stage 7, or new transport behavior is added.

## 4. Slice gate

Each slice is complete only when:

1. its maintained unit and integration behavior matches the accepted decisions;
2. affected command smoke reaches the expected classified terminal outcome;
3. its immutable evidence cells receive the expected independent verdict; and
4. package-map and dependency entries remain factual.

Failure of any conjunct leaves the slice incomplete. Stage 6 as a whole remains
open until every slice, J02/J03/J05 trace, independent report, and Product Owner
disposition is complete.
