package receiving

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

// LocalAllowance retains one receiver allowance without reserving physical work.
// Its sequential caller must close it after the last local operation.
type LocalAllowance struct {
	owner  *Owner
	grant  Grant
	active bool
}

// OpenLocalAllowance opens the original receiving owner for local operations.
func OpenLocalAllowance(root string, receiver Receiver, observe func() (Observation, error)) (*LocalAllowance, error) {
	owner, err := Open(root, receiver, observe)
	if err != nil {
		return nil, err
	}
	return &LocalAllowance{owner: owner}, nil
}

func (s *LocalAllowance) replace(next Grant) error {
	if err := s.grant.Release(); err != nil {
		_ = next.Release()
		return err
	}
	s.grant, s.active = next, true
	return nil
}

// Accept spends a token and retains its finite allowance without physical work.
func (s *LocalAllowance) Accept(ctx context.Context, class admission.Class, token []byte, deadline time.Time) error {
	if s.active {
		return errors.New("allowance already retained")
	}
	next, err := s.owner.Accept(ctx, class, token, deadline, func() (func() error, error) { return nil, nil })
	if err != nil {
		return err
	}
	return s.replace(next)
}

// Refill replaces an active allowance while preserving its original deadline.
func (s *LocalAllowance) Refill(ctx context.Context, remaining uint64, token []byte) error {
	if !s.active {
		return errors.New("allowance absent")
	}
	next, err := s.owner.Refill(ctx, s.grant, remaining, token, func() (func() error, error) { return nil, nil })
	if err != nil {
		return err
	}
	return s.replace(next)
}

// Release retires the retained allowance.
func (s *LocalAllowance) Release() error {
	err := s.grant.Release()
	s.active = false
	return err
}

// Current returns the retained allowance and whether it is active.
func (s *LocalAllowance) Current() (Allowance, bool) { return s.grant.Allowance(), s.active }

// Close releases the allowance before closing its receiving owner.
func (s *LocalAllowance) Close() error { return errors.Join(s.grant.Release(), s.owner.Close()) }
