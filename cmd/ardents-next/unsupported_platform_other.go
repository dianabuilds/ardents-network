//go:build !linux

package main

import (
	"encoding/json"
	"io"
)

// unsupportedPlatformRefusal projects the explicit typed unavailable outcome
// for non-native command stubs (ADR-0129 §3). Each caller supplies its operation
// and report writer. The exit code is 1 for a written refusal, or 2 when the
// report cannot be written.
func unsupportedPlatformRefusal(out io.Writer, operation string) int {
	err := json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{operation, "refused", "unsupported-platform"})
	if err != nil {
		return 2
	}
	return 1
}
