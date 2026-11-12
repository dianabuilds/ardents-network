package systemd

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
)

// NodeSlots retains one root caller's independent read-only manager connection
// and the absence of all five fixed Node service/socket slots. Every call
// reobserves the original manager and search paths. These copied observations
// grant no account, filesystem, Reload, LoadUnit, Start or runtime permission.
type NodeSlots struct{ original *nodeSlots }

type nodeSlots struct {
	reference *managerReference
	once      sync.Once
	result    error
}

// OpenNodeSlots returns every partially acquired connection to its creator for
// Close. It accepts no unit selector and shares no other owner's live client.
func OpenNodeSlots(ctx context.Context) (*NodeSlots, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, nodeSlotsReply)
	if r == nil {
		return nil, err
	}
	s := &NodeSlots{original: &nodeSlots{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = s.CheckAbsent()
	}
	return s, err
}

// CheckAbsent loads or references nothing. Present, installed, uncertain or
// malformed service/socket observations all refuse, including inactive units.
func (s *NodeSlots) CheckAbsent() ([]string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil, ErrInput
	}
	r := s.original.reference
	if err := r.performOriginal(observeNodeSlotsAbsent); err != nil {
		return nil, err
	}
	return append([]string(nil), r.paths...), nil
}

// CheckOriginal retains this same original manager after the caller's separate
// native observations. A later absence decision still requires CheckAbsent.
func (s *NodeSlots) CheckOriginal() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return ErrInput
	}
	return s.original.reference.performOriginal(func(context.Context, *managerReference) error { return nil })
}

// Close first denies and interrupts, then joins the original manager users and
// physical transport. Copies share the same retained completion result.
func (s *NodeSlots) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	p := s.original
	p.once.Do(func() { p.result = errors.Join(p.reference.close(), p.reference.original.Err()) })
	return p.result
}

func observeNodeSlotsAbsent(ctx context.Context, r *managerReference) error {
	for _, unit := range nodeUnitNames {
		if err := nodeSlotAbsent(ctx, r, unit); err != nil {
			return err
		}
	}
	return nil
}

var nodeUnitNames = [...]string{
	"ardents-forwarding.service", "ardents-issuer.service",
	"ardents-descriptor.service", "ardents-introduction.service", "ardents-join.service",
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
