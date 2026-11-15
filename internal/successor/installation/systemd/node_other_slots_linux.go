package systemd

import (
	"context"
	"errors"
	"github.com/godbus/dbus/v5"
)

// The same fixed absence requests retain their original before/after round.
// All independent read-only requests are submitted on this original bus before
// awaiting their replies. Both exact absence errors are required for every
// name. The original requestObservation owns their shared deadline/interruption;
// every issued request joins even when another reply refuses. No verdict is
// cached, bus method added or native observation round removed.
func otherNodeSlotsAbsent(ctx context.Context, r *managerReference, purpose fixedUnitPurpose) error {
	if ctx == nil || r == nil || r.client.bus == nil {
		return ErrInput
	}
	frame, ok := ctx.Value(requestObservationKey{}).(*requestObservation)
	if !ok || frame.client != &r.client || !frame.active.Load() {
		return ErrInput
	}
	if r.client.transport == nil || !purpose.matches(r.client.transport.unitReply, false) {
		return ErrInput
	}
	names := otherNodeSlotNames(purpose)
	if len(names) != 9 {
		return ErrInput
	}
	methods := []struct{ name, absent string }{
		{"GetUnit", "org.freedesktop.systemd1.NoSuchUnit"},
		{"GetUnitFileState", "org.freedesktop.DBus.Error.FileNotFound"},
	}
	var calls [18]*dbus.Call
	var expected [18]string
	var failures [18]error
	position := 0
	for _, name := range names {
		for _, method := range methods {
			if err := ctx.Err(); err != nil {
				failures[position] = err
			} else {
				calls[position] = r.client.bus.Object(r.client.manager, "/org/freedesktop/systemd1").GoWithContext(ctx, "org.freedesktop.systemd1.Manager."+method.name, 0, make(chan *dbus.Call, 1), name)
				expected[position] = method.absent
			}
			position++
		}
	}
	for i, call := range calls {
		if call == nil {
			continue
		}
		answer := <-call.Done
		err := errors.Join(r.client.callFailure(ctx, answer.Err), ctx.Err())
		var refusal dbus.Error
		if !errors.As(err, &refusal) || refusal.Name != expected[i] {
			if nodeAbsenceContextOnly(ctx, err) {
				failures[i] = err
			} else {
				failures[i] = errors.Join(ErrObservation, err)
			}
		}
	}
	return errors.Join(errors.Join(failures[:]...), ctx.Err())
}

// otherNodeSlotNames preserves the exact per-role observation roster.
func otherNodeSlotNames(purpose fixedUnitPurpose) []string {
	var names []string
	switch purpose {
	case joinControlPurpose:
		names = []string{"ardents-forwarding.service", "ardents-forwarding.socket", "ardents-issuer.service", "ardents-issuer.socket", "ardents-descriptor.service", "ardents-descriptor.socket", "ardents-introduction.service", "ardents-introduction.socket", "ardents-join.socket"}
	case descriptorControlPurpose:
		names = []string{"ardents-forwarding.service", "ardents-forwarding.socket", "ardents-issuer.service", "ardents-issuer.socket", "ardents-join.service", "ardents-join.socket", "ardents-introduction.service", "ardents-introduction.socket", "ardents-descriptor.socket"}
	case introductionControlPurpose:
		names = []string{"ardents-forwarding.service", "ardents-forwarding.socket", "ardents-issuer.service", "ardents-issuer.socket", "ardents-join.service", "ardents-join.socket", "ardents-descriptor.service", "ardents-descriptor.socket", "ardents-introduction.socket"}
	}
	return names
}
