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
| F024 | **Confirmed; open.** Control's in-memory transition map and Store's externally supplied materialization remain uncomposed, with no atomic submit-to-current/restart path. |
| F025 | **Partially repaired in M5 (`f32f65f`); open.** Control now resolves the complete immediate-parent-to-root chain under its transition lock, so admitted root → child → grandchild create, publish, and renew no longer require caller-authored Record graphs; a released root denies descendant renewal. The exported `Record`/`Op` helpers, durable current-state transaction, restart path, and sealed Namespace Interface remain to be replaced before closure. |
| F026 | **Substantively repaired in M5 (`aeac675`); open pending Interface cutover.** R-068/ADR-0022 add signed millisecond `RecordNotAfter` in Record V4, bind it through Authority transitions, materialize the minimum own/parent/Record validity boundary, and enforce it at exact proof verification. V3 Target records are decode-only and fail closed until re-published as V4. The remaining F031 replacement of exported state/operation bags by the sealed Namespace Interface is tracked separately. |
| F027 | **Confirmed conflict; decide first.** Exact gateway-millisecond signed-time equality and the alternate time-only recovery advance need an accepted transcript/freshness rule before mutation. |
| F028 | **Confirmed; open.** Current admission is bound to a later control exchange rather than the epoch claim commitment, and the same claim is verified twice. |
| F029 | **Confirmed; open.** Gateway-controlled `accepted` receipts remain unauthenticated opaque state; submission and current materialization must become distinct result contracts. |
| F030 | **Confirmed; open.** A global verification mutex and delayed saturation discovery do not implement the accepted independent per-surface capacity profile. |
| F031 | **Confirmed; open.** Public Record/Op field bags and duplicated JSON/validation/wire limits preserve distributed lifecycle authority. |
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
