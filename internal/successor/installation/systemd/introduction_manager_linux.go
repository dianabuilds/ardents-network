package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
)

// IntroductionObservation is detached complete fixed-unit data. Installation must
// match its own generation, expected protection and original native invocation.
type IntroductionObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// IntroductionRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type IntroductionRuntime struct{ original *introductionRuntime }

type introductionRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// IntroductionControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain completion. Closing returns the original server reference.
type IntroductionControl struct{ original *introductionControl }

type introductionControl struct {
	*fixedUnitControl
	once     sync.Once
	closeErr error
}

func OpenIntroductionRuntime(ctx context.Context) (*IntroductionRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, introductionUnitReply)
	if r == nil {
		return nil, err
	}
	s := &IntroductionRuntime{original: &introductionRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.introductionLoadedObservation()
	}
	return s, err
}

func OpenIntroductionControl(ctx context.Context) (*IntroductionControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, introductionControlReply)
	if r == nil {
		return nil, err
	}
	s := &IntroductionControl{original: &introductionControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: introductionControlPurpose}}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainFixedUnit(introductionControlPurpose)
	}
	if err == nil {
		_, err = r.introductionLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Introduction control may receive native job object replies.
		r.client.transport.introductionJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) introductionLoadedObservation() (IntroductionObservation, error) {
	observation, err := r.fixedNodeObservation(introductionControlPurpose)
	return IntroductionObservation(observation), err
}

func (s *IntroductionRuntime) Observe() (IntroductionObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return IntroductionObservation{}, ErrInput
	}
	return s.original.reference.introductionLoadedObservation()
}

func (s *IntroductionControl) Observe() (IntroductionObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return IntroductionObservation{}, ErrInput
	}
	return s.original.reference.introductionLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *IntroductionControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *IntroductionRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *IntroductionControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
