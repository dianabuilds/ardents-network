package state

import (
	"context"
	"errors"
	"time"
)

var errRefreshUnavailable = errors.New("finite sources are temporarily unavailable")
var errClockUncertain = errors.New("clock confidence is outside the two-second bound")
var errRefreshActive = errors.New("network state refresh is already active")

func (s *networkState) runAutomaticRefresh(ctx context.Context, ticks <-chan time.Time, results chan<- error) {
	defer s.work.Done()
	if ticks == nil {
		ticker := time.NewTicker(s.config.automatic)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			_, err := s.Refresh(ctx)
			if results != nil {
				select {
				case results <- err:
				case <-ctx.Done():
					return
				}
			}
			if err != nil && !errors.Is(err, errRefreshUnavailable) &&
				!errors.Is(err, errClockUncertain) && !errors.Is(err, errRefreshActive) && !errors.Is(err, context.Canceled) {
				s.mu.Lock()
				s.automaticErr = err
				s.mu.Unlock()
				return
			}
		}
	}
}
