//go:build !linux

package main

import (
	"encoding/json"
	"io"
)

// unsupportedPlatformRefusal projects the explicit typed unavailable outcome
// shared by every non-native command stub (ADR-0129 §3). It uses the same
// refusal grammar as the node-role stubs — operation, phase "refused", outcome
// "unsupported-platform" — so an operator can distinguish an unsupported
// platform from a usage error. It returns the refusal exit code 1, or 2 when
// the report itself could not be written. The caller passes the command's own
// report writer; no native qualification or participant effect precedes it.
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
