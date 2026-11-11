package systemd

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type requestObservationKey struct{}

type requestObservation struct {
	client *startupBus
	active atomic.Bool
}

// observeRequests shares only the physical deadline and cancellation hook of
// one synchronous read-only observation. Every request still reaches the original
// bus and validates its fresh reply. The marker lends no accepting facts and
// cannot be reused by another client or after this original observation joins.
func (client *startupBus) observeRequests(ctx context.Context, read func(context.Context) error) (returned error) {
	if ctx == nil || read == nil || client == nil || client.transport == nil || client.transport.socket == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	end, _ := bounded.Deadline()
	if err := client.transport.socket.SetDeadline(end); err != nil {
		return client.callFailure(bounded, err)
	}
	frame := &requestObservation{client: client}
	frame.active.Store(true)
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() { _ = client.interrupt(); close(interrupted) })
	defer func() {
		frame.active.Store(false)
		if !stop() {
			<-interrupted
		}
		if err := errors.Join(bounded.Err(), ctx.Err()); err != nil {
			if returned == nil {
				returned = err
			} else {
				returned = errors.Join(returned, err)
			}
		}
	}()
	return read(context.WithValue(bounded, requestObservationKey{}, frame))
}
