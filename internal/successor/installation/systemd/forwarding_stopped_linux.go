package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// ForwardingStopped owns a fresh root-only manager connection for the distinct
// initial stopped Forwarding transaction. It checks all five Node slots, may
// attempt one daemon reload, and loads only the fixed Forwarding service. It
// cannot start, stop, enable, reconnect or accept an arbitrary unit selector.
// Installation retains artifact admission, expected policy and native joins.
type ForwardingStopped struct{ original *forwardingStopped }

type forwardingStopped struct {
	reference *managerReference
	reloaded  bool // serialized by reference.ioMu
	closeOnce sync.Once
	closeErr  error
}

var nodeUnitNames = [...]string{
	"ardents-forwarding.service", "ardents-issuer.service",
	"ardents-descriptor.service", "ardents-introduction.service", "ardents-join.service",
}

// OpenForwardingStopped returns partial connection custody on error. The
// creator must Close every non-nil result, including failed admission.
func OpenForwardingStopped(ctx context.Context) (*ForwardingStopped, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, forwardingUnitReply)
	if r == nil {
		return nil, err
	}
	s := &ForwardingStopped{original: &forwardingStopped{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = s.CheckAbsent()
	}
	return s, err
}

// operation serializes only this owner's physical manager I/O. Its private
// callers supply fixed native operations, never an Installation policy callback.
func (s *ForwardingStopped) operation(effect func(context.Context, *managerReference) error) (err error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	r := s.original.reference
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
	if err = effect(ctx, r); err != nil {
		return err
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if r.observe != nil {
		r.observe()
	}
	if err = r.local(); err != nil {
		return err
	}
	return r.client.transport.socket.SetDeadline(time.Time{})
}

// CheckAbsent refuses every loaded or installed Node slot and every associated
// socket, including inactive units. It neither loads nor retains a unit. Returned
// paths are detached manager search paths, not writable-path authorization.
func (s *ForwardingStopped) CheckAbsent() ([]string, error) {
	err := s.operation(func(ctx context.Context, r *managerReference) error {
		if s.original.reloaded {
			return ErrObservation
		}
		for _, unit := range nodeUnitNames {
			if err := nodeSlotAbsent(ctx, r, unit); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return append([]string(nil), s.original.reference.paths...), nil
}

func nodeSlotAbsent(ctx context.Context, r *managerReference, unit string) error {
	for _, name := range []string{unit, unit[:len(unit)-len("service")] + "socket"} {
		if err := nodeNameAbsent(ctx, r, name); err != nil {
			return err
		}
	}
	return nil
}

func nodeNameAbsent(ctx context.Context, r *managerReference, name string) error {
	for _, method := range []struct{ name, absent string }{
		{"GetUnit", "org.freedesktop.systemd1.NoSuchUnit"},
		{"GetUnitFileState", "org.freedesktop.DBus.Error.FileNotFound"},
	} {
		_, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager."+method.name, name)
		var refusal dbus.Error
		if !errors.As(err, &refusal) || refusal.Name != method.absent {
			// An interrupted request supplied no absence observation. Preserve
			// only its exact original context leaves without inventing a changed
			// unit identity; foreign replies and physical wrappers still fail.
			if nodeAbsenceContextOnly(ctx, err) {
				return err
			}
			return errors.Join(ErrObservation, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func nodeAbsenceContextOnly(ctx context.Context, err error) bool {
	if ctx.Err() == nil || err == nil {
		return false
	}
	if err == ctx.Err() {
		return true
	}
	if group, ok := err.(interface{ Unwrap() []error }); ok {
		children := group.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !nodeAbsenceContextOnly(ctx, child) {
				return false
			}
		}
		return true
	}
	return false
}

// CheckOriginal reobserves original manager identity/configuration without a
// phase-specific unit assertion. A later caller still needs its own phase check.
func (s *ForwardingStopped) CheckOriginal() error {
	return s.operation(func(_ context.Context, _ *managerReference) error { return nil })
}

// Reload attempts exactly one fixed manager effect. An uncertain reply remains
// failed and cannot be retried on this connection or reported as completion.
func (s *ForwardingStopped) Reload() error {
	return s.operation(func(ctx context.Context, r *managerReference) error {
		r.mu.Lock()
		if r.closed.Load() || r.terminal != nil || r.original.Err() != nil || s.original.reloaded {
			r.mu.Unlock()
			return ErrUnavailable
		}
		s.original.reloaded = true
		r.mu.Unlock()
		_, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.Reload")
		return err
	})
}

// ObserveLoaded returns complete fixed Forwarding Unit/Service facts, surrounded
// by absence checks of the other four Node slots. It grants no policy or startup
// permission. Source's unit and lifetime are never referenced or observed here.
func (s *ForwardingStopped) ObserveLoaded() (unit, service Properties, err error) {
	err = s.operation(func(ctx context.Context, r *managerReference) error {
		if !s.original.reloaded {
			return ErrObservation
		}
		for round := 0; round < 2; round++ {
			for _, name := range nodeUnitNames[1:] {
				if err := nodeSlotAbsent(ctx, r, name); err != nil {
					return err
				}
			}
			if round != 0 {
				break
			}
			call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.LoadUnit", "ardents-forwarding.service")
			if err != nil {
				return err
			}
			if len(call.Body) != 1 {
				return ErrObservation
			}
			object, ok := call.Body[0].(dbus.ObjectPath)
			if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice" {
				return ErrObservation
			}
			for i, iface := range []string{"Unit", "Service"} {
				call, err = r.client.call(ctx, string(object), "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
				if err != nil {
					return err
				}
				facts, err := startupProperties(call)
				if err != nil {
					return err
				}
				if i == 0 {
					unit = facts
				} else {
					service = facts
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return unit, service, nil
}

// Close denies and interrupts before waiting for original manager users and
// library/transport joins. Copies share the original retained terminal result.
func (s *ForwardingStopped) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	s.original.closeOnce.Do(func() {
		r := s.original.reference
		s.original.closeErr = errors.Join(r.close(), r.original.Err())
	})
	return s.original.closeErr
}
