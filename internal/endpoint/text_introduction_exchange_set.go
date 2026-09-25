//go:build linux

package endpoint

import "context"

// textIntroductionExchangeSet owns the context-local exchange reservations.
// Its caller holds textContext.mu for every transition and checks job authority
// before admission. A retained exchange is still joined by Context shutdown.
type textIntroductionExchangeSet struct {
	active map[*textIntroductionExchange]struct{}
}

func (set *textIntroductionExchangeSet) fullLocked(limit int) bool {
	return len(set.active) >= limit
}

func (set *textIntroductionExchangeSet) addLocked(cancel context.CancelFunc) *textIntroductionExchange {
	flight := &textIntroductionExchange{cancel: cancel, done: make(chan struct{})}
	if set.active == nil {
		set.active = make(map[*textIntroductionExchange]struct{})
	}
	set.active[flight] = struct{}{}
	return flight
}

func (set *textIntroductionExchangeSet) removeLocked(flight *textIntroductionExchange) {
	delete(set.active, flight)
	close(flight.done)
}

func (set *textIntroductionExchangeSet) retainLocked(flight *textIntroductionExchange) bool {
	if flight == nil || flight.retained {
		return false
	}
	if _, present := set.active[flight]; !present {
		return false
	}
	flight.retained = true
	return true
}

func (set *textIntroductionExchangeSet) stopLocked() []*textIntroductionExchange {
	flights := make([]*textIntroductionExchange, 0, len(set.active))
	for flight := range set.active {
		if !flight.retained {
			flight.cancel()
		}
		flights = append(flights, flight)
	}
	return flights
}
