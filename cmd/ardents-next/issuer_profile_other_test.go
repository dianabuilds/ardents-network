//go:build !linux

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIssuerProfileCommandsRefuseBeforeReadingOnOtherPlatforms(t *testing.T) {
	for _, command := range [][]string{{"node-identity", "import"}, {"issuance", "initialize-profile"}, {"issuance", "inspect-profile"}, {"admission", "prepare-binding"}} {
		args := append(command, "--config", "missing-private-input")
		var out, log bytes.Buffer
		code := run(t.Context(), args, &out, &log)
		var result issuanceResult
		if json.Unmarshal(out.Bytes(), &result) != nil || code != 1 || result.Outcome != "unsupported-platform" {
			t.Fatal(code, &out, &log)
		}
	}
}

func TestIssuerProfileUnsupportedCommandRefusesBeforeConfigRead(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "private-config-that-does-not-exist")
	for _, args := range [][]string{{"node-identity", "import"}, {"issuance", "initialize-profile"}, {"issuance", "inspect-profile"}, {"admission", "prepare-binding"}} {
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), append(args, "--config", missing)...)
		raw, e := cmd.Output()
		if e == nil {
			t.Fatal("unsupported command accepted", args)
		}
		var result issuanceResult
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
