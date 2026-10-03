# Hosting

Hosting owns the installed provider-period allowance, directional whole-interface
charging, durable work/termination reservations and local observation sharing.
Admission owns token authority and spending. Node composes these decisions and
joins accepted work before releasing capacity or closing its borrowed handle.

| Source | Responsibility |
|---|---|
| policy.go | Denominations, period and interface bounds, charging and low watermark |
| ledger.go | Serialized durable transactions, reserve and exactly-once release |
| storage.go | Canonical bounded state, immutable policy pin and commit continuity |
| sample.go | Cross-process committed observation freshness and elected refresh |
| platform_linux.go | Linux counter identity and process locks |
| platform_other.go | Explicit unsupported-platform refusal |
| shared.go | Root-local observation sharing and independent handle lifetime |

`Initialize` is an explicit operator action. `Open` never resets a period.
`OpenShared` adds bounded observation sharing for concurrent Node duties. Both
operate on the existing `ardents-hosting-period-v1` root. Independent successor
Hosting retains its separate `ardents-hosting-budget-v1` contract; no automatic
migration or cross-import exists.

Each reservation's copies share private release state. Cancellation before the
mutation callback permits retry; once mutation starts, an uncertain release cannot
retry a refund against another reservation. Process loss retains outstanding debit.
A fresh deadline check precedes mutation and follows the durable commit; late
transfer refuses and attempts bounded cleanup of only its own reservation.

Shared handle copies share close state. Independent opens retain independent ledger
handles and one reference-counted sampler. The handle gate joins active operations
before close and refuses subsequent reads, including cached reads. The sampler
copies slice-backed observations on ingress and egress. Every reservation attempt
invalidates cache and older flights, including storage refusal or uncertain commit.
A failed release also invalidates reuse; successful release may retain an overestimate until the next fresh sample (at most one second);
it cannot create a falsely larger available budget.

Behavior checks cover actual Linux interface counters and storage, copied handles,
concurrent owners, continuity failures, low watermark, cancellation, expiry, pending
journals, reopen, freshness and cache ownership. Node's real-Hosting redemption
scenario combines genuine blind tokens with durable receiving spend and capacity
roots, and checks refusal, replay, retained work, failed spending and post-spend
expiry together. These are bounded local behavior claims, not deployment or provider
billing certification. Filesystem trust and recovery limits remain those of the
current accepted contract; this change adds no rollback-resistant external anchor.
