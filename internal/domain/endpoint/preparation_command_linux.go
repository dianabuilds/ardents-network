package endpoint

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func runEndpointPlanConstruction(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 4 {
		return 2
	}
	err := PreparePlansFromFiles(ctx, args[0], args[1], args[2], args[3])
	if errors.Is(err, ErrPreparationPath) {
		return 2
	}
	if err != nil {
		_ = runtimeevent.WriteCommand(diagnostic, "endpoint.construct-plan", "plan-construction", "unavailable")
		return 1
	}
	if runtimeevent.WriteCommand(diagnostic, "endpoint.construct-plan", "plan-construction", "constructed") != nil {
		return 1
	}
	return 0
}

func runEndpointHistoryPreparation(ctx context.Context, paths []string, diagnostic io.Writer) int {
	if ctx.Err() != nil {
		return 1
	}
	err := PrepareHistoriesFromFiles(ctx, paths[0], paths[1], paths[2])
	if errors.Is(err, ErrPreparationInput) {
		return 2
	}
	outcome, code := "prepared", 0
	if err != nil {
		outcome, code = "unavailable", 1
	}
	if runtimeevent.WriteCommand(diagnostic, "endpoint.prepare-history", "history-preparation", outcome) != nil {
		return 1
	}
	return code
}
