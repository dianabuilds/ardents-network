//go:build linux

package introduction

import (
	"errors"
	"time"
)

// Admission owns the context-local opening rate and accepted
// delivery replay floor. Its caller holds dutyContext.mu for every transition.
// The zero value is ready for use.
type Admission struct {
	replays  map[[32]byte]time.Time
	openings [4]time.Time
}

// At most four candidates per second over the 600-second registration plus
// the required 60-second replay retention. Failed or forged openings cannot
// clear accepted replay history.
const maximumReplays = 4 * (600 + 60)

// ReserveOpeningLocked consumes one opening-rate slot and rejects replayed or
// over-capacity nonces.
func (admission *Admission) ReserveOpeningLocked(nonce [32]byte, now time.Time) error {
	for retained, expiry := range admission.replays {
		if !now.Before(expiry) {
			delete(admission.replays, retained)
		}
	}
	if _, replay := admission.replays[nonce]; replay || len(admission.replays) >= maximumReplays {
		return errors.New("text Introduction replay or capacity refusal")
	}
	if now.Before(admission.openings[3]) || now.Before(admission.openings[0].Add(time.Second)) {
		return errors.New("text Introduction opening rate unavailable")
	}
	copy(admission.openings[:3], admission.openings[1:])
	admission.openings[3] = now
	return nil
}

// RetainAcceptedLocked records one accepted delivery nonce for the required
// replay retention window beyond the registration expiry.
func (admission *Admission) RetainAcceptedLocked(nonce [32]byte, registrationExpiry time.Time) {
	if admission.replays == nil {
		admission.replays = make(map[[32]byte]time.Time)
	}
	admission.replays[nonce] = registrationExpiry.Add(60 * time.Second)
}

// StopLocked clears every replay and rate slot without joining anything.
func (admission *Admission) StopLocked() {
	clear(admission.replays)
	admission.replays = nil
	admission.openings = [4]time.Time{}
}
