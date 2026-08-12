package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlJournalRetainsOnlyCurrentAndSuccessor(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var current string
	for index := range 80 {
		current = fmt.Sprintf("%064x", index+1)
		if err := root.CommitControl(current, []byte(fmt.Sprintf("state-%d", index))); err != nil {
			t.Fatal(err)
		}
		entries, readErr := os.ReadDir(filepath.Join(root.path, "distribution", "generations"))
		if readErr != nil || len(entries) > 2 {
			t.Fatalf("control retention = %d entries, error %v", len(entries), readErr)
		}
	}
	name, raw, err := root.LoadControl()
	if err != nil || name != current || !strings.HasPrefix(string(raw), "state-") {
		t.Fatalf("current control = %q %q, error %v", name, raw, err)
	}
}
