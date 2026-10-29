package forwarding

import (
	"os"
	"path/filepath"
	"testing"
)

// Filesystem name classification only. These invented journal bytes establish
// no native installation, Release, original cut or successful recovery.
func TestForwardingReplacementJournalBirthBlocksBeforePending(t *testing.T) {
	parent := t.TempDir()
	dir, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Error(err)
		}
	})
	root := filepath.Join(parent, "installation")
	check := func(expected ...string) error {
		return checkCompletedJournalInventory(root, &inputDirectory{path: parent, file: dir}, expected, &completedFiles{})
	}
	if err := check(); err != nil {
		t.Fatal(err)
	}
	journal := replacementJournal(root, digest([]byte("uncommitted candidate")))
	// Exact physical cut: an empty original-name sibling exists, but no root,
	// pending marker, completed transcript or generation has been created.
	if err := os.WriteFile(journal, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if check() == nil {
		t.Fatal("unknown empty journal allowed ordinary startup")
	}
	if err := os.WriteFile(journal, []byte("{\"sequence\":0"), 0600); err != nil {
		t.Fatal(err)
	}
	if check() == nil {
		t.Fatal("unknown torn journal allowed ordinary startup")
	}
	if err := check(journal); err != nil {
		t.Fatal("exact expected name classification", err)
	}
	if check(journal, journal) == nil {
		t.Fatal("duplicate expectation accepted")
	}
	if check(filepath.Join(parent, "other", filepath.Base(journal))) == nil {
		t.Fatal("foreign parent expectation accepted")
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if check(journal) == nil {
		t.Fatal("missing retained journal accepted")
	}
	for _, suffix := range []string{"unknown", digest([]byte("other")) + "-journal.original-cut-pending"} {
		path := root + ".forwarding-replacement-" + suffix
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if check() == nil {
			t.Fatal("foreign replacement sibling accepted", suffix)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := check(); err != nil {
		t.Fatal("removed test-only controls left phantom names", err)
	}
	for _, suffix := range []string{".native-before", ".native-after"} {
		path := journal + suffix
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if check() == nil {
			t.Fatal("empty unknown context birth admitted without complete native lineage", suffix)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestForwardingCutSidecarBlocksEveryOrdinaryLineageReader(t *testing.T) {
	for _, operation := range []string{"initial", "replacement", "start", "retirement"} {
		for _, suffix := range []string{".original-cut-journal", ".original-cut-result", ".original-cut-pending", ".recovery-completion", ".recovery-pending"} {
			t.Run(operation+suffix, func(t *testing.T) {
				parent := t.TempDir()
				dir, err := os.Open(parent)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := dir.Close(); err != nil {
						t.Error(err)
					}
				})
				root := filepath.Join(parent, "installation")
				journal := root + ".forwarding-initial-journal"
				switch operation {
				case "replacement":
					journal = replacementJournal(root, digest([]byte("uncommitted candidate")))
				case "start":
					journal = attemptNumber(0).journal(root)
				case "retirement":
					journal = attemptNumber(0).retirementJournal(root)
				}
				path := journal + suffix
				if err := os.WriteFile(path, []byte("complete-looking copied sidecar grants no admission\n"), 0600); err != nil {
					t.Fatal(err)
				}
				parentDir := &inputDirectory{path: parent, file: dir}
				if checkCompletedJournalInventory(root, parentDir, nil, &completedFiles{}) == nil || checkCompletedJournalInventory(root, parentDir, []string{path}, &completedFiles{}) == nil {
					t.Fatal("cut sidecar bypassed full recovered validation without an original blocker", path)
				}
			})
		}
	}
}
