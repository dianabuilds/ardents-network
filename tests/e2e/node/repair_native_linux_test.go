//go:build linux && node_process_native

package node

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNativeNodeRepairRefusalKeepsBoundedReport(t *testing.T) {
	binary := PinnedCommand(t)
	root := t.TempDir()
	plan := filepath.Join(root, "missing-repair-plan")
	for _, role := range []string{"forwarding", "descriptor", "introduction", "join"} {
		t.Run(role, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			command := exec.CommandContext(t.Context(), binary, "node", role, "repair-retained", plan)
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			const refusal = `{"operation":"source.serve","phase":"repair","outcome":"failed","first_owner":"state"}` + "\n"
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || out.Len() != 0 || diagnostic.String() != refusal {
				t.Fatalf("repair did not keep its bounded refusal: %v, stdout %q, stderr %q", err, &out, &diagnostic)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("repair refusal changed its input directory", entries, err)
			}
		})
	}
}
