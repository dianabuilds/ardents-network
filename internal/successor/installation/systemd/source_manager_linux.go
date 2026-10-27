package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// SourceObservation is a fresh detached complete fixed-unit observation.
// Installation separately compares its selected generation and actual process.
type SourceObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// SourceRuntime owns its own original read-only manager connection. It has no
// LoadUnit, reload, Start, Stop, arbitrary selector or reconnect operation.
type SourceRuntime struct{ original *sourceRuntime }

type sourceRuntime struct {
	reference *managerReference
	closeOnce sync.Once
	closeErr  error
}

// SourceControl owns root's separate first-start connection. It can attempt
// one fixed Source reference, one Start and one Stop; none proves admission or
// process join. The original connection keeps a stopped unit from collection.
type SourceControl struct{ original *sourceControl }

type sourceControl struct {
	reference                     *managerReference
	startAttempted, stopAttempted bool // original physical I/O mutex
	closeOnce                     sync.Once
	closeErr                      error
}

func OpenSourceRuntime(ctx context.Context) (*SourceRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openLoadedSourceReference(ctx)
	if r == nil {
		return nil, err
	}
	return &SourceRuntime{original: &sourceRuntime{reference: r}}, err
}

func OpenSourceControl(ctx context.Context) (*SourceControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, sourceUnitReply)
	if r == nil {
		return nil, err
	}
	result := &SourceControl{original: &sourceControl{reference: r}}
	if err != nil {
		return result, err
	}
	if err = r.initializeManagerDeclaration(); err != nil {
		return result, err
	}
	if err = r.retainSourceControlUnit(); err != nil {
		return result, err
	}
	if _, err = r.sourceLoadedObservation(); err != nil {
		return result, err
	}
	// Other Source/Endpoint observations retain their closed read-only vocabulary.
	// This admits only canonical native job replies, never a method/unit selector.
	r.client.transport.sourceJobs.Store(true)
	return result, nil
}

// RefUnit loads only the fixed Source and binds its in-memory lifetime to this
// original root connection. A successful reply grants no Start or loaded-policy
// fact. Runtime/preflight never call it. Closing the original connection drops
// its server-side reference, including after an uncertain reply or cancellation.
func (r *managerReference) retainSourceControlUnit() (err error) {
	if err = r.begin(); err != nil {
		return err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() { err = r.managerResult(err, ctx) }()
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return err
	}
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", "ardents-source.service")
	if err != nil {
		return err
	}
	if len(call.Body) != 0 {
		return ErrObservation
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if err = r.local(); err != nil {
		return err
	}
	return r.client.transport.socket.SetDeadline(time.Time{})
}

func openLoadedSourceReference(ctx context.Context) (*managerReference, error) {
	r, err := openManagerReference(ctx, sourceUnitReply)
	if err != nil {
		return r, err
	}
	if err = r.initializeManagerDeclaration(); err != nil {
		return r, err
	}
	_, err = r.sourceLoadedObservation()
	return r, err
}

func (r *managerReference) sourceLoadedObservation() (result SourceObservation, err error) {
	if err = r.begin(); err != nil {
		return result, err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() {
		err = r.managerResult(err, ctx)
		if err != nil {
			result = SourceObservation{}
		}
	}()
	if err = r.local(); err != nil {
		return result, err
	}
	if !r.client.bus.Connected() {
		return result, r.unavailable()
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return result, err
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return result, err
	}
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", "ardents-source.service")
	if err != nil {
		return result, err
	}
	if len(call.Body) != 1 {
		return result, ErrObservation
	}
	object, ok := call.Body[0].(dbus.ObjectPath)
	if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice" {
		return result, ErrObservation
	}
	for index, iface := range []string{"Unit", "Service"} {
		call, err = r.client.call(ctx, "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice", "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
		if err != nil {
			return result, err
		}
		properties, err := startupProperties(call)
		if err != nil {
			return result, err
		}
		if index == 0 {
			result.Unit = properties
		} else {
			result.Service = properties
		}
	}
	if r.observe != nil {
		r.observe()
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return result, err
	}
	if err = r.local(); err != nil {
		return result, err
	}
	if err = r.client.transport.socket.SetDeadline(time.Time{}); err != nil {
		return result, err
	}
	result.Manager, result.Version = r.client.manager, r.version
	result.UnitPaths = append([]string(nil), r.paths...)
	return result, nil
}

func (s *SourceRuntime) Observe() (SourceObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return SourceObservation{}, ErrInput
	}
	return s.original.reference.sourceLoadedObservation()
}

func (s *SourceControl) Observe() (SourceObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return SourceObservation{}, ErrInput
	}
	return s.original.reference.sourceLoadedObservation()
}

func (s *SourceRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.closeOnce.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *SourceControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.closeOnce.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}
