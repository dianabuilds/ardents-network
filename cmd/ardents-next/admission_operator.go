package main

import (
	"context"
	"encoding/json"
	"io"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
)

func readAdmissionOperatorConfig(ctx context.Context, args []string) (raw []byte, collector, outcome string) {
	if ctx == nil {
		return nil, "", "invalid-input"
	}
	if ctx.Err() != nil {
		return nil, "", "canceled"
	}
	if len(args) == 2 {
		collector = args[1]
	}
	if telemetry.ValidateCollector(collector) != nil {
		return nil, "", "invalid-input"
	}
	raw, err := readBounded(args[0], 16<<10)
	if err != nil {
		return nil, "", "invalid-input"
	}
	return raw, collector, ""
}

func writeAdmissionReport(out io.Writer, report any, outcome string, success ...string) int {
	if err := json.NewEncoder(out).Encode(report); err != nil {
		return 2
	}
	switch outcome {
	case "invalid-input":
		return 2
	case "canceled":
		return 130
	}
	if slices.Contains(success, outcome) {
		return 0
	}
	return 1
}
