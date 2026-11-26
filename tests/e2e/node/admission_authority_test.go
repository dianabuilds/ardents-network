package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func TestAdmissionAuthorityDoesNotInitializeMissingHistory(t *testing.T) {
	binary, _, holderConfig := admissionConsoleFixture(t)
	send, closeConsole := processfixture.OpenAdmissionConsole(t, binary, "holder", holderConfig)
	if result := send(map[string]string{"operation": "status"}); result.Outcome != "completed" {
		t.Fatal("positive holder control failed", result)
	}
	closeConsole()
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
			data, err := json.Marshal(map[string]any{"root": filepath.Join(base, "holder"), "network": plan, "role": admission.AllocationUser})
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(base, "config.json")
			if err := os.WriteFile(config, data, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), binary, "admission", "holder", "--config", config)
			command.Stdin = strings.NewReader("")
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			var exit *exec.ExitError
			if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 || out.Len() != 0 {
				t.Fatalf("missing history accepted: error=%v stdout=%q stderr=%q", err, out.String(), diagnostic.String())
			}
			entries, err := os.ReadDir(root)
			if existing && (err != nil || len(entries) != 0) {
				t.Fatalf("authority opening modified empty root: %v %v", entries, err)
			}
			if !existing && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("authority opening created missing root: %v", err)
			}
		})
	}
}
