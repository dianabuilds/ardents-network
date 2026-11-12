package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestJoinPreflightRejectsShapeBeforeInputsOrDiagnostics(t *testing.T) {
	parent := t.TempDir()
	diagnostic := filepath.Join(parent, "diagnostic.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", diagnostic)
	for _, args := range [][]string{{"join"}, {"join", "start-installed", "unaccepted"}, {"join", "inspect-inputs"}, {"join", "inspect-inputs", "one", "extra"}} {
		var output bytes.Buffer
		if runCommand(t.Context(), args, &output) == nil || output.Len() != 0 {
			t.Fatal("unimplemented or malformed JOIN operation accepted", args)
		}
	}
	if _, err := os.Lstat(diagnostic); !os.IsNotExist(err) {
		t.Fatal("JOIN validation opened process diagnostics", err)
	}
}
