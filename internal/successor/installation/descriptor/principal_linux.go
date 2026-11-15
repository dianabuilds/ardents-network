package descriptor

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/nodeprincipal"
)

// observeNodePrincipal keeps the role's error vocabulary while the observer
// independently owns and closes each original host-file and NSS acquisition.
func observeNodePrincipal(ctx context.Context, uid, gid uint32) error {
	return principalObservationError(nodeprincipal.Observe(ctx, uid, gid))
}

func principalObservationError(err error) error {
	switch err {
	case nil:
		return nil
	case nodeprincipal.ErrInput:
		return ErrInput
	case nodeprincipal.ErrBinding:
		return ErrBinding
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		mapped := make([]error, len(causes))
		for i, cause := range causes {
			mapped[i] = principalObservationError(cause)
		}
		return errors.Join(mapped...)
	}
	return err
}
