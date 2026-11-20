package state_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestRetiredHeadlessReaderRefusesBeforeRootEffects(t *testing.T) {
	binary := buildCommand(t, "ardents")
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	if err := os.Mkdir(stateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(stateRoot, "retained.bin")
	retained := []byte("opaque retained State bytes")
	if err := os.WriteFile(statePath, retained, 0600); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "reader.json")
	plan, err := json.Marshal(map[string]string{
		"schema":             "ardents-headless-runtime-v2",
		"role":               "reader",
		"network_state_root": stateRoot,
		"entry_state_root":   filepath.Join(root, "entry"),
		"application_socket": filepath.Join(root, "reader.sock"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}

	const refusal = "endpoint headless is retired; use ardents-next endpoint start-installed under the platform unit\n"
	for _, arguments := range [][]string{
		{"endpoint", "headless", planPath},
		{"endpoint", "headless", filepath.Join(root, "absent.json")},
		{"endpoint", "headless"},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		output, commandErr := exec.CommandContext(ctx, binary, arguments...).CombinedOutput()
		cancel()
		if commandErr == nil || string(output) != refusal {
			t.Fatalf("retired headless command: err=%v output=%q", commandErr, output)
		}
	}

	for _, expected := range []struct {
		path string
		body []byte
	}{
		{planPath, plan},
		{statePath, retained},
	} {
		actual, err := os.ReadFile(expected.path)
		if err != nil || !bytes.Equal(actual, expected.body) {
			t.Fatalf("retired command changed %s: %v", expected.path, err)
		}
	}
	for path, expected := range map[string][]string{root: {"reader.json", "state"}, stateRoot: {"retained.bin"}} {
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if !slices.Equal(names, expected) {
			t.Fatalf("retired command changed directory %s: %v", path, names)
		}
	}
}
