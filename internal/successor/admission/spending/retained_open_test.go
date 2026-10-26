package spending

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetainedOpeningRefusesEmptiedOriginalHistory(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	token := bytes.Repeat([]byte{42}, 354)
	original, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Spend(token, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	// Both exact original files are retained elsewhere as a test fault. The
	// same directory inode is now empty; no existence precheck may recreate it.
	for _, name := range []string{".ardents-closed-spend-lock", "closed-token-spends"} {
		if err := os.Rename(filepath.Join(root, name), filepath.Join(evidence, name)); err != nil {
			t.Fatal(err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		owner, err := OpenRetained(root, binding)
		if owner != nil {
			_ = owner.Close()
		}
		if owner != nil || err == nil {
			t.Fatal("emptied retained history initialized")
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("refused retained open created native files", err)
		}
	}
	// Restoring these exact files is only a lease/burn oracle, never recovery.
	for _, name := range []string{".ardents-closed-spend-lock", "closed-token-spends"} {
		if err := os.Rename(filepath.Join(evidence, name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	retained, err := OpenRetained(root, binding)
	if err != nil {
		t.Fatal("refused attempts retained a foreign lease", err)
	}
	defer retained.Close()
	if err := retained.Spend(token, window, window.Add(2*time.Minute)); err == nil {
		t.Fatal("retained history forgot the committed spend")
	}
}

func TestRetainedOpeningPreservesDamagedHistoryAndReturnsOwnLease(t *testing.T) {
	for _, fault := range []string{"missing-lock", "missing-journal", "bad-lock", "bad-header", "crash-tail", "foreign-binding"} {
		t.Run(fault, func(t *testing.T) {
			root := t.TempDir()
			binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
			token := bytes.Repeat([]byte{42}, 354)
			original, err := Open(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := original.Spend(token, window, window.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := original.Close(); err != nil {
				t.Fatal(err)
			}
			lock, journal := filepath.Join(root, ".ardents-closed-spend-lock"), filepath.Join(root, "closed-token-spends")
			valid, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			expectedLock, expectedJournal := []byte{}, bytes.Clone(valid)
			candidate := binding
			switch fault {
			case "missing-lock":
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
			case "missing-journal":
				if err := os.Remove(journal); err != nil {
					t.Fatal(err)
				}
			case "bad-lock":
				expectedLock = []byte("foreign lease bytes")
			case "bad-header":
				expectedJournal[0] ^= 1
			case "crash-tail":
				expectedJournal = append(expectedJournal, 99)
			case "foreign-binding":
				candidate.ReceiverDutyGeneration++
			}
			if fault == "bad-lock" {
				if err := os.WriteFile(lock, expectedLock, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "bad-header" || fault == "crash-tail" {
				if err := os.WriteFile(journal, expectedJournal, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for attempt := 0; attempt < 2; attempt++ {
				owner, err := OpenRetained(root, candidate)
				if owner != nil {
					_ = owner.Close()
				}
				if err == nil || owner != nil {
					t.Fatal("damaged retained history accepted")
				}
				for _, check := range []struct {
					path string
					raw  []byte
					gone bool
				}{{lock, expectedLock, fault == "missing-lock"}, {journal, expectedJournal, fault == "missing-journal"}} {
					raw, err := os.ReadFile(check.path)
					if check.gone {
						if !os.IsNotExist(err) {
							t.Fatal("refusal recreated missing history", err)
						}
					} else if err != nil || !bytes.Equal(raw, check.raw) {
						t.Fatal("refusal repaired or changed original evidence", err)
					}
				}
			}
			// Test-only restoration proves failed acquisition released its own
			// actual native lease without waiting for GC or process exit.
			if err := os.WriteFile(lock, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(journal, valid, 0600); err != nil {
				t.Fatal(err)
			}
			owner, err := OpenRetained(root, binding)
			if err != nil {
				t.Fatal("failed opening kept an original lease", err)
			}
			if err := owner.Spend(token, window, window.Add(2*time.Minute)); err == nil {
				t.Fatal("committed spend disappeared")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRetainedEmptyHeaderHasNoCreationFactAndKeepsExclusiveLease(t *testing.T) {
	root := t.TempDir()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	original, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err := OpenRetained(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.TakeFreshRoot(); err == nil {
		t.Fatal("retained empty history reissued fresh-root fact")
	}
	for range 2 {
		other, err := OpenRetained(root, binding)
		if other != nil {
			_ = other.Close()
		}
		if other != nil || err == nil {
			t.Fatal("live original lease admitted a second owner")
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	joined, err := OpenRetained(root, binding)
	if err != nil {
		t.Fatal("joined retained lease not returned", err)
	}
	if err := joined.Close(); err != nil {
		t.Fatal(err)
	}
}
