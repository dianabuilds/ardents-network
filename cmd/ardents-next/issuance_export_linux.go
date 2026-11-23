//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// Issuance exports never write into a domain's retained or pending roots.
func checkIssuanceExportPath(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return issuance.ErrInvalid
	}
	markers := [...]string{
		"identity.pin", "identity.key", "identity.lock", "identity.pending",
		"profile.pin", "profile.bytes", "profile.lock", "profile.pending",
		"issuer.pin", "issuer.keys", "issuer.lock", "issuer.pending",
		"admission.pin", "admission.lock", "admission.journal", "admission.floor", "admission.pending",
		"results.pin", "results.lock", "results.journal", "results.floor", "results.pending",
		"budget.pin", "budget.lock", "budget.json", "budget.pending",
	}
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		for _, marker := range markers {
			if _, err := os.Lstat(filepath.Join(directory, marker)); !errors.Is(err, os.ErrNotExist) {
				return issuance.ErrUnavailable
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	return ctx.Err()
}

func checkIssuanceOutput(ctx context.Context, path string) error {
	if err := checkIssuanceExportPath(ctx, path); err != nil {
		return err
	}
	err := artifacts.CheckPrivateFileDestination(ctx, path)
	if errors.Is(err, artifacts.ErrInput) {
		return issuance.ErrUnavailable
	}
	return err
}

func exportIssuanceOutput(ctx context.Context, path string, raw []byte) error {
	if err := checkIssuanceExportPath(ctx, path); err != nil {
		return err
	}
	err := artifacts.PublishPrivateFile(ctx, path, raw)
	if errors.Is(err, artifacts.ErrOutputUncertain) {
		return errors.Join(issuance.ErrUncertain, err)
	}
	if errors.Is(err, artifacts.ErrInput) {
		return issuance.ErrUnavailable
	}
	return err
}
