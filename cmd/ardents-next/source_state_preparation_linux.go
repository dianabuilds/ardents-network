package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSourceStatePreparation(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 2 {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := state.PrepareSourceStateFiles(ctx, args[0], args[1]); err != nil {
		owner := "state"
		if errors.Is(err, state.ErrSourcePreparationPrincipal) {
			owner = "input"
		}
		return sourceStartupFailure(diagnostic, owner, err, nil)
	}
	const receipt = "{\"operation\":\"source.prepare-state\",\"outcome\":\"state-prepared\"}\n"
	if artifacts.WriteReport(ctx, out, []byte(receipt)) != nil {
		return 1
	}
	return 0
}
