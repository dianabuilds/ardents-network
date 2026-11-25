//go:build linux || darwin

package launch

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRootDirectoryInspectionRejectsMutablePaths(t *testing.T) {
	root := string(filepath.Separator)
	observed, err := rootDirectoryAncestors(root)
	if err != nil || len(observed) != 1 {
		t.Fatalf("inspect filesystem root: %v, %v", observed, err)
	}
	current, err := os.Lstat(root)
	if err != nil || !sameObservedDirectory(observed[root], current) {
		t.Fatalf("filesystem root identity lost: %v", err)
	}
	directory := t.TempDir()
	if err := os.Chmod(directory, 0777); err != nil {
		t.Fatal(err)
	}
	mutable, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if rootDirectory(mutable) || privateJournalDirectory(mutable) || sameObservedDirectory(current, mutable) {
		t.Fatal("mutable directory passed a root ownership or identity check")
	}
	if _, err := rootDirectoryAncestors(directory); !errors.Is(err, ErrNativeUnavailable) {
		t.Fatalf("mutable ancestor inspection: %v", err)
	}
	if err := syncDirectDirectory(directory); !errors.Is(err, ErrNativeUnavailable) {
		t.Fatalf("mutable directory sync: %v", err)
	}
}

func TestRootFileInspectionRejectsHardLinksAndSubstitution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record")
	body := []byte("record")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	first, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	same, err := os.Lstat(path)
	if err != nil || !sameObservedFile(first, same) {
		t.Fatalf("unchanged file did not retain identity: %v", err)
	}
	group := first.Sys().(*syscall.Stat_t).Gid
	wantRoot := os.Geteuid() == 0 && group == 0
	expected := fileObservation{identity: first, body: body, gid: group, mode: first.Mode()}
	if rootOwnedFile(first) != wantRoot || sameOwnedFile(first, same) != wantRoot ||
		observedFileMatches(expected, same) != (os.Geteuid() == 0) {
		t.Fatal("original file was misclassified for its actual owner")
	}
	link := path + ".link"
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	linked, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if rootOwnedFile(linked) || sameOwnedFile(first, linked) || sameObservedFile(first, linked) || observedFileMatches(expected, linked) {
		t.Fatal("hard-linked file retained custody")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if sameObservedFile(first, replacement) || observedFileMatches(expected, replacement) {
		t.Fatal("byte-identical replacement inherited the original identity")
	}
}
