package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// ForwardingObservation is detached complete fixed-unit data. Installation must
// match its own generation, expected protection and original native invocation.
type ForwardingObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// ForwardingRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type ForwardingRuntime struct{ original *forwardingRuntime }

type forwardingRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// ForwardingControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain join. Closing returns the original server reference.
type ForwardingControl struct{ original *forwardingControl }

type forwardingControl struct {
	*fixedUnitControl
	once     sync.Once
	closeErr error
}

func OpenForwardingRuntime(ctx context.Context) (*ForwardingRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, forwardingUnitReply)
	if r == nil {
		return nil, err
	}
	s := &ForwardingRuntime{original: &forwardingRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.forwardingLoadedObservation()
	}
	return s, err
}

func OpenForwardingControl(ctx context.Context) (*ForwardingControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, forwardingUnitReply)
	if r == nil {
		return nil, err
	}
	s := &ForwardingControl{original: &forwardingControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: forwardingControlPurpose}}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainFixedUnit(forwardingControlPurpose)
	}
	if err == nil {
		_, err = r.forwardingLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Forwarding control may receive native job object replies.
		r.client.transport.forwardingJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) forwardingLoadedObservation() (ForwardingObservation, error) {
	observation, err := r.fixedNodeObservation(forwardingControlPurpose)
	return ForwardingObservation(observation), err
}

func (s *ForwardingRuntime) Observe() (ForwardingObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ForwardingObservation{}, ErrInput
	}
	return s.original.reference.forwardingLoadedObservation()
}

func (s *ForwardingControl) Observe() (ForwardingObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ForwardingObservation{}, ErrInput
	}
	return s.original.reference.forwardingLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *ForwardingControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *ForwardingRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *ForwardingControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
