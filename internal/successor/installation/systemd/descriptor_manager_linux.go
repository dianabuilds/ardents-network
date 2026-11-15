package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// DescriptorObservation is detached complete fixed-unit data. Installation must
// match its own generation, expected protection and original native invocation.
type DescriptorObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// DescriptorRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type DescriptorRuntime struct{ original *descriptorRuntime }

type descriptorRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// DescriptorControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain completion. Closing returns the original server reference.
type DescriptorControl struct{ original *descriptorControl }

type descriptorControl struct {
	*fixedUnitControl
	once     sync.Once
	closeErr error
}

func OpenDescriptorRuntime(ctx context.Context) (*DescriptorRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorUnitReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorRuntime{original: &descriptorRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.descriptorLoadedObservation()
	}
	return s, err
}

func OpenDescriptorControl(ctx context.Context) (*DescriptorControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorControlReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorControl{original: &descriptorControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: descriptorControlPurpose}}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainFixedUnit(descriptorControlPurpose)
	}
	if err == nil {
		_, err = r.descriptorLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Descriptor control may receive native job object replies.
		r.client.transport.descriptorJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) descriptorLoadedObservation() (DescriptorObservation, error) {
	observation, err := r.fixedNodeObservation(descriptorControlPurpose)
	return DescriptorObservation(observation), err
}

func (s *DescriptorRuntime) Observe() (DescriptorObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return DescriptorObservation{}, ErrInput
	}
	return s.original.reference.descriptorLoadedObservation()
}

func (s *DescriptorControl) Observe() (DescriptorObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return DescriptorObservation{}, ErrInput
	}
	return s.original.reference.descriptorLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *DescriptorControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *DescriptorRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *DescriptorControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
