package issuer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
)

// Owner retains one original caller and quota/key/result lifetime. Copies share
// the same terminal latch. Route separately retains its physical work slot and
// must join its users before calling Close; Admission owns no transport.
type Owner struct{ state *issuingLifetime }

type issuingLifetime struct {
	mu          sync.Mutex
	users       sync.WaitGroup
	caller      context.Context
	ctx         context.Context
	cancel      context.CancelFunc
	binding     quota.LedgerBinding
	initial     admission.AuthorityFacts
	openedAt    time.Time
	lastIssueAt time.Time
	observe     func() (admission.AuthorityFacts, time.Time, error)
	ledger      *quota.Ledger
	keys        issuance.Store
	results     issuance.ResultStore
	active      chan struct{}
	denied      bool
	first       Result
	failure     error
	cleanup     error
	closing     bool
	done        chan struct{}
	terminal    Result
}

// OpenCurrent acquires prepared quota, immutable keys and results in that order.
// It never initializes, generates keys or accepts caller-declared authority.
// The observer must check the original external owners, outside their locks.
func OpenCurrent(ctx context.Context, p Plan, observe func() (admission.AuthorityFacts, time.Time, error)) (Owner, Result) {
	r := Result{Phase: "input", Outcome: "invalid-input"}
	if ctx == nil || observe == nil || !p.valid() {
		return Owner{}, r
	}
	if !issuance.Supported() {
		r.Outcome = "unsupported-platform"
		return Owner{}, r
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return Owner{}, r
	}
	initial, now, err := observe()
	if err != nil || initial.ValidateAt(now) != nil {
		return Owner{}, Result{Phase: "authority", Outcome: "authority-unavailable"}
	}
	if !matchesAuthority(p.AdmissionBinding, initial) {
		return Owner{}, Result{Phase: "authority", Outcome: "binding-mismatch"}
	}
	// Defensive binding: neither the caller's keys nor its Plan remain live input.
	b := p.AdmissionBinding
	b.Keys = append(b.Keys[:0:0], b.Keys...)
	for i := range b.Keys {
		b.Keys[i].SPKI = append([]byte(nil), b.Keys[i].SPKI...)
	}
	s := &issuingLifetime{caller: ctx, binding: b, initial: initial, openedAt: now, lastIssueAt: now, observe: observe, done: make(chan struct{})}
	s.ctx, s.cancel = context.WithCancel(ctx)
	r.Phase = "open-admission"
	s.ledger, err = quota.Open(p.AdmissionRoot, b)
	if err == nil {
		r.Phase = "open-keys"
		s.keys, err = issuance.Open(s.ctx, p.KeyRoot, p.KeyBinding)
		if err == nil {
			r.Phase = "open-results"
			s.results, err = issuance.OpenResults(s.ctx, p.ResultRoot, s.keys, b)
		}
	}
	if err != nil {
		r.Outcome = category(err)
		s.cancel()
		s.denied, s.closing = true, true
		s.failure = err
		s.first = r
		closeOwned := func(close func() error) func() error {
			return func() error { err := close(); s.cleanup = errors.Join(s.cleanup, err); return err }
		}
		if s.results != (issuance.ResultStore{}) {
			r.closeOwner(0, "close-results", closeOwned(s.results.Close))
		}
		if s.keys != (issuance.Store{}) {
			r.closeOwner(1, "close-keys", closeOwned(s.keys.Close))
		}
		if s.ledger != nil {
			r.closeOwner(2, "close-admission", closeOwned(s.ledger.Close))
		}
		s.terminal = r
		close(s.done)
		return Owner{state: s}, r
	}
	o := Owner{state: s}
	if err := o.Check(); err != nil {
		// No borrower is published yet. The creator owns joined partial cleanup.
		return o, o.Close(context.Background())
	}
	return o, Result{Phase: "startup", Outcome: "opened"}
}

// Deny stops admission synchronously and interrupts dependent issuing contexts.
// It releases no root; the original failure is retained through joined Close.
func (o Owner) Deny(cause error) {
	if o.state == nil {
		return
	}
	s := o.state
	s.mu.Lock()
	s.denied = true
	if s.first.Phase == "" {
		s.first = Result{Phase: "close", Outcome: "closed"}
	}
	if s.failure == nil && cause != nil {
		s.failure = cause
		s.first = Result{Phase: "authority", Outcome: "authority-unavailable"}
		if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
			s.first.Outcome = "canceled"
		}
	}
	s.mu.Unlock()
	s.cancel()
}

func (o Owner) fail(phase string, err error) error {
	s := o.state
	s.mu.Lock()
	s.denied = true
	if s.failure == nil {
		s.failure = err
		s.first = Result{Phase: phase, Outcome: category(err)}
	}
	s.mu.Unlock()
	s.cancel()
	return err
}

// Err retains the original cause and acquired-owner cleanup causes. It performs
// no observation and grants no authority; public command output stays bounded.
func (o Owner) Err() error {
	if o.state == nil {
		return issuance.ErrClosed
	}
	s := o.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(s.failure, s.cleanup)
}

// Check reobserves original authority and actual custody before effects. No
// foreign I/O or observer runs under the lifecycle state mutex.
func (o Owner) Check() error {
	_, err := o.current()
	return err
}

func (o Owner) current() (time.Time, error) {
	if o.state == nil {
		return time.Time{}, issuance.ErrClosed
	}
	s := o.state
	s.mu.Lock()
	denied := s.denied
	if !denied {
		s.users.Add(1)
	}
	s.mu.Unlock()
	if denied {
		return time.Time{}, issuance.ErrClosed
	}
	defer s.users.Done()
	if err := s.caller.Err(); err != nil {
		o.Deny(err)
		return time.Time{}, err
	}
	current, at, err := s.observe()
	if err != nil || current != s.initial || current.ValidateAt(at) != nil || at.Before(s.openedAt) {
		err = errors.Join(errors.New("original issuance authority unavailable"), err)
		o.Deny(err)
		return time.Time{}, err
	}
	if err := s.caller.Err(); err != nil {
		o.Deny(err)
		return time.Time{}, err
	}
	if err := s.ctx.Err(); err != nil {
		return time.Time{}, err
	}
	for _, check := range []struct {
		phase string
		run   func() error
	}{
		{"keys", s.keys.Check}, {"results", s.results.Check}, {"admission", s.ledger.Check},
	} {
		if err := check.run(); err != nil {
			return time.Time{}, o.fail(check.phase, err)
		}
	}
	if err := s.caller.Err(); err != nil {
		o.Deny(err)
		return time.Time{}, err
	}
	if err := s.ctx.Err(); err != nil {
		return time.Time{}, err
	}
	// Native key/journal I/O may have stalled. Reobserve actual external owners
	// afterward; the pre-I/O process/State observation cannot authorize an effect.
	current, fresh, err := s.observe()
	if err != nil || current != s.initial || current.ValidateAt(fresh) != nil || fresh.Before(at) {
		err = errors.Join(errors.New("original issuance authority unavailable after storage"), err)
		o.Deny(err)
		return time.Time{}, err
	}
	at = fresh
	if err := s.caller.Err(); err != nil {
		o.Deny(err)
		return time.Time{}, err
	}
	s.mu.Lock()
	denied = s.denied
	s.mu.Unlock()
	if denied {
		return time.Time{}, issuance.ErrClosed
	}
	return at, nil
}

// IssueCurrent refuses a concurrent request before observations, crypto or
// debit. Request cancellation and malformed/exhausted inputs do not retire a
// healthy original owner. Standard crypto/storage calls still physically join.
func (o Owner) IssueCurrent(ctx context.Context, raw []byte, kind quota.Kind) Result {
	if o.state == nil || ctx == nil {
		return Result{Phase: "input", Outcome: "invalid-input"}
	}
	s := o.state
	s.mu.Lock()
	if s.denied {
		r := s.first
		s.mu.Unlock()
		return r
	}
	if s.active != nil {
		s.mu.Unlock()
		return Result{Phase: "admission", Outcome: "busy"}
	}
	s.active = make(chan struct{})
	s.users.Add(1)
	active := s.active
	floor := s.lastIssueAt
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if floor.After(s.lastIssueAt) {
			s.lastIssueAt = floor
		}
		s.active = nil
		close(active)
		s.mu.Unlock()
		s.users.Done()
	}()
	if ctx.Err() != nil {
		return Result{Phase: "input", Outcome: "canceled"}
	}
	request, err := admission.DecodeClosedTokenBatch(raw)
	if err != nil {
		return Result{Phase: "input", Outcome: "invalid-input"}
	}
	raw = append([]byte(nil), raw...)
	defer clear(raw)
	work, cancel := context.WithCancel(s.ctx)
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { cancel(); close(callbackDone) })
	defer cancel()
	defer func() {
		if !stop() {
			<-callbackDone
		}
	}()
	b := s.binding
	f := admission.Facts{Network: b.Network, Issuer: b.Issuer, Authority: b.Authority, Holder: request.Permission.HolderKey, Duty: b.Duty, DutyNotBefore: b.Start, DutyNotAfter: b.End, Class: request.Class, Count: uint32(len(request.BlindedRequests))}
	check := func() (time.Time, error) {
		if ctx.Err() != nil {
			return time.Time{}, ctx.Err()
		}
		at, err := o.current()
		if err != nil {
			return time.Time{}, err
		}
		if at.Before(floor) {
			return time.Time{}, o.fail("authority", issuance.ErrValidity)
		}
		if at.Before(request.Permission.NotBefore) || !at.Before(request.Permission.NotAfter) {
			return time.Time{}, issuance.ErrValidity
		}
		floor = at
		return at.UTC().Truncate(time.Second), ctx.Err()
	}
	r := issueWithOwners(work, s.ledger, s.results, raw, f, kind, check)
	// Owner-local definite failure is distinct from request-specific refusal.
	if err := errors.Join(s.ledger.Check(), s.keys.Check(), s.results.Check()); err != nil {
		o.fail("storage", err)
		clear(r.Response)
		r.Response = nil
	}
	if r.Response != nil {
		if _, err := check(); err != nil {
			clear(r.Response)
			r.Response = nil
			r.Phase = "authority"
			r.Outcome = "authority-unavailable"
		}
	}
	return r
}

// Close denies first, then joins issuing borrowers and closes results→keys→quota.
// A bounded wait failure retains the roots and the original completion signal.
func (o Owner) Close(ctx context.Context) Result {
	if o.state == nil || ctx == nil {
		return Result{Phase: "close", Outcome: "invalid-input"}
	}
	s := o.state
	callerError := s.caller.Err()
	s.mu.Lock()
	if !s.denied {
		s.denied = true
		s.first = Result{Phase: "close", Outcome: "closed"}
		if callerError != nil {
			s.failure = callerError
			s.first = Result{Phase: "authority", Outcome: "canceled"}
		}
	}
	if s.first.Phase == "" {
		s.first = Result{Phase: "close", Outcome: "closed"}
	}
	if !s.closing {
		s.closing = true
		active := s.active
		go func() {
			if active != nil {
				<-active
			}
			s.users.Wait()
			s.mu.Lock()
			r := s.first
			s.mu.Unlock()
			var cleanup error
			closeOwned := func(close func() error) func() error {
				return func() error { err := close(); cleanup = errors.Join(cleanup, err); return err }
			}
			r.closeOwner(0, "close-results", closeOwned(s.results.Close))
			r.closeOwner(1, "close-keys", closeOwned(s.keys.Close))
			r.closeOwner(2, "close-admission", closeOwned(s.ledger.Close))
			s.mu.Lock()
			s.cleanup = cleanup
			s.terminal = r
			close(s.done)
			s.mu.Unlock()
		}()
	}
	s.mu.Unlock()
	s.cancel()
	// An already joined original result wins over a subsequently expired waiter.
	select {
	case <-s.done:
		s.mu.Lock()
		r := s.terminal
		s.mu.Unlock()
		return r
	default:
	}
	select {
	case <-s.done:
		s.mu.Lock()
		r := s.terminal
		s.mu.Unlock()
		return r
	case <-ctx.Done():
		return Result{Phase: "join", Outcome: "completion-unavailable"}
	}
}
