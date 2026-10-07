//go:build !linux

package installation

import "context"

func readProvisionRequest(ctx context.Context, _ string) (Request, error) {
	if ctx == nil {
		return Request{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	return Request{}, ErrNativeUnavailable
}

func provisionInitial(ctx context.Context, _ Request, _ Authorization) (ProvisionResult, error) {
	if ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{}, ErrNativeUnavailable
}
