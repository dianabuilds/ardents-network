//go:build linux

package introduction

import "context"

// Exchange is one bounded delivery or transport reservation. Its cancel stays
// with the reserving root orchestration; the set closes Done only when the
// reservation is removed.
type Exchange struct {
	cancel   context.CancelFunc
	done     chan struct{}
	retained bool
}

// Done is closed exactly once, when the exchange leaves the set. Context
// retirement joins it even for retained exchanges.
func (exchange *Exchange) Done() <-chan struct{} { return exchange.done }

// ExchangeSet owns the context-local exchange reservations.
// Its caller holds dutyContext.mu for every transition and checks job authority
// before admission. A retained exchange is still joined by Context shutdown.
// The zero value is ready for use.
type ExchangeSet struct {
	active map[*Exchange]struct{}
}

// FullLocked reports whether the caller-supplied reservation limit is reached.
func (set *ExchangeSet) FullLocked(limit int) bool {
	return len(set.active) >= limit
}

// AddLocked admits one reservation under its root-owned cancel.
func (set *ExchangeSet) AddLocked(cancel context.CancelFunc) *Exchange {
	flight := &Exchange{cancel: cancel, done: make(chan struct{})}
	if set.active == nil {
		set.active = make(map[*Exchange]struct{})
	}
	set.active[flight] = struct{}{}
	return flight
}

// RemoveLocked releases the reservation and closes its join channel.
func (set *ExchangeSet) RemoveLocked(flight *Exchange) {
	delete(set.active, flight)
	close(flight.done)
}

// RetainLocked marks one active exchange as surviving job loss. It reports
// whether this call performed the transition.
func (set *ExchangeSet) RetainLocked(flight *Exchange) bool {
	if flight == nil || flight.retained {
		return false
	}
	if _, present := set.active[flight]; !present {
		return false
	}
	flight.retained = true
	return true
}

// StopLocked cancels every non-retained reservation and returns all flights
// for the caller-owned retirement join.
func (set *ExchangeSet) StopLocked() []*Exchange {
	flights := make([]*Exchange, 0, len(set.active))
	for flight := range set.active {
		if !flight.retained {
			flight.cancel()
		}
		flights = append(flights, flight)
	}
	return flights
}
