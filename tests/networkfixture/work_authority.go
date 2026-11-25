package networkfixture

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

// WorkAuthority retains the accepted facts and a monotonic observation floor.
func WorkAuthority(initial admission.AuthorityFacts, floor, deadline time.Time, observe func() (admission.AuthorityFacts, time.Time, error)) func() error {
	return func() error {
		facts, now, err := observe()
		if err != nil || facts != initial || now.Before(floor) || now.Before(initial.NotBefore) || !now.Before(initial.NotAfter) || !now.Before(deadline) {
			return errors.Join(errors.New("work authority unavailable"), err)
		}
		floor = now
		return nil
	}
}
