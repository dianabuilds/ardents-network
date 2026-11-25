//go:build linux || darwin

package node_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type issuanceResult struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// TestIssuanceInventoryInputRefusal exercises the actual command and ensures
// malformed declarations are refused before an inventory root is created.
func TestIssuanceInventoryInputRefusal(t *testing.T) {
	binary := buildNodeCommand(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "keys")
	input := map[string]any{"root": root, "inventory_file": filepath.Join(dir, "public"), "binding": issuanceBindingInput{strings.Repeat("ab", 32), strings.Repeat("cd", 32), strings.Repeat("ef", 32), "2030-01-01T00:00:00Z", "2030-01-01T01:00:00Z"}}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"duplicate":  strings.Replace(string(raw), `"network":`, `"network":"`+strings.Repeat("ab", 32)+`","network":`, 1),
		"case":       strings.Replace(string(raw), `"network":`, `"Network":`, 1),
		"missing":    strings.Replace(string(raw), `"signer":"`+strings.Repeat("ef", 32)+`",`, "", 1),
		"unknown":    strings.Replace(string(raw), `"binding":{`, `"binding":{"extra":0,`, 1),
		"uppercase":  strings.Replace(string(raw), strings.Repeat("ab", 32), strings.Repeat("AB", 32), 1),
		"null":       strings.Replace(string(raw), `"binding":{`, `"extra":null,"binding":{`, 1),
		"fractional": strings.Replace(string(raw), "00:00:00Z", "00:00:00.001Z", 1),
		"timezone":   strings.Replace(string(raw), "00:00:00Z", "00:00:00+00:00", 1),
		"trailing":   string(raw) + " {}",
	}
	cases["valid"] = string(raw)
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if name != "valid" && value == string(raw) {
				t.Fatal("mutation did not change declaration")
			}
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), binary, "issuance", "inspect", "--config", path)
			output, err := cmd.Output()
			if err == nil {
				t.Fatal("missing root accepted")
			}
			var report issuanceResult
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("invalid report %q: %v", output, err)
			}
			want := "invalid-input"
			if name == "valid" {
				want = "storage-unavailable"
			}
			if report.Outcome != want {
				t.Fatalf("got outcome %q, want %q", report.Outcome, want)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("inventory root changed: %v", err)
			}
		})
	}
}
