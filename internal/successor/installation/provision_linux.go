package installation

import (
	"context"
	"errors"
)

func readProvisionRequest(ctx context.Context, filename string) (Request, error) {
	if ctx == nil {
		return Request{}, ErrInput
	}
	if err := observeInstallationPlatform(ctx); err != nil {
		return Request{}, err
	}
	request, err := ReadOwnedRequest(ctx, filename, true)
	if err != nil {
		return Request{}, err
	}
	if err := preflightInitialEffects(ctx, request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func provisionInitial(ctx context.Context, request Request, authorization Authorization) (result ProvisionResult, returnedErr error) {
	if ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	if err := observeInstallationPlatform(ctx); err != nil {
		return ProvisionResult{}, err
	}
	owned, err := prepareInitialNative(ctx, request, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, ctx.Err())
		if returnedErr != nil {
			owned.terminal = returnedErr
			owned.stage.retainFailure(ctx, returnedErr)
			owned.terminal = errors.Join(owned.terminal, owned.journal.RecordFailure(ctx, returnedErr))
		}
		returnedErr = errors.Join(returnedErr, owned.close(), ctx.Err())
		if returnedErr != nil {
			result = ProvisionResult{}
		}
	}()
	if err := owned.observe(ctx, request); err != nil {
		return ProvisionResult{}, err
	}
	if err := owned.observeStoppedInstallation(ctx, request); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{Status: "installed-stopped", GenerationDigest: owned.stage.selected.GenerationDigest}, nil
}
