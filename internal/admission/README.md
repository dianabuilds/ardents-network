# Admission domain

Admission answers one question: may this holder obtain, present and spend a
finite right to consume work at this receiver? It owns the rules and state of
that right. It does not own the route that carries it or the provider's physical
resource budget.

The domain includes distinct authority holders. Allocation, issuer and receiver
state are not one shared aggregate or database; a holder cannot acquire issuer
authority merely because their code belongs to the same domain.

## Ownership

| Responsibility | Admission owner | Consumer and external responsibility |
| --- | --- | --- |
| Finite resource classes | `Class` and its byte/lifetime policy | Route interprets frames and enforces the returned limits; Hosting translates the allowance into its physical envelope |
| Permission allocation | `allocation`; canonical permission and holder request values | Custody protects the authority key, serializes encrypted persistence and signs the bounded decision |
| Issuance | `issuer`; public token grammar and blind-state operations in `token` | Node provides current authenticated State and owns listener/process lifetime; Route carries the exchange |
| Holder permission, pending batch and stock | `stock` | Endpoint supplies context authorization and composes Source operations; it must not decide quotas or mutate stock independently |
| Durable presentation attempt | `attempts` | Endpoint retains the local root and calls the exact mark-before-presentation operation |
| Receiving spend and replay | `spending` | A receiving duty retains its exclusive root through joined shutdown |
| Receiving decision | `receiving` verifies the current profile, receiver, class, key and hour; reservation and irreversible spend ordering belong to the same domain use case | State supplies authenticated facts; Hosting supplies a finite reservation; Route supplies the channel binding and owns its physical termination |

The receiving use case is `receiving.Redeem`: authorization and Hosting reservation
precede channel capacity, durable spend precedes acceptance, and the deadline is
checked again after durable I/O. Every refusal releases acquired reservations
and retains cleanup failures; expiry after spend never refunds the token.
`route/closed_admission_channel.go` adapts initial framed admission and transfers
physical channel ownership; `node/hosting/admission.go` adapts refills on an
existing channel to the same use case. `node/authority/token.go` obtains the live
State profile and adapts the duty facts to `receiving.VerifyToken`.
Channel framing, forwarding scheduling and physical termination remain outside
Admission.

## Holder aggregate and coordination

`stock.Owner` owns one holder permission, its per-class reservations, pending
blind batch, finalized stock and single issuance slot. Endpoint sees opaque
observation and operation handles, not writable balances, holder keys or blind
state. Admission alone reserves quota, matches exact retries, deposits issued
tokens and removes presented tokens.

The shared Endpoint mutex deliberately covers local context revocation and
these Admission transitions. It is a coordination lock, not shared ownership of
the permission. Splitting it would require another atomic authorization protocol
between the two owners and would not improve the current domain model.

| Transition | Admission linearization and external coordination |
| --- | --- |
| Begin issuance | `BeginIssuanceLocked` validates current facts, reserves the exact batch and acquires the single operation slot under the context lock; Endpoint runs transport after unlocking |
| Accept issuance | `operation.complete` rechecks the same operation, permission identity, profile, deadline and cancellation before installing tokens |
| Revoke | Endpoint revokes local authority; `StopLocked` detaches the permission and cancels its operation under the same lock; Endpoint joins outside it; Admission erases retired operation secrets after completion |
| Present | `TakeTokenLocked` removes stock, durably marks the attempt, rechecks surviving authority and cancellation, then returns bytes; failure cannot restore the token |

The `Host` adapter supplies live authority facts and the selected Source's
transport capabilities. `operation` is Admission's issuance use case; concrete
Carrier behavior stays in Route. Source/JOIN scheduling and worker lifetime
remain Endpoint responsibilities. The transport-specific projection preserves
the selected closed protocol; it does not allow Endpoint to write Admission
state or decide quota. A generic repository or transport framework is unnecessary.

## Invariants

- A permission is bound to its holder, issuer duty, exact hour and finite classes.
- Issuance commits quota before exposing signatures; an exact retry cannot debit
  twice or change the original request.
- Stock retains the pending blind request until a terminal result, and exposes
  no holder key or blind state to an application.
- Presentation is durably marked before token bytes cross the transport seam.
  An uncertain or failed exchange cannot refund that attempt.
- Redemption binds the token to the current authenticated receiver, class and
  hour; durable spend prevents a second accepting use.
- Admission never extends State authority or a caller's deadline. Missing
  currentness, capacity or required durable state yields refusal.
- Cancellation revokes future authority immediately; owned work and reservation
  cleanup still join. Successful business output does not erase cleanup failure.

Behavior checks must cross the same interfaces as maintained consumers and
include issue/retry, revoke/cancel, restart, presentation and duplicate spend.
Transport repairs are separate work unless required to preserve a migrated
Admission invariant.

## Offline successor

The isolated `internal/successor/admission` tree implements the separately
accepted offline issuance contracts. It is also Admission work, but supplies no
live State authority and cannot be used as proof that live consumers have been
migrated. Existing isolation and persisted-format obligations continue to apply.
The package map records exact allowed imports; the issue tracker records
execution and acceptance.
