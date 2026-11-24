//go:build !linux

package node_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledNodeUnsupportedPlatformRefusesBeforePrivateInput(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	binary := buildNodeCommand(t)
	for _, route := range [][]string{
		{"node", "forwarding", "--config"},
		{"node", "issuer", "--config"},
		{"node", "descriptor", "--config"},
		{"node", "introduction", "--config"},
		{"node", "join", "--config"},
		{"node", "forwarding", "repair-retained"},
		{"node", "descriptor", "repair-retained"},
		{"node", "introduction", "repair-retained"},
		{"node", "join", "repair-retained"},
	} {
		t.Run(strings.Join(route[1:], "/"), func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "unavailable-private-input")
			args := append(route, input)
			command := exec.CommandContext(t.Context(), binary, args...)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			want := `{"operation":"node.` + route[1] + `","phase":"refused","outcome":"unsupported-platform"}` + "\n"
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || out.String() != want {
				t.Fatal("compiled Node read unavailable private input or lost native refusal", err, out.String())
			}
			if diagnostic.Len() != 0 {
				t.Fatal("unsupported Node exposed private input or a native cause")
			}
			if _, err := os.Stat(input); !os.IsNotExist(err) {
				t.Fatal("unsupported Node created private runtime input", err)
			}
		})
	}
}
