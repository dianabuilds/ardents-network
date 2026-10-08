//go:build installation_native

package journal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func retainedTransitionFixture(t *testing.T) (string, os.FileInfo, os.FileInfo, string, map[string]RetainedRecord) {
	t.Helper()
	parent, expected, name := transitionParent(t)
	created, err := CreateTransition(t.Context(), parent, expected, name)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("original staged record\n")
	if err := created.Write(t.Context(), Transitions, "0001.json", body); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Lstat(filepath.Join(parent, name))
	if err != nil {
		t.Fatal(err)
	}
	record, err := os.Lstat(filepath.Join(parent, name, "0001.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	return parent, expected, directory, name, map[string]RetainedRecord{"0001.json": {Identity: record, Bytes: body}}
}

func TestInstallationNativeRetainedTransitionResyncAppendAndClose(t *testing.T) {
	parent, expected, directory, name, records := retainedTransitionFixture(t)
	retained, err := OpenTransition(t.Context(), parent, expected, directory, name, records)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = retained.Close() })
	records["0001.json"].Bytes[0] = 'X'
	if !bytes.Equal(retained.Bytes(Transitions, "0001.json"), []byte("original staged record\n")) {
		t.Fatal("caller changed original journal bytes")
	}
	if err := retained.Resync(t.Context(), Transitions, "0001.json"); err != nil {
		t.Fatal(err)
	}
	if err := retained.Write(t.Context(), Transitions, "0002.json", []byte("next physical record\n")); err != nil {
		t.Fatal(err)
	}
	if err := retained.Ensure(t.Context(), Replacements); err != nil {
		t.Fatal(err)
	}
	if err := retained.Write(t.Context(), Replacements, strings.Repeat("b", 64)+".json", []byte("physical replacement provenance\n")); err != nil {
		t.Fatal(err)
	}
	if err := retained.Observe(); err != nil {
		t.Fatal(err)
	}
	fd := retained.directory.file
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := fd.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("retained original descriptor survived close", err)
	}
}

func TestInstallationNativeFailedPhaseRetirementRequiresExactDurableCopy(t *testing.T) {
	for _, mutation := range []string{"none", "missing-copy", "conflict", "phase-inode", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			parent, expected, directory, name, records := retainedTransitionFixture(t)
			retained, err := OpenTransition(t.Context(), parent, expected, directory, name, records)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = retained.Close() })
			failed := []byte("exact original failed phase\n")
			if err := retained.Write(t.Context(), Transitions, "0002.json", failed); err != nil {
				t.Fatal(err)
			}
			if mutation != "missing-copy" {
				copy := bytes.Clone(failed)
				if mutation == "conflict" {
					copy[0] = 'X'
				}
				if err := retained.Write(t.Context(), Transitions, "original-transition-failure.json", copy); err != nil {
					t.Fatal(err)
				}
			}
			phase := filepath.Join(parent, name, "0002.json")
			if mutation == "phase-inode" {
				if err := os.Rename(phase, filepath.Join(parent, "original-phase")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(phase, failed, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mutation == "cancel" {
				cancel()
			}
			err = retained.RetireGenerationFailure(ctx)
			if mutation != "none" {
				if err == nil {
					t.Fatal("unsafe retirement admitted", mutation)
				}
				body, readErr := os.ReadFile(phase)
				if readErr != nil || !bytes.Equal(body, failed) {
					t.Fatal("refusal changed failed phase", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(phase); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed slot not retired", err)
			}
			body, err := os.ReadFile(filepath.Join(parent, name, "original-transition-failure.json"))
			if err != nil || !bytes.Equal(body, failed) {
				t.Fatal("first error changed", err)
			}
			if err := retained.Write(t.Context(), Transitions, "0002.json", []byte("independently admitted completed phase\n")); err != nil {
				t.Fatal(err)
			}
			if err := retained.Observe(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInstallationNativeRetainedTransitionRefusesForeignFirstObservation(t *testing.T) {
	for _, mutation := range []string{"record-inode", "directory-inode", "record-access-reset", "foreign-record", "foreign-group", "changed-bytes", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			parent, expected, directory, name, records := retainedTransitionFixture(t)
			path := filepath.Join(parent, name)
			ctx := t.Context()
			switch mutation {
			case "record-inode":
				if err := os.Rename(filepath.Join(path, "0001.json"), filepath.Join(parent, "retained-original")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "0001.json"), records["0001.json"].Bytes, 0600); err != nil {
					t.Fatal(err)
				}
			case "directory-inode":
				if err := os.Rename(path, filepath.Join(parent, "retained-original-directory")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "0001.json"), records["0001.json"].Bytes, 0600); err != nil {
					t.Fatal(err)
				}
			case "record-access-reset":
				if err := os.Chmod(filepath.Join(path, "0001.json"), 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(filepath.Join(path, "0001.json"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-record":
				if err := os.WriteFile(filepath.Join(path, "foreign.json"), []byte("foreign\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-group":
				if err := os.Mkdir(filepath.Join(path, "replacements"), 0700); err != nil {
					t.Fatal(err)
				}
			case "changed-bytes":
				if err := os.WriteFile(filepath.Join(path, "0001.json"), []byte("foreign staged record\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			retained, err := OpenTransition(ctx, parent, expected, directory, name, records)
			if retained != nil || err == nil {
				if retained != nil {
					_ = retained.Close()
				}
				t.Fatal("foreign or cancelled original admitted", mutation, err)
			}
			if mutation == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation lost", err)
			}
		})
	}
}

func retainedReplacementsFixture(t *testing.T, recorded bool) (string, os.FileInfo, os.FileInfo, string, map[string]RetainedRecord, RetainedReplacements) {
	t.Helper()
	parent, expected, directory, name, records := retainedTransitionFixture(t)
	owner, err := OpenTransition(t.Context(), parent, expected, directory, name, records)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	if err := owner.Ensure(t.Context(), Replacements); err != nil {
		t.Fatal(err)
	}
	groupPath := filepath.Join(parent, name, "replacements")
	identity, err := os.Lstat(groupPath)
	if err != nil {
		t.Fatal(err)
	}
	group := RetainedReplacements{Identity: identity, Records: make(map[string]RetainedRecord)}
	if recorded {
		leaf, body := strings.Repeat("b", 64)+".json", []byte("original replacement record\n")
		if err := owner.Write(t.Context(), Replacements, leaf, body); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(filepath.Join(groupPath, leaf))
		if err != nil {
			t.Fatal(err)
		}
		group.Records[leaf] = RetainedRecord{Identity: info, Bytes: body}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	return parent, expected, directory, name, records, group
}

func TestInstallationNativeRetainedReplacementGroupResyncAndAppend(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "recorded"}[recorded], func(t *testing.T) {
			parent, expected, directory, name, records, group := retainedReplacementsFixture(t, recorded)
			owner, err := OpenTransition(t.Context(), parent, expected, directory, name, records, group)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			if !owner.HasCollection(Replacements) || !os.SameFile(group.Identity, owner.groups[Replacements].identity) {
				t.Fatal("lost original replacement directory")
			}
			if recorded {
				group.Records[strings.Repeat("b", 64)+".json"].Bytes[0] = 'X'
				if !bytes.Equal(owner.Bytes(Replacements, strings.Repeat("b", 64)+".json"), []byte("original replacement record\n")) {
					t.Fatal("caller mutated retained provenance")
				}
			}
			if err := owner.ResyncReplacements(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := owner.Ensure(t.Context(), Replacements); err != nil {
				t.Fatal("attempted new group birth", err)
			}
			if err := owner.Write(t.Context(), Replacements, strings.Repeat("c", 64)+".json", []byte("next physical record\n")); err != nil {
				t.Fatal(err)
			}
			if err := owner.Observe(); err != nil {
				t.Fatal(err)
			}
			fd := owner.groups[Replacements].file
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := fd.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("original group descriptor survived close", err)
			}
		})
	}
}

func TestInstallationNativeRetainedReplacementGroupRefusesSubstitution(t *testing.T) {
	for _, mutation := range []string{"group-inode", "empty-group-inode", "group-access", "record-inode", "record-access-reset", "foreign-record", "unknown-group", "after-open", "cancel-after-open"} {
		t.Run(mutation, func(t *testing.T) {
			parent, expected, directory, name, records, group := retainedReplacementsFixture(t, mutation != "empty-group-inode")
			path := filepath.Join(parent, name)
			groupPath := filepath.Join(path, "replacements")
			leaf := strings.Repeat("b", 64) + ".json"
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var owner *Transition
			var err error
			if mutation == "after-open" || mutation == "cancel-after-open" {
				owner, err = OpenTransition(ctx, parent, expected, directory, name, records, group)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = owner.Close() })
			}
			switch mutation {
			case "group-inode", "empty-group-inode":
				err = os.Rename(groupPath, filepath.Join(parent, "original-group"))
				if err == nil {
					err = os.Mkdir(groupPath, 0700)
				}
				if err == nil && mutation != "empty-group-inode" {
					err = os.WriteFile(filepath.Join(groupPath, leaf), group.Records[leaf].Bytes, 0600)
				}
			case "group-access":
				err = os.Chmod(groupPath, 0750)
			case "record-inode", "after-open":
				err = os.Rename(filepath.Join(groupPath, leaf), filepath.Join(parent, "original-record"))
				if err == nil {
					err = os.WriteFile(filepath.Join(groupPath, leaf), group.Records[leaf].Bytes, 0600)
				}
			case "record-access-reset":
				err = os.Chmod(filepath.Join(groupPath, leaf), 0640)
				if err == nil {
					err = os.Chmod(filepath.Join(groupPath, leaf), 0600)
				}
			case "foreign-record":
				err = os.WriteFile(filepath.Join(groupPath, "foreign.json"), []byte("foreign\n"), 0600)
			case "unknown-group":
				err = os.Mkdir(filepath.Join(path, "creations"), 0700)
			case "cancel-after-open":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			if owner != nil {
				if err := owner.ResyncReplacements(ctx); err == nil {
					t.Fatal("changed or cancelled group resynchronized")
				}
				if err := owner.Ensure(t.Context(), Replacements); err == nil {
					t.Fatal("fresh caller erased first refusal")
				}
			} else {
				owner, err = OpenTransition(ctx, parent, expected, directory, name, records, group)
				if owner != nil {
					_ = owner.Close()
				}
				if err == nil {
					t.Fatal("foreign original accepted")
				}
			}
		})
	}
}

func TestInstallationNativeRetainedTransitionHasSeparateFileBirthCapacity(t *testing.T) {
	parent, expected, name := transitionParent(t)
	created, err := CreateTransition(t.Context(), parent, expected, name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = created.Close() }()
	records := map[string]RetainedRecord{}
	for i := 0; i < 15; i++ {
		n := fmt.Sprintf("generation-file-%064x.json", i)
		body := []byte("detached physical file-birth record\n")
		if err := created.Write(t.Context(), Transitions, n, body); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(filepath.Join(parent, name, n))
		if err != nil {
			t.Fatal(err)
		}
		records[n] = RetainedRecord{Identity: info, Bytes: body}
	}
	for _, n := range []string{"0001.json", "0002.json", "generation-directory.json"} {
		body := []byte("ordinary original transition record\n")
		if err := created.Write(t.Context(), Transitions, n, body); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(filepath.Join(parent, name, n))
		if err != nil {
			t.Fatal(err)
		}
		records[n] = RetainedRecord{Identity: info, Bytes: body}
	}
	dir, err := os.Lstat(filepath.Join(parent, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	retained, err := OpenTransition(t.Context(), parent, expected, dir, name, records)
	if err != nil {
		t.Fatal("fifteen births consumed ordinary transition slots", err)
	}
	if err := retained.Resync(t.Context(), Transitions, "0002.json"); err != nil {
		t.Fatal(err)
	}
	if err := retained.Write(t.Context(), Transitions, fmt.Sprintf("generation-file-%064x.json", 15), []byte("foreign sixteenth")); !errors.Is(err, ErrInput) {
		t.Fatal("ordinary spare capacity admitted sixteenth birth", err)
	}
	if _, err := os.Lstat(filepath.Join(parent, name, fmt.Sprintf("generation-file-%064x.json", 15))); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("capacity refusal created file", err)
	}
	if err := retained.Close(); !errors.Is(err, ErrInput) {
		t.Fatal("close lost capacity refusal", err)
	}
}
