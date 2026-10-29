package systemd

import (
	"context"
	"errors"
	"os"
)

// ObserveForwardingManagerDeclaration owns a fresh root read-only connection. It
// neither loads nor references a unit, and joins the connection before returning.
// Historical installation records cannot supply this current native fact.
func ObserveForwardingManagerDeclaration(ctx context.Context) (identity string, paths []string, err error) {
	if ctx == nil || os.Geteuid() != 0 {
		return "", nil, ErrInput
	}
	r, err := openManagerReference(ctx, forwardingUnitReply)
	if r != nil {
		defer func() {
			err = errors.Join(err, r.close(), ctx.Err())
			if err != nil {
				identity = ""
				paths = nil
			}
		}()
	}
	if err != nil {
		return "", nil, err
	}
	if err = r.initializeManagerDeclaration(); err != nil {
		return "", nil, err
	}
	identity, err = r.managerIdentity()
	if err != nil {
		return "", nil, err
	}
	return identity, append([]string(nil), r.paths...), nil
}
