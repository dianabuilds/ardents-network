//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"
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
	return writeSourceReportContext(ctx, out, append(raw, '\n'))
}

func sourceReportCode(out io.Writer, phase, outcome string, code int) int {
	if sourceReport(out, phase, outcome) != nil {
		return 2
	}
	return code
}
