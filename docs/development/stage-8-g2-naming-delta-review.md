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
| F023 | **Confirmed; open.** Current Claim Order remains a single-Name/two-claim tracer and its wire cap is inconsistent with the accepted global maximum corpus. S8.3 must redesign the global-close versus per-Name proof boundary. |
| F024 | **Partially repaired in M5 (`3c82836`); open.** R-069 makes control result `submitted` or `denied` only: submission is a volatile Gateway fact, not a current Name assertion. The pending chain and threshold Store are still uncomposed; the required durable submit-to-current/restart transaction remains open. |
| F025 | **Partially repaired in M5 (`f32f65f`); open.** Control now resolves the complete immediate-parent-to-root chain under its transition lock, so admitted root → child → grandchild create, publish, and renew no longer require caller-authored Record graphs; a released root denies descendant renewal. The exported `Record`/`Op` helpers, durable current-state transaction, restart path, and sealed Namespace Interface remain to be replaced before closure. |
| F026 | **Substantively repaired in M5 (`aeac675`); open pending Interface cutover.** R-068/ADR-0022 add signed millisecond `RecordNotAfter` in Record V4, bind it through Authority transitions, materialize the minimum own/parent/Record validity boundary, and enforce it at exact proof verification. V3 Target records are decode-only and fail closed until re-published as V4. The remaining F031 replacement of exported state/operation bags by the sealed Namespace Interface is tracked separately. |
| F027 | **Substantively repaired in M5 (`4f2ed15`, `dc6e9fe`); open pending Interface cutover.** R-065 gives control one Gateway decision time with explicit Lease seconds and Policy/Recovery milliseconds; stale/future recovery boundaries fail closed. The public time-only `advance` path can no longer complete pending Recovery: threshold completion is the sole rule. Exported direct transition bags remain an F031 cutover obligation. |
| F028 | **Confirmed; open.** Current admission is bound to a later control exchange rather than the epoch claim commitment, and the same claim is verified twice. |
| F029 | **Substantively repaired in M5 (`3c82836`); open pending sealed Interface.** The OHTTP control result exposes only `submitted` or `denied`; it no longer carries unsigned state, generation, revision, or a client-visible current success. A verified threshold Namespace proof remains required for current state. The underlying exported control/Record transition surface is still an F031 cutover obligation. |
| F030 | **Substantively repaired in M5; open pending sealed Interface.** Each accepted surface now owns its own spent set, expiry clock, mutex, and in-flight limiter. `Issue` rejects a full surface before issuing work, while a full surface does not prevent issuance on another. The remaining direct `Admission` surface is part of the F031 sealed Namespace Interface cutover. |
| F031 | **Partially repaired in M5; open.** Namespace now owns the sole production parser and validator for canonical static control input through opaque `Submission`; private Resolution control V2 carries that opaque input with its transport binding, and its former duplicate 25-field lifecycle representation/validator is deleted. Public `Record`/`Op`, direct `Apply`, the remaining command/test fixture field builders, and a single compatibility-limit table still require the sealed Namespace Interface cutover. |
| F032 | **Confirmed; decide first.** The unaccepted 4,096-record cap and complete-corpus lookup make Namespace scale/resource support a research decision rather than a refactor. |
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
