package localread_test

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
)

func TestDocumentReadRejectsInvalidArguments(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, args := range [][]string{{"read"}, {"read", "relative.sock"}, {"read", "/socket", "/output-file"}} {
		command := exec.CommandContext(t.Context(), binary, args...)
		var output, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &output, &diagnostic
		err := command.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("%v: error=%v, stdout=%q, stderr=%q", args, err, &output, &diagnostic)
		}
	}
}
