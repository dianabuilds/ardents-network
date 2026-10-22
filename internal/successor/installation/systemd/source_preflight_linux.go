package systemd

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// SourcePreflight owns one read-only original manager connection. It never
// loads, references, reloads or starts a unit. Detached paths grant no authority.
type SourcePreflight struct{ owner *sourcePreflight }
type sourcePreflight struct {
	client    startupBus
	original  context.Context
	version   string
	paths     []string
	mu        sync.Mutex // short admission/result state only
	ioMu      sync.Mutex // physical observation serialization
	uses      sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
	terminal  error
	// Test-only barrier after actual original manager I/O, before handoff.
	// No observation or authority can be injected through it.
	observe func()
}

// OpenSourcePreflight returns partial custody after connection acquisition;
// the creator must Close even after refusal. No bus/name/method is configurable.
func OpenSourcePreflight(ctx context.Context) (result *SourcePreflight, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openSourceReference(ctx)
	if r == nil {
		return nil, err
	}
	return &SourcePreflight{owner: r}, err
}

// Each fixed-purpose Source opener owns a new physical connection. This helper
// authenticates only the actual caller; it accepts no supplied UID or bus path.
func openSourceReference(ctx context.Context) (result *sourcePreflight, returnedErr error) {
	if ctx == nil {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	socket, err := (&net.Dialer{}).DialContext(bounded, "unix", "/run/dbus/system_bus_socket")
	if err != nil {
		return nil, startupDialFailure(bounded, err)
	}
	u, ok := socket.(*net.UnixConn)
	if !ok {
		return nil, errors.Join(ErrObservation, socket.Close())
	}
	t := &referenceTransport{socket: u, properties: true}
	r := &sourcePreflight{original: ctx, client: startupBus{transport: t, interruptedIO: new(atomic.Bool)}}
	result = r
	defer func() {
		if returnedErr != nil {
			r.terminal = returnedErr
		}
	}()
	joined := make(chan struct{})
	stop := context.AfterFunc(bounded, func() { _ = r.client.interrupt(); close(joined) })
	defer func() {
		if !stop() {
			<-joined
		}
		returnedErr = errors.Join(returnedErr, bounded.Err())
	}()
	raw, err := u.SyscallConn()
	if err != nil {
		return result, err
	}
	var peer *syscall.Ucred
	var native error
	if err = raw.Control(func(fd uintptr) {
		peer, native = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || native != nil || peer == nil || peer.Uid != 0 || peer.Pid <= 0 {
		return result, errors.Join(ErrObservation, err, native)
	}
	deadline, _ := bounded.Deadline()
	if err = u.SetDeadline(deadline); err != nil {
		return result, err
	}
	r.client.bus, err = dbus.NewConn(t, dbus.WithContext(context.WithoutCancel(ctx)))
	if err != nil {
		return result, err
	}
	if err = r.client.bus.Auth([]dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Geteuid()))}); err != nil {
		return result, err
	}
	if err = r.client.bus.Hello(); err != nil {
		return result, err
	}
	if err = r.client.owner(bounded, &r.client.manager); err != nil {
		return result, err
	}
	if !strings.HasPrefix(r.client.manager, ":") || len(r.client.manager) > 255 {
		return result, ErrObservation
	}
	t.owner.Store(r.client.manager)
	return result, nil
}

// Check obtains complete fixed namespace observations on the original manager.
// Repeated checks compare Version and UnitPath, and never reconnect after loss.
func (reference *SourcePreflight) Check() (pathsResult []string, returnedErr error) {
	if reference == nil || reference.owner == nil {
		return nil, ErrInput
	}
	r := reference.owner
	if err := r.begin(); err != nil {
		return nil, err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	defer func() {
		if returnedErr != nil {
			r.mu.Lock()
			r.terminal = errors.Join(r.terminal, returnedErr)
			r.mu.Unlock()
			pathsResult = nil
		}
	}()
	return r.observeAbsent()
}

// observeAbsent requires the original physical serialization and active-use
// borrow. Read-only preflight and stopped publication retain separate wrappers.
func (r *sourcePreflight) observeAbsent() (pathsResult []string, returnedErr error) {
	if err := r.local(); err != nil {
		return nil, err
	}
	if r.client.bus == nil || !r.client.bus.Connected() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	if err := r.client.sameOwner(ctx, r.client.manager); err != nil {
		return nil, err
	}
	version, paths, err := r.sourceDeclaration(ctx)
	if err != nil {
		return nil, err
	}
	for _, unit := range []string{"ardents-source.service", "ardents-source.socket"} {
		for _, method := range []struct{ name, absent string }{
			{"GetUnit", "org.freedesktop.systemd1.NoSuchUnit"},
			{"GetUnitFileState", "org.freedesktop.DBus.Error.FileNotFound"},
		} {
			_, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager."+method.name, unit)
			var refusal dbus.Error
			if !errors.As(err, &refusal) || refusal.Name != method.absent {
				return nil, errors.Join(ErrObservation, err)
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
	}
	if err := r.client.sameOwner(ctx, r.client.manager); err != nil {
		return nil, err
	}
	if r.observe != nil {
		r.observe()
	}
	if err := r.original.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.local(); err != nil {
		return nil, err
	}
	if err := r.client.sameOwner(ctx, r.client.manager); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed.Load() {
		return nil, ErrUnavailable
	}
	if r.terminal != nil {
		return nil, r.terminal
	}
	if r.version != "" && (r.version != version || strings.Join(r.paths, "\x00") != strings.Join(paths, "\x00")) {
		return nil, ErrObservation
	}
	r.version = version
	r.paths = append([]string(nil), paths...)
	return append([]string(nil), paths...), errors.Join(ctx.Err(), r.original.Err())
}

func (r *sourcePreflight) begin() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.terminal != nil {
		return r.terminal
	}
	if r.closed.Load() {
		return ErrUnavailable
	}
	if err := r.original.Err(); err != nil {
		r.terminal = err
		return err
	}
	r.uses.Add(1)
	return nil
}
func (r *sourcePreflight) local() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.terminal != nil {
		return r.terminal
	}
	if r.closed.Load() {
		return ErrUnavailable
	}
	return r.original.Err()
}

// Close first interrupts actual I/O, then joins the original serialized user,
// library goroutines and transport. Repeated calls retain the same close result.
func (reference *SourcePreflight) Close() error {
	if reference == nil || reference.owner == nil {
		return nil
	}
	return reference.owner.close()
}

func (r *sourcePreflight) close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed.Store(true)
		r.mu.Unlock()
		interrupted := r.client.transport.Close()
		r.uses.Wait()
		var busErr error
		if r.client.bus != nil {
			busErr = r.client.bus.Close()
		}
		r.mu.Lock()
		r.closeErr = errors.Join(r.terminal, interrupted, busErr)
		r.mu.Unlock()
	})
	return r.closeErr
}
