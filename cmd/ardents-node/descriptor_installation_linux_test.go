package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptorPreflightRejectsShapeBeforeInputsOrDiagnostics(t *testing.T) {
	diagnostic := filepath.Join(t.TempDir(), "diagnostic.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", diagnostic)
	for _, args := range [][]string{{"descriptor"}, {"descriptor", "start-installed", "unaccepted"}, {"descriptor", "inspect-inputs"}, {"descriptor", "inspect-inputs", "one", "extra"}} {
		var output bytes.Buffer
		if runCommand(t.Context(), args, &output) == nil || output.Len() != 0 {
			t.Fatal("unimplemented or malformed Descriptor operation accepted", args)
		}
	}
	if _, err := os.Lstat(diagnostic); !os.IsNotExist(err) {
		t.Fatal("Descriptor validation opened process diagnostics", err)
	}
}
