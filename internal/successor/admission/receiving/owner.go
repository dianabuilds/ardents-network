package receiving

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"sync"
	"sync/atomic"
	"time"
)

// Observation is supplied by the external authority owner for this exact duty.
type Observation struct {
	Profile       admission.AuthorityFacts
	Receiver      Receiver
	Now, NotAfter time.Time
}

// Owner owns receiver-local spending and verification admission, not physical
// capacity. Closing it joins synchronous admissions but never releases an
// already transferred work reservation.
type Owner struct {
	mu       sync.Mutex
	receiver Receiver
	ledger   *spending.Ledger
	observe  func() (Observation, error)
	gate     VerificationGate
	floor    time.Time
	closed   bool
	closeErr error
}

func Open(root string, receiver Receiver, observe func() (Observation, error)) (*Owner, error) {
	if observe == nil {
		return nil, errors.New("receiver observer absent")
	}
	o := &Owner{receiver: receiver, observe: observe}
	if _, err := o.current(); err != nil {
		return nil, err
	}
	ledger, err := spending.Open(root, spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, err
	}
	o.ledger = ledger
	return o, nil
}

func (o *Owner) current() (Observation, error) {
	if o.closed {
		return Observation{}, errors.New("receiver closed")
	}
	v, err := o.observe()
	if err != nil || v.Receiver != o.receiver || v.Profile.ValidateAt(v.Now) != nil || v.Now.Before(o.floor) || !v.Now.Before(v.NotAfter) ||
		v.Profile.NetworkID != o.receiver.NetworkID || v.Profile.Digest != o.receiver.ProfileDigest || v.Profile.StateGeneration != o.receiver.StateGeneration || v.Profile.StateDigest != o.receiver.StateDigest {
		return Observation{}, errors.Join(errors.New("receiver authority unavailable"), err)
	}
	o.floor = v.Now
	if v.Profile.NotAfter.Before(v.NotAfter) {
		v.NotAfter = v.Profile.NotAfter
	}
	return v, nil
}

// Grant transfers the successful capacity reservation to the work owner.
// Copies share one idempotent release and cannot renew the original deadline.
type Grant struct{ state *grantState }
type grantState struct {
	owner     *Owner
	allowance Allowance
	ended     atomic.Bool
	once      sync.Once
	release   func() error
	err       error
}

func (g Grant) Allowance() Allowance {
	if g.state == nil {
		return Allowance{}
	}
	return g.state.allowance
}
func (g Grant) Release() error {
	if g.state == nil {
		return nil
	}
	s := g.state
	s.once.Do(func() {
		s.ended.Store(true)
		if s.release != nil {
			s.err = s.release()
		}
	})
	return s.err
}

// Accept verifies a token before capacity or spending effects.
func (o *Owner) Accept(ctx context.Context, class admission.Class, raw []byte, deadline time.Time, reserve func() (func() error, error)) (Grant, error) {
	return o.admit(ctx, class, raw, deadline, Grant{}, 0, reserve)
}

// Refill requires a still-live original Grant and retains its deadline.
func (o *Owner) Refill(ctx context.Context, prior Grant, remaining uint64, raw []byte, reserve func() (func() error, error)) (Grant, error) {
	if o != nil {
		o.ledger.InvalidateFreshRoot()
	}
	if prior.state == nil || prior.state.owner != o {
		return Grant{}, errors.New("foreign admission grant")
	}
	return o.admit(ctx, admission.ForwardClass, raw, prior.state.allowance.Deadline(), prior, remaining, reserve)
}
func (o *Owner) admit(ctx context.Context, class admission.Class, raw []byte, deadline time.Time, prior Grant, remaining uint64, reserve func() (func() error, error)) (Grant, error) {
	if o != nil {
		o.ledger.InvalidateFreshRoot()
	}
	if o == nil || ctx == nil || reserve == nil {
		return Grant{}, errors.New("receiver unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if ctx.Err() != nil {
		return Grant{}, ctx.Err()
	}
	initial, err := o.current()
	if err != nil {
		return Grant{}, err
	}
	finish, err := o.gate.Begin(initial.Now)
	if err != nil {
		return Grant{}, err
	}
	defer finish()
	allowance, err := NewAllowance(class, initial.Now, deadline, initial.NotAfter)
	if prior.state != nil {
		if prior.state.ended.Load() {
			return Grant{}, errors.New("grant released")
		}
		allowance, err = prior.state.allowance.Replenish(initial.Now, remaining)
		if allowance.Deadline().After(initial.NotAfter) {
			return Grant{}, errors.New("duty cannot cover refill")
		}
	}
	if err != nil {
		return Grant{}, err
	}
	window, err := VerifyToken(initial.Profile, o.receiver, class, raw, initial.Now)
	if err != nil {
		return Grant{}, err
	}
	currentTime := initial.Now
	recheck := func() error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		v, err := o.current()
		if err != nil || v.Profile != initial.Profile || !v.Now.Before(allowance.Deadline()) || allowance.Deadline().After(v.NotAfter) || (prior.state != nil && prior.state.ended.Load()) {
			return errors.Join(errors.New("admission changed during commit"), err)
		}
		currentTime = v.Now
		return nil
	}
	approval, err := Redeem(Redemption{Class: class, Token: raw, Deadline: allowance.Deadline()}, o.ledger, func() time.Time { return currentTime },
		func() (Approval, error) {
			release, err := reserve()
			if err == nil {
				err = recheck()
			}
			return Approval{Window: window, Release: release, recheck: recheck}, err
		}, nil)
	if err != nil {
		return Grant{}, err
	}
	return Grant{&grantState{owner: o, allowance: allowance, release: approval.Release}}, nil
}

// TakeFreshRoot transfers the opaque durable creation fact before receiving
// attempts. It cannot be reconstructed from an empty retained spend journal.
func (o *Owner) TakeFreshRoot() (*spending.FreshRoot, error) {
	if o == nil || o.ledger == nil {
		return nil, errors.New("receiver unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.current(); err != nil {
		o.ledger.InvalidateFreshRoot()
		return nil, err
	}
	return o.ledger.TakeFreshRoot()
}

func (o *Owner) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.closed = true
		o.closeErr = o.ledger.Close()
	}
	return o.closeErr
}
