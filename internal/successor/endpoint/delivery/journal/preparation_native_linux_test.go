//go:build installation_native

package journal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativePreparationJournalDurablePhases(t *testing.T) {
	directory := filepath.Join(nativeDirectory(t), "preparation")
	first := preparationRecordFixture()
	j, err := Create(t.Context(), directory, first)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := j.Close(); err != nil {
			t.Error(err)
		}
	}()
	second := first
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	if err := j.Append(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	third := second
	third.Phase = "mutable-roots-prepared"
	if err := j.Append(t.Context(), third); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []struct {
		name   string
		record Record
	}{
		{"0001.json", first}, {"0002.json", second}, {"0003.json", third},
	} {
		body, err := os.ReadFile(filepath.Join(directory, phase.name))
		wanted, encodeErr := Bytes(phase.record)
		info, statErr := os.Lstat(filepath.Join(directory, phase.name))
		if err != nil || encodeErr != nil || statErr != nil || !bytes.Equal(body, wanted) ||
			!ownedRequestFile(info) || info.Mode().Perm() != 0600 {
			t.Fatal("actual phase bytes or custody differ")
		}
	}
	if reopened, err := Create(t.Context(), directory, first); err == nil || reopened != nil {
		t.Fatal("retained directory adopted as a new journal")
	}
}

func TestInstallationNativePreparationJournalRetainsWrittenCancellation(t *testing.T) {
	directory := filepath.Join(nativeDirectory(t), "preparation")
	j, err := Create(t.Context(), directory, preparationRecordFixture())
	if err != nil {
		t.Fatal(err)
	}
	second := preparationRecordFixture()
	second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
	original, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &writeCancellation{Context: original, at: 2, mutate: cancel}
	err = j.Append(ctx, second)
	if !errors.Is(err, context.Canceled) || j.previous != second {
		t.Fatal("durable phase lost after original cancellation")
	}
	if _, err := os.Stat(filepath.Join(directory, "0002.json")); err != nil {
		t.Fatal("written phase disappeared")
	}
	third := second
	third.Phase = "mutable-roots-prepared"
	if !errors.Is(j.Append(t.Context(), third), context.Canceled) {
		t.Fatal("later caller renewed a failed preparation")
	}
	if _, err := os.Stat(filepath.Join(directory, "0003.json")); !os.IsNotExist(err) {
		t.Fatal("failed journal wrote a later phase")
	}
	if !errors.Is(j.Close(), context.Canceled) {
		t.Fatal("close erased original failure")
	}
	if !errors.Is(j.Close(), context.Canceled) || !errors.Is(j.Append(t.Context(), third), context.Canceled) {
		t.Fatal("late operation lost the retained terminal result")
	}
}

func TestInstallationNativePreparationJournalRefusesForeignResidue(t *testing.T) {
	for _, name := range []string{"existing-phase", "directory-substitution", "hardlinked-phase"} {
		t.Run(name, func(t *testing.T) {
			parent := nativeDirectory(t)
			directory := filepath.Join(parent, "preparation")
			j, err := Create(t.Context(), directory, preparationRecordFixture())
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
			if err := j.Append(t.Context(), second); err == nil {
				t.Fatal("foreign residue admitted")
			}
			if err := j.Close(); err == nil {
				t.Fatal("close erased journal failure")
			}
		})
	}
}

func TestInstallationNativePreparationJournalRetainsCompleteIntentChain(t *testing.T) {
	for _, name := range []string{"lost-first", "substituted-first", "foreign-entry"} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(nativeDirectory(t), "preparation")
			j, err := Create(t.Context(), directory, preparationRecordFixture())
			if err != nil {
				t.Fatal(err)
			}
			second := preparationRecordFixture()
			second.Phase, second.UID, second.GID = "creating-mutable-roots", 1000, 1001
			if err := j.Append(t.Context(), second); err != nil {
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
			appendErr := j.Append(t.Context(), third)
			closeErr := j.Close()
			if appendErr == nil || closeErr == nil {
				t.Fatal("incomplete or foreign intent chain admitted")
			}
		})
	}
}

func TestInstallationNativePreparationFailureAfterDurableCompletion(t *testing.T) {
	directory := filepath.Join(nativeDirectory(t), "preparation")
	r := preparationRecordFixture()
	j, err := Create(t.Context(), directory, r)
	if err != nil {
		t.Fatal(err)
	}
	r.Phase, r.UID, r.GID = "creating-mutable-roots", 1000, 1001
	if err := j.Append(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	r.Phase = "mutable-roots-prepared"
	if err := j.Append(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	original, cancel := context.WithCancel(t.Context())
	cancel()
	if err := j.RecordFailure(original, context.Canceled); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "0003.json")); err != nil {
		t.Fatal("actual completed phase erased")
	}
	if _, err := os.Stat(filepath.Join(directory, "failure.json")); err != nil {
		t.Fatal("original failure not retained")
	}
	if !errors.Is(j.Append(t.Context(), preparationRecordFixture()), context.Canceled) || !errors.Is(j.Close(), context.Canceled) {
		t.Fatal("cleanup renewed authority or lost terminal error")
	}
}

func nativeDirectory(t *testing.T) string {
	t.Helper()
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" {
		t.Fatal("invalid environment: installation_native requires root and a trusted temporary parent")
	}
	if _, err := rootDirectoryAncestors(parent); err != nil {
		t.Fatal("invalid temporary parent", err)
	}
	directory, err := os.MkdirTemp(parent, "installation-journal-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

type writeCancellation struct {
	context.Context
	calls  int
	at     int
	mutate func()
}

func (c *writeCancellation) Err() error {
	c.calls++
	if c.calls == c.at {
		c.mutate()
	}
	return c.Context.Err()
}
