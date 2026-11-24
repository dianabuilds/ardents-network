//go:build !linux

package infrastructure

import (
	"context"
	"encoding/json"
	"io"
)

// RunCommand reports the unavailable native infrastructure profile.
func RunCommand(_ context.Context, _ []string, out io.Writer) int {
	if json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"infrastructure", "refused", "unsupported-platform"}) != nil {
		return 2
	}
	return 1
}
