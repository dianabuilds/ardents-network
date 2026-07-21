package docker

import (
	"fmt"

	"github.com/containerd/errdefs"
)

func dockerSafeError(operation string, err error) error {
	category := "runtime error"
	switch {
	case errdefs.IsNotFound(err):
		category = "not found"
	case errdefs.IsAlreadyExists(err), errdefs.IsConflict(err):
		category = "conflict"
	case errdefs.IsUnauthorized(err), errdefs.IsPermissionDenied(err):
		category = "access denied"
	case errdefs.IsInvalidArgument(err), errdefs.IsFailedPrecondition(err):
		category = "invalid request"
	case errdefs.IsResourceExhausted(err):
		category = "resource exhausted"
	case errdefs.IsDeadlineExceeded(err):
		category = "deadline exceeded"
	case errdefs.IsUnavailable(err):
		category = "unavailable"
	}
	return fmt.Errorf("%s: Docker %s", operation, category)
}
