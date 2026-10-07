//go:build !linux

package installation

import (
	"context"
	"time"
)

type recoveryNative struct{}

func openInitialRecovery(context.Context, string, time.Time) (*Recovery, error) {
	return nil, ErrNativeUnavailable
}

func completeInitialRecovery(context.Context, *recoveryOperation, Authorization) (ProvisionResult, error) {
	return ProvisionResult{}, ErrNativeUnavailable
}

func closeInitialRecovery(*recoveryNative) error { return nil }
