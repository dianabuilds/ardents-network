package endpointcli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTransitionalInstallationCommandsRefuseBeforeEffects(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, verb := range []string{"provision", "upgrade-installed", "recover-initial", "recover-installed", "check"} {
		t.Run(verb, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent")
			command := exec.CommandContext(t.Context(), binary, "installation", verb, path)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.Len() != 0 {
				t.Fatalf("retired command: %v, stdout %q, stderr %q", err, &out, &diagnostic)
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("retired command touched input path: %v", err)
			}
		})
	}
}
