package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// ForwardingReplacement owns an independent original fixed Forwarding unit
// reference, complete five-slot observations and one explicit reload attempt.
// It cannot Start, Stop, Enable, choose a unit or reconnect. Installation alone
// admits effects and proves original retirement, generation and native context.
type ForwardingReplacement struct{ original *forwardingReplacement }

type forwardingReplacement struct {
	reference *managerReference
	reloaded  bool
	once      sync.Once
	closeErr  error
}

func OpenForwardingReplacement(ctx context.Context) (*ForwardingReplacement, error) {
	return openForwardingReplacement(ctx, "")
}

// The copied expectation only restricts native manager admission before the
// original RefUnit effect. It never supplies a genuine cut or recovery right.
func OpenForwardingRecoveryReplacement(ctx context.Context, expected string) (*ForwardingReplacement, error) {
	if expected == "" || len(expected) > 288 {
		return nil, ErrInput
	}
	return openForwardingReplacement(ctx, expected)
}

func openForwardingReplacement(ctx context.Context, expected string) (*ForwardingReplacement, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, forwardingUnitReply)
	if r == nil {
		return nil, err
	}
	s := &ForwardingReplacement{original: &forwardingReplacement{reference: r}}
	if err != nil {
		return s, err
	}
	if expected != "" {
		actual, err := r.managerIdentity()
		if err != nil {
			return s, err
		}
		if actual != expected {
			return s, r.managerResult(ErrObservation, ctx)
		}
	}
	if err := r.initializeManagerDeclaration(); err != nil {
		return s, err
	}
	if err := r.retainForwardingControlUnit(); err != nil {
		return s, err
	}
	if _, err := r.forwardingLoadedObservation(); err != nil {
		return s, err
	}
	return s, nil
}

func (s *ForwardingReplacement) Observe() (ForwardingObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ForwardingObservation{}, ErrInput
	}
	return s.original.reference.forwardingLoadedObservation()
}

func (s *ForwardingReplacement) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *ForwardingReplacement) Reload() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.reloadForwarding(&s.original.reloaded)
}

func (s *ForwardingReplacement) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
