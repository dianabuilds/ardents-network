package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDistributionJournalRetainsOnlyCurrentAndSuccessor(t *testing.T) {
	root, err := openDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.close()
	var current string
	for index := range 80 {
		current = fmt.Sprintf("%064x", index+1)
		if err := root.commitControl(current, []byte(fmt.Sprintf("state-%d", index))); err != nil {
			t.Fatal(err)
		}
		entries, readErr := os.ReadDir(filepath.Join(root.path, "distribution", "generations"))
		if readErr != nil || len(entries) > 2 {
			t.Fatalf("control retention = %d entries, error %v", len(entries), readErr)
		}
	}
	name, raw, err := root.loadControl()
	if err != nil || name != current || !strings.HasPrefix(string(raw), "state-") {
		t.Fatalf("current control = %q %q, error %v", name, raw, err)
	}
}
