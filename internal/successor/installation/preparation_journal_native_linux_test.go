//go:build installation_native

package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativePreparationJournalDurablePhases(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "preparation")
	first := preparationRecordFixture()
	j, err := createPreparationJournal(t.Context(), directory, first)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := j.close(); err != nil {
			t.Error(err)
		}
	}()
	second := first
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	if err := j.append(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	third := second
	third.Phase = "mutable-roots-prepared"
	if err := j.append(t.Context(), third); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []struct {
		name   string
		record preparationRecord
	}{
		{"0001.json", first}, {"0002.json", second}, {"0003.json", third},
	} {
		body, err := os.ReadFile(filepath.Join(directory, phase.name))
		wanted, encodeErr := preparationRecordBytes(phase.record)
		info, statErr := os.Lstat(filepath.Join(directory, phase.name))
		if err != nil || encodeErr != nil || statErr != nil || !bytes.Equal(body, wanted) ||
			!ownedRequestFile(info) || info.Mode().Perm() != 0600 {
			t.Fatal("actual phase bytes or custody differ")
		}
	}
	if reopened, err := createPreparationJournal(t.Context(), directory, first); err == nil || reopened != nil {
		t.Fatal("retained directory adopted as a new journal")
	}
}

func TestInstallationNativePreparationJournalRetainsWrittenCancellation(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "preparation")
	j, err := createPreparationJournal(t.Context(), directory, preparationRecordFixture())
	if err != nil {
		t.Fatal(err)
	}
	second := preparationRecordFixture()
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	original, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &nativeRequestReadMutation{Context: original, at: 2, mutate: cancel}
	err = j.append(ctx, second)
	if !errors.Is(err, context.Canceled) || j.previous != second {
		t.Fatal("durable phase lost after original cancellation")
	}
	if _, err := os.Stat(filepath.Join(directory, "0002.json")); err != nil {
		t.Fatal("written phase disappeared")
	}
	third := second
	third.Phase = "mutable-roots-prepared"
	if !errors.Is(j.append(t.Context(), third), context.Canceled) {
		t.Fatal("later caller renewed a failed preparation")
	}
	if _, err := os.Stat(filepath.Join(directory, "0003.json")); !os.IsNotExist(err) {
		t.Fatal("failed journal wrote a later phase")
	}
	if !errors.Is(j.close(), context.Canceled) {
		t.Fatal("close erased original failure")
	}
	if !errors.Is(j.close(), context.Canceled) || !errors.Is(j.append(t.Context(), third), context.Canceled) {
		t.Fatal("late operation lost the retained terminal result")
	}
}

func TestInstallationNativePreparationJournalRefusesForeignResidue(t *testing.T) {
	for _, name := range []string{"existing-phase", "directory-substitution", "hardlinked-phase"} {
		t.Run(name, func(t *testing.T) {
			parent := nativeRequestDirectory(t)
			directory := filepath.Join(parent, "preparation")
			j, err := createPreparationJournal(t.Context(), directory, preparationRecordFixture())
			if err != nil {
				t.Fatal(err)
			}
			second := preparationRecordFixture()
			second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
			var mutationErr error
			switch name {
			case "existing-phase":
				mutationErr = os.WriteFile(filepath.Join(directory, "0002.json"), []byte("foreign\n"), 0600)
			case "directory-substitution":
				mutationErr = os.Rename(directory, directory+".original")
				if mutationErr == nil {
					mutationErr = os.Mkdir(directory, 0700)
				}
			case "hardlinked-phase":
				mutationErr = os.Link(filepath.Join(directory, "0001.json"), filepath.Join(parent, "alias"))
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			if err := j.append(t.Context(), second); err == nil {
				t.Fatal("foreign residue admitted")
			}
			if err := j.close(); err == nil {
				t.Fatal("close erased journal failure")
			}
		})
	}
}

func TestInstallationNativePreparationJournalRetainsCompleteIntentChain(t *testing.T) {
	for _, name := range []string{"lost-first", "substituted-first", "foreign-entry"} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(nativeRequestDirectory(t), "preparation")
			j, err := createPreparationJournal(t.Context(), directory, preparationRecordFixture())
			if err != nil {
				t.Fatal(err)
			}
			second := preparationRecordFixture()
			second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
			if err := j.append(t.Context(), second); err != nil {
				t.Fatal(err)
			}
			first := filepath.Join(directory, "0001.json")
			var mutationErr error
			switch name {
			case "lost-first":
				mutationErr = os.Remove(first)
			case "substituted-first":
				mutationErr = os.Remove(first)
				if mutationErr == nil {
					mutationErr = os.WriteFile(first, []byte("substitute\n"), 0600)
				}
			case "foreign-entry":
				mutationErr = os.WriteFile(filepath.Join(directory, "foreign"), []byte("residue\n"), 0600)
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			third := second
			third.Phase = "mutable-roots-prepared"
			appendErr := j.append(t.Context(), third)
			closeErr := j.close()
			if appendErr == nil || closeErr == nil {
				t.Fatal("incomplete or foreign intent chain admitted")
			}
		})
	}
}
