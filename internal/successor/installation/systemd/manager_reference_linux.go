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

// managerReference owns only one authenticated physical bus connection and its
// serialized observations. Fixed-purpose owners each create a fresh reference;
// none borrows another owner's connection, lifetime, unit or admission rights.
type managerReference struct {
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

// managerUnitReply binds only the physical LoadUnit reply grammar of this
// connection. Fixed-purpose owners still select their own methods and compare
// their own complete policy. Neither vocabulary grants Start or other rights.
type managerUnitReply uint8

const (
	sourceUnitReply managerUnitReply = iota
	forwardingUnitReply
)

func (reply managerUnitReply) object() string {
	switch reply {
	case sourceUnitReply:
		return "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"
	case forwardingUnitReply:
		return "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice"
	default:
		return ""
	}
}

// Each fixed-purpose opener owns a new physical connection. This helper
// authenticates only the actual caller; it accepts no supplied UID, unit or bus path.
func openManagerReference(ctx context.Context, reply managerUnitReply) (result *managerReference, returnedErr error) {
	if ctx == nil || reply.object() == "" {
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
	t := &referenceTransport{socket: u, properties: true, unitReply: reply}
	r := &managerReference{original: ctx, client: startupBus{transport: t, interruptedIO: new(atomic.Bool)}}
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

func (r *managerReference) begin() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.terminal != nil {
		return r.terminal
	}
	if err := r.original.Err(); err != nil {
		r.terminal = err
		return err
	}
	if r.closed.Load() {
		return ErrUnavailable
	}
	r.uses.Add(1)
	return nil
}
func (r *managerReference) local() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.terminal != nil {
		return r.terminal
	}
	// An original cancellation may race physical Close while an admitted
	// observer waits for ioMu. Close's local denial is not a second physical
	// failure; an earlier terminal failure above still retains precedence.
	if err := r.original.Err(); err != nil {
		return err
	}
	if r.closed.Load() {
		return ErrUnavailable
	}
	return nil
}

// A caller can cancel and Close can disconnect the bus after the first local
// admission check. Reobserve that original denial before synthesizing a missing
// connection error; an earlier retained failure still has local precedence.
func (r *managerReference) unavailable() error {
	if err := r.local(); err != nil {
		return err
	}
	return ErrUnavailable
}

func (r *managerReference) close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed.Store(true)
		r.mu.Unlock()
		// Mark only this original owner's interruption before closing the socket.
		// Concurrent calls can then retain its context cause for the induced
		// net.ErrClosed, while EOF and independent physical failures survive.
		interrupted := r.client.interrupt()
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

func (r *managerReference) managerDeclaration(ctx context.Context) (string, []string, error) {
	get := func(name string) (dbus.Variant, error) {
		call, err := r.client.call(ctx, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties.Get", "org.freedesktop.systemd1.Manager", name)
		var value dbus.Variant
		if err != nil {
			return value, err
		}
		err = call.Store(&value)
		if err != nil {
			return value, errors.Join(ErrObservation, err)
		}
		return value, nil
	}
	v, err := get("Version")
	if err != nil {
		return "", nil, err
	}
	if v.Signature().String() != "s" {
		return "", nil, ErrObservation
	}
	version, ok := v.Value().(string)
	if !ok || !(version == "255" || strings.HasPrefix(version, "255.")) {
		return "", nil, ErrObservation
	}
	v, err = get("UnitPath")
	if err != nil {
		return "", nil, err
	}
	if v.Signature().String() != "as" {
		return "", nil, ErrObservation
	}
	paths, ok := v.Value().([]string)
	if !ok || len(paths) == 0 || len(paths) > 32 {
		return "", nil, ErrObservation
	}
	return version, append([]string(nil), paths...), nil
}

func (r *managerReference) initializeManagerDeclaration() (err error) {
	if err = r.begin(); err != nil {
		return err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() { err = r.managerResult(err, ctx) }()
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	version, paths, err := r.managerDeclaration(ctx)
	if err != nil {
		return err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return err
	}
	if err = r.local(); err != nil {
		return err
	}
	r.version, r.paths = version, paths
	return r.client.transport.socket.SetDeadline(time.Time{})
}

func (r *managerReference) managerResult(err error, ctx context.Context) error {
	err = r.client.callFailure(ctx, err)
	err = errors.Join(err, ctx.Err(), r.original.Err())
	if err != nil {
		r.mu.Lock()
		r.terminal = errors.Join(r.terminal, err)
		r.mu.Unlock()
	}
	return err
}

func (r *managerReference) matchManagerDeclaration(ctx context.Context) error {
	version, paths, err := r.managerDeclaration(ctx)
	if err != nil {
		return err
	}
	if r.version == "" || version != r.version || strings.Join(paths, "\x00") != strings.Join(r.paths, "\x00") {
		return ErrObservation
	}
	return nil
}

func (r *managerReference) managerProperties() error {
	for _, name := range []string{"Version", "UnitPath"} {
		call, e := r.client.call(r.original, "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties.Get", "org.freedesktop.systemd1.Manager", name)
		if e != nil {
			return e
		}
		var v dbus.Variant
		if e = call.Store(&v); e != nil {
			return e
		}
		if name == "Version" {
			value, ok := v.Value().(string)
			if !ok || v.Signature().String() != "s" || value != r.version {
				return ErrObservation
			}
		} else {
			value, ok := v.Value().([]string)
			if !ok || v.Signature().String() != "as" || strings.Join(value, "\x00") != strings.Join(r.paths, "\x00") {
				return ErrObservation
			}
		}
	}
	return nil
}

func (r *managerReference) managerIdentity() (identity string, err error) {
	if r == nil {
		return "", ErrInput
	}
	if err = r.begin(); err != nil {
		return "", err
	}
	defer r.uses.Done()
	r.ioMu.Lock()
	defer r.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	defer func() {
		err = r.managerResult(err, ctx)
		if err != nil {
			identity = ""
		}
	}()
	if err = r.local(); err != nil {
		return "", err
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return "", err
	}
	answer, err := r.client.request(ctx, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus.GetId")
	if err != nil {
		return "", err
	}
	var broker string
	if err = answer.Store(&broker); err != nil {
		return "", err
	}
	if len(broker) != 32 || !strings.HasPrefix(r.client.manager, ":") || len(r.client.manager) > 255 {
		return "", ErrObservation
	}
	for _, c := range broker {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", ErrObservation
		}
	}
	if err = r.client.sameOwner(ctx, r.client.manager); err != nil {
		return "", err
	}
	if err = errors.Join(r.client.transport.socket.SetDeadline(time.Time{}), r.local()); err != nil {
		return "", err
	}
	// GetId's128-bit UUID is32 hex digits. Normalize the textual case before
	// comparison; it identifies the broker instance, not the machine.
	// https://dbus.freedesktop.org/doc/dbus-specification.html#bus-messages-get-id
	return strings.ToLower(broker) + "/" + r.client.manager, nil
}
