//go:build !linux

package command

import (
	"context"
	"encoding/json"
	"io"
)

func run(_ context.Context, operation string, _ []string, out, diagnostic io.Writer) int {
	report := diagnostic
	if operation == "prepare-trust" {
		report = out
	}
	name := "source"
	if operation != "" {
		name += "." + operation
	}
	if err := json.NewEncoder(report).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{name, "refused", "unsupported-platform"}); err != nil {
		return 2
	}
	return 1
}
