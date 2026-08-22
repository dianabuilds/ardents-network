# Stage 8 G2 Release and Update delta review

Status: **S8.0 factual delta review; not a target-design decision.** This
temporary record rechecks G2 findings F001--F012 at the Stage 8 source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.

## Method and scope

The first G2 pass was anchored at
`e843f556dfb003c7aa8862fe2e4095ddc134ae49`. The entry delta contains 82
changed files and 9,226 added / 229 removed lines in the Release/Update, Node,
and Route paths. The substantive change is Update Transaction recovery: 45
production files and 30 test files now belong to that Module. Release Decision
production files did not change in this delta.

The review inspected the current public caller, Release Decision entry points,
Update request validation, durable-lock implementations, and the current
recovery corpus. It also reproduced:

```text
go test ./internal/releasedecision ./internal/updatetransaction \
  ./cmd/ardents-release -count=1 -shuffle=on
go test ./internal/updatetransaction -count=1 -shuffle=on
```

Both commands passed. The second includes the R00--R14 recovery matrix,
interruption, rollback, cleanup, corruption, and resource-envelope tests. A
passing current corpus is evidence about the current tracer; it is not an
installation, platform, or product-activation claim.

## Finding disposition

| Finding | Entry evidence | S8.0 disposition |
|---|---|---|
| F001 | `updatetransaction.validateRequest` still accepts a caller-constructible `releasedecision.Decision` and compares its string fields with `"release-accepted"`. | **Confirmed; open.** S8.3 must make release authorization unforgeable to Update. |
| F002 | `releasedecision.Store`, `OpenFloorStore`, and its commit methods remain public; `cmd/ardents-release.run` opens and closes the Store. | **Confirmed; open.** S8.3 must decide the Release persistence boundary. |
| F003 | `Evaluate` still documents/preserves verified roots before later executable metadata rejection, while the cited R-049 rule requires the opposite transaction meaning. | **Confirmed conflict; decide first.** No state-format or recovery mutation is authorized before research authority resolves it. |
| F004 | Release's `floorStore` remains separate from the later Update lock remediation and was not changed in the entry delta. | **Unrechecked implementation; open.** A focused Release filesystem review is required before a target lock/format decision. |
| F005 | Release generation/archive verification code was not changed in the entry delta. | **Unrechecked implementation; open.** Current content-address and root-chain restart behavior needs a focused review. |
| F006 | Release inputs and public Decision are still caller-facing values; the entry delta did not alter Release input ownership. | **Unrechecked implementation; open.** Recheck byte ownership and TOCTOU at the future Release Interface. |
| F007 | `Evaluate` still calls `errStringContains` to select `release-incompatible`; the public Decision still renders notices. | **Confirmed; open.** S8.3 must define typed internal failures and bounded/redacted boundary results. |
| F008 | Update now opens a permanent existing lock, uses non-blocking OS locking, verifies held-handle/path identity and shape, and joins unlock/close errors. Current recovery takes its inventory after acquiring this lock. | **Invalidated in its historical form.** The old marker-lock finding is replaced by the current physical-lock evidence. S8.3 still decides whether this tracer format/interface is retained. |
| F009 | `Request` still accepts caller-supplied `Generation`, `ActiveWork`, and `SchemaPlan`; `cmd/ardents-release.run` supplies `1`, `0`, and `no-op-v1`. | **Confirmed; open.** S8.3 must give Update ownership of successor generation, work observation, and schema policy. |
| F010 | The sole production caller still supplies `stoppedRuntime` and `offlineCandidateTest`; neither performs real stop/drain, activation, or IPC readiness. | **Confirmed; open tracer limitation.** A real product Adapter is not present. |
| F011 | Current Update recovery has explicit rollback-pending, rollback-refused, rolled-back, repair-required, and successful-self-test paths, all covered by current restart tests. | **Historical statement invalidated for the former recovery gap.** The product limitation in F010 remains: no real activation/self-test composition is proven. |
| F012 | No Custody Module exists; `CustodyNotice` remains persisted and rendered by Release/Update result formats. | **Confirmed; open.** S8.1 must give its format a preserve/migrate/break decision; S8.3/M12 owns a real custody boundary if retained. |

## Consequence for the Stage 8 design

The Stage 7 delta may be used as current recovery evidence for its named
tracer, but it is not silently promoted as the final Update Module. F001--F007,
F009--F010, and F012 are live input to S8.1/S8.3; F003 blocks any Release
format change until its semantic conflict is decided. F008 and F011 are
historical findings whose exact former defects are no longer current, but their
replacement does not select a product contract.

The remaining G2 finding groups need the same source-bound disposition review:
Network State/Store (F013--F022), naming (F023 and later related rows),
route/service/application composition, documentation/testing, and platform
storage. This record does not close S8.0 by itself.
