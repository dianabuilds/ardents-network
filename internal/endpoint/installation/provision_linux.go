//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
)

func provisionInitial(ctx context.Context, path string) (result ProvisionResult, returnedErr error) {
	if os.Geteuid() != 0 {
		return ProvisionResult{}, errors.New("Endpoint provisioning requires root")
	}
	lease, err := acquireRootLease("/run/ardents-installation.lock")
	if err != nil {
		return ProvisionResult{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, lease.Close())
		if returnedErr != nil {
			result = ProvisionResult{}
		}
	}()
	request, authorization, err := loadProvisionInput(ctx, path)
	if err != nil {
		return ProvisionResult{}, err
	}
	prepared, err := prepareInitialInstallation(ctx, request, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	selected, err := writeGeneration(ctx, request.InstallationRoot, authorization, request, prepared.uid, prepared.gid, prepared.roots)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := installInitialFixedResources(ctx, request.InstallationRoot, selected); err != nil {
		return ProvisionResult{}, err
	}
	if err := selectInitialInstallation(ctx, request.InstallationRoot, selected); err != nil {
		return ProvisionResult{}, err
	}
	checked, err := Check(ctx, request.InstallationRoot)
	if err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{Status: "installed-stopped", GenerationDigest: checked.GenerationDigest, Role: checked.Role}, nil
}
