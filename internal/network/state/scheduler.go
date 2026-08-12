package state

import (
	"context"
	"errors"
	"time"
)

var errRefreshUnavailable = errors.New("finite sources are temporarily unavailable")
var errClockUncertain = errors.New("clock confidence is outside the two-second bound")

func (s *store) runAutomaticRefresh(ctx context.Context) {
	defer s.work.Done()
	ticker := time.NewTicker(s.config.automatic)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Refresh(ctx); err != nil && !errors.Is(err, errRefreshUnavailable) &&
				!errors.Is(err, errClockUncertain) && !errors.Is(err, context.Canceled) {
				s.mu.Lock()
				s.automaticErr = err
				s.mu.Unlock()
				return
			}
		}
	}
}
