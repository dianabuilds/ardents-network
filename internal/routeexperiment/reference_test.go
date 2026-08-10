package routeexperiment

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestReferenceTemporaryDataPathLeavesTheRepository(t *testing.T) {
	t.Parallel()
	directory, err := os.MkdirTemp("", "ar-r013-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(directory+string(filepath.Separator), repository+string(filepath.Separator)) {
		t.Fatalf("reference data directory must be outside the repository: %s", directory)
	}
	if runtime.GOOS != "windows" {
		controlSocket := filepath.Join(directory, "nodes.2147483647", "004ba", "control")
		if len(controlSocket) >= 108 {
			t.Fatalf("Tor control socket path exceeds the Unix socket limit: %s", controlSocket)
		}
	}
}

func TestReferenceLockNamesExactExternalClosure(t *testing.T) {
	t.Parallel()
	inputs, err := readReferenceLock(filepath.Join("..", "..", "carrier-lab", "reference.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Packages) != 13 || len(inputs.Wheels) != 3 || inputs.Archive != "chutney-988fc372cc418fbecc60558fe27e75d07d76b996.tar.gz" {
		t.Fatalf("unexpected reference closure: %+v", inputs)
	}
}

func TestReferenceNamespaceIsCreatedByRootThenDropsToRunner(t *testing.T) {
	t.Parallel()
	command := isolatedReferenceCommand(context.Background(), []string{"HOME=/owned/reference-home", "LOCKED=value"}, []string{"reference-command", "argument"})
	for _, required := range []string{"sudo", "--non-interactive", "unshare", "--net", "setpriv", "--clear-groups", "reference-command"} {
		if !slices.ContainsFunc(command.Args, func(value string) bool { return value == required || strings.Contains(value, required) }) {
			t.Fatalf("isolated reference command is missing %q: %v", required, command.Args)
		}
	}
	if slices.Contains(command.Args, "--user") {
		t.Fatal("reference command still depends on disabled rootless user namespaces")
	}
	if !slices.Contains(command.Args, "HOME=/owned/reference-home") {
		t.Fatal("reference command does not keep HOME inside the owned run")
	}
}
