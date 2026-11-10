//go:build linux

package receiver

import (
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// admissionRetirement retains capacity even when receiving Admission refuses
// after reservation or spend. Admission can request rollback synchronously;
// the Route owner completes it only after its physical borrowers have joined.
type admissionRetirement struct {
	mu             sync.Mutex
	joined         bool
	returns        []*reservationReturn
	registry       *introduction.Registry
	registration   *introduction.Capacity
	joinQueues     *framing.Budget
	join           *join.Capacity
	issuerTerminal func() error
}

// HoldIssuerReservation keeps the separately owned, original termination
// share until CLOSE joins. Work returns once payload I/O is joined, while
// Admission still receives only its original one-shot release callback.
func (c Channel) HoldIssuerReservation(work, terminal func() error) (func() error, error) {
	returnBoth := func() (err error) {
		if work != nil {
			err = work()
		}
		if terminal != nil {
			err = errors.Join(err, terminal())
		}
		return err
	}
	if c.Hello.Purpose != ardp.PurposeIssuer || c.capacity == nil || work == nil || terminal == nil {
		release, err := c.HoldReservation(returnBoth)
		return release, errors.Join(err, errors.New("route Issuer termination capacity unavailable"))
	}
	c.capacity.mu.Lock()
	if c.capacity.issuerTerminal != nil {
		c.capacity.mu.Unlock()
		release, err := c.HoldReservation(returnBoth)
		return release, errors.Join(err, errors.New("route Issuer termination capacity already held"))
	}
	c.capacity.issuerTerminal = terminal
	c.capacity.mu.Unlock()
	return c.HoldReservation(work)
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
func (c Channel) HoldReservation(release func() error) (func() error, error) {
	if c.capacity == nil || release == nil {
		return release, errors.New("route capacity lifetime absent")
	}
	r := &reservationReturn{owner: c.capacity, release: release}
	c.capacity.mu.Lock()
	defer c.capacity.mu.Unlock()
	c.capacity.returns = append(c.capacity.returns, r)
	if c.Hello.Purpose == ardp.PurposeIssuer && c.capacity.issuerTerminal == nil {
		return r.request, errors.New("route Issuer termination capacity unavailable")
	}
	if c.Hello.Purpose == ardp.PurposeDataJoin {
		if c.capacity.join != nil {
			return r.request, errors.New("route JOIN capacity already held")
		}
		position, err := join.ReserveCapacity(c.capacity.joinQueues)
		if err != nil {
			return r.request, err
		}
		c.capacity.join = position
		r.release = func() error { position.Release(); return release() }
	}
	if c.Hello.Purpose == ardp.PurposeIntroduction && c.capacity.registry != nil {
		if c.capacity.registration != nil {
			return r.request, errors.New("route registration capacity already held")
		}
		position, err := c.capacity.registry.Reserve(time.Now())
		if err != nil {
			return r.request, err
		}
		c.capacity.registration = position
		r.release = func() error { position.Release(); return release() }
	}
	return r.request, nil
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
	// Additional reservations borrow the original termination coverage.
	// Return them first, after physical join, before their original owner.
	for i := len(o.returns) - 1; i >= 0; i-- {
		r := o.returns[i]
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
