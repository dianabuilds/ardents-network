//go:build linux

package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// loadProvisionInput is the initial root-provisioning admission boundary.
// Only Release floors may change here; no account, participant or unit is made.
func loadProvisionInput(ctx context.Context, path string) (Request, Authorization, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return Request{}, Authorization{}, errors.New("endpoint provisioning requires root and context")
	}
	if err := observePlatform(ctx); err != nil {
		return Request{}, Authorization{}, err
	}
	body, err := readInstalledFile(path, 64<<10)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	request, err := decodeRequest(body)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	if request.ManifestSHA256 == "" {
		return Request{}, Authorization{}, errors.New("initial Endpoint provisioning requires the independent manifest pin")
	}
	if err := checkRootAncestors(request.BundleRoot, false); err != nil {
		return Request{}, Authorization{}, err
	}
	if err := checkRootAncestors(request.ReleaseFloorRoot, true); err != nil {
		return Request{}, Authorization{}, err
	}
	if _, err := os.Lstat(request.InstallationRoot); !os.IsNotExist(err) {
		return Request{}, Authorization{}, errors.New("initial Endpoint installation path already exists or is unavailable")
	}
	if err := checkRootAncestors(filepath.Dir(request.InstallationRoot), false); err != nil {
		return Request{}, Authorization{}, err
	}
	if err := preflightInitialInstallation(ctx, request); err != nil {
		return Request{}, Authorization{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	enrollmentRequest, err := enrollment.RequestFromManifestPin(request.BundleRoot, filepath.Join(request.BundleRoot, "ardents-linux-amd64"), request.ManifestSHA256, at)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	enrolled, err := enrollment.VerifyHeadless(enrollmentRequest)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	verifier, err := release.Open(request.ReleaseFloorRoot)
	if err != nil {
		return Request{}, Authorization{}, err
	}
	authorization, authErr := Authenticate(ctx, verifier, enrolled)
	if err := errors.Join(authErr, verifier.Close()); err != nil {
		return Request{}, Authorization{}, err
	}
	directory := filepath.Join(request.InstallationRoot, "generations", digestHex(authorization.descriptor))
	if _, err := renderEndpointUnit(authorization.resources["ardents-endpoint.service"], request, directory); err != nil {
		return Request{}, Authorization{}, err
	}
	return request, authorization, nil
}

// missingLeaf permits exactly the new floor directory, never missing parents.
func checkRootAncestors(path string, missingLeaf bool) error {
	if !canonicalPath(path) {
		return errors.New("provisioning root path is invalid")
	}
	for first := true; ; first = false {
		info, err := os.Lstat(path)
		if first && missingLeaf && os.IsNotExist(err) {
			path = filepath.Dir(path)
			continue
		}
		if err != nil {
			return err
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !ok || identity.Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.New("provisioning root ancestor is untrusted")
		}
		if filepath.Dir(path) == path {
			return nil
		}
		path = filepath.Dir(path)
	}
}
