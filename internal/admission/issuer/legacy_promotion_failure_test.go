package issuer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyIssuerPromotionStorageFailuresPreserveHistory(t *testing.T) {
	for _, point := range []string{
		"binding-create", "binding-partial-write", "binding-write-uncertain", "binding-directory-sync",
		"stage-create", "stage-partial-write", "stage-write-uncertain",
		"rename-before", "rename-after", "commit-directory-sync", "reopen-barrier",
	} {
		t.Run(point, func(t *testing.T) {
			root := t.TempDir()
			legacy, promoted, window := legacyPromotionBytes()
			journal := filepath.Join(root, "closed-token-ledger")
			binding := filepath.Join(root, ".closed-token-ledger-binding")
			stage := filepath.Join(root, ".closed-token-ledger-next")
			if err := os.WriteFile(journal, legacy, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(journal)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("selected physical transition failed")
			triggered, renamed, directorySyncs := false, false, 0
			storage := nativeIssuerLedgerPersistence()
			storage.writeExclusive = func(path string, raw []byte) error {
				kind := "stage"
				if path == binding {
					kind = "binding"
				} else if path != stage {
					t.Fatalf("unexpected migration write: %s", filepath.Base(path))
				}
				switch point {
				case kind + "-create":
					triggered = true
					return failure
				case kind + "-partial-write":
					// Real truncated bytes model a failed writer; they are never
					// claimed to be a durable complete successor.
					if err := writeIssuerExclusive(path, raw[:len(raw)/2]); err != nil {
						t.Fatal(err)
					}
					triggered = true
					return failure
				case kind + "-write-uncertain":
					if err := writeIssuerExclusive(path, raw); err != nil {
						t.Fatal(err)
					}
					triggered = true
					return failure
				default:
					return writeIssuerExclusive(path, raw)
				}
			}
			storage.rename = func(from, to string) error {
				if from != stage || to != journal {
					t.Fatal("migration replaced a different file")
				}
				raw, err := os.ReadFile(from)
				if err != nil || !bytes.Equal(raw, promoted) {
					t.Fatal("replacement did not contain the independently expected whole history")
				}
				if point == "rename-before" {
					triggered = true
					return failure
				}
				if err := os.Rename(from, to); err != nil {
					return err
				}
				renamed = true
				if point == "rename-after" {
					triggered = true
					return failure
				}
				return nil
			}
			storage.syncDirectory = func(path string) error {
				if path != root {
					t.Fatal("flush used a different root")
				}
				directorySyncs++
				if point == "binding-directory-sync" && directorySyncs == 1 || point == "commit-directory-sync" && directorySyncs == 2 {
					triggered = true
					return failure
				}
				return syncIssuerDirectory(path)
			}
			barrier := func(path string) error {
				if point == "reopen-barrier" {
					triggered = true
					return failure
				}
				return syncIssuerLedger(path)
			}
			owner, err := openClosedTokenIssuerLedgerWithPersistence(root, [32]byte{1}, [32]byte{2}, [32]byte{3}, barrier, storage)
			if !triggered || owner != nil || !errors.Is(err, failure) {
				t.Fatalf("failed transition exposed owner or lost failure: triggered=%t owner=%v err=%v", triggered, owner, err)
			}
			want := legacy
			if renamed {
				want = promoted
			}
			raw, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(raw, want) {
				t.Fatalf("failure lost the complete authoritative history: %v", err)
			}
			if !renamed {
				after, err := os.Stat(journal)
				if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("pre-replacement failure changed original journal identity")
				}
			}
			if point == "binding-partial-write" {
				// A damaged retained identity is repair-required, never an
				// instruction to discard/reconstruct the binding or debit file.
				partial, err := os.ReadFile(binding)
				if err != nil {
					t.Fatal(err)
				}
				for range 2 {
					owner, err := openClosedTokenIssuerLedger(root, [32]byte{1}, [32]byte{2}, [32]byte{3})
					if err == nil || owner != nil {
						t.Fatal("damaged retained binding accepted")
					}
					got, err := os.ReadFile(binding)
					if err != nil || !bytes.Equal(got, partial) {
						t.Fatal("damaged retained binding was repaired implicitly")
					}
					got, err = os.ReadFile(journal)
					if err != nil || !bytes.Equal(got, legacy) {
						t.Fatal("refused reopen changed old debit history")
					}
				}
				return
			}
			checkPromotedLegacyQuota(t, root, window)
			if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("healthy recovery retained an uncommitted stage: %v", err)
			}
		})
	}
}
