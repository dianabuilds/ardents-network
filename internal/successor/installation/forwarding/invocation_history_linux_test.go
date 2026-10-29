package forwarding

import (
	"os"
	"path/filepath"
	"testing"
)

// Namespace-only controls supply no successful Start or installed authority.
func TestForwardingInvocationChainEmptyHistoryRejectsOrphanOriginalBirths(t *testing.T) {
	root := t.TempDir()
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	c := &completedInitial{binding: binding{Request: Request{InstallationRoot: root}}, files: &completedFiles{dirs: map[string]*inputDirectory{root: {file: dir}}}}
	chain := &invocationChain{ctx: t.Context(), owner: c}
	if err := chain.readChain(""); err != nil {
		t.Fatal("genuinely absent child history", err)
	}
	for _, path := range []string{attemptNumber(0).journal(root), attemptNumber(63).journal(root), attemptNumber(0).retirementJournal(root), attemptNumber(63).retirementJournal(root), filepath.Join(root, "forwarding-retirement-unknown")} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := chain.readChain(""); err == nil {
			t.Fatal("orphan original birth supplied empty history", path)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
