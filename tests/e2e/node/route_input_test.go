package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// An incomplete receiving declaration must not create any Route-related root.
// This is a process-level assertion on every host, including hosts where the
// receiving implementation refuses the platform before opening configuration.
func TestRouteReceiveRefusesBeforeCreatingRoots(t *testing.T) {
	root := t.TempDir()
	spend := filepath.Join(root, "spend")
	hosting := filepath.Join(root, "hosting")
	introduction := filepath.Join(root, "introduction")
	config, err := json.Marshal(map[string]any{
		"spend_root": spend, "hosting_root": hosting, "introduction_root": introduction,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "route.json")
	if err := os.WriteFile(path, config, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "route", "receive", "--config", path)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err == nil {
		t.Fatal("incomplete Route declaration was accepted")
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("Route command did not execute: %v", err)
		}
		switch exit.ExitCode() {
		case 2:
			if output.Len() != 0 || diagnostic.Len() != 0 {
				t.Fatalf("invalid declaration leaked output: %q %q", output.String(), diagnostic.String())
			}
		case 1:
			var report struct {
				Operation string `json:"operation"`
				Phase     string `json:"phase"`
				Outcome   string `json:"outcome"`
			}
			if json.Unmarshal(output.Bytes(), &report) != nil || report.Operation != "route" ||
				report.Phase != "refused" || report.Outcome != "unsupported-platform" || diagnostic.Len() != 0 {
				t.Fatalf("unexpected Route refusal: %q %q", output.String(), diagnostic.String())
			}
		default:
			t.Fatalf("unexpected Route exit %d: %q %q", exit.ExitCode(), output.String(), diagnostic.String())
		}
	}
	for _, path := range []string{spend, hosting, introduction} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refused Route declaration touched %s: %v", path, err)
		}
	}
}
