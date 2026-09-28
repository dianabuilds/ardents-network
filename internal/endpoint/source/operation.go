//go:build linux

package source

import (
	"context"
	"errors"
	"sync"
)

// operationGate serializes Source opening and issuance over the entire Context
// lifetime. It is never reset when a prefix is replaced or stopped.
type operationGate struct {
	once sync.Once
	busy chan struct{}
}

func (gate *operationGate) acquire(ctx, lease context.Context) (func(), error) {
	gate.once.Do(func() { gate.busy = make(chan struct{}, 1) })
	select {
	case gate.busy <- struct{}{}:
		if ctx.Err() != nil || lease.Err() != nil {
			<-gate.busy
			return nil, errors.New("text Source operation cancelled")
		}
		return func() { <-gate.busy }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lease.Done():
		return nil, lease.Err()
	}
}

// AcquireOperation reserves actual Source opening or issuance. The caller must
// first validate Context authority and supply its independently revocable lease.
// The successful caller must invoke the returned release function exactly once.
func (lifecycle *Lifecycle) AcquireOperation(ctx, lease context.Context) (func(), error) {
	return lifecycle.operation.acquire(ctx, lease)
}
