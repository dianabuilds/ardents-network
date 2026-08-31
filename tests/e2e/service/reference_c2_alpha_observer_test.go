//go:build referencec2

package service_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertC2AlphaObserverResolved keeps the second independent Endpoint's
// private-resolution evidence separate from C-2 route assertions.
func assertC2AlphaObserverResolved(t *testing.T, process commandResult) {
	t.Helper()
	if process.err != nil {
		t.Fatalf("C2 alpha observer Endpoint process failed: %v\n%s", process.err, process.output)
	}
	var observed struct {
		Schema, Role, Class string
		Passed              bool
	}
	line := strings.TrimSpace(string(process.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" ||
		observed.Role != "alpha-observer" || observed.Class != "resolved" || !observed.Passed {
		t.Fatalf("C2 alpha observer result = %q / %+v / %v", process.output, observed, err)
	}
}
