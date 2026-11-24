//go:build !linux

package command

import (
	"context"
	"encoding/json"
	"io"
)

// Run reports that the native Source operation is unavailable before reading input.
func Run(_ context.Context, args []string, out, diagnostic io.Writer) int {
	operation := "source"
	report := diagnostic
	if len(args) > 0 {
		switch args[0] {
		case "prepare-trust":
			report = out
			fallthrough
		case "prepare-state", "inspect-latest":
			operation += "." + args[0]
		}
	}
	if err := json.NewEncoder(report).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{operation, "refused", "unsupported-platform"}); err != nil {
		return 2
	}
	return 1
}
