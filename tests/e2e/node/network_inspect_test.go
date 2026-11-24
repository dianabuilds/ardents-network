package node_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestNetworkInspectDoesNotInitializeMissingHistory(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent root", true: "empty retained root"}[existing], func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "state")
			if existing {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			plan := state.AuthorityDeclaration{Root: root, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(base, "clock")}
			input, err := json.Marshal(struct {
				Network *state.AuthorityDeclaration `json:"network"`
			}{Network: &plan})
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(base, "config.json")
			if err := os.WriteFile(file, input, 0600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.CommandContext(t.Context(), binary, "network", "inspect", "--config", file).CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("missing history was not refused: %v %s", err, output)
			}
			entries, err := os.ReadDir(root)
			if existing && (err != nil || len(entries) != 0) {
				t.Fatalf("inspect modified empty root: %v %v", entries, err)
			}
			if !existing && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("inspect created missing root: %v", err)
			}
		})
	}
}
