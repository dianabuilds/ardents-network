# Stage 8 G2 Network State delta review

Status: **S8.0 factual delta review; not a target-design decision.** This
temporary record rechecks G2 F013--F022 at the Stage 8 source entry
`1cf7100da3ada32ba53abb51201aaf7b6183a3da`.

## Method

The preparation pass closed at
`c83f7d7ffd6438af20ac588fc3d7c415833c87b0`. There is no Stage-7-entry delta
in `internal/network/state`, `internal/network/store`, `internal/network/epoch`,
`internal/network/source`, `internal/localroles`, or `internal/namestore`.
Thus no prepared finding in this group can be considered closed by unrelated
Update Transaction work.

The current entry source was inspected at its named state/configuration,
recovery, commit, outcome, and Namespace storage seams. The following focused
diagnostic passed:

```text
go test ./internal/network/state ./internal/network/store \
  ./internal/network/epoch/... ./internal/network/framing \
  ./internal/network/source ./internal/namestore -count=1 -shuffle=on
```

This confirms current behavior only; it cannot override an accepted persistence
contract, fill an omitted fault cell, or qualify a platform claim.

## Finding disposition

| Finding | Entry evidence | S8.0 disposition |
|---|---|---|
| F013 | Current offline and source acceptance still use the time/commit pattern described in the prepared pass; no production delta changes it. | **Confirmed; open.** S8.3 must select one owned decision-time rule and add expiry-at-commit oracles. |
| F014 | `recoverMissingCurrent` remains the entry recovery seam and the contrary missing-pointer test remains in the current suite. | **Confirmed conflict; decide first.** The accepted R-027/R-029 recovery rule must be reconciled before a format or repair change. |
| F015 | Current Network Store has its own lease/filesystem implementation; the later Update physical-lock remediation did not alter it. | **Confirmed as unrechecked high-risk implementation; open.** S8.3 needs a current platform/physical-object review and format plan. |
| F016 | `commitActiveDecision` remains the two-pointer commit choreography named by the prepared pass. | **Confirmed; open.** S8.3 must select authoritative state and typed degraded/committed recovery semantics. |
| F017 | `Wait`, `Close`, `Current`, and Refresh retain their current lifecycle/callback ownership; no delta changes the observation model. | **Confirmed; open.** A future Interface must own terminal-worker observation and invoke external ports outside locks. |
| F018 | Public State Config still includes concrete source configuration and `LocalRoleStateRoot`; State opens/writes local-role state. | **Confirmed; open.** S8.3 must introduce consumer-owned Source and Duty seams if these behaviors survive S8.1. |
| F019 | Current outcome classification still lowers/scans error text, and the status-string mapper retains its implicit concrete-Plan precondition. | **Confirmed; open.** S8.3 must choose a closed typed source result/failure vocabulary. |
| F020 | `namestore.Open` still calls `networkstore.Open` and stores `*networkstore.Root`. | **Confirmed; open.** The shared generation engine requires an explicit technical-Adapter disposition, not a silent Network or Namespace fold. |
| F021 | Current `network/epoch/merkle` is still a demonstrated Network and Namespace commitment dependency. | **Confirmed; open package-placement decision.** S8.3 must select one precisely named shared foundation or explicit versioned domain representations. |
| F022 | Current focused tests omit the fault/reentrancy/physical-object rows listed in the prepared pass, including the F014 contradiction. | **Confirmed; open.** S8.2/G3 must replace the portfolio by risk-owned Module and Adapter contract suites. |

## Consequence for Stage 8

The accepted target **domain** responsibility of `network/state` remains a
valid design input. The entry does not support preserving its current concrete
source/duty/storage composition, treating either current pointer as implicitly
authoritative, or folding the shared filesystem/Merkle mechanisms into a domain
package without a compatibility decision. F014 blocks any missing-pointer
repair or format migration until its accepted authority conflict is resolved.

The next G2 review group is naming/private-resolution; this record leaves all
S8.1 product-disposition choices untouched.
