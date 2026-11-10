//go:build installation_native

package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func completedClockNativeDirectory(t *testing.T) string {
	t.Helper()
	root := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	info, err := os.Lstat(root)
	if os.Geteuid() != 0 || !filepath.IsAbs(root) || err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		t.Fatal("invalid environment: root and trusted direct native parent required", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != 0 || st.Gid != 0 {
		t.Fatal("invalid native parent owner")
	}
	dir, err := os.MkdirTemp(root, "completed-clock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return dir
}
func completedClockNativeNoDescriptor(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatal(err)
		}
		if target == path || strings.HasPrefix(target, path+" (deleted)") {
			t.Fatal("clock observation retained its descriptor", entry.Name())
		}
	}
}
func TestInstallationNativeCompletedClockAllowsLiveWriterAndCloses(t *testing.T) {
	dir := completedClockNativeDirectory(t)
	path := filepath.Join(dir, "clock")
	parent, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	original := parent
	t.Cleanup(func() {
		if parent != nil {
			if err := parent.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	owner := &inputDirectory{file: original}
	for _, body := range []string{"", "one observation\n", "a later longer root writer observation\n"} {
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if err := checkCompletedClock(owner, path); err != nil {
			t.Fatal("current root clock refused", err)
		}
		completedClockNativeNoDescriptor(t, path)
	}
	if err := parent.Close(); err != nil {
		t.Fatal(err)
	}
	parent = nil
	if err := checkCompletedClock(owner, path); !errors.Is(err, syscall.EBADF) {
		t.Fatal("lost original parent borrowed another descriptor", err)
	}
}
func TestInstallationNativeCompletedClockRefusesPhysicalSubstitution(t *testing.T) {
	root := completedClockNativeDirectory(t)
	dir := filepath.Join(root, "original")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "clock")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Rename(dir, dir+".retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkCompletedClock(&inputDirectory{file: parent}, path); !errors.Is(err, ErrBinding) {
		t.Fatal("opened and freshly named different inode accepted", err)
	}
	completedClockNativeNoDescriptor(t, filepath.Join(dir+".retained", "clock"))
}
func TestInstallationNativeCompletedClockRefusesTypeAccessAndLinks(t *testing.T) {
	for _, mode := range []string{"symlink", "fifo", "directory", "hardlink", "writable", "special-mode", "foreign-owner"} {
		t.Run(mode, func(t *testing.T) {
			dir := completedClockNativeDirectory(t)
			path := filepath.Join(dir, "clock")
			parent, err := os.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := parent.Close(); err != nil {
					t.Error(err)
				}
			}()
			switch mode {
			case "symlink":
				err = os.Symlink("missing", path)
			case "fifo":
				err = syscall.Mkfifo(path, 0644)
			case "directory":
				err = os.Mkdir(path, 0644)
			default:
				err = os.WriteFile(path, []byte("root observation\n"), 0644)
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "hardlink":
					err = os.Link(path, path+".alias")
				case "writable":
					err = os.Chmod(path, 0666)
				case "special-mode":
					err = os.Chmod(path, 0644|os.ModeSetuid)
				case "foreign-owner":
					err = os.Chown(path, 997, 997)
				}
			}
			if err != nil {
				t.Fatal("invalid native fixture", err)
			}
			if err = checkCompletedClock(&inputDirectory{file: parent}, path); err == nil {
				t.Fatal("wrong clock type/access/link accepted", mode)
			}
			completedClockNativeNoDescriptor(t, path)
		})
	}
}
