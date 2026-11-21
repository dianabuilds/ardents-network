package systemd

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// EndpointReference retains an original loaded unit without changing its
// configuration or starting it. Observations grant no Installation authority.
// Caller cancellation denies effects at the domain; Close releases this
// physical reference only after the caller has joined its original processes.
type EndpointReference struct {
	mu        sync.Mutex
	transport *referenceTransport
	bus       *dbus.Conn
	manager   string
	attempted bool
	closed    bool
	terminal  error
}

// RetainEndpoint returns partial custody on failure, which must also be closed.
// Neither the bus address nor unit, method, auth mechanism or manager identity
// is selected by public input. A replacement manager is never rebound.
func RetainEndpoint(ctx context.Context) (result *EndpointReference, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	acquire, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	socket, err := (&net.Dialer{}).DialContext(acquire, "unix", "/run/dbus/system_bus_socket")
	if err != nil {
		return nil, errors.Join(ErrUnavailable, err)
	}
	unix, ok := socket.(*net.UnixConn)
	if !ok {
		return nil, errors.Join(ErrUnavailable, socket.Close())
	}
	ref := &EndpointReference{transport: &referenceTransport{socket: unix}}
	result = ref
	defer func() {
		if returnedErr != nil {
			ref.terminal = errors.Join(ErrUnavailable, returnedErr)
		}
	}()
	raw, err := unix.SyscallConn()
	if err != nil {
		return ref, err
	}
	var peer *syscall.Ucred
	var nativeErr error
	if err = raw.Control(func(fd uintptr) {
		peer, nativeErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || nativeErr != nil || peer == nil || peer.Uid != 0 || peer.Pid <= 0 {
		return ref, errors.Join(ErrObservation, err, nativeErr)
	}
	if err = unix.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return ref, err
	}
	joined := make(chan struct{})
	stop := context.AfterFunc(acquire, func() { _ = ref.transport.Close(); close(joined) })
	defer func() {
		if !stop() {
			<-joined
			returnedErr = errors.Join(returnedErr, acquire.Err())
		}
	}()
	ref.bus, err = dbus.NewConn(ref.transport, dbus.WithContext(context.WithoutCancel(ctx)))
	if err != nil {
		return ref, err
	}
	if err = ref.bus.Auth([]dbus.Auth{dbus.AuthExternal("0")}); err != nil {
		return ref, err
	}
	if err = ref.bus.Hello(); err != nil {
		return ref, err
	}
	call := ref.bus.BusObject().CallWithContext(acquire, "org.freedesktop.DBus.GetNameOwner", 0, "org.freedesktop.systemd1")
	if err = call.Store(&ref.manager); err != nil {
		return ref, err
	}
	if !strings.HasPrefix(ref.manager, ":") || len(ref.manager) > 255 {
		return ref, ErrObservation
	}
	ref.transport.owner.Store(ref.manager)
	ref.attempted = true // an uncertain reply still owns possible server reference
	call = ref.bus.Object(ref.manager, "/org/freedesktop/systemd1").CallWithContext(acquire, "org.freedesktop.systemd1.Manager.RefUnit", 0, "ardents-endpoint.service")
	if call.Err != nil || len(call.Body) != 0 {
		return ref, errors.Join(ErrObservation, call.Err)
	}
	if err = acquire.Err(); err != nil {
		return ref, err
	}
	if err = unix.SetDeadline(time.Time{}); err != nil {
		return ref, err
	}
	return ref, nil
}

// Observe may use the domain's explicit physical-cleanup context. It still
// observes the same connection and manager and grants no renewed effect right.
func (ref *EndpointReference) Observe(ctx context.Context) error {
	if ref == nil || ctx == nil {
		return ErrInput
	}
	ref.mu.Lock()
	defer ref.mu.Unlock()
	if ref.closed || ref.bus == nil || !ref.attempted {
		return ErrInput
	}
	if ref.terminal != nil {
		return ref.terminal
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ref.bus.Connected() {
		ref.terminal = ErrUnavailable
		return ref.terminal
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := ref.transport.socket.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		ref.terminal = err
		return err
	}
	var owner string
	call := ref.bus.BusObject().CallWithContext(bounded, "org.freedesktop.DBus.GetNameOwner", 0, "org.freedesktop.systemd1")
	err := call.Store(&owner)
	if err != nil || owner != ref.manager {
		ref.terminal = errors.Join(ErrObservation, err)
		return ref.terminal
	}
	call = ref.bus.Object(ref.manager, "/org/freedesktop/systemd1").CallWithContext(bounded, "org.freedesktop.DBus.Peer.Ping", 0)
	if call.Err != nil || len(call.Body) != 0 {
		ref.terminal = errors.Join(ErrObservation, call.Err)
	}
	return errors.Join(ref.terminal, ctx.Err())
}

func (ref *EndpointReference) Close() error {
	if ref == nil {
		return nil
	}
	ref.mu.Lock()
	defer ref.mu.Unlock()
	if ref.closed {
		return ref.terminal
	}
	ref.closed = true
	if ref.attempted && ref.bus != nil && ref.bus.Connected() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		ref.terminal = errors.Join(ref.terminal, ref.transport.socket.SetWriteDeadline(time.Now().Add(3*time.Second)))
		call := ref.bus.Object(ref.manager, "/org/freedesktop/systemd1").CallWithContext(cleanup, "org.freedesktop.systemd1.Manager.UnrefUnit", 0, "ardents-endpoint.service")
		ref.terminal = errors.Join(ref.terminal, call.Err)
		if len(call.Body) != 0 {
			ref.terminal = errors.Join(ref.terminal, ErrObservation)
		}
		cancel()
	}
	// Interrupt first: the client's output lock must not wait on blocked I/O.
	ref.terminal = errors.Join(ref.terminal, ref.transport.Close())
	if ref.bus != nil {
		ref.terminal = errors.Join(ref.terminal, ref.bus.Close())
	}
	return ref.terminal
}
