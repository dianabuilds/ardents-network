package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	generationauthorization "github.com/dianabuilds/ardents-network/internal/release/endpoint"
)

// verify-initial requires independently pinned bytes and keeps Release history
// separate. Its output is verification, never installation or live readiness.
func runRelease(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 5 || args[0] != "verify-initial" {
		return enrollmentReport(out, "invalid-input", 2)
	}
	ref, err := time.Parse(time.RFC3339, args[4])
	if err != nil {
		return enrollmentReport(out, "invalid-input", 2)
	}
	result := generationauthorization.VerifyInitial(ctx, args[1], args[2], args[3], ref)
	code := 1
	if result.Authorized {
		code = 0
	} else if result.Canceled {
		code = 130
	}
	if !result.Detailed {
		return enrollmentReport(out, result.Outcome, code)
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome     string `json:"outcome"`
		RootVersion int64  `json:"root_version"`
	}{result.Outcome, result.RootVersion}) != nil {
		return 2
	}
	return code
}
