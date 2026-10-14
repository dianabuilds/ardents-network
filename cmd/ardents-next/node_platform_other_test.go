//go:build !linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompiledNodeUnsupportedPlatformRefusesBeforePrivateInput(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction"} {
		t.Run(role, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "unavailable-private-input")
			command := exec.CommandContext(t.Context(), compiledCommand(t), "node", role, "--config", input)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			var event map[string]string
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || json.Unmarshal(out.Bytes(), &event) != nil || len(event) != 3 ||
				event["operation"] != "node."+role || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" {
				t.Fatal("compiled Node read unavailable private input or lost native refusal", err, out.String())
			}
			if diagnostic.Len() != 0 || bytes.Contains(out.Bytes(), []byte(input)) {
				t.Fatal("unsupported Node exposed private input or a native cause")
			}
			if _, err := os.Stat(input); !os.IsNotExist(err) {
				t.Fatal("unsupported Node created private runtime input", err)
			}
		})
	}
}
