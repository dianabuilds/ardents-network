package main

import (
	"context"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func runInitialKeys(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || args[0] != "prepare-initial-keys" {
		return 2
	}
	for _, path := range args[1:] {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) == path {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	receipt, err := provisioning.PrepareInitialNetworkKeys(ctx, args[1], args[2])
	if err != nil {
		return 1
	}
	if writeNetworkPreparationReceipt(ctx, out, receipt) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
