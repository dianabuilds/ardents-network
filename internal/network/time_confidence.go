package network

import (
	"errors"
	"time"
)

var ErrClockUncertain = errors.New("clock confidence is outside the two-second bound")

// ClockEvidence separates wall time, elapsed-time continuity since opening,
// and the independent observation. The application supplies these observations;
// the domain decides whether their agreement permits current network facts.
type ClockEvidence struct {
	Wall, Monotonic, Independent time.Time
}

// TrustedTime is one confirmed observation, not a running clock or a lease.
// Its zero value provides no time authority.
type TrustedTime struct{ instant time.Time }

func (observation TrustedTime) Instant() time.Time { return observation.instant }

// ConfirmTime preserves the persisted whole-second floor and its existing
// two-second rollback tolerance. Time never moves below that floor, even when
// a small wall-clock correction is tolerated. No I/O or clock is sampled here.
func ConfirmTime(evidence ClockEvidence, floor int64) (TrustedTime, error) {
	now := evidence.Wall.UTC()
	if evidence.Monotonic.After(now) {
		now = evidence.Monotonic.UTC()
	}
	if now.IsZero() || evidence.Independent.IsZero() || now.Sub(evidence.Independent).Abs() > 2*time.Second {
		return TrustedTime{}, ErrClockUncertain
	}
	if now.Unix()+2 < floor {
		return TrustedTime{}, ErrClockUncertain
	}
	if now.Unix() < floor {
		now = time.Unix(floor, 0).UTC()
	}
	return TrustedTime{instant: now}, nil
}
