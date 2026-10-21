package systemd

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// SourceStopped owns a separate original connection for initial stopped Source
// publication. Its only manager mutation is one explicit daemon reload. It has
// no Start, Stop, Enable, arbitrary unit selector, or reconnect operation.
type SourceStopped struct{ original *sourceStopped }
type sourceStopped struct {
	reference *SourcePreflight
	reloaded  bool // serialized by the original physical observation mutex
	closeOnce sync.Once
	closeErr  error
}

func OpenSourceStopped(ctx context.Context) (*SourceStopped, error) {
	r, err := OpenSourcePreflight(ctx)
	s := &SourceStopped{original: &sourceStopped{reference: r}}
	if err == nil {
		_, err = s.CheckAbsent()
	}
	return s, err
}

// CheckAbsent is valid only before publication. Loaded observations have a
// separate operation; absence is never disabled by a supplied boolean.
func (s *SourceStopped) CheckAbsent() (paths []string, err error) {
	if s == nil || s.original == nil || s.original.reference == nil || s.original.reference.owner == nil {
		return nil, ErrInput
	}
	r := s.original.reference.owner
	if err = r.begin(); err != nil {
		return nil, err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	defer func() {
		if err != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, err)
			r.mu.Unlock()
			paths = nil
		}
	}()
	if s.original.reloaded {
		return nil, ErrObservation
	}
	paths, err = r.observeAbsent()
	if err != nil {
		return nil, err
	}
	// Per-call deadlines remain finite. Idle time belongs to this original
	// transition, rather than to the last completed observation's timeout.
	if err = r.client.transport.socket.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return paths, r.local()
}

// Reload attempts exactly one reload on the original unique manager. Failure
// remains retained even if the daemon may have completed its side of the call.
func (s *SourceStopped) Reload() (err error) {
	if s == nil || s.original == nil || s.original.reference == nil || s.original.reference.owner == nil {
		return ErrInput
	}
	r := s.original.reference.owner
	if err = r.begin(); err != nil {
		return err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	defer func() {
		if err != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, err)
			r.mu.Unlock()
		}
	}()
	if err = r.local(); err != nil {
		return err
	}
	if s.original.reloaded {
		return ErrObservation
	}
	if err = r.client.sameOwner(r.original, r.client.manager); err != nil {
		return err
	}
	s.original.reloaded = true
	_, err = r.client.call(r.original, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.Reload")
	if err != nil {
		return err
	}
	if err = r.client.sameOwner(r.original, r.client.manager); err != nil {
		return err
	}
	return errors.Join(r.client.transport.socket.SetDeadline(time.Time{}), r.local())
}

// CheckOriginal reobserves only the original manager identity/configuration.
// Phase-specific absence and loaded-unit policy remain separate operations.
func (s *SourceStopped) CheckOriginal() (err error) {
	if s == nil || s.original == nil || s.original.reference == nil || s.original.reference.owner == nil {
		return ErrInput
	}
	r := s.original.reference.owner
	if err = r.begin(); err != nil {
		return err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	defer func() {
		if err != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, err)
			r.mu.Unlock()
		}
	}()
	if err = r.local(); err != nil {
		return err
	}
	if !r.client.bus.Connected() {
		return ErrUnavailable
	}
	if err = r.client.sameOwner(r.original, r.client.manager); err != nil {
		return err
	}
	if err = r.sourceManagerProperties(); err != nil {
		return err
	}
	if err = r.client.sameOwner(r.original, r.client.manager); err != nil {
		return err
	}
	return errors.Join(r.client.transport.socket.SetDeadline(time.Time{}), r.local())
}

func (r *sourcePreflight) sourceManagerProperties() error {
	for _, name := range []string{"Version", "UnitPath"} {
		call, e := r.client.call(r.original, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties.Get", "org.freedesktop.systemd1.Manager", name)
		if e != nil {
			return e
		}
		var v dbus.Variant
		if e = call.Store(&v); e != nil {
			return e
		}
		if name == "Version" {
			value, ok := v.Value().(string)
			if !ok || v.Signature().String() != "s" || value != r.version {
				return ErrObservation
			}
		} else {
			value, ok := v.Value().([]string)
			if !ok || v.Signature().String() != "as" || strings.Join(value, "\x00") != strings.Join(r.paths, "\x00") {
				return ErrObservation
			}
		}
	}
	return nil
}

// ObserveLoaded obtains detached complete Unit/Service dictionaries on the
// same original manager. Installation owns expected bytes and stopped policy.
func (s *SourceStopped) ObserveLoaded() (unit, service Properties, err error) {
	if s == nil || s.original == nil || s.original.reference == nil || s.original.reference.owner == nil {
		return nil, nil, ErrInput
	}
	r := s.original.reference.owner
	if err = r.begin(); err != nil {
		return nil, nil, err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	defer func() {
		if err != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, err)
			r.mu.Unlock()
			unit, service = nil, nil
		}
	}()
	if err = r.local(); err != nil {
		return nil, nil, err
	}
	if !s.original.reloaded || !r.client.bus.Connected() {
		return nil, nil, ErrObservation
	}
	if err = r.client.sameOwner(r.original, r.client.manager); err != nil {
		return nil, nil, err
	}
	if err = r.sourceManagerProperties(); err != nil {
		return nil, nil, err
	}
	call, e := r.client.call(r.original, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.LoadUnit", "ardents-source.service")
	if e != nil {
		return nil, nil, e
	}
	if len(call.Body) != 1 {
		return nil, nil, ErrObservation
	}
	object, ok := call.Body[0].(dbus.ObjectPath)
	if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice" {
		return nil, nil, ErrObservation
	}
	for i, iface := range []string{"Unit", "Service"} {
		call, e = r.client.call(r.original, string(object), "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
		if e != nil {
			return nil, nil, e
		}
		p, e := startupProperties(call)
		if e != nil {
			return nil, nil, e
		}
		if i == 0 {
			unit = p
		} else {
			service = p
		}
	}
	err = errors.Join(r.client.sameOwner(r.original, r.client.manager), r.local())
	if err == nil {
		err = r.client.transport.socket.SetDeadline(time.Time{})
	}
	return unit, service, err
}

func (s *SourceStopped) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	s.original.closeOnce.Do(func() {
		s.original.closeErr = s.original.reference.Close()
		if s.original.reference != nil && s.original.reference.owner != nil {
			s.original.closeErr = errors.Join(s.original.closeErr, s.original.reference.owner.original.Err())
		}
	})
	return s.original.closeErr
}
