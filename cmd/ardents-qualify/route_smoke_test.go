package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRouteSmokeCommandOwnsItsValidatedInterface(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if code := run([]string{"route-smoke"}, &output, &diagnostics); code != 2 {
		t.Fatalf("route-smoke without inputs returned %d, want invalid exit 2", code)
	}
	if !strings.Contains(output.String(), "route smoke evidence root") {
		t.Fatalf("route-smoke did not reach its input contract: %s", output.String())
	}
}
