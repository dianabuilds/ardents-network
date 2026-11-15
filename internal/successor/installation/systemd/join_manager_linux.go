package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// JoinObservation is detached complete fixed-unit data. Installation must
// match its own generation, expected protection and original native invocation.
type JoinObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// JoinRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type JoinRuntime struct{ original *joinRuntime }

type joinRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// JoinControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain join. Closing returns the original server reference.
type JoinControl struct{ original *joinControl }

type joinControl struct {
	*fixedUnitControl
	once     sync.Once
	closeErr error
}

func OpenJoinRuntime(ctx context.Context) (*JoinRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, joinUnitReply)
	if r == nil {
		return nil, err
	}
	s := &JoinRuntime{original: &joinRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.joinLoadedObservation()
	}
	return s, err
}

func OpenJoinControl(ctx context.Context) (*JoinControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, joinControlReply)
	if r == nil {
		return nil, err
	}
	s := &JoinControl{original: &joinControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: joinControlPurpose}}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainFixedUnit(joinControlPurpose)
	}
	if err == nil {
		_, err = r.joinLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Join control may receive native job object replies.
		r.client.transport.joinJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) joinLoadedObservation() (JoinObservation, error) {
	observation, err := r.fixedNodeObservation(joinControlPurpose)
	return JoinObservation(observation), err
}

func (s *JoinRuntime) Observe() (JoinObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return JoinObservation{}, ErrInput
	}
	return s.original.reference.joinLoadedObservation()
}

func (s *JoinControl) Observe() (JoinObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return JoinObservation{}, ErrInput
	}
	return s.original.reference.joinLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *JoinControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *JoinRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *JoinControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
