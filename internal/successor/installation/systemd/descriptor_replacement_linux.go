package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// DescriptorReplacement owns an independent original fixed Descriptor unit
// reference, complete five-slot observations and one explicit reload attempt.
// It cannot Start, Stop, Enable, choose a unit or reconnect. Installation alone
// admits effects and proves original retirement, generation and native context.
type DescriptorReplacement struct{ original *descriptorReplacement }

type descriptorReplacement struct {
	reference *managerReference
	reloaded  bool
	once      sync.Once
	closeErr  error
}

func OpenDescriptorReplacement(ctx context.Context) (*DescriptorReplacement, error) {
	return openDescriptorReplacement(ctx)
}

func openDescriptorReplacement(ctx context.Context) (*DescriptorReplacement, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorUnitReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorReplacement{original: &descriptorReplacement{reference: r}}
	if err != nil {
		return s, err
	}
	if err := r.initializeManagerDeclaration(); err != nil {
		return s, err
	}
	if err := r.retainDescriptorControlUnit(); err != nil {
		return s, err
	}
	if _, err := r.descriptorLoadedObservation(); err != nil {
		return s, err
	}
	return s, nil
}

func (s *DescriptorReplacement) Observe() (DescriptorObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return DescriptorObservation{}, ErrInput
	}
	return s.original.reference.descriptorLoadedObservation()
}

func (s *DescriptorReplacement) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *DescriptorReplacement) Reload() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.reloadOriginal(&s.original.reloaded)
}

func (s *DescriptorReplacement) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
