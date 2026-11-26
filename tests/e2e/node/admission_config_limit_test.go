package node_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The allocation command must read its declaration within the declared bound
// before invoking its domain owner.
func TestAdmissionAllocationConfigBound(t *testing.T) {
	binary := buildNodeCommand(t)
	const limit = (4 << 20) + (64 << 10)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := append([]byte("{}"), bytes.Repeat([]byte(" "), limit-2)...)
	for _, scenario := range []struct {
		name string
		data []byte
		path string
		code int
	}{
		{"exact bound", data, path, 1},
		{"over bound", append(bytes.Clone(data), ' '), path, 2},
		{"directory", nil, dir, 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.data != nil {
				if err := os.WriteFile(scenario.path, scenario.data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.CommandContext(t.Context(), binary, "admission", "allocate", "--config", scenario.path)
			var output, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &output, &diagnostic
			outcome := "failed"
			if scenario.code == 2 {
				outcome = "usage-or-output-failure"
			}
			wantDiagnostic := fmt.Sprintf("{\"operation\":\"admission.allocate\",\"phase\":\"command\",\"outcome\":%q}\n", outcome)
			var exit *exec.ExitError
			if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != scenario.code || output.Len() != 0 || diagnostic.String() != wantDiagnostic {
				t.Fatalf("config boundary: error=%v stdout=%q stderr=%q", err, output.String(), diagnostic.String())
			}
		})
	}
}
