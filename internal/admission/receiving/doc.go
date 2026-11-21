// Package receiving owns receiver-side Admission decisions against current
// authenticated facts supplied by Network State. It selects the token's exact
// issuer key, receiver, class and hour; it grants no State authority and owns
// no transport, listener or provider resource budget. Redemption owns reserve-
// before-spend ordering, final expiry, and rollback without token refunds.
// OpenRetained selects the strict existing spend owner, without creating or
// repairing missing/damaged history or issuing a fresh-root fact.
// Original operations serialize observations and spends outside the local
// state mutex. Close denies admission before joining those users and their
// refusal cleanup; only the work owner releases a transferred reservation.
package receiving
