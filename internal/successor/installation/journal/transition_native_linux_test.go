//go:build installation_native

package journal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func transitionParent(t *testing.T) (string, os.FileInfo, string) {
	t.Helper()
	parent := filepath.Join(nativeDirectory(t), "journals")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	return parent, info, strings.Repeat("a", 64)
}

func transitionFixture(t *testing.T) (*Transition, string) {
	t.Helper()
	parent, info, name := transitionParent(t)
	j, err := CreateTransition(t.Context(), parent, info, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j, filepath.Join(parent, name)
}

func TestInstallationNativeTransitionOwnsRecordsGroupsAndPhysicalClose(t *testing.T) {
	j, path := transitionFixture(t)
	for _, group := range []Collection{Creations, DirectoryCreations, Replacements} {
		if err := j.Ensure(t.Context(), group); err != nil {
			t.Fatal(err)
		}
		name := strings.Repeat("b", 64) + ".json"
		body := []byte("original provenance\n")
		if err := j.Write(t.Context(), group, name, body); err != nil {
			t.Fatal(err)
		}
		body[0] = 'X'
		copy := j.Bytes(group, name)
		copy[0] = 'Y'
		actual, err := os.ReadFile(filepath.Join(path, collectionName(group), name))
		info, statErr := os.Lstat(filepath.Join(path, collectionName(group), name))
		if err != nil || statErr != nil || !bytes.Equal(actual, []byte("original provenance\n")) || !ownedRequestFile(info) || info.Mode() != 0600 {
			t.Fatal("record lost exact bytes or original private custody", err, statErr)
		}
		if err := j.Resync(t.Context(), group, name); err != nil {
			t.Fatal(err)
		}
		after, err := os.Lstat(filepath.Join(path, collectionName(group), name))
		if err != nil || !os.SameFile(info, after) {
			t.Fatal("resync replaced original record", err)
		}
	}
	if err := j.Observe(); err != nil {
		t.Fatal(err)
	}
	parent := j.parent
	roots := []*os.Root{j.directory.root}
	files := []*os.File{j.directory.file}
	for _, group := range j.order {
		roots = append(roots, group.root)
		files = append(files, group.file)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	for _, root := range append(roots, parent) {
		if _, err := root.Stat("."); err == nil {
			t.Fatal("original directory descriptor survived close")
		}
	}
	for _, file := range files {
		if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("original file descriptor survived close", err)
		}
	}
	if j.Bytes(Replacements, strings.Repeat("b", 64)+".json") != nil || j.HasCollection(Replacements) {
		t.Fatal("closed custody exposed live record facts")
	}
	if err := j.Close(); err != nil {
		t.Fatal("physical close was not idempotent", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("close deleted durable provenance", err)
	}
}

// The cancellation is observed only after the real filesystem effect. It grants
// no successful Installation phase, manager observation or startup authority.
type transitionEffectCancellation struct {
	context.Context
	path string
}

func (ctx transitionEffectCancellation) Err() error {
	if _, err := os.Lstat(ctx.path); err == nil {
		return context.Canceled
	}
	return ctx.Context.Err()
}

func TestInstallationNativeTransitionRetainsPartialBirthUntilClose(t *testing.T) {
	parent, info, name := transitionParent(t)
	path := filepath.Join(parent, name)
	ctx := transitionEffectCancellation{Context: t.Context(), path: path}
	j, err := CreateTransition(ctx, parent, info, name)
	if j == nil || !errors.Is(err, context.Canceled) || j.parent == nil || j.directory == nil || j.directory.file == nil || j.directory.root == nil {
		t.Fatal("post-birth refusal discarded original custody", err)
	}
	parentRoot, directoryRoot, file := j.parent, j.directory.root, j.directory.file
	if _, err := file.Stat(); err != nil {
		t.Fatal("partial custody closed before owner join", err)
	}
	if err := j.Write(t.Context(), Transitions, "0001.json", []byte("later\n")); !errors.Is(err, context.Canceled) {
		t.Fatal("fresh caller renewed failed birth", err)
	}
	if err := j.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("close erased birth cancellation", err)
	}
	for _, root := range []*os.Root{parentRoot, directoryRoot} {
		if _, err := root.Stat("."); err == nil {
			t.Fatal("partial root survived physical close")
		}
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("partial descriptor survived physical close", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("failed birth was silently removed", err)
	}
}

func TestInstallationNativeTransitionRetainsWrittenCancellationAndFailureOnlyRecord(t *testing.T) {
	j, path := transitionFixture(t)
	if err := j.Ensure(t.Context(), Replacements); err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("c", 64) + ".json"
	body := []byte("original replacement\n")
	ctx := transitionEffectCancellation{Context: t.Context(), path: filepath.Join(path, "replacements", name)}
	if err := j.Write(ctx, Replacements, name, body); !errors.Is(err, context.Canceled) {
		t.Fatal("written cancellation was lost", err)
	}
	actual, err := os.ReadFile(ctx.path)
	if err != nil || !bytes.Equal(actual, body) || !bytes.Equal(j.Bytes(Replacements, name), body) {
		t.Fatal("visible original provenance was discarded", err)
	}
	if _, err := j.groups[Replacements].file.Stat(); err != nil {
		t.Fatal("record group closed before original owner join", err)
	}
	if err := j.RecordFailure(ctx, "0002.json", []byte("original failure\n"), context.Canceled); err != nil {
		t.Fatal("failure-only bookkeeping refused original custody", err)
	}
	if err := j.Write(t.Context(), Transitions, "0003.json", []byte("renewed\n")); !errors.Is(err, context.Canceled) {
		t.Fatal("bookkeeping renewed ordinary writes", err)
	}
	if err := j.Observe(); !errors.Is(err, context.Canceled) {
		t.Fatal("observation erased first failure", err)
	}
	if _, err := os.Lstat(filepath.Join(path, "0003.json")); !os.IsNotExist(err) {
		t.Fatal("later phase was written", err)
	}
}

func TestInstallationNativeTransitionRefusesForeignOriginalsAndRetainsFailure(t *testing.T) {
	for _, mutation := range []string{"inventory", "record-inode", "record-bytes", "record-hardlink", "directory-access", "parent-inode"} {
		t.Run(mutation, func(t *testing.T) {
			j, path := transitionFixture(t)
			if err := j.Write(t.Context(), Transitions, "0001.json", []byte("original\n")); err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(path, "0001.json")
			var err error
			switch mutation {
			case "inventory":
				err = os.WriteFile(filepath.Join(path, "foreign"), []byte("foreign\n"), 0600)
			case "record-inode":
				err = os.Rename(filename, filepath.Join(filepath.Dir(j.parentPath), "original-record"))
				if err == nil {
					err = os.WriteFile(filename, []byte("original\n"), 0600)
				}
			case "record-bytes":
				err = os.WriteFile(filename, []byte("tampered\n"), 0600)
				if err == nil {
					originalTime := j.directory.records["0001.json"].identity.ModTime()
					err = os.Chtimes(filename, originalTime, originalTime)
				}
			case "record-hardlink":
				err = os.Link(filename, filepath.Join(filepath.Dir(j.parentPath), "alias"))
			case "directory-access":
				err = os.Chmod(path, 0750)
			case "parent-inode":
				err = os.Rename(j.parentPath, j.parentPath+".original")
				if err == nil {
					err = os.Mkdir(j.parentPath, 0700)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := j.Observe(); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign physical original accepted", err)
			}
			if mutation == "directory-access" {
				if err := os.Chmod(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := j.Ensure(t.Context(), Creations); !errors.Is(err, ErrBinding) {
				t.Fatal("fresh caller renewed foreign journal", err)
			}
			if err := j.Close(); !errors.Is(err, ErrBinding) {
				t.Fatal("close erased substitution failure", err)
			}
		})
	}
}

func TestInstallationNativeTransitionRefusesOpenNamesBeforeEffects(t *testing.T) {
	for _, name := range []string{"../escape.json", "0008.json", "", strings.Repeat("d", 64) + ".json"} {
		t.Run(name, func(t *testing.T) {
			j, path := transitionFixture(t)
			if err := j.Write(t.Context(), Transitions, name, []byte("foreign\n")); !errors.Is(err, ErrInput) {
				t.Fatal("open transition name accepted", err)
			}
			entries, err := os.ReadDir(path)
			if err != nil || len(entries) != 0 {
				t.Fatal("invalid name caused a filesystem effect", err)
			}
		})
	}
	for _, group := range []Collection{Transitions, 255} {
		j, path := transitionFixture(t)
		if err := j.Ensure(t.Context(), group); !errors.Is(err, ErrInput) {
			t.Fatal("open collection accepted", err)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			t.Fatal("invalid collection caused a filesystem effect", err)
		}
	}
}
