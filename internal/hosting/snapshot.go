package hosting

import (
	"context"
	"errors"
	"time"
)

// Snapshot is a confirmed observation, never permission to start work.
// All fields are values; callers cannot mutate retained accounting.
type Snapshot struct {
	at, until   time.Time
	observation Observation
}

func (s Snapshot) ObservedAt() time.Time    { return s.at }
func (s Snapshot) ValidUntil() time.Time    { return s.until }
func (s Snapshot) Observation() Observation { return s.observation }

// Sample shares a recent persisted measurement across independent handles and
// processes. It always validates the journal under its lease; an unconfirmed
// replacement is never treated as committed. Reserve never uses this path.
func (owner *Budget) Sample(ctx context.Context, maximumAge time.Duration) (Snapshot, error) {
	if maximumAge <= 0 || maximumAge > time.Second {
		return Snapshot{}, ErrInvalid
	}
	state, _, err := owner.transact(ctx, maximumAge, nil)
	if err != nil {
		return Snapshot{}, err
	}
	now := owner.now()
	if ctx.Err() != nil || now.Before(state.Observed) || now.Sub(state.Observed) > maximumAge {
		return Snapshot{}, errors.Join(errors.New("hosting sample unavailable at transfer"), ctx.Err())
	}
	until := state.Observed.Add(maximumAge)
	if state.Policy.End.Before(until) {
		until = state.Policy.End
	}
	// Expired periods remain observable as Drain, never as usable capacity.
	observation := state.observation(now)
	return Snapshot{at: state.Observed, until: until, observation: observation}, nil
}
