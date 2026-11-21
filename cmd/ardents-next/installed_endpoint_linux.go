//go:build linux

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"io"
)

func runInstalledEndpoint(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	if args[0] == "construct-plan" {
		if len(args) != 5 {
			return 2
		}
		return runEndpointPlanConstruction(ctx, args[1:], diagnostic)
	}
	if args[0] == "prepare-history" {
		if len(args) != 4 {
			return 2
		}
		return runEndpointHistoryPreparation(ctx, args[1:], diagnostic)
	}
	if len(args) != 2 || args[0] != "start-installed" {
		return 2
	}
	if ctx.Err() != nil {
		return 1
	}
	phase, err := endpoint.Run(ctx, args[1], func(progress endpoint.Progress) error {
		return json.NewEncoder(diagnostic).Encode(map[string]string{"phase": progress.Phase, "role": progress.Role, "request_digest": hex.EncodeToString(progress.Digest[:])})
	})
	code := installedEndpointFailure(ctx, err)
	if code != 0 {
		_ = json.NewEncoder(diagnostic).Encode(map[string]string{"phase": phase, "outcome": "unavailable"})
	}
	return code
}

// A requested shutdown is not a failed system-manager invocation once original
// cleanup joins. Owners still retain their cancellation results; only the exit
// projection treats an exclusively canceled result as expected. Any additional
// failure, including one joined with cancellation, keeps the failed exit.
func installedEndpointFailure(ctx context.Context, err error) int {
	if err == nil {
		return 0
	}
	if ctx.Err() == context.Canceled && installedEndpointCanceledOnly(err) {
		return 0
	}
	return 1
}

func installedEndpointCanceledOnly(err error) bool {
	if err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !installedEndpointCanceledOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return installedEndpointCanceledOnly(wrapped.Unwrap())
	}
	return false
}
