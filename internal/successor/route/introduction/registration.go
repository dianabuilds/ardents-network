package introduction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// RegistrationConfig retains the selected exact delivery duty, transport
// revision and original expiry. It grants no Publication or Service authority.
type RegistrationConfig struct {
	Duty     network.RetainedDuty
	Revision uint64
	Deadline time.Time
}

// HolderRegistration retains the holder side of one channel-owned slot.
// Its reader, owning
// withdrawal and physical termination share the immutable original lifetime.
// Close joins every borrower and retains the same result on repeated calls.
type HolderRegistration struct {
	borrow   *prefix.Borrow
	conn     net.Conn
	terminal *prefix.IntroductionChannel
	request  Request
	receipt  Receipt
	ctx      context.Context
	cancel   context.CancelFunc
	check    func() error

	mu                                              sync.Mutex
	pending                                         [32]byte
	withdrawing, stopped                            bool
	failure, physicalErr, withdrawErr, result       error
	reply                                           chan error
	readerDone, watcherDone, physicalDone, retiring chan struct{}
	withdrawal                                      sync.WaitGroup
	once                                            sync.Once
}

// Register reaches a selected Introduction duty through this retained Domain4
// Entry/Interior prefix and presents genuine class3 Stock on fresh inner TLS.
func Register(ctx context.Context, p *prefix.Prefix, config RegistrationConfig) (_ *HolderRegistration, result error) {
	if p == nil || ctx == nil || config.Revision == 0 || config.Deadline != config.Deadline.UTC().Truncate(time.Second) || !time.Now().Before(config.Deadline) || config.Deadline.After(time.Now().Add(admission.RegistrationClass.Lifetime())) {
		return nil, errors.New("route registration bounds invalid")
	}
	if end, exists := ctx.Deadline(); exists && config.Deadline.After(end) {
		return nil, errors.New("route registration exceeds caller bound")
	}
	t, err := p.OpenIntroductionChannel(ctx, config.Duty, config.Deadline)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, t.CloseSetup())
		}
		t.FinishSetup()
	}()
	child, cancel, check := t.Context(), t.Cancel, t.Check
	secured := t.Stream()
	created := time.Now()
	request := Request{Revision: config.Revision, Expiry: config.Deadline}
	if _, err := rand.Read(request.Nonce[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(request.Slot[:]); err != nil {
		return nil, err
	}
	body, err := EncodeRequest(request)
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
	status, err := DecodeResult(frame.Body, request.Nonce)
	if err != nil || frame.Kind != ardp.KindResult || frame.Lane != 0 || status != 0 {
		return nil, errors.Join(errors.New("route registration refused"), err)
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), child.Err()); err != nil {
		return nil, err
	}
	if err := t.StopOpening(); err != nil {
		return nil, err
	}
	r := &HolderRegistration{conn: secured, terminal: t, request: request, ctx: child, cancel: cancel, check: check, reply: make(chan error, 1), readerDone: make(chan struct{}), watcherDone: make(chan struct{}), physicalDone: make(chan struct{}), retiring: make(chan struct{})}
	r.receipt = Receipt{owner: r, facts: RegistrationFacts{Network: config.Duty.Epoch.Network, Profile: t.ProfileDigest(), Node: config.Duty.NodeID, Slot: request.Slot, Revision: request.Revision, Created: created, Expiry: request.Expiry, Acknowledgement: sha256.Sum256(frame.Body)}}
	r.borrow = t.Borrow(func() { r.stop(nil) }, r.Close, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return !r.stopped
	})
	context.AfterFunc(child, func() {
		defer close(r.physicalDone)
		err := r.conn.Close()
		r.mu.Lock()
		r.physicalErr = err
		r.mu.Unlock()
	})
	go r.read()
	go r.watch()
	if err := t.Publish(r.borrow); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	if err := errors.Join(ctx.Err(), child.Err()); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

func (r *HolderRegistration) stop(cause error) {
	r.mu.Lock()
	if !r.stopped {
		r.stopped = true
		r.failure = cause
		close(r.retiring)
	}
	r.mu.Unlock()
	r.cancel()
	r.borrow.ChangedActivity()
}

func (r *HolderRegistration) watch() {
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

func (r *HolderRegistration) read() {
	defer close(r.readerDone)
	frame, err := ardp.ReadFrame(r.conn)
	if err != nil {
		r.stop(err)
		return
	}
	r.mu.Lock()
	nonce, withdrawing := r.pending, r.withdrawing
	r.mu.Unlock()
	status, err := DecodeResult(frame.Body, nonce)
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
func (r *HolderRegistration) Withdraw(ctx context.Context) error {
	if r == nil || ctx == nil {
		return errors.New("route registration unavailable")
	}
	r.mu.Lock()
	if r.stopped || r.withdrawing {
		r.mu.Unlock()
		return errors.New("route withdrawal unavailable")
	}
	r.withdrawing = true
	request := Request{Slot: r.request.Slot, Revision: r.request.Revision, Withdraw: true}
	_, err := rand.Read(request.Nonce[:])
	r.pending = request.Nonce
	r.withdrawal.Add(1)
	r.mu.Unlock()
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() {
		defer close(callerDone)
		r.stop(ctx.Err())
	})
	// Completion joins cancellation and retains the original caller result
	// before publishing physical completion. A concurrent Close waits for this
	// entire withdrawal, including ACK and caller handoff, not just its write.
	finish := func(err error) error {
		if !stopCaller() {
			<-callerDone
		}
		err = errors.Join(err, ctx.Err())
		r.mu.Lock()
		r.withdrawErr = errors.Join(r.withdrawErr, err)
		r.mu.Unlock()
		if err != nil {
			r.stop(err)
		}
		r.withdrawal.Done()
		return errors.Join(err, r.Close())
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = errors.Join(r.check(), ctx.Err())
	}
	if err == nil {
		var body []byte
		body, err = EncodeRequest(request)
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = ardp.WriteFrame(r.conn, ardp.Frame{Kind: ardp.KindOperation, Body: body})
		}
		if err == nil {
			err = ctx.Err()
		}
	}
	if err != nil {
		return finish(err)
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
	return finish(err)
}

// Done signals retirement. The work owner must call Close to retrieve the
// joined result before releasing its roots or declaring withdrawal completed.
func (r *HolderRegistration) Done() <-chan struct{} { return r.retiring }

// Slot is the opaque exact receiving slot. Possession of this transport result
// does not authorize publication or make a Service ready.
func (r *HolderRegistration) Slot() [32]byte { return r.request.Slot }
func (r *HolderRegistration) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		r.stop(nil)
		r.terminal.JoinCaller()
		<-r.readerDone
		<-r.watcherDone
		<-r.physicalDone
		r.withdrawal.Wait()
		r.terminal.FinishParent()
		r.mu.Lock()
		r.result = errors.Join(r.failure, r.physicalErr, r.withdrawErr)
		r.mu.Unlock()
		r.borrow.ReturnJoined()
	})
	return r.result
}
