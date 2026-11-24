//go:build !linux

package launch

import (
	"context"
	"encoding/json"
	"io"
)

// RunNative reports unavailable native platform operations.
func RunNative(_ context.Context, _ []string, out io.Writer) int {
	if json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"platform", "refused", "unsupported-platform"}) != nil {
		return 2
	}
	return 1
}
