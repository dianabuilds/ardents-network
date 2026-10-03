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
	if o == nil {
		return Attempt{}, errors.New("holder unavailable")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	p, now, err := o.current()
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
	o.mu.Lock()
	defer o.mu.Unlock()
	p, now, err := o.current()
	if err != nil || s.done || o.active != s || o.permission != s.permission || p != s.profile || !now.Before(s.deadline) {
		return nil, time.Time{}, errors.Join(errors.New("attempt unavailable"), err)
	}
	return bytes.Clone(s.batch.Pending.Request()), s.deadline, nil
}

// Complete is one terminal transition. A failed external exchange preserves
// the exact pending blind batch for retry, without refunding its reservation.
func (a Attempt) Complete(payload []byte, exchangeErr error) error {
	if a.state == nil {
		return errors.New("attempt absent")
	}
	s := a.state
	o := s.owner
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.done {
		return errors.New("attempt already completed")
	}
	s.done = true
	if o.active != s || o.permission != s.permission {
		return errors.New("attempt revoked")
	}
	o.active = nil
	p, now, err := o.current()
	if err != nil || p != s.profile || !now.Before(s.deadline) || !s.permission.CurrentFor(p, now) {
		s.permission.discardPendingBatch(s.batch)
		return errors.Join(errors.New("issuance authority expired or changed"), err)
	}
	if exchangeErr != nil {
		return exchangeErr
	}
	return s.permission.acceptIssuedBatch(s.batch, payload)
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
