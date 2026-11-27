package endpointcli_test

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
)

func TestEnrollmentCommandRefusesBeforeBundleEffects(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, args := range [][]string{
		{"enrollment"}, {"enrollment", "verify", "missing", "not-a-pin"}, {"enrollment", "install", "missing", "not-a-pin"},
	} {
		command := exec.CommandContext(t.Context(), binary, args...)
		var out, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &out, &diagnostic
		err := command.Run()
		wantDiagnostic := ""
		if len(args) > 1 && args[1] == "verify" {
			wantDiagnostic = `{"operation":"enrollment.verify","phase":"command","outcome":"usage-or-output-failure"}` + "\n"
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || out.String() != "{\"outcome\":\"invalid-input\"}\n" || diagnostic.String() != wantDiagnostic {
			t.Fatalf("args %v: error %v, stdout %q, stderr %q", args, err, &out, &diagnostic)
		}
	}
}
