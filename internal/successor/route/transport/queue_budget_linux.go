//go:build linux

package transport

import (
	"errors"
	"sync"
)

// queueBudget belongs to one physical Route principal, not one connection.
// Control reservations and retained payload share its finite aggregate cap.
type queueBudget struct {
	mu                 sync.Mutex
	used, maximum      uint64
	channels, children uint32
}

func (q *queueBudget) reserve(n uint64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n > q.maximum-q.used {
		return false
	}
	q.used += n
	return true
}
func (q *queueBudget) release(n uint64) { q.mu.Lock(); q.used -= n; q.mu.Unlock() }
func (q *queueBudget) channel() (func(), error) {
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
func (q *queueBudget) child() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.children >= 1024 {
		return false
	}
	q.children++
	return true
}
func (q *queueBudget) releaseChild() { q.mu.Lock(); q.children--; q.mu.Unlock() }
