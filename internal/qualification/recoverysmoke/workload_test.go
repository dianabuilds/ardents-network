package recoverysmoke

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixedCampaignInitializesBothDirectionalWorkloads(t *testing.T) {
	root := t.TempDir()
	if err := initializeRecoveryWorkload(root, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"client-seed.hex", "publisher-seed.hex"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || len(raw) != 64 {
			t.Fatalf("workload %s length=%d err=%v", name, len(raw), err)
		}
	}
	empty := t.TempDir()
	if err := initializeRecoveryWorkload(empty, false); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(empty); err != nil || len(entries) != 0 {
		t.Fatalf("non-fixed campaign initialized workload: entries=%d err=%v", len(entries), err)
	}
}
