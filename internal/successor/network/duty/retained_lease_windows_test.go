package duty

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedParticipationLeaseRefusesReparsePoint(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, rootLockName)); err != nil {
		t.Fatal(err)
	}
	if lease, err := acquireRootLeaseMode(root, false); err == nil {
		_ = lease.release()
		t.Fatal("foreign lease accepted")
	}
}
