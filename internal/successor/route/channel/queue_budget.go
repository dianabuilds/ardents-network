package channel

import (
	"errors"
	"sync"
)

// queueBudget belongs to one physical Route principal, not one connection.
// Control reservations and retained payload share its finite aggregate cap.
type Budget struct {
	mu                 sync.Mutex
	used, maximum      uint64
	channels, children uint32
}

// NewBudget creates the finite memory owner shared by one physical principal.
func NewBudget(maximum uint64) *Budget { return &Budget{maximum: maximum} }

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
	q.used += memory
	q.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			q.mu.Lock()
			q.children--
			q.used -= memory
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
	return true
}
func (q *Budget) release(n uint64) { q.mu.Lock(); q.used -= n; q.mu.Unlock() }
func (q *Budget) HoldControl() (func(), error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.channels >= 1024 || 16<<10 > q.maximum-q.used {
		return nil, errors.New("route control capacity unavailable")
	}
	q.channels++
	q.used += 16 << 10
	var once sync.Once
	return func() { once.Do(func() { q.mu.Lock(); q.channels--; q.used -= 16 << 10; q.mu.Unlock() }) }, nil
}
func (q *Budget) child() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.children >= 1024 {
		return false
	}
	q.children++
	return true
}
func (q *Budget) releaseChild() { q.mu.Lock(); q.children--; q.mu.Unlock() }
