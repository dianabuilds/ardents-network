package systemd

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// ManagerIdentity returns detached broker-instance/unique-owner facts from the
// original connection. It does not grant reload, stopped-policy or Start rights.
func (s *SourceStopped) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.owner.sourceManagerIdentity()
}

func (s *SourceReplacement) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.owner.sourceManagerIdentity()
}

// ObserveSourceManagerIdentity owns a fresh root read-only connection and joins
// it before returning. Historical bytes cannot supply this fresh observation.
func ObserveSourceManagerIdentity(ctx context.Context) (identity string, err error) {
	if ctx == nil || os.Geteuid() != 0 {
		return "", ErrInput
	}
	r, err := openSourceReference(ctx)
	if r != nil {
		defer func() {
			err = errors.Join(err, r.close(), ctx.Err())
			if err != nil {
				identity = ""
			}
		}()
	}
	if err != nil {
		return "", err
	}
	return r.sourceManagerIdentity()
}

func (r *sourcePreflight) sourceManagerIdentity() (identity string, err error) {
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
		err = r.sourceResult(err, ctx)
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
