package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
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
	reference                     *managerReference
	startAttempted, stopAttempted bool // original physical I/O serialization
	once                          sync.Once
	closeErr                      error
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
	s := &ForwardingControl{original: &forwardingControl{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainForwardingControlUnit()
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

func (r *managerReference) retainForwardingControlUnit() (err error) {
	if err = r.begin(); err != nil {
		return err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() { err = r.managerResult(err, ctx) }()
	if err = r.local(); err != nil {
		return err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return err
	}
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", "ardents-forwarding.service")
	if err != nil {
		return err
	}
	if len(call.Body) != 0 {
		return ErrObservation
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if err = r.local(); err != nil {
		return err
	}
	return r.client.transport.socket.SetDeadline(time.Time{})
}

func (r *managerReference) forwardingLoadedObservation() (result ForwardingObservation, err error) {
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
			result = ForwardingObservation{}
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
	for round := 0; round < 2; round++ {
		for _, unit := range nodeUnitNames[1:] {
			if err = nodeSlotAbsent(ctx, r, unit); err != nil {
				return result, err
			}
		}
		if err = nodeNameAbsent(ctx, r, "ardents-forwarding.socket"); err != nil {
			return result, err
		}
		if round != 0 {
			break
		}
		call, e := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", "ardents-forwarding.service")
		if e != nil {
			return result, e
		}
		if len(call.Body) != 1 {
			return result, ErrObservation
		}
		object, ok := call.Body[0].(dbus.ObjectPath)
		if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice" {
			return result, ErrObservation
		}
		for i, iface := range []string{"Unit", "Service"} {
			call, e = r.client.call(ctx, string(object), "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
			if e != nil {
				return result, e
			}
			properties, e := startupProperties(call)
			if e != nil {
				return result, e
			}
			if i == 0 {
				result.Unit = properties
			} else {
				result.Service = properties
			}
		}
	}
	if r.observe != nil {
		r.observe()
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return result, err
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
	result.Manager, result.Version, result.UnitPaths = r.client.manager, r.version, append([]string(nil), r.paths...)
	return result, nil
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
