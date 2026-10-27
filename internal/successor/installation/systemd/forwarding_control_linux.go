package systemd

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

// ForwardingJob records only whether the finite fixed-unit request was attempted
// and its canonical native reply. It proves neither a process nor completion;
// an attempted request with an error may already have affected the manager.
type ForwardingJob struct {
	Attempted bool
	Object    string
}

// Start attempts the fixed Forwarding StartUnit exactly once, with mode fail and
// the original caller/manager/connection. Installation must admit the effect
// beforehand and retain an uncertain attempt through original physical join.
func (s *ForwardingControl) Start() (result ForwardingJob, err error) {
	if s == nil || s.original == nil || s.original.reference == nil || os.Geteuid() != 0 {
		return result, ErrInput
	}
	o := s.original
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
	if err = o.admitForwardingStart(); err != nil {
		return result, err
	}
	result.Attempted = true
	result.Object, err = r.forwardingJob(ctx, "org.freedesktop.systemd1.Manager.StartUnit")
	if err != nil {
		return result, err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return result, err
	}
	return result, r.client.transport.socket.SetDeadline(time.Time{})
}

// Stop is cleanup of this control's own attempted fixed Forwarding start. A bounded
// cleanup caller may outlive original admission; it cannot clear the original
// failure, reconnect, start another invocation or supply join. Installation
// separately reobserves its actual original process/scope before calling Stop.
func (s *ForwardingControl) Stop(ctx context.Context) (result ForwardingJob, err error) {
	if s == nil || s.original == nil || s.original.reference == nil || ctx == nil || os.Geteuid() != 0 {
		return result, ErrInput
	}
	o := s.original
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
	if err = o.admitForwardingStop(bounded); err != nil {
		return result, err
	}
	result.Attempted = true
	result.Object, err = r.forwardingJob(bounded, "org.freedesktop.systemd1.Manager.StopUnit")
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
func (s *forwardingControl) admitForwardingStart() error {
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

func (s *forwardingControl) admitForwardingStop(ctx context.Context) error {
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
func (r *managerReference) forwardingJob(ctx context.Context, method string) (string, error) {
	if method != "org.freedesktop.systemd1.Manager.StartUnit" && method != "org.freedesktop.systemd1.Manager.StopUnit" {
		return "", ErrInput
	}
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", method, "ardents-forwarding.service", "fail")
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
