package channel

import (
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

// Budget belongs to one physical Route principal, not one connection.
// Control reservations and retained payload share its finite aggregate cap.
type Budget struct {
	mu                                          sync.Mutex
	bytesMetric, childrenMetric, channelsMetric measurement.Gauge
	used, maximum                               uint64
	channels, children                          uint32
}

// NewBudget creates the finite memory owner shared by one physical principal.
func NewBudget(maximum uint64) *Budget {
	return &Budget{maximum: maximum, bytesMetric: measurement.Track(runtimeevent.RouteReservedBytes), childrenMetric: measurement.Track(runtimeevent.RouteChildren), channelsMetric: measurement.Track(runtimeevent.RouteChannels)}
}

// HoldChild reserves one child and its fixed memory together. The release
// belongs to this exact claim and is idempotent; callers retain it until join.
func (q *Budget) HoldChild(memory uint64) (func(), error) {
	if q == nil {
		return nil, errors.New("route child capacity unavailable")
	}
	q.mu.Lock()
	if q.children >= 1024 || memory > q.maximum-q.used {
		q.mu.Unlock()
		return nil, errors.New("route child capacity unavailable")
	}
	q.children++
	q.childrenMetric.Add(1)
	q.used += memory
	q.bytesMetric.Increase(memory)
	q.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			q.mu.Lock()
			q.children--
			q.childrenMetric.Add(-1)
			q.used -= memory
			q.bytesMetric.Decrease(memory)
			q.mu.Unlock()
		})
	}, nil
}

func (q *Budget) reserve(n uint64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n > q.maximum-q.used {
		return false
	}
	q.used += n
	q.bytesMetric.Increase(n)
	return true
}
func (q *Budget) release(n uint64) {
	q.mu.Lock()
	q.used -= n
	q.bytesMetric.Decrease(n)
	q.mu.Unlock()
}
func (q *Budget) HoldControl() (func(), error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.channels >= 1024 || 16<<10 > q.maximum-q.used {
		return nil, errors.New("route control capacity unavailable")
	}
	q.channels++
	q.channelsMetric.Add(1)
	q.used += 16 << 10
	q.bytesMetric.Add(16 << 10)
	var once sync.Once
	return func() {
		once.Do(func() {
			q.mu.Lock()
			q.channels--
			q.channelsMetric.Add(-1)
			q.used -= 16 << 10
			q.bytesMetric.Add(-(16 << 10))
			q.mu.Unlock()
		})
	}, nil
}
func (q *Budget) child() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.children >= 1024 {
		return false
	}
	q.children++
	q.childrenMetric.Add(1)
	return true
}
func (q *Budget) releaseChild() {
	q.mu.Lock()
	q.children--
	q.childrenMetric.Add(-1)
	q.mu.Unlock()
}
