//go:build !linux

package launch

import (
	"context"
	"encoding/json"
	"io"
)

// RunRepair reports platform refusal before reading any private input.
func RunRepair(_ context.Context, role string, _ []string, out, diagnostic io.Writer) int {
	operation := "node." + role
	if role == "source" {
		operation, out = "source.repair-retained", diagnostic
	}
	if err := json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{operation, "refused", "unsupported-platform"}); err != nil {
		return 2
	}
	return 1
}
