package replacement

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedTemporaryResiduePreservesCommittedProgramVerificationAndRecovery(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "replacement-state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	programDirectory := filepath.Join(workspace, "program")
	if err := os.Mkdir(programDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	programPath := filepath.Join(programDirectory, "ardents")
	program := []byte("committed Endpoint program")
	if err := os.WriteFile(programPath, program, 0o700); err != nil {
		t.Fatal(err)
	}

	committed := Record{
		TargetPath:     programPath,
		Length:         int64(len(program)),
		Digest:         sha256.Sum256(program),
		Platform:       "linux-amd64",
		Architecture:   "amd64",
		Environment:    "closed-alpha",
		Network:        "ardents",
		ReleaseID:      "test-release",
		ReleaseVersion: 1,
	}
	encoded, err := encodeRecord(committed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, markerName), []byte(markerValue), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, currentName), encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, destination := range []string{markerName, currentName, preparedName, journalName, rollbackName} {
		destination := destination
		t.Run(destination, func(t *testing.T) {
			residuePath := filepath.Join(root, "."+destination+"-0123456789abcdef.tmp")
			residue := []byte("interrupted owned write for " + destination)
			if maximum, ok := temporaryEntryMaximum(filepath.Base(residuePath)); !ok || int64(len(residue)) > maximum {
				t.Fatalf("writer temporary name %q is not a valid bounded owned residue", filepath.Base(residuePath))
			}
			if err := os.WriteFile(residuePath, residue, 0o600); err != nil {
				t.Fatal(err)
			}

			running, err := VerifyRunning(root, programPath)
			if err != nil || running.State != StateCurrent || running.Record.Digest != committed.Digest {
				t.Fatalf("VerifyRunning() = %+v, %v", running, err)
			}
			recovered, err := Recover(root, programPath)
			if err != nil || recovered.State != string(StateCurrent) || recovered.Current.Digest != committed.Digest {
				t.Fatalf("Recover() = %+v, %v", recovered, err)
			}
			gotResidue, err := os.ReadFile(residuePath)
			if err != nil || string(gotResidue) != string(residue) {
				t.Fatalf("read-only verification/recovery changed owned residue = %q, %v", gotResidue, err)
			}
			if err := os.Remove(residuePath); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("foreign entry remains rejected and untouched", func(t *testing.T) {
		foreignPath := filepath.Join(root, "foreign")
		if err := os.WriteFile(foreignPath, []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyRunning(root, programPath); err == nil {
			t.Fatal("VerifyRunning() accepted a foreign entry")
		}
		if _, err := Recover(root, programPath); err == nil {
			t.Fatal("Recover() accepted a foreign entry")
		}
		if got, err := os.ReadFile(foreignPath); err != nil || string(got) != "foreign" {
			t.Fatalf("read-only verification/recovery changed foreign entry = %q, %v", got, err)
		}
	})
}

func TestOwnedTemporaryEntryRecognitionRequiresExactWriterFormat(t *testing.T) {
	for _, name := range []string{
		".current-0123456789abcde.tmp",
		".current-0123456789abcdef.tmp.bak",
		".current-0123456789ABCDEF.tmp",
		".current-0123456789abcdef0.tmp",
		".other-0123456789abcdef.tmp",
	} {
		if maximum, ok := temporaryEntryMaximum(name); ok || maximum != 0 {
			t.Fatalf("temporaryEntryMaximum(%q) = %d, %t", name, maximum, ok)
		}
	}
}

func TestReadStoreRejectsForeignOrUnsafeTemporaryResidue(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func(t *testing.T, root string)
	}{
		{
			name: "foreign file",
			create: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, "foreign"), []byte("foreign"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "owned-form directory",
			create: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(root, ".journal-0123456789abcdef.tmp"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "oversized owned-form residue",
			create: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, ".journal-0123456789abcdef.tmp"), make([]byte, maximumText+1), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, markerName), []byte(markerValue), 0o600); err != nil {
				t.Fatal(err)
			}
			test.create(t, root)
			if _, err := openReadStore(root); err == nil {
				t.Fatal("openReadStore() accepted unsafe residue")
			}
		})
	}
}
