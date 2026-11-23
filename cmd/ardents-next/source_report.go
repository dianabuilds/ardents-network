//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func sourceReport(out io.Writer, phase, outcome string) error {
	return sourceReportContext(context.Background(), out, phase, outcome)
}

func sourceReportContext(ctx context.Context, out io.Writer, phase, outcome string) error {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"source.serve", phase, outcome})
	if err != nil {
		return err
	}
	return artifacts.WriteReport(ctx, out, append(raw, '\n'))
}
