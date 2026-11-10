package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// These are actual metadata observations, not an accepting installation handle.
func TestCompletedNamedObservationRetainsOriginalAndRefusesSubstitution(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "input")
	if err := os.WriteFile(path, []byte("original"), 0400); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	owner := &inputDirectory{file: parent}
	named, err := statCompletedName(owner, "input", false)
	if err != nil || !sameCompletedFile(before, named) {
		t.Fatal("fresh original metadata refused", err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0400); err != nil {
		t.Fatal(err)
	}
	named, err = statCompletedName(owner, "input", false)
	if err != nil || sameCompletedFile(before, named) {
		t.Fatal("substituted inode accepted or fixture unavailable", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("input.original", path); err != nil {
		t.Fatal(err)
	}
	if _, err := statCompletedName(owner, "input", false); !errors.Is(err, syscall.ELOOP) {
		t.Fatal("fresh named observation followed a symlink", err)
	}
	if err := parent.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := statCompletedName(owner, "input", false); !errors.Is(err, syscall.EBADF) {
		t.Fatal("lost original parent descriptor was replaced", err)
	}
}

func TestCompletedNamedObservationKeepsFileMetadataRestrictions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("original"), 0400); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	original := *before.Sys().(*syscall.Stat_t)
	for name, change := range map[string]func(*syscall.Stat_t){
		"device": func(s *syscall.Stat_t) { s.Dev ^= 1 },
		"inode":  func(s *syscall.Stat_t) { s.Ino ^= 1 },
		"mode":   func(s *syscall.Stat_t) { s.Mode ^= 0200 },
		"type":   func(s *syscall.Stat_t) { s.Mode = s.Mode&07777 | syscall.S_IFIFO },
		"owner":  func(s *syscall.Stat_t) { s.Uid ^= 1 },
		"group":  func(s *syscall.Stat_t) { s.Gid ^= 1 },
		"links":  func(s *syscall.Stat_t) { s.Nlink++ },
		"size":   func(s *syscall.Stat_t) { s.Size++ },
		"ctime":  func(s *syscall.Stat_t) { s.Ctim.Nsec ^= 1 },
		"mtime":  func(s *syscall.Stat_t) { s.Mtim.Nsec ^= 1 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := original
			change(&changed)
			if sameCompletedFile(before, changed) {
				t.Fatal("changed original file restriction accepted")
			}
		})
	}
	accessed := original
	accessed.Atim.Nsec ^= 1
	if !sameCompletedFile(before, accessed) {
		t.Fatal("read access time became an immutable-file restriction")
	}
}

func TestCompletedNamedDirectoryAllowsContentsAndRefusesOtherKinds(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	owner := &inputDirectory{file: parent}
	if err := os.Mkdir(filepath.Join(directory, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	named, err := statCompletedName(owner, "directory", true)
	if err != nil || !sameCompletedDirectory(before, named) || sameCompletedFile(before, named) {
		t.Fatal("directory contents changed identity/access policy", err)
	}
	contents := named
	contents.Size++
	contents.Nlink++
	contents.Ctim.Nsec ^= 1
	contents.Mtim.Nsec ^= 1
	if !sameCompletedDirectory(before, contents) {
		t.Fatal("mutable directory contents became an immutable-file restriction")
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	// With no writer, dropping O_NONBLOCK would hang this real FIFO opening.
	named, err = statCompletedName(owner, "fifo", false)
	if err != nil || named.Mode&syscall.S_IFMT != syscall.S_IFIFO || sameCompletedFile(before, named) || sameCompletedDirectory(before, named) {
		t.Fatal("nonblocking non-regular observation admitted a directory/file", err)
	}
	if _, err := statCompletedName(owner, "fifo", true); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatal("directory opening accepted another inode kind", err)
	}
}
