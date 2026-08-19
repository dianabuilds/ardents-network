# Stage 6 development plan

Status: **S6.0 freezes all decided (R-041, R-046, R-042, R-045, R-043,
R-044 + ADR-0013) on 2026-08-19. Stage 5 development advanced on 2026-08-19.
The remaining gate is Product Owner acceptance of the corrected documents and
the coding start decision.**

This plan maps accepted R-003/R-039 semantics and the Stage 6 brief into
sequential implementation slices without selecting undecided foundations in code.

## 0. Entry and S6.0 decision freeze

Before Stage 6 production implementation starts:

1. Product Owner records Stage 5 maintained-development `advance`, including
   S5.4 and S5.5. **Satisfied 2026-08-19; final qualification remains S9.6.**
2. Product Owner accepts the corrected brief, plan, readiness checklist, and
   evidence contract. **Pending; corrected documents now reference the
   decided S6.0 freezes.**
3. A decision record freezes exact canonical-name limits and schema version.
   **R-041, decided 2026-08-19: label 1–63 ASCII, total ≤ 253, depth ≤ 127,
   `[a-z0-9-]`, parent on the right, no leading/trailing/empty label, no
   leading/trailing or consecutive hyphen, no all-digit root, length-prefixed
   wire encoding, `schema_version = 1`.**
4. A decision record freezes deterministic claim ordering, conflict/fork
   classification, and the proof required to identify a loser.
   **R-042, decided 2026-08-19: order key
   `(network_id, epoch_number, SHA-256(canonical claim payload))`, five-state
   classification, coverage map for the eight brief scenarios.**
5. Persistence, restart, rollback, and cache-proof ownership is selected from
   current H3 principles; any lock-in receives the required research and ADR.
   **R-043, decided 2026-08-19: Go `Storage` interface in
   `internal/namelease/store.go`, default `internal/network/store`, durable /
   restart-derived / cache-bounded sets, tamper fail-closed, `epoch_number`
   replay-bound, atomic write semantics. No new technology; no ADR.**
6. Cryptographic/key-management mechanisms or explicit replaceable interfaces are
   selected through research; no primitive is implemented locally.
   **R-044 + ADR-0013, decided 2026-08-19: Ed25519 (Name Authority), BLS12-381
   (Recovery Policy threshold), OHTTP (resolver query hiding, reuse R-026
   supply), SHA-256 + HKDF-SHA-256, replaceable Go interface boundary. No
   local primitive.**
7. The R-010-compatible Anonymous Cost and local admission profile is selected
   with measurable finite limits and no identity/fairness claim.
   **R-045, decided 2026-08-19: Hashcash-style SHA-256 PoW with per-surface
   bit difficulty plus per-endpoint per-epoch rate limit plus scoped
   short-lived capability, calibrated to R-023 `2 vCPU` / `2 GiB` reference
   host. Per-surface table: claim 1/20/100, renewal 10/16/1000, resolution
   100/8/10000, policy 1/18/10, recovery 0/22/1 per generation.**
8. A field-level role matrix freezes resolution and control-operation inputs,
   observations, stable identifiers, Role Domains, known-family exclusions, and
   Isolation Context boundaries. **R-046, decided 2026-08-19: five roles with
   per-role concrete types (`endpoint-adjacent`, `naming-rendezvous`,
   `local-resolver`, `authority-operation`, `observer`), forbidden combined
   view `User/Endpoint location ∧ exact Service Name / lookup value`.**
9. The evidence schema/profile is immutable and the independent verifier
   responsibility is defined separately from the runner.
   **`stage-6-private-naming-evidence.md` § Blocking S6.0 decisions now
   references the decided freezes.**

S6.0 produces decisions and schemas, not a disguised implementation spike. A
decision that selects storage, consensus, wire protocol, cryptography, or another
lock-in follows the repository research and ADR rules.

## 1. Sequential implementation slices

### Slice A - S6.1 encoding and lifecycle core

Owner: Codex

- Implement one canonical Service Name and Service Link parser from the frozen
  S6.0 limits and schema.
- Represent Name Lease, Name Generation, Name Record, revision, parent binding,
  and the three independent state dimensions.
- Implement deterministic encoding and strict rejection of non-canonical forms.
- Implement Active/Grace/Released, renewal, reclaim, and parent/child invariants.
- Keep Conflict/Fork orthogonal to Lease state; neither can release a Lease.

Definition of Done:

- every legal and illegal parser/state edge has a maintained behavior test;
- old generation, revision, descendant, signature, and cache replay fails;
- no command or persistence behavior is added outside the accepted profile.

### Slice B - S6.2 role separation

Owner: Codex

- Implement explicit role-local request and observation types from the S6.0 field
  matrix; do not pass a superset object and hide fields by convention.
- Separate endpoint-adjacent, naming/Rendezvous, local resolver,
  authority-operation, and bounded observer responsibilities.
- Enforce query hiding, Rendezvous Role Domain and known-family exclusions, and
  Isolation Context unlinkability.
- Apply the separation to resolution and to claim/update/renew/policy/recovery
  control operations.
- Return classified failure when the private path or role evidence is absent,
  stale, conflicting, forked, or invalid.

Definition of Done:

- adversarial tests prove that no ordinary role obtains the forbidden combined
  view and no stable query identifier crosses Isolation Contexts;
- DNS, HTTP, search, local alias, alternate Namespace, and less-private fallback
  are absent and represented as explicit failure.

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

- Implement the frozen deterministic ordering rule without using process race or
  local arrival time as authority.
- Distinguish ordered collision from unresolved Conflict, partition, and rule Fork.
- Implement fail-closed resolver behavior for every unresolved state.
- Apply the frozen Anonymous Cost and local admission profile to claim, renewal,
  resolution, policy, and recovery surfaces.
- Cover front-running, observation copying, withholding, flooding, rollback,
  equivocation, and incompatible rule versions.

Definition of Done:

- deterministic loser behavior exists only when accepted evidence proves order;
- unresolved branches remain visible and cannot release a valid Lease;
- resource bounds and honest-client limitations are measured, not inferred.

### Slice F - independent evidence and verifier

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
- `internal/nameauthority`: authority transitions and recovery policy;
  may import only `internal/namelease` and the standard library.
- `internal/nameresolver`: role-separated private resolution and fail-closed
  outcomes; may import only `internal/namelease` and the standard library.

These entries record existing boundaries, not Stage 6 completion. No verifier or
lab package name is reserved here. If one is created, its real implementation,
tests, non-test caller, `doc.go`, exact imports, and package-map row land together.
Production packages cannot import `internal/lab/*`.

## 3. Command and integration impact

Stage 6 command impact is limited to maintained paths that:

- claim/update/renew/release/transfer/recover an exact Service Name;
- resolve an exact Service Name for User connect;
- publish or replace the bound Service Target; and
- expose bounded evidence without authoring a verdict.

No Stage 1-5 command is rewired outside the naming path. No public route, DNS,
search, release/update/installer, Stage 7, or new transport behavior is added.

## 4. Slice gate

Each slice is complete only when:

1. its maintained unit and integration behavior matches the frozen profile;
2. affected command smoke reaches the expected classified terminal outcome;
3. its immutable evidence cells receive the expected independent verdict; and
4. package-map and dependency entries remain factual.

Failure of any conjunct leaves the slice incomplete. Stage 6 as a whole remains
open until every slice, J02/J03/J05 trace, independent report, and Product Owner
disposition is complete.
