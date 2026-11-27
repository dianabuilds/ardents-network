package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstalledEndpointCancelledBeforeEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent-installation")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output, diagnostic bytes.Buffer
	if code := Run(ctx, []string{"start-installed", root}, &diagnostic); code != 1 {
		t.Fatal("cancelled installed opening accepted", code)
	}
	assertInstalledEndpointCancelledRefusal(t, &output, &diagnostic, "endpoint.start-installed")
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refusal changed installation", err)
	}
	output.Reset()
	diagnostic.Reset()
	if code := Run(ctx, []string{"prepare-history", root, root, root}, &diagnostic); code != 1 {
		t.Fatal("canceled preparation read inputs or reported success", code)
	}
	assertInstalledEndpointCancelledRefusal(t, &output, &diagnostic, "endpoint.prepare-history")
	for _, args := range [][]string{{"endpoint"}, {"endpoint", "start-installed"}, {"endpoint", "foreign", root}, {"endpoint", "prepare-history", root}} {
		output.Reset()
		diagnostic.Reset()
		if code := Run(t.Context(), args[1:], &diagnostic); code != 2 {
			t.Fatal("malformed installed selector admitted", args, code)
		}
	}
}

// A cancelled installed opening refuses before any participant effect on every
// platform. Linux projects no report for the cancelled before-effect refusal;
// the non-native stub projects the explicit typed unsupported-platform refusal
// (ADR-0129 §3) on its diagnostic writer. Neither reports participant readiness.
func assertInstalledEndpointCancelledRefusal(t *testing.T, output, diagnostic *bytes.Buffer, operation string) {
	t.Helper()
	if runtime.GOOS == "linux" {
		if output.Len() != 0 || diagnostic.Len() != 0 {
			t.Fatal("cancelled installed opening reported participant readiness")
		}
		return
	}
	if output.Len() != 0 {
		t.Fatal("unsupported installed opening leaked stdout", output.String())
	}
	var event map[string]string
	if json.Unmarshal(diagnostic.Bytes(), &event) != nil || len(event) != 3 ||
		event["operation"] != operation || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" {
		t.Fatal("unsupported installed opening lost the typed refusal", diagnostic.String())
	}
}
