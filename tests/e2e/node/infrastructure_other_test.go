//go:build !linux

package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInfrastructureUnsupportedPlatformRefusesBeforeEffects(t *testing.T) {
	binary := buildNodeCommand(t)
	root := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{
		{"prepare-runtime-descriptor", "source", "release", "1", root, root, root},
		{"prepare-runtime-inventory", "source", root, root},
		{"authenticate-initial", root},
		{"authenticate-candidate", root},
	} {
		command := exec.CommandContext(t.Context(), binary, append([]string{"infrastructure"}, args...)...)
		var out, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &out, &diagnostic
		err := command.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			t.Fatalf("unsupported %s: %v", args[0], err)
		}
		var event map[string]string
		if err := json.Unmarshal(out.Bytes(), &event); err != nil || len(event) != 3 ||
			event["operation"] != "infrastructure" || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" || diagnostic.Len() != 0 {
			t.Fatalf("unsupported %s: stdout=%q stderr=%q", args[0], &out, &diagnostic)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("unsupported path touched input")
		}
	}
}
