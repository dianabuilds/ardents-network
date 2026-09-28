//go:build linux

package introduction

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

// The symbols in this file exist for the endpoint package's integration
// tests, which must observe registration facts, force refresh and predecessor
// deadlines, inspect admission and dispatch capacity, and transplant pair
// slots without the production types exposing raw field access across the
// package boundary. Production code drives every transition only through the
// ...Locked methods and must not call these functions. The deadcode allowlist
// tracks these symbols under "test fixture support" until the test audit
// slice reworks the whitebox refresh, admission, and pair-identity fixtures.

// CreatedAt returns the registration creation instant.
func CreatedAt(registered *Registration) time.Time {
	return registered.createdAt
}

// PublishedAt returns the first verified ACK transition instant.
func PublishedAt(registered *Registration) time.Time {
	return registered.publishedAt
}

// Slot returns the immutable registration slot identity.
func Slot(registered *Registration) [32]byte {
	return registered.request.Slot
}

// Node returns the immutable registration receiver node.
func Node(registered *Registration) [32]byte {
	return registered.node
}

// Recipient returns the live private recipient proof so an integration test
// can retire it and verify that sealed capsules stop decrypting.
func Recipient(registered *Registration) *instance.PrivateRecipient {
	return registered.recipient
}

// ForceRefreshAt moves the refresh instant so an integration test can trigger
// rotation without waiting for the production schedule.
func ForceRefreshAt(registered *Registration, at time.Time) {
	registered.refreshAt = at
}

// PairPending returns the pending successor slot exactly as retained.
func PairPending(pair *PairLifecycle) *Registration {
	if pair == nil {
		return nil
	}
	return pair.pendingRegistration
}

// TransplantCurrent installs registered as the pair's acknowledged current so
// identity guards can be exercised against a registration owned by another
// context. A nil registration clears the slot exactly like the retired
// whitebox field write.
func TransplantCurrent(pair *PairLifecycle, registered *Registration) {
	pair.registration = registered
}

// ForcePreviousUntil moves the predecessor retention instant so an
// integration test can trigger predecessor retirement without waiting.
func ForcePreviousUntil(pair *PairLifecycle, until time.Time) {
	pair.previousUntil = until
}

// ReplayCount returns the retained replay-window size.
func ReplayCount(admission *Admission) int {
	return len(admission.replays)
}

// RetainedReplay returns the replay retention deadline of one nonce.
func RetainedReplay(admission *Admission, nonce [32]byte) (time.Time, bool) {
	until, retained := admission.replays[nonce]
	return until, retained
}

// SeedReplay records one replay retention entry, replacing the retired
// whitebox map assignment.
func SeedReplay(admission *Admission, nonce [32]byte, until time.Time) {
	if admission.replays == nil {
		admission.replays = make(map[[32]byte]time.Time)
	}
	admission.replays[nonce] = until
}

// OpeningWindow copies the four opening-rate instants.
func OpeningWindow(admission *Admission) [4]time.Time {
	return admission.openings
}

// ClearOpeningWindow resets the opening rate exactly like the retired whitebox
// array write.
func ClearOpeningWindow(admission *Admission) {
	admission.openings = [4]time.Time{}
}

// WaiterCount returns the occupied dispatch waiter slots.
func WaiterCount(dispatch *Dispatch) int {
	return len(dispatch.waiters)
}

// ConsumerGate returns the single dispatch consumer gate, nil before first
// waiter registration.
func ConsumerGate(dispatch *Dispatch) chan struct{} {
	return dispatch.gate
}

// RecoveryCount returns the recovery owners retained by the dispatch state.
func RecoveryCount(dispatch *Dispatch) int {
	return len(dispatch.recovery)
}

// BufferedRecovery returns the deliveries buffered for one recovery owner.
func BufferedRecovery(owner *RecoveryOwner) int {
	if owner == nil {
		return 0
	}
	return len(owner.delivery)
}

// ActiveExchangeCount returns the occupied exchange reservations.
func ActiveExchangeCount(set *ExchangeSet) int {
	return len(set.active)
}

// ExchangeRetained reports whether the exchange survived job loss.
func ExchangeRetained(exchange *Exchange) bool {
	return exchange != nil && exchange.retained
}
