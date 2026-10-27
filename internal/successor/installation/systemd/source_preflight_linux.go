package systemd

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// SourcePreflight owns one read-only original manager connection. It never
// loads, references, reloads or starts a unit. Detached paths grant no authority.
type SourcePreflight struct{ owner *managerReference }

// OpenSourcePreflight returns partial custody after connection acquisition;
// the creator must Close even after refusal. No bus/name/method is configurable.
func OpenSourcePreflight(ctx context.Context) (result *SourcePreflight, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	r, err := openManagerReference(ctx, sourceUnitReply)
	if r == nil {
		return nil, err
	}
	return &SourcePreflight{owner: r}, err
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
func (r *managerReference) observeAbsent() (pathsResult []string, returnedErr error) {
	if err := r.local(); err != nil {
		return nil, err
	}
	if r.client.bus == nil || !r.client.bus.Connected() {
		return nil, r.unavailable()
	}
	ctx, cancel := context.WithTimeout(r.original, 3*time.Second)
	defer cancel()
	if err := r.client.sameOwner(ctx, r.client.manager); err != nil {
		return nil, err
	}
	version, paths, err := r.managerDeclaration(ctx)
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

// Close first interrupts actual I/O, then joins the original serialized user,
// library goroutines and transport. Repeated calls retain the same close result.
func (reference *SourcePreflight) Close() error {
	if reference == nil || reference.owner == nil {
		return nil
	}
	return reference.owner.close()
}
