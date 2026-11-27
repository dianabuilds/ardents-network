//go:build !linux && !darwin

package admissioncli_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIssuerProfileUnsupportedCommandRefusesBeforeConfigRead(t *testing.T) {
	binary := buildNodeCommand(t)
	base := t.TempDir()
	missing := filepath.Join(base, "private-config-that-does-not-exist")
	for _, args := range [][]string{
		{"node-identity", "import"}, {"issuance", "initialize-profile"},
		{"issuance", "inspect-profile"}, {"admission", "prepare-binding"},
		{"issuance", "initialize"}, {"issuance", "inspect"},
		{"issuance", "initialize-results"}, {"issuance", "issue"},
		{"admission", "initialize"},
	} {
		cmd := exec.CommandContext(t.Context(), binary, append(args, "--config", missing)...)
		raw, e := cmd.Output()
		var exit *exec.ExitError
		if !errors.As(e, &exit) || exit.ExitCode() != 1 {
			t.Fatal("unsupported command returned unexpected exit", args, e)
		}
		var result struct {
			Phase   string `json:"phase"`
			Outcome string `json:"outcome"`
		}
		if e = json.Unmarshal(raw, &result); e != nil {
			t.Fatal(e)
		}
		if result.Phase != "input" || result.Outcome != "unsupported-platform" {
			t.Fatal("private config parsing preceded unsupported refusal", result)
		}
	}
	if entries, e := os.ReadDir(base); e != nil || len(entries) != 0 {
		t.Fatal("unsupported command created state", e)
	}
}
