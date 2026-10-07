//go:build !linux

package installation

import "context"

func checkInstalled(ctx context.Context, _ string) (CheckResult, error) {
	return CheckResult{}, ErrNativeUnavailable
}
