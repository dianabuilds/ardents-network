//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
)

// Export directories must not be owned state roots or their descendants.
func exportIssuerProfile(ctx context.Context, path string, raw []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		for _, marker := range []string{"identity.pin", "identity.key", "identity.lock", "identity.pending", "profile.pin", "profile.bytes", "profile.lock", "profile.pending", "issuer.pin", "issuer.keys", "issuer.lock", "issuer.pending", "admission.pin", "admission.lock", "admission.journal", "admission.floor", "admission.pending", "results.pin", "results.lock", "results.journal", "results.floor", "results.pending", "budget.pin", "budget.lock", "budget.json", "budget.pending"} {
			if _, e := os.Lstat(filepath.Join(directory, marker)); !errors.Is(e, os.ErrNotExist) {
				return issuance.ErrUnavailable
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	return exportIssuanceInventory(ctx, path, raw)
}
