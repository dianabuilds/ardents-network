package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// DescriptorObservation is detached complete fixed-unit data. Installation must
// match its own generation, expected protection and original native invocation.
type DescriptorObservation struct {
	Manager, Version string
	UnitPaths        []string
	Unit, Service    Properties
}

// DescriptorRuntime owns an independently authenticated read-only connection.
// It exposes no load, reference, reload, start, stop, selector or reconnect.
type DescriptorRuntime struct{ original *descriptorRuntime }

type descriptorRuntime struct {
	reference *managerReference
	once      sync.Once
	closeErr  error
}

// DescriptorControl owns root's original fixed-unit reference and at most one
// start/cleanup-stop attempt. Neither a reference nor job reply grants admission
// or original process/domain completion. Closing returns the original server reference.
type DescriptorControl struct{ original *descriptorControl }

type descriptorControl struct {
	reference                     *managerReference
	startAttempted, stopAttempted bool // original physical I/O serialization
	once                          sync.Once
	closeErr                      error
}

func OpenDescriptorRuntime(ctx context.Context) (*DescriptorRuntime, error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorUnitReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorRuntime{original: &descriptorRuntime{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		_, err = r.descriptorLoadedObservation()
	}
	return s, err
}

func OpenDescriptorControl(ctx context.Context) (*DescriptorControl, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, descriptorControlReply)
	if r == nil {
		return nil, err
	}
	s := &DescriptorControl{original: &descriptorControl{reference: r}}
	if err == nil {
		err = r.initializeManagerDeclaration()
	}
	if err == nil {
		err = r.retainDescriptorControlUnit()
	}
	if err == nil {
		_, err = r.descriptorLoadedObservation()
	}
	if err == nil {
		// Source and non-root/read-only vocabularies remain separate. Only this
		// original fixed Descriptor control may receive native job object replies.
		r.client.transport.descriptorJobs.Store(true)
	}
	return s, err
}

func (r *managerReference) retainDescriptorControlUnit() (err error) {
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
	call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.RefUnit", "ardents-descriptor.service")
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

func (r *managerReference) descriptorLoadedObservation() (result DescriptorObservation, err error) {
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
			result = DescriptorObservation{}
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
			if err = descriptorOtherSlotsAbsent(ctx, r); err != nil {
				return err
			}
			if round != 0 {
				break
			}
			call, e := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager.GetUnit", "ardents-descriptor.service")
			if e != nil {
				return e
			}
			if len(call.Body) != 1 {
				return ErrObservation
			}
			object, ok := call.Body[0].(dbus.ObjectPath)
			if !ok || object != "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice" {
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

func (s *DescriptorRuntime) Observe() (DescriptorObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return DescriptorObservation{}, ErrInput
	}
	return s.original.reference.descriptorLoadedObservation()
}

func (s *DescriptorControl) Observe() (DescriptorObservation, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return DescriptorObservation{}, ErrInput
	}
	return s.original.reference.descriptorLoadedObservation()
}

// ManagerIdentity returns fresh detached broker-instance/unique-manager facts
// from this original connection, never another connection or accepting right.
func (s *DescriptorControl) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func (s *DescriptorRuntime) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

func (s *DescriptorControl) Close() error {
	if s == nil || s.original == nil || s.original.reference == nil {
		return nil
	}
	o := s.original
	o.once.Do(func() { o.closeErr = errors.Join(o.reference.close(), o.reference.original.Err()) })
	return o.closeErr
}

// The same fixed absence requests retain their original before/after round.
// All independent read-only requests are submitted on this original bus before
// awaiting their replies. Both exact absence errors are required for every
// name. The original requestObservation owns their shared deadline/interruption;
// every issued request joins even when another reply refuses. No verdict is
// cached, bus method added or native observation round removed.
func descriptorOtherSlotsAbsent(ctx context.Context, r *managerReference) error {
	if ctx == nil || r == nil || r.client.bus == nil {
		return ErrInput
	}
	frame, ok := ctx.Value(requestObservationKey{}).(*requestObservation)
	if !ok || frame.client != &r.client || !frame.active.Load() {
		return ErrInput
	}
	names := []string{"ardents-forwarding.service", "ardents-forwarding.socket", "ardents-issuer.service", "ardents-issuer.socket", "ardents-join.service", "ardents-join.socket", "ardents-introduction.service", "ardents-introduction.socket", "ardents-descriptor.socket"}
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
