//go:build linux

package endpoint

import "context"

// introductionExchangeSet owns the context-local exchange reservations.
// Its caller holds textContext.mu for every transition and checks job authority
// before admission. A retained exchange is still joined by Context shutdown.
type introductionExchangeSet struct {
	active map[*introductionExchange]struct{}
}

func (set *introductionExchangeSet) fullLocked(limit int) bool {
	return len(set.active) >= limit
}

func (set *introductionExchangeSet) addLocked(cancel context.CancelFunc) *introductionExchange {
	flight := &introductionExchange{cancel: cancel, done: make(chan struct{})}
	if set.active == nil {
		set.active = make(map[*introductionExchange]struct{})
	}
	set.active[flight] = struct{}{}
	return flight
}

func (set *introductionExchangeSet) removeLocked(flight *introductionExchange) {
	delete(set.active, flight)
	close(flight.done)
}

func (set *introductionExchangeSet) retainLocked(flight *introductionExchange) bool {
	if flight == nil || flight.retained {
		return false
	}
	if _, present := set.active[flight]; !present {
		return false
	}
	flight.retained = true
	return true
}

func (set *introductionExchangeSet) stopLocked() []*introductionExchange {
	flights := make([]*introductionExchange, 0, len(set.active))
	for flight := range set.active {
		if !flight.retained {
			flight.cancel()
		}
		flights = append(flights, flight)
	}
	return flights
}
