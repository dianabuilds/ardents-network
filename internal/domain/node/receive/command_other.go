//go:build !linux

package receive

import (
	"context"
	"encoding/json"
	"io"
)

// RunRouteReceive reports the unavailable receiving Route host profile.
func RunRouteReceive(_ context.Context, _ []string, out, _ io.Writer) int {
	if err := json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"route", "refused", "unsupported-platform"}); err != nil {
		return 2
	}
	return 1
}
