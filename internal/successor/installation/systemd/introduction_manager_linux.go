package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// IntroductionObservation is detached complete fixed-unit data. The repair must
// match its own generation, expected protection and original native invocation.
type IntroductionObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// IntroductionRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type IntroductionRuntime struct{ original *introductionRuntime }

type introductionRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// IntroductionControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain completion. Closing returns the original server reference.
type IntroductionControl struct{ original *introductionControl }

type introductionControl struct {
	reference                     *managerReference
	startAttempted, stopAttempted bool // original physical I/O serialization
	once                          sync.Once
	closeErr                      error
}

func OpenIntroductionRuntime(ctx context.Context) (*IntroductionRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, introductionUnitReply)
	if r == nil {
		return nil, err
	}
	s := &IntroductionRuntime{original: &introductionRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.introductionLoadedObservation()
	}
	return s, err
}

func OpenIntroductionControl(ctx context.Context) (*IntroductionControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, introductionControlReply)
	if r == nil {
		return nil, err
	}
	s := &IntroductionControl{original: &introductionControl{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainIntroductionControlUnit()
	}
	if err == nil {
		_, err = r.introductionLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Introduction control may receive native job object replies.
		r.client.transport.introductionJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) retainIntroductionControlUnit() (err error) {
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
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", "ardents-introduction.service")
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

func (r *managerReference) introductionLoadedObservation() (result IntroductionObservation, err error) {
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
			result = IntroductionObservation{}
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
			if err = introductionOtherSlotsAbsent(ctx, r); err != nil {
				return err
			}
			if round != 0 {
				break
			}
			call, e := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", "ardents-introduction.service")
			if e != nil {
				return e
			}
			if len(call.Body) != 1 {
				return ErrObservation
			}
			object, ok := call.Body[0].(dbus.ObjectPath)
			if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice" {
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

func (s *IntroductionRuntime) Observe() (IntroductionObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return IntroductionObservation{}, ErrInput
	}
	return s.original.reference.introductionLoadedObservation()
}

func (s *IntroductionControl) Observe() (IntroductionObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return IntroductionObservation{}, ErrInput
	}
	return s.original.reference.introductionLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *IntroductionControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *IntroductionRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *IntroductionControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

// The same fixed absence requests retain their original before/after round. All
// independent read-only requests are submitted on this original bus before
// awaiting their replies. Both exact absence errors are required for every name.
// Introduction shares the ardents-node account with sibling units; absence of the
// other fixed slots is observed per unit, never by an account-wide process scan.
func introductionOtherSlotsAbsent(ctx context.Context, r *managerReference) error {
	if ctx == nil || r == nil || r.client.bus == nil {
		return ErrInput
	}
	frame, ok := ctx.Value(requestObservationKey{}).(*requestObservation)
	if !ok || frame.client != &r.client || !frame.active.Load() {
		return ErrInput
	}
	names := []string{"ardents-forwarding.service", "ardents-forwarding.socket", "ardents-issuer.service", "ardents-issuer.socket", "ardents-join.service", "ardents-join.socket", "ardents-descriptor.service", "ardents-descriptor.socket", "ardents-introduction.socket"}
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
