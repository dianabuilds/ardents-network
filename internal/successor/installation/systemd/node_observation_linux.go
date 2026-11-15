package systemd

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

// fixedNodeObservation is detached output; each observation independently owns
// its original connection, manager identity, absence rounds and joined calls.
type fixedNodeObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

func (r *managerReference) fixedNodeObservation(purpose fixedUnitPurpose) (result fixedNodeObservation, err error) {
	if r == nil || r.client.transport == nil || !purpose.matches(r.client.transport.unitReply, false) || (purpose != joinControlPurpose && purpose != descriptorControlPurpose && purpose != introductionControlPurpose && purpose != forwardingControlPurpose) {
		return result, ErrInput
	}
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
			result = fixedNodeObservation{}
		}
	}()
	if err = r.local(); err != nil {
		return result, err
	}
	if !r.client.bus.Connected() {
		return result, r.unavailable()
	}
	err = r.client.observeRequests(ctx, func(ctx context.Context) error {
		if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
			return err
		}
		if err = r.matchManagerDeclaration(ctx); err != nil {
			return err
		}
		for round := 0; round < 2; round++ {
			if purpose == forwardingControlPurpose {
				for _, unit := range nodeUnitNames[1:] {
					if err = nodeSlotAbsent(ctx, r, unit); err != nil {
						return err
					}
				}
				if err = nodeNameAbsent(ctx, r, "ardents-forwarding.socket"); err != nil {
					return err
				}
			} else if err = otherNodeSlotsAbsent(ctx, r, purpose); err != nil {
				return err
			}
			if round != 0 {
				break
			}
			call, e := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", purpose.unit())
			if e != nil {
				return e
			}
			if len(call.Body) != 1 {
				return ErrObservation
			}
			object, ok := call.Body[0].(dbus.ObjectPath)
			if !ok || object != dbus.ObjectPath(r.client.transport.unitReply.object()) {
				return ErrObservation
			}
			for i, iface := range []string{"Unit", "Service"} {
				call, e = r.client.call(ctx, string(object), "org.freedesktop.DBus.Properties.GetAll", "org.freedesktop.systemd1."+iface)
				if e != nil {
					return e
				}
				properties, e := startupProperties(call)
				if e != nil {
					return e
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
			return err
		}
		if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
			return err
		}
		if err = r.local(); err != nil {
			return err
		}
		if err = r.client.transport.socket.SetDeadline(time.Time{}); err != nil {
			return err
		}
		result.Manager, result.Version, result.UnitPaths = r.client.manager, r.version, append([]string(nil), r.paths...)
		return nil
	})
	return result, err
}
