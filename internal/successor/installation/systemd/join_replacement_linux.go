package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// JoinReplacement owns an independent original fixed Join unit
// reference, complete five-slot observations and one explicit reload attempt.
// It cannot Start, Stop, Enable, choose a unit or reconnect. Installation alone
// admits effects and proves original retirement, generation and native context.
type JoinReplacement struct{ original *joinReplacement }

type joinReplacement struct {
	reference *managerReference
	reloaded  bool
	once      sync.Once
	closeErr  error
}

func OpenJoinReplacement(ctx context.Context) (*JoinReplacement, error) {
	return openJoinReplacement(ctx, "")
}

func openJoinReplacement(ctx context.Context, expected string) (*JoinReplacement, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, joinUnitReply)
	if r == nil {
		return nil, err
	}
	s := &JoinReplacement{original: &joinReplacement{reference: r}}
	if err != nil {
		return s, err
	}
	if expected != "" {
		actual, e := r.managerIdentity()
		if e != nil {
			return s, e
		}
		if actual != expected {
			return s, r.managerResult(ErrObservation, ctx)
		}
	}
	if err := r.initializeManagerDeclaration(); err != nil {
		return s, err
	}
	if err := r.retainJoinControlUnit(); err != nil {
		return s, err
	}
	if _, err := r.joinLoadedObservation(); err != nil {
		return s, err
	}
	return s, nil
}

func (s *JoinReplacement) Observe() (JoinObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return JoinObservation{}, ErrInput
	}
	return s.original.reference.joinLoadedObservation()
}

func (s *JoinReplacement) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *JoinReplacement) Reload() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.reloadOriginal(&s.original.reloaded)
}

func (s *JoinReplacement) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

// JoinRecoveryReplacement observes one fixed original manager and stopped JOIN
// unit. It exposes no Reload, Start, Stop, selector or reconnect operation.
type JoinRecoveryReplacement struct{ original *JoinReplacement }

func OpenJoinRecoveryReplacement(ctx context.Context, expected string) (*JoinRecoveryReplacement, error) {
	if expected == "" || len(expected) > 288 {
		return nil, ErrInput
	}
	original, err := openJoinReplacement(ctx, expected)
	return &JoinRecoveryReplacement{original}, err
}
func (s *JoinRecoveryReplacement) Observe() (JoinObservation, error) {
	if s == nil {
		return JoinObservation{}, ErrInput
	}
	return s.original.Observe()
}
func (s *JoinRecoveryReplacement) ManagerIdentity() (string, error) {
	if s == nil {
		return "", ErrInput
	}
	return s.original.ManagerIdentity()
}
func (s *JoinRecoveryReplacement) Close() error {
	if s == nil {
		return nil
	}
	return s.original.Close()
}
