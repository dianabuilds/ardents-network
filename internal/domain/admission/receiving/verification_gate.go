package receiving

import (
	"errors"
	"sync"
	"time"
)

// VerificationGate bounds expensive token checks for one receiver duty.
// A backwards clock cannot reset the per-second allowance.
type VerificationGate struct {
	mu              sync.Mutex
	window          time.Time
	started, active uint16
}

func (gate *VerificationGate) Begin(now time.Time) (func(), error) {
	if gate == nil || now.IsZero() {
		return nil, errors.New("verification unavailable")
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	now = now.UTC().Truncate(time.Second)
	if now.Before(gate.window) {
		return nil, errors.New("verification time regressed")
	}
	if now.After(gate.window) {
		gate.window, gate.started = now, 0
	}
	if gate.started >= 128 || gate.active >= 4 {
		return nil, errors.New("verification capacity exhausted")
	}
	gate.started++
	gate.active++
	var once sync.Once
	return func() { once.Do(func() { gate.mu.Lock(); gate.active--; gate.mu.Unlock() }) }, nil
}
