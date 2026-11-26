//go:build !linux

package endpoint

import (
	"context"
	"encoding/json"
	"io"
)

func RunInstalledCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 || !(args[0] == "start-installed" && len(args) == 2 || args[0] == "prepare-history" && len(args) == 4) {
		return 2
	}
	err := json.NewEncoder(diagnostic).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"endpoint", "refused", "unsupported-platform"})
	if err != nil {
		return 2
	}
	return 1
}
