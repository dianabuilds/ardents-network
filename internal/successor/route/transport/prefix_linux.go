//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// PrefixConfig retains the selected leg and absolute bounds. Present performs
// genuine Stock presentation. Release returns the local Hosting reservation
// only after this owner has joined the complete physical tree.
type PrefixConfig struct {
	Leg      selection.Leg
	Current  func() (network.RuntimeView, error)
	Present  Present
	Deadline time.Time
	Release  func() error
}

// Prefix is a bounded Entry/Interior transport. It carries no Service data or
// fake terminal workload. Its accepted result means both exact roles admitted
// fresh authenticated channels; Close retains one joined terminal result.
type Prefix struct {
	entry, interior    *session
	child              *lane
	ctx                context.Context
	cancel             context.CancelFunc
	done               chan struct{}
	closing            chan struct{}
	once               sync.Once
	err                error
	release            func() error
	config             PrefixConfig
	registrationMu     sync.Mutex
	registrations      map[*Registration]struct{}
	registrationSetups map[*registrationOpening]struct{}
	openings           sync.WaitGroup
	activity           chan struct{}
}

func OpenPrefix(ctx context.Context, config PrefixConfig) (_ *Prefix, result error) {
	if ctx == nil || config.Current == nil || config.Present == nil || config.Release == nil || config.Deadline.IsZero() || config.Deadline != config.Deadline.UTC().Truncate(time.Second) ||
		!time.Now().Before(config.Deadline) || config.Deadline.After(config.Leg.NotAfter) || config.Deadline.After(time.Now().Add(admission.ForwardClass.Lifetime())) {
		return nil, errors.New("route prefix composition or deadline invalid")
	}
	check := func() error {
		v, err := config.Current()
		if err != nil {
			return err
		}
		return config.Leg.Check(v, time.Now())
	}
	if err := check(); err != nil {
		return nil, err
	}
	childContext, cancel := context.WithDeadline(ctx, config.Deadline)
	p := &Prefix{ctx: childContext, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), release: config.Release, config: config, registrations: make(map[*Registration]struct{}), registrationSetups: make(map[*registrationOpening]struct{}), activity: make(chan struct{}, 1)}
	// The local reservation transfers at this point, including failed setup.
	defer func() {
		if result != nil {
			result = errors.Join(result, p.closeOpening())
		}
	}()
	queues := &queueBudget{maximum: 4 << 20}
	entryControl, err := queues.channel()
	if err != nil {
		return nil, err
	}
	interiorControl, err := queues.channel()
	if err != nil {
		entryControl()
		return nil, err
	}
	p.release = func() error { entryControl(); interiorControl(); return config.Release() }
	a := Authority{Current: config.Current, Duty: config.Leg.Entry, Profile: config.Leg.Profile}
	m, err := a.member()
	if err != nil {
		return nil, err
	}
	conn, err := carrier.OpenClosedRoleCarrier(childContext, carrier.ClosedRoleCarrierRequest{CarrierProfile: carrier.CarrierProfile(m.CarrierProfile), Endpoint: m.Endpoint, ExpectedServer: m.PublicKey, Deadline: minDeadline(config.Deadline, time.Now().Add(10*time.Second))})
	if err != nil {
		return nil, err
	}
	conn = &retiredConn{Conn: conn}
	// Cancellation interrupts setup even before a session reader exists.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(childContext, func() { defer close(interrupted); _ = conn.SetDeadline(time.Now()); _ = conn.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	if err := conn.SetDeadline(minDeadline(config.Deadline, time.Now().Add(10*time.Second))); err != nil {
		_ = conn.Close()
		return nil, err
	}
	h, err := freshHello(a, config.Deadline)
	if err == nil {
		err = presentChannel(childContext, ctx, conn, a, h, config.Present)
	}
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	if err := check(); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	if err := conn.SetDeadline(config.Deadline); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	p.entry = newSession(childContext, conn, config.Deadline, admission.ForwardClass.ByteLimit()-admissionWireBytes, check, false, queues, nil)
	opened := ardpHello{RecipientNodeID: config.Leg.InteriorMember.NodeID, RecipientDutyGeneration: config.Leg.InteriorMember.DutyGeneration, Purpose: 7, Deadline: config.Deadline}
	p.child, err = p.entry.openLane(childContext, encodeOpen(opened, false))
	if err != nil {
		return nil, err
	}
	a = Authority{Current: config.Current, Duty: config.Leg.Interior, Profile: config.Leg.Profile}
	m, err = a.member()
	if err != nil {
		return nil, err
	}
	secured, err := carrier.OpenClosedRoleTLS(childContext, p.child, m.PublicKey, minDeadline(config.Deadline, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	h, err = freshHello(a, config.Deadline)
	if err == nil {
		err = presentChannel(childContext, ctx, secured, a, h, config.Present)
	}
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := childContext.Err(); err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(config.Deadline); err != nil {
		return nil, err
	}
	p.interior = newSession(childContext, &retiredConn{Conn: secured}, config.Deadline, admission.ForwardClass.ByteLimit()-admissionWireBytes, check, false, queues, nil)
	if err := check(); err != nil {
		return nil, err
	}
	if err := childContext.Err(); err != nil {
		return nil, err
	}
	go p.watch(check, nil)
	return p, nil
}

func (p *Prefix) watch(check func() error, idleEvents <-chan time.Time) {
	defer close(p.done)
	idle := time.NewTimer(120 * time.Second)
	defer idle.Stop()
	if idleEvents == nil {
		idleEvents = idle.C
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.activity:
			idle.Reset(120 * time.Second)
		case <-idleEvents:
			p.registrationMu.Lock()
			busy := false
			for registration := range p.registrations {
				registration.mu.Lock()
				busy = busy || !registration.stopped
				registration.mu.Unlock()
			}
			p.registrationMu.Unlock()
			if busy {
				idle.Reset(120 * time.Second)
				continue
			}
			err := errors.New("route prefix idle readiness expired")
			p.interior.retire(err)
			p.entry.retire(err)
			p.cancel()
			return
		case <-p.ctx.Done():
			select {
			case <-p.closing:
				return
			default:
			}
			p.interior.retire(p.ctx.Err())
			p.entry.retire(p.ctx.Err())
			return
		case <-p.entry.readerDone:
			p.cancel()
			return
		case <-p.interior.readerDone:
			p.cancel()
			return
		case <-ticker.C:
			if err := check(); err != nil {
				p.entry.retire(err)
				p.cancel()
				return
			}
		}
	}
}

func (p *Prefix) closeOpening() error {
	p.registrationMu.Lock()
	for opening := range p.registrationSetups {
		opening.cancel()
	}
	registrations := make([]*Registration, 0, len(p.registrations))
	for registration := range p.registrations {
		registrations = append(registrations, registration)
		registration.stop(nil)
	}
	p.registrationMu.Unlock()
	// Terminal borrowers need their still-live framing parents to emit their
	// bounded CLOSE and join. Seal/cancel every terminal acquisition first;
	// only retire the parents after those original borrowers have joined.
	p.openings.Wait()
	var joined error
	for _, registration := range registrations {
		joined = errors.Join(joined, registration.Close())
	}
	// Retire the inner reader before interrupting its lower framing owner, so
	// our own lower closure cannot become an unexplained inner transport EOF.
	if p.interior != nil {
		p.interior.retire(nil)
	}
	if p.entry != nil {
		p.entry.retire(nil)
	}
	p.cancel()
	if p.interior != nil {
		joined = errors.Join(joined, p.interior.Close())
	}
	if p.entry != nil {
		joined = errors.Join(joined, p.entry.Close())
	}
	if p.release != nil {
		joined = errors.Join(joined, p.release())
	}
	return joined
}

// Done closes when retained readiness retires. It is not a completed physical
// join: the work owner must call Close before releasing its other roots.
func (p *Prefix) Done() <-chan struct{} { return p.done }
func (p *Prefix) Close() error {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		p.registrationMu.Lock()
		close(p.closing)
		p.registrationMu.Unlock()
		p.err = p.closeOpening()
		<-p.done
	})
	return p.err
}

var _ net.Conn = (*lane)(nil)
