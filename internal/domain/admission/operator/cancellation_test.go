package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOperatorCancellationAndPlatformRefusalPrecedeConfigRead(t *testing.T) {
	for _, scenario := range []struct {
		operation string
		args      string
		run       func(context.Context, []string, io.Writer, io.Writer) int
	}{
		{"admission.initialize", "initialize", RunAdmission},
		{"issuance.initialize", "initialize", RunIssuance},
		{"issuance.inspect-profile", "inspect-profile", RunIssuance},
	} {
		t.Run(scenario.operation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var out, diagnostic bytes.Buffer
			code := scenario.run(ctx, []string{scenario.args, "--config", filepath.Join(t.TempDir(), "missing")}, &out, &diagnostic)
			wantCode, wantOutcome := 130, "canceled"
			if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
				wantCode, wantOutcome = 1, "unsupported-platform"
			}
			var report struct {
				Operation string `json:"operation"`
				Phase     string `json:"phase"`
				Outcome   string `json:"outcome"`
			}
			err := json.Unmarshal(out.Bytes(), &report)
			if code != wantCode || err != nil || report.Operation != scenario.operation || report.Phase != "input" || report.Outcome != wantOutcome || diagnostic.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q decode=%v", code, out.Bytes(), diagnostic.Bytes(), err)
			}
		})
	}
}
