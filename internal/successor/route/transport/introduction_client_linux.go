//go:build linux

package transport

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// RegistrationConfig retains the selected exact delivery duty, transport
// revision and original expiry. It grants no Publication or Service authority.
type RegistrationConfig struct {
	Duty     network.RetainedDuty
	Revision uint64
	Deadline time.Time
}

// registrationOpening retains only cancellation of the original terminal
// setup. Prefix retirement can interrupt/join it without closing its parents.
type registrationOpening struct{ cancel context.CancelFunc }

// Registration is a real channel-owned receiving slot. One reader, one owning
// withdrawal and physical termination share the immutable original lifetime.
// Close joins every borrower and retains the same result on repeated calls.
type Registration struct {
	prefix                                          *Prefix
	conn                                            net.Conn
	lane                                            *lane
	request                                         introduction.Request
	ctx                                             context.Context
	cancel                                          context.CancelFunc
	check                                           func() error
	release                                         func()
	joinCaller                                      func()
	mu                                              sync.Mutex
	pending                                         [32]byte
	withdrawing, stopped                            bool
	failure, physicalErr, writeErr, result          error
	reply                                           chan error
	readerDone, watcherDone, physicalDone, retiring chan struct{}
	writes                                          sync.WaitGroup
	once                                            sync.Once
}

// Register reaches a selected Introduction duty through this retained Domain4
// Entry/Interior prefix and presents genuine class3 Stock on fresh inner TLS.
func (p *Prefix) Register(ctx context.Context, config RegistrationConfig) (_ *Registration, result error) {
	if p == nil || ctx == nil || config.Revision == 0 || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || !time.Now().Before(config.Deadline) || config.Deadline.After(time.Now().Add(admission.RegistrationClass.Lifetime())) {
		return nil, errors.New("route registration bounds invalid")
	}
	if end, exists := ctx.Deadline(); exists && config.Deadline.After(end) {
		return nil, errors.New("route registration exceeds caller bound")
	}
	p.registrationMu.Lock()
	select {
	case <-p.closing:
		p.registrationMu.Unlock()
		return nil, net.ErrClosed
	default:
	}
	if p.ctx.Err() != nil {
		p.registrationMu.Unlock()
		return nil, errors.New("route registration prefix unavailable")
	}
	child, cancel := context.WithDeadline(p.ctx, config.Deadline)
	opening := &registrationOpening{cancel: cancel}
	p.registrationSetups[opening] = struct{}{}
	p.openings.Add(1)
	p.registrationMu.Unlock()
	defer func() {
		if result != nil {
			cancel()
		}
		p.registrationMu.Lock()
		delete(p.registrationSetups, opening)
		p.registrationMu.Unlock()
		p.openings.Done()
	}()
	a := Authority{Current: p.config.Current, Duty: config.Duty, Profile: p.config.Leg.Profile}
	check := func() error {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(config.Deadline) {
			return errors.New("route registration expired")
		}
		view, err := p.config.Current()
		if err != nil {
			return err
		}
		if err := p.config.Leg.Check(view, time.Now()); err != nil {
			return err
		}
		member, err := a.member()
		if err != nil || member.RoleDomain != 4 || member.Subrole != 3 || p.config.Leg.EntryMember.RoleDomain != 4 || config.Deadline.After(p.config.Deadline) || config.Deadline.After(member.NotAfter()) || config.Deadline.After(a.Profile.NotAfter) || conflicting(member, p.config.Leg.EntryMember) || conflicting(member, p.config.Leg.InteriorMember) {
			return errors.Join(errors.New("route registration duty differs"), err)
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	release, err := p.interior.queues.channel()
	if err != nil {
		return nil, err
	}
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() {
		defer close(callerDone)
		cancel()
	})
	var callerJoin sync.Once
	joinCaller := func() {
		callerJoin.Do(func() {
			if !stopCaller() {
				<-callerDone
			}
		})
	}
	var raw *lane
	var secured net.Conn
	interrupted := make(chan struct{})
	stop := func() bool { return true }
	defer func() {
		if result != nil {
			cancel()
			if !stop() {
				<-interrupted
			}
			if secured != nil {
				result = errors.Join(result, secured.Close())
			}
			if raw != nil {
				result = errors.Join(result, raw.Close())
				raw.finish()
			}
			release()
		}
		if result != nil {
			joinCaller()
		}
	}()
	member, err := a.member()
	if err != nil {
		return nil, err
	}
	raw, err = p.interior.openLane(child, encodeOpen(ardpHello{RecipientNodeID: member.NodeID, RecipientDutyGeneration: member.DutyGeneration, Purpose: 4, Deadline: config.Deadline}, false))
	if err != nil {
		return nil, err
	}
	stop = context.AfterFunc(child, func() { defer close(interrupted); _ = raw.Close() })
	tls, err := carrier.OpenClosedRoleTLS(child, raw, member.PublicKey, minDeadline(config.Deadline, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	secured = &retiredConn{Conn: tls}
	if err := secured.SetDeadline(minDeadline(config.Deadline, time.Now().Add(10*time.Second))); err != nil {
		return nil, err
	}
	hello, err := freshPurposeHello(a, config.Deadline, ardp.PurposeIntroduction, false)
	if err == nil {
		err = presentChannel(child, secured, a, hello, p.config.Present)
	}
	if err != nil {
		return nil, err
	}
	if err := secured.SetDeadline(config.Deadline); err != nil {
		return nil, err
	}
	request := introduction.Request{Revision: config.Revision, Expiry: config.Deadline}
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(request.Slot[:]); err != nil {
		return nil, err
	}
	body, err := introduction.EncodeRequest(request)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(secured)
	if err != nil {
		return nil, err
	}
	status, err := introduction.DecodeResult(frame.Body, request.Nonce)
	if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 0 || status != 0 {
		return nil, errors.Join(errors.New("route registration refused"), err)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := child.Err(); err != nil {
		return nil, err
	}
	if !stop() {
		<-interrupted
		return nil, child.Err()
	}
	r := &Registration{prefix: p, conn: secured, lane: raw, request: request, ctx: child, cancel: cancel, check: check, release: release, joinCaller: joinCaller, reply: make(chan error, 1), readerDone: make(chan struct{}), watcherDone: make(chan struct{}), physicalDone: make(chan struct{}), retiring: make(chan struct{})}
	context.AfterFunc(child, func() {
		defer close(r.physicalDone)
		err := r.conn.Close()
		r.mu.Lock()
		r.physicalErr = err
		r.mu.Unlock()
	})
	go r.read()
	go r.watch()
	p.registrationMu.Lock()
	closing := false
	select {
	case <-p.closing:
		closing = true
	default:
	}
	if !closing && p.ctx.Err() == nil {
		p.registrations[r] = struct{}{}
	}
	p.registrationMu.Unlock()
	if closing {
		return nil, errors.Join(net.ErrClosed, r.Close())
	}
	if err := child.Err(); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	p.changedActivity()
	return r, nil
}

func (p *Prefix) changedActivity() {
	select {
	case p.activity <- struct{}{}:
	default:
	}
}

func (r *Registration) stop(cause error) {
	r.mu.Lock()
	if !r.stopped {
		r.stopped = true
		r.failure = cause
		close(r.retiring)
	}
	r.mu.Unlock()
	r.cancel()
	r.prefix.changedActivity()
}

func (r *Registration) watch() {
	defer close(r.watcherDone)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			if err := r.check(); err != nil {
				r.stop(err)
				return
			}
		}
	}
}

func (r *Registration) read() {
	defer close(r.readerDone)
	frame, err := ardp.ReadFrame(r.conn)
	if err != nil {
		r.stop(err)
		return
	}
	r.mu.Lock()
	nonce, withdrawing := r.pending, r.withdrawing
	r.mu.Unlock()
	status, err := introduction.DecodeResult(frame.Body, nonce)
	if !withdrawing || frame.Kind != ardp.KindResult || frame.Lane != 0 || status != 0 || err != nil {
		err = errors.Join(errors.New("route withdrawal result differs"), err)
	}
	if err == nil {
		err = r.check()
	}
	r.reply <- err
	r.stop(err)
}

// Withdraw sends one fresh owning request and joins physical retirement before
// returning the original result. Cancellation never authorizes another attempt.
func (r *Registration) Withdraw(ctx context.Context) error {
	if r == nil || ctx == nil {
		return errors.New("route registration unavailable")
	}
	r.mu.Lock()
	if r.stopped || r.withdrawing {
		r.mu.Unlock()
		return errors.New("route withdrawal unavailable")
	}
	r.withdrawing = true
	request := introduction.Request{Slot: r.request.Slot, Revision: r.request.Revision, Withdraw: true}
	_, err := rand.Read(request.Nonce[:])
	r.pending = request.Nonce
	r.writes.Add(1)
	r.mu.Unlock()
	stopCaller := context.AfterFunc(ctx, func() { r.stop(ctx.Err()) })
	defer stopCaller()
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = r.check()
	}
	if err == nil {
		var body []byte
		body, err = introduction.EncodeRequest(request)
		if err == nil {
			err = ardp.WriteFrame(r.conn, ardp.Frame{Kind: ardp.KindOperation, Body: body})
		}
	}
	if err != nil {
		r.mu.Lock()
		r.writeErr = errors.Join(r.writeErr, err)
		r.mu.Unlock()
	}
	r.writes.Done()
	if err != nil {
		r.stop(err)
		return errors.Join(err, r.Close())
	}
	select {
	case err = <-r.reply:
	case <-r.ctx.Done():
		select {
		case err = <-r.reply:
		default:
			err = r.ctx.Err()
		}
	}
	return errors.Join(err, r.Close())
}

// Done signals retirement. The work owner must call Close to retrieve the
// joined result before releasing its roots or declaring withdrawal completed.
func (r *Registration) Done() <-chan struct{} { return r.retiring }

// Slot is the opaque exact receiving slot. Possession of this transport result
// does not authorize publication or make a Service ready.
func (r *Registration) Slot() [32]byte { return r.request.Slot }
func (r *Registration) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		r.stop(nil)
		r.joinCaller()
		<-r.readerDone
		<-r.watcherDone
		<-r.physicalDone
		r.writes.Wait()
		r.lane.finish()
		r.release()
		r.mu.Lock()
		r.result = errors.Join(r.failure, r.physicalErr, r.writeErr)
		r.mu.Unlock()
		r.prefix.registrationMu.Lock()
		delete(r.prefix.registrations, r)
		r.prefix.registrationMu.Unlock()
		r.prefix.changedActivity()
	})
	return r.result
}
