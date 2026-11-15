package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIntroductionPreflightRejectsShapeBeforeInputsOrDiagnostics(t *testing.T) {
	diagnostic := filepath.Join(t.TempDir(), "diagnostic.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", diagnostic)
	for _, args := range [][]string{{"introduction"}, {"introduction", "start-installed", "unaccepted"}, {"introduction", "inspect-inputs"}, {"introduction", "inspect-inputs", "one", "extra"}} {
		var output bytes.Buffer
		if runCommand(t.Context(), args, &output) == nil || output.Len() != 0 {
			t.Fatal("unimplemented or malformed Introduction operation accepted", args)
		}
	}
	if _, err := os.Lstat(diagnostic); !os.IsNotExist(err) {
		t.Fatal("Introduction validation opened process diagnostics", err)
	}
}
