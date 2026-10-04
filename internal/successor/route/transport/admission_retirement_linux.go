//go:build linux

package transport

import (
	"errors"
	"sync"
)

// admissionRetirement retains capacity even when receiving Admission refuses
// after reservation or spend. Admission can request rollback synchronously;
// the Route owner completes it only after its physical borrowers have joined.
type admissionRetirement struct {
	mu      sync.Mutex
	joined  bool
	returns []*reservationReturn
}
type reservationReturn struct {
	owner     *admissionRetirement
	release   func() error
	requested bool
	once      sync.Once
	err       error
}

// HoldReservation gives receiving Admission an exactly-once release callback
// while retaining its physical capacity with this authenticated Route channel.
// Composition must use it for every Hosting reservation returned to Accept.
func (c Channel) HoldReservation(release func() error) func() error {
	if c.capacity == nil || release == nil {
		return func() error { return errors.New("route capacity lifetime absent") }
	}
	r := &reservationReturn{owner: c.capacity, release: release}
	c.capacity.mu.Lock()
	c.capacity.returns = append(c.capacity.returns, r)
	c.capacity.mu.Unlock()
	return r.request
}

func (r *reservationReturn) request() error {
	r.owner.mu.Lock()
	r.requested = true
	joined := r.owner.joined
	r.owner.mu.Unlock()
	if joined {
		r.once.Do(func() { r.err = r.release() })
		return r.err
	}
	return nil
}

func (o *admissionRetirement) finish() error {
	o.mu.Lock()
	o.joined = true
	var pending []*reservationReturn
	for _, r := range o.returns {
		if r.requested {
			pending = append(pending, r)
		}
	}
	o.mu.Unlock()
	var err error
	for _, r := range pending {
		err = errors.Join(err, r.request())
	}
	return err
}
