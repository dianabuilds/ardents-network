package forwarding

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStartupAbsenceReobservesEveryPresentInodeAndDirectoryBirth(t *testing.T) {
	for _, kind := range []string{"regular", "directory", "dangling-symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			paths := []string{filepath.Join(directory, "first"), filepath.Join(directory, "middle"), filepath.Join(directory, "last")}
			if err := observeStartupAbsences(paths); err != nil {
				t.Fatal("fresh absence refused", err)
			}
			switch kind {
			case "regular":
				if err := os.WriteFile(paths[1], nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(paths[1], 0700); err != nil {
					t.Fatal(err)
				}
			case "dangling-symlink":
				if err := os.Symlink("missing-target", paths[1]); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(paths[1], 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := observeStartupAbsences(paths); !errors.Is(err, ErrBinding) {
				t.Fatal("present inode accepted as absent", kind, err)
			}
			assertStartupAbsenceDirectoryClosed(t, directory)
			if err := os.Remove(paths[1]); err != nil {
				t.Fatal(err)
			}
			// Only the physical mechanism is tested here; its owning file/Startup
			// observer retains refusal and cannot revive after a later restoration.
			if err := observeStartupAbsences(paths); err != nil {
				t.Fatal("new physical observation refused", err)
			}
			assertStartupAbsenceDirectoryClosed(t, directory)
		})
	}
	directory := filepath.Join(t.TempDir(), "future")
	paths := []string{filepath.Join(directory, "first"), filepath.Join(directory, "second")}
	if err := observeStartupAbsences(paths); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[1], nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := observeStartupAbsences(paths); !errors.Is(err, ErrBinding) {
		t.Fatal("fresh parent birth reused old absence", err)
	}
	assertStartupAbsenceDirectoryClosed(t, directory)
}

func TestStartupAbsenceRejectsNonDirectoryAndSymlinkParents(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "regular")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := observeStartupAbsences([]string{filepath.Join(file, "child")}); !errors.Is(err, ErrBinding) {
		t.Fatal("non-directory parent accepted", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if err := observeStartupAbsences([]string{filepath.Join(link, "absent")}); !errors.Is(err, ErrBinding) {
		t.Fatal("symlink parent accepted", err)
	}
}

func assertStartupAbsenceDirectoryClosed(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal("invalid environment: actual Linux descriptor inventory required", err)
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && target == directory {
			t.Fatal("physical namespace directory descriptor leaked", entry.Name())
		}
	}
}

// Both strategies observe actual filesystem absence, not Startup or authority.
// The fixture has three present search directories and thirteen absent ones.
func BenchmarkStartupAbsenceSearchPaths(b *testing.B) {
	root := b.TempDir()
	var paths []string
	for i := range 16 {
		directory := filepath.Join(root, fmt.Sprintf("directory-%02d", i))
		if i < 3 {
			if err := os.Mkdir(directory, 0700); err != nil {
				b.Fatal(err)
			}
		}
		for n := range 23 {
			paths = append(paths, filepath.Join(directory, fmt.Sprintf("name-%02d", n)))
		}
	}
	b.Run("absolute-lstat", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, path := range paths {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("fresh-directory", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := observeStartupAbsences(paths); err != nil {
				b.Fatal(err)
			}
		}
	})
}
