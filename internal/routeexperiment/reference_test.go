package routeexperiment

import (
	"path/filepath"
	"testing"
)

func TestReferenceLockNamesExactExternalClosure(t *testing.T) {
	t.Parallel()
	inputs, err := readReferenceLock(filepath.Join("..", "..", "carrier-lab", "reference.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Packages) != 13 || len(inputs.Wheels) != 3 || inputs.Archive != "ardents-r013-chutney-988fc37.tar.gz" {
		t.Fatalf("unexpected reference closure: %+v", inputs)
	}
}
