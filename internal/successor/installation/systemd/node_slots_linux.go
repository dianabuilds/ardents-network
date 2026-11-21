package systemd

import (
	"context"
	"errors"

	"github.com/godbus/dbus/v5"
)

// The fixed Node service/socket slot absence observation is shared by the
// platform sibling-absence check and the retained read-only role managers. The
// transitional root NodeSlots custody wrapper retired with the last installed
// Descriptor preflight (#604 FJD stage D, ADR-0125); these copied absence
// observations grant no account, filesystem, Reload, LoadUnit, Start or runtime
// permission. Every call reobserves the original manager on its own connection.
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
