// Package receiving owns receiver-side Admission decisions against current
// authenticated facts supplied by Network State. It selects the token's exact
// issuer key, receiver, class and hour; it grants no State authority and owns
// no transport, listener or provider resource budget. Redemption owns reserve-
// before-spend ordering, final expiry, and rollback without token refunds.
package receiving
