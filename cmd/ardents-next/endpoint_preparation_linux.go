package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

func runEndpointPlanConstruction(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 4 {
		return 2
	}
	err := endpoint.PreparePlansFromFiles(ctx, args[0], args[1], args[2], args[3])
	if errors.Is(err, endpoint.ErrPreparationPath) {
		return 2
	}
	if err != nil {
		return 1
	}
	if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "plan-construction", "outcome": "constructed"}) != nil {
		return 1
	}
	return 0
}

func runEndpointHistoryPreparation(ctx context.Context, paths []string, diagnostic io.Writer) int {
	if ctx.Err() != nil {
		return 1
	}
	err := endpoint.PrepareHistoriesFromFiles(ctx, paths[0], paths[1], paths[2])
	if errors.Is(err, endpoint.ErrPreparationInput) {
		return 2
	}
	outcome, code := "prepared", 0
	if err != nil {
		outcome, code = "unavailable", 1
	}
	if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "history-preparation", "outcome": outcome}) != nil {
		return 1
	}
	return code
}
