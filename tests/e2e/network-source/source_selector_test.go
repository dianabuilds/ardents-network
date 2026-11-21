package state_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func buildProductCommand(t *testing.T, name string) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_E2E_PRODUCT_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))); prebuilt != "" {
		info, err := os.Lstat(prebuilt)
		if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("prebuilt product command %s is not a regular executable: %v", name, err)
		}
		return prebuilt
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	path := filepath.Join(t.TempDir(), name+suffix)
	command := exec.Command("go", "build", "-o", path, "./cmd/"+name)
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}
