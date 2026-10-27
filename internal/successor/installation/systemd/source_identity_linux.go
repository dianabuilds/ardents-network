package systemd

import (
	"context"
	"errors"
	"os"
)

// ManagerIdentity returns detached broker-instance/unique-owner facts from the
// original connection. It does not grant reload, stopped-policy or Start rights.
func (s *SourceStopped) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.owner.managerIdentity()
}

func (s *SourceReplacement) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.owner.managerIdentity()
}

// ObserveSourceManagerIdentity owns a fresh root read-only connection and joins
// it before returning. Historical bytes cannot supply this fresh observation.
func ObserveSourceManagerIdentity(ctx context.Context) (identity string, err error) {
	if ctx == nil || os.Geteuid() != 0 {
		return "", ErrInput
	}
	r, err := openManagerReference(ctx, sourceUnitReply)
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
	return r.managerIdentity()
}
