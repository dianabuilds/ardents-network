//go:build linux || darwin

package admissioncli_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileCommandStrictInput(t *testing.T) {
	binary := buildNodeCommand(t)
	dir := t.TempDir()
	root, _ := json.Marshal(filepath.Join(dir, "identity"))
	key, _ := json.Marshal(filepath.Join(dir, "missing-key"))
	digest := strings.Repeat("01", 32)
	binding := `{"network":"` + digest + `","node":"` + digest + `","signer":"` + digest + `"}`
	valid := `{"root":` + string(root) + `,"binding":` + binding + `,"key_file":` + string(key) + `}`
	for _, scenario := range []struct {
		name string
		raw  string
		code int
	}{
		{"valid declaration reaches import", valid, 1},
		{"duplicate root", strings.Replace(valid, `"root":`, `"root":`+string(root)+`,"root":`, 1), 2},
		{"wrong case", strings.Replace(valid, `"root":`, `"Root":`, 1), 2},
		{"null root", strings.Replace(valid, `"root":`+string(root), `"root":null`, 1), 2},
		{"unknown field", strings.TrimSuffix(valid, "}") + `,"extra":1}`, 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config")
			if err := os.WriteFile(path, []byte(scenario.raw), 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), binary, "node-identity", "import", "--config", path)
			output, err := command.Output()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != scenario.code {
				t.Fatalf("exit: %v, output %q", err, output)
			}
			var report struct {
				Phase   string `json:"phase"`
				Outcome string `json:"outcome"`
			}
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatal(err)
			}
			if scenario.code == 2 && (report.Phase != "input" || report.Outcome != "invalid-input") {
				t.Fatalf("malformed declaration: %+v", report)
			}
			if scenario.code == 1 && report.Phase != "import" {
				t.Fatalf("valid declaration did not reach import: %+v", report)
			}
			if _, err := os.Stat(filepath.Join(dir, "identity")); !os.IsNotExist(err) {
				t.Fatalf("import unexpectedly created identity: %v", err)
			}
		})
	}
}
