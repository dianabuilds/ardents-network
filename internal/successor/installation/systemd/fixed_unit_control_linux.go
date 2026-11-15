package systemd

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

// fixedUnitControl owns exactly one role-specific manager reference and its one
// Start/cleanup-Stop attempt. Separate constructed instances never share state.
// The fixed unit is chosen only by the role's private constructor.
type fixedUnitControl struct {
	reference                     *managerReference
	purpose                       fixedUnitPurpose
	startAttempted, stopAttempted bool
}
type fixedUnitPurpose uint8

const (
	joinControlPurpose fixedUnitPurpose = iota + 1
	descriptorControlPurpose
	introductionControlPurpose
	forwardingControlPurpose
	sourceControlPurpose
)

func (p fixedUnitPurpose) unit() string {
	switch p {
	case joinControlPurpose:
		return "ardents-join.service"
	case descriptorControlPurpose:
		return "ardents-descriptor.service"
	case introductionControlPurpose:
		return "ardents-introduction.service"
	case forwardingControlPurpose:
		return "ardents-forwarding.service"
	case sourceControlPurpose:
		return "ardents-source.service"
	default:
		return ""
	}
}

// The original connection's decoder vocabulary must match the fixed request.
// Replacement uses the corresponding read-only unit vocabulary for RefUnit;
// only the distinct control vocabulary admits Node Start/Stop replies.
func (p fixedUnitPurpose) matches(reply managerUnitReply, control bool) bool {
	switch p {
	case sourceControlPurpose:
		return reply == sourceUnitReply
	case forwardingControlPurpose:
		return reply == forwardingUnitReply
	case joinControlPurpose:
		return reply == joinControlReply || !control && reply == joinUnitReply
	case descriptorControlPurpose:
		return reply == descriptorControlReply || !control && reply == descriptorUnitReply
	case introductionControlPurpose:
		return reply == introductionControlReply || !control && reply == introductionUnitReply
	default:
		return false
	}
}

type fixedUnitJob struct {
	Attempted bool
	Object    string
}

// Start attempts the fixed role StartUnit exactly once, with mode fail and
// the original caller/manager/connection. Installation must admit the effect
// beforehand and retain an uncertain attempt through original physical join.
func (o *fixedUnitControl) start() (result fixedUnitJob, err error) {
	if o == nil || o.reference == nil || o.reference.client.transport == nil || !o.purpose.matches(o.reference.client.transport.unitReply, true) || os.Geteuid() != 0 {
		return result, ErrInput
	}
	r := o.reference
	if err = r.begin(); err != nil {
		return result, err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() { err = r.managerResult(err, ctx) }()
	if err = r.local(); err != nil {
		return result, err
	}
	if o.startAttempted || o.stopAttempted {
		return result, ErrObservation
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return result, err
	}
	if err = r.matchManagerDeclaration(ctx); err != nil {
		return result, err
	}
	if err = r.local(); err != nil {
		return result, err
	}
	if err = o.admitStart(); err != nil {
		return result, err
	}
	result.Attempted = true
	result.Object, err = o.job(ctx, "org.freedesktop.systemd1.Manager.StartUnit")
	if err != nil {
		return result, err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return result, err
	}
	return result, r.client.transport.socket.SetDeadline(time.Time{})
}

// Stop is cleanup of this control's own attempted fixed role start. A bounded
// cleanup caller may outlive original admission; it cannot clear the original
// failure, reconnect, start another invocation or supply completion. Installation
// separately reobserves its actual original process/scope before calling Stop.
func (o *fixedUnitControl) stop(ctx context.Context) (result fixedUnitJob, err error) {
	if o == nil || o.reference == nil || o.reference.client.transport == nil || !o.purpose.matches(o.reference.client.transport.unitReply, true) || ctx == nil || os.Geteuid() != 0 {
		return result, ErrInput
	}
	r := o.reference
	r.mu.Lock()
	if r.closed.Load() {
		r.mu.Unlock()
		return result, ErrUnavailable
	}
	r.uses.Add(1)
	r.mu.Unlock()
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	defer func() {
		err = errors.Join(err, bounded.Err(), ctx.Err())
		if err != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, err)
			r.mu.Unlock()
		}
	}()
	if !o.startAttempted || o.stopAttempted {
		return result, ErrObservation
	}
	if err = bounded.Err(); err != nil {
		return result, err
	}
	if !r.client.bus.Connected() {
		return result, ErrUnavailable
	}
	if err = r.client.sameOwner(bounded, r.client.manager); err != nil {
		return result, err
	}
	if err = r.matchManagerDeclaration(bounded); err != nil {
		return result, err
	}
	if err = o.admitStop(bounded); err != nil {
		return result, err
	}
	result.Attempted = true
	result.Object, err = o.job(bounded, "org.freedesktop.systemd1.Manager.StopUnit")
	if err != nil {
		return result, err
	}
	if err = r.client.sameOwner(bounded, r.client.manager); err != nil {
		return result, err
	}
	return result, r.client.transport.socket.SetDeadline(time.Time{})
}

// The caller holds the physical I/O mutex. These short local gates contain no
// foreign observation and order an in-flight native attempt against Close.
func (s *fixedUnitControl) admitStart() error {
	r := s.reference
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return ErrUnavailable
	}
	if r.terminal != nil {
		return r.terminal
	}
	if err := r.original.Err(); err != nil {
		return err
	}
	if s.startAttempted || s.stopAttempted {
		return ErrObservation
	}
	s.startAttempted = true
	return nil
}

func (s *fixedUnitControl) admitStop(ctx context.Context) error {
	r := s.reference
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.startAttempted || s.stopAttempted {
		return ErrObservation
	}
	s.stopAttempted = true
	return nil
}

// The selector is private and reached only by the two closed public purposes.
func (o *fixedUnitControl) job(ctx context.Context, method string) (string, error) {
	if method != "org.freedesktop.systemd1.Manager.StartUnit" && method != "org.freedesktop.systemd1.Manager.StopUnit" {
		return "", ErrInput
	}
	call, err := o.reference.client.call(ctx, "/org/freedesktop/systemd1", method, o.purpose.unit(), "fail")
	if err != nil {
		return "", err
	}
	if len(call.Body) != 1 {
		return "", ErrObservation
	}
	object, ok := call.Body[0].(dbus.ObjectPath)
	if !ok || !managerJobPath(string(object)) {
		return "", ErrObservation
	}
	return string(object), nil
}

// retainUnit holds this fixed Node unit on its own original manager connection.
func (r *managerReference) retainFixedUnit(purpose fixedUnitPurpose) (err error) {
	if r == nil || r.client.transport == nil || !purpose.matches(r.client.transport.unitReply, false) || purpose == sourceControlPurpose {
		return ErrInput
	}
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
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", purpose.unit())
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
