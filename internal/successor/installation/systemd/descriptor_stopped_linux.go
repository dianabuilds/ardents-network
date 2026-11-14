package systemd

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
)

// DescriptorStopped owns a fresh root-only manager connection for the distinct
// initial stopped Descriptor transaction. It checks all five Node slots, may
// attempt one daemon reload, and loads only the fixed Descriptor service. It
// cannot start, stop, enable, reconnect or accept an arbitrary unit selector.
// Installation retains artifact admission, expected policy and native joins.
type DescriptorStopped struct{ original *descriptorStopped }

type descriptorStopped struct {
	reference *managerReference
	reloaded  bool // serialized by reference.ioMu
	closeOnce sync.Once
	closeErr  error
}

// OpenDescriptorStopped returns partial connection custody on error. The
// creator must Close every non-nil result, including failed admission.
func OpenDescriptorStopped(ctx context.Context) (*DescriptorStopped, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorUnitReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorStopped{original: &descriptorStopped{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = s.CheckAbsent()
	}
	return s, err
}

// OpenDescriptorInstalledStopped retains only the already installed fixed Descriptor
// service. It has the same no-job vocabulary as initial stopped installation;
// a server reference is physical custody, never Start or stopped-policy proof.
func OpenDescriptorInstalledStopped(ctx context.Context) (*DescriptorStopped, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorUnitReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorStopped{original: &descriptorStopped{reference: r, reloaded: true}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = s.operation(func(ctx context.Context, r *managerReference) error {
			call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", "ardents-descriptor.service")
			if err != nil {
				return err
			}
			if len(call.Body) != 0 {
				return ErrObservation
			}
			return nil
		})
	}
	if err == nil {
		_, _, err = s.ObserveLoaded()
	}
	return s, err
}

func (s *DescriptorStopped) operation(effect func(context.Context, *managerReference) error) (err error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.performOriginal(effect)
}

// CheckAbsent refuses every loaded or installed Node slot and every associated
// socket, including inactive units. It neither loads nor retains a unit. Returned
// paths are detached manager search paths, not writable-path authorization.
func (s *DescriptorStopped) CheckAbsent() ([]string, error) {
	err := s.operation(func(ctx context.Context, r *managerReference) error {
		if s.original.reloaded {
			return ErrObservation
		}
		return observeNodeSlotsAbsent(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	return append([]string(nil), s.original.reference.paths...), nil
}

// CheckOriginal reobserves original manager identity/configuration without a
// phase-specific unit assertion. A later caller still needs its own phase check.
func (s *DescriptorStopped) CheckOriginal() error {
	return s.operation(func(_ context.Context, _ *managerReference) error { return nil })
}

// ManagerIdentity observes the original broker and unique manager; it grants no
// Installation or recovery right and never reconnects or borrows another client.
func (s *DescriptorStopped) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

// Reload attempts exactly one fixed manager effect. An uncertain reply remains
// failed and cannot be retried on this connection or reported as completion.
func (s *DescriptorStopped) Reload() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.reloadOriginal(&s.original.reloaded)
}

// ObserveLoaded returns complete fixed Descriptor Unit/Service facts, surrounded
// by absence checks of the other four Node slots. It grants no policy or startup
// permission. Source's unit and lifetime are never referenced or observed here.
func (s *DescriptorStopped) ObserveLoaded() (unit, service Properties, err error) {
	err = s.operation(func(ctx context.Context, r *managerReference) error {
		if !s.original.reloaded {
			return ErrObservation
		}
		for round := 0; round < 2; round++ {
			for _, name := range nodeUnitNames {
				if name == "ardents-descriptor.service" {
					continue
				}
				if err := nodeSlotAbsent(ctx, r, name); err != nil {
					return err
				}
			}
			if err := nodeNameAbsent(ctx, r, "ardents-descriptor.socket"); err != nil {
				return err
			}
			if round != 0 {
				break
			}
			call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.LoadUnit", "ardents-descriptor.service")
			if err != nil {
				return err
			}
			if len(call.Body) != 1 {
				return ErrObservation
			}
			object, ok := call.Body[0].(dbus.ObjectPath)
			if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice" {
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
func (s *DescriptorStopped) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	s.original.closeOnce.Do(func() {
		r := s.original.reference
		s.original.closeErr = errors.Join(r.close(), r.original.Err())
	})
	return s.original.closeErr
}

// UnitPaths returns freshly checked detached original search-path declarations.
func (s *DescriptorStopped) UnitPaths() ([]string, error) {
	if err := s.CheckOriginal(); err != nil {
		return nil, err
	}
	return append([]string(nil), s.original.reference.paths...), nil
}
