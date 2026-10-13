package stock

import (
	"bytes"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"time"
)

// IssuanceIntent pins an application-selected delivery identity, without
// granting transport authority. An exact retry must retain that identity.
type IssuanceIntent struct {
	Challenges        []admissiontoken.ClosedTokenContext
	Selection         ExchangeBinding
	Bootstrap, Refill bool
	Deadline          time.Time
}

type Attempt struct{ state *attemptState }
type attemptState struct {
	owner      *Owner
	permission *permission
	profile    admission.AuthorityFacts
	batch      *batch
	deadline   time.Time
	done       bool
}

// Begin reserves quota once and returns only public request access.
// The application executes transport outside this aggregate.
func (o *Owner) Begin(intent IssuanceIntent) (Attempt, error) {
	return o.begin(intent, nil)
}

func (o *Owner) begin(intent IssuanceIntent, preparation *Preparation) (Attempt, error) {
	if err := o.beginUse(); err != nil {
		return Attempt{}, err
	}
	defer o.users.Done()
	profile, at, observedErr := o.observeAndLock()
	defer o.mu.Unlock()
	if o.preparing != preparation || preparation != nil && (preparation.self != preparation || preparation.ctx.Err() != nil) {
		return Attempt{}, errors.New("original stock preparation is unavailable")
	}
	p, now, err := o.current(profile, at, observedErr)
	if err != nil || o.active != nil || !o.permission.CurrentFor(p, now) || intent.Selection.ID == [32]byte{} || intent.Selection.ProfileDigest != p.Digest || !now.Before(intent.Deadline) {
		return Attempt{}, errors.Join(errors.New("issuance unavailable"), err)
	}
	end := intent.Deadline
	if o.permission.accepted.NotAfter.Before(end) {
		end = o.permission.accepted.NotAfter
	}
	b, err := o.permission.reserveBatch(p, now, intent.Challenges, intent.Selection, intent.Refill, intent.Bootstrap)
	if err != nil {
		return Attempt{}, err
	}
	if b.Deadline.IsZero() {
		b.Deadline = end
	}
	if b.Deadline.Before(end) {
		end = b.Deadline
	}
	if !now.Before(end) {
		return Attempt{}, errors.New("original issuance deadline expired")
	}
	a := &attemptState{owner: o, permission: o.permission, profile: p, batch: b, deadline: end}
	o.active = a
	return Attempt{a}, nil
}

// Request returns a copy. Revocation and terminal completion erase access.
func (a Attempt) Request() ([]byte, time.Time, error) {
	if a.state == nil {
		return nil, time.Time{}, errors.New("attempt absent")
	}
	s := a.state
	o := s.owner
	if err := o.beginUse(); err != nil {
		return nil, time.Time{}, err
	}
	defer o.users.Done()
	profile, at, observedErr := o.observeAndLock()
	defer o.mu.Unlock()
	p, now, err := o.current(profile, at, observedErr)
	if err != nil || s.done || o.active != s || o.permission != s.permission || p != s.profile || !now.Before(s.deadline) {
		return nil, time.Time{}, errors.Join(errors.New("attempt unavailable"), err)
	}
	return bytes.Clone(s.batch.Pending.Request()), s.deadline, nil
}

// Complete is one terminal transition. A failed external exchange preserves
// the exact pending blind batch for retry, without refunding its reservation.
func (a Attempt) Complete(payload []byte, exchangeErr error) error {
	return a.complete(payload, exchangeErr, nil)
}

// CompleteBound additionally checks the original external delivery lifetime
// after token verification and immediately before deposit. The check may only
// restrict completion; it grants no Admission authority. It runs under the
// Stock lock and must perform no I/O or call back into Stock.
func (a Attempt) CompleteBound(payload []byte, exchangeErr error, check func() error) error {
	if check == nil {
		return errors.New("issuance completion lifetime absent")
	}
	return a.complete(payload, exchangeErr, check)
}

func (a Attempt) complete(payload []byte, exchangeErr error, check func() error) error {
	if a.state == nil {
		return errors.New("attempt absent")
	}
	s := a.state
	o := s.owner
	if err := o.beginUse(); err != nil {
		return err
	}
	defer o.users.Done()
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.done {
		return errors.New("attempt already completed")
	}
	s.done = true
	if o.active != s || o.permission != s.permission {
		return errors.New("attempt revoked")
	}
	defer func() {
		if o.active == s {
			o.active = nil
		}
	}()
	o.mu.Unlock()
	profile, at, observedErr := o.observeAndLock()
	p, now, err := o.current(profile, at, observedErr)
	if err != nil || o.active != s || o.permission != s.permission || p != s.profile || !now.Before(s.deadline) || !s.permission.CurrentFor(p, now) {
		s.permission.discardPendingBatch(s.batch)
		return errors.Join(errors.New("issuance authority expired or changed"), err)
	}
	if exchangeErr != nil {
		return exchangeErr
	}
	finalCheck := func() error {
		// FinalizeEncoded has already consumed the blind state. Verified tokens
		// remain local to acceptIssuedBatch until this original final gate; Close
		// can erase permission while observation waits, but cannot acquire them.
		o.mu.Unlock()
		profile, at, observedErr := o.observeAndLock()
		p, now, err := o.current(profile, at, observedErr)
		if err != nil || o.active != s || o.permission != s.permission || p != s.profile || !now.Before(s.deadline) || !s.permission.CurrentFor(p, now) {
			return errors.Join(errors.New("issuance authority changed during verification"), err)
		}
		if check != nil {
			return check()
		}
		return nil
	}
	return s.permission.acceptIssuedBatch(s.batch, payload, finalCheck)
}

// Discard retires the exact pending request without restoring quota.
func (a Attempt) Discard() {
	if a.state == nil {
		return
	}
	s := a.state
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.done {
		return
	}
	s.done = true
	if o.active == s {
		o.active = nil
		s.permission.discardPendingBatch(s.batch)
	}
}
