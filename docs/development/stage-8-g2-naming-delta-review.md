# Stage 8 G2 naming and private-resolution delta review

Status: **S8.0 factual delta review; not a target-design decision.** This
temporary record rechecks G2 F023--F033 at the Stage 8 source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.

## Method

The preparation pass was anchored at
`840266e08174efe9a9a4bd056182cea097ca7194`. No Stage-7-entry delta touches
`naming`, `nameadmission`, `nameauthority`, `nameclaim`, `namelease`,
`namerecovery`, `namestore`, `nameresolution`, or `cmd/ardents-name`. The
prepared findings therefore cannot be silently closed by the unrelated Update
work.

The entry source was inspected at claim ordering, control-to-materialization,
lineage/expiry/time/admission, Gateway result, and storage-resolution seams.
The current focused command and Module suites passed:

```text
go test ./internal/naming ./internal/nameadmission ./internal/nameauthority \
  ./internal/nameclaim ./internal/namelease ./internal/namerecovery \
  ./internal/namestore ./internal/nameresolution ./cmd/ardents-name \
  -count=1 -shuffle=on
```

This is current tracer evidence, not a proof that a green package portfolio
implements the accepted global Namespace, privacy, resource, or platform
contract.

## Finding disposition

| Finding | S8.0 disposition |
|---|---|
| F023 | **Partially repaired in M5; open.** R-071 replaces `ApplyOrderedClaim(raw proof, caller Op)` with an opaque verified `ClaimWinner`: `ClaimOrder.Verify` authenticates the R-042 close once, and materialization derives the root/reclaim transition without a later Name, Authority, ordinal, or lease-deadline choice. R-072 carries that fact through a Store-owned Epoch installation, which starts at verified current state and accepts only an exact signer response. This deliberately preserves R-042's per-Name 32-claim proof and does not invent a total-Epoch/product-scale cap. The remaining closure is complete global-close/commit-admission ingestion plus removal of the legacy raw corpus surface. |
| F024 | **Substantively repaired in M5 (`0c88d99`, `ee2769d`, `b781093`); open pending production startup composition and evidence replacement.** R-070/ADR-0023 require a Gateway decision to derive and verify the exact Authority-signed successor before atomically appending its opaque submission to the immutable pending journal. A threshold materialization may select only the durable pending successor prefix, persists its cursor in the generation integrity binding, and durable `OpenControl` restores the unapplied chain after restart. The retained Gateway control integration test composes that durable authority through the private OHTTP boundary, accepts its exact successor once, and rejects the stale repeat; the public startup constructor still receives its authority and needs the Namespace-owned final port. Stage 6 remains provenance, not proof of the new transaction. |
| F025 | **Partially repaired in M5 (`f32f65f`); open.** Control now resolves the complete immediate-parent-to-root chain under its transition lock, so admitted root → child → grandchild create, publish, and renew no longer require caller-authored Record graphs; a released root denies descendant renewal. The exported `Record`/`Op` helpers, durable current-state transaction, restart path, and sealed Namespace Interface remain to be replaced before closure. |
| F026 | **Substantively repaired in M5 (`aeac675`); open pending Interface cutover.** R-068/ADR-0022 add signed millisecond `RecordNotAfter` in Record V4, bind it through Authority transitions, materialize the minimum own/parent/Record validity boundary, and enforce it at exact proof verification. V3 Target records are decode-only and fail closed until re-published as V4. The remaining F031 replacement of exported state/operation bags by the sealed Namespace Interface is tracked separately. |
| F027 | **Substantively repaired in M5 (`4f2ed15`, `dc6e9fe`); open pending Interface cutover.** R-065 gives control one Gateway decision time with explicit Lease seconds and Policy/Recovery milliseconds; stale/future recovery boundaries fail closed. The public time-only `advance` path can no longer complete pending Recovery: threshold completion is the sole rule. Exported direct transition bags remain an F031 cutover obligation. |
| F028 | **Partially repaired in M5; open pending commit-ingestion owner.** Durable Gateway control rejects `claim` before admission or journal mutation, so an authenticated current-state submission can no longer consume a late root-claim challenge. R-071's opaque `ClaimWinner` isolates the single authenticated close verification from later materialization, and R-072 carries it through a threshold Store installation; the historical Stage 6 `NewControl.Apply` C4 bridge now uses that boundary instead of rerunning Claim Order. Namespace still needs the actual commit-ingestion owner for R-045's commit-bound admission and complete global-close materialization. |
| F029 | **Substantively repaired in M5 (`3c82836`); open pending sealed Interface.** The OHTTP control result exposes only `submitted` or `denied`; it no longer carries unsigned state, generation, revision, or a client-visible current success. A verified threshold Namespace proof remains required for current state. The underlying exported control/Record transition surface is still an F031 cutover obligation. |
| F030 | **Substantively repaired in M5; open pending sealed Interface.** Each accepted surface now owns its own spent set, expiry clock, mutex, and in-flight limiter. `Issue` rejects a full surface before issuing work, while a full surface does not prevent issuance on another. The remaining direct `Admission` surface is part of the F031 sealed Namespace Interface cutover. |
| F031 | **Partially repaired in M5; open.** Namespace owns the sole production parser and validator for canonical static control input through opaque `Submission`, and the Gateway authority contract now accepts only `Submit(Submission, Proof)` with the bounded `submitted`/`denied` result. Private Resolution carries that opaque value with its transport binding; its former duplicate 25-field lifecycle representation/validator is deleted. Its Gateway and Resolver now consume `VerifyBinding` rather than lifecycle `Record` values from current-proof verification. R-073 replaces the accidental 16 MiB Record ceiling with a measured 1,846-byte payload / 1,920-byte signed container limit, so an unreturnable Record is rejected before signing. Stage 6 retains an explicitly isolated C4 bridge to its historical detailed `Apply` evidence, which is not a runtime Gateway contract. The static canonical form omits zero-value scalar and empty byte fields, keeping the fixed private envelope bounded without treating padding as lifecycle state. Public `Record`/`Op`, direct `Apply`, the remaining command/test fixture field builders, optional successor omission before durable-authority submission, and the final sealed Namespace Interface still require cutover. |
| F032 | **Substantively repaired in M5; open pending scale redesign.** R-066 replaces the unmeasured 4,096-record implementation cap with the exact 127-record, one-writer technical-tracer envelope and the Store rejects a 128-record corpus. Exact lookup still loads and validates the complete immutable corpus, so this is not a product-scale selection: a supported capacity/index/cache needs the separately required resource decision and measurement. |
| F033 | **Confirmed; open.** Current package tests and historical Stage 6 evidence omit or contradict the stated global, restart, timing, forgery, capacity, and scale rows. S8.2/G3 must replace, not layer over, them. |

## Consequence for Stage 8

The entry supports retaining the product **responsibilities** of canonical
Service Name, admission, authority, lease/recovery, current materialization,
and private resolution only if S8.1 preserves them. It does not support
retaining their current package boundaries or asserting that `ardents-name`
is a complete product control/publish path. F027 and F032 are research/contract
decisions that block their respective state, transcript, and resource changes.

The succeeding G2 review group begins with Service Connection, publication,
Endpoint composition, and Application IPC. This record makes no S8.1 choice.
