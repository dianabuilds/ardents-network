# Alpha control transition contract

Status: **current Functional Alpha H4-6B contract.** This document describes
an inspectable project-operated alpha. It does not claim threshold governance,
independent custody, public control, availability, or a canonical Namespace.

## Ownership and scope

`internal/release` owns Release Safety. `internal/network/state` owns accepted
Epoch successors and duty withdrawal. The enrollment-pinned Compatibility
component binds accepted Release and Network facts without becoming either
authority. `internal/naming/namespace` retains local technical transitions, but
no global Namespace close/materialization input is selected. The
`internal/alphacontrol` declaration and `ardents-control inspect-transitions`
are read-only diagnostic projections; neither changes an owner root.

| Domain | Authority / predecessor / freshness | Rotation, revocation, and floor | Emergency, participant failure, and evidence |
|---|---|---|---|
| Release Safety | Enrollment-pinned Release trusted-root chain; retained Release floor and consecutive root chain; timestamp and Release Safety bounds. | Consecutive authenticated root rotation; authenticated build revocation/replacement; Endpoint-owned non-decreasing Release floor. | Stop new work or terminate at the authenticated deadline; `release unsafe`, `revoked`, `expired`, `conflicting`, or `unavailable`; exact metadata, artifact digest, Release component, and floor. |
| Network Epoch | Enrolled Network evidence pins authority set/threshold; persisted Epoch successor; Epoch validity and Time Confidence. | Verified State successor rotates facts and withdraws expired/ineligible duties; State-owned non-decreasing Epoch root. | Refuse State-dependent new work and drain/withdraw affected duty; `network state unavailable`, `expired`, `invalid`, `replayed`, or `conflicting`; exact Epoch bytes, authority IDs/signatures, material roots, and State floor. |
| Compatibility | Independently pinned Compatibility component; accepted Release identity plus Epoch/profile tuple; earlier bound validity limit. | Higher catalog/component generation bound to a successor tuple; revoked Release or incompatible profile invalidates it; catalog/component plus Release/State floors. | Refuse the incompatible profile, never downgrade Route; `build incompatible` or tuple unavailable/stale/forged/replayed/conflicting; exact component and all bound identities. |
| Namespace materialization | **No authority, predecessor, or accepted input in Functional Alpha.** | No rotation, revocation, or materialization floor exists. | Do not materialize, release, or reclaim a Namespace; `Namespace materialization is unavailable`; absence of a Namespace component/authority plus ADR-0054 and this contract. |

## Transition matrix

| Input condition | Required outcome | Evidence / recovery boundary |
|---|---|---|
| Forged | `forged`; reject that domain. | Invalid catalog/component signature, digest, or tuple; obtain independently authenticated replacement bytes. |
| Stale | `stale`; do not extend work or publish fresh state. | Authenticated validity bound; only a current successor can restore readiness. |
| Replayed | `replayed`; preserve the retained floor. | Lower catalog/component, Release, or State generation; never replace with older bytes. |
| Revoked | `revoked` for Release Safety; dependent Compatibility is refused. | Authenticated Release Safety decision; use an explicitly authorized replacement, never automatic install. |
| Conflicting | `conflicting`; no winner is invented. | Same-level divergent authenticated input; retain conflict evidence for owner resolution. |
| Withheld | `unavailable`; no silent cached extension beyond validity. | The Endpoint cannot distinguish intentional withholding from outage; retry only the same authenticated identity/source rule. |
| Unavailable | `unavailable`; no fallback root, source, profile, or Namespace authority. | Missing bounded input; cached valid state may be inspected only within its declared validity. |

`inspect-transitions` emits `ardents-alpha-transition-report-v1`, nested exact
H4-6A control identity, and a result for all four domains. The accepted
outcomes are `accepted` and `not-selected`; failure outcomes are
`forged`, `stale`, `replayed`, `revoked`, `conflicting`, and `unavailable`.

## Verification

The maintained report classifier exercises the complete matrix. The Linux
process test `TestAlphaControlTransitionsTwoFreshEnrolledEndpointsAgree` builds the
exact Endpoint/control pair, starts two fresh Endpoint processes with distinct
state roots, and demands byte-identical transition reports from two separate
fresh reader roots. On 2026-08-29 it passed in a network-disabled Docker
container limited to 1 vCPU, 1 GiB, and 128 PIDs. This is functional equality
evidence only; it is not a published-release qualification, independent audit,
or Public Beta promotion.

## Governing decisions

ADR-0004, ADR-0006, ADR-0038, ADR-0043, ADR-0053, and ADR-0054 govern this
contract. R-123 records its decision evidence.
