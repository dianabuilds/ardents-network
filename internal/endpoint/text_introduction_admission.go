//go:build linux

package endpoint

import (
	"errors"
	"time"
)

// textIntroductionAdmission owns the context-local opening rate and accepted
// delivery replay floor. Its caller holds textContext.mu for every transition.
type textIntroductionAdmission struct {
	replays  map[[32]byte]time.Time
	openings [4]time.Time
}

// At most four candidates per second over the 600-second registration plus
// the required 60-second replay retention. Failed or forged openings cannot
// clear accepted replay history.
const maximumTextIntroductionReplays = 4 * (600 + 60)

func (admission *textIntroductionAdmission) reserveOpeningLocked(nonce [32]byte, now time.Time) error {
	for retained, expiry := range admission.replays {
		if !now.Before(expiry) {
			delete(admission.replays, retained)
		}
	}
	if _, replay := admission.replays[nonce]; replay || len(admission.replays) >= maximumTextIntroductionReplays {
		return errors.New("text Introduction replay or capacity refusal")
	}
	if now.Before(admission.openings[3]) || now.Before(admission.openings[0].Add(time.Second)) {
		return errors.New("text Introduction opening rate unavailable")
	}
	copy(admission.openings[:3], admission.openings[1:])
	admission.openings[3] = now
	return nil
}

func (admission *textIntroductionAdmission) retainAcceptedLocked(nonce [32]byte, registrationExpiry time.Time) {
	if admission.replays == nil {
		admission.replays = make(map[[32]byte]time.Time)
	}
	admission.replays[nonce] = registrationExpiry.Add(60 * time.Second)
}

func (admission *textIntroductionAdmission) stopLocked() {
	clear(admission.replays)
	admission.replays = nil
	admission.openings = [4]time.Time{}
}
