package state_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The replaced Source selector must retire before optional diagnostic setup.
// Actual installed Source startup has a separate admitted native qualification.
func TestSourceSelectorRetiresBeforeRuntimeDiagnostics(t *testing.T) {
	node := buildProductCommand(t, "ardents-node")
	parent := t.TempDir()
	socket := filepath.Join(parent, "source.sock")
	missing := filepath.Join(parent, "missing-plan.json")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, "source", "--config", missing)
	command.Env = append(os.Environ(), "ARDENTS_DEBUG_SOCKET="+socket)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err == nil || !bytes.Contains(diagnostic.Bytes(), []byte("old Source profile is retired")) {
		t.Fatalf("retired Source reached diagnostic or plan work: %v; %s", err, diagnostic.String())
	}
	if output.Len() != 0 {
		t.Fatalf("retired Source published readiness: %s", output.String())
	}
	for _, path := range []string{socket, missing} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("retired Source changed path %s: %v", path, err)
		}
	}
}
