//go:build linux || darwin

package durablefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenatOpensRetainedChildWithoutFollowingSymlinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "child")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	fd, err := Openat(int(directory.Fd()), "child", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), path)
	body, err := io.ReadAll(file)
	if err != nil || string(body) != "original" {
		t.Fatalf("retained child read: %q %v", body, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Openat(int(directory.Fd()), "link", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0); !errors.Is(err, syscall.ELOOP) {
		t.Fatalf("symlink child admitted: %v", err)
	}
	if _, err := Openat(int(directory.Fd()), "child", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("regular file admitted as directory: %v", err)
	}
	if _, err := Openat(int(directory.Fd()), "absent", syscall.O_RDONLY|syscall.O_NOFOLLOW, 0); !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("absent child admitted: %v", err)
	}
}
